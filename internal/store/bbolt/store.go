// Package bbolt implements [store.ResourcePatchStore] backed by a BoltDB
// database file. It supports configurable durability, periodic sync, and
// optional s2 compression.
package bbolt

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.etcd.io/bbolt"

	"github.com/loog-project/loog/internal/store"
)

const (
	typeSnapshot byte = 1 << iota
	typePatch
)

var (
	bucketSnapshots = []byte("snapshots") // <obj>|rev  -> type_byte + payload
	bucketLatest    = []byte("latest")    // <obj>      -> uint64(nextRevisionCounter)
)

// Options controls how the store behaves.
type Options struct {
	// Codec to use for marshal/unmarshal. Nil means DefaultCodec (pooled msgpack).
	Codec store.Codec

	// Durable controls whether writes are fsynced to disk. When true and
	// SyncInterval is zero, every write triggers an fsync (safest, slowest).
	Durable bool

	// SyncInterval, when positive, decouples fsyncs from individual writes.
	// Writes go to the OS page cache immediately (~30us) and a background
	// goroutine calls db.Sync() at this interval. On crash, at most one
	// interval worth of writes may be lost. A good default is 50ms.
	// Only meaningful when Durable is true; ignored when Durable is false.
	SyncInterval time.Duration

	// Compress, when true, applies s2 compression to payloads before storing.
	// Reduces DB file size at a small CPU cost.
	Compress bool
}

type Store struct {
	db    *bbolt.DB
	codec store.Codec

	nextRevisionCounterMutex sync.RWMutex
	nextRevisionCounter      map[string]uint64

	compress bool

	stopSync  chan struct{} // nil when no periodic sync
	syncDone  chan struct{} // closed by syncLoop when it returns
	closeOnce sync.Once
}

var _ store.ResourcePatchStore = (*Store)(nil)

// New opens (or creates) a BoltDB-backed store. For full control use [NewWithOptions].
func New(path string, codec store.Codec, durable bool) (*Store, error) {
	return NewWithOptions(path, Options{
		Codec:   codec,
		Durable: durable,
	})
}

// NewWithOptions opens (or creates) a BoltDB-backed store with full control
// over durability, compression, and sync behavior.
func NewWithOptions(path string, opts Options) (*Store, error) {
	codec := opts.Codec
	if codec == nil {
		codec = store.DefaultCodec
	}

	// When SyncInterval is set, we disable per-write fsync and sync in the background.
	noSync := !opts.Durable
	if opts.Durable && opts.SyncInterval > 0 {
		noSync = true
	}

	db, err := bbolt.Open(path, 0600, &bbolt.Options{
		Timeout:      0,
		NoSync:       noSync,
		NoGrowSync:   noSync,
		FreelistType: bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, b := range [][]byte{bucketSnapshots, bucketLatest} {
			if _, e := tx.CreateBucketIfNotExists(b); e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create default buckets: %w", err)
	}

	s := &Store{
		db:                  db,
		codec:               codec,
		nextRevisionCounter: make(map[string]uint64),
		compress:            opts.Compress,
	}

	// Start periodic sync goroutine if configured.
	if opts.Durable && opts.SyncInterval > 0 {
		s.stopSync = make(chan struct{})
		s.syncDone = make(chan struct{})
		go s.syncLoop(opts.SyncInterval)
	}

	return s, nil
}

// syncLoop calls db.Sync() at the given interval until stopSync is closed.
func (s *Store) syncLoop(interval time.Duration) {
	defer close(s.syncDone)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.db.Sync(); err != nil {
				log.Error().Err(err).Msg("periodic db.Sync failed")
			}
		case <-s.stopSync:
			return
		}
	}
}

// Close flushes any pending writes and closes the database. It is idempotent.
func (s *Store) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.stopSync != nil {
			close(s.stopSync)
			// Wait for syncLoop to return so it can't call db.Sync()
			// concurrently with db.Close().
			<-s.syncDone
		}
		// Always do a final sync to make sure everything is on disk.
		if syncErr := s.db.Sync(); syncErr != nil {
			log.Error().Err(syncErr).Msg("Error syncing database before closing")
		}
		err = s.db.Close()
	})
	return err
}

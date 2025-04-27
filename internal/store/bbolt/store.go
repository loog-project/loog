package bbolt

import (
	"fmt"
	"log"
	"sync"

	"github.com/loog-project/loog/internal/store"
	"go.etcd.io/bbolt"
)

const (
	TypeSnapshot byte = 1 << iota
	TypePatch
)

var (
	bucketSnapshots = []byte("snapshots") // <obj>|rev  -> RevisionSnapshot
	bucketLatest    = []byte("latest")    // <obj>      -> uint64(latestRev)
)

type Store struct {
	db    *bbolt.DB
	codec store.Codec

	nextRevisionCounterMutex sync.RWMutex
	nextRevisionCounter      map[string]uint64

	durable bool
}

var _ store.ResourcePatchStore = (*Store)(nil)

// New opens (or creates) a BoltDB database file.
// Pass nil for [codec] to use the default MessagePack implementation.
func New(path string, codec store.Codec, durable bool) (*Store, error) {
	if codec == nil {
		codec = store.DefaultCodec
	}
	db, err := bbolt.Open(path, 0666, &bbolt.Options{
		Timeout:      0,
		NoSync:       !durable,
		NoGrowSync:   !durable,
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
		durable:             durable,
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if !s.durable {
		if err := s.db.Sync(); err != nil {
			log.Println("failed to sync database:", err)
		}
	}
	return s.db.Close()
}

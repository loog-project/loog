package bbolt

import (
	"errors"
	"fmt"
	"sync"

	"github.com/loog-project/loog/internal/patch"
	"github.com/loog-project/loog/internal/store"
	"go.etcd.io/bbolt"
)

var (
	bucketSnapshots = []byte("snapshots")   // <obj>|rev  -> RevisionSnapshot
	bucketChunks    = []byte("patchChunks") // <obj>|chunkID -> []rawPatch
	bucketIndex     = []byte("index")       // <obj>|rev  -> indexEntry
	bucketLatest    = []byte("latest")      // <obj>      -> uint64(latestRev)
)

var (
	errIndexEntryMissing  = errors.New("index entry missing")
	errRevisionIsSnapshot = errors.New("revision is a snapshot")
	errPatchChunkMissing  = errors.New("patch chunk missing")
)

const chunkSize = 64 // patches per chunk value

// ------------------------- index entry ---------------------------------------

type indexEntry struct {
	Snap   bool   `msgpack:"s"`
	Chunk  uint64 `msgpack:"c"`
	Offset uint16 `msgpack:"o"`
}

type rawPatch struct {
	Data []byte `msgpack:"d"`
}

// ------------------------- Store ---------------------------------------------

// ObjectMeta is stored in the `metadata` bucket.
type ObjectMeta struct {
	LatestRevision patch.RevisionID `msgpack:"latest_id"`
	LatestGen      int64            `msgpack:"latest_gen"`
}

type Store struct {
	db    *bbolt.DB
	codec store.Codec

	head  map[string]uint64 // hot cache: objectID -> latest rev
	mutex sync.RWMutex

	counterMu sync.RWMutex
	counter   map[string]uint64
}

var _ store.ResourcePatchStore = (*Store)(nil)

// New opens (or creates) a BoltDB database file.
// Pass nil for [codec] to use the default MessagePack implementation.
func New(path string, codec store.Codec) (*Store, error) {
	if codec == nil {
		codec = store.DefaultCodec
	}
	db, err := bbolt.Open(path, 0666, &bbolt.Options{
		Timeout:      0,
		FreelistType: bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}
	err = db.Update(func(tx *bbolt.Tx) error {
		for _, b := range [][]byte{bucketSnapshots, bucketChunks, bucketIndex, bucketLatest} {
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
	return &Store{
		db:      db,
		codec:   codec,
		head:    make(map[string]uint64),
		counter: make(map[string]uint64),
	}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

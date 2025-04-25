package bbolt

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/loog-project/loog/internal/patch"
	"github.com/loog-project/loog/internal/store"
	"github.com/vmihailenco/msgpack/v5"
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

var (
	counterMu sync.RWMutex
	counter   = map[string]uint64{}
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
		db:    db,
		codec: codec,
		head:  make(map[string]uint64),
	}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) GetSnapshot(_ context.Context, obj string, revID patch.RevisionID) (*patch.RevisionSnapshot, error) {
	var snapshot patch.RevisionSnapshot
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketSnapshots).Get(keyObjectRevision(obj, revID))
		if v == nil {
			return store.ErrNotFound
		}
		return s.codec.Unmarshal(v, &snapshot)
	})

	return &snapshot, err
}

func (s *Store) GetPatch(_ context.Context, obj string, revID patch.RevisionID) (*patch.RevisionPatch, error) {
	var patchRec patch.RevisionPatch
	err := s.db.View(func(tx *bbolt.Tx) error {
		idxBytes := tx.Bucket(bucketIndex).Get(keyObjectRevision(obj, revID))
		if idxBytes == nil {
			return errIndexEntryMissing
		}
		var idx indexEntry
		if err := msgpack.Unmarshal(idxBytes, &idx); err != nil {
			return err
		}
		if idx.Snap {
			return errRevisionIsSnapshot
		}

		chunkBytes := tx.Bucket(bucketChunks).Get(keyObjectChunk(obj, idx.Chunk))
		if chunkBytes == nil {
			return errPatchChunkMissing
		}
		var arr []rawPatch
		if err := s.codec.Unmarshal(chunkBytes, &arr); err != nil {
			return err
		}
		return s.codec.Unmarshal(arr[idx.Offset].Data, &patchRec)
	})
	return &patchRec, err
}

// setLatest updates the latest revision for the given object in the database and
func (s *Store) setLatest(tx *bbolt.Tx, obj string, rev uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, rev)
	if err := tx.Bucket(bucketLatest).Put([]byte(obj), buf); err != nil {
		return err
	}

	s.mutex.Lock()
	s.head[obj] = rev
	s.mutex.Unlock()

	return nil
}

package bbolt

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
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

// SaveSnapshot stores a [RevisionSnapshot] and updates metadata
func (s *Store) SaveSnapshot(_ context.Context, obj string, snap *patch.RevisionSnapshot) error {
	revision := idToUint64(snap.ID)
	payload, _ := s.codec.Marshal(snap)

	return s.db.Update(func(tx *bbolt.Tx) error {
		if err := tx.Bucket(bucketSnapshots).Put(keyObjRev(obj, revision), payload); err != nil {
			return err
		}
		raw, err := msgpack.Marshal(indexEntry{Snap: true})
		if err != nil {
			return err
		}
		err = tx.Bucket(bucketIndex).Put(keyObjRev(obj, revision), raw)
		if err != nil {
			return err
		}
		return s.setLatest(tx, obj, revision)
	})
}

// SavePatch stores a [RevisionPatch] and updates metadata
func (s *Store) SavePatch(_ context.Context, obj string, p *patch.RevisionPatch) error {
	revision := idToUint64(p.ID)
	payload, err := s.codec.Marshal(p)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		chunkID := revision / chunkSize
		offset := uint16(revision % chunkSize)

		cKey := keyObjChunk(obj, chunkID)
		var chunk []rawPatch
		if v := tx.Bucket(bucketChunks).Get(cKey); v != nil {
			// if the chunk already exists, unmarshal it
			if err := s.codec.Unmarshal(v, &chunk); err != nil {
				return err
			}
		} else {
			// if the chunk doesn't exist, create a new one
			chunk = make([]rawPatch, chunkSize)
		}
		chunk[offset] = rawPatch{Data: payload}

		// store the chunk
		encoded, err := s.codec.Marshal(chunk)
		if err != nil {
			return err
		}
		err = tx.Bucket(bucketChunks).Put(cKey, encoded)
		if err != nil {
			return err
		}

		// index entry
		idxBytes, err := msgpack.Marshal(indexEntry{
			Snap:   false,
			Chunk:  chunkID,
			Offset: offset,
		})
		if err != nil {
			return err
		}
		err = tx.Bucket(bucketIndex).Put(keyObjRev(obj, revision), idxBytes)
		if err != nil {
			return err
		}
		return s.setLatest(tx, obj, revision)
	})
}

func (s *Store) GetLatestRevision(_ context.Context, obj string) (patch.RevisionID, error) {
	s.mutex.RLock()
	if rev, ok := s.head[obj]; ok {
		s.mutex.RUnlock()
		return uint64ToID(rev), nil
	}
	s.mutex.RUnlock()

	var rev uint64
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketLatest).Get([]byte(obj))
		if v == nil {
			return store.ErrNotFound
		}
		rev = binary.BigEndian.Uint64(v)
		return nil
	})
	if err != nil {
		return "", err
	}

	s.mutex.Lock()
	s.head[obj] = rev
	s.mutex.Unlock()
	return uint64ToID(rev), nil
}

func (s *Store) GetSnapshot(_ context.Context, obj string, revID patch.RevisionID) (*patch.RevisionSnapshot, error) {
	rev := idToUint64(revID)

	var snapshot patch.RevisionSnapshot
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketSnapshots).Get(keyObjRev(obj, rev))
		if v == nil {
			return store.ErrNotFound
		}
		return s.codec.Unmarshal(v, &snapshot)
	})

	return &snapshot, err
}

func (s *Store) GetPatch(_ context.Context, obj string, revID patch.RevisionID) (*patch.RevisionPatch, error) {
	rev := idToUint64(revID)
	var patchRec patch.RevisionPatch

	err := s.db.View(func(tx *bbolt.Tx) error {
		idxBytes := tx.Bucket(bucketIndex).Get(keyObjRev(obj, rev))
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

		chunkBytes := tx.Bucket(bucketChunks).Get(keyObjChunk(obj, idx.Chunk))
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

func keyObjRev(obj string, rev uint64) []byte {
	buf := make([]byte, len(obj)+1+8)
	copy(buf, obj)
	buf[len(obj)] = '|'
	binary.BigEndian.PutUint64(buf[len(obj)+1:], rev)
	return buf
}

func keyObjChunk(obj string, chunk uint64) []byte {
	buf := make([]byte, len(obj)+1+8)
	copy(buf, obj)
	buf[len(obj)] = '|'
	binary.BigEndian.PutUint64(buf[len(obj)+1:], chunk)
	return buf
}

func idToUint64(id patch.RevisionID) uint64 {
	n, _ := strconv.ParseUint(id, 16, 64)
	return n
}
func uint64ToID(n uint64) patch.RevisionID {
	return fmt.Sprintf("%016x", n)
}

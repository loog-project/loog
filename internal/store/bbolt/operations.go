package bbolt

import (
	"context"
	"encoding/binary"

	"github.com/loog-project/loog/internal/patch"
	"github.com/loog-project/loog/internal/store"
	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// SaveSnapshot stores a full snapshot and bumps the counter.
func (s *Store) SaveSnapshot(
	_ context.Context,
	objectID string,
	snapshot *patch.RevisionSnapshot,
) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revNum, err := s.claimNextRevision(tx, objectID)
		if err != nil {
			return err
		}
		snapshot.ID = revNum

		// save the payload
		key := keyObjectRevision(objectID, revNum)
		payload, err := s.codec.Marshal(snapshot)
		if err != nil {
			return err
		}
		err = tx.Bucket(bucketSnapshots).Put(key, payload)
		if err != nil {
			return err
		}

		// update the index
		indexBytes, err := msgpack.Marshal(indexEntry{Snap: true})
		if err != nil {
			return err
		}
		return tx.Bucket(bucketIndex).Put(key, indexBytes)
	})
}

// SavePatch stores a delta and bumps the counter.
func (s *Store) SavePatch(
	_ context.Context,
	objectID string,
	rec *patch.RevisionPatch,
) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revNum, err := s.claimNextRevision(tx, objectID)
		if err != nil {
			return err
		}
		rec.ID = revNum

		chunkID := uint64(revNum) / chunkSize
		offset := uint16(revNum % chunkSize)
		recBytes, err := s.codec.Marshal(rec)
		if err != nil {
			return err
		}
		if err := s.putChunk(tx, objectID, chunkID, offset, recBytes); err != nil {
			return err
		}
		idx := indexEntry{Snap: false, Chunk: chunkID, Offset: offset}
		idxBytes, err := msgpack.Marshal(&idx)
		if err != nil {
			return err
		}
		return tx.Bucket(bucketIndex).Put(keyObjectRevision(objectID, revNum), idxBytes)
	})
}

func (s *Store) GetSnapshot(_ context.Context, obj string, revID patch.RevisionID) (*patch.RevisionSnapshot, error) {
	var snapshot patch.RevisionSnapshot
	err := s.db.View(func(tx *bbolt.Tx) error {
		key := keyObjectRevision(obj, revID)
		v := tx.Bucket(bucketSnapshots).Get(key)
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
			return errIndexEntryMissing // TODO(future): should this be store.ErrNotFound?
		}
		var idx indexEntry
		if err := msgpack.Unmarshal(idxBytes, &idx); err != nil {
			return err
		}
		if idx.Snap {
			return errRevisionIsSnapshot
		}

		// read chunks
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

// GetLatestRevision returns the highest committed revision for objectID.
func (s *Store) GetLatestRevision(
	_ context.Context,
	objectID string,
) (patch.RevisionID, error) {
	// check cache first
	s.counterMu.RLock()
	if next, ok := s.counter[objectID]; ok {
		s.counterMu.RUnlock()
		return patch.RevisionID(next - 1), nil
	}
	s.counterMu.RUnlock()

	var next uint64
	err := s.db.View(func(tx *bbolt.Tx) error {
		v := tx.Bucket(bucketLatest).Get([]byte(objectID))
		if v == nil {
			return store.ErrNotFound
		}
		next = binary.BigEndian.Uint64(v)
		return nil
	})
	if err != nil {
		return 0, err
	}

	s.counterMu.Lock()
	s.counter[objectID] = next
	s.counterMu.Unlock()
	return patch.RevisionID(next - 1), nil
}

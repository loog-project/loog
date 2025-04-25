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
		revNum, err := claimNextRevision(tx, objectID)
		if err != nil {
			return err
		}
		snapshot.ID = patch.RevisionID(revNum)

		key := keyObjectRevision(objectID, revNum)
		payload, err := s.codec.Marshal(snapshot)
		if err != nil {
			return err
		}

		if err := tx.Bucket(bucketSnapshots).Put(key, payload); err != nil {
			return err
		}
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
		revNum, err := claimNextRevision(tx, objectID)
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

// GetLatestRevision returns the highest committed revision for objectID.
func (s *Store) GetLatestRevision(
	_ context.Context,
	objectID string,
) (patch.RevisionID, error) {

	// check cache first
	counterMu.RLock()
	if next, ok := counter[objectID]; ok {
		counterMu.RUnlock()
		return patch.RevisionID(next - 1), nil
	}
	counterMu.RUnlock()

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

	counterMu.Lock()
	counter[objectID] = next
	counterMu.Unlock()
	return patch.RevisionID(next - 1), nil
}

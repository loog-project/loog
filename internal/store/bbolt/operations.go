package bbolt

import (
	"context"
	"encoding/binary"

	"go.etcd.io/bbolt"

	"github.com/loog-project/loog/internal/store"
)

func (s *Store) Get(
	_ context.Context,
	uid string,
	revisionID store.RevisionID,
) (snapshot *store.Snapshot, patch *store.Patch, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error {
		key := keyObjectRevision(uid, revisionID)
		v := tx.Bucket(bucketSnapshots).Get(key)
		if v == nil {
			return store.ErrNotFound
		}
		snapshot, patch, err = s.parsePatchOrSnapshot(v)
		return err
	})
	return
}

func (s *Store) SetSnapshot(_ context.Context, uid string, snapshot *store.Snapshot) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revisionID, err := s.claimNextRevision(tx, uid)
		if err != nil {
			return err
		}
		snapshot.ID = revisionID

		key := keyObjectRevision(uid, revisionID)
		payload, err := s.codec.Marshal(snapshot)
		if err != nil {
			return err
		}

		buf := make([]byte, 1+len(payload))
		buf[0] = TypeSnapshot
		copy(buf[1:], payload)

		return tx.Bucket(bucketSnapshots).Put(key, buf)
	})
}

func (s *Store) SetPatch(_ context.Context, uid string, patch *store.Patch) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revisionID, err := s.claimNextRevision(tx, uid)
		if err != nil {
			return err
		}
		patch.ID = revisionID

		key := keyObjectRevision(uid, revisionID)
		payload, err := s.codec.Marshal(patch)
		if err != nil {
			return err
		}

		buf := make([]byte, 1+len(payload))
		buf[0] = TypePatch
		copy(buf[1:], payload)

		return tx.Bucket(bucketSnapshots).Put(key, buf)
	})
}

// GetLatestRevision returns the highest committed revision for objectID.
func (s *Store) GetLatestRevision(
	_ context.Context,
	objectID string,
) (store.RevisionID, error) {
	// check cache first
	s.nextRevisionCounterMutex.RLock()
	if next, ok := s.nextRevisionCounter[objectID]; ok {
		s.nextRevisionCounterMutex.RUnlock()
		return store.RevisionID(next - 1), nil
	}
	s.nextRevisionCounterMutex.RUnlock()

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

	s.nextRevisionCounterMutex.Lock()
	s.nextRevisionCounter[objectID] = next
	s.nextRevisionCounterMutex.Unlock()

	return store.RevisionID(next - 1), nil
}

func (s *Store) WalkObjectRevisions(yield func(string, store.RevisionID, *store.Snapshot, *store.Patch) bool) error {
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSnapshots)

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			uid, revisionID := splitObjectRevisionKey(k)
			if uid == "" {
				continue
			}

			snapshot, patch, err := s.parsePatchOrSnapshot(v)
			if err != nil {
				return err
			}
			if !yield(uid, revisionID, snapshot, patch) {
				return nil
			}
		}
		return nil
	})
	return err
}

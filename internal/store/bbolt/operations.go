package bbolt

import (
	"context"
	"encoding/binary"
	"sync"

	"go.etcd.io/bbolt"

	"github.com/loog-project/loog/internal/store"
)

// payloadPool reuses buffers for type-tag + marshalled payload merging.
var payloadPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 2048)
		return &b
	},
}

func (s *Store) Get(
	_ context.Context,
	uid string,
	revisionID store.RevisionID,
) (snapshot *store.Snapshot, patch *store.Patch, err error) {
	err = s.db.View(func(tx *bbolt.Tx) error {
		bp := keyObjectRevisionPooled(uid, revisionID)
		v := tx.Bucket(bucketSnapshots).Get(*bp)
		putKeyBuf(bp)
		if v == nil {
			return store.ErrNotFound
		}
		snapshot, patch, err = s.parsePatchOrSnapshot(v)
		return err
	})
	return
}

// storeRevision is the shared write logic for both snapshots and patches.
// It claims a revision, sets the ID on the value, marshals, optionally
// compresses, prepends the type tag, and puts the result into bbolt.
func (s *Store) storeRevision(tx *bbolt.Tx, uid string, typeByte byte, revisionID store.RevisionID, v any) error {
	key := keyObjectRevision(uid, revisionID)
	payload, err := s.codec.Marshal(v)
	if err != nil {
		return err
	}

	if s.compress {
		payload = compressPayload(payload)
	}

	// Merge type tag + payload using pooled buffer
	bp := payloadPool.Get().(*[]byte)
	needed := 1 + len(payload)
	if cap(*bp) < needed {
		*bp = make([]byte, needed)
	} else {
		*bp = (*bp)[:needed]
	}
	(*bp)[0] = typeByte
	copy((*bp)[1:], payload)

	err = tx.Bucket(bucketSnapshots).Put(key, *bp)
	payloadPool.Put(bp)
	return err
}

func (s *Store) SetSnapshot(_ context.Context, uid string, snapshot *store.Snapshot) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revisionID, err := s.claimNextRevision(tx, uid)
		if err != nil {
			return err
		}
		snapshot.ID = revisionID
		return s.storeRevision(tx, uid, TypeSnapshot, revisionID, snapshot)
	})
}

func (s *Store) SetPatch(_ context.Context, uid string, patch *store.Patch) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		revisionID, err := s.claimNextRevision(tx, uid)
		if err != nil {
			return err
		}
		patch.ID = revisionID
		return s.storeRevision(tx, uid, TypePatch, revisionID, patch)
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
	return s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucketSnapshots).Cursor()
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
}

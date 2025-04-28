package bbolt

import (
	"encoding/binary"

	"github.com/loog-project/loog/internal/store"
	"go.etcd.io/bbolt"
)

func keyObjectRevision(objectUID string, id store.RevisionID) []byte {
	buf := make([]byte, len(objectUID)+1+8)
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], uint64(id))
	return buf
}

// claimNextRevision atomically increments the nextRevisionCounter in bucketLatest *and*
// updates the in-memory cache. It returns the newly assigned revision number.
func (s *Store) claimNextRevision(tx *bbolt.Tx, objectID string) (store.RevisionID, error) {
	latest := tx.Bucket(bucketLatest)

	var next uint64
	if raw := latest.Get([]byte(objectID)); raw != nil {
		next = binary.BigEndian.Uint64(raw)
	}
	revisionNumber := store.RevisionID(next)
	next++

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, next)
	if err := latest.Put([]byte(objectID), buf); err != nil {
		return 0, err
	}

	s.nextRevisionCounterMutex.Lock()
	s.nextRevisionCounter[objectID] = next
	s.nextRevisionCounterMutex.Unlock()

	return revisionNumber, nil
}

// setLatest updates the latest revision for the given object in the database.
func (s *Store) setLatest(tx *bbolt.Tx, obj string, revisionID store.RevisionID) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(revisionID))
	if err := tx.Bucket(bucketLatest).Put([]byte(obj), buf); err != nil {
		return err
	}
	return nil
}

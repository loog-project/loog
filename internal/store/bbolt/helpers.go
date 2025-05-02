package bbolt

import (
	"bytes"
	"encoding/binary"

	"go.etcd.io/bbolt"

	"github.com/loog-project/loog/internal/store"
)

func keyObjectRevision(objectUID string, id store.RevisionID) []byte {
	buf := make([]byte, len(objectUID)+1+8)
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], uint64(id))
	return buf
}

func splitObjectRevisionKey(key []byte) (string, store.RevisionID) {
	sep := bytes.IndexByte(key, '|')
	if sep == -1 {
		return "", 0
	}
	objectUID := string(key[:sep])
	id := binary.BigEndian.Uint64(key[sep+1:])
	return objectUID, store.RevisionID(id)
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

func (s *Store) parsePatchOrSnapshot(v []byte) (*store.Snapshot, *store.Patch, error) {
	switch v[0] {
	case TypePatch:
		var patch store.Patch
		return nil, &patch, s.codec.Unmarshal(v[1:], &patch)
	case TypeSnapshot:
		var snapshot store.Snapshot
		return &snapshot, nil, s.codec.Unmarshal(v[1:], &snapshot)
	default:
		return nil, nil, store.ErrInvalidRevision
	}
}

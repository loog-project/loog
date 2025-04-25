package bbolt

import (
	"encoding/binary"

	"github.com/loog-project/loog/internal/patch"
	"go.etcd.io/bbolt"
)

func keyObjectRevision(objectUID string, id patch.RevisionID) []byte {
	buf := make([]byte, len(objectUID)+1+8)
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], uint64(id))
	return buf
}

func keyObjectChunk(objectUID string, chunkID uint64) []byte {
	buf := make([]byte, len(objectUID)+1+8)
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], chunkID)
	return buf
}

// claimNextRevision atomically increments the counter in bLatest *and*
// updates the in-memory cache. It returns the newly assigned revision number.
func claimNextRevision(tx *bbolt.Tx, objectID string) (patch.RevisionID, error) {
	latest := tx.Bucket(bucketLatest)

	raw := latest.Get([]byte(objectID))
	var next uint64
	if raw != nil {
		next = binary.BigEndian.Uint64(raw)
	}
	revisionNumber := next
	next++

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, next)
	if err := latest.Put([]byte(objectID), buf); err != nil {
		return 0, err
	}

	counterMu.Lock()
	counter[objectID] = next
	counterMu.Unlock()

	return patch.NewRevisionID(revisionNumber), nil
}

// putChunk updates (or creates) the patch chunk for objectID/chunkID and
// returns the encoded offset that SavePatch must store in the index bucket.
func (s *Store) putChunk(
	tx *bbolt.Tx,
	objectID string,
	chunkID uint64,
	offset uint16,
	patchBytes []byte,
) error {
	cKey := keyObjectChunk(objectID, chunkID)

	var chunk []rawPatch
	if v := tx.Bucket(bucketChunks).Get(cKey); v != nil {
		if err := s.codec.Unmarshal(v, &chunk); err != nil {
			return err
		}
	} else {
		chunk = make([]rawPatch, chunkSize)
	}
	chunk[offset] = rawPatch{Data: patchBytes}

	enc, err := s.codec.Marshal(chunk)
	if err != nil {
		return err
	}
	return tx.Bucket(bucketChunks).Put(cKey, enc)
}

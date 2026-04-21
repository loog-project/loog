package bbolt

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/klauspost/compress/s2"
	"go.etcd.io/bbolt"

	"github.com/loog-project/loog/internal/store"
)

// Pool for key construction buffers. Most object UIDs are 36-char UUIDs,
// so 36+1+8 = 45 bytes is the common size.
var keyPool = sync.Pool{
	New: func() any {
		return new(make([]byte, 0, 45))
	},
}

func keyObjectRevision(objectUID string, id store.RevisionID) []byte {
	size := len(objectUID) + 1 + 8
	buf := make([]byte, size)
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], uint64(id))
	return buf
}

// keyObjectRevisionPooled writes the key into a pooled buffer and returns it.
// Caller must call putKeyBuf when done with the returned *[]byte.
func keyObjectRevisionPooled(objectUID string, id store.RevisionID) *[]byte {
	bp := keyPool.Get().(*[]byte)
	size := len(objectUID) + 1 + 8
	if cap(*bp) < size {
		*bp = make([]byte, size)
	} else {
		*bp = (*bp)[:size]
	}
	buf := *bp
	copy(buf, objectUID)
	buf[len(objectUID)] = '|'
	binary.BigEndian.PutUint64(buf[len(objectUID)+1:], uint64(id))
	return bp
}

func putKeyBuf(bp *[]byte) {
	keyPool.Put(bp)
}

func splitObjectRevisionKey(key []byte) (string, store.RevisionID) {
	before, after, ok := bytes.Cut(key, []byte{'|'})
	if !ok || len(after) < 8 {
		return "", 0
	}
	objectUID := string(before)
	id := binary.BigEndian.Uint64(after)
	return objectUID, store.RevisionID(id)
}

// claimNextRevision atomically increments the nextRevisionCounter in bucketLatest
// and updates the in-memory cache. Returns the newly assigned revision number.
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
	err := latest.Put([]byte(objectID), buf)
	if err != nil {
		return 0, err
	}

	s.nextRevisionCounterMutex.Lock()
	s.nextRevisionCounter[objectID] = next
	s.nextRevisionCounterMutex.Unlock()

	return revisionNumber, nil
}

// compressPool reuses destination buffers for s2 compression.
var compressPool = sync.Pool{
	New: func() any {
		return new(make([]byte, 0, 2048))
	},
}

// compressPayload compresses raw payload bytes using s2. Returns the
// compressed bytes (which may be backed by a pooled buffer: caller should copy if the data outlives the current scope).
func compressPayload(raw []byte) []byte {
	bp := compressPool.Get().(*[]byte)
	*bp = s2.Encode(*bp, raw)
	out := make([]byte, len(*bp))
	copy(out, *bp)
	compressPool.Put(bp)
	return out
}

// decompressPayload decompresses an s2-compressed payload.
func decompressPayload(compressed []byte) ([]byte, error) {
	return s2.Decode(nil, compressed)
}

func (s *Store) parsePatchOrSnapshot(v []byte) (*store.Snapshot, *store.Patch, error) {
	if len(v) < 1 {
		return nil, nil, store.ErrInvalidRevision
	}
	payload := v[1:]
	if s.compress {
		var err error
		payload, err = decompressPayload(payload)
		if err != nil {
			return nil, nil, err
		}
	}
	switch v[0] {
	case typePatch:
		var patch store.Patch
		return nil, &patch, s.codec.Unmarshal(payload, &patch)
	case typeSnapshot:
		var snapshot store.Snapshot
		return &snapshot, nil, s.codec.Unmarshal(payload, &snapshot)
	default:
		return nil, nil, store.ErrInvalidRevision
	}
}

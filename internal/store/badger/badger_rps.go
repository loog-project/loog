package badger

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/loog-project/loog/internal/store"
)

var _ store.ResourcePatchStore = (*badgerResourcePatchStore)(nil)

type badgerResourcePatchStore struct {
	db *badger.DB
}

// NewResourcePatchStore returns a new ResourcePatchStore using Badger as the underlying database.
func NewResourcePatchStore(dir string, syncWrites bool) (store.ResourcePatchStore, error) {
	opts := badger.
		DefaultOptions(filepath.Clean(dir)).
		WithSyncWrites(syncWrites).
		WithCompression(options.ZSTD)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}
	return &badgerResourcePatchStore{
		db: db,
	}, nil
}

func (b *badgerResourcePatchStore) keySnapshot(uid string, revision uint64) []byte {
	// note that the prefix is also used in the [LatestRevision] method
	// and should be changed if the prefix changes
	return []byte(fmt.Sprintf("s/%s/%d", uid, revision))
}

func (b *badgerResourcePatchStore) keyPatch(uid string, revision uint64) []byte {
	// note that the prefix is also used in the [WalkPatches] method
	// and should be changed if the prefix changes
	return []byte(fmt.Sprintf("p/%s/%d", uid, revision))
}

func (b *badgerResourcePatchStore) keyLatestRevision(uid string) []byte {
	return []byte(fmt.Sprintf("l/%s", uid))
}

// Commit stores the payload in the database with the given uid and revision.
func (b *badgerResourcePatchStore) Commit(ctx context.Context, uid string, revision uint64, isSnapshot bool, payload []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		var key []byte
		if isSnapshot {
			key = b.keySnapshot(uid, revision)
		} else {
			key = b.keyPatch(uid, revision)
		}
		if err := txn.Set(key, payload); err != nil {
			return fmt.Errorf("failed to set key %s: %w", key, err)
		}
		if err := txn.Set(b.keyLatestRevision(uid), []byte(fmt.Sprintf("%d", revision))); err != nil {
			return fmt.Errorf("failed to set latest revision for uid %s: %w", uid, err)
		}
		return nil
	})
}

// LatestRevision retrieves the latest revision for the given uid.
func (b *badgerResourcePatchStore) LatestRevision(ctx context.Context, uid string) (uint64, error) {
	var rev uint64
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(b.keyLatestRevision(uid))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				// no error, just no revision
				rev = 0
				return nil
			}
			return fmt.Errorf("failed to get latest revision for uid %s: %w", uid, err)
		}
		err = item.Value(func(val []byte) error {
			rev = binary.BigEndian.Uint64(val)
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to copy value for uid %s: %w", uid, err)
		}
		return nil
	})
	return rev, err
}

func (b *badgerResourcePatchStore) LoadSnapshot(
	ctx context.Context,
	uid string,
	upto uint64,
) (payload []byte, snapRev uint64, err error) {
	err = b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		seekKey := b.keySnapshot(uid, upto)
		it.Seek(seekKey)

		prefix := []byte(fmt.Sprintf("s/%s/", uid))
		if !it.ValidForPrefix(prefix) || bytes.Compare(it.Item().Key(), seekKey) > 0 {
			it.Rewind()
			if !it.ValidForPrefix(prefix) {
				return fmt.Errorf("no snapshot found for uid %s", uid)
			}
		}

		snapRev = binary.BigEndian.Uint64(it.Item().Key()[len(prefix):])
		return it.Item().Value(func(val []byte) error {
			payload = make([]byte, len(val))
			copy(payload, val)
			return nil
		})
	})
	return
}

// WalkPatches retrieves all patches for the given uid between the from and to revisions.
func (b *badgerResourcePatchStore) WalkPatches(ctx context.Context, uid string, from, to uint64, fn func(revision uint64, payload []byte) error) error {
	if from > to {
		return nil // nothing to walk
	}
	patchPrefix := []byte(fmt.Sprintf("p/%s/", uid))
	return b.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		startKey := b.keyPatch(uid, from)
		endKey := b.keyPatch(uid, to)

		for it.Seek(startKey); it.Valid(); it.Next() {
			key := it.Item().Key()
			if bytes.Compare(key, endKey) >= 0 {
				break // reached the end key
			}
			revision := binary.BigEndian.Uint64(key[len(patchPrefix):])
			err := it.Item().Value(func(val []byte) error {
				return fn(revision, val)
			})
			if err != nil {
				return fmt.Errorf("error while walking: %w", err)
			}
		}
		return nil
	})
}

// Close flushes and closes the underlying DB
func (b *badgerResourcePatchStore) Close() error {
	return b.db.Close()
}

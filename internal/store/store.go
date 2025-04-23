package store

import "context"

type ResourcePatchStore interface {
	Commit(ctx context.Context, uid string, revision uint64, isSnapshot bool, payload []byte) error
	WalkPatches(ctx context.Context, uid string, from, to uint64, fn func(revision uint64, payload []byte) error) error
	LoadSnapshot(ctx context.Context, uid string, upto uint64) (payload []byte, snapRev uint64, err error)
	LatestRevision(ctx context.Context, uid string) (uint64, error)
	Close() error
}

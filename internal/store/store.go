package store

import (
	"context"
	"errors"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidRevision = errors.New("invalid revision")
)

// ResourcePatchStore is an interface for storing and retrieving resource patches and snapshots.
type ResourcePatchStore interface {
	Get(ctx context.Context, objectID string, revID RevisionID) (*Snapshot, *Patch, error)

	SetSnapshot(ctx context.Context, objectID string, snap *Snapshot) error
	SetPatch(ctx context.Context, objectID string, p *Patch) error

	GetLatestRevision(ctx context.Context, objectID string) (RevisionID, error)
	WalkObjectRevisions(yield func(string, RevisionID, *Snapshot, *Patch) bool) error
	Close() error
}

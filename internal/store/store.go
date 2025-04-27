package store

import (
	"context"
	"errors"

	"github.com/loog-project/loog/internal/patch"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrInvalidRevision = errors.New("invalid revision")
)

type ResourcePatchStore interface {
	Get(ctx context.Context, objectID string, revID patch.RevisionID) (*patch.RevisionSnapshot, *patch.RevisionPatch, error)

	SetSnapshot(ctx context.Context, objectID string, snap *patch.RevisionSnapshot) error
	SetPatch(ctx context.Context, objectID string, p *patch.RevisionPatch) error

	GetLatestRevision(ctx context.Context, objectID string) (patch.RevisionID, error)
	Close() error
}

package store

import (
	"context"
	"errors"

	"github.com/loog-project/loog/internal/patch"
)

var ErrNotFound = errors.New("not found")

type ResourcePatchStore interface {
	SaveSnapshot(ctx context.Context, objectID string, snap *patch.RevisionSnapshot) error
	SavePatch(ctx context.Context, objectID string, p *patch.RevisionPatch) error

	GetSnapshot(ctx context.Context, objectID string, revID patch.RevisionID) (*patch.RevisionSnapshot, error)
	GetPatch(ctx context.Context, objectID string, revID patch.RevisionID) (*patch.RevisionPatch, error)

	GetLatestRevision(ctx context.Context, objectID string) (patch.RevisionID, error)
	Close() error
}

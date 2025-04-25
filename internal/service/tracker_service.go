package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/loog-project/loog/internal/patch"
	"github.com/loog-project/loog/internal/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TrackerService is a service that tracks changes to Kubernetes resources.
// It stores the changes in a resource patch store and allows restoring
// the full object state at a specific revision.
type TrackerService struct {
	rps           store.ResourcePatchStore
	snapshotEvery uint64 // create full snapshot after this many patches
}

// NewTrackerService creates a new TrackerService instance.
func NewTrackerService(rps store.ResourcePatchStore, snapshotEvery uint64) *TrackerService {
	if snapshotEvery == 0 {
		snapshotEvery = 10
	}
	return &TrackerService{
		rps:           rps,
		snapshotEvery: snapshotEvery,
	}
}

// Commit persists *obj* and returns the new revision ID.
func (t *TrackerService) Commit(
	ctx context.Context,
	objID string,
	obj *unstructured.Unstructured,
) (patch.RevisionID, error) {
	latest, err := t.rps.GetLatestRevision(ctx, objID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return 0, err
		}

		snapshot := patch.RevisionSnapshot{Object: obj.Object}
		if err := t.rps.SaveSnapshot(ctx, objID, &snapshot); err != nil {
			return 0, err
		}

		return snapshot.ID, nil
	}

	chain, err := t.patchDistance(ctx, objID, latest)
	if err != nil {
		return 0, err
	}

	// check if it's time for a full snapshot
	if uint64(chain) >= t.snapshotEvery-1 {
		snapshot := patch.RevisionSnapshot{
			PreviousID: latest,
			Object:     obj.Object,
		}
		err = t.rps.SaveSnapshot(ctx, objID, &snapshot)
		if err != nil {
			return 0, err
		}
		return snapshot.ID, nil
	}

	// reconstruct latest state to diff
	base, err := t.Restore(ctx, objID, latest)
	if err != nil {
		return 0, err
	}

	operations, err := patch.Diff(base.Object, obj.Object)
	if err != nil {
		return 0, err
	}

	p := &patch.RevisionPatch{
		PreviousID: latest,
		Operations: operations,
	}
	err = t.rps.SavePatch(ctx, objID, p)
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// Restore brings back the object state at *rev*.
func (t *TrackerService) Restore(ctx context.Context, objID string, rev patch.RevisionID) (*patch.RevisionSnapshot, error) {
	var chain []patch.RevisionID
	cur := rev
	for {
		if snap, err := t.rps.GetSnapshot(ctx, objID, cur); err == nil {
			// we have now found the base snapshot
			state := snap.Object
			for i := len(chain) - 1; i >= 0; i-- {
				p, err := t.rps.GetPatch(ctx, objID, chain[i])
				if err != nil {
					return nil, fmt.Errorf("broken chain at %s: %w", chain[i], err)
				}
				newData, err := patch.ApplyOperations(state, p.Operations)
				if err != nil {
					return nil, fmt.Errorf("failed to apply operations: %w", err)
				}
				err = json.Unmarshal(newData, &state)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal new data: %w", err)
				}
			}
			return &patch.RevisionSnapshot{
				ID:     rev,
				Object: state,
			}, nil
		}
		p, err := t.rps.GetPatch(ctx, objID, cur)
		if err != nil {
			return nil, fmt.Errorf("broken chain at %s: %w", cur, err)
		}
		chain = append(chain, cur)
		cur = p.PreviousID
	}
}

func (t *TrackerService) patchDistance(ctx context.Context, obj string, from patch.RevisionID) (int, error) {
	n := 0
	cur := from
	for {
		if _, err := t.rps.GetSnapshot(ctx, obj, cur); err == nil {
			return n, nil
		}
		p, err := t.rps.GetPatch(ctx, obj, cur)
		if err != nil {
			return 0, err
		}
		n++
		cur = p.PreviousID
	}
}

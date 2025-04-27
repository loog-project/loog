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
func (t *TrackerService) Restore(ctx context.Context, objID string, revision patch.RevisionID) (*patch.RevisionSnapshot, error) {
	var patchChain []*patch.RevisionPatch
	currentRevision := revision

	// build the chain of patches
	for {
		// first we need to find the base snapshot
		// meaning either the current revision is already a snapshot
		// or we need to find the previous snapshot
		snapshot, err := t.rps.GetSnapshot(ctx, objID, currentRevision)
		if err == nil {
			// we have now found some base snapshot so we can use the chain (which might be empty if base == revision)
			// to reconstruct the object state at revision
			state := snapshot.Object
			for i := len(patchChain) - 1; i >= 0; i-- {
				currentPatch := patchChain[i]

				newData, err := patch.ApplyOperations(state, currentPatch.Operations)
				if err != nil {
					return nil, fmt.Errorf("failed to apply operations: %w", err)
				}

				// newData is the state at currentPatch.ID
				// we need to unmarshal it back to the original object type so in the next iteration
				// we can apply the next patch
				err = json.Unmarshal(newData, &state)
				if err != nil {
					return nil, fmt.Errorf("failed to unmarshal new data: %w", err)
				}
			}
			return &patch.RevisionSnapshot{
				ID:     revision,
				Object: state,
			}, nil
		}
		if !errors.Is(err, store.ErrNotFound) { // TODO(future): maybe optimize this using !=
			// the error is not related to building the chain
			return nil, err
		}
		p, err := t.rps.GetPatch(ctx, objID, currentRevision)
		if err != nil {
			return nil, fmt.Errorf("broken chain at %s: %w", currentRevision, err)
		}
		patchChain = append(patchChain, p)

		currentRevision = p.PreviousID
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

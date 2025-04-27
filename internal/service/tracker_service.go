package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/pkg/diffmap"
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
		snapshotEvery = 8
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
	newObject *unstructured.Unstructured,
) (store.RevisionID, error) {
	latest, err := t.rps.GetLatestRevision(ctx, objID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return 0, err
		}

		snapshot := store.Snapshot{Object: newObject.Object}
		if err := t.rps.SetSnapshot(ctx, objID, &snapshot); err != nil {
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
		snapshot := store.Snapshot{
			PreviousID: latest,
			Object:     newObject.Object,
		}
		err = t.rps.SetSnapshot(ctx, objID, &snapshot)
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

	p := &store.Patch{
		PreviousID: latest,
		Patch:      diffmap.Diff(base.Object, newObject.Object),
	}
	err = t.rps.SetPatch(ctx, objID, p)
	if err != nil {
		return 0, err
	}
	return p.ID, nil
}

// Restore brings back the object state at *rev*.
func (t *TrackerService) Restore(ctx context.Context, objID string, revision store.RevisionID) (*store.Snapshot, error) {
	var patchChain []*store.Patch
	currentRevision := revision

	// build the chain of patches
	for {
		snapshot, p, err := t.rps.Get(ctx, objID, currentRevision)
		if err != nil {
			return nil, err
		}

		if snapshot != nil {
			// we have found the base snapshot, so we can use the chain to reconstruct the object state
			state := snapshot.Object
			for i := len(patchChain) - 1; i >= 0; i-- {
				currentPatch := patchChain[i]
				diffmap.Apply(state, currentPatch.Patch)
			}
			return &store.Snapshot{
				ID:     revision,
				Object: state,
			}, nil
		}

		if p != nil {
			patchChain = append(patchChain, p)
			currentRevision = p.PreviousID

			continue // find the next patch / snapshot
		}

		// if we reach here, it means we have no more patches or snapshots
		// but we have not found the base snapshot
		return nil, fmt.Errorf("no base snapshot found for revision %d", revision)
	}
}

func (t *TrackerService) patchDistance(ctx context.Context, obj string, from store.RevisionID) (int, error) {
	n := 0
	cur := from
	for {
		snapshot, p, err := t.rps.Get(ctx, obj, cur)
		if err != nil {
			return 0, err
		}
		if snapshot != nil {
			return n, nil
		}
		if p != nil {
			n++
			cur = p.PreviousID
			continue
		}
		// if we reach here, it means we have no more patches or snapshots
		// but we have not found the base snapshot
		return 0, fmt.Errorf("no base snapshot found for revision %d", from)
	}
}

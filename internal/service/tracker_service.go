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
		if errors.Is(err, store.ErrNotFound) {
			// if there's not yet a revision, we can just save the object as a snapshot
			snap := &patch.RevisionSnapshot{
				Object: obj.Object,
			}
			if err := t.rps.SaveSnapshot(ctx, objID, snap); err != nil {
				return 0, err
			}
			return snap.ID, nil
		}
		return 0, err
	}

	chain, err := t.patchDistance(ctx, objID, latest)
	if err != nil {
		return 0, err
	}

	if uint64(chain) >= t.snapshotEvery-1 {
		snap := &patch.RevisionSnapshot{
			PreviousID: latest,
			Object:     obj.Object,
		}
		if err := t.rps.SaveSnapshot(ctx, objID, snap); err != nil {
			return 0, err
		}
		return snap.ID, nil
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
	if err := t.rps.SavePatch(ctx, objID, p); err != nil {
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

// ------------------- helpers --------------------------------------------------

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

/*

func (t *TrackerService) countPatchesSinceLastSnapshot(
	ctx context.Context,
	objectID string,
	from patch.RevisionID,
) (int, error) {
	n := 0
	curr := from
	for {
		if _, err := t.rps.GetSnapshot(ctx, objectID, curr); err == nil {
			return n, nil
		}
		p, err := t.rps.GetPatch(ctx, objectID, curr)
		if err != nil {
			return 0, err
		}
		n++
		curr = p.PreviousID
	}
}

func (t *TrackerService) Commit(
	ctx context.Context,
	objectID string,
	obj *unstructured.Unstructured,
) (patch.RevisionID, error) {
	newRevisionID := patch.NewRevisionID()

	latest, err := t.rps.GetLatestRevision(ctx, objectID)
	if err != nil {
		// if there's not yet a revision, we can just save the object as a snapshot
		if errors.Is(err, store.ErrNotFound) {
			snapshot := &patch.RevisionSnapshot{
				ID:     newRevisionID,
				Object: obj.Object,
			}
			return newRevisionID, t.rps.SaveSnapshot(ctx, objectID, snapshot)
		}
		return "", err
	}

	// TODO: this can be optimized by using a cache

	// reconstruct latest object to compute diff
	restoredObject, err := t.RestoreAtRevision(ctx, objectID, latest)
	if err != nil {
		return "", err
	}

	operations, err := patch.Diff(restoredObject.Object, obj.Object)
	if err != nil {
		return "", err
	}

	chainLen, err := t.countPatchesSinceLastSnapshot(ctx, objectID, latest)
	if err != nil {
		return "", err
	}

	if uint64(chainLen)+1 >= t.snapshotEvery {
		snapshot := &patch.RevisionSnapshot{
			ID:         newRevisionID,
			PreviousID: latest,
			Object:     obj.Object,
		}
		return newRevisionID, t.rps.SaveSnapshot(ctx, objectID, snapshot)
	}
	p := &patch.RevisionPatch{
		ID:         newRevisionID,
		PreviousID: latest,
		Operations: operations,
	}
	return newRevisionID, t.rps.SavePatch(ctx, objectID, p)
}

// RestoreAtRevision rebuilds the full object state at *revID*.
func (t *TrackerService) RestoreAtRevision(
	ctx context.Context,
	objectID string,
	revID patch.RevisionID,
) (*patch.RevisionSnapshot, error) {
	var chain []patch.RevisionID // patches to replay (youngest→oldest)
	curr := revID
	for {
		if snap, err := t.rps.GetSnapshot(ctx, objectID, curr); err == nil {
			if snap.ID == revID {
				// we are already at the requested revision, no need to replay patches
				return snap, nil
			}
			// Found base snapshot – apply patches in reverse.
			state := snap.Object
			for i := len(chain) - 1; i >= 0; i-- {
				p, _ := t.st.GetPatch(ctx, objectID, chain[i])
				data, _ := patch.ApplyOperations(state, p.Operations)
				if err := json.Unmarshal(data, &state); err != nil {
					return nil, err
				}
			}
			return &patch.RevisionSnapshot{ID: revID, Object: state, Generation: snap.Generation}, nil
		}
		p, err := t.st.GetPatch(ctx, objectID, curr)
		if err != nil {
			return nil, fmt.Errorf("revision chain broken at %s: %w", curr, err)
		}
		chain = append(chain, curr)
		curr = p.PreviousID
	}
}

*/

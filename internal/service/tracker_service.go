package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
)

const lockTTL = 60 * time.Second

// TrackerService is a service that tracks changes to Kubernetes resources.
// It stores the changes in a resource patch store and allows restoring
// the full object state at a specific revision.
type TrackerService struct {
	rps           store.ResourcePatchStore
	snapshotEvery uint64 // create full snapshot after this many patches

	cache *stateCache

	commitLocks           map[string]*lockWrap
	commitLockMutex       sync.RWMutex
	stopCommitLockJanitor chan struct{}
	closeOnce             sync.Once
}

type lockWrap struct {
	mu      sync.Mutex
	lastUse int64 // unix-nanosec; atomic
}

// NewTrackerService creates a new TrackerService instance.
func NewTrackerService(rps store.ResourcePatchStore, snapshotEvery uint64, withCache bool) *TrackerService {
	if snapshotEvery == 0 {
		snapshotEvery = 8
	}
	t := &TrackerService{
		rps:           rps,
		snapshotEvery: snapshotEvery,

		commitLocks:           make(map[string]*lockWrap),
		stopCommitLockJanitor: make(chan struct{}),
	}
	if withCache {
		t.cache = newStateCache()
	}
	go t.lockJanitor()
	return t
}

// Close closes the TrackerService and releases any resources it holds.
// After you call Close, the TrackerService should not be used anymore.
func (t *TrackerService) Close() error {
	if t == nil {
		return nil
	}
	t.closeOnce.Do(func() {
		close(t.stopCommitLockJanitor)

		if t.cache != nil {
			t.cache.close()
		}

		t.commitLockMutex.Lock()
		t.commitLocks = nil
		t.commitLockMutex.Unlock()
	})

	return nil
}

// DuplicateResourceVersionError is thrown when a Kubernetes object was committed that is already in the rps
type DuplicateResourceVersionError struct {
	rev             store.RevisionID
	resourceVersion string
}

func (n DuplicateResourceVersionError) Error() string {
	return fmt.Sprintf("resource version %s is already present in revision %d", n.resourceVersion, n.rev)
}

// Commit persists *obj* and returns the new revision ID.
func (t *TrackerService) Commit(
	ctx context.Context,
	objID string,
	newObject *unstructured.Unstructured,
) (store.RevisionID, error) {
	lw := t.lockObject(objID)
	defer lw.mu.Unlock()

	// try to hot-state cache
	var ts *trackerState
	if t.cache != nil {
		ts = t.cache.get(objID)
	}

	// if we have a cold start, we should load the latest revision and cache it
	if ts == nil {
		latest, err := t.rps.GetLatestRevision(ctx, objID)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				return 0, err
			}

			snapshot := newSnapshot(newObject, 0)
			if err := t.rps.SetSnapshot(ctx, objID, &snapshot); err != nil {
				return 0, err
			}

			if t.cache != nil {
				t.cache.set(objID, &trackerState{obj: snapshot.Object, rev: snapshot.ID})
			}
			return snapshot.ID, nil
		}

		// we have a valid revision, so we can use it to create a new tracker state
		snapshot, err := t.Restore(ctx, objID, latest)
		if err != nil {
			return 0, err
		}

		ts = &trackerState{obj: snapshot.Object, rev: latest}
		if t.cache != nil {
			t.cache.set(objID, ts)
		}
	}

	lastRevisionResourceVersion, ok := util.ExtractResourceVersion(ts.obj)
	if ok && lastRevisionResourceVersion == newObject.GetResourceVersion() {
		return 0, DuplicateResourceVersionError{
			rev:             ts.rev,
			resourceVersion: lastRevisionResourceVersion,
		}
	}

	patchesSince := uint64(ts.rev) % t.snapshotEvery
	if patchesSince == t.snapshotEvery-1 {
		// we are at the end of a snapshot period, so we should create a new snapshot
		snapshot := newSnapshot(newObject, ts.rev)
		err := t.rps.SetSnapshot(ctx, objID, &snapshot)
		if err != nil {
			return 0, err
		}
		ts.obj = resource.CloneMap(newObject.Object)
		ts.rev = snapshot.ID
		return snapshot.ID, nil
	}

	diff := diffmap.Diff(ts.obj, newObject.Object)
	p := newPatch(ts.rev, diff)
	err := t.rps.SetPatch(ctx, objID, &p)
	if err != nil {
		return 0, err
	}
	// Apply only the diff to the cached state instead of copying the
	// entire object. This is O(changed keys) rather than O(all keys).
	if ts.obj == nil {
		ts.obj = make(diffmap.DiffMap)
	}
	diffmap.Apply(ts.obj, diff)
	ts.rev = p.ID

	return p.ID, nil
}

func newPatch(previousID store.RevisionID, diff diffmap.DiffMap) store.Patch {
	return store.Patch{
		PreviousID: previousID,
		Patch:      diff,
		Time:       time.Now(),
	}
}

func newSnapshot(newObject *unstructured.Unstructured, previousID store.RevisionID) store.Snapshot {
	// Deep-clone the object so the snapshot is independent of the caller's map.
	// Without this, subsequent mutations of newObject.Object corrupt the stored snapshot.
	return store.Snapshot{
		PreviousID: previousID,
		Object:     resource.CloneMap(newObject.Object),
		Time:       time.Now(),
	}
}

// Restore brings back the object state at *rev*.
func (t *TrackerService) Restore(
	ctx context.Context,
	objID string,
	revision store.RevisionID,
) (*store.Snapshot, error) {
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
			// we have the final state, so we can cache it

			return &store.Snapshot{
				ID:     revision,
				Object: state,
				Time:   snapshot.Time,
			}, nil
		}

		if p != nil {
			patchChain = append(patchChain, p)
			// Guard against a corrupted chain: PreviousID must strictly
			// decrease toward the base snapshot. A self-reference or cycle
			// would otherwise loop forever and grow patchChain without bound.
			if p.PreviousID >= currentRevision {
				return nil, fmt.Errorf(
					"corrupted patch chain for %s: revision %d points back to %d",
					objID, currentRevision, p.PreviousID)
			}
			currentRevision = p.PreviousID

			continue // find the next patch / snapshot
		}

		// if we reach here, it means we have no more patches or snapshots
		// but we have not found the base snapshot
		return nil, fmt.Errorf("no base snapshot found for revision %d", revision)
	}
}

func (t *TrackerService) lockJanitor() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()
			t.commitLockMutex.Lock()
			for key, lw := range t.commitLocks {
				if now-atomic.LoadInt64(&lw.lastUse) > int64(lockTTL) && lw.mu.TryLock() {
					delete(t.commitLocks, key) // safe to drop
					lw.mu.Unlock()
				}
			}
			t.commitLockMutex.Unlock()
		case <-t.stopCommitLockJanitor:
			return
		}
	}
}

func (t *TrackerService) objLock(uid string) *lockWrap {
	t.commitLockMutex.RLock()
	mu, ok := t.commitLocks[uid]
	t.commitLockMutex.RUnlock()
	if ok {
		return mu
	}
	t.commitLockMutex.Lock()
	defer t.commitLockMutex.Unlock()
	if t.commitLocks == nil {
		// Service is closed. Hand back a standalone lock so callers don't panic
		// writing to a nil map; the janitor is stopped so there's no contention.
		return &lockWrap{lastUse: time.Now().UnixNano()}
	}
	if mu = t.commitLocks[uid]; mu == nil {
		// Stamp lastUse at creation so a brand-new lock isn't immediately
		// eligible for janitor eviction (lastUse == 0 would always look stale).
		mu = &lockWrap{lastUse: time.Now().UnixNano()}
		t.commitLocks[uid] = mu
	}
	return mu
}

// lockObject acquires the per-object commit lock and guards against the janitor
// evicting the lock (and another goroutine installing a fresh one) in the window
// between lookup and Lock. Without this, two commits for the same object could
// run concurrently on the same cached trackerState. The caller must Unlock the
// returned lock.
func (t *TrackerService) lockObject(uid string) *lockWrap {
	for {
		lw := t.objLock(uid)
		lw.mu.Lock()

		t.commitLockMutex.RLock()
		current, tracked := t.commitLocks[uid]
		closed := t.commitLocks == nil
		t.commitLockMutex.RUnlock()

		if closed || (tracked && current == lw) {
			atomic.StoreInt64(&lw.lastUse, time.Now().UnixNano())
			return lw
		}
		// The lock we grabbed was evicted; retry with the current one.
		lw.mu.Unlock()
	}
}

func (t *TrackerService) WarmCache(uid string, obj diffmap.DiffMap, rev store.RevisionID) {
	if t.cache == nil {
		return
	}
	lock := t.lockObject(uid)
	defer lock.mu.Unlock()
	t.cache.set(uid, &trackerState{obj: obj, rev: rev})
}

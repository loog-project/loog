package adapter

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/store"
)

// SortTimeline should order a timeline that was ingested grouped-by-resource
// (as bulk history load does) into a globally chronological, newest-first list.
func TestLiveStore_SortTimeline(t *testing.T) {
	s := NewLiveStore()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Ingest resource A's revisions, then resource B's, out of global time order.
	s.IngestRevision("a", "Pod", "a", "default", resource.Revision{ID: 1, Time: base.Add(1 * time.Minute)})
	s.IngestRevision("a", "Pod", "a", "default", resource.Revision{ID: 2, Time: base.Add(3 * time.Minute)})
	s.IngestRevision("b", "Pod", "b", "default", resource.Revision{ID: 3, Time: base.Add(2 * time.Minute)})
	s.IngestRevision("b", "Pod", "b", "default", resource.Revision{ID: 4, Time: base.Add(4 * time.Minute)})

	s.SortTimeline()

	tl := s.Timeline()
	if len(tl) != 4 {
		t.Fatalf("expected 4 timeline entries, got %d", len(tl))
	}
	for i := 1; i < len(tl); i++ {
		if tl[i-1].Revision.Time.Before(tl[i].Revision.Time) {
			t.Errorf("timeline not newest-first at %d: %v before %v",
				i, tl[i-1].Revision.Time, tl[i].Revision.Time)
		}
	}
	// Newest first means the +4m revision leads.
	if !tl[0].Revision.Time.Equal(base.Add(4 * time.Minute)) {
		t.Errorf("expected newest (+4m) first, got %v", tl[0].Revision.Time)
	}
}

// A revision built from a patch (no snapshot) must be MODIFIED, not ADDED.
// This mirrors the history-load path, which previously mislabeled everything.
func TestBuildRevision_PatchIsModified(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "Pod",
		"metadata": map[string]any{"uid": "u1", "name": "p"},
	}}
	patch := &store.Patch{PreviousID: 5, Time: time.Now(), Patch: map[string]any{"spec": map[string]any{"x": 1}}}

	rev := buildRevision(obj, 6, nil, patch)
	if rev.EventType != resource.EventModified {
		t.Errorf("patch revision event = %v, want MODIFIED", rev.EventType)
	}
	if rev.PreviousID != resource.RevisionID(5) {
		t.Errorf("patch revision PreviousID = %d, want 5", rev.PreviousID)
	}
	if rev.Patch == nil {
		t.Error("patch revision should carry the patch diff")
	}
}

// A snapshot revision with a non-zero PreviousID is a MODIFIED, not ADDED.
func TestBuildRevision_LaterSnapshotIsModified(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{"kind": "Pod"}}
	snap := &store.Snapshot{PreviousID: 8, Time: time.Now()}
	rev := buildRevision(obj, 16, snap, nil)
	if rev.EventType != resource.EventModified {
		t.Errorf("later snapshot event = %v, want MODIFIED", rev.EventType)
	}
}

// The very first snapshot (PreviousID 0) is ADDED.
func TestBuildRevision_FirstSnapshotIsAdded(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{"kind": "Pod"}}
	snap := &store.Snapshot{PreviousID: 0, Time: time.Now()}
	rev := buildRevision(obj, 1, snap, nil)
	if rev.EventType != resource.EventAdded {
		t.Errorf("first snapshot event = %v, want ADDED", rev.EventType)
	}
}

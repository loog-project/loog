package segment

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/pkg/diffmap"
)

func cm(uid, val string, rv string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"uid":             uid,
			"namespace":       "default",
			"name":            "cm-" + uid,
			"resourceVersion": rv,
		},
		"data": map[string]any{"val": val},
	}}
}

// Rotation must open a new file and, once the tracker cache is reset, the first
// commit for an object in the new segment must be a self-contained snapshot.
func TestSegmentedStore_RotationIsSelfContained(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	st, err := Open(dir, Options{Interval: time.Hour, Retention: 0})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	svc := service.NewTrackerService(st, 8, true)
	defer func() { _ = svc.Close() }()

	uid := "u1"
	if _, err := svc.Commit(ctx, uid, cm(uid, "a", "1")); err != nil {
		t.Fatalf("commit 1: %v", err)
	}
	if _, err := svc.Commit(ctx, uid, cm(uid, "b", "2")); err != nil {
		t.Fatalf("commit 2: %v", err)
	}

	// Force a rotation the way the recorder loop does.
	if err := st.Rotate(); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	svc.ResetCache()

	// First commit after rotation lands in the fresh segment as revision 0,
	// which must be a snapshot (PreviousID 0), not a patch against the old file.
	rev, err := svc.Commit(ctx, uid, cm(uid, "c", "3"))
	if err != nil {
		t.Fatalf("commit after rotate: %v", err)
	}
	if rev != 0 {
		t.Fatalf("expected revision 0 in fresh segment, got %d", rev)
	}
	snap, patch, err := st.Get(ctx, uid, rev)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if snap == nil || patch != nil {
		t.Fatalf("first post-rotation revision must be a snapshot, got snap=%v patch=%v", snap, patch)
	}
	if snap.PreviousID != 0 {
		t.Fatalf("post-rotation snapshot PreviousID = %d, want 0", snap.PreviousID)
	}

	segs, err := ListSegments(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments after one rotation, got %d", len(segs))
	}
}

// ListSegments must exclude a path passed as excluded (the active file).
func TestListSegments_ExcludeActive(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir, Options{Interval: time.Hour})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	active := st.ActivePath()
	_ = st.Rotate() // now there are 2 files, newest is active
	active2 := st.ActivePath()
	_ = st.Close()

	all, err := ListSegments(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(all))
	}
	excluded, err := ListSegments(dir, active2)
	if err != nil {
		t.Fatalf("list exclude: %v", err)
	}
	if len(excluded) != 1 {
		t.Fatalf("expected 1 segment after excluding active, got %d", len(excluded))
	}
	if excluded[0].Path == active2 {
		t.Fatalf("excluded path still present")
	}
	_ = active
}

// Extraction must preserve original timestamps and clip to the requested
// window, writing a self-contained, replayable .loog.
func TestExtract_PreservesTimeAndClips(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Write one segment directly so we control the timestamps precisely.
	segStore, err := Open(dir, Options{Interval: time.Hour, Bolt: bboltStore.Options{Durable: true}})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Timestamps must fall at/after the segment's creation time (as live
	// recording produces), so base off "now" plus a margin. Segment selection
	// keys off the file's start time; a window entirely before it is skipped.
	base := time.Now().Add(time.Minute).UTC()

	// Three revisions of the same object at +0m, +5m, +10m.
	writeSnap := func(uid string, prevID store.RevisionID, val string, ts time.Time, deleted bool) {
		snap := &store.Snapshot{
			PreviousID: prevID,
			Object: diffmap.DiffMap{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   diffmap.DiffMap{"uid": uid, "name": "cm", "namespace": "default"},
				"data":       diffmap.DiffMap{"val": val},
			},
			Time:    ts,
			Deleted: deleted,
		}
		if err := segStore.SetSnapshot(ctx, uid, snap); err != nil {
			t.Fatalf("set snapshot: %v", err)
		}
	}
	writeSnap("u1", 0, "v0", base, false)                    // ADDED  @ +0
	writeSnap("u1", 1, "v1", base.Add(5*time.Minute), false) // MOD    @ +5
	writeSnap("u1", 1, "v2", base.Add(10*time.Minute), true) // DELETE @ +10
	_ = segStore.Close()

	out := t.TempDir() + "/clip.loog"
	// Window covers only +4m .. +11m, so we expect the +5m and +10m revisions.
	stats, err := Extract(ctx, dir, out, ExtractOptions{
		From: base.Add(4 * time.Minute),
		To:   base.Add(11 * time.Minute),
		Bolt: bboltStore.Options{Durable: true},
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if stats.RevisionsWritten != 2 {
		t.Fatalf("expected 2 revisions written, got %d (scanned %d)", stats.RevisionsWritten, stats.RevisionsScanned)
	}

	// Read the output back and verify timestamps + delete marker survived.
	outStore, err := bboltStore.NewWithOptions(out, bboltStore.Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer func() { _ = outStore.Close() }()

	var times []time.Time
	var sawDelete bool
	err = outStore.WalkObjectRevisions(func(_ string, _ store.RevisionID, snap *store.Snapshot, _ *store.Patch) bool {
		if snap == nil {
			t.Errorf("output should contain only snapshots")
			return true
		}
		times = append(times, snap.Time.UTC())
		if snap.Deleted {
			sawDelete = true
		}
		return true
	})
	if err != nil {
		t.Fatalf("walk output: %v", err)
	}
	if len(times) != 2 {
		t.Fatalf("expected 2 output revisions, got %d", len(times))
	}
	if !times[0].Equal(base.Add(5*time.Minute)) || !times[1].Equal(base.Add(10*time.Minute)) {
		t.Fatalf("timestamps not preserved: %v", times)
	}
	if !sawDelete {
		t.Fatalf("delete marker not preserved in extract")
	}
}

// Retention must prune a segment whose window ends before the cutoff.
func TestSegmentedStore_RetentionPrunes(t *testing.T) {
	dir := t.TempDir()

	// Interval tiny so the second Open/rotate makes the first segment "old"
	// relative to a short retention.
	st, err := Open(dir, Options{Interval: time.Millisecond, Retention: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Roll a few times with gaps so earlier segments age past retention.
	for i := 0; i < 3; i++ {
		time.Sleep(30 * time.Millisecond)
		if err := st.Rotate(); err != nil {
			t.Fatalf("rotate: %v", err)
		}
	}
	_ = st.Close()

	segs, err := ListSegments(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// With a 50ms retention and ~30ms spacing, older segments should be gone;
	// at least the very first one must have been pruned.
	if len(segs) >= 4 {
		t.Fatalf("expected retention to prune old segments, still have %d", len(segs))
	}
}

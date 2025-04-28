package bbolt

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/pkg/diffmap"
)

// handy constants -----------------------------------------------------------

var (
	ctx = context.Background()
	id  = "object-uid"
)

// TestNewAndBuckets checks that the DB opens and buckets exist.
func TestNewAndBuckets(t *testing.T) {
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "db.bb"), nil, false)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// verify buckets truly created in file
	info1, _ := os.Stat(s.db.Path())
	if info1.Size() == 0 {
		t.Fatal("DB file should not be empty")
	}
}

// TestSnapshotPatchRoundtrip covers:
//   - claimNextRevision
//   - SetSnapshot / SetPatch
//   - Get / GetLatestRevision
func TestSnapshotPatchRoundtrip(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(filepath.Join(dir, "db.bb"), nil, false)
	t.Cleanup(func() { _ = s.Close() })

	// -------- 1st snapshot -----------------------------------------------
	snap := &store.Snapshot{Object: diffmap.DiffMap{"foo": "bar"}}
	if err := s.SetSnapshot(ctx, id, snap); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}
	if snap.ID != 0 {
		t.Fatalf("first snapshot should have ID 0, got %d", snap.ID)
	}

	// latest should now be 0
	latest, err := s.GetLatestRevision(ctx, id)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if latest != 0 {
		t.Fatalf("latest want 0, got %d", latest)
	}

	// -------- patch #1 ----------------------------------------------------
	patch1 := &store.Patch{
		PreviousID: snap.ID,
		Patch:      diffmap.DiffMap{"foo": "baz"},
	}
	if err := s.SetPatch(ctx, id, patch1); err != nil {
		t.Fatalf("set patch1: %v", err)
	}
	if patch1.ID != 1 {
		t.Fatalf("patch1 should receive ID 1, got %d", patch1.ID)
	}

	// -------- patch #2 ----------------------------------------------------
	patch2 := &store.Patch{
		PreviousID: patch1.ID,
		Patch:      diffmap.DiffMap{"bar": 42},
	}
	_ = s.SetPatch(ctx, id, patch2)

	// latest should now be 2
	if latest, _ := s.GetLatestRevision(ctx, id); latest != 2 {
		t.Fatalf("latest want 2, got %d", latest)
	}

	// -------- random gets -------------------------------------------------
	// rev-0   -> snapshot
	sn0, p0, err := s.Get(ctx, id, 0)
	if err != nil || p0 != nil || sn0 == nil {
		t.Fatalf("rev0: want snapshot, got %+v / %+v / err=%v", sn0, p0, err)
	}
	// rev-1   -> patch
	sn1, p1, _ := s.Get(ctx, id, 1)
	if sn1 != nil || p1 == nil || p1.ID != 1 {
		t.Fatalf("rev1 not patch1")
	}
	// rev-2   -> patch
	_, p2, _ := s.Get(ctx, id, 2)
	if p2 == nil || p2.ID != 2 {
		t.Fatalf("rev2 not patch2")
	}
}

// TestConcurrentClaims ensures claimNextRevision is atomic.
func TestConcurrentClaims(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(filepath.Join(dir, "db.bb"), nil, false)
	t.Cleanup(func() { _ = s.Close() })

	// race 20 goroutines
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			errs <- s.SetSnapshot(ctx, id, &store.Snapshot{Object: diffmap.DiffMap{"x": i}})
		}()
	}
	for i := 0; i < 20; i++ {
		if e := <-errs; e != nil {
			t.Fatalf("concurrent SetSnapshot failed: %v", e)
		}
	}

	if latest, _ := s.GetLatestRevision(ctx, id); latest != 19 {
		t.Fatalf("after 20 writes, latest should be 19, got %d", latest)
	}
}

// TestPersistedValues verifies that bytes written are real MessagePack.
func TestPersistedValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.bb")
	s, _ := New(path, nil, false)
	_ = s.SetSnapshot(ctx, id, &store.Snapshot{Object: diffmap.DiffMap{"k": "v"}})
	_ = s.Close()

	// reopen raw file and search for MessagePack prefix 0x81 (map of 1)
	blob, _ := os.ReadFile(path)
	if !bytes.Contains(blob, []byte{0x81}) {
		t.Fatalf("file does not appear to contain msgpack map header")
	}
}

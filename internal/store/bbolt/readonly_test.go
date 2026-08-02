package bbolt

import (
	"context"
	"testing"
	"time"

	"github.com/loog-project/loog/internal/store"
)

// A read-only store must serve reads from an existing file and reject writes.
func TestStore_ReadOnly(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/capture.loog"

	// Populate a normal store, then close it.
	rw, err := New(path, nil, false)
	if err != nil {
		t.Fatalf("open rw: %v", err)
	}
	snap := &store.Snapshot{
		PreviousID: 0,
		Object:     map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"uid": "u1"}},
		Time:       time.Now(),
	}
	if err := rw.SetSnapshot(ctx, "u1", snap); err != nil {
		t.Fatalf("SetSnapshot: %v", err)
	}
	if err := rw.Close(); err != nil {
		t.Fatalf("close rw: %v", err)
	}

	// Reopen read-only.
	ro, err := NewWithOptions(path, Options{ReadOnly: true})
	if err != nil {
		t.Fatalf("open ro: %v", err)
	}
	defer func() { _ = ro.Close() }()

	// Reads work.
	count := 0
	if walkErr := ro.WalkObjectRevisions(func(uid string, _ store.RevisionID, s *store.Snapshot, _ *store.Patch) bool {
		if uid == "u1" && s != nil {
			count++
		}
		return true
	}); walkErr != nil {
		t.Fatalf("walk ro: %v", walkErr)
	}
	if count != 1 {
		t.Errorf("expected 1 snapshot from read-only store, got %d", count)
	}

	// Writes must fail on a read-only store.
	if err := ro.SetSnapshot(ctx, "u2", snap); err == nil {
		t.Error("expected SetSnapshot to fail on a read-only store")
	}
}

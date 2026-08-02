package adapter

import (
	"sync"
	"testing"

	"github.com/loog-project/loog/internal/resource"
)

// TestUnwatchedKinds_ReturnedSliceNotCorruptedByAddWatchKind verifies that a
// slice returned from UnwatchedKinds() is not mutated by a later AddWatchKind
// call. Previously AddWatchKind reused the backing array via [:0], corrupting
// any slice the watch manager still held.
func TestUnwatchedKinds_ReturnedSliceNotCorruptedByAddWatchKind(t *testing.T) {
	s := NewLiveStore()
	s.SetUnwatchedKinds([]resource.Kind{
		{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
		{Kind: "Service", APIVersion: "v1", Resource: "services", Namespaced: true},
		{Kind: "ConfigMap", APIVersion: "v1", Resource: "configmaps", Namespaced: true},
	})

	held := s.UnwatchedKinds()
	if len(held) != 3 {
		t.Fatalf("expected 3 unwatched kinds, got %d", len(held))
	}
	before := make([]string, len(held))
	for i, k := range held {
		before[i] = k.Kind
	}

	// Start watching one kind; this must not mutate the slice we already hold.
	s.AddWatchKind(resource.Kind{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true})

	for i, k := range held {
		if k.Kind != before[i] {
			t.Errorf("held slice mutated at %d: got %q, want %q", i, k.Kind, before[i])
		}
	}

	// And the fresh view should have Pod removed.
	after := s.UnwatchedKinds()
	if len(after) != 2 {
		t.Fatalf("expected 2 unwatched kinds after AddWatchKind, got %d", len(after))
	}
	for _, k := range after {
		if k.Kind == "Pod" {
			t.Errorf("Pod should have been removed from unwatched kinds")
		}
	}
}

// TestSetUnwatchedKinds_CopiesInput verifies the store does not alias the
// caller's slice.
func TestSetUnwatchedKinds_CopiesInput(t *testing.T) {
	s := NewLiveStore()
	input := []resource.Kind{
		{Kind: "Pod", APIVersion: "v1", Resource: "pods", Namespaced: true},
	}
	s.SetUnwatchedKinds(input)

	// Mutate the caller's slice; the store must be unaffected.
	input[0] = resource.Kind{Kind: "Hacked"}

	got := s.UnwatchedKinds()
	if len(got) != 1 || got[0].Kind != "Pod" {
		t.Errorf("store aliased caller slice: got %+v", got)
	}
}

// TestKindGroups_ReturnsCopy verifies mutating the returned slice header does
// not affect the store's internal slice.
func TestKindGroups_ReturnsCopy(t *testing.T) {
	s := NewLiveStore()
	s.IngestRevision("uid-1", "Pod", "nginx", "default", resource.Revision{ID: 1})
	s.RebuildKindGroups()

	groups := s.KindGroups()
	if len(groups) == 0 {
		t.Fatal("expected at least one kind group")
	}
	// Truncating the returned slice must not affect the store.
	_ = groups[:0]
	if len(s.KindGroups()) == 0 {
		t.Error("store's kind groups were affected by caller truncation")
	}
}

// TestConcurrentIngestAndRead runs ingestion and reads concurrently to catch
// races under `go test -race`.
func TestConcurrentIngestAndRead(t *testing.T) {
	s := NewLiveStore()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			s.IngestRevision("uid-1", "Pod", "nginx", "default", resource.Revision{ID: resource.RevisionID(i)})
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			_ = s.AllResources()
			_ = s.Timeline()
			_ = s.KindGroups()
			s.RebuildKindGroups()
			s.ForEachResource(func(string, *resource.Data) {})
		}
	}()

	wg.Wait()
}

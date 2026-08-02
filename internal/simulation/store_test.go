package simulation

import (
	"testing"

	"github.com/loog-project/loog/internal/resource"
)

// TestGeneratedRevisionIDsAreUnique verifies that IDs produced by the store's
// generators do not collide, including against the pre-built demo data.
// Previously IDs were derived from nanosecond-masking and rand, which could
// collide and make the TUI conflate distinct revisions.
func TestGeneratedRevisionIDsAreUnique(t *testing.T) {
	s := New()

	seen := make(map[resource.RevisionID]bool)
	// Collect all pre-built IDs.
	s.ForEachResource(func(_ string, rd *resource.Data) {
		for _, rev := range rd.Revisions {
			if seen[rev.ID] {
				t.Fatalf("duplicate pre-built revision ID: %d", rev.ID)
			}
			seen[rev.ID] = true
		}
	})

	// Generate many new revisions and ensure none collide.
	all := s.AllResources()
	if len(all) == 0 {
		t.Fatal("expected pre-built resources")
	}
	for i := 0; i < 1000; i++ {
		rd := all[i%len(all)]
		rev := s.GenerateRevision(rd)
		if seen[rev.ID] {
			t.Fatalf("duplicate generated revision ID: %d (iteration %d)", rev.ID, i)
		}
		seen[rev.ID] = true
		s.AddRevision(rd.Resource.UID, rev)
		// Refresh rd so LatestRevision reflects the appended revision.
		rd = s.GetResource(rd.Resource.UID)
		all[i%len(all)] = rd
	}
}

// TestAddWatchKindIDsUnique verifies AddWatchKind assigns unique IDs that do
// not collide with existing data.
func TestAddWatchKindIDsUnique(t *testing.T) {
	s := New()

	seen := make(map[resource.RevisionID]bool)
	s.ForEachResource(func(_ string, rd *resource.Data) {
		for _, rev := range rd.Revisions {
			seen[rev.ID] = true
		}
	})

	created := s.AddWatchKind(resource.Kind{
		Kind: "Secret", APIVersion: "v1", Resource: "secrets", Namespaced: true,
	})
	if len(created) == 0 {
		t.Fatal("expected AddWatchKind to create resources")
	}
	for _, rd := range created {
		for _, rev := range rd.Revisions {
			if seen[rev.ID] {
				t.Errorf("AddWatchKind produced colliding revision ID: %d", rev.ID)
			}
			seen[rev.ID] = true
		}
	}
}

// TestGenerateRevisionFirstIsAdded verifies a resource's first generated
// revision is tagged ADDED, and subsequent ones MODIFIED with a correct chain.
func TestGenerateRevisionFirstIsAdded(t *testing.T) {
	s := NewStore(nil, nil, nil, nil)
	rd := &resource.Data{
		Resource: resource.Resource{UID: "u1", Kind: "Pod", Name: "p", Namespace: "default"},
	}

	first := s.GenerateRevision(rd)
	if first.EventType != resource.EventAdded {
		t.Errorf("first revision event = %v, want ADDED", first.EventType)
	}
	if first.PreviousID != 0 {
		t.Errorf("first revision PreviousID = %d, want 0", first.PreviousID)
	}
	rd.Revisions = append(rd.Revisions, first)

	second := s.GenerateRevision(rd)
	if second.EventType != resource.EventModified {
		t.Errorf("second revision event = %v, want MODIFIED", second.EventType)
	}
	if second.PreviousID != first.ID {
		t.Errorf("second revision PreviousID = %d, want %d", second.PreviousID, first.ID)
	}
	if second.ID == first.ID {
		t.Errorf("second revision ID must differ from first (%d)", first.ID)
	}
}

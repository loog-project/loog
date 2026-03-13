package adapter

import (
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/pkg/diffmap"
)

// LiveRevisionMsg tells the TUI that a new revision has been ingested into the LiveStore.
// The TUI should refresh its views — data is already in the store.
type LiveRevisionMsg struct {
	ResourceUID string
}

// TUIRevisionHandler implements the revisionHandler interface from cmd/root.go.
// It ingests each revision into the LiveStore and sends a LiveRevisionMsg to the
// bubbletea program so the UI refreshes.
type TUIRevisionHandler struct {
	Store   *LiveStore
	Program *tea.Program
}

// HandleRevision is called by the collector goroutine (and loadHistoryFromDB) for each
// new or historic revision. It extracts resource metadata from the unstructured object,
// builds a resource.Revision, ingests it into the LiveStore, and notifies the TUI.
//
// The obj parameter always contains the full object state at this revision:
//   - In the collector path: obj is the live watch event object
//   - In the history path: obj is reconstructed from snapshot + patches
func (h *TUIRevisionHandler) HandleRevision(
	obj *unstructured.Unstructured,
	revisionID store.RevisionID,
	snapshot *store.Snapshot,
	patch *store.Patch,
) error {
	uid := string(obj.GetUID())
	kind := obj.GetKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()

	rev := buildRevision(obj, revisionID, snapshot, patch)

	h.Store.IngestRevision(uid, kind, name, namespace, rev)

	// Send message to TUI (non-blocking: bubbletea's Send is goroutine-safe)
	if h.Program != nil {
		h.Program.Send(LiveRevisionMsg{ResourceUID: uid})
	}

	return nil
}

// buildRevision constructs a resource.Revision from the production store types.
// The full object is always taken from obj.Object (available in both live and history paths).
// Snapshot/patch provide metadata (time, previousID) and the patch diff.
func buildRevision(
	obj *unstructured.Unstructured,
	revID store.RevisionID,
	snapshot *store.Snapshot,
	patch *store.Patch,
) resource.Revision {
	rev := resource.Revision{
		ID:     revID,
		Object: resource.CloneMap(obj.Object),
	}

	if snapshot != nil {
		rev.PreviousID = snapshot.PreviousID
		rev.Time = snapshot.Time

		// First revision (no previous) → ADDED, otherwise MODIFIED
		if snapshot.PreviousID == 0 {
			rev.EventType = resource.EventAdded
		} else {
			rev.EventType = resource.EventModified
		}
	} else if patch != nil {
		rev.PreviousID = patch.PreviousID
		rev.Time = patch.Time
		rev.Patch = cloneDiffMap(patch.Patch)
		rev.EventType = resource.EventModified
	}

	return rev
}

// cloneDiffMap creates a shallow clone of a DiffMap.
func cloneDiffMap(m diffmap.DiffMap) diffmap.DiffMap {
	if m == nil {
		return nil
	}
	result := make(diffmap.DiffMap, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

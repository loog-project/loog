// Package resource defines the core domain types for Kubernetes resource tracking.
// These types are independent of any TUI framework and can be used by the store,
// simulation, analysis, and presentation layers.
package resource

import (
	"fmt"
	"time"

	"github.com/loog-project/loog/internal/store"
)

// RevisionID is a 64-bit revision identifier displayed as hex.
// It is a type alias for store.RevisionID so the two are interchangeable.
type RevisionID = store.RevisionID

// EventType represents the type of Kubernetes watch event.
type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
)

// Symbol returns a compact single-character symbol for the event type.
func (et EventType) Symbol() string {
	switch et {
	case EventAdded:
		return "+"
	case EventModified:
		return "~"
	case EventDeleted:
		return "-"
	default:
		return "?"
	}
}

// Resource represents a tracked Kubernetes resource instance.
type Resource struct {
	UID       string
	Kind      string
	Name      string
	Namespace string
	Starred   bool
}

// KindName returns "Kind/name" (e.g., "Pod/nginx-abc").
func (r Resource) KindName() string {
	return r.Kind + "/" + r.Name
}

// ShortName returns a truncated name for narrow panels.
func (r Resource) ShortName(maxLen int) string {
	runes := []rune(r.Name)
	if maxLen <= 0 || len(runes) <= maxLen {
		return r.Name
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// Revision represents a single recorded version of a resource.
type Revision struct {
	ID         RevisionID
	PreviousID RevisionID
	EventType  EventType
	Time       time.Time
	Object     map[string]any // full state at this revision
	Patch      map[string]any // diff from previous (nil for ADD/snapshots)
}

// TimelineEntry represents a single entry in the unified timeline.
type TimelineEntry struct {
	Resource Resource
	Revision Revision
}

// CompareSelection holds the two items being compared.
type CompareSelection struct {
	Left  *CompareItem
	Right *CompareItem
}

// CompareItem is one side of a comparison.
type CompareItem struct {
	Resource Resource
	Revision Revision
}

// Data holds a resource and all its revisions.
type Data struct {
	Resource  Resource
	Revisions []Revision // sorted oldest-first (index 0 = oldest)
}

// LatestRevision returns the most recent revision, or nil if empty.
func (rd *Data) LatestRevision() *Revision {
	if len(rd.Revisions) == 0 {
		return nil
	}
	return &rd.Revisions[len(rd.Revisions)-1]
}

// CreationTime returns the Kubernetes creationTimestamp from the first
// revision's object metadata. Returns zero time if unavailable.
func (rd *Data) CreationTime() time.Time {
	if len(rd.Revisions) == 0 {
		return time.Time{}
	}
	obj := rd.Revisions[0].Object
	if obj == nil {
		return time.Time{}
	}
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return time.Time{}
	}
	ts, ok := meta["creationTimestamp"].(string)
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (rd *Data) RevisionCount() int {
	return len(rd.Revisions)
}

// ChangeFrequency returns changes per minute over the resource's lifetime.
func (rd *Data) ChangeFrequency() float64 {
	if len(rd.Revisions) < 2 {
		return 0
	}
	first := rd.Revisions[0].Time
	last := rd.Revisions[len(rd.Revisions)-1].Time
	duration := last.Sub(first)
	if duration <= 0 {
		return 0
	}
	return float64(len(rd.Revisions)-1) / duration.Minutes()
}

// KindGroup represents a collapsible group in the resource tree.
type KindGroup struct {
	Kind      string
	Resources []*Data
	Expanded  bool
}

// Kind represents a Kubernetes resource type (CRD or built-in)
// available on the cluster.
type Kind struct {
	Kind       string // e.g., "Pod", "Deployment", "Secret"
	APIVersion string // e.g., "v1", "apps/v1"
	Resource   string // plural resource name, e.g., "pods", "deployments"
	Namespaced bool   // whether instances live in a namespace
}

// String returns the Kind name (used for display and fuzzy matching).
func (rk Kind) String() string {
	return rk.Kind
}

// GVR returns the "group/version/resource" string, e.g. "apps/v1/deployments" or "v1/pods".
func (rk Kind) GVR() string {
	return rk.APIVersion + "/" + rk.Resource
}

// BurstGroup represents a group of timeline entries that occurred within
// a short window, likely from a single operator reconciliation cycle.
type BurstGroup struct {
	Entries []TimelineEntry
}

// RelativeTime formats a time as a human-readable relative string (e.g., "5m", "2h").
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		return "future"
	}
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// FormatTimestamp returns a short timestamp like "14:32:05".
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}

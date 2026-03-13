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

// Resource represents a tracked Kubernetes resource instance.
type Resource struct {
	UID       string
	Kind      string
	Name      string
	Namespace string
	Starred   bool
}

// QualifiedName returns "namespace/name" or just "name" for cluster-scoped resources.
func (r Resource) QualifiedName() string {
	if r.Namespace != "" {
		return r.Namespace + "/" + r.Name
	}
	return r.Name
}

// KindName returns "Kind/name" (e.g., "Pod/nginx-abc").
func (r Resource) KindName() string {
	return r.Kind + "/" + r.Name
}

// ShortName returns a truncated name for narrow panels.
func (r Resource) ShortName(maxLen int) string {
	if maxLen <= 3 {
		return r.Name
	}
	if len(r.Name) > maxLen {
		return r.Name[:maxLen-1] + "…"
	}
	return r.Name
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

// ResourceData holds a resource and all its revisions.
type ResourceData struct {
	Resource  Resource
	Revisions []Revision // sorted oldest-first (index 0 = oldest)
}

// LatestRevision returns the most recent revision, or nil if empty.
func (rd *ResourceData) LatestRevision() *Revision {
	if len(rd.Revisions) == 0 {
		return nil
	}
	return &rd.Revisions[len(rd.Revisions)-1]
}

// RevisionCount returns the number of revisions.
func (rd *ResourceData) RevisionCount() int {
	return len(rd.Revisions)
}

// ChangeFrequency returns changes per minute over the resource's lifetime.
func (rd *ResourceData) ChangeFrequency() float64 {
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
	Resources []*ResourceData
	Expanded  bool
}

// TotalRevisions returns the total revision count across all resources in the group.
func (kg *KindGroup) TotalRevisions() int {
	total := 0
	for _, rd := range kg.Resources {
		total += len(rd.Revisions)
	}
	return total
}

// ResourceKind represents a Kubernetes resource type (CRD or built-in)
// available on the cluster.
type ResourceKind struct {
	Kind       string // e.g., "Pod", "Deployment", "Secret"
	APIVersion string // e.g., "v1", "apps/v1"
	Namespaced bool   // whether instances live in a namespace
}

// String returns the Kind name (used for display and fuzzy matching).
func (rk ResourceKind) String() string {
	return rk.Kind
}

// BurstGroup represents a group of timeline entries that occurred within
// a short window, likely from a single operator reconciliation cycle.
type BurstGroup struct {
	Entries []TimelineEntry
}

// KindIcon returns a Unicode icon for common Kubernetes resource kinds.
func KindIcon(kind string) string {
	switch kind {
	case "Pod":
		return "◎"
	case "Deployment":
		return "◈"
	case "ReplicaSet":
		return "◇"
	case "StatefulSet":
		return "◆"
	case "DaemonSet":
		return "◉"
	case "Service":
		return "◎"
	case "Ingress":
		return "⇋"
	case "ConfigMap":
		return "☰"
	case "Secret":
		return "⚿"
	case "Namespace":
		return "▣"
	case "Node":
		return "⬡"
	case "PersistentVolumeClaim", "PersistentVolume":
		return "▤"
	case "Job":
		return "⚙"
	case "CronJob":
		return "⏱"
	case "ServiceAccount":
		return "⊕"
	case "Role", "ClusterRole":
		return "⛊"
	case "RoleBinding", "ClusterRoleBinding":
		return "⛓"
	case "NetworkPolicy":
		return "⊞"
	case "HorizontalPodAutoscaler":
		return "⇕"
	case "Endpoints":
		return "⊙"
	default:
		return "□"
	}
}

// RelativeTime formats a time as a human-readable relative string (e.g., "5m", "2h").
func RelativeTime(t time.Time) string {
	d := time.Since(t)
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

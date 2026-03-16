package tui

import tea "github.com/charmbracelet/bubbletea"

// Store is the data source interface consumed by the TUI.
// Both the simulation package and the production adapter (adapter.LiveStore) implement this.
type Store interface {

	// AllResources returns all tracked resources, sorted by kind then name.
	AllResources() []*ResourceData

	// StarredResources returns only starred resources.
	StarredResources() []*ResourceData

	// GetResource returns a single resource by UID, or nil if not found.
	GetResource(uid string) *ResourceData

	// TotalResourceCount returns the number of tracked resources.
	TotalResourceCount() int

	// TotalRevisionCount returns the total number of revisions across all resources.
	TotalRevisionCount() int

	// FilterResources returns resources matching a text filter expression.
	FilterResources(expr string) []*ResourceData

	// FilterTimeline returns timeline entries matching a text filter and/or starred-only flag.
	FilterTimeline(expr string, starredOnly bool) []TimelineEntry

	// Timeline returns all timeline entries (newest first).
	Timeline() []TimelineEntry

	// KindGroups returns the current kind groups for tree display.
	KindGroups() []*KindGroup

	// WatchedKinds returns a sorted list of kind names currently being watched.
	WatchedKinds() []string

	// ResourceCountByKind returns the number of tracked resources of a given kind.
	ResourceCountByKind(kind string) int

	// RevisionCountByKind returns the total revision count across all resources of a kind.
	RevisionCountByKind(kind string) int

	// UnwatchedKinds returns resource types available on the cluster that aren't currently watched.
	UnwatchedKinds() []ResourceKind

	// AddWatchKind starts watching a resource type, returning any newly created resources.
	AddWatchKind(rk ResourceKind) []*ResourceData

	// RemoveWatchKind removes all resources of a given kind from active watching.
	RemoveWatchKind(kind string)

	// ToggleStar toggles the starred flag on a resource, returning whether it is now starred.
	ToggleStar(uid string) bool

	// AddRevision adds a new revision for a resource and updates the timeline.
	AddRevision(resourceUID string, rev Revision)

	// RebuildKindGroups rebuilds the kind groups (call after any mutation that affects the tree).
	RebuildKindGroups()

	// ForEachResource calls fn for each resource. Iteration order is undefined.
	ForEachResource(fn func(uid string, rd *ResourceData))
}

// Simulator is an optional interface for generating live data in the TUI.
// The simulation package implements this; production stores do not.
// Pass a Simulator to NewApp via WithSimulator to enable live data generation.
type Simulator interface {
	// ScheduleNextTick returns a tea.Cmd that, after a delay, sends a
	// SimulationTickMsg for a random resource. Returns nil if no resources exist.
	ScheduleNextTick() tea.Cmd

	// GenerateRevision creates a new simulated revision for the given resource.
	GenerateRevision(rd *ResourceData) Revision
}

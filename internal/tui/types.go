package tui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
)

// ViewID identifies which top-level view is active.
type ViewID int

const (
	ExplorerView ViewID = iota
	TimelineView
	WatchlistView
	CompareView
)

func (v ViewID) String() string {
	switch v {
	case ExplorerView:
		return "Explorer"
	case TimelineView:
		return "Timeline"
	case WatchlistView:
		return "Watchlist"
	case CompareView:
		return "Compare"
	default:
		return "Unknown"
	}
}

// AllViews returns all available view IDs in tab order.
func AllViews() []ViewID {
	return []ViewID{ExplorerView, TimelineView, WatchlistView, CompareView}
}

// ViewMode determines how the detail panel renders content.
type ViewMode int

const (
	DiffMode   ViewMode = iota // Full object YAML with diff highlighting
	ObjectMode                 // Full YAML, no diff annotations
	PatchMode                  // Only changed fields
	JSONMode                   // Raw JSON
)

func (m ViewMode) String() string {
	switch m {
	case DiffMode:
		return "Diff"
	case ObjectMode:
		return "Object"
	case PatchMode:
		return "Patch"
	case JSONMode:
		return "JSON"
	default:
		return "?"
	}
}

// PanelID identifies which panel has focus within a view.
type PanelID int

const (
	PanelLeft PanelID = iota
	PanelMiddle
	PanelRight
)

// EventType represents the type of Kubernetes watch event.
type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
)

// Resource represents a tracked Kubernetes resource.
type Resource struct {
	UID       string
	Kind      string
	Name      string
	Namespace string
	Starred   bool
}

// QualifiedName returns "namespace/name" or just "name".
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

// RevisionID is a 64-bit identifier displayed as hex.
type RevisionID uint64

func (id RevisionID) String() string {
	return fmt.Sprintf("%04x", uint64(id))
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
	Revisions []Revision // sorted oldest-first (index 0 = first)
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

// DetectLoop checks if the resource is oscillating between states.
// It returns true if among the last N revisions, the same object state
// appears more than once (indicating a reconcile loop).
func (rd *ResourceData) DetectLoop(windowSize int) bool {
	revs := rd.Revisions
	if len(revs) < 3 {
		return false
	}
	start := len(revs) - windowSize
	if start < 0 {
		start = 0
	}
	window := revs[start:]

	// Serialize each object to JSON for comparison
	seen := make(map[string]int)
	for _, rev := range window {
		if rev.Object == nil {
			continue
		}
		b, err := json.Marshal(rev.Object)
		if err != nil {
			continue
		}
		key := string(b)
		seen[key]++
		if seen[key] >= 2 {
			return true // same state appeared twice in the window
		}
	}
	return false
}

// LoopInfo returns detailed loop information.
type LoopInfo struct {
	IsLoop           bool
	OscillationCount int // how many times it flipped
	FirstSeen        time.Time
	Period           time.Duration // avg time between oscillations
}

// AnalyzeLoop performs detailed loop analysis on recent revisions.
func (rd *ResourceData) AnalyzeLoop(windowSize int) LoopInfo {
	revs := rd.Revisions
	if len(revs) < 4 {
		return LoopInfo{}
	}
	start := len(revs) - windowSize
	if start < 0 {
		start = 0
	}
	window := revs[start:]

	// Track state fingerprints
	type stateOccurrence struct {
		count int
		times []time.Time
	}
	seen := make(map[string]*stateOccurrence)
	for _, rev := range window {
		if rev.Object == nil {
			continue
		}
		b, _ := json.Marshal(rev.Object)
		key := string(b)
		if _, ok := seen[key]; !ok {
			seen[key] = &stateOccurrence{}
		}
		seen[key].count++
		seen[key].times = append(seen[key].times, rev.Time)
	}

	// Find the most repeated state
	maxCount := 0
	var maxTimes []time.Time
	for _, occ := range seen {
		if occ.count > maxCount {
			maxCount = occ.count
			maxTimes = occ.times
		}
	}

	if maxCount < 2 {
		return LoopInfo{}
	}

	// Calculate average period
	var totalDuration time.Duration
	for i := 1; i < len(maxTimes); i++ {
		totalDuration += maxTimes[i].Sub(maxTimes[i-1])
	}
	avgPeriod := totalDuration / time.Duration(len(maxTimes)-1)

	return LoopInfo{
		IsLoop:           true,
		OscillationCount: maxCount,
		FirstSeen:        maxTimes[0],
		Period:           avgPeriod,
	}
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

// ResourceKind represents a Kubernetes resource type (CRD or built-in) available in the cluster.
type ResourceKind struct {
	Kind       string // e.g., "Pod", "Deployment", "Secret"
	APIVersion string // e.g., "v1", "apps/v1"
	Namespaced bool   // whether instances live in a namespace
}

// String returns the Kind name (used for display and fuzzy matching).
func (rk ResourceKind) String() string {
	return rk.Kind
}

// DummyStore holds all the dummy data needed by the prototype.
type DummyStore struct {
	Resources            map[string]*ResourceData // keyed by UID — currently watched
	Timeline             []TimelineEntry          // sorted newest-first
	KindGroups           []*KindGroup             // organized by kind for the tree view
	ClusterResourceTypes []ResourceKind           // resource types available on the cluster
}

// AllResources returns a flat list of all ResourceData.
func (ds *DummyStore) AllResources() []*ResourceData {
	result := make([]*ResourceData, 0, len(ds.Resources))
	for _, rd := range ds.Resources {
		result = append(result, rd)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resource.Kind != result[j].Resource.Kind {
			return result[i].Resource.Kind < result[j].Resource.Kind
		}
		return result[i].Resource.Name < result[j].Resource.Name
	})
	return result
}

// StarredResources returns only starred resources.
func (ds *DummyStore) StarredResources() []*ResourceData {
	var result []*ResourceData
	for _, rd := range ds.Resources {
		if rd.Resource.Starred {
			result = append(result, rd)
		}
	}
	return result
}

// TotalResourceCount returns the number of resources.
func (ds *DummyStore) TotalResourceCount() int { return len(ds.Resources) }

// TotalRevisionCount returns the total number of revisions across all resources.
func (ds *DummyStore) TotalRevisionCount() int {
	total := 0
	for _, rd := range ds.Resources {
		total += len(rd.Revisions)
	}
	return total
}

// FilterResources returns resources matching a simple text filter.
// For the prototype, this does case-insensitive substring matching on
// kind, name, and namespace. In production, this would use expr-lang.
func (ds *DummyStore) FilterResources(expr string) []*ResourceData {
	if expr == "" {
		return ds.AllResources()
	}
	expr = strings.ToLower(expr)
	var result []*ResourceData
	for _, rd := range ds.Resources {
		r := rd.Resource
		searchable := strings.ToLower(r.Kind + " " + r.Name + " " + r.Namespace)
		if strings.Contains(searchable, expr) {
			result = append(result, rd)
		}
	}
	return result
}

// WatchedKinds returns a deduplicated sorted list of kind names currently being watched.
func (ds *DummyStore) WatchedKinds() []string {
	kindSet := make(map[string]bool)
	for _, rd := range ds.Resources {
		kindSet[rd.Resource.Kind] = true
	}
	kinds := make([]string, 0, len(kindSet))
	for k := range kindSet {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

// ResourceCountByKind returns the number of tracked resources of a given kind.
func (ds *DummyStore) ResourceCountByKind(kind string) int {
	count := 0
	for _, rd := range ds.Resources {
		if rd.Resource.Kind == kind {
			count++
		}
	}
	return count
}

// RevisionCountByKind returns the total number of revisions across all resources of a given kind.
func (ds *DummyStore) RevisionCountByKind(kind string) int {
	count := 0
	for _, rd := range ds.Resources {
		if rd.Resource.Kind == kind {
			count += len(rd.Revisions)
		}
	}
	return count
}

// UnwatchedKinds returns resource types from ClusterResourceTypes that are NOT currently watched.
func (ds *DummyStore) UnwatchedKinds() []ResourceKind {
	watched := ds.WatchedKinds()
	watchedSet := make(map[string]bool, len(watched))
	for _, k := range watched {
		watchedSet[k] = true
	}
	var result []ResourceKind
	for _, rk := range ds.ClusterResourceTypes {
		if !watchedSet[rk.Kind] {
			result = append(result, rk)
		}
	}
	return result
}

// AddWatchKind starts watching a resource type by generating dummy resources of that kind.
// Returns the newly created ResourceData entries.
func (ds *DummyStore) AddWatchKind(rk ResourceKind) []*ResourceData {
	now := time.Now()
	var created []*ResourceData

	// Generate some dummy resources for this kind
	dummyResources := generateDummyResourcesForKind(rk, now)

	for _, r := range dummyResources {
		initRev := Revision{
			ID:        RevisionID(uint64(now.UnixNano()&0xFFFF) + uint64(len(ds.Resources))),
			EventType: EventAdded,
			Time:      now.Add(-time.Duration(len(created)) * 500 * time.Millisecond), // stagger slightly
			Object: map[string]any{
				"apiVersion": rk.APIVersion,
				"kind":       rk.Kind,
				"metadata":   r.meta,
				"spec":       r.spec,
				"status":     r.status,
			},
		}

		rd := &ResourceData{
			Resource: Resource{
				UID:       r.uid,
				Kind:      rk.Kind,
				Name:      r.name,
				Namespace: r.namespace,
			},
			Revisions: []Revision{initRev},
		}
		ds.Resources[r.uid] = rd
		created = append(created, rd)

		// Add to timeline
		ds.Timeline = append([]TimelineEntry{{
			Resource: rd.Resource,
			Revision: initRev,
		}}, ds.Timeline...)
	}

	// Re-sort timeline
	sort.Slice(ds.Timeline, func(i, j int) bool {
		return ds.Timeline[i].Revision.Time.After(ds.Timeline[j].Revision.Time)
	})

	return created
}

// RemoveWatchKind removes ALL resources of a given kind from active watching.
func (ds *DummyStore) RemoveWatchKind(kind string) {
	// Collect UIDs to remove
	var toRemove []string
	for uid, rd := range ds.Resources {
		if rd.Resource.Kind == kind {
			toRemove = append(toRemove, uid)
		}
	}

	// Remove from resources
	for _, uid := range toRemove {
		delete(ds.Resources, uid)
	}

	// Remove from timeline
	removeSet := make(map[string]bool, len(toRemove))
	for _, uid := range toRemove {
		removeSet[uid] = true
	}
	var filtered []TimelineEntry
	for _, e := range ds.Timeline {
		if !removeSet[e.Resource.UID] {
			filtered = append(filtered, e)
		}
	}
	ds.Timeline = filtered
}

// dummyResourceSpec is used internally to generate dummy resources for a new kind.
type dummyResourceSpec struct {
	uid       string
	name      string
	namespace string
	meta      map[string]any
	spec      map[string]any
	status    map[string]any
}

// generateDummyResourcesForKind creates realistic dummy resources for a given resource type.
func generateDummyResourcesForKind(rk ResourceKind, now time.Time) []dummyResourceSpec {
	ns := "default"
	if !rk.Namespaced {
		ns = ""
	}
	uid := func(suffix string) string {
		return "watched-" + strings.ToLower(rk.Kind) + "-" + suffix
	}
	meta := func(name, namespace string) map[string]any {
		m := map[string]any{
			"name":              name,
			"creationTimestamp": now.Add(-1 * time.Hour).Format(time.RFC3339),
		}
		if namespace != "" {
			m["namespace"] = namespace
		}
		return m
	}

	switch rk.Kind {
	case "Secret":
		return []dummyResourceSpec{
			{uid: uid("tls-cert"), name: "tls-cert", namespace: ns,
				meta: meta("tls-cert", ns), spec: map[string]any{"type": "kubernetes.io/tls"},
				status: map[string]any{}},
			{uid: uid("db-creds"), name: "db-credentials", namespace: ns,
				meta: meta("db-credentials", ns), spec: map[string]any{"type": "Opaque"},
				status: map[string]any{}},
		}
	case "Ingress":
		return []dummyResourceSpec{
			{uid: uid("api-gw"), name: "api-gateway", namespace: ns,
				meta:   meta("api-gateway", ns),
				spec:   map[string]any{"rules": []any{map[string]any{"host": "api.example.com", "http": map[string]any{"paths": []any{map[string]any{"path": "/", "backend": map[string]any{"service": map[string]any{"name": "api-svc", "port": float64(8080)}}}}}}}},
				status: map[string]any{"loadBalancer": map[string]any{}}},
			{uid: uid("web-fe"), name: "web-frontend", namespace: ns,
				meta:   meta("web-frontend", ns),
				spec:   map[string]any{"rules": []any{map[string]any{"host": "www.example.com"}}},
				status: map[string]any{"loadBalancer": map[string]any{}}},
		}
	case "CronJob":
		return []dummyResourceSpec{
			{uid: uid("db-backup"), name: "db-backup", namespace: ns,
				meta:   meta("db-backup", ns),
				spec:   map[string]any{"schedule": "0 2 * * *", "jobTemplate": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "backup", "image": "postgres:16"}}}}}}},
				status: map[string]any{"lastScheduleTime": now.Add(-2 * time.Hour).Format(time.RFC3339)}},
			{uid: uid("log-cleanup"), name: "log-cleanup", namespace: "kube-system",
				meta:   meta("log-cleanup", "kube-system"),
				spec:   map[string]any{"schedule": "0 */6 * * *", "jobTemplate": map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "cleaner", "image": "busybox:latest"}}}}}}},
				status: map[string]any{}},
		}
	case "HPA":
		return []dummyResourceSpec{
			{uid: uid("nginx-as"), name: "nginx-autoscaler", namespace: ns,
				meta:   meta("nginx-autoscaler", ns),
				spec:   map[string]any{"scaleTargetRef": map[string]any{"kind": "Deployment", "name": "nginx-deployment"}, "minReplicas": float64(1), "maxReplicas": float64(10), "targetCPUUtilizationPercentage": float64(50)},
				status: map[string]any{"currentReplicas": float64(3), "desiredReplicas": float64(3)}},
		}
	case "PodDisruptionBudget":
		return []dummyResourceSpec{
			{uid: uid("nginx-pdb"), name: "nginx-pdb", namespace: ns,
				meta:   meta("nginx-pdb", ns),
				spec:   map[string]any{"minAvailable": float64(1), "selector": map[string]any{"matchLabels": map[string]any{"app": "nginx"}}},
				status: map[string]any{"currentHealthy": float64(3), "desiredHealthy": float64(1)}},
		}
	case "Namespace":
		return []dummyResourceSpec{
			{uid: uid("monitoring"), name: "monitoring", namespace: "",
				meta: meta("monitoring", ""), spec: map[string]any{},
				status: map[string]any{"phase": "Active"}},
			{uid: uid("staging"), name: "staging", namespace: "",
				meta: meta("staging", ""), spec: map[string]any{},
				status: map[string]any{"phase": "Active"}},
		}
	case "ServiceAccount":
		return []dummyResourceSpec{
			{uid: uid("deploy-bot"), name: "deploy-bot", namespace: ns,
				meta:   meta("deploy-bot", ns),
				spec:   map[string]any{"automountServiceAccountToken": true},
				status: map[string]any{}},
		}
	case "ClusterRole":
		return []dummyResourceSpec{
			{uid: uid("custom-admin"), name: "custom-admin", namespace: "",
				meta:   meta("custom-admin", ""),
				spec:   map[string]any{"rules": []any{map[string]any{"apiGroups": []any{"*"}, "resources": []any{"*"}, "verbs": []any{"*"}}}},
				status: map[string]any{}},
		}
	case "Endpoints":
		return []dummyResourceSpec{
			{uid: uid("nginx-ep"), name: "nginx-svc", namespace: ns,
				meta:   meta("nginx-svc", ns),
				spec:   map[string]any{"subsets": []any{map[string]any{"addresses": []any{map[string]any{"ip": "10.0.1.15"}, map[string]any{"ip": "10.0.1.16"}}, "ports": []any{map[string]any{"port": float64(80)}}}}},
				status: map[string]any{}},
		}
	case "PersistentVolume":
		return []dummyResourceSpec{
			{uid: uid("data-pv"), name: "data-pv-01", namespace: "",
				meta:   meta("data-pv-01", ""),
				spec:   map[string]any{"capacity": map[string]any{"storage": "100Gi"}, "accessModes": []any{"ReadWriteOnce"}, "storageClassName": "standard"},
				status: map[string]any{"phase": "Bound"}},
		}
	case "PersistentVolumeClaim":
		return []dummyResourceSpec{
			{uid: uid("data-pvc"), name: "data-pvc", namespace: ns,
				meta:   meta("data-pvc", ns),
				spec:   map[string]any{"accessModes": []any{"ReadWriteOnce"}, "resources": map[string]any{"requests": map[string]any{"storage": "50Gi"}}},
				status: map[string]any{"phase": "Bound", "capacity": map[string]any{"storage": "50Gi"}}},
		}
	case "NetworkPolicy":
		return []dummyResourceSpec{
			{uid: uid("default-deny"), name: "default-deny", namespace: ns,
				meta:   meta("default-deny", ns),
				spec:   map[string]any{"podSelector": map[string]any{}, "policyTypes": []any{"Ingress", "Egress"}},
				status: map[string]any{}},
		}
	case "Job":
		return []dummyResourceSpec{
			{uid: uid("db-migrate"), name: "db-migrate-v2", namespace: ns,
				meta:   meta("db-migrate-v2", ns),
				spec:   map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"name": "migrate", "image": "myapp/migrate:v2"}}, "restartPolicy": "Never"}}},
				status: map[string]any{"succeeded": float64(1), "completionTime": now.Add(-30 * time.Minute).Format(time.RFC3339)}},
		}
	default:
		// Generic fallback for unknown kinds
		return []dummyResourceSpec{
			{uid: uid("instance-1"), name: rk.Kind + "-instance-1", namespace: ns,
				meta:   meta(rk.Kind+"-instance-1", ns),
				spec:   map[string]any{"placeholder": "(auto-generated)"},
				status: map[string]any{"phase": "Active"}},
		}
	}
}

// BuildKindGroups organizes resources into kind groups for the tree view.
func BuildKindGroups(resources []*ResourceData) []*KindGroup {
	kindMap := make(map[string]*KindGroup)
	for _, rd := range resources {
		k := rd.Resource.Kind
		if _, ok := kindMap[k]; !ok {
			kindMap[k] = &KindGroup{Kind: k, Expanded: true}
		}
		kindMap[k].Resources = append(kindMap[k].Resources, rd)
	}

	// Sort kinds in a preferred order
	kindOrder := map[string]int{
		"Pod": 0, "Deployment": 1, "ReplicaSet": 2, "StatefulSet": 3,
		"Service": 4, "Ingress": 5, "ConfigMap": 6, "Secret": 7,
		"MyApp": 8,
	}
	groups := make([]*KindGroup, 0, len(kindMap))
	for _, g := range kindMap {
		sort.Slice(g.Resources, func(i, j int) bool {
			return g.Resources[i].Resource.Name < g.Resources[j].Resource.Name
		})
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		oi, ok1 := kindOrder[groups[i].Kind]
		oj, ok2 := kindOrder[groups[j].Kind]
		if ok1 && ok2 {
			return oi < oj
		}
		if ok1 {
			return true
		}
		if ok2 {
			return false
		}
		return groups[i].Kind < groups[j].Kind
	})
	return groups
}

// BurstGroup represents a group of timeline entries that happened within
// a short window, likely from a single operator reconciliation cycle.
type BurstGroup struct {
	Entries []TimelineEntry
}

// GroupTimelineByBurst groups timeline entries into bursts.
// Entries within `window` duration of each other are grouped together.
func GroupTimelineByBurst(entries []TimelineEntry, window time.Duration) []interface{} {
	if len(entries) == 0 {
		return nil
	}

	var result []interface{} // either TimelineEntry or BurstGroup
	var currentBurst []TimelineEntry
	currentBurst = append(currentBurst, entries[0])

	for i := 1; i < len(entries); i++ {
		prev := currentBurst[len(currentBurst)-1]
		curr := entries[i]

		// Timeline is newest-first, so prev.Time >= curr.Time
		gap := prev.Revision.Time.Sub(curr.Revision.Time)
		if gap < 0 {
			gap = -gap
		}

		if gap <= window {
			currentBurst = append(currentBurst, curr)
		} else {
			// Flush current burst
			if len(currentBurst) >= 2 {
				result = append(result, BurstGroup{Entries: currentBurst})
			} else {
				result = append(result, currentBurst[0])
			}
			currentBurst = []TimelineEntry{curr}
		}
	}

	// Flush remaining
	if len(currentBurst) >= 2 {
		result = append(result, BurstGroup{Entries: currentBurst})
	} else if len(currentBurst) == 1 {
		result = append(result, currentBurst[0])
	}

	return result
}

// RelativeTime formats a time as a human-readable relative string.
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		s := int(d.Seconds())
		return fmt.Sprintf("%ds", s)
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
}

// FormatTimestamp returns a short timestamp like "14:32:05".
func FormatTimestamp(t time.Time) string {
	return t.Format("15:04:05")
}

// RenderYAMLObject renders a map as simple YAML text with syntax highlighting.
func RenderYAMLObject(obj map[string]any, theme Theme, indent int) string {
	if obj == nil {
		return theme.MutedStyle().Render("(empty)")
	}
	var sb strings.Builder
	renderYAMLMap(&sb, obj, theme, indent, 0)
	return sb.String()
}

func renderYAMLMap(sb *strings.Builder, m map[string]any, theme Theme, indentSize, depth int) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	indent := strings.Repeat(" ", depth*indentSize)
	for _, k := range keys {
		v := m[k]
		keyStr := theme.SyntaxKeyStyle().Render(k) + ":"
		switch val := v.(type) {
		case map[string]any:
			sb.WriteString(indent + keyStr + "\n")
			renderYAMLMap(sb, val, theme, indentSize, depth+1)
		case []any:
			sb.WriteString(indent + keyStr + "\n")
			renderYAMLList(sb, val, theme, indentSize, depth+1)
		case string:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
		case bool:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxBoolStyle().Render(fmt.Sprintf("%v", val)) + "\n")
		case int, int64, float64:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxNumberStyle().Render(fmt.Sprintf("%v", val)) + "\n")
		case nil:
			sb.WriteString(indent + keyStr + " " + theme.SyntaxNullStyle().Render("null") + "\n")
		default:
			sb.WriteString(indent + keyStr + " " + fmt.Sprintf("%v", val) + "\n")
		}
	}
}

func renderYAMLList(sb *strings.Builder, list []any, theme Theme, indentSize, depth int) {
	indent := strings.Repeat(" ", depth*indentSize)
	for _, item := range list {
		switch val := item.(type) {
		case map[string]any:
			// YAML convention: first key on same line as "- "
			keys := make([]string, 0, len(val))
			for k := range val {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				firstKey := keys[0]
				firstVal := val[firstKey]
				firstKeyStr := theme.SyntaxKeyStyle().Render(firstKey) + ":"
				sb.WriteString(indent + "- " + firstKeyStr)
				writeYAMLValue(sb, firstVal, theme, indentSize, depth+1)
				// Remaining keys at depth+1
				for _, k := range keys[1:] {
					v := val[k]
					subIndent := strings.Repeat(" ", (depth+1)*indentSize)
					keyStr := theme.SyntaxKeyStyle().Render(k) + ":"
					sb.WriteString(subIndent + keyStr)
					writeYAMLValue(sb, v, theme, indentSize, depth+1)
				}
			} else {
				sb.WriteString(indent + "- {}\n")
			}
		case string:
			sb.WriteString(indent + "- " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
		default:
			sb.WriteString(indent + "- " + fmt.Sprintf("%v", item) + "\n")
		}
	}
}

// writeYAMLValue writes a value for a key-value pair (after the key: prefix).
func writeYAMLValue(sb *strings.Builder, v any, theme Theme, indentSize, depth int) {
	switch val := v.(type) {
	case map[string]any:
		sb.WriteString("\n")
		renderYAMLMap(sb, val, theme, indentSize, depth+1)
	case []any:
		sb.WriteString("\n")
		renderYAMLList(sb, val, theme, indentSize, depth+1)
	case string:
		sb.WriteString(" " + theme.SyntaxStringStyle().Render("\""+val+"\"") + "\n")
	case bool:
		sb.WriteString(" " + theme.SyntaxBoolStyle().Render(fmt.Sprintf("%v", val)) + "\n")
	case int, int64, float64:
		sb.WriteString(" " + theme.SyntaxNumberStyle().Render(fmt.Sprintf("%v", val)) + "\n")
	case nil:
		sb.WriteString(" " + theme.SyntaxNullStyle().Render("null") + "\n")
	default:
		sb.WriteString(" " + fmt.Sprintf("%v", val) + "\n")
	}
}

// RenderJSONObject renders a map as pretty-printed JSON.
func RenderJSONObject(obj map[string]any, theme Theme) string {
	if obj == nil {
		return theme.MutedStyle().Render("(empty)")
	}
	b, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return theme.ErrorStyle().Render("(json error: " + err.Error() + ")")
	}
	return string(b)
}

// DeepEqual compares two maps for equality.
func DeepEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(a, b)
}

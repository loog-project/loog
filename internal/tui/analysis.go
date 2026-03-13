package tui

import (
	"fmt"
	"math/rand"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Change Tags ───

// ChangeTag classifies what kind of change a revision represents.
type ChangeTag string

const (
	TagSpec     ChangeTag = "spec"
	TagStatus   ChangeTag = "status"
	TagImage    ChangeTag = "image"
	TagLabels   ChangeTag = "labels"
	TagConfig   ChangeTag = "config"
	TagReplicas ChangeTag = "replicas"
	TagUnknown  ChangeTag = "unknown"
)

// TagRevision classifies a revision by comparing it to the previous one.
// Returns one or more tags describing what changed.
func TagRevision(prev, curr map[string]any) []ChangeTag {
	if prev == nil || curr == nil {
		return []ChangeTag{TagUnknown}
	}

	var tags []ChangeTag

	// Check spec changes
	prevSpec, _ := prev["spec"].(map[string]any)
	currSpec, _ := curr["spec"].(map[string]any)
	if prevSpec != nil && currSpec != nil {
		// Check image change
		if hasImageChange(prevSpec, currSpec) {
			tags = append(tags, TagImage)
		}
		// Check replicas change
		if hasReplicasChange(prevSpec, currSpec) {
			tags = append(tags, TagReplicas)
		}
		// Generic spec change if nothing specific detected
		if len(tags) == 0 && !DeepEqual(prevSpec, currSpec) {
			tags = append(tags, TagSpec)
		}
	}

	// Check status changes
	prevStatus, _ := prev["status"].(map[string]any)
	currStatus, _ := curr["status"].(map[string]any)
	if !DeepEqual(prevStatus, currStatus) {
		tags = append(tags, TagStatus)
	}

	// Check label changes
	prevMeta, _ := prev["metadata"].(map[string]any)
	currMeta, _ := curr["metadata"].(map[string]any)
	if prevMeta != nil && currMeta != nil {
		prevLabels, _ := prevMeta["labels"].(map[string]any)
		currLabels, _ := currMeta["labels"].(map[string]any)
		if !DeepEqual(prevLabels, currLabels) {
			tags = append(tags, TagLabels)
		}
		// Check annotations for config changes
		prevAnno, _ := prevMeta["annotations"].(map[string]any)
		currAnno, _ := currMeta["annotations"].(map[string]any)
		if !DeepEqual(prevAnno, currAnno) {
			tags = append(tags, TagConfig)
		}
	}

	// Check data field changes (ConfigMap/Secret)
	prevData, _ := prev["data"].(map[string]any)
	currData, _ := curr["data"].(map[string]any)
	if !DeepEqual(prevData, currData) {
		tags = append(tags, TagConfig)
	}

	if len(tags) == 0 {
		tags = append(tags, TagUnknown)
	}

	// Deduplicate
	seen := make(map[ChangeTag]bool)
	unique := tags[:0]
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}
	return unique
}

// hasImageChange checks if any container image changed between two spec maps.
func hasImageChange(prevSpec, currSpec map[string]any) bool {
	prevContainers := extractContainers(prevSpec)
	currContainers := extractContainers(currSpec)

	if len(prevContainers) != len(currContainers) {
		return true
	}
	for i := range prevContainers {
		prevImg, _ := prevContainers[i]["image"].(string)
		currImg, _ := currContainers[i]["image"].(string)
		if prevImg != currImg {
			return true
		}
	}
	return false
}

// hasReplicasChange checks if replicas field changed.
func hasReplicasChange(prevSpec, currSpec map[string]any) bool {
	prevR, ok1 := prevSpec["replicas"]
	currR, ok2 := currSpec["replicas"]
	if !ok1 && !ok2 {
		return false
	}
	if ok1 != ok2 {
		return true
	}
	return fmt.Sprintf("%v", prevR) != fmt.Sprintf("%v", currR)
}

// extractContainers pulls the containers array from a spec, handling
// both direct .containers and .template.spec.containers paths.
func extractContainers(spec map[string]any) []map[string]any {
	// Direct containers (Pod spec)
	if containers, ok := spec["containers"].([]any); ok {
		return toMapSlice(containers)
	}
	// Template containers (Deployment/StatefulSet spec)
	if tmpl, ok := spec["template"].(map[string]any); ok {
		if tmplSpec, ok := tmpl["spec"].(map[string]any); ok {
			if containers, ok := tmplSpec["containers"].([]any); ok {
				return toMapSlice(containers)
			}
		}
	}
	return nil
}

func toMapSlice(items []any) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		}
	}
	return result
}

// ─── Analysis Result ───

// AnalysisResult holds the result of background analysis for a resource.
type AnalysisResult struct {
	ResourceUID string
	Tags        map[RevisionID][]ChangeTag // revision ID -> tags
	LoopInfo    LoopInfo
}

// RunAnalysisCmd returns a tea.Cmd that performs background analysis
// on a ResourceData and returns the result as an AnalysisCompleteMsg.
// Simulates a 200ms delay to demonstrate async behavior.
func RunAnalysisCmd(rd *ResourceData) tea.Cmd {
	if rd == nil {
		return nil
	}
	// Capture values for the closure
	uid := rd.Resource.UID
	revisions := make([]Revision, len(rd.Revisions))
	copy(revisions, rd.Revisions)

	return func() tea.Msg {
		// Simulate analysis delay
		time.Sleep(200 * time.Millisecond)

		tags := make(map[RevisionID][]ChangeTag)
		for i, rev := range revisions {
			if i == 0 {
				tags[rev.ID] = []ChangeTag{TagUnknown}
				continue
			}
			prev := revisions[i-1]
			tags[rev.ID] = TagRevision(prev.Object, rev.Object)
		}

		loopInfo := (&ResourceData{Revisions: revisions}).AnalyzeLoop(8)

		return AnalysisCompleteMsg{
			Result: AnalysisResult{
				ResourceUID: uid,
				Tags:        tags,
				LoopInfo:    loopInfo,
			},
		}
	}
}

// ─── Simulation ───

// WindowMode represents a time window centered on a selected revision.
// When active, the timeline shows only entries within ±duration of the anchor timestamp.
type WindowMode int

const (
	WindowAll WindowMode = iota
	Window15s            // ±15s around anchor
	Window30s            // ±30s around anchor
	Window1m             // ±1m around anchor
	Window5m             // ±5m around anchor
)

func (w WindowMode) String() string {
	switch w {
	case Window15s:
		return "±15s"
	case Window30s:
		return "±30s"
	case Window1m:
		return "±1m"
	case Window5m:
		return "±5m"
	default:
		return "all"
	}
}

// NextWindowMode cycles: all -> ±15s -> ±30s -> ±1m -> ±5m -> all
func NextWindowMode(current WindowMode) WindowMode {
	switch current {
	case WindowAll:
		return Window15s
	case Window15s:
		return Window30s
	case Window30s:
		return Window1m
	case Window1m:
		return Window5m
	case Window5m:
		return WindowAll
	default:
		return WindowAll
	}
}

// WindowHalfDuration returns the half-span for a WindowMode.
// The full window is [anchor - half, anchor + half].
// Returns 0 for WindowAll (no filter).
func WindowHalfDuration(w WindowMode) time.Duration {
	switch w {
	case Window15s:
		return 15 * time.Second
	case Window30s:
		return 30 * time.Second
	case Window1m:
		return 1 * time.Minute
	case Window5m:
		return 5 * time.Minute
	default:
		return 0
	}
}

// SimulateNewRevisionCmd returns a tea.Cmd that generates a new revision
// for a random resource after a 3-5 second delay, simulating live data.
func SimulateNewRevisionCmd(store *DummyStore) tea.Cmd {
	if store == nil || len(store.Resources) == 0 {
		return nil
	}

	// Pick a random resource UID
	uids := make([]string, 0, len(store.Resources))
	for uid := range store.Resources {
		uids = append(uids, uid)
	}

	return func() tea.Msg {
		delay := time.Duration(3+rand.Intn(3)) * time.Second
		time.Sleep(delay)

		uid := uids[rand.Intn(len(uids))]
		return SimulationTickMsg{ResourceUID: uid}
	}
}

// GenerateSimulatedRevision creates a new MODIFIED revision for the given
// resource, simulating a "last-seen" annotation update.
func GenerateSimulatedRevision(rd *ResourceData) Revision {
	now := time.Now()
	latest := rd.LatestRevision()

	// Clone the latest object and update annotation
	var newObj map[string]any
	if latest != nil && latest.Object != nil {
		newObj = cloneMap(latest.Object)
	} else {
		newObj = map[string]any{
			"apiVersion": "v1",
			"kind":       rd.Resource.Kind,
			"metadata":   map[string]any{"name": rd.Resource.Name},
		}
	}

	// Update metadata.annotations.last-seen
	meta, _ := newObj["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		newObj["metadata"] = meta
	}
	annotations, _ := meta["annotations"].(map[string]any)
	if annotations == nil {
		annotations = map[string]any{}
		meta["annotations"] = annotations
	}
	annotations["loog.dev/last-seen"] = now.Format(time.RFC3339)

	// Generate new ID
	var newID RevisionID
	var prevID RevisionID
	if latest != nil {
		prevID = latest.ID
		newID = RevisionID(uint64(latest.ID) + 1)
	} else {
		newID = RevisionID(0xF000 + uint64(rand.Intn(0xFFF)))
	}

	return Revision{
		ID:         newID,
		PreviousID: prevID,
		EventType:  EventModified,
		Time:       now,
		Object:     newObj,
		Patch: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"loog.dev/last-seen": now.Format(time.RFC3339),
				},
			},
		},
	}
}

// cloneMap performs a shallow-ish clone of a map[string]any.
// It recursively clones nested maps, but shares non-map values.
func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[k] = cloneMap(val)
		case []any:
			cloned := make([]any, len(val))
			for i, item := range val {
				if sub, ok := item.(map[string]any); ok {
					cloned[i] = cloneMap(sub)
				} else {
					cloned[i] = item
				}
			}
			result[k] = cloned
		default:
			result[k] = v
		}
	}
	return result
}

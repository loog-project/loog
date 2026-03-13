package resource

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
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

// TagRevision classifies a revision by comparing two object states.
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
		if hasImageChange(prevSpec, currSpec) {
			tags = append(tags, TagImage)
		}
		if hasReplicasChange(prevSpec, currSpec) {
			tags = append(tags, TagReplicas)
		}
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
	seen := make(map[ChangeTag]bool, len(tags))
	unique := tags[:0]
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			unique = append(unique, t)
		}
	}
	return unique
}

// AnalysisResult holds the result of background analysis for a resource.
type AnalysisResult struct {
	ResourceUID string
	Tags        map[RevisionID][]ChangeTag
	LoopInfo    LoopInfo
}

// Analyze performs synchronous analysis on a ResourceData: tags each revision
// and detects reconcile loops. This is a pure computation with no side effects.
func Analyze(rd *ResourceData, loopWindowSize int) AnalysisResult {
	tags := make(map[RevisionID][]ChangeTag, len(rd.Revisions))
	for i, rev := range rd.Revisions {
		if i == 0 {
			tags[rev.ID] = []ChangeTag{TagUnknown}
			continue
		}
		tags[rev.ID] = TagRevision(rd.Revisions[i-1].Object, rev.Object)
	}
	return AnalysisResult{
		ResourceUID: rd.Resource.UID,
		Tags:        tags,
		LoopInfo:    rd.AnalyzeLoop(loopWindowSize),
	}
}

// ─── Loop Detection ───

// LoopInfo holds detailed information about a detected reconcile loop.
type LoopInfo struct {
	IsLoop bool

	// DistinctStates is the number of unique object states in the loop (e.g., 2 for A↔B).
	DistinctStates int
	// Cycles counts complete oscillation cycles. For A→B→A→B→A, there are 2 full cycles.
	Cycles int
	// Period is the average duration of one full cycle.
	Period    time.Duration
	FirstSeen time.Time

	// LoopRevisions maps revision IDs to their state group label ("A", "B", ...).
	LoopRevisions map[RevisionID]string

	// PatternSample is a short pre-built sample of the oscillation pattern
	// from the tail of the window, e.g., "A→B→A→B". At most 6 labels.
	PatternSample string
}

// DetectLoop checks if the resource is oscillating between states.
// It returns true if among the last windowSize revisions, the same object
// state appears more than once (indicating a reconcile loop).
func (rd *ResourceData) DetectLoop(windowSize int) bool {
	revs := rd.Revisions
	if len(revs) < 3 {
		return false
	}
	start := len(revs) - windowSize
	if start < 0 {
		start = 0
	}

	seen := make(map[string]int)
	for _, rev := range revs[start:] {
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
			return true
		}
	}
	return false
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

	type stateOccurrence struct {
		count int
		times []time.Time
		revs  []RevisionID
	}
	seen := make(map[string]*stateOccurrence)
	var fingerOrder []string

	type revFP struct {
		revID RevisionID
		fp    string
	}
	var revFPs []revFP

	for _, rev := range window {
		if rev.Object == nil {
			continue
		}
		b, _ := json.Marshal(rev.Object)
		key := string(b)
		if _, ok := seen[key]; !ok {
			seen[key] = &stateOccurrence{}
			fingerOrder = append(fingerOrder, key)
		}
		seen[key].count++
		seen[key].times = append(seen[key].times, rev.Time)
		seen[key].revs = append(seen[key].revs, rev.ID)
		revFPs = append(revFPs, revFP{revID: rev.ID, fp: key})
	}

	// Build label map: only states appearing 2+ times are loop participants
	loopRevs := make(map[RevisionID]string)
	nextLabel := 'A'
	labelMap := make(map[string]string)
	distinctStates := 0
	for _, fp := range fingerOrder {
		occ := seen[fp]
		if occ.count >= 2 {
			label := string(nextLabel)
			labelMap[fp] = label
			distinctStates++
			if nextLabel < 'Z' {
				nextLabel++
			}
			for _, rid := range occ.revs {
				loopRevs[rid] = label
			}
		}
	}

	if distinctStates < 2 {
		return LoopInfo{}
	}

	// Count transitions between loop-participating states
	var prevLabel string
	transitions := 0
	for _, rfp := range revFPs {
		label, ok := labelMap[rfp.fp]
		if !ok {
			continue
		}
		if prevLabel != "" && label != prevLabel {
			transitions++
		}
		prevLabel = label
	}
	cycles := transitions / 2

	// Calculate cycle period from the most frequently occurring state
	var period time.Duration
	var firstSeen time.Time
	var bestTimes []time.Time
	bestCount := 0
	for fp, occ := range seen {
		if _, isLoop := labelMap[fp]; !isLoop {
			continue
		}
		if occ.count > bestCount {
			bestCount = occ.count
			bestTimes = occ.times
		}
	}
	if len(bestTimes) >= 2 {
		firstSeen = bestTimes[0]
		var totalGap time.Duration
		for i := 1; i < len(bestTimes); i++ {
			totalGap += bestTimes[i].Sub(bestTimes[i-1])
		}
		period = totalGap / time.Duration(len(bestTimes)-1)
	}

	// Build compact pattern sample (max 6 labels, deduplicated consecutive)
	var patternLabels []string
	for i := len(revFPs) - 1; i >= 0 && len(patternLabels) < 6; i-- {
		if label, ok := labelMap[revFPs[i].fp]; ok {
			patternLabels = append([]string{label}, patternLabels...)
		}
	}
	var deduped []string
	for _, l := range patternLabels {
		if len(deduped) == 0 || deduped[len(deduped)-1] != l {
			deduped = append(deduped, l)
		}
	}

	return LoopInfo{
		IsLoop:         true,
		DistinctStates: distinctStates,
		Cycles:         cycles,
		Period:         period,
		FirstSeen:      firstSeen,
		LoopRevisions:  loopRevs,
		PatternSample:  strings.Join(deduped, "→"),
	}
}

// ─── Window Mode ───

// WindowMode represents a time window centered on a selected revision.
type WindowMode int

const (
	WindowAll WindowMode = iota
	Window15s
	Window30s
	Window1m
	Window5m
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

// NextWindowMode cycles: all -> ±15s -> ±30s -> ±1m -> ±5m -> all.
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
	default:
		return WindowAll
	}
}

// WindowHalfDuration returns the half-span for a WindowMode.
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

// ─── Helpers ───

// DeepEqual compares two maps for equality.
func DeepEqual(a, b map[string]any) bool {
	return reflect.DeepEqual(a, b)
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

// hasReplicasChange checks if the replicas field changed.
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
	if containers, ok := spec["containers"].([]any); ok {
		return toMapSlice(containers)
	}
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

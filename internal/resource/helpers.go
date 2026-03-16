package resource

import (
	"sort"
	"strings"
	"time"
)

// BuildKindGroups organizes resources into kind groups for tree display.
// Groups are sorted in a preferred Kubernetes kind order, and resources
// within each group are sorted by name.
func BuildKindGroups(resources []*Data) []*KindGroup {
	kindMap := make(map[string]*KindGroup)
	for _, rd := range resources {
		k := rd.Resource.Kind
		if _, ok := kindMap[k]; !ok {
			kindMap[k] = &KindGroup{Kind: k, Expanded: true}
		}
		kindMap[k].Resources = append(kindMap[k].Resources, rd)
	}

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

// GroupTimelineByBurst groups timeline entries into bursts.
// Entries within the given window duration of each other are grouped together.
// Returns a slice of either TimelineEntry or BurstGroup values.
func GroupTimelineByBurst(entries []TimelineEntry, window time.Duration) []any {
	if len(entries) == 0 {
		return nil
	}

	var result []any
	currentBurst := []TimelineEntry{entries[0]}

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

// CloneMap performs a recursive clone of a map[string]any.
// It recursively clones nested maps and slices, but shares scalar values.
func CloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			result[k] = CloneMap(val)
		case []any:
			cloned := make([]any, len(val))
			for i, item := range val {
				if sub, ok := item.(map[string]any); ok {
					cloned[i] = CloneMap(sub)
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

// MatchesSubstring returns true if the query (already lowercased) appears as a
// case-insensitive substring in any of the resource's name, kind, namespace, or
// kind/name combination. Returns true for an empty query.
func MatchesSubstring(query string, r Resource) bool {
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(r.Name), query) ||
		strings.Contains(strings.ToLower(r.Kind), query) ||
		strings.Contains(strings.ToLower(r.Namespace), query) ||
		strings.Contains(strings.ToLower(r.KindName()), query)
}

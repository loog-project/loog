package diffpreview

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/loog-project/loog/pkg/diffmap"
)

// ChangeType indicates the kind of change at a node
type ChangeType int

const (
	Unchanged ChangeType = iota
	Added
	Removed
	Modified
)

// AnnotatedNode represents a node in the annotated tree
type AnnotatedNode struct {
	Value    any
	Change   ChangeType
	Children map[string]*AnnotatedNode
	// List holds annotated list elements (used when the value is []any)
	List []*AnnotatedNode
}

// Diff compares two maps and builds an annotated tree
func Diff(a, b map[string]any) *AnnotatedNode {
	changeset := diffmap.Diff(a, b)
	return buildAnnotatedTree(a, b, changeset)
}

// buildAnnotatedTree recursively builds a node tree based on diffmap output
func buildAnnotatedTree(a, b any, changes any) *AnnotatedNode {
	if changes == nil {
		return &AnnotatedNode{Value: a, Change: Unchanged}
	}

	if changeMap, ok := changes.(map[string]any); ok {
		node := &AnnotatedNode{Children: make(map[string]*AnnotatedNode)}
		aMap, _ := a.(map[string]any)
		bMap, _ := b.(map[string]any)

		for key, subchange := range changeMap {
			subA := aMap[key]
			subB := bMap[key]

			switch {
			case subchange == nil:
				node.Children[key] = &AnnotatedNode{Value: subA, Change: Removed}
			case subA == nil:
				node.Children[key] = &AnnotatedNode{Value: subB, Change: Added}
			default:
				node.Children[key] = buildAnnotatedTree(subA, subB, subchange)
			}
		}
		return node
	}

	// Scalar or list change
	return &AnnotatedNode{Value: b, Change: Modified}
}

func DiffRecursive(a, b map[string]any) *AnnotatedNode {
	return diffRecursive(a, b)
}

func diffRecursive(a, b map[string]any) *AnnotatedNode {
	node := &AnnotatedNode{Children: make(map[string]*AnnotatedNode)}

	keys := collectAllKeys(a, b)

	for _, key := range keys {
		valA, okA := a[key]
		valB, okB := b[key]

		switch {
		case okA && !okB:
			// Key exists in a, missing in b -> REMOVED
			node.Children[key] = buildFullNode(valA, Removed)

		case !okA && okB:
			// Key exists only in b -> ADDED
			node.Children[key] = buildFullNode(valB, Added)

		case okA && okB:
			// Key exists in both -> check deep equality
			mapA, okMapA := valA.(map[string]any)
			mapB, okMapB := valB.(map[string]any)

			listA, okListA := valA.([]any)
			listB, okListB := valB.([]any)

			switch {
			case okMapA && okMapB:
				// Nested map -> recurse
				child := diffRecursive(mapA, mapB)
				node.Children[key] = child
			case okListA && okListB:
				// Both are lists -> element-level diff
				node.Children[key] = diffList(listA, listB)
			case reflect.DeepEqual(valA, valB):
				// Same value -> unchanged
				node.Children[key] = buildUnchangedNode(valA)
			default:
				// Scalar changed -> modified
				node.Children[key] = &AnnotatedNode{Value: valB, Change: Modified}
			}
		}
	}

	return node
}

// diffList performs element-level diffing between two lists.
// It uses a best-effort matching strategy:
//   - For lists of maps, it tries to match elements by a "name" or "type" key.
//   - Falls back to positional matching.
//   - Extra elements in b are Added, missing elements from a are Removed.
func diffList(a, b []any) *AnnotatedNode {
	node := &AnnotatedNode{Change: Unchanged}

	// Try to match by key field (name, type, containerPort, port, host, etc.)
	matchKey := findListMatchKey(a, b)
	if matchKey != "" {
		node.List = diffListByKey(a, b, matchKey)
	} else {
		node.List = diffListPositional(a, b)
	}

	// Check if any children are non-unchanged
	for _, child := range node.List {
		if child.Change != Unchanged || hasChanges(child) {
			// Parent node has modifications somewhere
			return node
		}
	}

	return node
}

// findListMatchKey finds a common key in map elements that can be used to match them.
func findListMatchKey(a, b []any) string {
	// Candidate keys in priority order
	candidates := []string{"name", "type", "containerPort", "port", "host", "key", "path", "kind"}

	// Check if all map elements in both lists have the same candidate key
	for _, key := range candidates {
		if allMapsHaveKey(a, key) && allMapsHaveKey(b, key) {
			return key
		}
	}
	return ""
}

func allMapsHaveKey(list []any, key string) bool {
	hasMaps := false
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue // skip non-maps
		}
		hasMaps = true
		if _, ok := m[key]; !ok {
			return false
		}
	}
	return hasMaps
}

// diffListByKey matches list elements by a key field and diffs them.
func diffListByKey(a, b []any, matchKey string) []*AnnotatedNode {
	// Build index from b
	bByKey := make(map[string]any)
	bUsed := make(map[string]bool)
	var bOrder []string
	for _, item := range b {
		if m, ok := item.(map[string]any); ok {
			if keyVal, ok := m[matchKey]; ok {
				k := formatKeyVal(keyVal)
				bByKey[k] = item
				bOrder = append(bOrder, k)
			}
		}
	}

	var result []*AnnotatedNode

	// Process elements from a
	aUsed := make(map[string]bool)
	for _, item := range a {
		if m, ok := item.(map[string]any); ok {
			if keyVal, ok := m[matchKey]; ok {
				k := formatKeyVal(keyVal)
				aUsed[k] = true
				bItem, found := bByKey[k]
				if !found {
					// Removed element
					result = append(result, buildFullNode(item, Removed))
				} else {
					bUsed[k] = true
					// Both exist — diff them
					bMap, okBMap := bItem.(map[string]any)
					if okBMap {
						child := diffRecursive(m, bMap)
						result = append(result, child)
					} else if reflect.DeepEqual(item, bItem) {
						result = append(result, buildUnchangedNode(item))
					} else {
						result = append(result, &AnnotatedNode{Value: bItem, Change: Modified})
					}
				}
			} else {
				// No key — treat as removed
				result = append(result, buildFullNode(item, Removed))
			}
		} else {
			// Non-map element in a, check positionally later — for now mark removed
			result = append(result, buildFullNode(item, Removed))
		}
	}

	// Add new elements from b that weren't in a
	for _, k := range bOrder {
		if !bUsed[k] && !aUsed[k] {
			result = append(result, buildFullNode(bByKey[k], Added))
		}
	}

	return result
}

func formatKeyVal(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}

// diffListPositional matches list elements by position.
func diffListPositional(a, b []any) []*AnnotatedNode {
	var result []*AnnotatedNode
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(a) {
			// New element in b
			result = append(result, buildFullNode(b[i], Added))
		} else if i >= len(b) {
			// Removed element from a
			result = append(result, buildFullNode(a[i], Removed))
		} else {
			// Both exist
			mapA, okA := a[i].(map[string]any)
			mapB, okB := b[i].(map[string]any)

			if okA && okB {
				child := diffRecursive(mapA, mapB)
				result = append(result, child)
			} else if reflect.DeepEqual(a[i], b[i]) {
				result = append(result, buildUnchangedNode(a[i]))
			} else {
				result = append(result, &AnnotatedNode{Value: b[i], Change: Modified})
			}
		}
	}

	return result
}

// hasChanges checks if an annotated node tree contains any non-unchanged nodes.
func hasChanges(node *AnnotatedNode) bool {
	if node.Change != Unchanged {
		return true
	}
	for _, child := range node.Children {
		if hasChanges(child) {
			return true
		}
	}
	for _, child := range node.List {
		if hasChanges(child) {
			return true
		}
	}
	return false
}

// buildFullNode recursively builds a node tree where every node has the given change type.
func buildFullNode(val any, change ChangeType) *AnnotatedNode {
	switch v := val.(type) {
	case map[string]any:
		node := &AnnotatedNode{Change: change, Children: make(map[string]*AnnotatedNode)}
		for k, sub := range v {
			node.Children[k] = buildFullNode(sub, change)
		}
		return node
	case []any:
		node := &AnnotatedNode{Change: change}
		for _, item := range v {
			node.List = append(node.List, buildFullNode(item, change))
		}
		return node
	default:
		return &AnnotatedNode{Value: v, Change: change}
	}
}

func collectAllKeys(a, b map[string]any) []string {
	keySet := make(map[string]struct{})
	for k := range a {
		keySet[k] = struct{}{}
	}
	for k := range b {
		keySet[k] = struct{}{}
	}

	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

func buildUnchangedNode(val any) *AnnotatedNode {
	switch v := val.(type) {
	case map[string]any:
		node := &AnnotatedNode{Children: make(map[string]*AnnotatedNode)}
		for k, sub := range v {
			node.Children[k] = buildUnchangedNode(sub)
		}
		return node
	case []any:
		node := &AnnotatedNode{Change: Unchanged}
		for _, item := range v {
			node.List = append(node.List, buildUnchangedNode(item))
		}
		return node
	default:
		return &AnnotatedNode{Value: v, Change: Unchanged}
	}
}

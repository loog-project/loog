package diffpreview

import (
	"fmt"
	"reflect"
	"sort"
)

type ChangeType int

const (
	Unchanged ChangeType = iota
	Added
	Removed
	Modified
)

// AnnotatedNode represents a single node in the diff tree.
//
// A node is exactly one of: a map (Children is set), a list (List is set),
// or a scalar leaf (Value is set). Change records what happened to the node
// between the old and new versions.
type AnnotatedNode struct {
	Value    any
	Change   ChangeType
	Children map[string]*AnnotatedNode
	List     []*AnnotatedNode
}

// Diff compares two maps and returns a tree where every node is annotated
// with a ChangeType. Pass the result to RenderYAML to get highlighted output.
func Diff(a, b map[string]any) *AnnotatedNode {
	return diffMaps(a, b)
}

func diffMaps(a, b map[string]any) *AnnotatedNode {
	node := &AnnotatedNode{Children: make(map[string]*AnnotatedNode)}

	for _, key := range unionKeys(a, b) {
		valA, inA := a[key]
		valB, inB := b[key]

		switch {
		case inA && !inB:
			node.Children[key] = buildFullNode(valA, Removed)
		case !inA && inB:
			node.Children[key] = buildFullNode(valB, Added)
		default:
			node.Children[key] = diffValues(valA, valB)
		}
	}
	return node
}

// diffValues compares two values of any type and returns the matching node.
// Maps and lists are diffed recursively; scalars are compared for equality.
func diffValues(a, b any) *AnnotatedNode {
	mapA, aIsMap := a.(map[string]any)
	mapB, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		return diffMaps(mapA, mapB)
	}

	listA, aIsList := a.([]any)
	listB, bIsList := b.([]any)
	if aIsList && bIsList {
		return diffLists(listA, listB)
	}

	if reflect.DeepEqual(a, b) {
		return buildFullNode(a, Unchanged)
	}
	return &AnnotatedNode{Value: b, Change: Modified}
}

// diffLists diffs two lists element by element.
// When both lists contain maps with a shared identifier key (like "name"),
// elements are matched by that key. Otherwise they're compared by position.
func diffLists(a, b []any) *AnnotatedNode {
	node := &AnnotatedNode{Change: Unchanged}

	if key := findMatchKey(a, b); key != "" {
		node.List = diffListByKey(a, b, key)
	} else {
		node.List = diffListPositional(a, b)
	}
	return node
}

// findMatchKey checks a set of common Kubernetes field names and returns the
// first one that appears in every map element of both lists. Returns "" if
// no usable key is found.
func findMatchKey(a, b []any) string {
	candidates := []string{
		"name", "type", "containerPort", "port",
		"host", "key", "path", "kind",
	}
	for _, key := range candidates {
		if allMapsHaveKey(a, key) && allMapsHaveKey(b, key) {
			return key
		}
	}
	return ""
}

// allMapsHaveKey returns true when the list has at least one map element
// and every map element contains the given key. Non-map items are skipped.
func allMapsHaveKey(list []any, key string) bool {
	found := false
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		found = true
		if _, has := m[key]; !has {
			return false
		}
	}
	return found
}

// diffListByKey pairs elements from a and b by matchKey, then diffs the
// matched pairs. Elements only in a are marked Removed; elements only in b
// are marked Added.
func diffListByKey(a, b []any, matchKey string) []*AnnotatedNode {
	keyOf := func(item any) string {
		if m, ok := item.(map[string]any); ok {
			if v, ok := m[matchKey]; ok {
				return fmt.Sprintf("%v", v)
			}
		}
		return ""
	}

	// Index b elements by their match-key value while preserving order.
	bByKey := make(map[string]any, len(b))
	var bOrder []string
	for _, item := range b {
		k := keyOf(item)
		if k != "" {
			bByKey[k] = item
			bOrder = append(bOrder, k)
		}
	}

	var result []*AnnotatedNode
	matched := make(map[string]bool)

	for _, item := range a {
		k := keyOf(item)
		if k == "" {
			result = append(result, buildFullNode(item, Removed))
			continue
		}
		bItem, found := bByKey[k]
		if !found {
			result = append(result, buildFullNode(item, Removed))
			continue
		}
		matched[k] = true
		result = append(result, diffValues(item, bItem))
	}

	// Append elements from b that had no match in a.
	for _, k := range bOrder {
		if !matched[k] {
			result = append(result, buildFullNode(bByKey[k], Added))
		}
	}
	return result
}

func diffListPositional(a, b []any) []*AnnotatedNode {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}

	result := make([]*AnnotatedNode, 0, n)
	for i := range n {
		switch {
		case i >= len(a):
			result = append(result, buildFullNode(b[i], Added))
		case i >= len(b):
			result = append(result, buildFullNode(a[i], Removed))
		default:
			result = append(result, diffValues(a[i], b[i]))
		}
	}
	return result
}

// buildFullNode wraps a value in a full AnnotatedNode tree where every node
// carries the same change type. Used for subtrees that are entirely added,
// removed, or unchanged.
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

func unionKeys(a, b map[string]any) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

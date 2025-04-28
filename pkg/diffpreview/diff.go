package diffpreview

import (
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
			// Key exists in a, missing in b → REMOVED
			node.Children[key] = &AnnotatedNode{Value: valA, Change: Removed}

		case !okA && okB:
			// Key exists only in b → ADDED
			node.Children[key] = &AnnotatedNode{Value: valB, Change: Added}

		case okA && okB:
			// Key exists in both → check deep equality
			mapA, okMapA := valA.(map[string]any)
			mapB, okMapB := valB.(map[string]any)

			if okMapA && okMapB {
				// Nested map → recurse
				child := diffRecursive(mapA, mapB)
				node.Children[key] = child
			} else if reflect.DeepEqual(valA, valB) {
				// Same value → unchanged
				node.Children[key] = buildUnchangedNode(valA)
			} else {
				// Scalar changed → modified
				node.Children[key] = &AnnotatedNode{Value: valB, Change: Modified}
			}
		}
	}

	return node
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
		// Lists are treated atomic for now
		return &AnnotatedNode{Value: v, Change: Unchanged}
	default:
		return &AnnotatedNode{Value: v, Change: Unchanged}
	}
}

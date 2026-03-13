package resource

import (
	"sort"
	"strings"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// BuildKindGroups organizes resources into kind groups for tree display.
// Groups are sorted in a preferred Kubernetes kind order, and resources
// within each group are sorted by name.
func BuildKindGroups(resources []*ResourceData) []*KindGroup {
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

// ── Filter Evaluation ──

// resourceFilterEnv is an expr-lang environment that evaluates filter expressions
// against in-memory Resource data. It mirrors the method signatures from
// internal/util.EventEntryEnv but operates on Resource fields instead of
// *unstructured.Unstructured, avoiding k8s imports.
type resourceFilterEnv struct {
	Res Resource
}

func (e resourceFilterEnv) All() bool  { return true }
func (e resourceFilterEnv) None() bool { return false }

func (e resourceFilterEnv) Namespaces(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, v := range vals {
		if v == e.Res.Namespace {
			return true
		}
	}
	return false
}

func (e resourceFilterEnv) Namespace(vals ...string) bool { return e.Namespaces(vals...) }

func (e resourceFilterEnv) Names(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, v := range vals {
		if v == e.Res.Name {
			return true
		}
	}
	return false
}

func (e resourceFilterEnv) Name(vals ...string) bool { return e.Names(vals...) }

func (e resourceFilterEnv) Namespaced(namespace, name string) bool {
	return e.Res.Namespace == namespace && e.Res.Name == name
}

func (e resourceFilterEnv) LabelExists(_ ...string) bool {
	// Labels are not stored in Resource — always match (conservative).
	return true
}

func (e resourceFilterEnv) Label(_, _ string) bool {
	// Labels are not stored in Resource — always match (conservative).
	return true
}

// CompileFilter compiles a filter expression string for use with MatchesFilter.
// Returns nil if the expression is empty or not a valid expr-lang expression.
func CompileFilter(expression string) *vm.Program {
	if expression == "" {
		return nil
	}
	prog, err := expr.Compile(expression, expr.Env(resourceFilterEnv{}), expr.AsBool())
	if err != nil {
		return nil
	}
	return prog
}

// MatchesFilter evaluates a compiled filter program against a Resource.
// Returns true if the resource matches. Returns true if prog is nil (no filter).
func MatchesFilter(prog *vm.Program, r Resource) bool {
	if prog == nil {
		return true
	}
	result, err := expr.Run(prog, resourceFilterEnv{Res: r})
	if err != nil {
		return true // conservative: show on error
	}
	b, ok := result.(bool)
	if !ok {
		return true
	}
	return b
}

// MatchesFilterOrSubstring first tries the compiled expr-lang program, and if prog
// is nil, falls back to case-insensitive substring matching on kind/name/namespace.
func MatchesFilterOrSubstring(prog *vm.Program, r Resource, lowerExpr string) bool {
	if prog != nil {
		return MatchesFilter(prog, r)
	}
	// Fallback: substring match for plain text queries
	if lowerExpr == "" {
		return true
	}
	return strings.Contains(strings.ToLower(r.Name), lowerExpr) ||
		strings.Contains(strings.ToLower(r.Kind), lowerExpr) ||
		strings.Contains(strings.ToLower(r.Namespace), lowerExpr) ||
		strings.Contains(strings.ToLower(r.KindName()), lowerExpr)
}

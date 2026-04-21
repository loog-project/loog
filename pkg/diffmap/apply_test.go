package diffmap_test

import (
	"reflect"
	"testing"

	"github.com/loog-project/loog/pkg/diffmap"
)

// deepCopy returns an independent deep copy of m so that nested maps are not shared.
func deepCopy(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			out[k] = deepCopy(sub)
		} else {
			out[k] = v
		}
	}
	return out
}

func TestApplyRoundTrip(t *testing.T) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	b := map[string]any{"a": 1, "b": map[string]any{"c": true}}

	// Diff then apply, expect to arrive at b.
	chg := diffmap.Diff(a, b)

	dst := deepCopy(a)
	diffmap.Apply(dst, chg)

	if !reflect.DeepEqual(dst, b) {
		t.Fatalf("apply failed: got %v, want %v", dst, b)
	}
}

func TestApply_NilDst(t *testing.T) {
	chg := map[string]any{"a": 1}
	// Apply on nil dst should be a no-op (no panic).
	diffmap.Apply(nil, chg)
}

func TestApply_AddNestedMap(t *testing.T) {
	// Key "a" starts as a scalar; change-set replaces it with a nested map.
	dst := map[string]any{"a": "scalar"}
	chg := map[string]any{"a": map[string]any{"x": 1, "y": 2}}

	diffmap.Apply(dst, chg)

	want := map[string]any{"a": map[string]any{"x": 1, "y": 2}}
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("got %v, want %v", dst, want)
	}
}

func TestApply_DeleteKey(t *testing.T) {
	dst := map[string]any{"a": 1, "b": 2}
	chg := map[string]any{"b": nil}

	diffmap.Apply(dst, chg)

	want := map[string]any{"a": 1}
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("got %v, want %v", dst, want)
	}
}

func BenchmarkApply_Small(b *testing.B) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	bb := map[string]any{"a": 1, "b": map[string]any{"c": true}}
	chg := diffmap.Diff(a, bb)
	for i := 0; i < b.N; i++ {
		dst := deepCopy(a)
		diffmap.Apply(dst, chg)
	}
}

func BenchmarkApply_1k(b *testing.B) {
	a, bb := genMaps(1000)
	chg := diffmap.Diff(a, bb)
	for i := 0; i < b.N; i++ {
		dst := deepCopy(a)
		diffmap.Apply(dst, chg)
	}
}

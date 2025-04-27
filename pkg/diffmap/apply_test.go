package diffmap_test

import (
	"reflect"
	"testing"

	"github.com/loog-project/loog/pkg/diffmap"
)

func TestApplyRoundTrip(t *testing.T) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	b := map[string]any{"a": 1, "b": map[string]any{"c": true}}

	// Diff then apply, expect to arrive at b.
	chg := diffmap.Diff(a, b)
	dst := copyMap(a)
	diffmap.Apply(dst, chg)

	if !reflect.DeepEqual(dst, b) {
		t.Fatalf("apply failed: got %v, want %v", dst, b)
	}
}

func BenchmarkApply_Small(b *testing.B) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	bb := map[string]any{"a": 1, "b": map[string]any{"c": true}}
	chg := diffmap.Diff(a, bb)
	for i := 0; i < b.N; i++ {
		dst := copyMap(a)
		diffmap.Apply(dst, chg)
	}
}

func BenchmarkApply_1k(b *testing.B) {
	a, bb := genMaps(1000)
	chg := diffmap.Diff(a, bb)
	for i := 0; i < b.N; i++ {
		dst := copyMap(a)
		diffmap.Apply(dst, chg)
	}
}

// helper: fast shallow copy.
func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

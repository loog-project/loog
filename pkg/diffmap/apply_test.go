package diffmap_test

import (
	"maps"
	"reflect"
	"testing"

	"github.com/loog-project/loog/pkg/diffmap"
)

func TestApplyRoundTrip(t *testing.T) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	b := map[string]any{"a": 1, "b": map[string]any{"c": true}}

	// Diff then apply, expect to arrive at b.
	chg := diffmap.Diff(a, b)

	dst := make(map[string]any)
	maps.Copy(dst, a)
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
		dst := make(map[string]any)
		maps.Copy(dst, a)
		diffmap.Apply(dst, chg)
	}
}

func BenchmarkApply_1k(b *testing.B) {
	a, bb := genMaps(1000)
	chg := diffmap.Diff(a, bb)
	for i := 0; i < b.N; i++ {
		var dst map[string]any
		maps.Copy(dst, a)
		diffmap.Apply(dst, chg)
	}
}

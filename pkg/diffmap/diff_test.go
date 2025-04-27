package diffmap_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/loog-project/loog/pkg/diffmap"
)

func TestDiffExamples(t *testing.T) {
	cases := []struct {
		a, b, want map[string]any
	}{
		{
			map[string]any{"a": 1, "b": map[string]any{"c": false}},
			map[string]any{"a": 1, "b": map[string]any{"c": true}},
			map[string]any{"b": map[string]any{"c": true}},
		},
		{
			map[string]any{"a": 1, "b": map[string]any{"c": false}},
			map[string]any{"a": 2, "b": map[string]any{"c": false}},
			map[string]any{"a": 2},
		},
		{
			map[string]any{"a": 1, "b": map[string]any{"c": false}},
			map[string]any{"a": 1, "b": map[string]any{"e": true}},
			map[string]any{"b": map[string]any{"c": nil, "e": true}},
		},
		{
			map[string]any{"a": 1, "b": map[string]any{"c": false}},
			map[string]any{"b": map[string]any{"c": false}},
			map[string]any{"a": nil},
		},
	}
	for i, tc := range cases {
		got := diffmap.Diff(tc.a, tc.b)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("case %d: want %v, got %v", i, tc.want, got)
		}
	}
}

func BenchmarkDiff_Small(b *testing.B) {
	a := map[string]any{"a": 1, "b": map[string]any{"c": false}}
	bb := map[string]any{"a": 1, "b": map[string]any{"c": true}}
	for i := 0; i < b.N; i++ {
		_ = diffmap.Diff(a, bb)
	}
}

func BenchmarkDiff_1k(b *testing.B) {
	a, bb := genMaps(1000)
	for i := 0; i < b.N; i++ {
		_ = diffmap.Diff(a, bb)
	}
}

// genMaps creates two 1-k-entry maps with 10 % churn.
func genMaps(n int) (map[string]any, map[string]any) {
	a := make(map[string]any, n)
	b := make(map[string]any, n)
	for i := 0; i < n; i++ {
		key := "k" + strconv.Itoa(i)
		a[key] = i
		if i%10 == 0 {
			// mutated
			b[key] = i + 1
		} else {
			b[key] = i
		}
	}
	return a, b
}

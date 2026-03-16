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
	for i := range n {
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

func TestDiff_NilInputs(t *testing.T) {
	someMap := map[string]any{"a": 1}

	// nil, nil -> nil
	if d := diffmap.Diff(nil, nil); d != nil {
		t.Fatalf("Diff(nil, nil) = %v, want nil", d)
	}

	// nil, someMap -> adds everything from someMap
	d := diffmap.Diff(nil, someMap)
	want := map[string]any{"a": 1}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("Diff(nil, someMap) = %v, want %v", d, want)
	}

	// someMap, nil -> removes everything from someMap
	d = diffmap.Diff(someMap, nil)
	want = map[string]any{"a": nil}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("Diff(someMap, nil) = %v, want %v", d, want)
	}
}

func TestDiff_SliceValues(t *testing.T) {
	a := map[string]any{"items": []any{"a", "b"}}
	b := map[string]any{"items": []any{"a", "c"}}

	d := diffmap.Diff(a, b)
	want := map[string]any{"items": []any{"a", "c"}}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("got %v, want %v", d, want)
	}
}

func TestDiff_TypeChange(t *testing.T) {
	// scalar -> map
	a := map[string]any{"k": "scalar"}
	b := map[string]any{"k": map[string]any{"nested": true}}

	d := diffmap.Diff(a, b)
	want := map[string]any{"k": map[string]any{"nested": true}}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("scalar->map: got %v, want %v", d, want)
	}

	// map -> scalar
	d = diffmap.Diff(b, a)
	want = map[string]any{"k": "scalar"}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("map->scalar: got %v, want %v", d, want)
	}
}

func TestDiff_DeepNested(t *testing.T) {
	a := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": map[string]any{
					"l4": "old",
				},
			},
		},
	}
	b := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": map[string]any{
					"l4": "new",
				},
			},
		},
	}

	d := diffmap.Diff(a, b)
	want := map[string]any{
		"l1": map[string]any{
			"l2": map[string]any{
				"l3": map[string]any{
					"l4": "new",
				},
			},
		},
	}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("got %v, want %v", d, want)
	}
}

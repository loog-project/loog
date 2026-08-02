package diffpreview

import (
	"strings"
	"testing"
)

// Empty collections must not render as "null".

func TestRender_EmptyListNotNull(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"items": buildFullNode([]any{}, Unchanged),
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "items: []")
	if strings.Contains(out, "null") {
		t.Errorf("empty list rendered as null:\n%s", out)
	}
}

func TestRender_EmptyMapNotNull(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"meta": {Change: Unchanged, Children: map[string]*AnnotatedNode{}},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "meta: {}")
}

// Integer types other than int should still be number-styled / rendered.

func TestRender_IntegerTypes(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"i64": {Value: int64(42), Change: Unchanged},
			"i32": {Value: int32(7), Change: Unchanged},
			"u64": {Value: uint64(9), Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "i64: 42")
	assertContains(t, out, "i32: 7")
	assertContains(t, out, "u64: 9")
}

// A keyed list that also contains scalar elements in b must not drop those
// scalars from the diff.
func TestDiff_KeyedListKeepsScalarAdditions(t *testing.T) {
	a := map[string]any{
		"ports": []any{
			map[string]any{"name": "http", "port": float64(80)},
		},
	}
	b := map[string]any{
		"ports": []any{
			map[string]any{"name": "http", "port": float64(80)},
			"extra-scalar",
		},
	}
	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "extra-scalar")
}

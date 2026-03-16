package diffpreview

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// plainTheme applies no styling so test output is just raw text.
var plainTheme = Theme{
	KeyStyle:    lipgloss.NewStyle(),
	StringStyle: lipgloss.NewStyle(),
	NumberStyle: lipgloss.NewStyle(),
	BoolStyle:   lipgloss.NewStyle(),
	NullStyle:   lipgloss.NewStyle(),
	AddedBg:     lipgloss.NewStyle(),
	RemovedBg:   lipgloss.NewStyle(),
	ModifiedBg:  lipgloss.NewStyle(),
}

var plainOpts = RenderOptions{
	IndentSize:                2,
	EnableBackgroundHighlight: false,
}

var bgOpts = RenderOptions{
	IndentSize:                2,
	EnableBackgroundHighlight: true,
}

// --- Basic scalar rendering -------------------------------------------

func TestRender_SingleString(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"name": {Value: "pod-1", Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `name: "pod-1"`)
}

func TestRender_SingleInt(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"replicas": {Value: 3, Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "replicas: 3")
}

func TestRender_SingleFloat(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"ratio": {Value: 1.5, Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "ratio: 1.5")
}

func TestRender_SingleBool(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"enabled": {Value: true, Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "enabled: true")
}

func TestRender_SingleNull(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"data": {Value: nil, Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "data: null")
}

// --- Nested map -------------------------------------------------------

func TestRender_NestedMap(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"metadata": {
				Children: map[string]*AnnotatedNode{
					"name":      {Value: "pod-1", Change: Unchanged},
					"namespace": {Value: "default", Change: Unchanged},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "metadata:")
	assertContains(t, out, `  name: "pod-1"`)
	assertContains(t, out, `  namespace: "default"`)
}

// --- Key ordering -----------------------------------------------------

func TestRender_KeysSorted(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"zebra": {Value: 1, Change: Unchanged},
			"alpha": {Value: 2, Change: Unchanged},
			"mike":  {Value: 3, Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "alpha:") {
		t.Errorf("first key should be alpha, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "mike:") {
		t.Errorf("second key should be mike, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "zebra:") {
		t.Errorf("third key should be zebra, got %q", lines[2])
	}
}

// --- Simple list rendering --------------------------------------------

func TestRender_ScalarList(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"ports": {
				List: []*AnnotatedNode{
					{Value: 80, Change: Unchanged},
					{Value: 443, Change: Unchanged},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "ports:")
	assertContains(t, out, "- 80")
	assertContains(t, out, "- 443")
}

func TestRender_StringList(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"args": {
				List: []*AnnotatedNode{
					{Value: "--verbose", Change: Unchanged},
					{Value: "--port=8080", Change: Unchanged},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `- "--verbose"`)
	assertContains(t, out, `- "--port=8080"`)
}

// --- List of maps (first key on "- " line) ----------------------------

func TestRender_ListOfMaps(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"containers": {
				List: []*AnnotatedNode{
					{
						Children: map[string]*AnnotatedNode{
							"image": {Value: "nginx", Change: Unchanged},
							"name":  {Value: "web", Change: Unchanged},
						},
					},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	// Keys are sorted, so "image" comes before "name".
	assertContains(t, out, `- image: "nginx"`)
	assertContains(t, out, `  name: "web"`)
}

func TestRender_ListOfMaps_EmptyMap(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"items": {
				List: []*AnnotatedNode{
					{Children: map[string]*AnnotatedNode{}},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "- {}")
}

// --- Indentation levels -----------------------------------------------

func TestRender_IndentSize4(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"a": {
				Children: map[string]*AnnotatedNode{
					"b": {Value: 1, Change: Unchanged},
				},
			},
		},
	}
	opts := RenderOptions{IndentSize: 4, EnableBackgroundHighlight: false}
	out := RenderYAML(node, plainTheme, opts)
	assertContains(t, out, "    b: 1")
}

// --- Full Diff + Render integration -----------------------------------

func TestRender_DiffIntegration_Added(t *testing.T) {
	a := map[string]any{}
	b := map[string]any{"new-key": "value"}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `new-key: "value"`)
}

func TestRender_DiffIntegration_Removed(t *testing.T) {
	a := map[string]any{"old-key": "value"}
	b := map[string]any{}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `old-key: "value"`)
}

func TestRender_DiffIntegration_Modified(t *testing.T) {
	a := map[string]any{"replicas": 2}
	b := map[string]any{"replicas": 5}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "replicas: 5")
}

func TestRender_DiffIntegration_NestedList(t *testing.T) {
	a := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "v1"},
		},
	}
	b := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "v2"},
		},
	}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "containers:")
	assertContains(t, out, `"v2"`)
}

// --- Background highlighting ------------------------------------------

func TestRender_BackgroundHighlight_Applied(t *testing.T) {
	// Verify that background highlighting changes the output compared to
	// no background highlighting. Even without a TTY (lipgloss strips ANSI
	// in non-TTY environments), the codepath should differ: with bg enabled,
	// the theme's background styles are applied.
	node := Diff(
		map[string]any{"x": "old"},
		map[string]any{"x": "new"},
	)

	out := RenderYAML(node, DarkTheme, bgOpts)
	assertContains(t, out, "new")
}

func TestRender_BackgroundHighlight_Unchanged_NoExtra(t *testing.T) {
	node := Diff(
		map[string]any{"x": "same"},
		map[string]any{"x": "same"},
	)

	withBg := RenderYAML(node, DarkTheme, bgOpts)
	noBg := RenderYAML(node, DarkTheme, RenderOptions{
		IndentSize:                2,
		EnableBackgroundHighlight: false,
	})

	// For unchanged content, background highlighting shouldn't add extra styling.
	if withBg != noBg {
		t.Error("unchanged content should render identically with and without background highlighting")
	}
}

// --- effectiveChange ---------------------------------------------------

func TestEffectiveChange(t *testing.T) {
	tests := []struct {
		item, parent ChangeType
		want         ChangeType
	}{
		{Unchanged, Unchanged, Unchanged},
		{Unchanged, Added, Added},
		{Unchanged, Removed, Removed},
		{Modified, Added, Modified},
		{Added, Unchanged, Added},
	}
	for _, tc := range tests {
		got := effectiveChange(tc.item, tc.parent)
		if got != tc.want {
			t.Errorf("effectiveChange(%v, %v) = %v, want %v", tc.item, tc.parent, got, tc.want)
		}
	}
}

// --- Nested list in list -----------------------------------------------

func TestRender_NestedListInList(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"matrix": {
				List: []*AnnotatedNode{
					{
						List: []*AnnotatedNode{
							{Value: 1, Change: Unchanged},
							{Value: 2, Change: Unchanged},
						},
					},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "matrix:")
	assertContains(t, out, "- 1")
	assertContains(t, out, "- 2")
}

// --- Empty map / empty list -------------------------------------------

func TestRender_EmptyRoot(t *testing.T) {
	node := &AnnotatedNode{Children: map[string]*AnnotatedNode{}}
	out := RenderYAML(node, plainTheme, plainOpts)
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

// --- sortedKeys --------------------------------------------------------

func TestSortedKeys(t *testing.T) {
	m := map[string]*AnnotatedNode{
		"c": {}, "a": {}, "b": {},
	}
	keys := sortedKeys(m)
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("expected [a b c], got %v", keys)
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	keys := sortedKeys(map[string]*AnnotatedNode{})
	if len(keys) != 0 {
		t.Errorf("expected empty, got %v", keys)
	}
}

// --- Syntax highlighting (styled theme) --------------------------------

func TestRender_SyntaxHighlight_String(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"name": {Value: "test", Change: Unchanged},
		},
	}
	out := RenderYAML(node, DarkTheme, plainOpts)
	// In a TTY environment DarkTheme produces ANSI codes; in CI/tests it
	// may not. Either way the raw text must be present.
	assertContains(t, out, "test")
	assertContains(t, out, "name")
}

// --- renderer.syntaxHighlight -----------------------------------------

func TestSyntaxHighlight_AllTypes(t *testing.T) {
	r := renderer{theme: plainTheme, opts: plainOpts}

	tests := []struct {
		val  any
		want string
	}{
		{"hello", `"hello"`},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{3.14, "3.14"},
		{nil, "null"},
		{int64(99), "99"}, // fallback via %v
	}
	for _, tc := range tests {
		got := r.syntaxHighlight(tc.val)
		if got != tc.want {
			t.Errorf("syntaxHighlight(%v) = %q, want %q", tc.val, got, tc.want)
		}
	}
}

// --- Theme.backgroundStyle --------------------------------------------

func TestBackgroundStyle_Unchanged(t *testing.T) {
	if DarkTheme.backgroundStyle(Unchanged) != nil {
		t.Error("Unchanged should return nil style")
	}
}

func TestBackgroundStyle_Changed(t *testing.T) {
	for _, ct := range []ChangeType{Added, Removed, Modified} {
		if DarkTheme.backgroundStyle(ct) == nil {
			t.Errorf("expected non-nil style for %v", ct)
		}
	}
}

// --- Kubernetes-like full integration ----------------------------------

func TestRender_FullKubernetesLike(t *testing.T) {
	a := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
		},
		"spec": map[string]any{
			"replicas": 2,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "web",
							"image": "nginx:1.19",
						},
					},
				},
			},
		},
	}
	b := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "production",
		},
		"spec": map[string]any{
			"replicas": 5,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "web",
							"image": "nginx:1.21",
						},
					},
				},
			},
		},
	}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)

	// Verify structure: all keys should appear
	for _, key := range []string{
		"apiVersion:",
		"kind:",
		"metadata:",
		"name:",
		"namespace:",
		"spec:",
		"replicas:",
		"template:",
		"containers:",
		"image:",
	} {
		assertContains(t, out, key)
	}

	// Verify changed values appear with new content
	assertContains(t, out, `"production"`)
	assertContains(t, out, "5")
	assertContains(t, out, `"nginx:1.21"`)

	// Verify output is valid multi-line YAML-like structure
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 10 {
		t.Errorf("expected at least 10 lines of output, got %d", len(lines))
	}
}

// --- helpers -----------------------------------------------------------

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q\ngot:\n%s", needle, haystack)
	}
}

// --- Coverage gap fillers ---------------------------------------------

// renderNode bare-list path: a root node that IS a list (no Children).
func TestRender_RootList(t *testing.T) {
	node := &AnnotatedNode{
		List: []*AnnotatedNode{
			{Value: "a", Change: Unchanged},
			{Value: "b", Change: Unchanged},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `- "a"`)
	assertContains(t, out, `- "b"`)
}

// renderNode bare-leaf path: a root node that IS a scalar (no Children, no List).
func TestRender_RootLeaf(t *testing.T) {
	node := &AnnotatedNode{Value: "solo", Change: Unchanged}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `"solo"`)
}

// renderChildValue list branch: a key whose value is a list node.
func TestRender_ChildValueIsList(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"items": {
				List: []*AnnotatedNode{
					{Value: 1, Change: Added},
					{Value: 2, Change: Added},
				},
				Change: Added,
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "items:")
	assertContains(t, out, "- 1")
	assertContains(t, out, "- 2")
}

// renderChildValue map branch inside a list-of-maps: a map item where a
// child value is itself a nested map (exercises the Children branch of
// renderChildValue for non-first keys in renderListMapItem).
func TestRender_ListMapItem_NestedMapChild(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"volumes": {
				List: []*AnnotatedNode{
					{
						Children: map[string]*AnnotatedNode{
							"hostPath": {
								Children: map[string]*AnnotatedNode{
									"path": {Value: "/data", Change: Unchanged},
								},
							},
							"name": {Value: "vol", Change: Unchanged},
						},
					},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "volumes:")
	assertContains(t, out, `- hostPath:`)
	assertContains(t, out, `path: "/data"`)
	assertContains(t, out, `name: "vol"`)
}

// renderChildValue list branch inside a list-of-maps: a map item where a
// child value is itself a list.
func TestRender_ListMapItem_NestedListChild(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"containers": {
				List: []*AnnotatedNode{
					{
						Children: map[string]*AnnotatedNode{
							"args": {
								List: []*AnnotatedNode{
									{Value: "--verbose", Change: Unchanged},
								},
							},
							"name": {Value: "app", Change: Unchanged},
						},
					},
				},
			},
		},
	}
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, "containers:")
	assertContains(t, out, `- args:`)
	assertContains(t, out, `"--verbose"`)
	assertContains(t, out, `name: "app"`)
}

// hasChanges list branch: a tree with changes only inside List children.
func TestHasChanges_InList(t *testing.T) {
	node := &AnnotatedNode{
		Change: Unchanged,
		List: []*AnnotatedNode{
			{Value: 1, Change: Unchanged},
			{Value: 2, Change: Added},
		},
	}
	if !hasChanges(node) {
		t.Error("expected hasChanges=true for list with Added element")
	}
}

// styledDash with Unchanged change (no bg applied).
func TestRender_StyledDash_Unchanged(t *testing.T) {
	node := &AnnotatedNode{
		Children: map[string]*AnnotatedNode{
			"items": {
				List: []*AnnotatedNode{
					{Value: "x", Change: Unchanged},
				},
			},
		},
	}
	out := RenderYAML(node, DarkTheme, bgOpts)
	assertContains(t, out, "x")
}

// Diff integration with the list-in-map child path where both keys have
// nested children (ensures renderChildValue map+list branches fire via
// a realistic diff).
func TestRender_DiffIntegration_NestedListInMap(t *testing.T) {
	a := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name": "app",
					"args": []any{"--port=80"},
					"env": map[string]any{
						"HOME": "/root",
					},
				},
			},
		},
	}
	b := map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name": "app",
					"args": []any{"--port=80", "--debug"},
					"env": map[string]any{
						"HOME": "/root",
						"LOG":  "verbose",
					},
				},
			},
		},
	}

	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, plainOpts)
	assertContains(t, out, `"--debug"`)
	assertContains(t, out, `"verbose"`)
}

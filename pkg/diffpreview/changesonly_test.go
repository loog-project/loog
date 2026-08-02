package diffpreview

import (
	"strings"
	"testing"
)

// ChangesOnly should keep changed paths and drop unchanged siblings.
func TestRender_ChangesOnly(t *testing.T) {
	a := map[string]any{
		"metadata": map[string]any{"name": "p", "resourceVersion": "1"},
		"spec":     map[string]any{"replicas": float64(1), "image": "nginx:1"},
		"status":   map[string]any{"phase": "Running"},
	}
	b := map[string]any{
		"metadata": map[string]any{"name": "p", "resourceVersion": "2"},
		"spec":     map[string]any{"replicas": float64(3), "image": "nginx:1"},
		"status":   map[string]any{"phase": "Running"},
	}
	node := Diff(a, b)
	out := RenderYAML(node, plainTheme, RenderOptions{IndentSize: 2, ChangesOnly: true})

	assertContains(t, out, "resourceVersion")
	assertContains(t, out, "replicas")
	if strings.Contains(out, "phase") {
		t.Errorf("ChangesOnly should drop unchanged 'status.phase':\n%s", out)
	}
	if strings.Contains(out, "image") {
		t.Errorf("ChangesOnly should drop unchanged 'spec.image':\n%s", out)
	}
	if strings.Contains(out, `name: "p"`) {
		t.Errorf("ChangesOnly should drop unchanged 'metadata.name':\n%s", out)
	}
}

// ChangesOnly on identical objects yields no output.
func TestRender_ChangesOnlyEmptyWhenEqual(t *testing.T) {
	m := map[string]any{"spec": map[string]any{"replicas": float64(1)}}
	node := Diff(m, m)
	out := RenderYAML(node, plainTheme, RenderOptions{IndentSize: 2, ChangesOnly: true})
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty output for equal objects, got:\n%q", out)
	}
}

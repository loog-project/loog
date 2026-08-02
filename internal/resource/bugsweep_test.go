package resource

import (
	"testing"
	"time"
)

func TestLoopLabel(t *testing.T) {
	cases := map[int]string{
		0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA",
	}
	for i, want := range cases {
		if got := loopLabel(i); got != want {
			t.Errorf("loopLabel(%d) = %q, want %q", i, got, want)
		}
	}
	// All labels for 0..100 must be unique (regression for the old ">25 -> Z" bug).
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		l := loopLabel(i)
		if seen[l] {
			t.Fatalf("loopLabel produced duplicate label %q at %d", l, i)
		}
		seen[l] = true
	}
}

func TestDetectLoopNegativeWindowNoPanic(t *testing.T) {
	rd := &Data{Revisions: []Revision{
		{ID: 1, Object: map[string]any{"a": 1}},
		{ID: 2, Object: map[string]any{"a": 2}},
		{ID: 3, Object: map[string]any{"a": 1}},
	}}
	// Must not panic and must return false for a non-positive window.
	if rd.DetectLoop(-5) {
		t.Error("DetectLoop(-5) should be false")
	}
	if rd.DetectLoop(0) {
		t.Error("DetectLoop(0) should be false")
	}
}

func TestAnalyzeLoopNegativeWindowNoPanic(t *testing.T) {
	rd := &Data{Revisions: []Revision{
		{ID: 1, Time: time.Now(), Object: map[string]any{"a": 1}},
		{ID: 2, Time: time.Now(), Object: map[string]any{"a": 2}},
		{ID: 3, Time: time.Now(), Object: map[string]any{"a": 1}},
		{ID: 4, Time: time.Now(), Object: map[string]any{"a": 2}},
	}}
	got := rd.AnalyzeLoop(-1)
	if got.IsLoop {
		t.Error("AnalyzeLoop(-1) should report no loop")
	}
}

func TestCloneMapDeepClonesNestedTypes(t *testing.T) {
	orig := map[string]any{
		"scalar": "x",
		"nested": map[string]any{"n": 1},
		"list":   []any{map[string]any{"a": 1}, "s"},
		"maps":   []map[string]any{{"k": "v"}},
	}
	clone := CloneMap(orig)

	// Mutate the clone's nested structures; the original must not change.
	clone["nested"].(map[string]any)["n"] = 99
	clone["list"].([]any)[0].(map[string]any)["a"] = 99
	clone["maps"].([]map[string]any)[0]["k"] = "changed"

	if orig["nested"].(map[string]any)["n"] != 1 {
		t.Error("nested map was not deep-cloned")
	}
	if orig["list"].([]any)[0].(map[string]any)["a"] != 1 {
		t.Error("[]any element was not deep-cloned")
	}
	if orig["maps"].([]map[string]any)[0]["k"] != "v" {
		t.Error("[]map[string]any element was not deep-cloned")
	}
}

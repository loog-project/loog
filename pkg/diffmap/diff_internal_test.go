package diffmap

import "testing"

func BenchmarkEqualFast_Slice(b *testing.B) {
	a := map[string]any{"items": []any{"a", "b", "c", 1, 2, 3}}
	for b.Loop() {
		equalFast(a["items"], a["items"])
	}
}

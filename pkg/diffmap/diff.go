// Package diffmap computes the change-set that would turn map [a] into map [b].
//
// A change-set is itself a [map[string]any] that contains only the keys that
// differ. Added keys get their new value, removed keys get a nil value, and
// modified nested-maps are expressed recursively.
package diffmap

import "reflect"

// Diff returns the minimal change-set required to transform [a] into [b].
// If [a] and [b] are equal it returns nil (not an empty map) so callers can
// test `if Diff(...) == nil { }` with zero allocations.
func Diff(a, b map[string]any) map[string]any {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	diff := make(map[string]any)
	diffRecursive(a, b, diff)
	if len(diff) == 0 {
		return nil
	}
	return diff
}

// diffRecursive recursively computes the difference between two maps.
func diffRecursive(a, b map[string]any, out map[string]any) {
	for keyA, valueA := range a {
		valueBFromKeyA, hasAInB := b[keyA]
		if !hasAInB { // the key was removed
			out[keyA] = nil
			continue
		}

		if equalFast(valueA, valueBFromKeyA) {
			continue
		}

		// Both present but not equal.
		if valueAAsMap, okA := valueA.(map[string]any); okA {
			if valueBFromKeyAAsMap, okB := valueBFromKeyA.(map[string]any); okB {
				sub := make(map[string]any)
				diffRecursive(valueAAsMap, valueBFromKeyAAsMap, sub)
				if len(sub) != 0 {
					out[keyA] = sub
				}
				continue
			}
		}
		out[keyA] = valueBFromKeyA // scalar changed or type mismatch
	}
	for k, vb := range b {
		if _, already := a[k]; !already {
			out[k] = vb
		}
	}
}

// equalFast is a tight equality test that avoids reflection.
//
// Fallback to reflect.DeepEqual only for _weird_ values (like structs, slices, pointers, ...)
func equalFast(a, b any) bool {
	switch va := a.(type) {
	case string:
		vb, ok := b.(string)
		return ok && va == vb
	case float64:
		vb, ok := b.(float64)
		return ok && va == vb
	case int:
		vb, ok := b.(int)
		return ok && va == vb
	case int64:
		vb, ok := b.(int64)
		return ok && va == vb
	case bool:
		vb, ok := b.(bool)
		return ok && va == vb
	case nil:
		return b == nil
	case map[string]any:
		// We do *not* recurse here; we only need to know “equal or not”.
		vb, ok := b.(map[string]any)
		return ok && len(va) == 0 && len(vb) == 0 // tiny shortcut
	}
	return reflect.DeepEqual(a, b)
}

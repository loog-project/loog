package diffmap

import "reflect"

// Diff returns the minimal change-set required to transform [a] into [b].
// If [a] and [b] are equal it returns nil (not an empty map) so callers can
// test `if Diff(...) == nil { }` with zero allocations.
func Diff(a, b DiffMap) DiffMap {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	diff := make(DiffMap)
	diffRecursive(a, b, diff)
	if len(diff) == 0 {
		return nil
	}
	return diff
}

// diffRecursive recursively computes the difference between two maps.
func diffRecursive(a, b DiffMap, out DiffMap) {
	for keyA, valueA := range a {
		valueBFromKeyA, hasAInB := b[keyA]
		if !hasAInB {
			out[keyA] = nil
			continue
		}

		if equalFast(valueA, valueBFromKeyA) {
			continue
		}

		// Both present but not equal.
		if valueAAsMap, okA := valueA.(DiffMap); okA {
			if valueBFromKeyAAsMap, okB := valueBFromKeyA.(DiffMap); okB {
				sub := make(DiffMap)
				diffRecursive(valueAAsMap, valueBFromKeyAAsMap, sub)
				if len(sub) != 0 {
					out[keyA] = sub
				}
				continue
			}
		}
		out[keyA] = valueBFromKeyA
	}
	for k, vb := range b {
		if _, already := a[k]; !already {
			out[k] = vb
		}
	}
}

// equalFast is a tight equality test that avoids reflection for common types.
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
	case DiffMap:
		vb, ok := b.(DiffMap)
		if !ok {
			return false
		}
		return diffMapsEqual(va, vb)
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !equalFast(va[i], vb[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a, b)
}

// diffMapsEqual recursively checks if two DiffMaps are equal without
// allocating a diff map. This avoids the reflect.DeepEqual fallback
// for the very common case of nested map[string]any values.
func diffMapsEqual(a, b DiffMap) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !equalFast(va, vb) {
			return false
		}
	}
	return true
}

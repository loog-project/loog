package diffmap

// Apply mutates [dst] so that, after the call, [dst] equals
// the map that originally produced the given change-set `chg`.
//
//	dst := map[string]any{"a": 1, "b": map[string]any{"c": false}}
//	chg := map[string]any{"b": map[string]any{"c": true}}
//	diffmap.Apply(dst, chg) // dst is now {"a":1,"b":{"c":true}}
func Apply(dst, chg DiffMap) {
	if dst == nil || chg == nil || len(chg) == 0 {
		return
	}
	applyRecursive(dst, chg)
}

// applyRecursive recursively applies the change-set to the destination map.
func applyRecursive(dst, chg DiffMap) {
	for keyChange, valueChange := range chg {
		switch value := valueChange.(type) {
		case nil: // deletion
			delete(dst, keyChange)

		case DiffMap: // nested change-set
			if value == nil {
				delete(dst, keyChange)
				continue
			}

			subDst, ok := dst[keyChange].(DiffMap)
			if !ok {
				// Either key absent or not a map -> allocate once
				subDst = make(DiffMap, len(value))
				dst[keyChange] = subDst
			}
			applyRecursive(subDst, value)

		default: // scalar add / replace
			dst[keyChange] = value
		}
	}
}

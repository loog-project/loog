// Package diffmap computes the change-set that would turn map [a] into map [b].
//
// A change-set is itself a [map[string]any] that contains only the keys that
// differ. Added keys get their new value, removed keys get a nil value, and
// modified nested-maps are expressed recursively.
package diffmap

type DiffMap = map[string]any

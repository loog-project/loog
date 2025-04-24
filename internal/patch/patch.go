package patch

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wI2L/jsondiff"
)

// Diff returns a list of operations describing the changes needed to transform [oldObj] into [newObj]
func Diff(oldObj, newObj map[string]any) ([]jsondiff.Operation, error) {
	return jsondiff.Compare(oldObj, newObj)
}

// ApplyOperations applies [ops] to the given [base] map, returning the modified
// object encoded as JSON bytes (suitable for a subsequent [json.Unmarshal]).
// TODO: make this more efficient, currently we're juggling between marshaling and unmarshalling
func ApplyOperations(base map[string]any, ops []jsondiff.Operation) ([]byte, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	patchBytes, err := json.Marshal(ops)
	if err != nil {
		return nil, err
	}
	p, err := jsonpatch.DecodePatch(patchBytes)
	if err != nil {
		return nil, fmt.Errorf("cannot decode patch: %w", err)
	}
	return p.Apply(baseBytes)
}

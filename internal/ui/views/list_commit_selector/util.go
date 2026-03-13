package list_commit_selector

import (
	"fmt"
	"strings"
)

// InsertAt inserts value into the slice at index i. The slice is modified via pointer.
func InsertAt[T any](slice *[]T, i int, value T) {
	if i < 0 {
		panic(fmt.Sprintf("invalid index: %d", i))
	}
	if i > len(*slice) {
		// Automatically grow the slice with zero-values
		extra := make([]T, i-len(*slice))
		*slice = append(*slice, extra...)
	}

	*slice = append(*slice, value)     // grow by one
	copy((*slice)[i+1:], (*slice)[i:]) // shift elements
	(*slice)[i] = value                // set value
}

func Join(elems ...string) string {
	return strings.Join(elems, ":")
}

package patch

import (
	"fmt"

	gojsondiff "github.com/wI2L/jsondiff"
)

type RevisionID uint64

func NewRevisionID(u uint64) RevisionID {
	return RevisionID(u)
}

func (id RevisionID) String() string {
	return fmt.Sprintf("%016x", uint64(id))
}

type RevisionPatch struct {
	/// Revision Metadata
	// ID of the revision
	ID RevisionID `msgpack:"id"`
	// PreviousID is the ID of the previous revision.
	// This should always be set since a patch cannot exist without a previous snapshot.
	PreviousID RevisionID `msgpack:"prev_id,omitempty"`

	/// Patch Metadata
	// Operations is a list of operations that describe the changes made in this revision.
	// It is a list of JSON Patch operations.
	Operations []gojsondiff.Operation `msgpack:"ops"`
}

type RevisionSnapshot struct {
	/// Revision Metadata
	// ID of the revision
	ID RevisionID `msgpack:"id"`
	// PreviousID is the ID of the previous revision. This can be empty if this is the first revision.
	PreviousID RevisionID `msgpack:"prev_id,omitempty"`

	/// Snapshot Metadata
	// Object is the actual object being stored in this revision.
	Object map[string]any `msgpack:"obj"`
}

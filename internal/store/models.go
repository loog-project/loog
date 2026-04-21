package store

import (
	"fmt"
	"time"

	"github.com/loog-project/loog/pkg/diffmap"
)

type RevisionID uint64

func (id RevisionID) String() string {
	return fmt.Sprintf("%04x", uint64(id))
}

type Patch struct {
	// ID is the revision identifier for this patch.
	ID RevisionID `msgpack:"i" json:"ID,omitempty"`
	// PreviousID is the ID of the previous revision.
	// This should always be set since a patch cannot exist without a previous snapshot.
	PreviousID RevisionID `msgpack:"<,omitempty" json:"previousID,omitempty"`

	// Patch is the diff between the previous revision and this revision.
	// See [diffmap.Diff] for more details.
	Patch diffmap.DiffMap `msgpack:"s" json:"patch,omitempty"`
	Time  time.Time       `msgpack:"t" json:"time"`
}

type Snapshot struct {
	// ID is the revision identifier for this snapshot.
	ID RevisionID `msgpack:"i" json:"ID,omitempty"`
	// PreviousID is the ID of the previous revision. This can be empty if this is the first revision.
	PreviousID RevisionID `msgpack:"<,omitempty" json:"previousID,omitempty"`

	// Object is the full resource state stored in this revision.
	Object diffmap.DiffMap `msgpack:"o" json:"object,omitempty"`
	Time   time.Time       `msgpack:"t" json:"time"`
}

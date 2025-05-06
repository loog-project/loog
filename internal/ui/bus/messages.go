package bus

import "github.com/loog-project/loog/internal/store"

type CommitMessage struct {
	//Object   *unstructured.Unstructured
	Revision store.RevisionID

	// Object Meta
	UID, Kind, Name, Namespace string

	// it's either a snapshot OR a patch,
	// one of those must be nil, the other must be set
	Snapshot *store.Snapshot
	Patch    *store.Patch
}

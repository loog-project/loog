package bus

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/store"
)

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

type ErrorMessage struct {
	Title   string
	Message string
}

func Emit[T any](v T) tea.Cmd {
	return func() tea.Msg {
		return v
	}
}

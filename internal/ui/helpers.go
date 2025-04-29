package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/loog-project/loog/internal/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Base struct {
	Width  int
	Height int
	Theme  Theme
}

func (b *Base) SetSize(width, height int) {
	b.Width = width
	b.Height = height
}

func (b *Base) SetTheme(theme Theme) {
	b.Theme = theme
}

type pushType uint

const (
	push pushType = iota
	pop
	replace
)

type pushViewMsg struct {
	view     View
	pushType pushType
}

type tickMsg struct{}

type alertMsg struct {
	Title string
	Err   error
}

type commitMsg struct {
	Time     time.Time
	Object   *unstructured.Unstructured
	Revision store.RevisionID

	// it's either a snapshot OR a patch,
	// one of those must be nil, the other must be set
	Snapshot *store.Snapshot
	Patch    *store.Patch
}

func PushChangeView(pushType pushType, view View) tea.Cmd {
	return func() tea.Msg {
		return pushViewMsg{
			view:     view,
			pushType: pushType,
		}
	}
}

func NewAlert(title string, err error) tea.Msg {
	return alertMsg{
		Title: title,
		Err:   err,
	}
}

func PushAlert(title string, err error) tea.Cmd {
	return func() tea.Msg {
		return NewAlert(title, err)
	}
}

// NewCommitCommand creates a command that pushes a commit message to the root model.
func NewCommitCommand(
	received time.Time,
	obj *unstructured.Unstructured,
	rev store.RevisionID,
	snapshot *store.Snapshot,
	patch *store.Patch,
) tea.Msg {
	return commitMsg{
		Time:     received,
		Object:   obj,
		Revision: rev,
		Snapshot: snapshot,
		Patch:    patch,
	}
}

// ternary is a generic function that returns one of two values based on a boolean condition.
// it should be used for rendering purposes only.
func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

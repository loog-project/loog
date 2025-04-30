package ui

import (
	"time"

	"github.com/charmbracelet/bubbles/viewport"
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
	Push pushType = iota
	Pop
	Replace
)

type pushViewMsg struct {
	view     View
	pushType pushType
}

type TickMsg struct{}

type alertMsg struct {
	Title string
	Err   error
}

type CommitMsg struct {
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
	return CommitMsg{
		Time:     received,
		Object:   obj,
		Revision: rev,
		Snapshot: snapshot,
		Patch:    patch,
	}
}

func ScrollViewport(k tea.KeyMsg, vp *viewport.Model) tea.Cmd {
	switch k.String() {
	case "up", "k":
		vp.ScrollUp(1)
	case "down", "j":
		vp.ScrollDown(1)
	case "pgup":
		vp.PageUp()
	case "pgdown":
		vp.PageDown()
	case "left":
		vp.ScrollLeft(1)
	case "right":
		vp.ScrollRight(1)
	}
	return nil
}

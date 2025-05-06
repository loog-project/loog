package ui

import (
	"math"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/loog-project/loog/internal/store"
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

type tickMsg struct{}

type alertMsg struct {
	Title string
	Err   error
}

type commitMsg struct {
	//Object   *unstructured.Unstructured
	Revision store.RevisionID

	// Object Meta
	UID, Kind, Name, Namespace string

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
	uid, kind, name, namespace string,
	rev store.RevisionID,
	snapshot *store.Snapshot,
	patch *store.Patch,
) tea.Msg {
	return commitMsg{
		Revision:  rev,
		UID:       uid,
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Snapshot:  snapshot,
		Patch:     patch,
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

var CustomHumanizeMagnitudes = []humanize.RelTimeMagnitude{
	{time.Second, "initial", time.Second},
	{2 * time.Second, "1 second %s", 1},
	{time.Minute, "%d seconds %s", time.Second},
	{2 * time.Minute, "1 minute %s", 1},
	{time.Hour, "%d minutes %s", time.Minute},
	{2 * time.Hour, "1 hour %s", 1},
	{humanize.Day, "%d hours %s", time.Hour},
	{2 * humanize.Day, "1 day %s", 1},
	{humanize.Week, "%d days %s", humanize.Day},
	{2 * humanize.Week, "1 week %s", 1},
	{humanize.Month, "%d weeks %s", humanize.Week},
	{2 * humanize.Month, "1 month %s", 1},
	{humanize.Year, "%d months %s", humanize.Month},
	{18 * humanize.Month, "1 year %s", 1},
	{2 * humanize.Year, "2 years %s", 1},
	{humanize.LongTime, "%d years %s", humanize.Year},
	{math.MaxInt64, "a long while %s", 1},
}

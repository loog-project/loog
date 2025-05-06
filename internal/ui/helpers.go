package ui

import (
	"math"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"
	"github.com/loog-project/loog/internal/store"
)

// CustomHumanizeMagnitudes is a custom set of humanize time formats.
// It is used to display `initial` instead of `now`
var CustomHumanizeMagnitudes = []humanize.RelTimeMagnitude{
	{D: time.Second, Format: "initial", DivBy: time.Second},
	{D: 2 * time.Second, Format: "1 second %s", DivBy: 1},
	{D: time.Minute, Format: "%d seconds %s", DivBy: time.Second},
	{D: 2 * time.Minute, Format: "1 minute %s", DivBy: 1},
	{D: time.Hour, Format: "%d minutes %s", DivBy: time.Minute},
	{D: 2 * time.Hour, Format: "1 hour %s", DivBy: 1},
	{D: humanize.Day, Format: "%d hours %s", DivBy: time.Hour},
	{D: 2 * humanize.Day, Format: "1 day %s", DivBy: 1},
	{D: humanize.Week, Format: "%d days %s", DivBy: humanize.Day},
	{D: 2 * humanize.Week, Format: "1 week %s", DivBy: 1},
	{D: humanize.Month, Format: "%d weeks %s", DivBy: humanize.Week},
	{D: 2 * humanize.Month, Format: "1 month %s", DivBy: 1},
	{D: humanize.Year, Format: "%d months %s", DivBy: humanize.Month},
	{D: 18 * humanize.Month, Format: "1 year %s", DivBy: 1},
	{D: 2 * humanize.Year, Format: "2 years %s", DivBy: 1},
	{D: humanize.LongTime, Format: "%d years %s", DivBy: humanize.Year},
	{D: math.MaxInt64, Format: "a long while %s", DivBy: 1},
}

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

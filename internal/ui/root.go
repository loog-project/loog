package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/loog-project/loog/internal/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _, _ = PushView, PopView

// View is the interface that all views must implement.
type View interface {
	Sizer

	Update(tea.Msg) (View, tea.Cmd)
	View() string
	KeyMap() string
	Breadcrumb() string
}

type Sizer interface {
	SetSize(width, height int)
}

type Size struct {
	Width  int
	Height int
}

type EventQueueOriginator interface {
	// OriginatesFromEventQueue returns true if the message originates from the event queue.
	// This is used to determine if the message should be requeued.
	OriginatesFromEventQueue() bool
}

/// Root Model

type tickMsg struct {
}

type alertMsg struct {
	Title string
	Err   error
}

// OriginatesFromEventQueue returns indicates that the message should be requeued
func (a alertMsg) OriginatesFromEventQueue() bool {
	return true
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

func (c commitMsg) OriginatesFromEventQueue() bool {
	return true
}

type pushViewMsg struct {
	View View
	Pop  bool
}

func (p pushViewMsg) OriginatesFromEventQueue() bool {
	return true
}

func (w *Size) SetSize(width, height int) {
	w.Width = width
	w.Height = height
}

type Root struct {
	Size

	ViewStack []View

	AlertTitle string
	AlertErr   error
}

func NewRoot(first View) *Root {
	root := &Root{
		ViewStack: []View{first},
	}
	return root
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (r Root) Init() tea.Cmd {
	return tea.Batch(tick())
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch v := msg.(type) {
	case pushViewMsg:
		// push view message:
		if v.Pop {
			if len(r.ViewStack) <= 1 {
				// can't pop the root view
				return r, nil
			}

			r.ViewStack = r.ViewStack[:len(r.ViewStack)-1]
		} else {
			// set the size of the new view
			v.View.SetSize(r.Width, r.Height)

			r.ViewStack = append(r.ViewStack, v.View)
		}
		return r, tea.Batch(cmds...)

	case alertMsg:
		// alert message:
		// displays an error message in the center of the screen

		r.AlertTitle = v.Title
		r.AlertErr = v.Err
		return r, tea.Batch(cmds...)

	case tickMsg:
		// tick message:
		// used to trigger a tick event

		cmds = append(cmds, tick())

	case tea.WindowSizeMsg:
		// window size message:
		// used to resize the views

		width, height := v.Width, v.Height-2 // -2 for the status bar

		// propagate the size to all views
		for i := range r.ViewStack {
			r.ViewStack[i].SetSize(width, height)
		}

		// also track the size in the root model so we can use it in the view
		r.SetSize(width, height)

	// still propagate the message as a view might need raw access to the window size

	case tea.KeyMsg:
		if v.String() == "esc" {
			if r.AlertErr != nil {
				r.AlertErr = nil
				r.AlertTitle = ""
				return r, tea.Batch(cmds...)
			}
		}
	}

	// propagate the message to the top view
	i := len(r.ViewStack) - 1

	var cmd tea.Cmd
	r.ViewStack[i], cmd = r.ViewStack[i].Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return r, tea.Batch(cmds...)
}

func (r Root) View() string {
	if r.Size.Height == 0 && r.Size.Width == 0 {
		return "" // no size yet
	}

	ui := r.ViewStack[len(r.ViewStack)-1].View()
	help := r.ViewStack[len(r.ViewStack)-1].KeyMap()

	breadcrumbStack := make([]string, 0, len(r.ViewStack))
	for _, view := range r.ViewStack {
		breadcrumbStack = append(breadcrumbStack, view.Breadcrumb())
	}

	// place error box on top
	if r.AlertErr != nil {
		w, h := r.Width-2, r.Height-1
		ui = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("#ff5f5f")).
			Width(w).
			Height(h).
			Render(lipgloss.PlaceVertical(h, lipgloss.Center,
				lipgloss.PlaceHorizontal(w, lipgloss.Center,
					fmt.Sprintf("%s\n\n%s\n(%s)",
						StyleDim.Render("AN ERROR OCCURRED:"),
						StyleError.Render(r.AlertErr.Error()),
						r.AlertTitle))))
		help = "[esc] to dismiss"

		breadcrumbStack = append(breadcrumbStack, "<error>")
	}

	bar := fmt.Sprintf("%s | %s",
		BarBreadcrumbs.Render(strings.Join(breadcrumbStack, " » ")),
		StyleDim.Render(help))

	return ui + "\n" + bar
}

func PushView(view View) tea.Cmd {
	return func() tea.Msg {
		return pushViewMsg{
			View: view,
			Pop:  false,
		}
	}
}

func PopView() tea.Cmd {
	return func() tea.Msg {
		return pushViewMsg{
			Pop: true,
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

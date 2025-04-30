package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Baser interface {
	SetSize(width, height int)
	SetTheme(theme Theme)
}

// View is the interface that all views must implement.
type View interface {
	Baser

	Update(tea.Msg) (View, tea.Cmd)
	View() string
	KeyMap() string
	Breadcrumb() string
}

/// Root Model

type Root struct {
	Width, Height int
	Theme         Theme

	ViewStack    []View
	ShuttingDown bool

	Logger *UILogger

	AlertTitle string
	AlertErr   error
}

func NewRoot(theme Theme, logger *UILogger, first View) *Root {
	r := &Root{
		Theme:  theme,
		Logger: logger,
	}
	r.applyTo(first)
	r.ViewStack = []View{first}
	return r
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (r Root) Init() tea.Cmd {
	return tea.Batch(tick())
}

func (r Root) applyTo(v View) View {
	v.SetSize(r.Width, r.Height)
	v.SetTheme(r.Theme)
	return v
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0)

	switch v := msg.(type) {
	case pushViewMsg:
		switch v.pushType {
		case Push:
			r.ViewStack = append(r.ViewStack, r.applyTo(v.view))
		case Replace:
			r.ViewStack[len(r.ViewStack)-1] = r.applyTo(v.view)
		case Pop:
			if len(r.ViewStack) <= 1 {
				// can't pop the root view
				return r, nil
			}
			r.ViewStack = r.ViewStack[:len(r.ViewStack)-1]
		}
		return r, tea.Batch(cmds...)

	case alertMsg:
		// alert message:
		// displays an error message in the center of the screen

		r.AlertTitle = v.Title
		r.AlertErr = v.Err
		return r, tea.Batch(cmds...)

	case TickMsg:
		cmds = append(cmds, tick())

	case tea.WindowSizeMsg:
		// window size message:
		// used to resize the views

		r.Width = v.Width
		r.Height = v.Height - 2

		// propagate the size to all views
		for i := range r.ViewStack {
			r.ViewStack[i].SetSize(r.Width, r.Height)
		}

	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "q":
			// TODO(future): remove `q` from here
			r.ShuttingDown = true
			return r, tea.Quit
		case "L":
			return r, PushChangeView(Push, NewLogView(r.Logger))
		case "esc":
			// TODO(future): alert should be its own view
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
	if r.Height == 0 && r.Width == 0 {
		return "" // no size yet
	}
	if r.ShuttingDown {
		// this makes sure the whole screen won't show in the terminal after quitting
		return r.Theme.MutedTextStyle.Render("Bye!")
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

		ui = lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, r.Theme.AlertContainerStyle.Render(
			fmt.Sprintf("%s\n\n%s\n(%s)",
				r.Theme.MutedTextStyle.Render("AN ERROR OCCURRED:"),
				r.Theme.ErrorTextStyle.Render(r.AlertErr.Error()),
				r.AlertTitle,
			),
		))

		help = "[esc] to dismiss"

		breadcrumbStack = append(breadcrumbStack, "<error>")
	}

	bar := fmt.Sprintf("%s | %s",
		r.Theme.BreadcrumbBarStyle.Render(strings.Join(breadcrumbStack, " » ")),
		r.Theme.MutedTextStyle.Render(help))

	return ui + "\n" + bar
}

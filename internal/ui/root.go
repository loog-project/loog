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

// isViewOpen checks if the view of type T is open in the stack.
func isViewOpen[T View](r Root) bool {
	if len(r.ViewStack) == 0 {
		return false
	}
	_, isOpen := r.ViewStack[len(r.ViewStack)-1].(T)
	return isOpen
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
		return r, nil

	case alertMsg:
		pt := Push
		if isViewOpen[*AlertView](r) {
			pt = Replace
		}
		view := &AlertView{
			Title: v.Title,
			Err:   v.Err,
		}
		return r, PushChangeView(pt, view)

	case TickMsg:
		cmds = append(cmds, tick())

	case tea.WindowSizeMsg:
		// window size message:
		// used to resize the views

		r.Width = v.Width
		r.Height = v.Height - 1 // -1 for the status bar

		// propagate the size to all views
		for i := range r.ViewStack {
			r.ViewStack[i].SetSize(r.Width, r.Height)
		}

	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "q":
			// TODO(future): remove `q` from here as views should handle it
			r.ShuttingDown = true
			return r, tea.Quit
		case "L":
			if isViewOpen[*LogView](r) {
				return r, PushChangeView(Pop, nil)
			}
			return r, PushChangeView(Push, NewLogView(r.Logger))
		case "E":
			return r, PushAlert("Test", fmt.Errorf("This is a test!"))
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

func (r Root) renderBar(breadcrumbs string, help string) string {
	breadcrumbsRender := r.Theme.BreadcrumbBarStyle.Render(breadcrumbs)

	var logRender string
	if r.Logger != nil {
		info, warn, err := r.Logger.peekUnread(false)

		if info+warn+err > 0 {
			// TODO(future): nicer format
			logRender = r.Theme.LoggerBarStyle.Render(fmt.Sprintf("🔔(%di|%dw|%de)",
				info, warn, err))
		} else {
			logRender = ""
		}
	}

	helpRender := r.Theme.HelpBarStyle.
		Width(r.Width - lipgloss.Width(breadcrumbsRender) - lipgloss.Width(logRender)).
		Render(help)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		helpRender,
		breadcrumbsRender,
		logRender,
	)
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
	breadcrumbs := strings.Join(breadcrumbStack, " ⟩ ")

	bar := fmt.Sprintf("%s | %s",
		r.Theme.BreadcrumbBarStyle.Render(breadcrumbs),
		r.Theme.MutedTextStyle.Render(help))
	_ = bar

	return ui + "\n" + r.renderBar(breadcrumbs, help) // + bar
}

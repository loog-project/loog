package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AlertView struct {
	Base

	Title string
	Err   error
}

var _ View = (*AlertView)(nil)

func (av *AlertView) View() string {
	return lipgloss.Place(av.Width, av.Height, lipgloss.Center, lipgloss.Center,
		av.Theme.AlertDialogContainerStyle.Render(fmt.Sprintf("%s\n\n%s\n(%s)",
			av.Theme.MutedTextStyle.Render("AN ERROR OCCURRED:"),
			av.Theme.ErrorTextStyle.Render(av.Err.Error()),
			av.Title,
		)))
}

func (av *AlertView) KeyMap() string {
	return NewShortcuts("esc", "close").Render(av.Theme)
}

func (av *AlertView) Breadcrumb() string {
	return "Error (" + av.Title + ")"
}

func (av *AlertView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d", "ctrl+q", "ctrl+z", "esc":
			return av, PushChangeView(Pop, nil)
		}
	}
	return av, nil
}

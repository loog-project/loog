package main

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/layouts"
	"github.com/loog-project/loog/internal/ui/theme"
)

type root struct {
	left   core.View
	right  core.View
	layout *layouts.SplitLayout
}

func (r *root) Init() tea.Cmd {
	return r.layout.Init()
}

func (r *root) Update(msg tea.Msg) (core.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "[", "]":
			r.layout.ToggleOrientation()
		case "-":
			r.layout.Decrease()
		case "=", "+":
			r.layout.Increase()
		}
	}
	newLayout, cmd := r.layout.Update(msg)
	r.layout = newLayout.(*layouts.SplitLayout)
	return r, cmd
}

func (r *root) View() string {
	return r.layout.View()
}

func (r *root) SetSize(width, height int) {
	r.layout.SetSize(width, height)
}

var _ core.View = (*root)(nil)

func main() {
	left := layouts.NewBorderLayout(core.Primitive("Left View"), theme.DarkTheme.BorderActiveContainerStyle)
	right := layouts.NewBorderLayout(core.Primitive("Right View"), theme.DarkTheme.BorderIdleContainerStyle)
	layout := layouts.NewSplitLayoutWithFraction(layouts.Horizontal, left, right, 0.5)

	r := &root{
		left:   left,
		right:  right,
		layout: layout,
	}

	app := core.NewApp(r, theme.DarkTheme)

	p := tea.NewProgram(app) //, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}

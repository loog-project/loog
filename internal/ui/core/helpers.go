package core

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/theme"
)

// Noop is a no-op command. It can be used as a placeholder for initialization commands or when no command is needed.
var Noop tea.Cmd = nil

type Sizeable interface {
	SetSize(width, height int)
}

type Themeable interface {
	SetTheme(theme theme.Theme)
}

type Focusable interface {
	SetFocus(active bool)
	HasFocus() bool
}

type Sizer struct {
	Width  int
	Height int
}

func (s *Sizer) SetSize(width, height int) {
	s.Width = width
	s.Height = height
}

type Themer struct {
	Theme theme.Theme
}

func (t *Themer) SetTheme(theme theme.Theme) {
	t.Theme = theme
}

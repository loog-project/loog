package layouts

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type StatusBarLayout struct {
	// content contains the "main" content of the view
	content core.View

	// statusBar contains the status bar of the view
	statusBar core.View

	width, height int
}

var _ Layout = (*StatusBarLayout)(nil)

func NewStatusBarLayout(content core.View, statusBar core.View) *StatusBarLayout {
	return &StatusBarLayout{
		statusBar: statusBar,
		content:   content,
	}
}

func (l *StatusBarLayout) Init() tea.Cmd {
	return tea.Batch(l.content.Init(), l.statusBar.Init())
}

func (l *StatusBarLayout) Update(msg tea.Msg) (core.View, tea.Cmd) {
	var commands []tea.Cmd
	if newView, cmd := l.content.Update(msg); newView != nil {
		l.content, commands = newView, append(commands, cmd)
	}
	if newView, cmd := l.statusBar.Update(msg); newView != nil {
		l.statusBar, commands = newView, append(commands, cmd)
	}
	return l, tea.Batch(commands...)
}

func (l *StatusBarLayout) View() string {
	return l.content.View() + "\n" + l.statusBar.View()
}

/// Dispatchers

// SetSize sets the size of the layout and its components
// it is overridden to set the size of the content and status bar
func (l *StatusBarLayout) SetSize(width, height int) {
	l.width, l.height = width, height

	// dispatch the size to the content and status bar
	if s, ok := l.content.(core.Sizeable); ok {
		h := l.height - 1 // -1 for status bar
		if h < 0 {
			h = 0
		}
		s.SetSize(l.width, h)
	}
	if s, ok := l.statusBar.(core.Sizeable); ok {
		s.SetSize(l.width, 1) // status bar is always 1 line
	}
}

// SetTheme sets the theme of the layout and its components
// it is overridden to set the theme of the content and status bar
func (l *StatusBarLayout) SetTheme(theme theme.Theme) {
	// dispatch the theme to the content and status bar
	dispatchTheme(theme, l.content, l.statusBar)
}

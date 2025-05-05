package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type BorderedLayout struct {
	// content contains the "main" content of the view
	content core.View

	// width and height of the inner content
	width, height int

	borderStyle lipgloss.Style
}

var _ Layout = (*BorderedLayout)(nil)

func NewBorderedLayout(content core.View, borderStyle lipgloss.Style) *BorderedLayout {
	return &BorderedLayout{
		content:     content,
		borderStyle: borderStyle,
	}
}

func (l *BorderedLayout) Init() tea.Cmd {
	return l.content.Init()
}

func (l *BorderedLayout) Update(msg tea.Msg) (core.View, tea.Cmd) {
	newView, cmd := l.content.Update(msg)
	l.content = newView
	return l, cmd
}

func (l *BorderedLayout) View() string {
	return l.borderStyle.
		Width(l.width - 2).   // -2 for border
		Height(l.height - 2). // -2 for border
		Render(l.content.View())
}

/// Dispatchers

func (l *BorderedLayout) dispatchSize() {
	if s, ok := l.content.(core.Sizeable); ok {
		s.SetSize(l.width-2, l.height-2) // -2 for border
	}
}

// SetSize sets the size of the layout and its components
func (l *BorderedLayout) SetSize(width, height int) {
	l.width, l.height = width, height
	l.dispatchSize()
}

// SetTheme sets the theme of the layout and its components
func (l *BorderedLayout) SetTheme(theme theme.Theme) {
	// dispatch the theme to the content
	dispatchTheme(theme, l.content)
}

/// Mutators

func (l *BorderedLayout) SetContent(content core.View) {
	l.content = content
	l.dispatchSize()
}

func (l *BorderedLayout) SetBorderStyle(borderStyle lipgloss.Style) {
	l.borderStyle = borderStyle
}

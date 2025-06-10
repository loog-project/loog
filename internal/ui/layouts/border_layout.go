package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type BorderLayout struct {
	// content contains the "main" content of the view
	content core.View

	// width and height of the inner content
	width, height int

	borderStyle lipgloss.Style
	borderWidth int
}

var _ Layout = (*BorderLayout)(nil)

func NewBorderLayout(content core.View, borderStyle lipgloss.Style) *BorderLayout {
	b := &BorderLayout{
		content: content,
	}
	b.SetBorderStyle(borderStyle)
	return b
}

func (l *BorderLayout) Init() tea.Cmd {
	return l.content.Init()
}

func (l *BorderLayout) Update(msg tea.Msg) (core.View, tea.Cmd) {
	newView, cmd := l.content.Update(msg)
	l.content = newView
	return l, cmd
}

func (l *BorderLayout) View() string {
	return l.borderStyle.
		Width(l.width - l.borderWidth).
		Height(l.height - l.borderWidth).
		Render(l.content.View())
}

func (l *BorderLayout) dispatchSize() {
	if s, ok := l.content.(core.Sizeable); ok {
		s.SetSize(
			l.width-l.borderWidth,
			l.height-l.borderWidth,
		)
	}
}

// SetSize sets the size of the layout and its components
func (l *BorderLayout) SetSize(width, height int) {
	l.width, l.height = width, height
	l.dispatchSize()
}

// SetTheme sets the theme of the layout and its components
func (l *BorderLayout) SetTheme(theme theme.Theme) {
	// dispatch the theme to the content
	dispatchTheme(theme, l.content)
}

/// Mutators

func (l *BorderLayout) SetContent(content core.View) {
	l.content = content
	l.dispatchSize()
}

func (l *BorderLayout) SetBorderStyle(borderStyle lipgloss.Style) {
	// dummy render empty content to calculate the actual border width
	borderWidth := lipgloss.Width(borderStyle.Render(""))

	l.borderStyle = borderStyle
	l.borderWidth = borderWidth
}

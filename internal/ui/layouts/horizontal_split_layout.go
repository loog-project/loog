package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type HorizontalSplitLayout struct {
	width, height int

	left, right core.View

	// fraction element (0, 1), for example 0.4 means 40% of the screen will be used by the left view
	fraction float64

	leftFocused  bool
	rightFocused bool
}

var _ Layout = (*HorizontalSplitLayout)(nil)

func NewHorizontalSplitLayout(left, right core.View, fraction float64) *HorizontalSplitLayout {
	if fraction < 0 || fraction > 1 {
		fraction = 0.5
	}
	return &HorizontalSplitLayout{
		left:     left,
		right:    right,
		fraction: fraction,
	}
}

func (l *HorizontalSplitLayout) Init() tea.Cmd {
	return tea.Batch(l.left.Init(), l.right.Init())
}

func (l *HorizontalSplitLayout) Update(msg tea.Msg) (core.View, tea.Cmd) {
	var commands []tea.Cmd
	if newView, cmd := l.left.Update(msg); newView != nil {
		l.left, commands = newView, append(commands, cmd)
	}
	if newView, cmd := l.right.Update(msg); newView != nil {
		l.right, commands = newView, append(commands, cmd)
	}
	return l, tea.Batch(commands...)
}

func (l *HorizontalSplitLayout) View() string {
	leftWidth := int(float64(l.width) * l.fraction)

	// if the left view is empty, just return right view
	if leftWidth == 0 {
		return l.right.View()
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, l.left.View(), l.right.View())
}

/// Dispatchers

func (l *HorizontalSplitLayout) dispatchSizes() {
	leftWidth := int(float64(l.width) * l.fraction)
	rightWidth := l.width - leftWidth

	// dispatch the size to the left and right views
	if s, ok := l.left.(core.Sizeable); ok {
		s.SetSize(leftWidth, l.height)
	}
	if s, ok := l.right.(core.Sizeable); ok {
		s.SetSize(rightWidth, l.height)
	}
}

// SetSize sets the size of the layout and its components
func (l *HorizontalSplitLayout) SetSize(width, height int) {
	l.width, l.height = width, height
	l.dispatchSizes()
}

// SetTheme sets the theme of the layout and its components
func (l *HorizontalSplitLayout) SetTheme(theme theme.Theme) {
	// dispatch the theme to the left and right views
	dispatchTheme(theme, l.left, l.right)
}

/// Mutators

func (l *HorizontalSplitLayout) SetFraction(fraction float64) {
	if fraction < 0 || fraction > 1 {
		fraction = 0.5
	}
	l.fraction = fraction
	l.dispatchSizes()
}

// Increase increases the fraction of the left view by 1/width
func (l *HorizontalSplitLayout) Increase() {
	l.SetFraction(l.fraction + (1.0 / float64(l.width)))
}

// Decrease decreases the fraction of the left view by 1/width
func (l *HorizontalSplitLayout) Decrease() {
	l.SetFraction(l.fraction - (1.0 / float64(l.width)))
}

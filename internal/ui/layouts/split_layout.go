package layouts

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
	"github.com/loog-project/loog/internal/util"
)

type Orientation uint8

const (
	Horizontal Orientation = iota
	Vertical
)

// SplitLayout is a layout that splits the screen into two views, either horizontally or vertically.
// It takes a fraction (0 to 1) to determine how much space each view should take.
// The left view will take the fraction of the screen, and the right view will take the remaining space.
// For example, if the fraction is 0.4, the left view will take 40% of the screen and the right view will take 60%.
//
// Horizontal:
//
//	+-----------------+------------------+
//	| Left View (50%) | Right View (50%) |
//	+-----------------+------------------+
//
// Vertical:
//
//	+-----------------+
//	| Left View (50%) |
//	+-----------------+
//	| Right View (50%)|
//	+-----------------+
type SplitLayout struct {
	width, height int

	left, right core.View

	orientation Orientation

	// fraction element (0, 1), for example 0.4 means 40% of the screen will be used by the left view
	fraction float64

	// fixedLeftSize and fixedRightSize can be used to set a specific size for the left and right views.
	// This will override the fraction. Set to 0 to fill the remaining space.
	fixedLeftSize, fixedRightSize uint
}

var _ Layout = (*SplitLayout)(nil)

// NewSplitLayoutWithFraction creates a new SplitLayout with the given orientation, left and right views, and fraction.
// If the fraction is not between 0 and 1, it will default to 0.5 (50%).
func NewSplitLayoutWithFraction(orientation Orientation, left, right core.View, fraction float64) *SplitLayout {
	if fraction < 0 || fraction > 1 {
		fraction = 0.5
	}
	return &SplitLayout{
		orientation: orientation,
		left:        left,
		right:       right,
		fraction:    fraction,
	}
}

// NewSplitLayoutWithFixedSize creates a new SplitLayout with the given orientation, left and right views, and a fixed size for the left view.
// The fixedLeftSize will override the fraction, meaning the left view will always have this size.
func NewSplitLayoutWithFixedSize(orientation Orientation, left, right core.View, fixedLeftSize, fixedRightSide uint) *SplitLayout {
	if fixedLeftSize == 0 && fixedRightSide == 0 {
		fixedRightSide = 1 // default to a status bar layout
	}
	return &SplitLayout{
		orientation:    orientation,
		left:           left,
		right:          right,
		fixedLeftSize:  fixedLeftSize,
		fixedRightSize: fixedRightSide,
	}
}

func (l *SplitLayout) Init() tea.Cmd {
	return tea.Batch(l.left.Init(), l.right.Init())
}

func (l *SplitLayout) Update(msg tea.Msg) (core.View, tea.Cmd) {
	var commands []tea.Cmd
	if newView, cmd := l.left.Update(msg); newView != nil {
		l.left, commands = newView, append(commands, cmd)
	}
	if newView, cmd := l.right.Update(msg); newView != nil {
		l.right, commands = newView, append(commands, cmd)
	}
	return l, tea.Batch(commands...)
}

func (l *SplitLayout) View() string {
	if l.fixedLeftSize == 0 && l.fixedRightSize == 0 {
		if int(l.fraction*float64(l.height)) == 0 {
			// if the fraction is 0, just return the right view
			return l.right.View()
		}
	}
	switch l.orientation {
	case Horizontal:
		return lipgloss.JoinHorizontal(lipgloss.Top, l.left.View(), l.right.View())
	case Vertical:
		return lipgloss.JoinVertical(lipgloss.Left, l.left.View(), l.right.View())
	}
	return ""
}

func (l *SplitLayout) dispatchSizes() {
	var leftWidth, leftHeight, rightWidth, rightHeight int

	switch l.orientation {
	case Horizontal:
		leftWidth, leftHeight = int(float64(l.width)*l.fraction), l.height
		rightWidth, rightHeight = l.width-leftWidth, l.height
	case Vertical:
		leftWidth, leftHeight = l.width, int(float64(l.height)*l.fraction)
		rightWidth, rightHeight = l.width, l.height-leftHeight
	}

	// dispatch the size to the left and right views
	if s, ok := l.left.(core.Sizeable); ok {
		s.SetSize(leftWidth, leftHeight)
	}
	if s, ok := l.right.(core.Sizeable); ok {
		s.SetSize(rightWidth, rightHeight)
	}
}

// SetSize sets the size of the layout and its components
func (l *SplitLayout) SetSize(width, height int) {
	l.width, l.height = width, height
	l.dispatchSizes()
}

// SetTheme sets the theme of the layout and its components
func (l *SplitLayout) SetTheme(theme theme.Theme) {
	// dispatch the theme to the left and right views
	dispatchTheme(theme, l.left, l.right)
}

func (l *SplitLayout) SetFraction(fraction float64) {
	l.fraction = util.RangeOrElse(fraction, 0, 1, 0.5)
	l.dispatchSizes()
}

// Increase increases the fraction of the left view by 1/width
func (l *SplitLayout) Increase() {
	l.SetFraction(l.fraction + (1.0 / float64(l.width)))
}

// Decrease decreases the fraction of the left view by 1/width
func (l *SplitLayout) Decrease() {
	l.SetFraction(l.fraction - (1.0 / float64(l.width)))
}

// ToggleOrientation toggles the orientation of the layout between horizontal and vertical
func (l *SplitLayout) ToggleOrientation() {
	if l.orientation == Horizontal {
		l.orientation = Vertical
	} else {
		l.orientation = Horizontal
	}
	l.dispatchSizes()
}

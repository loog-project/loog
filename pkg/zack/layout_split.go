package zack

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
)

type SplitOrientation uint8

const (
	Horizontal SplitOrientation = iota
	Vertical
)

// the SplitLayoutModel should implement the Boundable interface
// as it should resize the children based on the layout bounds
var _ Boundable = (*SplitLayoutModel)(nil)

// SplitLayoutModel is a layout that splits the screen into two views, either horizontally or vertically.
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
type SplitLayoutModel struct {
	orientation SplitOrientation

	leftChildBounds    Bounds
	leftChildBoundable Boundable

	rightChildBounds    Bounds
	rightChildBoundable Boundable

	layoutBounds Bounds

	// fraction element (0, 1), for example 0.4 means 40% of the screen will be used by the left view
	fraction float64

	// fixedLeftSize and fixedRightSize can be used to set a specific size for the left and right views.
	// This will override the fraction. Set to 0 to fill the remaining space.
	fixedLeftSize, fixedRightSize uint
}

// NewSplitLayoutWithFraction creates a new SplitLayoutModel with the given orientation.
// If the fraction is not between 0 and 1, it will default to 0.5 (50%).
func NewSplitLayoutWithFraction(orientation SplitOrientation, fraction float64) *SplitLayoutModel {
	if fraction < 0 || fraction > 1 {
		fraction = 0.5
	}
	return &SplitLayoutModel{
		orientation: orientation,
		fraction:    fraction,
	}
}

// NewSplitLayoutWithFixedSize creates a new SplitLayoutModel with the given orientation.
// [fixedLeftSize] will override the fraction, meaning the "left" (or top) view will always have this size.
func NewSplitLayoutWithFixedSize(
	orientation SplitOrientation,
	fixedLeftSize, fixedRightSide uint,
) *SplitLayoutModel {
	if fixedLeftSize == 0 && fixedRightSide == 0 {
		fixedRightSide = 1 // default to a status bar layout
	}
	return &SplitLayoutModel{
		orientation:    orientation,
		fixedLeftSize:  fixedLeftSize,
		fixedRightSize: fixedRightSide,
	}
}

func (m *SplitLayoutModel) Render(leftContent, rightContent string) string {
	if !m.layoutBounds.IsValid() {
		return "waiting for layout bounds to be set"
	}

	switch m.orientation {
	case Horizontal:
		if m.leftChildBounds.Width <= 0 {
			// if the left view has no width, return the right view only (fullscreen-ish)
			return rightContent
		}
		if m.rightChildBounds.Width <= 0 {
			// if the right view has no width, return the left view only (fullscreen-ish)
			return leftContent
		}
		return lipgloss.JoinHorizontal(lipgloss.Top,
			renderWithBounds(&m.leftChildBounds, leftContent),
			renderWithBounds(&m.rightChildBounds, rightContent))

	case Vertical:
		if m.leftChildBounds.Height <= 0 {
			return rightContent
		}
		if m.rightChildBounds.Height <= 0 {
			return leftContent
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			renderWithBounds(&m.leftChildBounds, leftContent),
			renderWithBounds(&m.rightChildBounds, rightContent))
	}

	log.Warn().Msgf("(SplitLayout.Render) Invalid SplitLayoutModel orientation: %v", m.orientation)
	return ""
}

// SetBounds sets the bounds for the left and right child views based on the parent bounds.
func (m *SplitLayoutModel) SetBounds(bounds Bounds) {
	m.layoutBounds = bounds

	switch m.orientation {
	case Horizontal:
		var leftWidth, rightWidth int
		if m.fixedLeftSize > 0 {
			leftWidth = int(m.fixedLeftSize)
			rightWidth = bounds.Width - leftWidth
		} else if m.fixedRightSize > 0 {
			rightWidth = int(m.fixedRightSize)
			leftWidth = bounds.Width - rightWidth
		} else {
			leftWidth = int(float64(bounds.Width) * m.fraction)
			rightWidth = bounds.Width - leftWidth
		}
		m.setLeftChildBounds(NewBounds(leftWidth, bounds.Height))
		m.setRightChildBounds(NewBounds(rightWidth, bounds.Height))
		return

	case Vertical:
		var leftHeight, rightHeight int
		if m.fixedLeftSize > 0 {
			leftHeight = int(m.fixedLeftSize)
			rightHeight = bounds.Height - leftHeight
		} else if m.fixedRightSize > 0 {
			rightHeight = int(m.fixedRightSize)
			leftHeight = bounds.Height - rightHeight
		} else {
			leftHeight = int(float64(bounds.Height) * m.fraction)
			rightHeight = bounds.Height - leftHeight
		}
		m.setLeftChildBounds(NewBounds(bounds.Width, leftHeight))
		m.setRightChildBounds(NewBounds(bounds.Width, rightHeight))
		return
	}

	log.Warn().Msgf("(SplitLayout.SetBounds) Invalid SplitLayoutModel orientation: %v", m.orientation)
}

// refreshBounds send the newest bounds to the left and right child boundables.
// This is useful when the _layout_ itself changes, e.g. when you increase or decrease the fraction,
func (m *SplitLayoutModel) refreshBounds() {
	m.SetBounds(m.layoutBounds)
}

// setLeftChildBounds sets the bounds for the left child view and updates its boundable.
func (m *SplitLayoutModel) setLeftChildBounds(bounds Bounds) {
	m.leftChildBounds = bounds
	if !bounds.IsValid() {
		log.Warn().Msgf("(SplitLayout.setLeftChildBounds) left child bounds are invalid: %v", bounds)
		return
	}
	if m.leftChildBoundable != nil {
		m.leftChildBoundable.SetBounds(bounds)
	}
}

// setRightChildBounds sets the bounds for the right child view and updates its boundable.
func (m *SplitLayoutModel) setRightChildBounds(bounds Bounds) {
	m.rightChildBounds = bounds
	if !bounds.IsValid() {
		log.Warn().Msgf("(SplitLayout.setRightChildBounds) right child bounds are invalid: %v", bounds)
		return
	}
	if m.rightChildBoundable != nil {
		m.rightChildBoundable.SetBounds(bounds)
	}
}

func (m *SplitLayoutModel) SetFraction(fraction float64) {
	m.fraction = Clamp(fraction, 0, 1)
	m.refreshBounds()
}

// Increase increases the fraction of the left view by 1/width
func (m *SplitLayoutModel) Increase() {
	switch m.orientation {
	case Horizontal:
		m.SetFraction(m.fraction + (1.0 / float64(m.layoutBounds.Width)))
	case Vertical:
		m.SetFraction(m.fraction + (1.0 / float64(m.layoutBounds.Height)))
	default:
		log.Warn().Msgf("(SplitLayout.Increase) Invalid SplitLayoutModel orientation: %v", m.orientation)
	}
}

// Decrease decreases the fraction of the left view by 1/width
func (m *SplitLayoutModel) Decrease() {
	switch m.orientation {
	case Horizontal:
		m.SetFraction(m.fraction - (1.0 / float64(m.layoutBounds.Width)))
	case Vertical:
		m.SetFraction(m.fraction - (1.0 / float64(m.layoutBounds.Height)))
	default:
		log.Warn().Msgf("(SplitLayout.Decrease) Invalid SplitLayoutModel orientation: %v", m.orientation)
	}
}

// ToggleOrientation toggles the orientation of the layout between horizontal and vertical
func (m *SplitLayoutModel) ToggleOrientation() {
	if m.orientation == Horizontal {
		m.orientation = Vertical
	} else {
		m.orientation = Horizontal
	}
	// we need to refresh the bounds here since the orientation changes -> the bounds of the children will change
	m.refreshBounds()
}

// AttachLeftBoundable attaches a boundable to the left side of the split layout
func (m *SplitLayoutModel) AttachLeftBoundable(boundable Boundable) {
	if boundable == nil {
		log.Warn().Msgf("(SplitLayoutModel.AttachLeftBoundable) boundable is nil, cannot attach")
		return
	}
	m.leftChildBoundable = boundable
}

// AttachRightBoundable attaches a boundable to the right side of the split layout
func (m *SplitLayoutModel) AttachRightBoundable(boundable Boundable) {
	if boundable == nil {
		log.Warn().Msgf("(SplitLayoutModel.AttachRightBoundable) boundable is nil, cannot attach")
		return
	}
	m.rightChildBoundable = boundable
}

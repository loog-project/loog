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
// The left view will take the fraction of the screen, and the end view will take the remaining space.
// For example, if the fraction is 0.4, the left view will take 40% of the screen and the end view will take 60%.
//
// Horizontal:
//
//	+------------------+------------------+
//	| Start View (50%) | End View (50%)   |
//	+------------------+------------------+
//
// Vertical:
//
//	+------------------+
//	| Start View (50%) |
//	+------------------+
//	| End View (50%)   |
//	+------------------+
type SplitLayoutModel struct {
	orientation SplitOrientation

	startChildBounds    Bounds
	startChildBoundable Boundable

	endChildBounds    Bounds
	endChildBoundable Boundable

	layoutBounds Bounds

	// fraction element (0, 1), for example 0.4 means 40% of the screen will be used by the start view
	fraction float64

	// fixedStartSize and fixedEndSize can be used to set a specific size for the start and end views.
	// This will override the fraction. Set to 0 to fill the remaining space.
	fixedStartSize, fixedEndSize uint
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
// [fixedStartSize] will override the fraction, meaning the "start" (or top) view will always have this size.
//
// You can set either [fixedStartSize] or [fixedEndSide] to a non-zero value.
// If both are set, [fixedStartSize] will take precedence.
func NewSplitLayoutWithFixedSize(
	orientation SplitOrientation,
	fixedStartSize, fixedEndSide uint,
) *SplitLayoutModel {
	if fixedStartSize == 0 && fixedEndSide == 0 {
		fixedEndSide = 1 // default to a status bar layout
	}
	return &SplitLayoutModel{
		orientation:    orientation,
		fixedStartSize: fixedStartSize,
		fixedEndSize:   fixedEndSide,
	}
}

func (m *SplitLayoutModel) Render(startContent, endContent string) string {
	if !m.layoutBounds.IsValid() {
		return "waiting for layout bounds to be set"
	}

	switch m.orientation {
	case Horizontal:
		if m.startChildBounds.Width <= 0 {
			// if the start view has no width, return the end view only (fullscreen-ish)
			return endContent
		}
		if m.endChildBounds.Width <= 0 {
			// if the end view has no width, return the start view only (fullscreen-ish)
			return startContent
		}
		return lipgloss.JoinHorizontal(lipgloss.Top,
			renderWithBounds(&m.startChildBounds, startContent),
			renderWithBounds(&m.endChildBounds, endContent))

	case Vertical:
		if m.startChildBounds.Height <= 0 {
			return endContent
		}
		if m.endChildBounds.Height <= 0 {
			return startContent
		}
		return lipgloss.JoinVertical(lipgloss.Left,
			renderWithBounds(&m.startChildBounds, startContent),
			renderWithBounds(&m.endChildBounds, endContent))
	}

	log.Warn().Msgf("(SplitLayout.Render) Invalid SplitLayoutModel orientation: %v", m.orientation)
	return ""
}

// SetBounds sets the bounds for the start and end child views based on the parent bounds.
func (m *SplitLayoutModel) SetBounds(bounds Bounds) {
	m.layoutBounds = bounds

	switch m.orientation {
	case Horizontal:
		var startWidth, endWidth int
		if m.fixedStartSize > 0 {
			if m.fixedEndSize > 0 {
				log.Warn().Msgf("(%s) Both fixedStartSize and fixedEndSize are set, using fixedStartSize: %d",
					"SplitLayoutModel.SetBounds",
					m.fixedStartSize)
			}
			startWidth = int(m.fixedStartSize)
			endWidth = bounds.Width - startWidth
		} else if m.fixedEndSize > 0 {
			endWidth = int(m.fixedEndSize)
			startWidth = bounds.Width - endWidth
		} else {
			startWidth = int(float64(bounds.Width) * m.fraction)
			endWidth = bounds.Width - startWidth
		}
		m.setStartChildBounds(NewBounds(startWidth, bounds.Height))
		m.setEndChildBounds(NewBounds(endWidth, bounds.Height))
		return

	case Vertical:
		var startHeight, endHeight int
		if m.fixedStartSize > 0 {
			startHeight = int(m.fixedStartSize)
			endHeight = bounds.Height - startHeight
		} else if m.fixedEndSize > 0 {
			endHeight = int(m.fixedEndSize)
			startHeight = bounds.Height - endHeight
		} else {
			startHeight = int(float64(bounds.Height) * m.fraction)
			endHeight = bounds.Height - startHeight
		}
		m.setStartChildBounds(NewBounds(bounds.Width, startHeight))
		m.setEndChildBounds(NewBounds(bounds.Width, endHeight))
		return
	}

	log.Warn().Msgf("(SplitLayout.SetBounds) Invalid SplitLayoutModel orientation: %v", m.orientation)
}

// refreshBounds send the newest bounds to the start and end child boundables.
// This is useful when the _layout_ itself changes, e.g. when you increase or decrease the fraction,
func (m *SplitLayoutModel) refreshBounds() {
	m.SetBounds(m.layoutBounds)
}

// setStartChildBounds sets the bounds for the start child view and updates its boundable.
func (m *SplitLayoutModel) setStartChildBounds(bounds Bounds) {
	m.startChildBounds = bounds
	if !bounds.IsValid() {
		log.Warn().Msgf("(SplitLayout.setStartChildBounds) start child bounds are invalid: %v", bounds)
		return
	}
	if m.startChildBoundable != nil {
		m.startChildBoundable.SetBounds(bounds)
	}
}

// setEndChildBounds sets the bounds for the end child view and updates its boundable.
func (m *SplitLayoutModel) setEndChildBounds(bounds Bounds) {
	m.endChildBounds = bounds
	if !bounds.IsValid() {
		log.Warn().Msgf("(SplitLayout.setEndChildBounds) end child bounds are invalid: %v", bounds)
		return
	}
	if m.endChildBoundable != nil {
		m.endChildBoundable.SetBounds(bounds)
	}
}

func (m *SplitLayoutModel) SetFraction(fraction float64) {
	m.fraction = Clamp(fraction, 0, 1)
	m.refreshBounds()
}

// Increase increases the fraction of the start view by 1/width
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

// Decrease decreases the fraction of the start view by 1/width
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

// AttachStartBoundable attaches a boundable to the start side of the split layout
func (m *SplitLayoutModel) AttachStartBoundable(boundable Boundable) {
	if boundable == nil {
		log.Warn().Msgf("(SplitLayoutModel.AttachStartBoundable) boundable is nil, cannot attach")
		return
	}
	m.startChildBoundable = boundable
}

// AttachEndBoundable attaches a boundable to the end side of the split layout
func (m *SplitLayoutModel) AttachEndBoundable(boundable Boundable) {
	if boundable == nil {
		log.Warn().Msgf("(SplitLayoutModel.AttachEndBoundable) boundable is nil, cannot attach")
		return
	}
	m.endChildBoundable = boundable
}

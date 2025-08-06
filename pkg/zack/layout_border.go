package zack

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
)

var (
	// DefaultBorderIdleStyle is the default style for the border when it is idle (not focused).
	// It uses a normal border style with gray foreground color.
	DefaultBorderIdleStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240"))

	// DefaultBorderActiveStyle is the default style for the border when it is active (focused).
	// It uses a normal border style with blue foreground color.
	DefaultBorderActiveStyle = DefaultBorderIdleStyle.
					BorderForeground(lipgloss.Color("33"))
)

var _ Boundable = (*BorderLayout)(nil)

// BorderLayout is a layout that renders a border around its content.
//
// +-----------------+
// |                 |
// |     content     |
// |                 |
// +-----------------+
type BorderLayout struct {
	Focuser

	childBounds    Bounds
	childBoundable Boundable

	layoutBounds Bounds

	borderActiveStyle       lipgloss.Style
	borderActiveWidthInset  int
	borderActiveHeightInset int

	borderIdleStyle       lipgloss.Style
	borderIdleWidthInset  int
	borderIdleHeightInset int
}

// NewBorderLayoutWithStyle creates a new BorderLayout instance with specified styles.
func NewBorderLayoutWithStyle(idleStyle, activeStyle lipgloss.Style) *BorderLayout {
	b := &BorderLayout{}
	b.SetIdleBorderStyle(idleStyle)
	b.SetActiveBorderStyle(activeStyle)
	return b
}

// NewBorderLayout creates a new BorderLayout instance with default styles.
func NewBorderLayout() *BorderLayout {
	return NewBorderLayoutWithStyle(DefaultBorderIdleStyle, DefaultBorderActiveStyle)
}

// Render adds a border around the content.
func (m *BorderLayout) Render(content string) string {
	if !m.layoutBounds.IsValid() {
		return "waiting for layout bounds to be set"
	}

	baseStyle := m.borderIdleStyle
	if m.HasFocus() {
		baseStyle = m.borderActiveStyle
	}
	return baseStyle.
		Width(m.childBounds.Width).
		Height(m.childBounds.Height).
		Render(content)
}

// SetBounds sets the bounds for the BorderLayout and its child view.
func (m *BorderLayout) SetBounds(bounds Bounds) {
	m.layoutBounds = bounds

	widthInset := m.borderIdleWidthInset
	heightInset := m.borderIdleHeightInset
	if m.HasFocus() {
		widthInset = m.borderActiveWidthInset
		heightInset = m.borderActiveHeightInset
	}

	m.childBounds = NewBounds(bounds.Width-widthInset, bounds.Height-heightInset)
	if !m.childBounds.IsValid() {
		log.Warn().Msgf("(BorderLayout.SetBounds) child bounds are invalid: %v", bounds)
		return
	}
	if m.childBoundable != nil {
		m.childBoundable.SetBounds(m.childBounds)
	}
}

// SetActiveBorderStyle sets the style for the border when it is active (focused).
func (m *BorderLayout) SetActiveBorderStyle(style lipgloss.Style) {
	m.borderActiveStyle = style
	m.borderActiveWidthInset = style.GetHorizontalFrameSize()
	m.borderActiveHeightInset = style.GetVerticalFrameSize()
}

// SetIdleBorderStyle sets the style for the border when it is idle (not focused).
func (m *BorderLayout) SetIdleBorderStyle(style lipgloss.Style) {
	m.borderIdleStyle = style
	m.borderIdleWidthInset = style.GetHorizontalFrameSize()
	m.borderIdleHeightInset = style.GetVerticalFrameSize()
}

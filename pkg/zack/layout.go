package zack

import (
	"github.com/charmbracelet/lipgloss"
)

// renderWithBounds renders a view with maximum bounds based on the provided core.Bounds.
// it set height, width AND max height and max width to the bounds.
func renderWithBounds(bounds *Bounds, view string) string {
	return lipgloss.NewStyle().
		Width(bounds.Width).
		Height(bounds.Height).
		Render(view)
}

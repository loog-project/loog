package diffpreview

import "github.com/charmbracelet/lipgloss"

// Theme defines the colours used when rendering YAML diffs.
type Theme struct {
	KeyStyle    lipgloss.Style
	StringStyle lipgloss.Style
	NumberStyle lipgloss.Style
	BoolStyle   lipgloss.Style
	NullStyle   lipgloss.Style

	AddedBg    lipgloss.Style
	RemovedBg  lipgloss.Style
	ModifiedBg lipgloss.Style
}

// backgroundStyle returns the background style for a change type, or nil
// when no highlighting is needed (Unchanged).
func (t Theme) backgroundStyle(change ChangeType) *lipgloss.Style {
	switch change {
	case Added:
		return &t.AddedBg
	case Removed:
		return &t.RemovedBg
	case Modified:
		return &t.ModifiedBg
	default:
		return nil
	}
}

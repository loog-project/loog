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

var DarkTheme = Theme{
	KeyStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")),
	StringStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")),
	NumberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")),
	BoolStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")),
	NullStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true),

	AddedBg:    lipgloss.NewStyle().Background(lipgloss.Color("#144212")).Foreground(lipgloss.Color("#A9DC76")),
	RemovedBg:  lipgloss.NewStyle().Background(lipgloss.Color("#4C1F1F")).Foreground(lipgloss.Color("#E06C75")),
	ModifiedBg: lipgloss.NewStyle().Background(lipgloss.Color("#3D3000")).Foreground(lipgloss.Color("#E5C07B")),
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

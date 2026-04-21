package diffpreview

import "github.com/charmbracelet/lipgloss"

// testTheme is a dark theme used by tests. It is not exported because
// production code should construct a Theme via the TUI's theme system.
var testTheme = Theme{
	KeyStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")),
	StringStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")),
	NumberStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("#61AFEF")),
	BoolStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")),
	NullStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Italic(true),

	AddedBg:    lipgloss.NewStyle().Background(lipgloss.Color("#144212")).Foreground(lipgloss.Color("#A9DC76")),
	RemovedBg:  lipgloss.NewStyle().Background(lipgloss.Color("#4C1F1F")).Foreground(lipgloss.Color("#E06C75")),
	ModifiedBg: lipgloss.NewStyle().Background(lipgloss.Color("#3D3000")).Foreground(lipgloss.Color("#E5C07B")),
}

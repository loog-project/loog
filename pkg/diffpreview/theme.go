package diffpreview

import "github.com/charmbracelet/lipgloss"

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

	// TODO: swap removed and added
	RemovedBg:  lipgloss.NewStyle().Background(lipgloss.Color("#144212")).Foreground(lipgloss.Color("#A9DC76")),
	AddedBg:    lipgloss.NewStyle().Background(lipgloss.Color("#4C1F1F")).Foreground(lipgloss.Color("#E06C75")),
	ModifiedBg: lipgloss.NewStyle().Background(lipgloss.Color("#FFA500")).Foreground(lipgloss.Color("#E5C07B")),
}

func (t Theme) SyntaxHighlight(kind string, content string) string {
	switch kind {
	case "key":
		return t.KeyStyle.Render(content)
	case "string":
		return t.StringStyle.Render(content)
	case "number":
		return t.NumberStyle.Render(content)
	case "bool":
		return t.BoolStyle.Render(content)
	case "null":
		return t.NullStyle.Render(content)
	default:
		return content
	}
}

func (t Theme) BackgroundHighlight(change ChangeType, content string) string {
	switch change {
	case Added:
		return t.AddedBg.Render(content)
	case Removed:
		return t.RemovedBg.Render(content)
	case Modified:
		return t.ModifiedBg.Render(content)
	default:
		return content
	}
}

package ui

import "github.com/charmbracelet/lipgloss"

var (
	StyleKind = lipgloss.NewStyle().
			Bold(true)
	StyleNS = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666")) // 😈
	StyleHot = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff8700"))
	StyleDim = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777"))
	StyleRev = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7d56f4"))
	StyleCur = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00afff"))
	BorderActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00afff"))
	BorderIdle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#777"))
)

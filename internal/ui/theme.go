package ui

import "github.com/charmbracelet/lipgloss"

var (
	ColorRed         = lipgloss.Color("1")
	ColorBlack       = lipgloss.Color("0")
	ColorWhite       = lipgloss.Color("255")
	ColorBrightBlue  = lipgloss.Color("33")
	ColorLightGray   = lipgloss.Color("243")
	ColorGray        = lipgloss.Color("238")
	ColorMutedPurple = lipgloss.Color("92")
	ColorOrange      = lipgloss.Color("214")
)

type Theme struct {
	ListKindNameTextStyle     lipgloss.Style
	ListNamespaceTextStyle    lipgloss.Style
	ListActivityTextStyle     lipgloss.Style
	ListRevisionTextStyle     lipgloss.Style
	ListCurrentArrowTextStyle lipgloss.Style

	AlertDialogContainerStyle lipgloss.Style

	BorderActiveContainerStyle lipgloss.Style
	BorderIdleContainerStyle   lipgloss.Style

	MutedTextStyle   lipgloss.Style
	ErrorTextStyle   lipgloss.Style
	PrimaryTextStyle lipgloss.Style

	BreadcrumbBarStyle lipgloss.Style
	LoggerBarStyle     lipgloss.Style
	HelpBarStyle       lipgloss.Style
}

var DarkTheme = Theme{
	ListKindNameTextStyle: lipgloss.NewStyle().
		Bold(true),
	ListNamespaceTextStyle: lipgloss.NewStyle().
		Foreground(ColorGray).
		Italic(true),
	ListActivityTextStyle: lipgloss.NewStyle().
		Foreground(ColorOrange).
		Bold(true),
	ListRevisionTextStyle: lipgloss.NewStyle().
		Foreground(ColorMutedPurple),
	ListCurrentArrowTextStyle: lipgloss.NewStyle().
		Foreground(ColorBrightBlue),

	AlertDialogContainerStyle: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorRed).
		Padding(1, 2),

	BorderActiveContainerStyle: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBrightBlue),
	BorderIdleContainerStyle: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGray),

	MutedTextStyle: lipgloss.NewStyle().
		Foreground(ColorLightGray),
	ErrorTextStyle: lipgloss.NewStyle().
		Foreground(ColorRed).
		Bold(true),
	PrimaryTextStyle: lipgloss.NewStyle().
		Foreground(ColorBrightBlue),

	BreadcrumbBarStyle: lipgloss.NewStyle().
		Padding(0, 1).
		Background(ColorBrightBlue).
		Foreground(ColorWhite),

	LoggerBarStyle: lipgloss.NewStyle().
		Padding(0, 1).
		Background(ColorOrange).
		Foreground(ColorWhite),

	HelpBarStyle: lipgloss.NewStyle().
		Padding(0, 1),
}

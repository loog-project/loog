package ui

import "github.com/charmbracelet/lipgloss"

// Some predefined colors

var (
	ColorRed         = lipgloss.Color("1")
	ColorBlack       = lipgloss.Color("0")
	ColorWhite       = lipgloss.Color("7")
	ColorBrightBlue  = lipgloss.Color("33")
	ColorLightGray   = lipgloss.Color("243")
	ColorGray        = lipgloss.Color("238")
	ColorMutedPurple = lipgloss.Color("92")
	ColorOrange      = lipgloss.Color("214")
)

type Theme struct {
	ListKindNameTextStyle     lipgloss.Style // StyleKind
	ListNamespaceTextStyle    lipgloss.Style // StyleNS
	ListActivityTextStyle     lipgloss.Style // StyleHot
	ListRevisionTextStyle     lipgloss.Style // StyleRev
	ListCurrentArrowTextStyle lipgloss.Style // StyleCur

	AlertContainerStyle        lipgloss.Style // StyleError
	BorderActiveContainerStyle lipgloss.Style // BorderActive
	BorderIdleContainerStyle   lipgloss.Style // BorderIdle

	MutedTextStyle   lipgloss.Style // StyleDim
	ErrorTextStyle   lipgloss.Style // StyleError
	PrimaryTextStyle lipgloss.Style // StyleDim

	BreadcrumbBarStyle lipgloss.Style // BarBreadcrumbs
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

	AlertContainerStyle: lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(ColorRed).
		Padding(4, 4),
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
}

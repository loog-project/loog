package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/pkg/diffpreview"
)

// Theme holds all color tokens and style factories for the TUI.
// Based on Catppuccin Mocha with adjustments for readability.
type Theme struct {
	// Base backgrounds
	Base     lipgloss.Color // main background
	Mantle   lipgloss.Color // slightly darker (panels)
	Crust    lipgloss.Color // darkest (status bar)
	Surface0 lipgloss.Color // elevated surfaces
	Surface1 lipgloss.Color // higher elevation
	Surface2 lipgloss.Color // highest elevation

	// Text
	Text     lipgloss.Color // primary text
	Subtext1 lipgloss.Color // secondary text
	Subtext0 lipgloss.Color // muted text
	Overlay2 lipgloss.Color // very muted
	Overlay1 lipgloss.Color // barely visible
	Overlay0 lipgloss.Color // ghost text

	// Accent colors
	Blue      lipgloss.Color
	Mauve     lipgloss.Color
	Teal      lipgloss.Color
	Sky       lipgloss.Color
	Peach     lipgloss.Color
	Green     lipgloss.Color
	Red       lipgloss.Color
	Yellow    lipgloss.Color
	Pink      lipgloss.Color
	Lavender  lipgloss.Color
	Flamingo  lipgloss.Color
	Rosewater lipgloss.Color
	Maroon    lipgloss.Color

	// Diff-specific backgrounds (subtle tints)
	DiffAddedBg    lipgloss.Color
	DiffRemovedBg  lipgloss.Color
	DiffModifiedBg lipgloss.Color

	// Modal overlay colors
	ModalDimFg    lipgloss.Color // dimmed background text color
	ModalDimBg    lipgloss.Color // dimmed background fill
	ModalShadowFg lipgloss.Color // drop shadow text
	ModalShadowBg lipgloss.Color // drop shadow fill
}

// CatppuccinMocha is the default dark theme.
var CatppuccinMocha = Theme{
	Base:     lipgloss.Color("#1e1e2e"),
	Mantle:   lipgloss.Color("#181825"),
	Crust:    lipgloss.Color("#11111b"),
	Surface0: lipgloss.Color("#313244"),
	Surface1: lipgloss.Color("#45475a"),
	Surface2: lipgloss.Color("#585b70"),

	Text:     lipgloss.Color("#cdd6f4"),
	Subtext1: lipgloss.Color("#bac2de"),
	Subtext0: lipgloss.Color("#a6adc8"),
	Overlay2: lipgloss.Color("#9399b2"),
	Overlay1: lipgloss.Color("#7f849c"),
	Overlay0: lipgloss.Color("#6c7086"),

	Blue:      lipgloss.Color("#89b4fa"),
	Mauve:     lipgloss.Color("#cba6f7"),
	Teal:      lipgloss.Color("#94e2d5"),
	Sky:       lipgloss.Color("#89dcfe"),
	Peach:     lipgloss.Color("#fab387"),
	Green:     lipgloss.Color("#a6e3a1"),
	Red:       lipgloss.Color("#f38ba8"),
	Yellow:    lipgloss.Color("#f9e2af"),
	Pink:      lipgloss.Color("#f5c2e7"),
	Lavender:  lipgloss.Color("#b4befe"),
	Flamingo:  lipgloss.Color("#f2cdcd"),
	Rosewater: lipgloss.Color("#f5e0dc"),
	Maroon:    lipgloss.Color("#eba0ac"),

	DiffAddedBg:    lipgloss.Color("#1a3a1a"),
	DiffRemovedBg:  lipgloss.Color("#3a1a1a"),
	DiffModifiedBg: lipgloss.Color("#3a2a1a"),

	ModalDimFg:    lipgloss.Color("#45475a"),
	ModalDimBg:    lipgloss.Color("#11111b"),
	ModalShadowFg: lipgloss.Color("#181825"),
	ModalShadowBg: lipgloss.Color("#0a0a14"),
}

// --- Style Factories ---

func (t Theme) ActiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(t.Blue).
		Foreground(t.Crust).
		Bold(true).
		Padding(0, 1)
}

func (t Theme) InactiveTabStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(t.Overlay1).
		Padding(0, 1)
}

// Text styles
func (t Theme) MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Overlay0)
}

func (t Theme) SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Green)
}

func (t Theme) ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Red)
}

func (t Theme) WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Peach)
}

func (t Theme) InfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Sky)
}

// Syntax highlighting styles
func (t Theme) SyntaxKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Blue)
}

func (t Theme) SyntaxStringStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Green)
}

func (t Theme) SyntaxNumberStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Peach)
}

func (t Theme) SyntaxBoolStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Yellow)
}
func (t Theme) SyntaxNullStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Overlay0).Italic(true)
}

// Badge styles for resource tree indicators
func (t Theme) HotBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Red).Bold(true)
}
func (t Theme) WarmBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Peach)
}
func (t Theme) StarStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(t.Yellow)
}

// KeyHint renders a keybinding hint like "ctrl+k commands".
func (t Theme) KeyHint(key, label string) string {
	k := lipgloss.NewStyle().Foreground(t.Overlay2).Render(key)
	l := lipgloss.NewStyle().Foreground(t.Overlay0).Render(label)
	return k + " " + l
}

// DiffPreviewTheme returns a diffpreview.Theme mapped from TUI theme colors.
func (t Theme) DiffPreviewTheme() diffpreview.Theme {
	return diffpreview.Theme{
		KeyStyle:    lipgloss.NewStyle().Foreground(t.Blue),
		StringStyle: lipgloss.NewStyle().Foreground(t.Green),
		NumberStyle: lipgloss.NewStyle().Foreground(t.Peach),
		BoolStyle:   lipgloss.NewStyle().Foreground(t.Yellow),
		NullStyle:   lipgloss.NewStyle().Foreground(t.Overlay0).Italic(true),
		AddedBg:     lipgloss.NewStyle().Background(t.DiffAddedBg).Foreground(t.Green),
		RemovedBg:   lipgloss.NewStyle().Background(t.DiffRemovedBg).Foreground(t.Red),
		ModifiedBg:  lipgloss.NewStyle().Background(t.DiffModifiedBg).Foreground(t.Peach),
	}
}

// EventTypeStyle returns the appropriate style for an event type.
func (t Theme) EventTypeStyle(et EventType) lipgloss.Style {
	switch et {
	case EventAdded:
		return t.SuccessStyle()
	case EventModified:
		return t.WarningStyle()
	case EventDeleted:
		return t.ErrorStyle()
	default:
		return t.MutedStyle()
	}
}

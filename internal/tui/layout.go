package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Overlay ---

// PlaceOverlay renders `fg` centered on `bg`.
// Uses lipgloss.Place for correct ANSI-aware centering.
func PlaceOverlay(fg, bg string, shadow bool) string {
	bgW := lipgloss.Width(bg)
	bgH := lipgloss.Height(bg)

	return lipgloss.Place(bgW, bgH, lipgloss.Center, lipgloss.Center, fg,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(lipgloss.Color("")),
	)
}

// --- Split Pane ---

// SplitHorizontal renders two pre-rendered content blocks side by side
// with a thin vertical separator. The content blocks must already be
// sized and rendered to the correct dimensions.
func SplitHorizontal(left, right string, leftWidth, totalWidth, height int) string {
	// Build vertical separator
	var sepLines []string
	for i := 0; i < height; i++ {
		sepLines = append(sepLines, "│")
	}
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#313244"))
	sep := sepStyle.Render(strings.Join(sepLines, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, right)
}

// SplitThreeColumns renders three pre-rendered content blocks side by side
// with thin vertical separators. The content blocks must already be
// sized and rendered to the correct dimensions.
func SplitThreeColumns(left, middle, right string, leftW, midW, totalW, height int) string {
	var sepLines []string
	for i := 0; i < height; i++ {
		sepLines = append(sepLines, "│")
	}
	sepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#313244"))
	sep := sepStyle.Render(strings.Join(sepLines, "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, sep, middle, sep, right)
}

// --- Panel Border ---

// PanelBorder wraps content in a manually drawn rounded border with a title.
// This avoids lipgloss.Border() + rune manipulation which breaks ANSI sequences.
// The `width` and `height` are the OUTER dimensions (including the border).
// The content should be pre-rendered to (width-2) x (height-2) inner dimensions.
func PanelBorder(content, title string, width, height int, focused bool, theme Theme) string {
	return PanelBorderEx(content, title, width, height, focused, theme, false, false)
}

// PanelBorderEx is like PanelBorder but with scroll indicators.
// canScrollUp/canScrollDown add ▲/▼ indicators at the top-right/bottom-right of the border.
func PanelBorderEx(content, title string, width, height int, focused bool, theme Theme, canScrollUp, canScrollDown bool) string {
	if width < 4 || height < 3 {
		return content
	}

	innerW := width - 2

	var borderColor lipgloss.Color
	var titleColor lipgloss.Color
	if focused {
		borderColor = theme.Blue
		titleColor = theme.Blue
	} else {
		borderColor = theme.Surface1
		titleColor = theme.Overlay0
	}

	bStyle := lipgloss.NewStyle().Foreground(borderColor)
	tStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(focused)

	// Build top border with title and optional scroll-up indicator
	var topLine string
	scrollUpStr := ""
	if canScrollUp {
		scrollUpStr = lipgloss.NewStyle().Foreground(theme.Overlay1).Render(" ▲")
	}
	scrollUpW := lipgloss.Width(scrollUpStr)

	if title != "" {
		titleStr := " " + tStyle.Render(title) + " "
		titleVisualW := lipgloss.Width(titleStr)
		remainingW := innerW - titleVisualW - scrollUpW
		if remainingW < 0 {
			remainingW = 0
		}
		leftPad := 1
		rightPad := remainingW - leftPad
		if rightPad < 0 {
			rightPad = 0
		}
		topLine = bStyle.Render("╭"+strings.Repeat("─", leftPad)) +
			titleStr +
			bStyle.Render(strings.Repeat("─", rightPad)) +
			scrollUpStr +
			bStyle.Render("╮")
	} else {
		fillW := innerW - scrollUpW
		if fillW < 0 {
			fillW = 0
		}
		topLine = bStyle.Render("╭"+strings.Repeat("─", fillW)) +
			scrollUpStr +
			bStyle.Render("╮")
	}

	// Build bottom border with optional scroll-down indicator
	scrollDownStr := ""
	if canScrollDown {
		scrollDownStr = lipgloss.NewStyle().Foreground(theme.Overlay1).Render(" ▼")
	}
	scrollDownW := lipgloss.Width(scrollDownStr)
	bottomFillW := innerW - scrollDownW
	if bottomFillW < 0 {
		bottomFillW = 0
	}
	bottomLine := bStyle.Render("╰"+strings.Repeat("─", bottomFillW)) +
		scrollDownStr +
		bStyle.Render("╯")

	// Build content lines with side borders
	contentLines := strings.Split(content, "\n")

	// Ensure we have exactly height-2 content lines
	innerH := height - 2
	for len(contentLines) < innerH {
		contentLines = append(contentLines, "")
	}
	if len(contentLines) > innerH {
		contentLines = contentLines[:innerH]
	}

	var lines []string
	lines = append(lines, topLine)
	leftBorder := bStyle.Render("│")
	rightBorder := bStyle.Render("│")
	for _, cl := range contentLines {
		// Pad or truncate the content line to inner width
		lineW := lipgloss.Width(cl)
		if lineW < innerW {
			cl = cl + strings.Repeat(" ", innerW-lineW)
		}
		lines = append(lines, leftBorder+cl+rightBorder)
	}
	lines = append(lines, bottomLine)

	return strings.Join(lines, "\n")
}

// Truncate truncates a string to maxLen, adding ellipsis if needed.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// PadRight pads a string to the given width with spaces.
func PadRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

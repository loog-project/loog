package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Modal Overlay ---

// ansiStripRe matches ANSI escape sequences for stripping during dimming.
var ansiStripRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiStripRe.ReplaceAllString(s, "")
}

// ModalOverlay composites a pre-rendered modal dialog on top of a background
// with background dimming and a drop shadow effect.
// The modal is centered on the background. Background text is stripped of ANSI
// and re-rendered in dim colors. A shadow (2 cols right, 1 row down) appears
// behind the modal.
func ModalOverlay(modal, bg string, theme Theme) string {
	bgW := lipgloss.Width(bg)
	bgH := lipgloss.Height(bg)
	modalW := lipgloss.Width(modal)
	modalH := lipgloss.Height(modal)

	bgLines := strings.Split(bg, "\n")
	modalLines := strings.Split(modal, "\n")

	for len(bgLines) < bgH {
		bgLines = append(bgLines, "")
	}
	if len(bgLines) > bgH {
		bgLines = bgLines[:bgH]
	}

	// Center the modal
	modalTop := (bgH - modalH) / 2
	modalLeft := (bgW - modalW) / 2
	if modalTop < 1 {
		modalTop = 1
	}
	if modalLeft < 2 {
		modalLeft = 2
	}

	// Shadow: offset +1 row, +2 cols from modal
	shadowTop := modalTop + 1

	// Right shadow strip: 2 chars at cols [modalLeft+modalW, +2)
	// on rows [shadowTop, modalTop+modalH)
	shadowRColStart := modalLeft + modalW
	shadowRColEnd := shadowRColStart + 2

	// Bottom shadow strip: 1 row at modalTop+modalH,
	// cols [modalLeft+2, modalLeft+2+modalW)
	shadowBottomRow := modalTop + modalH
	shadowBColStart := modalLeft + 2
	shadowBColEnd := shadowBColStart + modalW

	dimStyle := lipgloss.NewStyle().Foreground(theme.ModalDimFg).Background(theme.ModalDimBg)
	shadowStyle := lipgloss.NewStyle().Foreground(theme.ModalShadowFg).Background(theme.ModalShadowBg)

	result := make([]string, bgH)

	for row := 0; row < bgH; row++ {
		bgLine := ""
		if row < len(bgLines) {
			bgLine = bgLines[row]
		}

		inModalVert := row >= modalTop && row < modalTop+modalH
		inShadowRightVert := row >= shadowTop && row < modalTop+modalH
		isBottomShadowRow := row == shadowBottomRow

		// Fast path: rows outside modal/shadow — just dim
		if !inModalVert && !inShadowRightVert && !isBottomShadowRow {
			result[row] = dimLine(bgLine, bgW, dimStyle)
			continue
		}

		// Build segments for this row
		segs := buildRowSegments(row, bgW,
			inModalVert, modalLeft, modalW,
			inShadowRightVert, shadowRColStart, shadowRColEnd,
			isBottomShadowRow, shadowBColStart, shadowBColEnd,
		)

		bgRunes := []rune(stripANSI(bgLine))
		for len(bgRunes) < bgW {
			bgRunes = append(bgRunes, ' ')
		}

		var modalLineStr string
		if inModalVert {
			idx := row - modalTop
			if idx >= 0 && idx < len(modalLines) {
				modalLineStr = modalLines[idx]
			}
		}

		var sb strings.Builder
		for _, seg := range segs {
			switch seg.kind {
			case segModal:
				sb.WriteString(modalLineStr)
			case segShadow:
				end := seg.end
				if end > len(bgRunes) {
					end = len(bgRunes)
				}
				sb.WriteString(shadowStyle.Render(string(bgRunes[seg.start:end])))
			case segBg:
				end := seg.end
				if end > len(bgRunes) {
					end = len(bgRunes)
				}
				sb.WriteString(dimStyle.Render(string(bgRunes[seg.start:end])))
			}
		}
		result[row] = sb.String()
	}

	return strings.Join(result, "\n")
}

// segKind classifies what treatment a column range gets.
type segKind int

const (
	segBg     segKind = iota // dimmed background
	segModal                 // modal content (pre-rendered ANSI)
	segShadow                // drop shadow
)

// segment is a horizontal run of columns sharing the same treatment.
type segment struct {
	kind       segKind
	start, end int // [start, end)
}

// buildRowSegments partitions [0, bgW) into typed segments (bg, modal, shadow).
func buildRowSegments(
	row, bgW int,
	inModal bool, modalLeft, modalW int,
	inShadowRight bool, shadowRColStart, shadowRColEnd int,
	isBottomShadow bool, shadowBColStart, shadowBColEnd int,
) []segment {
	type interval struct {
		kind       segKind
		start, end int
	}
	var specials []interval

	if inModal {
		specials = append(specials, interval{segModal, modalLeft, modalLeft + modalW})
	}
	if inShadowRight && shadowRColStart < bgW {
		end := shadowRColEnd
		if end > bgW {
			end = bgW
		}
		specials = append(specials, interval{segShadow, shadowRColStart, end})
	}
	if isBottomShadow && shadowBColStart < bgW {
		end := shadowBColEnd
		if end > bgW {
			end = bgW
		}
		specials = append(specials, interval{segShadow, shadowBColStart, end})
	}

	// Sort by start (insertion sort — at most 3 elements)
	for i := 1; i < len(specials); i++ {
		for j := i; j > 0 && specials[j].start < specials[j-1].start; j-- {
			specials[j], specials[j-1] = specials[j-1], specials[j]
		}
	}

	var segs []segment
	cursor := 0
	for _, sp := range specials {
		if sp.start > cursor {
			segs = append(segs, segment{segBg, cursor, sp.start})
		}
		segs = append(segs, segment{sp.kind, sp.start, sp.end})
		cursor = sp.end
	}
	if cursor < bgW {
		segs = append(segs, segment{segBg, cursor, bgW})
	}
	return segs
}

// dimLine strips ANSI from a line and renders it in dim style.
func dimLine(line string, width int, dimStyle lipgloss.Style) string {
	runes := []rune(stripANSI(line))
	for len(runes) < width {
		runes = append(runes, ' ')
	}
	if len(runes) > width {
		runes = runes[:width]
	}
	return dimStyle.Render(string(runes))
}

// --- Split Pane ---

// SplitHorizontal renders two pre-rendered content blocks side by side
// with a thin vertical separator.
func SplitHorizontal(left, right string, height int) string {
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
// with thin vertical separators.
func SplitThreeColumns(left, middle, right string, height int) string {
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

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Header renders the top bar: logo, view tabs, and recording status.
type Header struct {
	width      int
	theme      Theme
	activeView ViewID
	recording  bool
	frozen     bool
}

func NewHeader(theme Theme) *Header {
	return &Header{
		theme:     theme,
		recording: true,
	}
}

func (h *Header) SetSize(width int) {
	h.width = width
}

func (h *Header) SetView(v ViewID) {
	h.activeView = v
}

func (h *Header) SetRecording(on bool) {
	h.recording = on
}

func (h *Header) SetFrozen(frozen bool) {
	h.frozen = frozen
}

func (h *Header) View() string {
	if h.width <= 0 {
		return ""
	}

	// Logo
	logo := lipgloss.NewStyle().
		Foreground(h.theme.Blue).
		Bold(true).
		Render("loog")

	// View tabs
	var tabs []string
	for _, v := range AllViews() {
		if v == h.activeView {
			tabs = append(tabs, h.theme.ActiveTabStyle().Render(v.String()))
		} else {
			tabs = append(tabs, h.theme.InactiveTabStyle().Render(v.String()))
		}
	}
	tabBar := strings.Join(tabs, " ")

	// Right side: state indicators + time
	var rightParts []string

	// Frozen indicator (takes visual priority)
	if h.frozen {
		indicator := lipgloss.NewStyle().Foreground(h.theme.Sky).Bold(true).Render("◆")
		label := lipgloss.NewStyle().Foreground(h.theme.Sky).Render("frozen")
		rightParts = append(rightParts, indicator+" "+label)
	}

	// Recording / paused state
	if h.recording {
		indicator := lipgloss.NewStyle().Foreground(h.theme.Red).Bold(true).Render("◆")
		label := lipgloss.NewStyle().Foreground(h.theme.Subtext0).Render("recording")
		rightParts = append(rightParts, indicator+" "+label)
	} else {
		indicator := lipgloss.NewStyle().Foreground(h.theme.Yellow).Bold(true).Render("■")
		label := lipgloss.NewStyle().Foreground(h.theme.Yellow).Render("paused")
		rightParts = append(rightParts, indicator+" "+label)
	}

	rightParts = append(rightParts,
		lipgloss.NewStyle().Foreground(h.theme.Overlay1).Render(time.Now().Format("15:04:05")),
	)
	rightSide := strings.Join(rightParts, "  ")

	// Layout: logo | tabs | spacer | right
	leftContent := logo + "  " + tabBar
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightSide)
	spacerWidth := max(h.width-leftWidth-rightWidth-2, 1)
	spacer := strings.Repeat(" ", spacerWidth)

	line := leftContent + spacer + rightSide

	return lipgloss.NewStyle().
		Background(h.theme.Mantle).
		Width(h.width).
		Padding(0, 1).
		Render(line)
}

// StatusBar renders the bottom bar with context info and keybind hints.
type StatusBar struct {
	width          int
	theme          Theme
	resourceInfo   string
	revisionInfo   string
	viewMode       ViewMode
	totalResources int
	totalRevisions int
	starredCount   int
	statusMsg      string
	statusIsError  bool
	hint           string
	autoScroll     bool
	windowMode     WindowMode
	simulating     bool
	isSimMode      bool // true only in simulation mode (not production recording)
}

func NewStatusBar(theme Theme) *StatusBar {
	return &StatusBar{theme: theme, viewMode: DiffMode}
}

func (sb *StatusBar) SetSize(width int) {
	sb.width = width
}

func (sb *StatusBar) SetResourceInfo(info string) {
	sb.resourceInfo = info
}

func (sb *StatusBar) SetRevisionInfo(info string) {
	sb.revisionInfo = info
}

func (sb *StatusBar) SetViewMode(mode ViewMode) {
	sb.viewMode = mode
}
func (sb *StatusBar) SetCounts(res, revs, starred int) {
	sb.totalResources = res
	sb.totalRevisions = revs
	sb.starredCount = starred
}
func (sb *StatusBar) SetStatus(text string, isErr bool) {
	sb.statusMsg = text
	sb.statusIsError = isErr
}
func (sb *StatusBar) SetHint(hint string) {
	sb.hint = hint
}

func (sb *StatusBar) SetAutoScroll(on bool) {
	sb.autoScroll = on
}

func (sb *StatusBar) SetWindowMode(wm WindowMode) {
	sb.windowMode = wm
}

func (sb *StatusBar) SetSimulating(on bool) {
	sb.simulating = on
}

func (sb *StatusBar) SetSimMode(on bool) {
	sb.isSimMode = on
}

func (sb *StatusBar) View() string {
	if sb.width <= 0 {
		return ""
	}

	// Left side: context info
	var leftParts []string
	if sb.statusMsg != "" {
		style := sb.theme.InfoStyle()
		if sb.statusIsError {
			style = sb.theme.ErrorStyle()
		}
		leftParts = append(leftParts, style.Render(sb.statusMsg))
	} else {
		if sb.resourceInfo != "" {
			leftParts = append(leftParts, lipgloss.NewStyle().Foreground(sb.theme.Text).Bold(true).Render(sb.resourceInfo))
		}
		if sb.revisionInfo != "" {
			leftParts = append(leftParts, lipgloss.NewStyle().Foreground(sb.theme.Mauve).Render(sb.revisionInfo))
		}
		modeBadge := lipgloss.NewStyle().Foreground(sb.theme.Teal).Render(sb.viewMode.String())
		leftParts = append(leftParts, modeBadge)
	}

	// Feature badges
	if sb.autoScroll {
		leftParts = append(leftParts, lipgloss.NewStyle().Foreground(sb.theme.Teal).Bold(true).Render("[AUTO]"))
	}
	if sb.windowMode != WindowAll {
		leftParts = append(leftParts, lipgloss.NewStyle().Foreground(sb.theme.Sky).Render("[W:"+sb.windowMode.String()+"]"))
	}
	if sb.isSimMode {
		leftParts = append(leftParts, lipgloss.NewStyle().Foreground(sb.theme.Green).Render("[SIM]"))
	}

	leftContent := strings.Join(leftParts, "  │  ")

	// Right side: counts + keybind hints
	var rightParts []string
	if sb.starredCount > 0 {
		rightParts = append(rightParts,
			lipgloss.NewStyle().Foreground(sb.theme.Yellow).Render(fmt.Sprintf("★ %d", sb.starredCount)))
	}
	rightParts = append(rightParts,
		lipgloss.NewStyle().Foreground(sb.theme.Overlay1).Render(
			fmt.Sprintf("%d res  %d revs", sb.totalResources, sb.totalRevisions)))
	rightParts = append(rightParts, sb.theme.KeyHint("ctrl+k", "cmds"))
	rightParts = append(rightParts, sb.theme.KeyHint("?", "help"))
	rightContent := strings.Join(rightParts, "  ")

	// Combine
	leftWidth := lipgloss.Width(leftContent)
	rightWidth := lipgloss.Width(rightContent)
	spacerWidth := max(sb.width-leftWidth-rightWidth-4, 1)

	line := " " + leftContent + strings.Repeat(" ", spacerWidth) + rightContent + " "

	statusLine := lipgloss.NewStyle().
		Background(sb.theme.Crust).
		Foreground(sb.theme.Subtext0).
		Width(sb.width).
		Render(line)

	// Second line: always rendered for stable layout height.
	// Shows context-sensitive hint when available, otherwise empty.
	hintContent := " "
	if sb.hint != "" {
		hintContent = " " + sb.hint
	}
	hintLine := lipgloss.NewStyle().
		Background(sb.theme.Crust).
		Foreground(sb.theme.Overlay0).
		Width(sb.width).
		Render(hintContent)

	return statusLine + "\n" + hintLine
}

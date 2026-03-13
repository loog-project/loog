package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay renders a keybinding reference table.
type HelpOverlay struct {
	width, height int
	theme         Theme
	visible       bool
	scrollOffset  int
}

func NewHelpOverlay(theme Theme) *HelpOverlay {
	return &HelpOverlay{theme: theme}
}

func (h *HelpOverlay) SetSize(w, ht int) { h.width = w; h.height = ht }
func (h *HelpOverlay) IsVisible() bool   { return h.visible }
func (h *HelpOverlay) Show()             { h.visible = true; h.scrollOffset = 0 }
func (h *HelpOverlay) Hide()             { h.visible = false }

func (h *HelpOverlay) Update(msg tea.Msg) tea.Cmd {
	if !h.visible {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			h.Hide()
			return Cmd(HideOverlayMsg{})
		case "j", "down":
			h.scrollOffset++
		case "k", "up":
			if h.scrollOffset > 0 {
				h.scrollOffset--
			}
		}
	}
	return nil
}

type helpSection struct {
	Title    string
	Bindings []helpBinding
}

type helpBinding struct {
	Key  string
	Desc string
}

func (h *HelpOverlay) View() string {
	if !h.visible {
		return ""
	}

	dialogWidth := h.width * 55 / 100
	if dialogWidth > 70 {
		dialogWidth = 70
	}
	if dialogWidth < 40 {
		dialogWidth = 40
	}
	innerWidth := dialogWidth - 6

	title := lipgloss.NewStyle().
		Foreground(h.theme.Blue).
		Bold(true).
		Render("Keyboard Shortcuts")

	sep := lipgloss.NewStyle().
		Foreground(h.theme.Surface1).
		Render(strings.Repeat("─", innerWidth))

	sections := []helpSection{
		{Title: "Global", Bindings: []helpBinding{
			{"q / ctrl+c", "Quit"},
			{"ctrl+k", "Command palette"},
			{"?", "Toggle help"},
			{"/", "Filter resources"},
			{"//", "Quick fuzzy search for resources"},
			{"F1-F4", "Switch views"},
			{"1 / 2 / 3", "Focus panel"},
			{"Tab", "Next panel"},
			{"Shift+Tab", "Previous panel"},
			{"f", "Toggle fullscreen"},
			{"a", "Toggle auto-scroll"},
			{"w", "Cycle time window (±15s/±30s/±1m/±5m around selected revision)"},
			{"P", "Pause/resume recording (stops generating new data)"},
			{"F5", "Freeze/unfreeze view (data keeps arriving, UI pauses)"},
			{"W", "Watch Manager (add/remove watched resources)"},
		}},
		{Title: "Lists (Resources / Revisions / Timeline)", Bindings: []helpBinding{
			{"j / k", "Move down / up"},
			{"g / G", "Go to top / bottom"},
			{"Enter", "Select / expand"},
			{"s", "Toggle star"},
			{"c", "Mark for compare"},
		}},
		{Title: "Detail View", Bindings: []helpBinding{
			{"d", "Diff mode"},
			{"o", "Object mode (full YAML)"},
			{"p", "Patch mode (changes only)"},
			{"J", "JSON mode"},
			{"[ / ]", "Previous / next revision"},
			{"e", "Export YAML"},
			{"y", "Copy to clipboard"},
			{"t", "Jump to timeline"},
		}},
		{Title: "Symbols", Bindings: []helpBinding{
			{"@", "Recently active resource"},
			{"o", "Idle resource"},
			{"~", "Reconcile loop detected"},
			{"!", "High change frequency"},
			{"*", "Starred resource"},
			{"[C1] / [C2]", "Compare mark left / right"},
			{"+ | `", "Burst group bracket"},
			{"[AUTO]", "Auto-scroll enabled"},
			{"[SIM]", "Simulation running"},
			{"frozen", "View frozen (blue header indicator)"},
			{"paused", "Recording paused (yellow header indicator)"},
		}},
		{Title: "Command Palette", Bindings: []helpBinding{
			{"typing", "Fuzzy search commands"},
			{"up / down", "Navigate results"},
			{"Enter", "Execute command"},
			{"Esc", "Close"},
		}},
	}

	var lines []string
	for _, section := range sections {
		sectionTitle := lipgloss.NewStyle().
			Foreground(h.theme.Mauve).
			Bold(true).
			Render(section.Title)
		lines = append(lines, sectionTitle)

		for _, b := range section.Bindings {
			keyStyle := lipgloss.NewStyle().
				Foreground(h.theme.Yellow).
				Width(16).
				Render(b.Key)
			descStyle := lipgloss.NewStyle().
				Foreground(h.theme.Text).
				Render(b.Desc)
			lines = append(lines, "  "+keyStyle+"  "+descStyle)
		}
		lines = append(lines, "") // blank separator
	}

	// Scroll
	allContent := strings.Join(lines, "\n")
	contentLines := strings.Split(allContent, "\n")
	maxVisible := h.height - 12
	if maxVisible < 5 {
		maxVisible = 5
	}
	if h.scrollOffset >= len(contentLines)-maxVisible {
		h.scrollOffset = len(contentLines) - maxVisible
	}
	if h.scrollOffset < 0 {
		h.scrollOffset = 0
	}

	endIdx := h.scrollOffset + maxVisible
	if endIdx > len(contentLines) {
		endIdx = len(contentLines)
	}
	visibleContent := strings.Join(contentLines[h.scrollOffset:endIdx], "\n")

	// Footer hint
	footer := lipgloss.NewStyle().
		Foreground(h.theme.Overlay0).
		Italic(true).
		Render("  Press ? or Esc to close  │  j/k to scroll")

	content := title + "\n" + sep + "\n\n" + visibleContent + "\n\n" + sep + "\n" + footer

	dialog := lipgloss.NewStyle().
		Background(h.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(h.theme.Blue).
		Padding(1, 2).
		Width(dialogWidth).
		Render(content)

	return dialog
}

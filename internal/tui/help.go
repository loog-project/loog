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

func (h *HelpOverlay) SetSize(w, ht int) {
	h.width = w
	h.height = ht
}

func (h *HelpOverlay) IsVisible() bool {
	return h.visible
}

func (h *HelpOverlay) Show() {
	h.visible = true
	h.scrollOffset = 0
}

func (h *HelpOverlay) Hide() {
	h.visible = false
}

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
		case "ctrl+d", "pgdown":
			pageSize := h.height / 3
			if pageSize < 5 {
				pageSize = 5
			}
			h.scrollOffset += pageSize
		case "ctrl+u", "pgup":
			pageSize := h.height / 3
			if pageSize < 5 {
				pageSize = 5
			}
			h.scrollOffset -= pageSize
			if h.scrollOffset < 0 {
				h.scrollOffset = 0
			}
		case "g":
			h.scrollOffset = 0
		case "G":
			h.scrollOffset = 9999 // will be clamped in View()
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
			{"F1-F4", "Switch views (Explorer / Timeline / Watchlist / Compare)"},
			{"alt+1-4", "Switch views (terminal-safe fallback for F-keys)"},
			{"1 / 2 / 3", "Focus panel"},
			{"Tab", "Next panel"},
			{"Shift+Tab", "Previous panel"},
			{"f", "Toggle fullscreen"},
			{"a", "Toggle auto-scroll"},
			{"w", "Cycle time window (±15s/±30s/±1m/±5m around selected revision)"},
			{"P", "Pause/resume recording (stops generating new data)"},
			{"F5 / alt+5", "Freeze/unfreeze view (data keeps arriving, UI pauses)"},
			{"W", "Watch Manager (add/remove watched resource types)"},
			{"F6 / alt+6", "Debug log viewer"},
			{":", "Developer console"},
		}},
		{Title: "Lists (Resources / Revisions / Timeline)", Bindings: []helpBinding{
			{"j / k", "Move down / up"},
			{"g / G (home / end)", "Go to top / bottom"},
			{"ctrl+d / pgdn", "Page down"},
			{"ctrl+u / pgup", "Page up"},
			{"Enter (space)", "Select / expand"},
			{"s", "Toggle star"},
			{"c", "Mark for compare"},
		}},
		{Title: "Timeline View", Bindings: []helpBinding{
			{"S", "Toggle starred-only filter (show only starred resources)"},
			{"w", "Cycle time window around selected revision"},
		}},
		{Title: "Compare View", Bindings: []helpBinding{
			{"Tab", "Switch focus between left and right panes"},
			{"X", "Clear compare selection (remove both marks)"},
			{"j / k", "Scroll diff up / down"},
			{"ctrl+d / pgdn", "Page down in diff"},
			{"ctrl+u / pgup", "Page up in diff"},
		}},
		{Title: "Detail View", Bindings: []helpBinding{
			{"d", "Diff mode (YAML with change highlighting)"},
			{"o", "Object mode (full YAML)"},
			{"p", "Patch mode (changes only)"},
			{"J", "JSON mode"},
			{"r", "Raw mode (database record for debugging)"},
			{"[ / ]", "Previous / next revision"},
			{"e", "Export YAML to file"},
			{"y", "Copy YAML to clipboard"},
			{"t", "Jump to timeline"},
		}},
		{Title: "Symbols", Bindings: []helpBinding{
			{"●", "Recently active resource (changed within seconds)"},
			{"○", "Idle resource"},
			{"↻", "Reconcile loop detected"},
			{"▲ / △", "High / moderate change frequency"},
			{"★", "Starred resource"},
			{"[C1] / [C2]", "Compare mark left / right"},
			{"╭ │ ╰", "Burst group bracket (rapid changes)"},
			{"▸", "Window anchor position"},
			{"▲ / ▼", "Scroll indicators (more content above/below)"},
			{"[AUTO]", "Auto-scroll enabled"},
			{"[SIM]", "Simulation running"},
			{"◆ frozen", "View frozen (blue header indicator)"},
			{"■ paused", "Recording paused (yellow header indicator)"},
		}},
		{Title: "Debug Tools", Bindings: []helpBinding{
			{"F6 / alt+6", "Open debug log viewer (see internal events)"},
			{":", "Open developer console (inspect store, resources, revisions)"},
			{"r", "Raw mode in detail view (database representation)"},
		}},
		{Title: "Console Commands (: key)", Bindings: []helpBinding{
			{"help", "Show available commands"},
			{"status", "App state summary"},
			{"resources", "List all resources with UIDs"},
			{"revisions <uid>", "List revisions for a resource"},
			{"inspect <uid>", "Show raw revision data (database format)"},
			{"store", "Dump store statistics"},
			{"kinds", "List resource kinds and counts"},
			{"sim start/stop", "Start or stop live simulation"},
			{"log <msg>", "Write a debug log entry"},
			{"clear", "Clear console output"},
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
		Render("  Press ? or Esc to close  │  j/k scroll  ctrl+d/u page  g/G top/bottom")

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

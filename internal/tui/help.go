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
			pageSize := max(h.height/3, 5)
			h.scrollOffset += pageSize
		case "ctrl+u", "pgup":
			pageSize := max(h.height/3, 5)
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

	dialogWidth := min(h.width*65/100, 82)
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
			{"/", "Inline filter (Enter apply, Esc clear)"},
			{"//", "Quick Jump (fuzzy resource finder)"},
			{"F1-F3", "Switch view (Explorer / Timeline / Compare)"},
			{"alt+1-3", "Switch view (terminal fallback for F-keys)"},
			{"1 / 2 / 3", "Focus panel"},
			{"Tab", "Next panel"},
			{"Shift+Tab", "Previous panel"},
			{"f", "Toggle fullscreen"},
			{"a", "Toggle auto-scroll"},
			{"w", "Cycle time window around selected revision"},
			{"P", "Pause/resume recording"},
			{"F5 / alt+5", "Freeze/unfreeze view"},
			{"W", "Watch Manager (add/remove resource types)"},
			{"F6 / alt+6", "Debug log viewer"},
			{":", "Developer console"},
		}},
		{Title: "Inline Filter (/)", Bindings: []helpBinding{
			{"/", "Activate filter on focused panel"},
			{"type...", "Preview: non-matches dimmed"},
			{"Enter", "Apply: hide non-matches"},
			{"Esc", "Clear filter and exit"},
		}},
		{Title: "Lists (Resources / Revisions / Timeline)", Bindings: []helpBinding{
			{"j / k", "Move down / up"},
			{"g / G", "Go to top / bottom"},
			{"ctrl+d / pgdn", "Page down"},
			{"ctrl+u / pgup", "Page up"},
			{"Enter / space", "Select / expand"},
			{"s", "Toggle star"},
			{"c", "Mark for compare"},
		}},
		{Title: "Explorer View", Bindings: []helpBinding{
			{"S", "Toggle starred-only filter"},
			{"O", "Toggle sort order (name / time)"},
		}},
		{Title: "Timeline View", Bindings: []helpBinding{
			{"S", "Toggle starred-only filter"},
			{"R", "Reverse sort direction"},
			{"w", "Cycle time window around selected"},
		}},
		{Title: "Compare View", Bindings: []helpBinding{
			{"Tab", "Switch left / right pane"},
			{"X", "Clear compare selection"},
			{"j / k", "Scroll diff up / down"},
			{"ctrl+d / pgdn", "Page down in diff"},
			{"ctrl+u / pgup", "Page up in diff"},
		}},
		{Title: "Detail View", Bindings: []helpBinding{
			{"d", "Diff mode (YAML with highlighting)"},
			{"o", "Object mode (full YAML)"},
			{"p", "Changes mode (only fields changed vs previous)"},
			{"J", "JSON mode"},
			{"r", "Raw mode (database record)"},
			{"[ / ]", "Previous / next revision"},
			{"e", "Export YAML to file"},
			{"y", "Copy YAML to clipboard"},
			{"t", "Jump to timeline"},
		}},
		{Title: "Symbols", Bindings: []helpBinding{
			{"●  ○", "Active / idle resource"},
			{"↻", "Reconcile loop detected"},
			{"▲ / △", "High / moderate change frequency"},
			{"★", "Starred resource"},
			{"+  ~  -", "Added / Modified / Deleted event"},
			{"[C1] / [C2]", "Compare mark left / right"},
			{"╭ │ ╰", "Burst group bracket"},
			{"▸", "Window anchor position"},
			{"▲ / ▼", "Scroll indicators"},
		}},
		{Title: "Debug Tools", Bindings: []helpBinding{
			{"F6 / alt+6", "Debug log viewer"},
			{":", "Developer console"},
			{"r", "Raw mode in detail view"},
		}},
		{Title: "Console Commands (: key)", Bindings: []helpBinding{
			{"help", "Show available commands"},
			{"status", "App state summary"},
			{"resources", "List all resources with UIDs"},
			{"revisions <uid>", "List revisions for a resource"},
			{"inspect <uid>", "Show raw revision data"},
			{"store", "Dump store statistics"},
			{"kinds", "List resource kinds and counts"},
			{"sim start/stop", "Start or stop live simulation"},
			{"log <msg>", "Write a debug log entry"},
			{"clear", "Clear console output"},
		}},
	}

	keyWidth := 16
	maxDescWidth := innerWidth - keyWidth - 4 // 4 for "  " indent + "  " gap

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
				Width(keyWidth).
				Render(b.Key)
			desc := b.Desc
			if lipgloss.Width(desc) > maxDescWidth {
				desc = desc[:maxDescWidth-1] + "…"
			}
			descStyle := lipgloss.NewStyle().
				Foreground(h.theme.Text).
				Render(desc)
			lines = append(lines, "  "+keyStyle+"  "+descStyle)
		}
		lines = append(lines, "")
	}

	// Scroll
	allContent := strings.Join(lines, "\n")
	contentLines := strings.Split(allContent, "\n")
	maxVisible := max(h.height-16, 5)
	if h.scrollOffset >= len(contentLines)-maxVisible {
		h.scrollOffset = len(contentLines) - maxVisible
	}
	if h.scrollOffset < 0 {
		h.scrollOffset = 0
	}

	endIdx := min(h.scrollOffset+maxVisible, len(contentLines))
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

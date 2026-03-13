package tui

import tea "github.com/charmbracelet/bubbletea"

// CommandCategory groups related commands in the palette.
type CommandCategory string

const (
	CatNavigation CommandCategory = "Navigation"
	CatView       CommandCategory = "View"
	CatAction     CommandCategory = "Action"
	CatFilter     CommandCategory = "Filter"
	CatSystem     CommandCategory = "System"
)

// Command represents an action that can be executed from the command palette.
type Command struct {
	Name        string          // display name (used for fuzzy matching)
	Description string          // secondary text
	Category    CommandCategory // grouping in the palette
	Shortcut    string          // keybinding hint (e.g., "F1")
	Action      func() tea.Cmd  // what to do when selected
}

// CommandRegistry holds all available commands.
type CommandRegistry struct {
	commands []Command
}

// NewCommandRegistry creates a registry with all default commands.
func NewCommandRegistry() *CommandRegistry {
	cr := &CommandRegistry{}

	// Navigation
	cr.Register(Command{
		Name: "Switch to Explorer", Description: "Three-column resource browser",
		Category: CatNavigation, Shortcut: "F1",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: ExplorerView}) },
	})
	cr.Register(Command{
		Name: "Switch to Timeline", Description: "Chronological event stream",
		Category: CatNavigation, Shortcut: "F2",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: TimelineView}) },
	})
	cr.Register(Command{
		Name: "Switch to Watchlist", Description: "Starred resources only",
		Category: CatNavigation, Shortcut: "F3",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: WatchlistView}) },
	})
	cr.Register(Command{
		Name: "Switch to Compare", Description: "Side-by-side YAML comparison",
		Category: CatNavigation, Shortcut: "F4",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: CompareView}) },
	})

	// Panel focus
	cr.Register(Command{
		Name: "Focus Left Panel", Description: "Move focus to the left panel",
		Category: CatNavigation, Shortcut: "1",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelLeft}) },
	})
	cr.Register(Command{
		Name: "Focus Middle Panel", Description: "Move focus to the middle panel",
		Category: CatNavigation, Shortcut: "2",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelMiddle}) },
	})
	cr.Register(Command{
		Name: "Focus Right Panel", Description: "Move focus to the right panel",
		Category: CatNavigation, Shortcut: "3",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelRight}) },
	})
	cr.Register(Command{
		Name: "Next Panel", Description: "Cycle focus to next panel",
		Category: CatNavigation, Shortcut: "Tab",
		Action: func() tea.Cmd { return Cmd(NextPanelMsg{}) },
	})

	// View modes
	cr.Register(Command{
		Name: "View: Diff Mode", Description: "Show YAML diff with highlights",
		Category: CatView, Shortcut: "d",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: DiffMode}) },
	})
	cr.Register(Command{
		Name: "View: Object Mode", Description: "Show full YAML object",
		Category: CatView, Shortcut: "o",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: ObjectMode}) },
	})
	cr.Register(Command{
		Name: "View: Patch Mode", Description: "Show only changed fields",
		Category: CatView, Shortcut: "p",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: PatchMode}) },
	})
	cr.Register(Command{
		Name: "View: JSON Mode", Description: "Show raw JSON",
		Category: CatView, Shortcut: "J",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: JSONMode}) },
	})

	// Actions
	cr.Register(Command{
		Name: "Filter Resources", Description: "Open filter bar to search/filter",
		Category: CatFilter, Shortcut: "/",
		Action: func() tea.Cmd { return Cmd(ShowFilterMsg{}) },
	})
	cr.Register(Command{
		Name: "Quick Search", Description: "Fuzzy search for resources by name",
		Category: CatFilter, Shortcut: "//",
		Action: func() tea.Cmd { return Cmd(ShowQuickSearchMsg{}) },
	})
	cr.Register(Command{
		Name: "Clear Filter", Description: "Remove current filter",
		Category: CatFilter,
		Action:   func() tea.Cmd { return Cmd(FilterChangedMsg{Expression: ""}) },
	})
	cr.Register(Command{
		Name: "Export YAML", Description: "Save current view as YAML file",
		Category: CatAction, Shortcut: "e",
		Action: func() tea.Cmd { return Cmd(ExportYAMLMsg{}) },
	})
	cr.Register(Command{
		Name: "Copy to Clipboard", Description: "Copy current content",
		Category: CatAction, Shortcut: "y",
		Action: func() tea.Cmd { return Cmd(CopyToClipboardMsg{}) },
	})
	cr.Register(Command{
		Name: "Toggle Fullscreen", Description: "Expand focused panel",
		Category: CatView, Shortcut: "f",
		Action: func() tea.Cmd { return Cmd(ToggleFullscreenMsg{}) },
	})
	cr.Register(Command{
		Name: "Toggle Auto-Scroll", Description: "Auto-jump to newest entries",
		Category: CatView, Shortcut: "a",
		Action: func() tea.Cmd { return Cmd(ToggleAutoScrollMsg{}) },
	})
	cr.Register(Command{
		Name: "Cycle Window Mode", Description: "Filter timeline by time window (all/30s/1m/5m)",
		Category: CatView, Shortcut: "w",
		Action: func() tea.Cmd { return Cmd(ToggleWindowModeMsg{}) },
	})
	cr.Register(Command{
		Name: "Pause Recording", Description: "Pause/resume simulation (no new data generated)",
		Category: CatAction, Shortcut: "P",
		Action: func() tea.Cmd { return Cmd(TogglePauseMsg{}) },
	})
	cr.Register(Command{
		Name: "Freeze View", Description: "Freeze/unfreeze UI (data keeps recording in background)",
		Category: CatAction, Shortcut: "F5",
		Action: func() tea.Cmd { return Cmd(ToggleFreezeMsg{}) },
	})
	cr.Register(Command{
		Name: "Watch Manager", Description: "Manage watched resources (add/remove)",
		Category: CatAction, Shortcut: "W",
		Action: func() tea.Cmd { return Cmd(ShowWatchManagerMsg{}) },
	})

	// System
	cr.Register(Command{
		Name: "Show Help", Description: "Display keybinding reference",
		Category: CatSystem, Shortcut: "?",
		Action: func() tea.Cmd { return Cmd(ShowHelpMsg{}) },
	})
	cr.Register(Command{
		Name: "Quit", Description: "Exit loog",
		Category: CatSystem, Shortcut: "q",
		Action: func() tea.Cmd { return tea.Quit },
	})

	return cr
}

func (cr *CommandRegistry) Register(cmd Command) {
	cr.commands = append(cr.commands, cmd)
}

func (cr *CommandRegistry) All() []Command {
	return cr.commands
}

// Names returns all command names for fuzzy matching.
func (cr *CommandRegistry) Names() []string {
	names := make([]string, len(cr.commands))
	for i, cmd := range cr.commands {
		names[i] = cmd.Name
	}
	return names
}

package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/loog-project/loog/internal/resource"
)

// Command represents an action executable from the command palette.
type Command struct {
	Name        string
	Description string
	Shortcut    string // keybinding hint shown in the palette
	Action      func() tea.Cmd
}

// CommandRegistry holds all available commands.
type CommandRegistry struct {
	commands []Command
}

// NewCommandRegistry creates a registry with all default commands.
func NewCommandRegistry() *CommandRegistry {
	cr := &CommandRegistry{}

	cr.Register(Command{
		Name: "Switch to Explorer", Description: "Three-column resource browser", Shortcut: "F1",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: ExplorerView}) },
	})
	cr.Register(Command{
		Name: "Switch to Timeline", Description: "Chronological event stream", Shortcut: "F2",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: TimelineView}) },
	})
	cr.Register(Command{
		Name: "Switch to Compare", Description: "Side-by-side YAML comparison", Shortcut: "F3",
		Action: func() tea.Cmd { return Cmd(SwitchViewMsg{View: CompareView}) },
	})
	cr.Register(Command{
		Name: "Focus Left Panel", Description: "Move focus to the left panel", Shortcut: "1",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelLeft}) },
	})
	cr.Register(Command{
		Name: "Focus Middle Panel", Description: "Move focus to the middle panel", Shortcut: "2",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelMiddle}) },
	})
	cr.Register(Command{
		Name: "Focus Right Panel", Description: "Move focus to the right panel", Shortcut: "3",
		Action: func() tea.Cmd { return Cmd(FocusPanelMsg{Panel: PanelRight}) },
	})
	cr.Register(Command{
		Name: "Next Panel", Description: "Cycle focus to next panel", Shortcut: "Tab",
		Action: func() tea.Cmd { return Cmd(NextPanelMsg{}) },
	})
	cr.Register(Command{
		Name: "View: Diff Mode", Description: "Show YAML diff with highlights", Shortcut: "d",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: DiffMode}) },
	})
	cr.Register(Command{
		Name: "View: Object Mode", Description: "Show full YAML object", Shortcut: "o",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: ObjectMode}) },
	})
	cr.Register(Command{
		Name: "View: Patch Mode", Description: "Show only changed fields", Shortcut: "p",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: PatchMode}) },
	})
	cr.Register(Command{
		Name: "View: JSON Mode", Description: "Show raw JSON", Shortcut: "J",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: JSONMode}) },
	})
	cr.Register(Command{
		Name: "View: Raw Mode", Description: "Show raw database record", Shortcut: "r",
		Action: func() tea.Cmd { return Cmd(ViewModeChangedMsg{Mode: RawMode}) },
	})
	cr.Register(Command{
		Name: "Quick Jump", Description: "Fuzzy jump to a resource by name", Shortcut: "//",
		Action: func() tea.Cmd { return Cmd(ShowQuickJumpMsg{}) },
	})
	cr.Register(Command{
		Name: "Toggle Fullscreen", Description: "Expand focused panel", Shortcut: "f",
		Action: func() tea.Cmd { return Cmd(ToggleFullscreenMsg{}) },
	})
	cr.Register(Command{
		Name: "Toggle Auto-Scroll", Description: "Auto-jump to newest entries", Shortcut: "a",
		Action: func() tea.Cmd { return Cmd(ToggleAutoScrollMsg{}) },
	})
	cr.Register(Command{
		Name: "Cycle Window Mode", Description: "Filter timeline by time window", Shortcut: "w",
		Action: func() tea.Cmd { return Cmd(ToggleWindowModeMsg{}) },
	})
	cr.Register(Command{
		Name: "Pause Recording", Description: "Pause/resume data collection", Shortcut: "P",
		Action: func() tea.Cmd { return Cmd(TogglePauseMsg{}) },
	})
	cr.Register(Command{
		Name: "Freeze View", Description: "Freeze UI while data keeps recording", Shortcut: "F5",
		Action: func() tea.Cmd { return Cmd(ToggleFreezeMsg{}) },
	})
	cr.Register(Command{
		Name: "Watch Manager", Description: "Manage watched resources (add/remove)", Shortcut: "W",
		Action: func() tea.Cmd { return Cmd(ShowWatchManagerMsg{}) },
	})
	cr.Register(Command{
		Name: "Show Help", Description: "Display keybinding reference", Shortcut: "?",
		Action: func() tea.Cmd { return Cmd(ShowHelpMsg{}) },
	})
	cr.Register(Command{
		Name: "Quit", Description: "Exit loog", Shortcut: "q",
		Action: func() tea.Cmd { return tea.Quit },
	})
	cr.Register(Command{
		Name: "Toggle Starred Only", Description: "Show only starred resources in timeline", Shortcut: "S",
		Action: func() tea.Cmd { return Cmd(ToggleTimelineStarredMsg{}) },
	})
	cr.Register(Command{
		Name: "Clear Compare", Description: "Remove both compare marks", Shortcut: "X",
		Action: func() tea.Cmd { return Cmd(CompareClearMsg{}) },
	})
	cr.Register(Command{
		Name: "Debug Log", Description: "View internal debug log", Shortcut: "F6",
		Action: func() tea.Cmd { return Cmd(ShowDebugLogMsg{}) },
	})
	cr.Register(Command{
		Name: "Developer Console", Description: "Interactive console for inspecting data", Shortcut: ":",
		Action: func() tea.Cmd { return Cmd(ShowDevConsoleMsg{}) },
	})

	return cr
}

func (cr *CommandRegistry) Register(cmd Command) {
	cr.commands = append(cr.commands, cmd)
}

func (cr *CommandRegistry) All() []Command {
	return cr.commands
}

func (cr *CommandRegistry) Names() []string {
	names := make([]string, len(cr.commands))
	for i, cmd := range cr.commands {
		names[i] = cmd.Name
	}
	return names
}

// exportYAMLCmd writes the current revision's object as YAML to a temporary file.
func exportYAMLCmd(rd *resource.Data, revIdx int) tea.Cmd {
	if rd == nil || revIdx < 0 || revIdx >= len(rd.Revisions) {
		return Cmd(StatusMsg{Text: "No revision to export", IsError: true})
	}
	rev := rd.Revisions[revIdx]
	obj := rev.Object
	if obj == nil {
		return Cmd(StatusMsg{Text: "Revision has no object data", IsError: true})
	}
	return func() tea.Msg {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("YAML marshal error: %v", err), IsError: true}
		}

		filename := fmt.Sprintf("loog-export-%s-%s-%s.yaml",
			rd.Resource.Kind, rd.Resource.Name, rev.ID.String())
		if writeErr := os.WriteFile(filename, data, 0o644); writeErr != nil {
			return StatusMsg{Text: fmt.Sprintf("Write error: %v", writeErr), IsError: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Exported to %s", filename)}
	}
}

// copyToClipboardCmd copies the current revision's object as YAML to the system clipboard.
func copyToClipboardCmd(rd *resource.Data, revIdx int) tea.Cmd {
	if rd == nil || revIdx < 0 || revIdx >= len(rd.Revisions) {
		return Cmd(StatusMsg{Text: "No revision to copy", IsError: true})
	}
	rev := rd.Revisions[revIdx]
	obj := rev.Object
	if obj == nil {
		return Cmd(StatusMsg{Text: "Revision has no object data", IsError: true})
	}
	return func() tea.Msg {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("YAML marshal error: %v", err), IsError: true}
		}

		if err := writeClipboard(data); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Clipboard error: %v", err), IsError: true}
		}
		return StatusMsg{Text: "Copied YAML to clipboard"}
	}
}

// writeClipboard writes data to the system clipboard using platform-specific tools.
func writeClipboard(data []byte) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		// Try xclip first, fall back to xsel
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip or xsel)")
		}
	case "windows":
		cmd = exec.Command("clip.exe")
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	cmd.Stdin = bytes.NewReader(data)
	return cmd.Run()
}

package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
)

// Navigation messages

type SwitchViewMsg struct{ View ViewID }
type FocusPanelMsg struct{ Panel PanelID }
type NextPanelMsg struct{}
type PrevPanelMsg struct{}
type ToggleFullscreenMsg struct{}

// Data selection messages

type ResourceSelectedMsg struct{ Resource *resource.Data }
type RevisionSelectedMsg struct {
	Resource *resource.Data
	Index    int
}
type TimelineEntrySelectedMsg struct{ Entry resource.TimelineEntry }

// State change messages

type ToggleStarMsg struct{ UID string }
type ViewModeChangedMsg struct{ Mode ViewMode }
type CompareMarkMsg struct {
	Resource *resource.Data
	Index    int
}
type JumpToTimelineMsg struct{ Entry resource.TimelineEntry }

// Overlay messages

type ShowCommandPaletteMsg struct{}
type HideOverlayMsg struct{}
type ShowHelpMsg struct{}
type ShowQuickSearchMsg struct{}
type ShowWatchManagerMsg struct{}
type ShowDebugLogMsg struct{}
type ShowDevConsoleMsg struct{}

// Watch management messages

type AddWatchKindMsg struct{ Kind resource.Kind }
type RemoveWatchKindMsg struct{ Kind string }

// Action messages

type StatusMsg struct {
	Text    string
	IsError bool
}

// ExportYAMLMsg requests exporting the current revision's object as a YAML file.
type ExportYAMLMsg struct {
	Resource *resource.Data
	RevIndex int
}

// CopyToClipboardMsg requests copying the current revision's object as YAML to the system clipboard.
type CopyToClipboardMsg struct {
	Resource *resource.Data
	RevIndex int
}

// Analysis and simulation messages

type AnalysisCompleteMsg struct {
	Result resource.AnalysisResult
}

type SimulationTickMsg struct {
	ResourceUID string
}

type ToggleAutoScrollMsg struct{}
type ToggleWindowModeMsg struct{}
type TogglePauseMsg struct{}
type ToggleFreezeMsg struct{}
type ToggleTimelineStarredMsg struct{}
type ToggleExplorerStarredMsg struct{}
type CompareClearMsg struct{}

// Cmd wraps a message as a tea.Cmd.
func Cmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

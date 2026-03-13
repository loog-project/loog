package tui

import tea "github.com/charmbracelet/bubbletea"

// --- Navigation Messages ---

type SwitchViewMsg struct{ View ViewID }
type FocusPanelMsg struct{ Panel PanelID }
type NextPanelMsg struct{}
type PrevPanelMsg struct{}
type ToggleFullscreenMsg struct{}

// --- Data Selection Messages ---

type ResourceSelectedMsg struct{ Resource *ResourceData }
type RevisionSelectedMsg struct {
	Resource *ResourceData
	Index    int
}
type TimelineEntrySelectedMsg struct{ Entry TimelineEntry }

// --- State Change Messages ---

type ToggleStarMsg struct{ UID string }
type ViewModeChangedMsg struct{ Mode ViewMode }
type FilterChangedMsg struct{ Expression string }
type CompareMarkMsg struct {
	Resource *ResourceData
	Index    int
}
type JumpToTimelineMsg struct{ Entry TimelineEntry }

// --- Overlay Messages ---

type ShowCommandPaletteMsg struct{}
type HideOverlayMsg struct{}
type ShowHelpMsg struct{}
type ShowFilterMsg struct{}
type ShowQuickSearchMsg struct{}
type ShowWatchManagerMsg struct{}
type ShowDebugLogMsg struct{}
type ShowDevConsoleMsg struct{}

// --- Watch Management Messages ---

type AddWatchKindMsg struct{ Kind ResourceKind }
type RemoveWatchKindMsg struct{ Kind string }

// --- Action Messages ---

type StatusMsg struct {
	Text    string
	IsError bool
}

// ExportYAMLMsg requests exporting the current revision's object as a YAML file.
type ExportYAMLMsg struct {
	Resource *ResourceData
	RevIndex int
}

// CopyToClipboardMsg requests copying the current revision's object as YAML to the system clipboard.
type CopyToClipboardMsg struct {
	Resource *ResourceData
	RevIndex int
}

// --- Analysis / Simulation Messages ---

type AnalysisCompleteMsg struct {
	Result AnalysisResult
}

type SimulationTickMsg struct {
	ResourceUID string
}

type ToggleAutoScrollMsg struct{}
type ToggleWindowModeMsg struct{}
type TogglePauseMsg struct{}
type ToggleFreezeMsg struct{}
type ToggleTimelineStarredMsg struct{}
type CompareClearMsg struct{}

// Cmd wraps a message as a tea.Cmd.
func Cmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return msg
	}
}

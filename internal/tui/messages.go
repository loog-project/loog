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
	Index    int // index into ResourceData.Revisions
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

// --- Watch Management Messages ---

// AddWatchKindMsg requests adding a resource type (kind) to active watching.
type AddWatchKindMsg struct{ Kind ResourceKind }

// RemoveWatchKindMsg requests removing all resources of a kind from active watching.
type RemoveWatchKindMsg struct{ Kind string }

// --- Action Messages ---

type ExportYAMLMsg struct{}
type CopyToClipboardMsg struct{}
type StatusMsg struct {
	Text    string
	IsError bool
}

// --- Tree Messages ---

type ToggleExpandMsg struct{ Kind string }

// --- Analysis / Simulation Messages ---

// AnalysisCompleteMsg is sent when background analysis finishes.
type AnalysisCompleteMsg struct {
	Result AnalysisResult
}

// SimulationTickMsg requests generating a new revision for a resource.
type SimulationTickMsg struct {
	ResourceUID string
}

// NewRevisionMsg is sent when a new revision has been added to the store.
type NewRevisionMsg struct {
	ResourceUID string
	Revision    Revision
}

// ToggleAutoScrollMsg toggles auto-scroll mode.
type ToggleAutoScrollMsg struct{}

// ToggleWindowModeMsg cycles the rolling window filter.
type ToggleWindowModeMsg struct{}

// TogglePauseMsg pauses/resumes recording (simulation stops generating new data).
type TogglePauseMsg struct{}

// ToggleFreezeMsg freezes/unfreezes the view (data keeps arriving but UI doesn't update).
type ToggleFreezeMsg struct{}

// --- Utility ---

// Cmd wraps a message as a tea.Cmd for convenience.
func Cmd(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// Batch combines multiple tea.Cmd into one.
func Batch(cmds ...tea.Cmd) tea.Cmd {
	return tea.Batch(cmds...)
}

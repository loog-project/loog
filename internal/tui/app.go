package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/internal/adapter"
)

// App is the root tea.Model that wires everything together.
type App struct {
	width, height int
	theme         Theme
	store         Store

	// Chrome
	header    *Header
	statusBar *StatusBar

	// Views
	activeView ViewID
	explorer   *ExplorerViewComponent
	timeline   *TimelineViewComponent
	compare    *CompareViewComponent

	// Overlays
	commandPalette *CommandPalette
	helpOverlay    *HelpOverlay
	quickSearch    *QuickSearch
	watchManager   *WatchManager
	debugLogViewer *DebugLogViewer
	devConsole     *DevConsole
	registry       *CommandRegistry

	// Debug log
	debugLog *DebugLog

	// State
	fullscreen          bool
	timelineStarredOnly bool
	statusText          string
	statusErr           bool
	statusTime          time.Time
	ready               bool

	// Auto-scroll
	autoScroll bool

	// Window mode
	windowMode   WindowMode
	windowAnchor time.Time // timestamp the window is centered on

	// Simulation
	simulator  Simulator // nil when running against a production store
	simulating bool

	// Recording: live data arriving from production watcher.
	// When true, the TUI is in "recording" mode (not simulation).
	recording bool

	// Freeze: data keeps arriving but the UI doesn't update.
	// Buffered revisions are flushed when unfrozen.
	frozen        bool
	pendingBuffer []pendingRevision // revisions that arrived while frozen

	// Analysis results cache
	analysisResults map[string]AnalysisResult // resourceUID -> result

	// External callbacks for watch kind management (production mode wiring)
	onWatchKindAdded   func(rk ResourceKind) // called when user adds a watch kind
	onWatchKindRemoved func(kind string)     // called when user removes a watch kind
}

// pendingRevision holds a revision that arrived during freeze.
type pendingRevision struct {
	ResourceUID string
	Revision    Revision
}

// AppOption configures optional behavior for the App.
type AppOption func(*App)

// WithSimulator enables live simulation data generation.
// When a Simulator is provided, the TUI starts generating new revisions automatically.
func WithSimulator(sim Simulator) AppOption {
	return func(a *App) {
		a.simulator = sim
		a.simulating = true
	}
}

// WithRecording marks the app as receiving live data from a production watcher.
// The TUI starts in "recording" mode: the status bar shows recording state,
// and the pause key can pause the watcher.
func WithRecording() AppOption {
	return func(a *App) {
		a.recording = true
		a.simulating = true // reuse simulating flag for "active data flow" state
	}
}

// WithWatchCallbacks sets callbacks invoked when the user adds or removes a watched kind.
// In production mode, these trigger mux.Add() / mux.Remove() on the dynamic informer.
func WithWatchCallbacks(onAdd func(rk ResourceKind), onRemove func(kind string)) AppOption {
	return func(a *App) {
		a.onWatchKindAdded = onAdd
		a.onWatchKindRemoved = onRemove
	}
}

// NewApp creates the root application model.
func NewApp(store Store, opts ...AppOption) *App {
	theme := CatppuccinMocha
	registry := NewCommandRegistry()
	debugLog := NewDebugLog(500)

	app := &App{
		theme:           theme,
		store:           store,
		registry:        registry,
		debugLog:        debugLog,
		analysisResults: make(map[string]AnalysisResult),

		// Chrome
		header:    NewHeader(theme),
		statusBar: NewStatusBar(theme),

		// Views
		activeView: ExplorerView,
		explorer:   NewExplorerViewComponent(theme),
		timeline:   NewTimelineViewComponent(theme),
		compare:    NewCompareViewComponent(theme),

		// Overlays
		commandPalette: NewCommandPalette(theme, registry),
		helpOverlay:    NewHelpOverlay(theme),
		quickSearch:    NewQuickSearch(theme),
		watchManager:   NewWatchManager(theme),
		debugLogViewer: NewDebugLogViewer(theme, debugLog),
		devConsole:     NewDevConsole(theme, store, debugLog),
	}

	// Apply options (e.g. WithSimulator)
	for _, opt := range opts {
		opt(app)
	}

	// Initialize views with data
	app.explorer.SetGroups(store.KindGroups())
	app.explorer.SetFocusPanel(PanelLeft)

	app.timeline.SetEntries(store.Timeline())
	app.timeline.SetStore(store)
	app.timeline.SetFocusPanel(PanelLeft)

	// Initialize status bar counts
	app.statusBar.SetCounts(
		store.TotalResourceCount(),
		store.TotalRevisionCount(),
		len(store.StarredResources()),
	)
	app.statusBar.SetSimulating(app.simulating)
	app.statusBar.SetSimMode(app.simulator != nil)

	debugLog.Info("app", "loog TUI started with %d resources, %d revisions",
		store.TotalResourceCount(), store.TotalRevisionCount())

	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
		tickCmd(),
	}
	// Start simulation if enabled
	if a.simulating && a.simulator != nil {
		cmds = append(cmds, a.simulator.ScheduleNextTick())
	}
	return tea.Batch(cmds...)
}

// tickCmd sends a tick every second for clock updates.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type tickMsg time.Time

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.layout()
		return a, nil

	case tickMsg:
		// Clear status message after 3 seconds
		if a.statusText != "" && time.Since(a.statusTime) > 3*time.Second {
			a.statusText = ""
			a.statusBar.SetStatus("", false)
		}
		// Update hint from focused component
		a.updateHint()
		return a, tickCmd()

	case tea.KeyMsg:
		// Overlays get first priority
		if a.commandPalette.IsVisible() {
			cmd := a.commandPalette.Update(msg)
			return a, cmd
		}
		if a.helpOverlay.IsVisible() {
			cmd := a.helpOverlay.Update(msg)
			return a, cmd
		}
		if a.quickSearch.IsVisible() {
			cmd := a.quickSearch.Update(msg)
			return a, cmd
		}
		if a.watchManager.IsVisible() {
			cmd := a.watchManager.Update(msg)
			return a, cmd
		}
		if a.debugLogViewer.IsVisible() {
			cmd := a.debugLogViewer.Update(msg)
			return a, cmd
		}
		if a.devConsole.IsVisible() {
			cmd := a.devConsole.Update(msg)
			return a, cmd
		}

		// When a panel's inline filter is being edited, bypass all global keybindings
		// (except ctrl+c) and delegate directly to the view's Update handler.
		if a.isFilterEditing() {
			if key.Matches(msg, GlobalKeyMap.Quit) && msg.String() == "ctrl+c" {
				return a, tea.Quit
			}
			cmd := a.updateActiveView(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			a.updateHint()
			return a, tea.Batch(cmds...)
		}

		// Global keybindings
		switch {
		case key.Matches(msg, GlobalKeyMap.Quit):
			return a, tea.Quit

		case key.Matches(msg, GlobalKeyMap.CommandPalette):
			a.commandPalette.Show()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.Help):
			a.helpOverlay.Show()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.Filter):
			// Activate per-panel inline filter on the focused panel
			switch a.activeView {
			case ExplorerView:
				if a.explorer.StartFilter() {
					a.updateHint()
				}
			case TimelineView:
				if a.timeline.StartFilter() {
					a.updateHint()
				}
			}
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewExplorer):
			a.switchView(ExplorerView)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewTimeline):
			a.switchView(TimelineView)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewCompare):
			a.switchView(CompareView)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.NextPanel):
			a.nextPanel()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.PrevPanel):
			a.prevPanel()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.FocusLeft):
			a.focusPanel(PanelLeft)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.FocusMiddle):
			a.focusPanel(PanelMiddle)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.FocusRight):
			a.focusPanel(PanelRight)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.Fullscreen):
			a.fullscreen = !a.fullscreen
			if a.fullscreen {
				a.setStatus("Fullscreen: ON (press f to exit)", false)
			} else {
				a.setStatus("Fullscreen: OFF", false)
				a.layout() // Restore normal layout sizes
			}
			return a, nil

		case key.Matches(msg, GlobalKeyMap.AutoScroll):
			a.autoScroll = !a.autoScroll
			a.explorer.SetAutoScroll(a.autoScroll)
			a.timeline.SetAutoScroll(a.autoScroll)
			a.statusBar.SetAutoScroll(a.autoScroll)
			if a.autoScroll {
				a.setStatus("Auto-scroll: ON", false)
			} else {
				a.setStatus("Auto-scroll: OFF", false)
			}
			return a, nil

		case key.Matches(msg, GlobalKeyMap.WindowMode):
			a.windowMode = NextWindowMode(a.windowMode)
			a.timeline.SetWindowAnchor(a.windowAnchor)
			a.timeline.SetWindowMode(a.windowMode)
			a.statusBar.SetWindowMode(a.windowMode)
			if a.windowMode == WindowAll {
				a.setStatus("Window mode: all (showing everything)", false)
			} else if a.windowAnchor.IsZero() {
				a.setStatus("Window mode: "+a.windowMode.String()+" (select a revision first for anchor)", false)
			} else {
				a.setStatus("Window mode: "+a.windowMode.String()+" around "+FormatTimestamp(a.windowAnchor), false)
			}
			return a, nil

		case key.Matches(msg, GlobalKeyMap.PauseRecording):
			return a, Cmd(TogglePauseMsg{})

		case key.Matches(msg, GlobalKeyMap.FreezeView):
			return a, Cmd(ToggleFreezeMsg{})

		case key.Matches(msg, GlobalKeyMap.WatchManager):
			a.watchManager.Show(a.store, a.store.UnwatchedKinds())
			return a, nil

		case key.Matches(msg, GlobalKeyMap.DebugLog):
			a.debugLogViewer.Show()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.DevConsole):
			a.devConsole.Show()
			return a, nil
		}

		// Delegate to active view
		cmd := a.updateActiveView(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	// Handle application messages
	case SwitchViewMsg:
		a.switchView(msg.View)

	case FocusPanelMsg:
		a.focusPanel(msg.Panel)

	case NextPanelMsg:
		a.nextPanel()

	case PrevPanelMsg:
		a.prevPanel()

	case ResourceSelectedMsg:
		a.explorer.SetResource(msg.Resource)
		if msg.Resource != nil {
			a.debugLog.Debug("app", "resource selected: %s (%d revs)",
				msg.Resource.Resource.KindName(), msg.Resource.RevisionCount())
			a.statusBar.SetResourceInfo(msg.Resource.Resource.KindName())
			revCount := msg.Resource.RevisionCount()
			a.statusBar.SetRevisionInfo(fmt.Sprintf("%d revisions", revCount))

			// Trigger async analysis
			cmds = append(cmds, RunAnalysisCmd(msg.Resource))
		}
		// Pass compare marks to views
		a.syncCompareMarks()

	case RevisionSelectedMsg:
		a.explorer.SetRevision(msg.Resource, msg.Index)
		if msg.Index >= 0 && msg.Index < len(msg.Resource.Revisions) {
			rev := msg.Resource.Revisions[msg.Index]
			a.debugLog.Debug("app", "revision selected: %s [%d] %s",
				msg.Resource.Resource.KindName(), msg.Index, rev.ID.String())
			a.statusBar.SetRevisionInfo(rev.ID.String())
			// Update window anchor to selected revision's timestamp
			a.windowAnchor = rev.Time
			if a.windowMode != WindowAll {
				a.timeline.SetWindowAnchor(a.windowAnchor)
			}
		}

	case TimelineEntrySelectedMsg:
		a.timeline.SelectEntry(msg.Entry)
		// Update window anchor
		a.windowAnchor = msg.Entry.Revision.Time
		if a.windowMode != WindowAll {
			a.timeline.SetWindowAnchor(a.windowAnchor)
		}

	case ToggleStarMsg:
		a.toggleStar(msg.UID)

	case ViewModeChangedMsg:
		a.statusBar.SetViewMode(msg.Mode)
		// Also change the detail view in the active view
		switch a.activeView {
		case ExplorerView:
			a.explorer.detail.SetViewMode(msg.Mode)
		case TimelineView:
			a.timeline.detail.SetViewMode(msg.Mode)
		}

	case CompareMarkMsg:
		if msg.Resource != nil && msg.Index >= 0 && msg.Index < len(msg.Resource.Revisions) {
			item := CompareItem{
				Resource: msg.Resource.Resource,
				Revision: msg.Resource.Revisions[msg.Index],
			}
			a.compare.AddItem(item)
			a.setStatus("Marked for compare: "+item.Resource.KindName()+" @ "+item.Revision.ID.String(), false)
			a.syncCompareMarks()
		}

	case JumpToTimelineMsg:
		a.switchView(TimelineView)
		// Try to find and select the matching timeline entry
		a.timeline.ScrollToEntry(msg.Entry)

	case ShowCommandPaletteMsg:
		a.commandPalette.Show()

	case HideOverlayMsg:
		// overlays handle their own hiding

	case ShowHelpMsg:
		a.helpOverlay.Show()

	case ShowQuickSearchMsg:
		a.quickSearch.Show(a.store.AllResources())

	case ShowWatchManagerMsg:
		a.watchManager.Show(a.store, a.store.UnwatchedKinds())

	case ShowDebugLogMsg:
		a.debugLogViewer.Show()

	case ShowDevConsoleMsg:
		a.devConsole.Show()

	case AddWatchKindMsg:
		created := a.store.AddWatchKind(msg.Kind)
		// Notify external watcher (e.g. mux.Add) so real data starts flowing
		if a.onWatchKindAdded != nil {
			a.onWatchKindAdded(msg.Kind)
		}
		a.store.RebuildKindGroups()
		a.refreshExplorerGroups()
		a.refreshTimeline()
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		a.setStatus(fmt.Sprintf("Now watching: %s (%d resources added)", msg.Kind.Kind, len(created)), false)

	case RemoveWatchKindMsg:
		count := a.store.ResourceCountByKind(msg.Kind)
		// Check if the currently selected resource is of this kind
		selectedUID := a.explorer.tree.SelectedUID()
		selectedIsOfKind := false
		if rd := a.store.GetResource(selectedUID); rd != nil && rd.Resource.Kind == msg.Kind {
			selectedIsOfKind = true
		}

		a.store.RemoveWatchKind(msg.Kind)
		// Notify external watcher (e.g. mux.Remove) so it stops watching this kind
		if a.onWatchKindRemoved != nil {
			a.onWatchKindRemoved(msg.Kind)
		}
		a.store.RebuildKindGroups()
		a.refreshExplorerGroups()
		a.refreshTimeline()
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		if selectedIsOfKind {
			a.explorer.SetResource(nil)
		}
		a.setStatus(fmt.Sprintf("Unwatched: %s (%d resources removed)", msg.Kind, count), false)

	case StatusMsg:
		a.setStatus(msg.Text, msg.IsError)

	case ExportYAMLMsg:
		return a, exportYAMLCmd(msg.Resource, msg.RevIndex)

	case CopyToClipboardMsg:
		return a, copyToClipboardCmd(msg.Resource, msg.RevIndex)

	case ToggleTimelineStarredMsg:
		a.timelineStarredOnly = !a.timelineStarredOnly
		a.timeline.timeline.SetStarredOnly(a.timelineStarredOnly)
		a.refreshTimeline()
		if a.timelineStarredOnly {
			a.setStatus("Timeline: showing starred resources only", false)
		} else {
			a.setStatus("Timeline: showing all resources", false)
		}

	case ToggleExplorerStarredMsg:
		starredOnly := a.explorer.tree.ToggleStarredOnly()
		// If selected resource is no longer visible, deselect it
		if starredOnly {
			uid := a.explorer.tree.SelectedUID()
			if rd := a.store.GetResource(uid); rd != nil && !rd.Resource.Starred {
				a.explorer.SetResource(nil)
			}
			a.setStatus("Explorer: showing starred resources only", false)
		} else {
			a.setStatus("Explorer: showing all resources", false)
		}

	case CompareClearMsg:
		a.compare.Clear()
		a.syncCompareMarks()
		a.setStatus("Compare selection cleared", false)

	case ToggleFullscreenMsg:
		a.fullscreen = !a.fullscreen
		if !a.fullscreen {
			a.layout()
		}

	case ToggleAutoScrollMsg:
		a.autoScroll = !a.autoScroll
		a.explorer.SetAutoScroll(a.autoScroll)
		a.timeline.SetAutoScroll(a.autoScroll)
		a.statusBar.SetAutoScroll(a.autoScroll)

	case ToggleWindowModeMsg:
		a.windowMode = NextWindowMode(a.windowMode)
		a.timeline.SetWindowAnchor(a.windowAnchor)
		a.timeline.SetWindowMode(a.windowMode)
		a.statusBar.SetWindowMode(a.windowMode)

	case TogglePauseMsg:
		a.simulating = !a.simulating
		a.header.SetRecording(a.simulating)
		a.statusBar.SetSimulating(a.simulating)
		if a.simulating {
			a.setStatus("Recording resumed", false)
			// Schedule a new simulation tick to restart generation (simulation mode only)
			if a.simulator != nil {
				cmds = append(cmds, a.simulator.ScheduleNextTick())
			}
		} else {
			a.setStatus("Recording paused", false)
		}

	case ToggleFreezeMsg:
		a.frozen = !a.frozen
		a.header.SetFrozen(a.frozen)
		if a.frozen {
			a.setStatus("View frozen — data continues recording in background", false)
		} else {
			a.setStatus(fmt.Sprintf("View unfrozen — flushing %d buffered revisions", len(a.pendingBuffer)), false)
			a.flushPendingBuffer()
		}

	case AnalysisCompleteMsg:
		a.analysisResults[msg.Result.ResourceUID] = msg.Result
		a.syncAnalysisTags()

	case SimulationTickMsg:
		a.handleSimulationTick(msg.ResourceUID)
		// Schedule next tick
		if a.simulating && a.simulator != nil {
			cmds = append(cmds, a.simulator.ScheduleNextTick())
		}

	case adapter.LiveRevisionMsg:
		a.handleLiveRevision(msg.ResourceUID)
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	// Build main content
	headerView := a.header.View()
	statusView := a.statusBar.View()

	// Status bar is always 2 lines for stable layout.
	// Content height = total - header(1) - status(2)
	contentHeight := a.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	var contentView string
	if a.fullscreen {
		contentView = a.viewFullscreen(contentHeight)
	} else {
		switch a.activeView {
		case ExplorerView:
			contentView = a.explorer.View()
		case TimelineView:
			contentView = a.timeline.View()
		case CompareView:
			contentView = a.compare.View()
		}
	}

	// Stack: header + content + status
	mainView := lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		contentView,
		statusView,
	)

	// Overlay rendering
	if a.commandPalette.IsVisible() {
		paletteView := a.commandPalette.View()
		mainView = ModalOverlay(paletteView, mainView, a.theme)
	}
	if a.helpOverlay.IsVisible() {
		helpView := a.helpOverlay.View()
		mainView = ModalOverlay(helpView, mainView, a.theme)
	}
	if a.quickSearch.IsVisible() {
		qsView := a.quickSearch.View()
		mainView = ModalOverlay(qsView, mainView, a.theme)
	}
	if a.watchManager.IsVisible() {
		wmView := a.watchManager.View()
		mainView = ModalOverlay(wmView, mainView, a.theme)
	}
	if a.debugLogViewer.IsVisible() {
		dlView := a.debugLogViewer.View()
		mainView = ModalOverlay(dlView, mainView, a.theme)
	}
	if a.devConsole.IsVisible() {
		dcView := a.devConsole.View()
		mainView = ModalOverlay(dcView, mainView, a.theme)
	}

	return mainView
}

// --- Internal Methods ---

func (a *App) layout() {
	a.header.SetSize(a.width)
	a.statusBar.SetSize(a.width)

	// Status bar is always 2 lines (status + hint) for stable layout.
	// Content height = total - header(1) - status(2)
	contentHeight := a.height - 3
	if contentHeight < 1 {
		contentHeight = 1
	}

	a.explorer.SetSize(a.width, contentHeight)
	a.timeline.SetSize(a.width, contentHeight)
	a.compare.SetSize(a.width, contentHeight)

	a.commandPalette.SetSize(a.width, a.height)
	a.helpOverlay.SetSize(a.width, a.height)
	a.quickSearch.SetSize(a.width, a.height)
	a.watchManager.SetSize(a.width, a.height)
	a.debugLogViewer.SetSize(a.width, a.height)
	a.devConsole.SetSize(a.width, a.height)
}

func (a *App) switchView(v ViewID) {
	a.debugLog.Debug("app", "switch view: %s → %s", a.activeView, v)
	a.activeView = v
	a.header.SetView(v)
}

func (a *App) nextPanel() {
	switch a.activeView {
	case ExplorerView:
		a.explorer.NextPanel()
	case TimelineView:
		a.timeline.NextPanel()
	}
}

func (a *App) prevPanel() {
	switch a.activeView {
	case ExplorerView:
		a.explorer.PrevPanel()
	case TimelineView:
		a.timeline.PrevPanel()
	}
}

func (a *App) focusPanel(p PanelID) {
	switch a.activeView {
	case ExplorerView:
		a.explorer.SetFocusPanel(p)
	case TimelineView:
		if p == PanelMiddle || p == PanelRight {
			a.timeline.SetFocusPanel(PanelRight)
		} else {
			a.timeline.SetFocusPanel(PanelLeft)
		}
	}
}

// isFilterEditing returns true if the active view's focused panel has an inline filter being edited.
func (a *App) isFilterEditing() bool {
	switch a.activeView {
	case ExplorerView:
		return a.explorer.IsFilterEditing()
	case TimelineView:
		return a.timeline.IsFilterEditing()
	}
	return false
}

func (a *App) updateActiveView(msg tea.Msg) tea.Cmd {
	switch a.activeView {
	case ExplorerView:
		return a.explorer.Update(msg)
	case TimelineView:
		return a.timeline.Update(msg)
	case CompareView:
		return a.compare.Update(msg)
	}
	return nil
}

func (a *App) toggleStar(uid string) {
	starred := a.store.ToggleStar(uid)
	rd := a.store.GetResource(uid)
	if rd == nil {
		return
	}

	// Refresh kind groups (to update star indicators)
	a.store.RebuildKindGroups()
	a.refreshExplorerGroups()
	a.refreshTimeline()
	a.statusBar.SetCounts(
		a.store.TotalResourceCount(),
		a.store.TotalRevisionCount(),
		len(a.store.StarredResources()),
	)
	// If starred-only is active and we just unstarred the selected resource,
	// deselect it so the detail panel doesn't show a hidden resource.
	if !starred && a.explorer.tree.StarredOnly() && a.explorer.tree.SelectedUID() == uid {
		a.explorer.SetResource(nil)
	}

	if starred {
		a.setStatus("Starred: "+rd.Resource.KindName(), false)
	} else {
		a.setStatus("Unstarred: "+rd.Resource.KindName(), false)
	}
}

func (a *App) applyFilter() {
	a.refreshExplorerGroups()
	a.refreshTimeline()
}

// refreshExplorerGroups rebuilds the explorer tree with unfiltered data.
func (a *App) refreshExplorerGroups() {
	a.explorer.SetGroups(a.store.KindGroups())
}

// refreshTimeline rebuilds the timeline entries applying starred-only filter.
func (a *App) refreshTimeline() {
	entries := a.store.FilterTimeline("", a.timelineStarredOnly)
	a.timeline.SetEntries(entries)
}

func (a *App) setStatus(text string, isErr bool) {
	a.statusText = text
	a.statusErr = isErr
	a.statusTime = time.Now()
	a.statusBar.SetStatus(text, isErr)
}

// viewFullscreen renders the active view's focused panel at full size.
func (a *App) viewFullscreen(contentHeight int) string {
	switch a.activeView {
	case ExplorerView:
		return a.explorer.ViewFullscreen(a.width, contentHeight)
	case TimelineView:
		return a.timeline.ViewFullscreen(a.width, contentHeight)
	case CompareView:
		return a.compare.ViewFullscreen(a.width, contentHeight)
	}
	return ""
}

// syncCompareMarks propagates compare selection to all views.
func (a *App) syncCompareMarks() {
	sel := a.compare.Selection()
	a.explorer.SetCompareMarks(sel.Left, sel.Right)
	a.timeline.SetCompareMarks(sel.Left, sel.Right)
}

// syncAnalysisTags propagates analysis results to view components.
func (a *App) syncAnalysisTags() {
	// Build per-resource tag summary (just the latest revision tags for tree display)
	resourceTags := make(map[string][]ChangeTag)
	for uid, result := range a.analysisResults {
		if rd := a.store.GetResource(uid); rd != nil && len(rd.Revisions) > 0 {
			latest := rd.Revisions[len(rd.Revisions)-1]
			if tags, ok := result.Tags[latest.ID]; ok {
				resourceTags[uid] = tags
			}
		}
	}

	// Pass to explorer (the currently selected resource's revision tags)
	a.explorer.SetAnalysisTags(resourceTags, a.currentRevisionTags())
}

// currentRevisionTags returns revision-level tags for whatever resource is selected.
func (a *App) currentRevisionTags() map[RevisionID][]ChangeTag {
	// Find the selected resource UID from the explorer tree
	uid := a.explorer.tree.SelectedUID()
	if result, ok := a.analysisResults[uid]; ok {
		return result.Tags
	}
	return nil
}

// updateHint collects the current hint from the active view's focused component.
func (a *App) updateHint() {
	var hint string
	switch a.activeView {
	case ExplorerView:
		hint = a.explorer.CurrentHint()
	case TimelineView:
		hint = a.timeline.CurrentHint()
	case CompareView:
		hint = "j/k: scroll  tab: switch pane  ctrl+d/u: page  X: clear compare"
	}
	a.statusBar.SetHint(hint)
}

// handleSimulationTick processes a simulation tick by generating a new revision.
func (a *App) handleSimulationTick(resourceUID string) {
	if a.simulator == nil {
		return
	}
	rd := a.store.GetResource(resourceUID)
	if rd == nil {
		return
	}

	// Generate and add the new revision to the store (always — even when frozen)
	newRev := a.simulator.GenerateRevision(rd)
	a.store.AddRevision(resourceUID, newRev)
	a.debugLog.Debug("sim", "new rev %s for %s (%s)", newRev.ID, rd.Resource.KindName(), newRev.EventType)

	// If frozen, buffer the revision for later replay instead of updating views
	if a.frozen {
		a.pendingBuffer = append(a.pendingBuffer, pendingRevision{
			ResourceUID: resourceUID,
			Revision:    newRev,
		})
		// Update the status bar count even while frozen so user sees data is still arriving
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		return
	}

	// Not frozen — update all views normally
	a.refreshViewsAfterNewRevision(resourceUID)
}

// handleLiveRevision processes a live revision from the production watcher.
// Unlike handleSimulationTick, data is already in the store (added by the adapter handler).
// We just need to refresh the UI.
func (a *App) handleLiveRevision(resourceUID string) {
	a.debugLog.Debug("live", "new revision for %s", resourceUID)

	// If frozen, just buffer for later and update the count
	if a.frozen {
		a.pendingBuffer = append(a.pendingBuffer, pendingRevision{
			ResourceUID: resourceUID,
		})
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		return
	}

	// Not frozen — refresh all views
	a.refreshViewsAfterNewRevision(resourceUID)
}

// refreshViewsAfterNewRevision updates all view components after new data has been added to the store.
func (a *App) refreshViewsAfterNewRevision(resourceUID string) {
	// Rebuild kind groups in the store (unfiltered cache)
	a.store.RebuildKindGroups()

	// Refresh explorer with current filter applied
	a.refreshExplorerGroups()
	a.refreshTimeline()

	// Update counts
	a.statusBar.SetCounts(
		a.store.TotalResourceCount(),
		a.store.TotalRevisionCount(),
		len(a.store.StarredResources()),
	)

	// Auto-scroll: jump to newest if enabled
	if a.autoScroll {
		rd := a.store.GetResource(resourceUID)
		if rd != nil {
			// Explorer: update revList + detail if the selected resource got a new rev
			selectedUID := a.explorer.tree.SelectedUID()
			if selectedUID == resourceUID {
				a.explorer.revList.JumpToNewest()
				a.explorer.detail.SetRevision(rd, len(rd.Revisions)-1)
			}
		}

		// Timeline: jump to newest entry and update its detail view
		a.timeline.timeline.JumpToNewest()
		if entry := a.timeline.timeline.SelectedEntry(); entry != nil {
			a.timeline.SelectEntry(*entry)
		}
	}
}

// flushPendingBuffer replays all buffered revisions into the views when unfreezing.
// The store already contains all the data (it was written during frozen ticks),
// so we just need to refresh the views once and optionally auto-scroll.
func (a *App) flushPendingBuffer() {
	if len(a.pendingBuffer) == 0 {
		return
	}

	// Find the last resourceUID that was buffered (for auto-scroll targeting)
	lastResourceUID := a.pendingBuffer[len(a.pendingBuffer)-1].ResourceUID

	// Clear the buffer
	a.pendingBuffer = nil

	// Single bulk refresh of all views (store already has all the data)
	a.refreshViewsAfterNewRevision(lastResourceUID)
}

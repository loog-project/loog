package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App is the root tea.Model that wires everything together.
type App struct {
	width, height int
	theme         Theme
	store         *DummyStore

	// Chrome
	header    *Header
	filterBar *FilterBar
	statusBar *StatusBar

	// Views
	activeView ViewID
	explorer   *ExplorerViewComponent
	timeline   *TimelineViewComponent
	watchlist  *WatchlistViewComponent
	compare    *CompareViewComponent

	// Overlays
	commandPalette *CommandPalette
	helpOverlay    *HelpOverlay
	quickSearch    *QuickSearch
	watchManager   *WatchManager
	registry       *CommandRegistry

	// State
	filterExpr string
	fullscreen bool
	statusText string
	statusErr  bool
	statusTime time.Time
	ready      bool

	// Auto-scroll
	autoScroll bool

	// Window mode
	windowMode   WindowMode
	windowAnchor time.Time // timestamp the window is centered on

	// Simulation
	simulating bool

	// Freeze: data keeps arriving but the UI doesn't update.
	// Buffered revisions are flushed when unfrozen.
	frozen        bool
	pendingBuffer []pendingRevision // revisions that arrived while frozen

	// Analysis results cache
	analysisResults map[string]AnalysisResult // resourceUID -> result
}

// pendingRevision holds a revision that arrived during freeze.
type pendingRevision struct {
	ResourceUID string
	Revision    Revision
}

// NewApp creates the root application model.
func NewApp(store *DummyStore) *App {
	theme := CatppuccinMocha
	registry := NewCommandRegistry()

	app := &App{
		theme:           theme,
		store:           store,
		registry:        registry,
		analysisResults: make(map[string]AnalysisResult),

		// Chrome
		header:    NewHeader(theme),
		filterBar: NewFilterBar(theme),
		statusBar: NewStatusBar(theme),

		// Views
		activeView: ExplorerView,
		explorer:   NewExplorerViewComponent(theme),
		timeline:   NewTimelineViewComponent(theme),
		watchlist:  NewWatchlistViewComponent(theme),
		compare:    NewCompareViewComponent(theme),

		// Overlays
		commandPalette: NewCommandPalette(theme, registry),
		helpOverlay:    NewHelpOverlay(theme),
		quickSearch:    NewQuickSearch(theme),
		watchManager:   NewWatchManager(theme),

		// Simulation on by default for demo
		simulating: true,
	}

	// Initialize views with data
	app.explorer.SetGroups(store.KindGroups)
	app.explorer.SetFocusPanel(PanelLeft)

	app.timeline.SetEntries(store.Timeline)
	app.timeline.SetStore(store)
	app.timeline.SetFocusPanel(PanelLeft)

	app.watchlist.SetStore(store)
	app.watchlist.SetFocusPanel(PanelLeft)

	// Initialize status bar counts
	app.statusBar.SetCounts(
		store.TotalResourceCount(),
		store.TotalRevisionCount(),
		len(store.StarredResources()),
	)
	app.statusBar.SetSimulating(app.simulating)

	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.EnterAltScreen,
		tickCmd(),
	}
	// Start simulation if enabled
	if a.simulating {
		cmds = append(cmds, SimulateNewRevisionCmd(a.store))
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

		// Filter bar editing mode
		if a.filterBar.IsEditing() {
			// Quick search: typing / while filter expression is empty opens fuzzy resource finder
			if msg.String() == "/" && a.filterBar.Expression() == "" {
				a.filterBar.StopEditing()
				a.quickSearch.Show(a.store.AllResources())
				return a, nil
			}
			expr, done := a.filterBar.HandleKey(msg.String())
			if done {
				a.filterExpr = expr
				a.applyFilter()
			}
			return a, nil
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
			a.filterBar.StartEditing()
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewExplorer):
			a.switchView(ExplorerView)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewTimeline):
			a.switchView(TimelineView)
			return a, nil

		case key.Matches(msg, GlobalKeyMap.ViewWatchlist):
			a.switchView(WatchlistView)
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
			a.watchlist.SetAutoScroll(a.autoScroll)
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
		a.watchlist.SetResource(msg.Resource)
		if msg.Resource != nil {
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
		a.watchlist.SetRevision(msg.Resource, msg.Index)
		if msg.Index >= 0 && msg.Index < len(msg.Resource.Revisions) {
			rev := msg.Resource.Revisions[msg.Index]
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

	case FilterChangedMsg:
		a.filterExpr = msg.Expression
		a.filterBar.SetExpression(msg.Expression)
		a.applyFilter()

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

	case ShowCommandPaletteMsg:
		a.commandPalette.Show()

	case HideOverlayMsg:
		// overlays handle their own hiding

	case ShowHelpMsg:
		a.helpOverlay.Show()

	case ShowFilterMsg:
		a.filterBar.StartEditing()

	case ShowQuickSearchMsg:
		a.quickSearch.Show(a.store.AllResources())

	case ShowWatchManagerMsg:
		a.watchManager.Show(a.store, a.store.UnwatchedKinds())

	case AddWatchKindMsg:
		created := a.store.AddWatchKind(msg.Kind)
		a.store.KindGroups = BuildKindGroups(a.store.AllResources())
		a.explorer.SetGroups(a.store.KindGroups)
		a.timeline.SetEntries(a.store.Timeline)
		a.watchlist.RefreshStarred()
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
		if rd, ok := a.store.Resources[selectedUID]; ok && rd.Resource.Kind == msg.Kind {
			selectedIsOfKind = true
		}

		a.store.RemoveWatchKind(msg.Kind)
		a.store.KindGroups = BuildKindGroups(a.store.AllResources())
		a.explorer.SetGroups(a.store.KindGroups)
		a.timeline.SetEntries(a.store.Timeline)
		a.watchlist.RefreshStarred()
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		if selectedIsOfKind {
			a.explorer.SetResource(nil)
			a.watchlist.SetResource(nil)
		}
		a.setStatus(fmt.Sprintf("Unwatched: %s (%d resources removed)", msg.Kind, count), false)

	case StatusMsg:
		a.setStatus(msg.Text, msg.IsError)

	case ToggleFullscreenMsg:
		a.fullscreen = !a.fullscreen
		if !a.fullscreen {
			a.layout()
		}

	case ToggleAutoScrollMsg:
		a.autoScroll = !a.autoScroll
		a.explorer.SetAutoScroll(a.autoScroll)
		a.timeline.SetAutoScroll(a.autoScroll)
		a.watchlist.SetAutoScroll(a.autoScroll)
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
			// Schedule a new simulation tick to restart generation
			cmds = append(cmds, SimulateNewRevisionCmd(a.store))
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
		if a.simulating {
			cmds = append(cmds, SimulateNewRevisionCmd(a.store))
		}

	case NewRevisionMsg:
		// Nothing extra needed; store already updated in handleSimulationTick
	}

	return a, Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if !a.ready {
		return "Initializing..."
	}

	// Build main content
	headerView := a.header.View()
	filterView := a.filterBar.View()
	statusView := a.statusBar.View()

	// Status bar is always 2 lines for stable layout.
	// Content height = total - header(1) - filter(1) - status(2)
	contentHeight := a.height - 4
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
		case WatchlistView:
			contentView = a.watchlist.View()
		case CompareView:
			contentView = a.compare.View()
		}
	}

	// Stack: header + filter + content + status
	mainView := lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		filterView,
		contentView,
		statusView,
	)

	// Overlay rendering
	if a.commandPalette.IsVisible() {
		paletteView := a.commandPalette.View()
		mainView = PlaceOverlay(paletteView, mainView, true)
	}
	if a.helpOverlay.IsVisible() {
		helpView := a.helpOverlay.View()
		mainView = PlaceOverlay(helpView, mainView, true)
	}
	if a.quickSearch.IsVisible() {
		qsView := a.quickSearch.View()
		mainView = PlaceOverlay(qsView, mainView, true)
	}
	if a.watchManager.IsVisible() {
		wmView := a.watchManager.View()
		mainView = PlaceOverlay(wmView, mainView, true)
	}

	return mainView
}

// --- Internal Methods ---

func (a *App) layout() {
	a.header.SetSize(a.width)
	a.filterBar.SetSize(a.width)
	a.statusBar.SetSize(a.width)

	// Status bar is always 2 lines (status + hint) for stable layout.
	// Content height = total - header(1) - filter(1) - status(2)
	contentHeight := a.height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	a.explorer.SetSize(a.width, contentHeight)
	a.timeline.SetSize(a.width, contentHeight)
	a.watchlist.SetSize(a.width, contentHeight)
	a.compare.SetSize(a.width, contentHeight)

	a.commandPalette.SetSize(a.width, a.height)
	a.helpOverlay.SetSize(a.width, a.height)
	a.quickSearch.SetSize(a.width, a.height)
	a.watchManager.SetSize(a.width, a.height)
}

func (a *App) switchView(v ViewID) {
	a.activeView = v
	a.header.SetView(v)
}

func (a *App) nextPanel() {
	switch a.activeView {
	case ExplorerView:
		a.explorer.NextPanel()
	case TimelineView:
		a.timeline.NextPanel()
	case WatchlistView:
		a.watchlist.NextPanel()
	}
}

func (a *App) prevPanel() {
	switch a.activeView {
	case ExplorerView:
		a.explorer.PrevPanel()
	case TimelineView:
		a.timeline.PrevPanel()
	case WatchlistView:
		a.watchlist.PrevPanel()
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
	case WatchlistView:
		a.watchlist.SetFocusPanel(p)
	}
}

func (a *App) updateActiveView(msg tea.Msg) tea.Cmd {
	switch a.activeView {
	case ExplorerView:
		return a.explorer.Update(msg)
	case TimelineView:
		return a.timeline.Update(msg)
	case WatchlistView:
		return a.watchlist.Update(msg)
	case CompareView:
		return a.compare.Update(msg)
	}
	return nil
}

func (a *App) toggleStar(uid string) {
	if rd, ok := a.store.Resources[uid]; ok {
		rd.Resource.Starred = !rd.Resource.Starred
		// Refresh kind groups (to update star indicators)
		a.store.KindGroups = BuildKindGroups(a.store.AllResources())
		a.explorer.SetGroups(a.store.KindGroups)
		a.watchlist.RefreshStarred()
		a.statusBar.SetCounts(
			a.store.TotalResourceCount(),
			a.store.TotalRevisionCount(),
			len(a.store.StarredResources()),
		)
		if rd.Resource.Starred {
			a.setStatus("Starred: "+rd.Resource.KindName(), false)
		} else {
			a.setStatus("Unstarred: "+rd.Resource.KindName(), false)
		}
	}
}

func (a *App) applyFilter() {
	filtered := a.store.FilterResources(a.filterExpr)
	groups := BuildKindGroups(filtered)
	a.explorer.SetGroups(groups)
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
	case WatchlistView:
		return a.watchlist.ViewFullscreen(a.width, contentHeight)
	case CompareView:
		return a.compare.ViewFullscreen(a.width, contentHeight)
	}
	return ""
}

// syncCompareMarks propagates compare selection to all views.
func (a *App) syncCompareMarks() {
	sel := a.compare.Selection()
	a.explorer.SetCompareMarks(sel.Left, sel.Right)
	a.watchlist.SetCompareMarks(sel.Left, sel.Right)
}

// syncAnalysisTags propagates analysis results to view components.
func (a *App) syncAnalysisTags() {
	// Build per-resource tag summary (just the latest revision tags for tree display)
	resourceTags := make(map[string][]ChangeTag)
	for uid, result := range a.analysisResults {
		if rd, ok := a.store.Resources[uid]; ok && len(rd.Revisions) > 0 {
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
	case WatchlistView:
		hint = a.watchlist.CurrentHint()
	case CompareView:
		hint = "Tab: switch side  j/k: scroll"
	}
	a.statusBar.SetHint(hint)
}

// handleSimulationTick processes a simulation tick by generating a new revision.
func (a *App) handleSimulationTick(resourceUID string) {
	rd, ok := a.store.Resources[resourceUID]
	if !ok {
		return
	}

	// Generate and add the new revision to the store (always — even when frozen)
	newRev := GenerateSimulatedRevision(rd)
	rd.Revisions = append(rd.Revisions, newRev)

	// Rebuild timeline in the store (always — keeps store consistent)
	a.store.Timeline = append([]TimelineEntry{{
		Resource: rd.Resource,
		Revision: newRev,
	}}, a.store.Timeline...)
	sort.Slice(a.store.Timeline, func(i, j int) bool {
		return a.store.Timeline[i].Revision.Time.After(a.store.Timeline[j].Revision.Time)
	})

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

// refreshViewsAfterNewRevision updates all view components after new data has been added to the store.
func (a *App) refreshViewsAfterNewRevision(resourceUID string) {
	// Rebuild kind groups and refresh all views
	a.store.KindGroups = BuildKindGroups(a.store.AllResources())
	a.explorer.SetGroups(a.store.KindGroups)
	a.timeline.SetEntries(a.store.Timeline)
	a.watchlist.RefreshStarred()

	// Update counts
	a.statusBar.SetCounts(
		a.store.TotalResourceCount(),
		a.store.TotalRevisionCount(),
		len(a.store.StarredResources()),
	)

	// Auto-scroll: jump to newest if enabled
	if a.autoScroll {
		rd, ok := a.store.Resources[resourceUID]
		if ok {
			// Explorer: update revList + detail if the selected resource got a new rev
			selectedUID := a.explorer.tree.SelectedUID()
			if selectedUID == resourceUID {
				a.explorer.revList.JumpToNewest()
				a.explorer.detail.SetRevision(rd, len(rd.Revisions)-1)
			}

			// Watchlist: same logic for its own tree
			watchSelectedUID := a.watchlist.tree.SelectedUID()
			if watchSelectedUID == resourceUID {
				a.watchlist.revList.JumpToNewest()
				a.watchlist.detail.SetRevision(rd, len(rd.Revisions)-1)
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

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Explorer View ───
// Three-column layout: ResourceTree | RevisionList | DetailView

type ExplorerViewComponent struct {
	width, height int
	theme         Theme
	focusPanel    PanelID

	tree     *ResourceTree
	revList  *RevisionList
	detail   *DetailView
	resource *ResourceData

	// Cached outer panel widths for View()
	treeOuterW, revOuterW, detailOuterW int
}

func NewExplorerViewComponent(theme Theme) *ExplorerViewComponent {
	return &ExplorerViewComponent{
		theme:      theme,
		focusPanel: PanelLeft,
		tree:       NewResourceTree(theme),
		revList:    NewRevisionList(theme),
		detail:     NewDetailView(theme),
	}
}

func (ev *ExplorerViewComponent) SetSize(w, h int) {
	ev.width = w
	ev.height = h

	// Outer panel widths (including borders)
	ev.treeOuterW = w * 25 / 100
	if ev.treeOuterW < 22 {
		ev.treeOuterW = 22
	}
	ev.revOuterW = w * 20 / 100
	if ev.revOuterW < 20 {
		ev.revOuterW = 20
	}
	ev.detailOuterW = w - ev.treeOuterW - ev.revOuterW - 2 // 2 separators
	if ev.detailOuterW < 12 {
		ev.detailOuterW = 12
	}

	// Components get INNER dimensions (outer minus border chrome: 2 width, 2 height)
	ev.tree.SetSize(ev.treeOuterW-2, h-2)
	ev.revList.SetSize(ev.revOuterW-2, h-2)
	ev.detail.SetSize(ev.detailOuterW-2, h-2)
}

func (ev *ExplorerViewComponent) SetGroups(groups []*KindGroup) {
	ev.tree.SetGroups(groups)
}

func (ev *ExplorerViewComponent) SetFocusPanel(p PanelID) {
	ev.focusPanel = p
	ev.tree.SetFocus(p == PanelLeft)
	ev.revList.SetFocus(p == PanelMiddle)
	ev.detail.SetFocus(p == PanelRight)
}

func (ev *ExplorerViewComponent) FocusPanel() PanelID {
	return ev.focusPanel
}

func (ev *ExplorerViewComponent) NextPanel() {
	switch ev.focusPanel {
	case PanelLeft:
		ev.SetFocusPanel(PanelMiddle)
	case PanelMiddle:
		ev.SetFocusPanel(PanelRight)
	case PanelRight:
		ev.SetFocusPanel(PanelLeft)
	}
}

func (ev *ExplorerViewComponent) PrevPanel() {
	switch ev.focusPanel {
	case PanelLeft:
		ev.SetFocusPanel(PanelRight)
	case PanelMiddle:
		ev.SetFocusPanel(PanelLeft)
	case PanelRight:
		ev.SetFocusPanel(PanelMiddle)
	}
}

func (ev *ExplorerViewComponent) SetResource(rd *ResourceData) {
	ev.resource = rd
	ev.revList.SetResource(rd)
	if rd != nil {
		ev.tree.SelectByUID(rd.Resource.UID)
		if len(rd.Revisions) > 0 {
			ev.detail.SetRevision(rd, len(rd.Revisions)-1)
		}
	}
}

func (ev *ExplorerViewComponent) SetRevision(rd *ResourceData, index int) {
	ev.detail.SetRevision(rd, index)
}

func (ev *ExplorerViewComponent) SetCompareMarks(left, right *CompareItem) {
	ev.tree.SetCompareMarks(left, right)
	ev.revList.SetCompareMarks(left, right)
}

func (ev *ExplorerViewComponent) SetAnalysisTags(resourceTags map[string][]ChangeTag, revisionTags map[RevisionID][]ChangeTag) {
	ev.tree.SetAnalysisTags(resourceTags)
	ev.revList.SetAnalysisTags(revisionTags)
}

func (ev *ExplorerViewComponent) SetAutoScroll(on bool) {
	ev.revList.SetAutoScroll(on)
}

// CurrentHint returns the focused component's hint.
func (ev *ExplorerViewComponent) CurrentHint() string {
	switch ev.focusPanel {
	case PanelLeft:
		return ev.tree.CurrentHint()
	case PanelMiddle:
		return ev.revList.CurrentHint()
	case PanelRight:
		return ev.detail.CurrentHint()
	}
	return ""
}

func (ev *ExplorerViewComponent) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch ev.focusPanel {
	case PanelLeft:
		if cmd := ev.tree.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case PanelMiddle:
		if cmd := ev.revList.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case PanelRight:
		if cmd := ev.detail.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return Batch(cmds...)
}

func (ev *ExplorerViewComponent) View() string {
	tc, tt := ev.tree.CursorInfo()
	treeTitle := "Resources"
	if tt > 0 {
		treeTitle += " " + ScrollPosition(tc, tt)
	}
	rc, rt := ev.revList.CursorInfo()
	revTitle := "Revisions"
	if rt > 0 {
		revTitle += " " + ScrollPosition(rc, rt)
	}
	modeLabel := "Detail [" + ev.detail.ViewMode().String() + "]"

	treeContent := PanelBorder(ev.tree.View(), treeTitle, ev.treeOuterW, ev.height, ev.focusPanel == PanelLeft, ev.theme)
	revContent := PanelBorder(ev.revList.View(), revTitle, ev.revOuterW, ev.height, ev.focusPanel == PanelMiddle, ev.theme)
	detailContent := PanelBorder(ev.detail.View(), modeLabel, ev.detailOuterW, ev.height, ev.focusPanel == PanelRight, ev.theme)

	return SplitThreeColumns(treeContent, revContent, detailContent, ev.treeOuterW, ev.revOuterW, ev.width, ev.height)
}

// ViewFullscreen renders only the focused panel at full width.
func (ev *ExplorerViewComponent) ViewFullscreen(w, h int) string {
	switch ev.focusPanel {
	case PanelLeft:
		ev.tree.SetSize(w-2, h-2)
		return PanelBorder(ev.tree.View(), "Resources [fullscreen]", w, h, true, ev.theme)
	case PanelMiddle:
		ev.revList.SetSize(w-2, h-2)
		return PanelBorder(ev.revList.View(), "Revisions [fullscreen]", w, h, true, ev.theme)
	case PanelRight:
		ev.detail.SetSize(w-2, h-2)
		modeLabel := "Detail [" + ev.detail.ViewMode().String() + "] [fullscreen]"
		return PanelBorder(ev.detail.View(), modeLabel, w, h, true, ev.theme)
	}
	return ""
}

// ─── Timeline View ───
// Two-column layout: TimelineList | DetailView

type TimelineViewComponent struct {
	width, height int
	theme         Theme
	focusPanel    PanelID

	timeline *TimelineList
	detail   *DetailView
	store    *DummyStore

	listOuterW, detailOuterW int
}

func NewTimelineViewComponent(theme Theme) *TimelineViewComponent {
	return &TimelineViewComponent{
		theme:      theme,
		focusPanel: PanelLeft,
		timeline:   NewTimelineList(theme),
		detail:     NewDetailView(theme),
	}
}

func (tv *TimelineViewComponent) SetSize(w, h int) {
	tv.width = w
	tv.height = h

	tv.listOuterW = w * 40 / 100
	if tv.listOuterW < 32 {
		tv.listOuterW = 32
	}
	tv.detailOuterW = w - tv.listOuterW - 1 // 1 separator
	if tv.detailOuterW < 12 {
		tv.detailOuterW = 12
	}

	// Inner dimensions for components
	tv.timeline.SetSize(tv.listOuterW-2, h-2)
	tv.detail.SetSize(tv.detailOuterW-2, h-2)
}

func (tv *TimelineViewComponent) SetEntries(entries []TimelineEntry) {
	tv.timeline.SetEntries(entries)
}

func (tv *TimelineViewComponent) SetStore(store *DummyStore) {
	tv.store = store
}

func (tv *TimelineViewComponent) SetFocusPanel(p PanelID) {
	tv.focusPanel = p
	tv.timeline.SetFocus(p == PanelLeft)
	tv.detail.SetFocus(p == PanelRight)
}

func (tv *TimelineViewComponent) FocusPanel() PanelID {
	return tv.focusPanel
}

func (tv *TimelineViewComponent) NextPanel() {
	if tv.focusPanel == PanelLeft {
		tv.SetFocusPanel(PanelRight)
	} else {
		tv.SetFocusPanel(PanelLeft)
	}
}

func (tv *TimelineViewComponent) PrevPanel() {
	tv.NextPanel()
}

func (tv *TimelineViewComponent) SelectEntry(entry TimelineEntry) {
	if tv.store != nil {
		if rd, ok := tv.store.Resources[entry.Resource.UID]; ok {
			for i, rev := range rd.Revisions {
				if rev.ID == entry.Revision.ID {
					tv.detail.SetRevision(rd, i)
					return
				}
			}
		}
	}
}

func (tv *TimelineViewComponent) SetAutoScroll(on bool) {
	tv.timeline.SetAutoScroll(on)
}

func (tv *TimelineViewComponent) SetWindowMode(wm WindowMode) {
	tv.timeline.SetWindowMode(wm)
}

func (tv *TimelineViewComponent) SetWindowAnchor(t time.Time) {
	tv.timeline.SetWindowAnchor(t)
}

// CurrentHint returns the focused component's hint.
func (tv *TimelineViewComponent) CurrentHint() string {
	if tv.focusPanel == PanelLeft {
		return tv.timeline.CurrentHint()
	}
	return tv.detail.CurrentHint()
}

func (tv *TimelineViewComponent) Update(msg tea.Msg) tea.Cmd {
	switch tv.focusPanel {
	case PanelLeft:
		return tv.timeline.Update(msg)
	default:
		return tv.detail.Update(msg)
	}
}

func (tv *TimelineViewComponent) View() string {
	tc, tt := tv.timeline.CursorInfo()
	listTitle := "Timeline"
	if tt > 0 {
		listTitle += " " + ScrollPosition(tc, tt)
	}
	modeLabel := "Detail [" + tv.detail.ViewMode().String() + "]"

	listContent := PanelBorder(tv.timeline.View(), listTitle, tv.listOuterW, tv.height, tv.focusPanel == PanelLeft, tv.theme)
	detailContent := PanelBorder(tv.detail.View(), modeLabel, tv.detailOuterW, tv.height, tv.focusPanel == PanelRight, tv.theme)

	return SplitHorizontal(listContent, detailContent, tv.listOuterW, tv.width, tv.height)
}

// ViewFullscreen renders only the focused panel at full width.
func (tv *TimelineViewComponent) ViewFullscreen(w, h int) string {
	switch tv.focusPanel {
	case PanelLeft:
		tv.timeline.SetSize(w-2, h-2)
		return PanelBorder(tv.timeline.View(), "Timeline [fullscreen]", w, h, true, tv.theme)
	default:
		tv.detail.SetSize(w-2, h-2)
		modeLabel := "Detail [" + tv.detail.ViewMode().String() + "] [fullscreen]"
		return PanelBorder(tv.detail.View(), modeLabel, w, h, true, tv.theme)
	}
}

// ─── Watchlist View ───
// Same as Explorer but filtered to starred resources only.

type WatchlistViewComponent struct {
	width, height int
	theme         Theme
	focusPanel    PanelID

	tree    *ResourceTree
	revList *RevisionList
	detail  *DetailView
	store   *DummyStore

	treeOuterW, revOuterW, detailOuterW int
}

func NewWatchlistViewComponent(theme Theme) *WatchlistViewComponent {
	return &WatchlistViewComponent{
		theme:      theme,
		focusPanel: PanelLeft,
		tree:       NewResourceTree(theme),
		revList:    NewRevisionList(theme),
		detail:     NewDetailView(theme),
	}
}

func (wv *WatchlistViewComponent) SetSize(w, h int) {
	wv.width = w
	wv.height = h

	wv.treeOuterW = w * 30 / 100
	if wv.treeOuterW < 22 {
		wv.treeOuterW = 22
	}
	wv.revOuterW = w * 20 / 100
	if wv.revOuterW < 20 {
		wv.revOuterW = 20
	}
	wv.detailOuterW = w - wv.treeOuterW - wv.revOuterW - 2
	if wv.detailOuterW < 12 {
		wv.detailOuterW = 12
	}

	wv.tree.SetSize(wv.treeOuterW-2, h-2)
	wv.revList.SetSize(wv.revOuterW-2, h-2)
	wv.detail.SetSize(wv.detailOuterW-2, h-2)
}

func (wv *WatchlistViewComponent) SetStore(store *DummyStore) {
	wv.store = store
	wv.RefreshStarred()
}

func (wv *WatchlistViewComponent) RefreshStarred() {
	if wv.store == nil {
		return
	}
	starred := wv.store.StarredResources()
	groups := BuildKindGroups(starred)
	wv.tree.SetGroups(groups)
}

func (wv *WatchlistViewComponent) SetFocusPanel(p PanelID) {
	wv.focusPanel = p
	wv.tree.SetFocus(p == PanelLeft)
	wv.revList.SetFocus(p == PanelMiddle)
	wv.detail.SetFocus(p == PanelRight)
}

func (wv *WatchlistViewComponent) FocusPanel() PanelID { return wv.focusPanel }

func (wv *WatchlistViewComponent) NextPanel() {
	switch wv.focusPanel {
	case PanelLeft:
		wv.SetFocusPanel(PanelMiddle)
	case PanelMiddle:
		wv.SetFocusPanel(PanelRight)
	case PanelRight:
		wv.SetFocusPanel(PanelLeft)
	}
}

func (wv *WatchlistViewComponent) PrevPanel() {
	switch wv.focusPanel {
	case PanelLeft:
		wv.SetFocusPanel(PanelRight)
	case PanelMiddle:
		wv.SetFocusPanel(PanelLeft)
	case PanelRight:
		wv.SetFocusPanel(PanelMiddle)
	}
}

func (wv *WatchlistViewComponent) SetResource(rd *ResourceData) {
	wv.revList.SetResource(rd)
	if rd != nil {
		wv.tree.SelectByUID(rd.Resource.UID)
		if len(rd.Revisions) > 0 {
			wv.detail.SetRevision(rd, len(rd.Revisions)-1)
		}
	}
}

func (wv *WatchlistViewComponent) SetRevision(rd *ResourceData, index int) {
	wv.detail.SetRevision(rd, index)
}

func (wv *WatchlistViewComponent) SetCompareMarks(left, right *CompareItem) {
	wv.tree.SetCompareMarks(left, right)
	wv.revList.SetCompareMarks(left, right)
}

func (wv *WatchlistViewComponent) SetAutoScroll(on bool) {
	wv.revList.SetAutoScroll(on)
}

// CurrentHint returns the focused component's hint.
func (wv *WatchlistViewComponent) CurrentHint() string {
	switch wv.focusPanel {
	case PanelLeft:
		return wv.tree.CurrentHint()
	case PanelMiddle:
		return wv.revList.CurrentHint()
	case PanelRight:
		return wv.detail.CurrentHint()
	}
	return ""
}

func (wv *WatchlistViewComponent) Update(msg tea.Msg) tea.Cmd {
	switch wv.focusPanel {
	case PanelLeft:
		return wv.tree.Update(msg)
	case PanelMiddle:
		return wv.revList.Update(msg)
	default:
		return wv.detail.Update(msg)
	}
}

func (wv *WatchlistViewComponent) View() string {
	tc, tt := wv.tree.CursorInfo()
	treeTitle := "Watchlist"
	if tt > 0 {
		treeTitle += " " + ScrollPosition(tc, tt)
	}
	rc, rt := wv.revList.CursorInfo()
	revTitle := "Revisions"
	if rt > 0 {
		revTitle += " " + ScrollPosition(rc, rt)
	}
	modeLabel := "Detail [" + wv.detail.ViewMode().String() + "]"

	treeContent := PanelBorder(wv.tree.View(), treeTitle, wv.treeOuterW, wv.height, wv.focusPanel == PanelLeft, wv.theme)
	revContent := PanelBorder(wv.revList.View(), revTitle, wv.revOuterW, wv.height, wv.focusPanel == PanelMiddle, wv.theme)
	detailContent := PanelBorder(wv.detail.View(), modeLabel, wv.detailOuterW, wv.height, wv.focusPanel == PanelRight, wv.theme)

	return SplitThreeColumns(treeContent, revContent, detailContent, wv.treeOuterW, wv.revOuterW, wv.width, wv.height)
}

// ViewFullscreen renders only the focused panel at full width.
func (wv *WatchlistViewComponent) ViewFullscreen(w, h int) string {
	switch wv.focusPanel {
	case PanelLeft:
		wv.tree.SetSize(w-2, h-2)
		return PanelBorder(wv.tree.View(), "Watchlist [fullscreen]", w, h, true, wv.theme)
	case PanelMiddle:
		wv.revList.SetSize(w-2, h-2)
		return PanelBorder(wv.revList.View(), "Revisions [fullscreen]", w, h, true, wv.theme)
	case PanelRight:
		wv.detail.SetSize(w-2, h-2)
		modeLabel := "Detail [" + wv.detail.ViewMode().String() + "] [fullscreen]"
		return PanelBorder(wv.detail.View(), modeLabel, w, h, true, wv.theme)
	}
	return ""
}

// ─── Compare View ───
// Full-width side-by-side comparison panel.

type CompareViewComponent struct {
	width, height int
	theme         Theme
	panel         *ComparePanel
	selection     CompareSelection
}

func NewCompareViewComponent(theme Theme) *CompareViewComponent {
	return &CompareViewComponent{
		theme: theme,
		panel: NewComparePanel(theme),
	}
}

func (cv *CompareViewComponent) SetSize(w, h int) {
	cv.width = w
	cv.height = h
	// Compare panel gets inner dimensions
	cv.panel.SetSize(w-2, h-2)
}

func (cv *CompareViewComponent) SetSelection(sel CompareSelection) {
	cv.selection = sel
	cv.panel.SetItems(sel.Left, sel.Right)
}

func (cv *CompareViewComponent) AddItem(item CompareItem) CompareSelection {
	if cv.selection.Left == nil {
		cv.selection.Left = &item
	} else if cv.selection.Right == nil {
		cv.selection.Right = &item
	} else {
		cv.selection.Left = cv.selection.Right
		cv.selection.Right = &item
	}
	cv.panel.SetItems(cv.selection.Left, cv.selection.Right)
	return cv.selection
}

func (cv *CompareViewComponent) Selection() CompareSelection {
	return cv.selection
}

func (cv *CompareViewComponent) Clear() {
	cv.selection = CompareSelection{}
	cv.panel.SetItems(nil, nil)
}

func (cv *CompareViewComponent) Update(msg tea.Msg) tea.Cmd {
	return cv.panel.Update(msg)
}

func (cv *CompareViewComponent) View() string {
	return PanelBorder(cv.panel.View(), "Compare", cv.width, cv.height, true, cv.theme)
}

// ViewFullscreen is the same as View for compare (it's always full width).
func (cv *CompareViewComponent) ViewFullscreen(w, h int) string {
	cv.panel.SetSize(w-2, h-2)
	return PanelBorder(cv.panel.View(), "Compare [fullscreen]", w, h, true, cv.theme)
}

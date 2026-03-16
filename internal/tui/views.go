package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
)

type ExplorerViewComponent struct {
	width, height int
	theme         Theme
	focusPanel    PanelID

	tree     *ResourceTree
	revList  *RevisionList
	detail   *DetailView
	resource *resource.Data

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
	ev.treeOuterW = max(w*25/100, 22)
	ev.revOuterW = max(w*20/100, 20)
	ev.detailOuterW = max(
		// 2 separators
		w-ev.treeOuterW-ev.revOuterW-2, 12)

	// Components get INNER dimensions (outer minus border chrome: 2 width, 2 height)
	ev.tree.SetSize(ev.treeOuterW-2, h-2)
	ev.revList.SetSize(ev.revOuterW-2, h-2)
	ev.detail.SetSize(ev.detailOuterW-2, h-2)
}

func (ev *ExplorerViewComponent) SetGroups(groups []*resource.KindGroup) {
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

func (ev *ExplorerViewComponent) SetResource(rd *resource.Data) {
	ev.resource = rd
	ev.revList.SetResource(rd)
	if rd != nil {
		ev.tree.SelectByUID(rd.Resource.UID)
		if len(rd.Revisions) > 0 {
			ev.detail.SetRevision(rd, len(rd.Revisions)-1)
		}
	}
}

func (ev *ExplorerViewComponent) SetRevision(rd *resource.Data, index int) {
	ev.revList.SelectIndex(index)
	ev.detail.SetRevision(rd, index)
}

func (ev *ExplorerViewComponent) SetCompareMarks(left, right *resource.CompareItem) {
	ev.tree.SetCompareMarks(left, right)
	ev.revList.SetCompareMarks(left, right)
}

func (ev *ExplorerViewComponent) SetAnalysisTags(
	resourceTags map[string][]resource.ChangeTag,
	revisionTags map[resource.RevisionID][]resource.ChangeTag,
) {
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
		if ev.tree.filterEditing {
			return strings.Join([]string{
				ev.tree.theme.KeyHint("type", "filter"),
				ev.tree.theme.KeyHint("Enter", "apply"),
				ev.tree.theme.KeyHint("Esc", "clear"),
				ev.tree.theme.KeyHint("//", "quick jump"),
			}, "  ")
		}
		return ev.tree.CurrentHint()
	case PanelMiddle:
		return ev.revList.CurrentHint()
	case PanelRight:
		return ev.detail.CurrentHint()
	}
	return ""
}

// StartFilter activates inline filter on the focused panel (only ResourceTree supports it).
func (ev *ExplorerViewComponent) StartFilter() (bool, tea.Cmd) {
	if ev.focusPanel == PanelLeft {
		cmd := ev.tree.StartFilter()
		return true, cmd
	}
	return false, nil
}

// IsFilterEditing returns true if the focused panel is currently editing a filter.
func (ev *ExplorerViewComponent) IsFilterEditing() bool {
	return ev.focusPanel == PanelLeft && ev.tree.filterEditing
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

	return tea.Batch(cmds...)
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

	treeContent := PanelBorderEx(ev.tree.View(), treeTitle, ev.treeOuterW, ev.height, ev.focusPanel == PanelLeft, ev.theme, ev.tree.CanScrollUp(), ev.tree.CanScrollDown())
	revContent := PanelBorderEx(ev.revList.View(), revTitle, ev.revOuterW, ev.height, ev.focusPanel == PanelMiddle, ev.theme, ev.revList.CanScrollUp(), ev.revList.CanScrollDown())
	detailContent := PanelBorderEx(ev.detail.View(), modeLabel, ev.detailOuterW, ev.height, ev.focusPanel == PanelRight, ev.theme, ev.detail.CanScrollUp(), ev.detail.CanScrollDown())

	return SplitThreeColumns(treeContent, revContent, detailContent, ev.height)
}

// ViewFullscreen renders only the focused panel at full width.
func (ev *ExplorerViewComponent) ViewFullscreen(w, h int) string {
	switch ev.focusPanel {
	case PanelLeft:
		ev.tree.SetSize(w-2, h-2)
		return PanelBorderEx(ev.tree.View(), "Resources [fullscreen]", w, h, true, ev.theme, ev.tree.CanScrollUp(), ev.tree.CanScrollDown())
	case PanelMiddle:
		ev.revList.SetSize(w-2, h-2)
		return PanelBorderEx(ev.revList.View(), "Revisions [fullscreen]", w, h, true, ev.theme, ev.revList.CanScrollUp(), ev.revList.CanScrollDown())
	case PanelRight:
		ev.detail.SetSize(w-2, h-2)
		modeLabel := "Detail [" + ev.detail.ViewMode().String() + "] [fullscreen]"
		return PanelBorderEx(ev.detail.View(), modeLabel, w, h, true, ev.theme, ev.detail.CanScrollUp(), ev.detail.CanScrollDown())
	}
	return ""
}

type TimelineViewComponent struct {
	width, height int
	theme         Theme
	focusPanel    PanelID

	timeline *TimelineList
	detail   *DetailView
	store    Store

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

	tv.listOuterW = max(w*40/100, 32)
	tv.detailOuterW = max(
		// 1 separator
		w-tv.listOuterW-1, 12)

	// Inner dimensions for components
	tv.timeline.SetSize(tv.listOuterW-2, h-2)
	tv.detail.SetSize(tv.detailOuterW-2, h-2)
}

func (tv *TimelineViewComponent) SetEntries(entries []resource.TimelineEntry) {
	tv.timeline.SetEntries(entries)
}

// ScrollToEntry finds and selects a specific timeline entry by matching revision ID.
func (tv *TimelineViewComponent) ScrollToEntry(entry resource.TimelineEntry) {
	tv.timeline.ScrollToRevision(entry.Revision.ID)
	// Also update the detail view
	if sel := tv.timeline.SelectedEntry(); sel != nil {
		if rd := tv.store.GetResource(sel.Resource.UID); rd != nil {
			for i, rev := range rd.Revisions {
				if rev.ID == sel.Revision.ID {
					tv.detail.SetRevision(rd, i)
					break
				}
			}
		}
	}
}

func (tv *TimelineViewComponent) SetStore(store Store) {
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

func (tv *TimelineViewComponent) SelectEntry(entry resource.TimelineEntry) {
	if tv.store != nil {
		if rd := tv.store.GetResource(entry.Resource.UID); rd != nil {
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

func (tv *TimelineViewComponent) SetWindowMode(wm resource.WindowMode) {
	tv.timeline.SetWindowMode(wm)
}

func (tv *TimelineViewComponent) SetWindowAnchor(t time.Time) {
	tv.timeline.SetWindowAnchor(t)
}

func (tv *TimelineViewComponent) SetCompareMarks(left, right *resource.CompareItem) {
	tv.timeline.SetCompareMarks(left, right)
}

// CurrentHint returns the focused component's hint.
func (tv *TimelineViewComponent) CurrentHint() string {
	if tv.focusPanel == PanelLeft {
		if tv.timeline.filterEditing {
			return strings.Join([]string{
				tv.timeline.theme.KeyHint("type", "filter"),
				tv.timeline.theme.KeyHint("Enter", "apply"),
				tv.timeline.theme.KeyHint("Esc", "clear"),
				tv.timeline.theme.KeyHint("//", "quick jump"),
			}, "  ")
		}
		return tv.timeline.CurrentHint()
	}
	return tv.detail.CurrentHint()
}

// StartFilter activates inline filter on the focused panel (only TimelineList supports it).
func (tv *TimelineViewComponent) StartFilter() (bool, tea.Cmd) {
	if tv.focusPanel == PanelLeft {
		cmd := tv.timeline.StartFilter()
		return true, cmd
	}
	return false, nil
}

// IsFilterEditing returns true if the focused panel is currently editing a filter.
func (tv *TimelineViewComponent) IsFilterEditing() bool {
	return tv.focusPanel == PanelLeft && tv.timeline.filterEditing
}

func (tv *TimelineViewComponent) Update(msg tea.Msg) tea.Cmd {
	switch tv.focusPanel {
	case PanelLeft:
		// Intercept "c" for compare marking -- but only when NOT in filter editing mode.
		// TimelineList doesn't have store access, so we resolve the
		// TimelineEntry -> ResourceData + revision index here.
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "c" && !tv.timeline.filterEditing {
			if entry := tv.timeline.SelectedEntry(); entry != nil && tv.store != nil {
				if rd := tv.store.GetResource(entry.Resource.UID); rd != nil {
					for i, rev := range rd.Revisions {
						if rev.ID == entry.Revision.ID {
							return Cmd(CompareMarkMsg{Resource: rd, Index: i})
						}
					}
				}
			}
			return nil
		}
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

	listContent := PanelBorderEx(tv.timeline.View(), listTitle, tv.listOuterW, tv.height, tv.focusPanel == PanelLeft, tv.theme, tv.timeline.CanScrollUp(), tv.timeline.CanScrollDown())
	detailContent := PanelBorderEx(tv.detail.View(), modeLabel, tv.detailOuterW, tv.height, tv.focusPanel == PanelRight, tv.theme, tv.detail.CanScrollUp(), tv.detail.CanScrollDown())

	return SplitHorizontal(listContent, detailContent, tv.height)
}

// ViewFullscreen renders only the focused panel at full width.
func (tv *TimelineViewComponent) ViewFullscreen(w, h int) string {
	switch tv.focusPanel {
	case PanelLeft:
		tv.timeline.SetSize(w-2, h-2)
		return PanelBorderEx(tv.timeline.View(), "Timeline [fullscreen]", w, h, true, tv.theme, tv.timeline.CanScrollUp(), tv.timeline.CanScrollDown())
	default:
		tv.detail.SetSize(w-2, h-2)
		modeLabel := "Detail [" + tv.detail.ViewMode().String() + "] [fullscreen]"
		return PanelBorderEx(tv.detail.View(), modeLabel, w, h, true, tv.theme, tv.detail.CanScrollUp(), tv.detail.CanScrollDown())
	}
}

type CompareViewComponent struct {
	width, height int
	theme         Theme
	panel         *ComparePanel
	selection     resource.CompareSelection
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

func (cv *CompareViewComponent) SetSelection(sel resource.CompareSelection) {
	cv.selection = sel
	cv.panel.SetItems(sel.Left, sel.Right)
}

func (cv *CompareViewComponent) AddItem(item resource.CompareItem) resource.CompareSelection {
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

func (cv *CompareViewComponent) Selection() resource.CompareSelection {
	return cv.selection
}

func (cv *CompareViewComponent) Clear() {
	cv.selection = resource.CompareSelection{}
	cv.panel.SetItems(nil, nil)
}

func (cv *CompareViewComponent) Update(msg tea.Msg) tea.Cmd {
	return cv.panel.Update(msg)
}

func (cv *CompareViewComponent) View() string {
	return PanelBorderEx(cv.panel.View(), "Compare", cv.width, cv.height, true, cv.theme, cv.panel.CanScrollUp(), cv.panel.CanScrollDown())
}

// ViewFullscreen is the same as View for compare (it's always full width).
func (cv *CompareViewComponent) ViewFullscreen(w, h int) string {
	cv.panel.SetSize(w-2, h-2)
	return PanelBorderEx(cv.panel.View(), "Compare [fullscreen]", w, h, true, cv.theme, cv.panel.CanScrollUp(), cv.panel.CanScrollDown())
}

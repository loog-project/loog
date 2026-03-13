package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/loog-project/loog/pkg/diffpreview"
)

// ─── Resource Tree ───

// ResourceTree is the left panel showing collapsible Kind > Resource groups.
type ResourceTree struct {
	width, height int
	theme         Theme
	groups        []*KindGroup
	focused       bool
	cursor        int        // flat index in the visible items
	items         []treeItem // flattened visible items
	selected      string     // UID of the selected resource

	// Persistent expanded state keyed by kind name.
	// true = expanded, false = collapsed. Kinds not in the map default to expanded.
	expandState map[string]bool

	// Compare mark support
	compareLeft  *CompareItem
	compareRight *CompareItem

	// Analysis tags
	analysisTags map[string][]ChangeTag // resourceUID -> tags for latest revision
}

type treeItemType int

const (
	treeItemKind treeItemType = iota
	treeItemResource
)

type treeItem struct {
	Type     treeItemType
	Kind     string        // for both types
	Resource *ResourceData // only for treeItemResource
}

func NewResourceTree(theme Theme) *ResourceTree {
	return &ResourceTree{theme: theme, expandState: make(map[string]bool)}
}

func (rt *ResourceTree) SetSize(w, h int) { rt.width = w; rt.height = h; rt.buildItems() }
func (rt *ResourceTree) SetFocus(f bool)  { rt.focused = f }
func (rt *ResourceTree) SetGroups(groups []*KindGroup) {
	// Apply persistent expand state: if the user has previously collapsed a kind,
	// carry that forward. New kinds default to expanded.
	for _, g := range groups {
		if expanded, ok := rt.expandState[g.Kind]; ok {
			g.Expanded = expanded
		}
	}
	rt.groups = groups
	rt.buildItems()
}
func (rt *ResourceTree) SelectedUID() string { return rt.selected }

// SelectByUID selects a resource by its UID, expanding the kind group if needed
// and moving the cursor to the matching item.
func (rt *ResourceTree) SelectByUID(uid string) {
	rt.selected = uid

	// First, check if it's already visible in the flat items
	for i, item := range rt.items {
		if item.Type == treeItemResource && item.Resource != nil && item.Resource.Resource.UID == uid {
			rt.cursor = i
			return
		}
	}

	// Not visible — find the kind group and expand it
	for _, g := range rt.groups {
		for _, rd := range g.Resources {
			if rd.Resource.UID == uid {
				if !g.Expanded {
					g.Expanded = true
					rt.expandState[g.Kind] = true
					rt.buildItems()
				}
				// Now find it in the rebuilt items
				for i, item := range rt.items {
					if item.Type == treeItemResource && item.Resource != nil && item.Resource.Resource.UID == uid {
						rt.cursor = i
						return
					}
				}
				return
			}
		}
	}
}

// CursorInfo returns cursor position and total item count.
func (rt *ResourceTree) CursorInfo() (int, int) { return rt.cursor, len(rt.items) }

// CanScrollUp returns true if there are items above the visible window.
func (rt *ResourceTree) CanScrollUp() bool {
	if len(rt.items) == 0 || rt.height <= 0 {
		return false
	}
	startIdx := 0
	if rt.cursor >= rt.height {
		startIdx = rt.cursor - rt.height + 1
	}
	return startIdx > 0
}

// CanScrollDown returns true if there are items below the visible window.
func (rt *ResourceTree) CanScrollDown() bool {
	if len(rt.items) == 0 || rt.height <= 0 {
		return false
	}
	startIdx := 0
	if rt.cursor >= rt.height {
		startIdx = rt.cursor - rt.height + 1
	}
	return startIdx+rt.height < len(rt.items)
}

func (rt *ResourceTree) SetCompareMarks(left, right *CompareItem) {
	rt.compareLeft = left
	rt.compareRight = right
}

func (rt *ResourceTree) SetAnalysisTags(tags map[string][]ChangeTag) {
	rt.analysisTags = tags
}

func (rt *ResourceTree) buildItems() {
	rt.items = nil
	for _, g := range rt.groups {
		rt.items = append(rt.items, treeItem{Type: treeItemKind, Kind: g.Kind})
		if g.Expanded {
			for _, rd := range g.Resources {
				rt.items = append(rt.items, treeItem{Type: treeItemResource, Kind: g.Kind, Resource: rd})
			}
		}
	}

	// After rebuilding, try to re-locate the selected UID so the cursor
	// stays on the correct item even if item order changed.
	if rt.selected != "" {
		for i, item := range rt.items {
			if item.Type == treeItemResource && item.Resource != nil && item.Resource.Resource.UID == rt.selected {
				rt.cursor = i
				return
			}
		}
	}

	// Fallback: clamp cursor if selected UID wasn't found (e.g. collapsed group)
	if rt.cursor >= len(rt.items) && len(rt.items) > 0 {
		rt.cursor = len(rt.items) - 1
	}
}

func (rt *ResourceTree) Update(msg tea.Msg) tea.Cmd {
	if !rt.focused {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if rt.cursor < len(rt.items)-1 {
				rt.cursor++
			}
			return rt.selectCurrent()
		case "k", "up":
			if rt.cursor > 0 {
				rt.cursor--
			}
			return rt.selectCurrent()
		case "enter", " ":
			if rt.cursor < len(rt.items) {
				item := rt.items[rt.cursor]
				if item.Type == treeItemKind {
					for _, g := range rt.groups {
						if g.Kind == item.Kind {
							g.Expanded = !g.Expanded
							rt.expandState[g.Kind] = g.Expanded
							break
						}
					}
					rt.buildItems()
					return nil
				}
				return rt.selectCurrent()
			}
		case "s":
			if rt.cursor < len(rt.items) {
				item := rt.items[rt.cursor]
				if item.Type == treeItemResource && item.Resource != nil {
					return Cmd(ToggleStarMsg{UID: item.Resource.Resource.UID})
				}
			}
		case "g", "home":
			rt.cursor = 0
			return rt.selectCurrent()
		case "G", "end":
			if len(rt.items) > 0 {
				rt.cursor = len(rt.items) - 1
			}
			return rt.selectCurrent()
		case "ctrl+d", "pgdown":
			pageSize := rt.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			rt.cursor += pageSize
			if rt.cursor >= len(rt.items) {
				rt.cursor = len(rt.items) - 1
			}
			return rt.selectCurrent()
		case "ctrl+u", "pgup":
			pageSize := rt.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			rt.cursor -= pageSize
			if rt.cursor < 0 {
				rt.cursor = 0
			}
			return rt.selectCurrent()
		}
	}
	return nil
}

func (rt *ResourceTree) selectCurrent() tea.Cmd {
	if rt.cursor >= len(rt.items) {
		return nil
	}
	item := rt.items[rt.cursor]
	if item.Type == treeItemResource && item.Resource != nil {
		rt.selected = item.Resource.Resource.UID
		return Cmd(ResourceSelectedMsg{Resource: item.Resource})
	}
	return nil
}

// CurrentHint returns a context-sensitive hint for the status bar.
func (rt *ResourceTree) CurrentHint() string {
	if rt.cursor >= len(rt.items) {
		return ""
	}
	item := rt.items[rt.cursor]
	switch item.Type {
	case treeItemKind:
		return "Enter: expand/collapse  s: star  ctrl+d/u: page"
	case treeItemResource:
		if item.Resource == nil {
			return ""
		}
		rd := item.Resource
		var hints []string
		if rd.Resource.Starred {
			hints = append(hints, "*=starred")
		}
		if rd.DetectLoop(6) {
			hints = append(hints, "~=loop detected")
		}
		freq := rd.ChangeFrequency()
		if freq > 5 {
			hints = append(hints, "!=high frequency")
		} else if freq > 2 {
			hints = append(hints, "~=warm frequency")
		}
		latest := rd.LatestRevision()
		if latest != nil {
			age := RelativeTime(latest.Time)
			if age == "now" || strings.HasSuffix(age, "s") {
				hints = append(hints, "@=recently active")
			} else {
				hints = append(hints, "o=idle")
			}
		}
		if len(hints) > 0 {
			return strings.Join(hints, "  ")
		}
		return "Enter: select  s: star  c: compare  ctrl+d/u: page"
	}
	return ""
}

func (rt *ResourceTree) View() string {
	if len(rt.items) == 0 {
		placeholder := lipgloss.NewStyle().
			Foreground(rt.theme.Overlay0).
			Italic(true).
			Render("  ◇ No resources to display")
		var lines []string
		lines = append(lines, PadRight(placeholder, rt.width))
		hint := lipgloss.NewStyle().Foreground(rt.theme.Overlay0).
			Render("    Press 's' to star resources")
		lines = append(lines, PadRight(hint, rt.width))
		for len(lines) < rt.height {
			lines = append(lines, strings.Repeat(" ", rt.width))
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	// Reserve space for: indent(2) + star(2) + indicator(1) + space(1) + name + badges(~5)
	maxNameLen := rt.width - 12
	if maxNameLen < 5 {
		maxNameLen = 5
	}

	// Calculate visible window (scrolling)
	startIdx := 0
	if rt.cursor >= rt.height {
		startIdx = rt.cursor - rt.height + 1
	}

	for i := startIdx; i < len(rt.items) && len(lines) < rt.height; i++ {
		item := rt.items[i]
		isSelected := i == rt.cursor && rt.focused
		// Dim highlight for the selected resource when panel is unfocused
		isPassive := !rt.focused && item.Type == treeItemResource &&
			item.Resource != nil && item.Resource.Resource.UID == rt.selected && rt.selected != ""

		var line string
		switch item.Type {
		case treeItemKind:
			var g *KindGroup
			for _, grp := range rt.groups {
				if grp.Kind == item.Kind {
					g = grp
					break
				}
			}
			arrow := "▸"
			if g != nil && g.Expanded {
				arrow = "▾"
			}
			arrowStyle := lipgloss.NewStyle().Foreground(rt.theme.Overlay1)
			kindStyle := lipgloss.NewStyle().Foreground(rt.theme.Blue).Bold(true)
			countStyle := lipgloss.NewStyle().Foreground(rt.theme.Overlay1)
			count := 0
			if g != nil {
				count = len(g.Resources)
			}
			line = arrowStyle.Render(arrow) + " " + kindStyle.Render(item.Kind) + " " + countStyle.Render(fmt.Sprintf("%d", count))

		case treeItemResource:
			rd := item.Resource
			r := rd.Resource

			star := "  "
			if r.Starred {
				star = rt.theme.StarStyle().Render("★") + " "
			}

			indicator := lipgloss.NewStyle().Foreground(rt.theme.Overlay0).Render("○")
			if rd.LatestRevision() != nil {
				age := RelativeTime(rd.LatestRevision().Time)
				if age == "now" || strings.HasSuffix(age, "s") {
					indicator = lipgloss.NewStyle().Foreground(rt.theme.Peach).Render("●")
				}
			}

			loopBadge := ""
			if rd.DetectLoop(6) {
				loopBadge = " " + lipgloss.NewStyle().
					Foreground(rt.theme.Red).Bold(true).
					Render("↻")
			}

			freqBadge := ""
			freq := rd.ChangeFrequency()
			if freq > 5 {
				freqBadge = " " + rt.theme.HotBadgeStyle().Render("▲")
			} else if freq > 2 {
				freqBadge = " " + rt.theme.WarmBadgeStyle().Render("△")
			}

			// Compare mark badges
			compareBadge := ""
			if rt.compareLeft != nil && rt.compareLeft.Resource.UID == r.UID {
				compareBadge = " " + lipgloss.NewStyle().Foreground(rt.theme.Blue).Bold(true).Render("[C1]")
			}
			if rt.compareRight != nil && rt.compareRight.Resource.UID == r.UID {
				compareBadge = " " + lipgloss.NewStyle().Foreground(rt.theme.Mauve).Bold(true).Render("[C2]")
			}

			name := r.ShortName(maxNameLen)
			nameStyle := lipgloss.NewStyle().Foreground(rt.theme.Text)

			line = "  " + star + indicator + " " + nameStyle.Render(name) + loopBadge + freqBadge + compareBadge
		}

		// Pad to exact width, then apply background if selected
		padded := PadRight(line, rt.width)
		if isSelected {
			padded = lipgloss.NewStyle().
				Background(rt.theme.Surface0).
				Bold(true).
				Render(padded)
		} else if isPassive {
			padded = lipgloss.NewStyle().
				Background(rt.theme.Surface0).
				Render(padded)
		}

		lines = append(lines, padded)
	}

	// Pad remaining lines
	for len(lines) < rt.height {
		lines = append(lines, strings.Repeat(" ", rt.width))
	}

	return strings.Join(lines, "\n")
}

// ─── Revision List ───

// RevisionList shows the revision history for a selected resource.
type RevisionList struct {
	width, height int
	theme         Theme
	focused       bool
	resource      *ResourceData
	cursor        int // index into revisions slice (0 = oldest, len-1 = newest)

	// Stable selection by ID
	selectedID RevisionID

	// Compare mark support
	compareLeft  *CompareItem
	compareRight *CompareItem

	// Auto-scroll
	autoScroll bool

	// Analysis tags
	analysisTags map[RevisionID][]ChangeTag
}

func NewRevisionList(theme Theme) *RevisionList {
	return &RevisionList{theme: theme}
}

func (rl *RevisionList) SetSize(w, h int) { rl.width = w; rl.height = h }
func (rl *RevisionList) SetFocus(f bool)  { rl.focused = f }

func (rl *RevisionList) SetResource(rd *ResourceData) {
	oldResource := rl.resource
	rl.resource = rd
	if rd != nil && len(rd.Revisions) > 0 {
		// If same resource and we have a previously selected ID, try to preserve position
		if rl.selectedID != 0 && oldResource != nil && oldResource.Resource.UID == rd.Resource.UID {
			for i, rev := range rd.Revisions {
				if rev.ID == rl.selectedID {
					rl.cursor = i
					return
				}
			}
		}
		// New resource or old ID not found: select newest
		rl.cursor = len(rd.Revisions) - 1
		rl.selectedID = rd.Revisions[rl.cursor].ID
	} else {
		rl.cursor = 0
		rl.selectedID = 0
	}
}

func (rl *RevisionList) SetCompareMarks(left, right *CompareItem) {
	rl.compareLeft = left
	rl.compareRight = right
}

func (rl *RevisionList) SetAutoScroll(on bool) {
	rl.autoScroll = on
}

func (rl *RevisionList) SetAnalysisTags(tags map[RevisionID][]ChangeTag) {
	rl.analysisTags = tags
}

// JumpToNewest moves cursor to the newest revision (for auto-scroll).
func (rl *RevisionList) JumpToNewest() {
	if rl.resource != nil && len(rl.resource.Revisions) > 0 {
		rl.cursor = len(rl.resource.Revisions) - 1
		rl.selectedID = rl.resource.Revisions[rl.cursor].ID
	}
}

// SelectIndex moves the cursor to a specific revision index and updates selectedID.
// This is used by external callers (e.g. SetRevision in views) to sync the
// RevisionList cursor when the selection changes from outside (e.g. [/] keys in DetailView).
func (rl *RevisionList) SelectIndex(idx int) {
	if rl.resource == nil || len(rl.resource.Revisions) == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rl.resource.Revisions) {
		idx = len(rl.resource.Revisions) - 1
	}
	rl.cursor = idx
	rl.selectedID = rl.resource.Revisions[idx].ID
}

// CursorInfo returns cursor position and total count.
func (rl *RevisionList) CursorInfo() (int, int) {
	if rl.resource == nil {
		return 0, 0
	}
	return rl.cursor, len(rl.resource.Revisions)
}

// CanScrollUp returns true if there are revisions above the visible window.
func (rl *RevisionList) CanScrollUp() bool {
	if rl.resource == nil {
		return false
	}
	revs := rl.resource.Revisions
	total := len(revs)
	itemHeight := rl.height - 1 // 1 line for title
	if itemHeight <= 0 || total <= itemHeight {
		return false
	}
	visualCursor := total - 1 - rl.cursor
	startVisual := 0
	if visualCursor >= itemHeight {
		startVisual = visualCursor - itemHeight + 1
	}
	if startVisual+itemHeight > total {
		startVisual = total - itemHeight
	}
	if startVisual < 0 {
		startVisual = 0
	}
	return startVisual > 0
}

// CanScrollDown returns true if there are revisions below the visible window.
func (rl *RevisionList) CanScrollDown() bool {
	if rl.resource == nil {
		return false
	}
	revs := rl.resource.Revisions
	total := len(revs)
	itemHeight := rl.height - 1 // 1 line for title
	if itemHeight <= 0 || total <= itemHeight {
		return false
	}
	visualCursor := total - 1 - rl.cursor
	startVisual := 0
	if visualCursor >= itemHeight {
		startVisual = visualCursor - itemHeight + 1
	}
	if startVisual+itemHeight > total {
		startVisual = total - itemHeight
	}
	if startVisual < 0 {
		startVisual = 0
	}
	return startVisual+itemHeight < total
}

func (rl *RevisionList) Update(msg tea.Msg) tea.Cmd {
	if !rl.focused || rl.resource == nil {
		return nil
	}
	revs := rl.resource.Revisions
	total := len(revs)
	if total == 0 {
		return nil
	}

	// cursor is a slice index into revs[]. Display order is newest-first:
	// visual row 0 = revs[total-1] (newest), visual row total-1 = revs[0] (oldest).
	// To convert: visualRow = total - 1 - cursor, cursor = total - 1 - visualRow.
	// j/down = move visual cursor down = toward older = decrease cursor
	// k/up = move visual cursor up = toward newer = increase cursor

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if rl.cursor > 0 {
				rl.cursor--
			}
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		case "k", "up":
			if rl.cursor < total-1 {
				rl.cursor++
			}
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		case "ctrl+d", "pgdown":
			pageSize := rl.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			rl.cursor -= pageSize
			if rl.cursor < 0 {
				rl.cursor = 0
			}
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		case "ctrl+u", "pgup":
			pageSize := rl.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			rl.cursor += pageSize
			if rl.cursor >= total {
				rl.cursor = total - 1
			}
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		case "enter":
			return rl.selectCurrent()
		case "s":
			return Cmd(ToggleStarMsg{UID: rl.resource.Resource.UID})
		case "c":
			return Cmd(CompareMarkMsg{Resource: rl.resource, Index: rl.cursor})
		case "g", "home":
			// Go to top of visual list = newest = highest index
			rl.cursor = total - 1
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		case "G", "end":
			// Go to bottom of visual list = oldest = index 0
			rl.cursor = 0
			rl.selectedID = revs[rl.cursor].ID
			return rl.selectCurrent()
		}
	}
	return nil
}

func (rl *RevisionList) selectCurrent() tea.Cmd {
	if rl.resource == nil || rl.cursor < 0 || rl.cursor >= len(rl.resource.Revisions) {
		return nil
	}
	return Cmd(RevisionSelectedMsg{Resource: rl.resource, Index: rl.cursor})
}

// CurrentHint returns a context-sensitive hint for the status bar.
func (rl *RevisionList) CurrentHint() string {
	if rl.resource == nil || rl.cursor < 0 || rl.cursor >= len(rl.resource.Revisions) {
		return ""
	}
	rev := rl.resource.Revisions[rl.cursor]
	var parts []string

	// Compare badges
	if rl.compareLeft != nil && rl.compareLeft.Revision.ID == rev.ID {
		parts = append(parts, "[C1]=compare left")
	}
	if rl.compareRight != nil && rl.compareRight.Revision.ID == rev.ID {
		parts = append(parts, "[C2]=compare right")
	}

	// Analysis tags
	if rl.analysisTags != nil {
		if tags, ok := rl.analysisTags[rev.ID]; ok {
			var tagStrs []string
			for _, t := range tags {
				tagStrs = append(tagStrs, string(t))
			}
			parts = append(parts, "changes: "+strings.Join(tagStrs, ","))
		}
	}

	if rl.autoScroll {
		parts = append(parts, "auto-scroll ON")
	}

	if len(parts) > 0 {
		return strings.Join(parts, "  ")
	}
	return "c: compare  [/]: prev/next  d/o/p/J: view modes  ctrl+d/u: page"
}

func (rl *RevisionList) View() string {
	if rl.resource == nil {
		placeholder := lipgloss.NewStyle().
			Foreground(rl.theme.Overlay0).
			Italic(true).
			Render("  ◇ Select a resource")
		var lines []string
		lines = append(lines, PadRight(placeholder, rl.width))
		for len(lines) < rl.height {
			lines = append(lines, strings.Repeat(" ", rl.width))
		}
		return strings.Join(lines, "\n")
	}

	revs := rl.resource.Revisions
	total := len(revs)
	loopInfo := rl.resource.AnalyzeLoop(8)

	var lines []string

	// Title line with loop detection
	title := lipgloss.NewStyle().Foreground(rl.theme.Overlay1).Render("Revisions")
	countStr := lipgloss.NewStyle().Foreground(rl.theme.Overlay0).Render(fmt.Sprintf(" (%d)", total))
	titleLine := " " + title + countStr
	if loopInfo.IsLoop {
		// Compact loop indicator: "↻ A↔B x3 ~30s"
		// Uses the pre-built PatternSample from AnalyzeLoop and fits within available width.
		loopStr := fmt.Sprintf(" ↻ %s", loopInfo.PatternSample)
		if loopInfo.Cycles > 0 {
			loopStr += fmt.Sprintf(" x%d", loopInfo.Cycles)
		}
		if loopInfo.Period > 0 {
			loopStr += fmt.Sprintf(" ~%s", loopInfo.Period.Round(time.Second))
		}
		// Truncate to fit remaining panel width (leave room for [AUTO] badge)
		maxLoopW := rl.width - lipgloss.Width(titleLine) - 8
		if maxLoopW < 6 {
			loopStr = " ↻"
		} else if lipgloss.Width(loopStr) > maxLoopW {
			loopStr = " ↻ " + loopInfo.PatternSample
			if lipgloss.Width(loopStr) > maxLoopW {
				loopStr = " ↻"
			}
		}
		loopWarn := lipgloss.NewStyle().
			Foreground(rl.theme.Red).Bold(true).
			Render(loopStr)
		titleLine += loopWarn
	}
	if rl.autoScroll {
		asBadge := lipgloss.NewStyle().Foreground(rl.theme.Teal).Bold(true).Render(" [AUTO]")
		titleLine += asBadge
	}
	lines = append(lines, PadRight(titleLine, rl.width))

	// Available height for revision items (after title line)
	itemHeight := rl.height - len(lines)
	if itemHeight <= 0 {
		return strings.Join(lines, "\n")
	}

	// Display order: visual row 0 = revs[total-1] (newest) ... row total-1 = revs[0] (oldest).
	// The cursor's visual row = total - 1 - rl.cursor.
	visualCursor := total - 1 - rl.cursor

	// Compute scroll window based on visual cursor
	startVisual := 0
	if visualCursor >= itemHeight {
		startVisual = visualCursor - itemHeight + 1
	}
	// Clamp so we don't go past the end
	if startVisual+itemHeight > total {
		startVisual = total - itemHeight
	}
	if startVisual < 0 {
		startVisual = 0
	}

	endVisual := startVisual + itemHeight
	if endVisual > total {
		endVisual = total
	}

	// Render the visible window
	for v := startVisual; v < endVisual; v++ {
		// Convert visual row to slice index
		sliceIdx := total - 1 - v
		rev := revs[sliceIdx]
		isSelected := sliceIdx == rl.cursor && rl.focused
		isPassive := sliceIdx == rl.cursor && !rl.focused
		isCurrent := sliceIdx == rl.cursor

		dot := lipgloss.NewStyle().Foreground(rl.theme.Overlay0).Render("○")
		if isCurrent {
			dot = lipgloss.NewStyle().Foreground(rl.theme.Blue).Render("●")
		}

		idStr := lipgloss.NewStyle().Foreground(rl.theme.Mauve).Render(rev.ID.String())

		etStyle := rl.theme.EventTypeStyle(rev.EventType)
		etStr := etStyle.Render(string(rev.EventType)[:3])

		timeStr := lipgloss.NewStyle().Foreground(timeColor(rl.theme, rev.Time)).Render(RelativeTime(rev.Time))

		// Compare badges
		compareBadge := ""
		if rl.compareLeft != nil && rl.compareLeft.Revision.ID == rev.ID {
			compareBadge = lipgloss.NewStyle().Foreground(rl.theme.Blue).Bold(true).Render("[C1]") + " "
		}
		if rl.compareRight != nil && rl.compareRight.Revision.ID == rev.ID {
			compareBadge = lipgloss.NewStyle().Foreground(rl.theme.Mauve).Bold(true).Render("[C2]") + " "
		}

		// Analysis tag badge
		tagBadge := ""
		if rl.analysisTags != nil {
			if tags, ok := rl.analysisTags[rev.ID]; ok && len(tags) > 0 {
				tagStr := string(tags[0])
				if len(tags) > 1 {
					tagStr += fmt.Sprintf("+%d", len(tags)-1)
				}
				tagBadge = " " + lipgloss.NewStyle().Foreground(rl.theme.Teal).Render("["+tagStr+"]")
			}
		}

		// Loop state label: show which "state" this revision matches (A, B, ...)
		loopBadge := ""
		if loopInfo.IsLoop && loopInfo.LoopRevisions != nil {
			if label, ok := loopInfo.LoopRevisions[rev.ID]; ok {
				loopBadge = " " + lipgloss.NewStyle().Foreground(rl.theme.Red).Render("↻"+label)
			}
		}

		line := " " + dot + " " + compareBadge + idStr + " " + etStr + " " + timeStr + tagBadge + loopBadge

		padded := PadRight(line, rl.width)
		if isSelected {
			padded = lipgloss.NewStyle().
				Background(rl.theme.Surface0).
				Bold(true).
				Render(padded)
		} else if isPassive {
			padded = lipgloss.NewStyle().
				Background(rl.theme.Surface0).
				Render(padded)
		}

		lines = append(lines, padded)
	}

	// Pad
	for len(lines) < rl.height {
		lines = append(lines, strings.Repeat(" ", rl.width))
	}

	return strings.Join(lines, "\n")
}

// ─── Detail View ───

// DetailView renders the diff/object/patch/JSON for a selected revision.
type DetailView struct {
	width, height int
	theme         Theme
	focused       bool
	viewport      viewport.Model
	resource      *ResourceData
	revIndex      int
	viewMode      ViewMode
	content       string
}

func NewDetailView(theme Theme) *DetailView {
	vp := viewport.New(0, 0)
	return &DetailView{
		theme:    theme,
		viewport: vp,
		viewMode: DiffMode,
	}
}

func (dv *DetailView) SetSize(w, h int) {
	dv.width = w
	dv.height = h
	dv.viewport.Width = w
	dv.viewport.Height = h
	dv.renderContent()
}

func (dv *DetailView) SetFocus(f bool) { dv.focused = f }

func (dv *DetailView) SetRevision(rd *ResourceData, index int) {
	dv.resource = rd
	dv.revIndex = index
	dv.renderContent()
}

func (dv *DetailView) SetViewMode(mode ViewMode) {
	dv.viewMode = mode
	dv.renderContent()
}

func (dv *DetailView) ViewMode() ViewMode { return dv.viewMode }

// CanScrollUp returns true if the viewport has content above the visible area.
func (dv *DetailView) CanScrollUp() bool { return !dv.viewport.AtTop() }

// CanScrollDown returns true if the viewport has content below the visible area.
func (dv *DetailView) CanScrollDown() bool { return !dv.viewport.AtBottom() }

// CurrentHint returns a context-sensitive hint for the status bar.
func (dv *DetailView) CurrentHint() string {
	if dv.resource == nil {
		return ""
	}
	return "d=diff  o=object  p=patch  J=json  r=raw  [/]=prev/next  c=compare  t=timeline  e=export  y=copy"
}

func (dv *DetailView) renderContent() {
	if dv.resource == nil || dv.revIndex < 0 || dv.revIndex >= len(dv.resource.Revisions) {
		dv.content = lipgloss.NewStyle().
			Foreground(dv.theme.Overlay0).Italic(true).
			Render("  ◇ Select a revision to view details")
		dv.viewport.SetContent(dv.content)
		return
	}

	rev := dv.resource.Revisions[dv.revIndex]

	title := fmt.Sprintf("%s  %s  %s  %s",
		dv.resource.Resource.KindName(),
		lipgloss.NewStyle().Foreground(dv.theme.Mauve).Render(rev.ID.String()),
		dv.theme.EventTypeStyle(rev.EventType).Render(string(rev.EventType)),
		lipgloss.NewStyle().Foreground(dv.theme.Overlay1).Render(FormatTimestamp(rev.Time)),
	)
	titleLine := lipgloss.NewStyle().Bold(true).Render(title)

	sepW := dv.width
	if sepW <= 0 {
		sepW = 40
	}
	separator := lipgloss.NewStyle().Foreground(dv.theme.Surface1).Render(strings.Repeat("─", sepW))

	var body string
	switch dv.viewMode {
	case DiffMode:
		body = dv.renderDiff(rev)
	case ObjectMode:
		body = RenderYAMLObject(rev.Object, dv.theme, 2)
	case PatchMode:
		if rev.Patch != nil {
			body = RenderYAMLObject(rev.Patch, dv.theme, 2)
		} else {
			body = lipgloss.NewStyle().Foreground(dv.theme.Overlay0).Italic(true).
				Render("(no patch - this is a full snapshot)")
		}
	case JSONMode:
		body = RenderJSONObject(rev.Object, dv.theme)
	case RawMode:
		body = dv.renderRaw(rev)
	}

	dv.content = titleLine + "\n" + separator + "\n" + body
	dv.viewport.SetContent(dv.content)
	dv.viewport.GotoTop()
}

func (dv *DetailView) renderDiff(rev Revision) string {
	if rev.Object == nil {
		return dv.theme.MutedStyle().Render("(no object data)")
	}

	var prevObj map[string]any
	if dv.revIndex > 0 {
		prevObj = dv.resource.Revisions[dv.revIndex-1].Object
	}

	if prevObj == nil {
		return RenderYAMLObject(rev.Object, dv.theme, 2)
	}

	dpTheme := diffpreview.Theme{
		KeyStyle:    lipgloss.NewStyle().Foreground(dv.theme.Blue),
		StringStyle: lipgloss.NewStyle().Foreground(dv.theme.Green),
		NumberStyle: lipgloss.NewStyle().Foreground(dv.theme.Peach),
		BoolStyle:   lipgloss.NewStyle().Foreground(dv.theme.Yellow),
		NullStyle:   lipgloss.NewStyle().Foreground(dv.theme.Overlay0).Italic(true),
		AddedBg:     lipgloss.NewStyle().Background(dv.theme.DiffAddedBg).Foreground(dv.theme.Green),
		RemovedBg:   lipgloss.NewStyle().Background(dv.theme.DiffRemovedBg).Foreground(dv.theme.Red),
		ModifiedBg:  lipgloss.NewStyle().Background(dv.theme.DiffModifiedBg).Foreground(dv.theme.Peach),
	}

	node := diffpreview.DiffRecursive(prevObj, rev.Object)
	return diffpreview.RenderYAML(node, dpTheme, diffpreview.RenderOptions{
		IndentSize:                2,
		EnableBackgroundHighlight: true,
	})
}

// renderRaw shows the revision as a raw database record — useful for debugging.
func (dv *DetailView) renderRaw(rev Revision) string {
	var lines []string
	headerStyle := lipgloss.NewStyle().Foreground(dv.theme.Lavender).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(dv.theme.Blue)
	valStyle := lipgloss.NewStyle().Foreground(dv.theme.Text)
	mutedStyle := lipgloss.NewStyle().Foreground(dv.theme.Overlay0)

	lines = append(lines, headerStyle.Render("── Database Record ──"))
	lines = append(lines, "")
	lines = append(lines, keyStyle.Render("Revision ID:  ")+valStyle.Render(rev.ID.String()))
	lines = append(lines, keyStyle.Render("Event Type:   ")+valStyle.Render(string(rev.EventType)))
	lines = append(lines, keyStyle.Render("Timestamp:    ")+valStyle.Render(rev.Time.Format("2006-01-02T15:04:05.000Z07:00")))
	if dv.resource != nil {
		r := dv.resource.Resource
		lines = append(lines, keyStyle.Render("Resource UID: ")+valStyle.Render(r.UID))
		lines = append(lines, keyStyle.Render("Kind:         ")+valStyle.Render(r.Kind))
		lines = append(lines, keyStyle.Render("Name:         ")+valStyle.Render(r.Name))
		lines = append(lines, keyStyle.Render("Namespace:    ")+valStyle.Render(r.Namespace))
		lines = append(lines, keyStyle.Render("Starred:      ")+valStyle.Render(fmt.Sprintf("%v", r.Starred)))
	}
	lines = append(lines, "")

	lines = append(lines, headerStyle.Render("── Object (raw JSON) ──"))
	lines = append(lines, "")
	if rev.Object != nil {
		raw, err := json.MarshalIndent(rev.Object, "", "  ")
		if err != nil {
			lines = append(lines, mutedStyle.Render("(error marshaling: "+err.Error()+")"))
		} else {
			lines = append(lines, string(raw))
		}
	} else {
		lines = append(lines, mutedStyle.Render("(nil)"))
	}
	lines = append(lines, "")

	lines = append(lines, headerStyle.Render("── Patch (raw JSON) ──"))
	lines = append(lines, "")
	if rev.Patch != nil {
		raw, err := json.MarshalIndent(rev.Patch, "", "  ")
		if err != nil {
			lines = append(lines, mutedStyle.Render("(error marshaling: "+err.Error()+")"))
		} else {
			lines = append(lines, string(raw))
		}
	} else {
		lines = append(lines, mutedStyle.Render("(nil)"))
	}

	// Analysis info
	lines = append(lines, "")
	lines = append(lines, headerStyle.Render("── Analysis ──"))
	lines = append(lines, "")
	if dv.resource != nil {
		loopInfo := dv.resource.AnalyzeLoop(8)
		if loopInfo.IsLoop {
			lines = append(lines, keyStyle.Render("Loop:         ")+valStyle.Render(fmt.Sprintf(
				"YES  pattern=%s  states=%d  cycles=%d  period=%s",
				loopInfo.PatternSample, loopInfo.DistinctStates, loopInfo.Cycles, loopInfo.Period.Round(time.Second))))
		} else {
			lines = append(lines, keyStyle.Render("Loop:         ")+valStyle.Render("no"))
		}
		lines = append(lines, keyStyle.Render("Frequency:    ")+valStyle.Render(fmt.Sprintf("%.2f changes/min", dv.resource.ChangeFrequency())))
		lines = append(lines, keyStyle.Render("Rev Index:    ")+valStyle.Render(fmt.Sprintf("%d / %d", dv.revIndex, len(dv.resource.Revisions)-1)))
	}

	return strings.Join(lines, "\n")
}

func (dv *DetailView) Update(msg tea.Msg) tea.Cmd {
	if !dv.focused {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "d":
			dv.SetViewMode(DiffMode)
			return Cmd(ViewModeChangedMsg{Mode: DiffMode})
		case "o":
			dv.SetViewMode(ObjectMode)
			return Cmd(ViewModeChangedMsg{Mode: ObjectMode})
		case "p":
			dv.SetViewMode(PatchMode)
			return Cmd(ViewModeChangedMsg{Mode: PatchMode})
		case "J":
			dv.SetViewMode(JSONMode)
			return Cmd(ViewModeChangedMsg{Mode: JSONMode})
		case "r":
			dv.SetViewMode(RawMode)
			return Cmd(ViewModeChangedMsg{Mode: RawMode})
		case "e":
			return Cmd(StatusMsg{Text: "Export: would save YAML to file (not implemented in prototype)", IsError: false})
		case "y":
			return Cmd(StatusMsg{Text: "Copied to clipboard (not implemented in prototype)", IsError: false})
		case "c":
			if dv.resource != nil {
				return Cmd(CompareMarkMsg{Resource: dv.resource, Index: dv.revIndex})
			}
		case "t":
			if dv.resource != nil && dv.revIndex < len(dv.resource.Revisions) {
				rev := dv.resource.Revisions[dv.revIndex]
				return Cmd(JumpToTimelineMsg{Entry: TimelineEntry{
					Resource: dv.resource.Resource,
					Revision: rev,
				}})
			}
		case "[":
			if dv.resource != nil && dv.revIndex > 0 {
				dv.revIndex--
				dv.renderContent()
				return Cmd(RevisionSelectedMsg{Resource: dv.resource, Index: dv.revIndex})
			}
		case "]":
			if dv.resource != nil && dv.revIndex < len(dv.resource.Revisions)-1 {
				dv.revIndex++
				dv.renderContent()
				return Cmd(RevisionSelectedMsg{Resource: dv.resource, Index: dv.revIndex})
			}
		default:
			var cmd tea.Cmd
			dv.viewport, cmd = dv.viewport.Update(msg)
			return cmd
		}
	}
	return nil
}

func (dv *DetailView) View() string {
	return dv.viewport.View()
}

// ─── Timeline List ───

// TimelineList shows a chronological cross-resource event stream.
type TimelineList struct {
	width, height int
	theme         Theme
	focused       bool
	entries       []TimelineEntry
	groups        []interface{} // TimelineEntry or BurstGroup
	cursor        int
	flatItems     []timelineFlatItem

	// Auto-scroll
	autoScroll bool

	// Window filter centered on anchor timestamp
	windowMode   WindowMode
	windowAnchor time.Time // the revision timestamp the window is centered on

	// Compare mark support
	compareLeft  *CompareItem
	compareRight *CompareItem
}

type timelineFlatItem struct {
	entry        *TimelineEntry
	isBurstStart bool
	isBurstEnd   bool
	isBurstMid   bool
	burstSize    int
}

func NewTimelineList(theme Theme) *TimelineList {
	return &TimelineList{theme: theme}
}

func (tl *TimelineList) SetSize(w, h int) { tl.width = w; tl.height = h }
func (tl *TimelineList) SetFocus(f bool)  { tl.focused = f }

func (tl *TimelineList) SetCompareMarks(left, right *CompareItem) {
	tl.compareLeft = left
	tl.compareRight = right
}

func (tl *TimelineList) SetAutoScroll(on bool) {
	tl.autoScroll = on
}

func (tl *TimelineList) SetWindowMode(wm WindowMode) {
	tl.windowMode = wm
	tl.rebuild()
}

func (tl *TimelineList) SetWindowAnchor(t time.Time) {
	tl.windowAnchor = t
	if tl.windowMode != WindowAll {
		tl.rebuild()
	}
}

func (tl *TimelineList) SetEntries(entries []TimelineEntry) {
	tl.entries = entries
	tl.rebuild()
}

func (tl *TimelineList) rebuild() {
	// Apply window filter centered on anchor timestamp
	filtered := tl.entries
	if tl.windowMode != WindowAll && !tl.windowAnchor.IsZero() {
		halfDur := WindowHalfDuration(tl.windowMode)
		windowStart := tl.windowAnchor.Add(-halfDur)
		windowEnd := tl.windowAnchor.Add(halfDur)
		filtered = nil
		for _, e := range tl.entries {
			t := e.Revision.Time
			if (t.Equal(windowStart) || t.After(windowStart)) &&
				(t.Equal(windowEnd) || t.Before(windowEnd)) {
				filtered = append(filtered, e)
			}
		}
	}

	tl.groups = GroupTimelineByBurst(filtered, 5*time.Second)
	tl.buildFlatItems()
	if tl.cursor >= len(tl.flatItems) && len(tl.flatItems) > 0 {
		tl.cursor = len(tl.flatItems) - 1
	}
	if len(tl.flatItems) > 0 && tl.cursor < 0 {
		tl.cursor = 0
	}
}

// JumpToNewest moves cursor to the first entry (newest).
func (tl *TimelineList) JumpToNewest() {
	if len(tl.flatItems) > 0 {
		tl.cursor = 0
	}
}

// ScrollToRevision finds a timeline entry by revision ID and moves the cursor to it.
func (tl *TimelineList) ScrollToRevision(revID RevisionID) {
	for i, item := range tl.flatItems {
		if item.entry != nil && item.entry.Revision.ID == revID {
			tl.cursor = i
			return
		}
	}
}

// SelectedEntry returns the currently selected timeline entry, or nil.
func (tl *TimelineList) SelectedEntry() *TimelineEntry {
	if tl.cursor >= 0 && tl.cursor < len(tl.flatItems) {
		return tl.flatItems[tl.cursor].entry
	}
	return nil
}

// CursorInfo returns cursor position and total count.
func (tl *TimelineList) CursorInfo() (int, int) { return tl.cursor, len(tl.flatItems) }

// CanScrollUp returns true if there are items above the visible window.
func (tl *TimelineList) CanScrollUp() bool {
	if len(tl.flatItems) == 0 || tl.height <= 0 {
		return false
	}
	// Account for optional header lines (window mode / auto-scroll banner)
	headerLines := 0
	if tl.windowMode != WindowAll || tl.autoScroll {
		headerLines = 1
	}
	visibleHeight := tl.height - headerLines
	if visibleHeight <= 0 {
		return false
	}
	startIdx := 0
	if tl.cursor >= visibleHeight {
		startIdx = tl.cursor - visibleHeight + 1
	}
	return startIdx > 0
}

// CanScrollDown returns true if there are items below the visible window.
func (tl *TimelineList) CanScrollDown() bool {
	if len(tl.flatItems) == 0 || tl.height <= 0 {
		return false
	}
	headerLines := 0
	if tl.windowMode != WindowAll || tl.autoScroll {
		headerLines = 1
	}
	visibleHeight := tl.height - headerLines
	if visibleHeight <= 0 {
		return false
	}
	startIdx := 0
	if tl.cursor >= visibleHeight {
		startIdx = tl.cursor - visibleHeight + 1
	}
	return startIdx+visibleHeight < len(tl.flatItems)
}

func (tl *TimelineList) buildFlatItems() {
	tl.flatItems = nil
	for _, g := range tl.groups {
		switch v := g.(type) {
		case TimelineEntry:
			tl.flatItems = append(tl.flatItems, timelineFlatItem{entry: &v})
		case BurstGroup:
			for i, e := range v.Entries {
				entry := e
				item := timelineFlatItem{
					entry:     &entry,
					burstSize: len(v.Entries),
				}
				if i == 0 {
					item.isBurstStart = true
				} else if i == len(v.Entries)-1 {
					item.isBurstEnd = true
				} else {
					item.isBurstMid = true
				}
				tl.flatItems = append(tl.flatItems, item)
			}
		}
	}
}

// CurrentHint returns a context-sensitive hint for the status bar.
func (tl *TimelineList) CurrentHint() string {
	if tl.cursor >= len(tl.flatItems) || tl.cursor < 0 {
		return ""
	}
	item := tl.flatItems[tl.cursor]
	var parts []string

	if item.isBurstStart {
		parts = append(parts, fmt.Sprintf("╭=burst start (%d events)", item.burstSize))
	} else if item.isBurstMid {
		parts = append(parts, "│=burst middle")
	} else if item.isBurstEnd {
		parts = append(parts, "╰=burst end")
	}

	if item.entry != nil && item.entry.Resource.Starred {
		parts = append(parts, "*=starred")
	}

	if tl.autoScroll {
		parts = append(parts, "auto-scroll ON")
	}

	if tl.windowMode != WindowAll {
		anchor := "none"
		if !tl.windowAnchor.IsZero() {
			anchor = FormatTimestamp(tl.windowAnchor)
		}
		parts = append(parts, "window: "+tl.windowMode.String()+" around "+anchor)
	}

	if len(parts) > 0 {
		return strings.Join(parts, "  ")
	}
	return "s: star  S: starred-only  Enter: select  w: time window  ctrl+d/u: page"
}

func (tl *TimelineList) Update(msg tea.Msg) tea.Cmd {
	if !tl.focused {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if tl.cursor < len(tl.flatItems)-1 {
				tl.cursor++
			}
			return tl.selectCurrent()
		case "k", "up":
			if tl.cursor > 0 {
				tl.cursor--
			}
			return tl.selectCurrent()
		case "enter":
			return tl.selectCurrent()
		case "s":
			if tl.cursor < len(tl.flatItems) {
				item := tl.flatItems[tl.cursor]
				if item.entry != nil {
					return Cmd(ToggleStarMsg{UID: item.entry.Resource.UID})
				}
			}
		case "g", "home":
			tl.cursor = 0
			return tl.selectCurrent()
		case "G", "end":
			if len(tl.flatItems) > 0 {
				tl.cursor = len(tl.flatItems) - 1
			}
			return tl.selectCurrent()
		case "ctrl+d", "pgdown":
			pageSize := tl.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			tl.cursor += pageSize
			if tl.cursor >= len(tl.flatItems) {
				tl.cursor = len(tl.flatItems) - 1
			}
			return tl.selectCurrent()
		case "ctrl+u", "pgup":
			pageSize := tl.height / 2
			if pageSize < 1 {
				pageSize = 1
			}
			tl.cursor -= pageSize
			if tl.cursor < 0 {
				tl.cursor = 0
			}
			return tl.selectCurrent()
		case "S":
			// Toggle starred-only filter for the timeline
			return Cmd(ToggleTimelineStarredMsg{})
		}
	}
	return nil
}

func (tl *TimelineList) selectCurrent() tea.Cmd {
	if tl.cursor >= len(tl.flatItems) {
		return nil
	}
	item := tl.flatItems[tl.cursor]
	if item.entry != nil {
		return Cmd(TimelineEntrySelectedMsg{Entry: *item.entry})
	}
	return nil
}

func (tl *TimelineList) View() string {
	var lines []string

	// Window mode banner when active
	if tl.windowMode != WindowAll || tl.autoScroll {
		var badges []string
		if tl.windowMode != WindowAll {
			anchorStr := "no anchor"
			if !tl.windowAnchor.IsZero() {
				anchorStr = FormatTimestamp(tl.windowAnchor)
			}
			badges = append(badges, lipgloss.NewStyle().Foreground(tl.theme.Sky).Render(
				tl.windowMode.String()+" around "+anchorStr))
			badges = append(badges, lipgloss.NewStyle().Foreground(tl.theme.Overlay0).Render(
				fmt.Sprintf("(%d events)", len(tl.flatItems))))
		}
		if tl.autoScroll {
			badges = append(badges, lipgloss.NewStyle().Foreground(tl.theme.Teal).Bold(true).Render("[AUTO]"))
		}
		headerLine := " " + strings.Join(badges, "  ")
		lines = append(lines, PadRight(headerLine, tl.width))
	}

	// Scroll window
	visibleHeight := tl.height - len(lines)
	startIdx := 0
	if tl.cursor >= visibleHeight {
		startIdx = tl.cursor - visibleHeight + 1
	}

	nameMaxW := tl.width - 26 // time(8) + spaces + event(3) + anchor(2) + padding
	if nameMaxW < 8 {
		nameMaxW = 8
	}

	for i := startIdx; i < len(tl.flatItems) && len(lines) < tl.height; i++ {
		item := tl.flatItems[i]
		if item.entry == nil {
			continue
		}

		isSelected := i == tl.cursor && tl.focused
		isPassive := i == tl.cursor && !tl.focused
		e := item.entry

		// Burst bracket prefix (2 chars)
		burstPrefix := "  "
		burstStyle := lipgloss.NewStyle().Foreground(tl.theme.Surface2)
		if item.isBurstStart {
			burstPrefix = burstStyle.Render("╭ ")
		} else if item.isBurstEnd {
			burstPrefix = burstStyle.Render("╰ ")
		} else if item.isBurstMid {
			burstPrefix = burstStyle.Render("│ ")
		}

		// Time with recency-based coloring
		timeStr := lipgloss.NewStyle().Foreground(timeColor(tl.theme, e.Revision.Time)).
			Render(FormatTimestamp(e.Revision.Time))

		// Anchor indicator: show "▸" next to the entry closest to anchor
		anchorMark := " "
		if tl.windowMode != WindowAll && !tl.windowAnchor.IsZero() {
			diff := e.Revision.Time.Sub(tl.windowAnchor)
			if diff < 0 {
				diff = -diff
			}
			if diff < 1*time.Second {
				anchorMark = lipgloss.NewStyle().Foreground(tl.theme.Yellow).Bold(true).Render("▸")
			}
		}

		kindName := lipgloss.NewStyle().Foreground(tl.theme.Text).Render(
			Truncate(e.Resource.KindName(), nameMaxW))

		etStyle := tl.theme.EventTypeStyle(e.Revision.EventType)
		etStr := etStyle.Render(string(e.Revision.EventType)[:3])

		star := ""
		if e.Resource.Starred {
			star = tl.theme.StarStyle().Render("★") + " "
		}

		// Compare badges
		compareBadge := ""
		if tl.compareLeft != nil && tl.compareLeft.Resource.UID == e.Resource.UID && tl.compareLeft.Revision.ID == e.Revision.ID {
			compareBadge = lipgloss.NewStyle().Foreground(tl.theme.Blue).Bold(true).Render("[C1]") + " "
		} else if tl.compareRight != nil && tl.compareRight.Resource.UID == e.Resource.UID && tl.compareRight.Revision.ID == e.Revision.ID {
			compareBadge = lipgloss.NewStyle().Foreground(tl.theme.Mauve).Bold(true).Render("[C2]") + " "
		}

		line := burstPrefix + anchorMark + timeStr + " " + compareBadge + star + kindName + " " + etStr

		padded := PadRight(line, tl.width)
		if isSelected {
			padded = lipgloss.NewStyle().
				Background(tl.theme.Surface0).Bold(true).
				Render(padded)
		} else if isPassive {
			padded = lipgloss.NewStyle().
				Background(tl.theme.Surface0).
				Render(padded)
		}

		lines = append(lines, padded)
	}

	for len(lines) < tl.height {
		lines = append(lines, strings.Repeat(" ", tl.width))
	}

	return strings.Join(lines, "\n")
}

// ─── Compare Panel ───

// ComparePanel renders side-by-side YAML comparison.
type ComparePanel struct {
	width, height int
	theme         Theme
	left          *CompareItem
	right         *CompareItem
	leftVP        viewport.Model
	rightVP       viewport.Model
	focusLeft     bool
}

func NewComparePanel(theme Theme) *ComparePanel {
	return &ComparePanel{
		theme:   theme,
		leftVP:  viewport.New(0, 0),
		rightVP: viewport.New(0, 0),
	}
}

func (cp *ComparePanel) SetSize(w, h int) {
	cp.width = w
	cp.height = h
	halfW := w/2 - 1
	if halfW < 5 {
		halfW = 5
	}
	cp.leftVP.Width = halfW
	cp.leftVP.Height = h - 2
	cp.rightVP.Width = halfW
	cp.rightVP.Height = h - 2
	cp.renderContent()
}

func (cp *ComparePanel) SetItems(left, right *CompareItem) {
	cp.left = left
	cp.right = right
	cp.renderContent()
}

// CanScrollUp returns true if the active viewport has content above.
func (cp *ComparePanel) CanScrollUp() bool {
	if cp.focusLeft {
		return !cp.leftVP.AtTop()
	}
	return !cp.rightVP.AtTop()
}

// CanScrollDown returns true if the active viewport has content below.
func (cp *ComparePanel) CanScrollDown() bool {
	if cp.focusLeft {
		return !cp.leftVP.AtBottom()
	}
	return !cp.rightVP.AtBottom()
}

func (cp *ComparePanel) renderContent() {
	dpTheme := diffpreview.Theme{
		KeyStyle:    lipgloss.NewStyle().Foreground(cp.theme.Blue),
		StringStyle: lipgloss.NewStyle().Foreground(cp.theme.Green),
		NumberStyle: lipgloss.NewStyle().Foreground(cp.theme.Peach),
		BoolStyle:   lipgloss.NewStyle().Foreground(cp.theme.Yellow),
		NullStyle:   lipgloss.NewStyle().Foreground(cp.theme.Overlay0).Italic(true),
		AddedBg:     lipgloss.NewStyle().Background(cp.theme.DiffAddedBg).Foreground(cp.theme.Green),
		RemovedBg:   lipgloss.NewStyle().Background(cp.theme.DiffRemovedBg).Foreground(cp.theme.Red),
		ModifiedBg:  lipgloss.NewStyle().Background(cp.theme.DiffModifiedBg).Foreground(cp.theme.Peach),
	}
	renderOpts := diffpreview.RenderOptions{
		IndentSize:                2,
		EnableBackgroundHighlight: true,
	}

	if cp.left != nil && cp.right != nil {
		// Both sides available — compute diff and highlight changes
		node := diffpreview.DiffRecursive(cp.left.Revision.Object, cp.right.Revision.Object)
		diffRendered := diffpreview.RenderYAML(node, dpTheme, renderOpts)
		// Show the diff on both sides: left = old, right = new (diff shows both)
		cp.leftVP.SetContent(RenderYAMLObject(cp.left.Revision.Object, cp.theme, 2))
		cp.rightVP.SetContent(diffRendered)
	} else {
		if cp.left != nil {
			cp.leftVP.SetContent(RenderYAMLObject(cp.left.Revision.Object, cp.theme, 2))
		} else {
			cp.leftVP.SetContent(lipgloss.NewStyle().Foreground(cp.theme.Overlay0).Italic(true).
				Render("Mark a revision with 'c' to compare"))
		}
		if cp.right != nil {
			cp.rightVP.SetContent(RenderYAMLObject(cp.right.Revision.Object, cp.theme, 2))
		} else {
			cp.rightVP.SetContent(lipgloss.NewStyle().Foreground(cp.theme.Overlay0).Italic(true).
				Render("Mark a second revision with 'c'"))
		}
	}
}

func (cp *ComparePanel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			cp.focusLeft = !cp.focusLeft
		case "X":
			return Cmd(CompareClearMsg{})
		default:
			if cp.focusLeft {
				var cmd tea.Cmd
				cp.leftVP, cmd = cp.leftVP.Update(msg)
				return cmd
			}
			var cmd tea.Cmd
			cp.rightVP, cmd = cp.rightVP.Update(msg)
			return cmd
		}
	}
	return nil
}

func (cp *ComparePanel) View() string {
	halfW := cp.width/2 - 1
	if halfW < 5 {
		halfW = 5
	}

	leftTitle := lipgloss.NewStyle().Foreground(cp.theme.Overlay0).Italic(true).Render("(empty)")
	if cp.left != nil {
		leftTitle = lipgloss.NewStyle().Foreground(cp.theme.Blue).Bold(true).
			Render(cp.left.Resource.KindName() + " @ " + cp.left.Revision.ID.String())
	}

	rightTitle := lipgloss.NewStyle().Foreground(cp.theme.Overlay0).Italic(true).Render("(empty)")
	if cp.right != nil {
		rightTitle = lipgloss.NewStyle().Foreground(cp.theme.Mauve).Bold(true).
			Render(cp.right.Resource.KindName() + " @ " + cp.right.Revision.ID.String())
	}

	leftHeader := PadRight(" "+leftTitle, halfW)
	rightHeader := PadRight(" "+rightTitle, halfW)
	sepStyle := lipgloss.NewStyle().Foreground(cp.theme.Surface1)
	header := leftHeader + sepStyle.Render("│") + rightHeader

	sep := sepStyle.Render(strings.Repeat("─", cp.width))

	leftContent := cp.leftVP.View()
	rightContent := cp.rightVP.View()

	// Join left and right viewports line by line
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")
	bodyH := cp.height - 2
	var bodyLines []string
	for i := 0; i < bodyH; i++ {
		var ll, rl string
		if i < len(leftLines) {
			ll = leftLines[i]
		}
		if i < len(rightLines) {
			rl = rightLines[i]
		}
		bodyLines = append(bodyLines, PadRight(ll, halfW)+sepStyle.Render("│")+PadRight(rl, halfW))
	}

	return header + "\n" + sep + "\n" + strings.Join(bodyLines, "\n")
}

// timeColor returns a color based on how recent a timestamp is.
// Very recent = bright green, minutes ago = teal, older = dim overlay.
func timeColor(theme Theme, t time.Time) lipgloss.Color {
	age := time.Since(t)
	switch {
	case age < 10*time.Second:
		return theme.Green
	case age < 1*time.Minute:
		return theme.Teal
	case age < 5*time.Minute:
		return theme.Subtext0
	case age < 15*time.Minute:
		return theme.Overlay1
	default:
		return theme.Overlay0
	}
}

// ScrollPosition returns a "cursor/total" string for display in panel titles.
func ScrollPosition(cursor, total int) string {
	if total == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", cursor+1, total)
}

package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// CommandPalette is a fuzzy-searchable modal overlay.
type CommandPalette struct {
	width, height int
	theme         Theme
	registry      *CommandRegistry
	query         string
	cursor        int
	visible       bool
	matches       []fuzzy.Match
}

func NewCommandPalette(theme Theme, registry *CommandRegistry) *CommandPalette {
	return &CommandPalette{
		theme:    theme,
		registry: registry,
	}
}

func (cp *CommandPalette) SetSize(w, h int) {
	cp.width = w
	cp.height = h
}

func (cp *CommandPalette) IsVisible() bool {
	return cp.visible
}

func (cp *CommandPalette) Show() {
	cp.visible = true
	cp.query = ""
	cp.cursor = 0
	cp.updateMatches()
}

func (cp *CommandPalette) Hide() {
	cp.visible = false
	cp.query = ""
	cp.cursor = 0
}

func (cp *CommandPalette) updateMatches() {
	names := cp.registry.Names()
	if cp.query == "" {
		// Show all commands
		cp.matches = make([]fuzzy.Match, len(names))
		for i, name := range names {
			cp.matches[i] = fuzzy.Match{Str: name, Index: i}
		}
	} else {
		cp.matches = fuzzy.Find(cp.query, names)
	}
	if cp.cursor >= len(cp.matches) {
		cp.cursor = 0
	}
}

func (cp *CommandPalette) Update(msg tea.Msg) tea.Cmd {
	if !cp.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+k":
			cp.Hide()
			return Cmd(HideOverlayMsg{})
		case "enter":
			if cp.cursor < len(cp.matches) {
				match := cp.matches[cp.cursor]
				commands := cp.registry.All()
				if match.Index < len(commands) {
					cmd := commands[match.Index]
					cp.Hide()
					return tea.Batch(
						Cmd(HideOverlayMsg{}),
						cmd.Action(),
					)
				}
			}
			cp.Hide()
			return Cmd(HideOverlayMsg{})
		case "up", "ctrl+p":
			if cp.cursor > 0 {
				cp.cursor--
			}
		case "down", "ctrl+n":
			if cp.cursor < len(cp.matches)-1 {
				cp.cursor++
			}
		case "backspace":
			if len(cp.query) > 0 {
				cp.query = cp.query[:len(cp.query)-1]
				cp.updateMatches()
			}
		case "ctrl+u":
			cp.query = ""
			cp.updateMatches()
		default:
			keyStr := msg.String()
			// Only accept printable characters
			if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
				cp.query += keyStr
				cp.updateMatches()
			}
		}
	}
	return nil
}

// renderFuzzyHighlight renders a string with matched character positions highlighted.
// matchedIndexes should contain rune indices (as provided by sahilm/fuzzy).
func renderFuzzyHighlight(text string, matchedIndexes []int, normalStyle, highlightStyle lipgloss.Style) string {
	if len(matchedIndexes) == 0 {
		return normalStyle.Render(text)
	}
	matchSet := make(map[int]bool, len(matchedIndexes))
	for _, idx := range matchedIndexes {
		matchSet[idx] = true
	}
	runes := []rune(text)
	var result strings.Builder
	for i, ch := range runes {
		if matchSet[i] {
			result.WriteString(highlightStyle.Render(string(ch)))
		} else {
			result.WriteString(normalStyle.Render(string(ch)))
		}
	}
	return result.String()
}

func (cp *CommandPalette) View() string {
	if !cp.visible {
		return ""
	}

	// Dialog content width (inside border + padding)
	dialogW := min(cp.width*60/100, 80)
	if dialogW < 36 {
		dialogW = 36
	}
	// Inner content width after border(2) + padding(4)
	contentW := max(dialogW-6, 20)

	// Title
	title := lipgloss.NewStyle().
		Foreground(cp.theme.Blue).
		Bold(true).
		Render("Commands")

	// Search input
	searchPrefix := lipgloss.NewStyle().
		Foreground(cp.theme.Blue).
		Bold(true).
		Render("> ")

	queryDisplay := cp.query
	if queryDisplay == "" {
		queryDisplay = lipgloss.NewStyle().
			Foreground(cp.theme.Overlay0).
			Italic(true).
			Render("Type to search...")
	} else {
		queryDisplay = lipgloss.NewStyle().
			Foreground(cp.theme.Text).
			Render(queryDisplay)
	}
	cursor := lipgloss.NewStyle().
		Background(cp.theme.Text).
		Foreground(cp.theme.Base).
		Render(" ")
	searchLine := searchPrefix + queryDisplay + cursor

	// Separator
	sep := lipgloss.NewStyle().
		Foreground(cp.theme.Surface1).
		Render(strings.Repeat("─", contentW))

	// Match list
	// Chrome lines: border(2) + padding(2) + title(1) + search(1) + sep(1) + sep(1) + count(1) = 9
	maxVisible := 10
	maxFit := max(cp.height-9, 3)
	if maxVisible > maxFit {
		maxVisible = maxFit
	}

	var matchLines []string
	commands := cp.registry.All()
	startIdx := 0
	if cp.cursor >= maxVisible {
		startIdx = cp.cursor - maxVisible + 1
	}

	for i := startIdx; i < len(cp.matches) && len(matchLines) < maxVisible; i++ {
		match := cp.matches[i]
		if match.Index >= len(commands) {
			continue
		}
		cmd := commands[match.Index]
		isSelected := i == cp.cursor

		// Build the line: name (with fuzzy highlights) + description, truncated
		nameStyle := lipgloss.NewStyle().Foreground(cp.theme.Text)
		nameHighlightStyle := lipgloss.NewStyle().Foreground(cp.theme.Yellow).Bold(true)
		descStyle := lipgloss.NewStyle().Foreground(cp.theme.Overlay1)

		name := renderFuzzyHighlight(cmd.Name, match.MatchedIndexes, nameStyle, nameHighlightStyle)
		desc := descStyle.Render(cmd.Description)

		shortcut := ""
		if cmd.Shortcut != "" {
			shortcut = lipgloss.NewStyle().
				Foreground(cp.theme.Overlay2).
				Render("[" + cmd.Shortcut + "]")
		}

		line := " " + name + "  " + desc
		if shortcut != "" {
			lineW := lipgloss.Width(line)
			scW := lipgloss.Width(shortcut)
			gap := max(contentW-lineW-scW, 1)
			line += strings.Repeat(" ", gap) + shortcut
		}

		// Pad and apply selection highlight
		padded := PadRight(line, contentW)
		if isSelected {
			padded = lipgloss.NewStyle().
				Background(cp.theme.Surface0).
				Bold(true).
				Render(padded)
		}

		matchLines = append(matchLines, padded)
	}

	if len(matchLines) == 0 {
		matchLines = append(matchLines, lipgloss.NewStyle().
			Foreground(cp.theme.Overlay0).Italic(true).
			Render("  No matching commands"))
	}

	// Result count
	countLine := lipgloss.NewStyle().
		Foreground(cp.theme.Overlay0).
		Render(fmt.Sprintf("  %d results", len(cp.matches)))

	// Build dialog content
	content := title + "\n" +
		searchLine + "\n" +
		sep + "\n" +
		strings.Join(matchLines, "\n") + "\n" +
		sep + "\n" +
		countLine

	dialog := lipgloss.NewStyle().
		Background(cp.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cp.theme.Blue).
		Padding(1, 2).
		Width(dialogW - 2). // minus border chars
		Render(content)

	return dialog
}

// ─── Quick Search ───
// QuickSearch is a fuzzy resource finder overlay triggered by // (slash-slash).

type QuickSearch struct {
	width, height int
	theme         Theme
	visible       bool
	query         string
	cursor        int
	resources     []*ResourceData
	names         []string // precomputed KindName() for fuzzy matching
	matches       []fuzzy.Match
}

func NewQuickSearch(theme Theme) *QuickSearch {
	return &QuickSearch{theme: theme}
}

func (qs *QuickSearch) SetSize(w, h int) {
	qs.width = w
	qs.height = h
}

func (qs *QuickSearch) IsVisible() bool {
	return qs.visible
}

func (qs *QuickSearch) Show(resources []*ResourceData) {
	qs.visible = true
	qs.query = ""
	qs.cursor = 0
	qs.resources = resources
	qs.names = make([]string, len(resources))
	for i, rd := range resources {
		qs.names[i] = rd.Resource.KindName()
	}
	qs.updateMatches()
}

func (qs *QuickSearch) Hide() {
	qs.visible = false
	qs.query = ""
	qs.cursor = 0
}

func (qs *QuickSearch) updateMatches() {
	if qs.query == "" {
		qs.matches = make([]fuzzy.Match, len(qs.names))
		for i, name := range qs.names {
			qs.matches[i] = fuzzy.Match{Str: name, Index: i}
		}
	} else {
		qs.matches = fuzzy.Find(qs.query, qs.names)
	}
	if qs.cursor >= len(qs.matches) {
		qs.cursor = 0
	}
}

func (qs *QuickSearch) Update(msg tea.Msg) tea.Cmd {
	if !qs.visible {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "/":
			qs.Hide()
			return Cmd(HideOverlayMsg{})
		case "enter":
			if qs.cursor < len(qs.matches) {
				match := qs.matches[qs.cursor]
				if match.Index < len(qs.resources) {
					rd := qs.resources[match.Index]
					qs.Hide()
					return tea.Batch(
						Cmd(HideOverlayMsg{}),
						Cmd(ResourceSelectedMsg{Resource: rd}),
					)
				}
			}
			qs.Hide()
			return Cmd(HideOverlayMsg{})
		case "up", "ctrl+p":
			if qs.cursor > 0 {
				qs.cursor--
			}
		case "down", "ctrl+n":
			if qs.cursor < len(qs.matches)-1 {
				qs.cursor++
			}
		case "backspace":
			if len(qs.query) > 0 {
				qs.query = qs.query[:len(qs.query)-1]
				qs.updateMatches()
			}
		case "ctrl+u":
			qs.query = ""
			qs.updateMatches()
		default:
			keyStr := msg.String()
			if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
				qs.query += keyStr
				qs.updateMatches()
			}
		}
	}
	return nil
}

func (qs *QuickSearch) View() string {
	if !qs.visible {
		return ""
	}

	dialogW := min(qs.width*60/100, 80)
	if dialogW < 36 {
		dialogW = 36
	}
	contentW := max(dialogW-6, 20)

	title := lipgloss.NewStyle().
		Foreground(qs.theme.Green).
		Bold(true).
		Render("Quick Search")

	searchPrefix := lipgloss.NewStyle().
		Foreground(qs.theme.Green).Bold(true).
		Render("// ")

	queryDisplay := qs.query
	if queryDisplay == "" {
		queryDisplay = lipgloss.NewStyle().
			Foreground(qs.theme.Overlay0).Italic(true).
			Render("Fuzzy search resources...")
	} else {
		queryDisplay = lipgloss.NewStyle().
			Foreground(qs.theme.Text).
			Render(queryDisplay)
	}
	cursor := lipgloss.NewStyle().
		Background(qs.theme.Text).Foreground(qs.theme.Base).
		Render(" ")
	searchLine := searchPrefix + queryDisplay + cursor

	sep := lipgloss.NewStyle().
		Foreground(qs.theme.Surface1).
		Render(strings.Repeat("─", contentW))

	// Chrome lines: border(2) + padding(2) + title(1) + search(1) + sep(1) + sep(1) + count(1) = 9
	maxVisible := 12
	maxFit := max(qs.height-9, 3)
	if maxVisible > maxFit {
		maxVisible = maxFit
	}

	var matchLines []string
	startIdx := 0
	if qs.cursor >= maxVisible {
		startIdx = qs.cursor - maxVisible + 1
	}

	nameStyle := lipgloss.NewStyle().Foreground(qs.theme.Text)
	nameHighlightStyle := lipgloss.NewStyle().Foreground(qs.theme.Yellow).Bold(true)

	for i := startIdx; i < len(qs.matches) && len(matchLines) < maxVisible; i++ {
		match := qs.matches[i]
		if match.Index >= len(qs.resources) {
			continue
		}
		rd := qs.resources[match.Index]
		isSelected := i == qs.cursor

		name := renderFuzzyHighlight(rd.Resource.KindName(), match.MatchedIndexes, nameStyle, nameHighlightStyle)

		// Extra info: namespace, revision count, badges
		var badges []string
		if rd.Resource.Namespace != "" {
			badges = append(badges, lipgloss.NewStyle().Foreground(qs.theme.Overlay1).
				Render(rd.Resource.Namespace))
		}
		badges = append(badges, lipgloss.NewStyle().Foreground(qs.theme.Overlay0).
			Render(fmt.Sprintf("%d revs", len(rd.Revisions))))
		if rd.Resource.Starred {
			badges = append(badges, lipgloss.NewStyle().Foreground(qs.theme.Yellow).Render("★"))
		}
		if rd.DetectLoop(6) {
			badges = append(badges, lipgloss.NewStyle().Foreground(qs.theme.Red).Render("↻loop"))
		}

		info := strings.Join(badges, " ")
		line := " " + name
		lineW := lipgloss.Width(line)
		infoW := lipgloss.Width(info)
		gap := max(contentW-lineW-infoW, 1)
		line += strings.Repeat(" ", gap) + info

		padded := PadRight(line, contentW)
		if isSelected {
			padded = lipgloss.NewStyle().
				Background(qs.theme.Surface0).Bold(true).
				Render(padded)
		}
		matchLines = append(matchLines, padded)
	}

	if len(matchLines) == 0 {
		matchLines = append(matchLines, lipgloss.NewStyle().
			Foreground(qs.theme.Overlay0).Italic(true).
			Render("  No matching resources"))
	}

	countLine := lipgloss.NewStyle().Foreground(qs.theme.Overlay0).
		Render(fmt.Sprintf("  %d / %d resources", len(qs.matches), len(qs.resources)))

	content := title + "\n" +
		searchLine + "\n" +
		sep + "\n" +
		strings.Join(matchLines, "\n") + "\n" +
		sep + "\n" +
		countLine

	dialog := lipgloss.NewStyle().
		Background(qs.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(qs.theme.Green).
		Padding(1, 2).
		Width(dialogW - 2).
		Render(content)

	return dialog
}

// ─── Watch Manager ───
// WatchManager is a two-tab overlay for managing watched resource types (kinds).
// Tab "Watching": shows currently watched resource kinds with counts.
// Tab "Add": fuzzy search available resource types from the cluster, Enter to add.

type watchManagerTab int

const (
	wmTabWatching watchManagerTab = iota
	wmTabAdd
)

// watchedKindEntry represents a kind currently being watched, with resource counts.
type watchedKindEntry struct {
	Kind          string
	ResourceCount int
	RevisionCount int
}

type WatchManager struct {
	width, height int
	theme         Theme
	visible       bool
	tab           watchManagerTab

	// Watching tab
	watched      []watchedKindEntry
	watchCursor  int
	watchFilter  string
	watchNames   []string
	watchMatches []fuzzy.Match

	// Add tab
	available  []ResourceKind
	addCursor  int
	addQuery   string
	addNames   []string
	addMatches []fuzzy.Match
}

func NewWatchManager(theme Theme) *WatchManager {
	return &WatchManager{theme: theme}
}

func (wm *WatchManager) SetSize(w, h int) {
	wm.width = w
	wm.height = h
}

func (wm *WatchManager) IsVisible() bool {
	return wm.visible
}

// Show populates the watch manager with current state.
// watchedKinds: unique kind names currently tracked.
// store: used to get resource/revision counts per kind.
// unwatchedKinds: resource types available but not yet watched.
func (wm *WatchManager) Show(store Store, unwatchedKinds []ResourceKind) {
	wm.visible = true
	wm.tab = wmTabWatching
	wm.watchCursor = 0
	wm.watchFilter = ""
	wm.addCursor = 0
	wm.addQuery = ""

	// Build watching entries
	kinds := store.WatchedKinds()
	wm.watched = make([]watchedKindEntry, len(kinds))
	wm.watchNames = make([]string, len(kinds))
	for i, kind := range kinds {
		wm.watched[i] = watchedKindEntry{
			Kind:          kind,
			ResourceCount: store.ResourceCountByKind(kind),
			RevisionCount: store.RevisionCountByKind(kind),
		}
		wm.watchNames[i] = kind
	}
	wm.updateWatchMatches()

	// Build available types
	wm.available = unwatchedKinds
	wm.addNames = make([]string, len(unwatchedKinds))
	for i, rk := range unwatchedKinds {
		wm.addNames[i] = rk.Kind
	}
	wm.updateAddMatches()
}

func (wm *WatchManager) Hide() {
	wm.visible = false
}

func (wm *WatchManager) updateWatchMatches() {
	if wm.watchFilter == "" {
		wm.watchMatches = make([]fuzzy.Match, len(wm.watchNames))
		for i, name := range wm.watchNames {
			wm.watchMatches[i] = fuzzy.Match{Str: name, Index: i}
		}
	} else {
		wm.watchMatches = fuzzy.Find(wm.watchFilter, wm.watchNames)
	}
	if wm.watchCursor >= len(wm.watchMatches) {
		wm.watchCursor = 0
	}
}

func (wm *WatchManager) updateAddMatches() {
	if wm.addQuery == "" {
		wm.addMatches = make([]fuzzy.Match, len(wm.addNames))
		for i, name := range wm.addNames {
			wm.addMatches[i] = fuzzy.Match{Str: name, Index: i}
		}
	} else {
		wm.addMatches = fuzzy.Find(wm.addQuery, wm.addNames)
	}
	if wm.addCursor >= len(wm.addMatches) {
		wm.addCursor = 0
	}
}

func (wm *WatchManager) Update(msg tea.Msg) tea.Cmd {
	if !wm.visible {
		return nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			wm.Hide()
			return Cmd(HideOverlayMsg{})
		case "tab":
			if wm.tab == wmTabWatching {
				wm.tab = wmTabAdd
			} else {
				wm.tab = wmTabWatching
			}
			return nil
		}

		if wm.tab == wmTabWatching {
			return wm.updateWatching(msg)
		}
		return wm.updateAdd(msg)
	}
	return nil
}

func (wm *WatchManager) updateWatching(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "ctrl+p", "k":
		if wm.watchCursor > 0 {
			wm.watchCursor--
		}
	case "down", "ctrl+n", "j":
		if wm.watchCursor < len(wm.watchMatches)-1 {
			wm.watchCursor++
		}
	case "d", "delete":
		if wm.watchCursor < len(wm.watchMatches) {
			match := wm.watchMatches[wm.watchCursor]
			if match.Index < len(wm.watched) {
				entry := wm.watched[match.Index]
				wm.Hide()
				return tea.Batch(
					Cmd(HideOverlayMsg{}),
					Cmd(RemoveWatchKindMsg{Kind: entry.Kind}),
				)
			}
		}
	case "enter":
		// Navigate to first resource of this kind in the explorer
		// (for now, just close — navigating by kind would require extra msg)
		wm.Hide()
		return Cmd(HideOverlayMsg{})
	case "backspace":
		if len(wm.watchFilter) > 0 {
			wm.watchFilter = wm.watchFilter[:len(wm.watchFilter)-1]
			wm.updateWatchMatches()
		}
	case "ctrl+u":
		wm.watchFilter = ""
		wm.updateWatchMatches()
	default:
		keyStr := msg.String()
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
			wm.watchFilter += keyStr
			wm.updateWatchMatches()
		}
	}
	return nil
}

func (wm *WatchManager) updateAdd(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "ctrl+p":
		if wm.addCursor > 0 {
			wm.addCursor--
		}
	case "down", "ctrl+n":
		if wm.addCursor < len(wm.addMatches)-1 {
			wm.addCursor++
		}
	case "enter":
		if wm.addCursor < len(wm.addMatches) {
			match := wm.addMatches[wm.addCursor]
			if match.Index < len(wm.available) {
				rk := wm.available[match.Index]
				wm.Hide()
				return tea.Batch(
					Cmd(HideOverlayMsg{}),
					Cmd(AddWatchKindMsg{Kind: rk}),
				)
			}
		}
	case "backspace":
		if len(wm.addQuery) > 0 {
			wm.addQuery = wm.addQuery[:len(wm.addQuery)-1]
			wm.updateAddMatches()
		}
	case "ctrl+u":
		wm.addQuery = ""
		wm.updateAddMatches()
	default:
		keyStr := msg.String()
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] < 127 {
			wm.addQuery += keyStr
			wm.updateAddMatches()
		}
	}
	return nil
}

func (wm *WatchManager) View() string {
	if !wm.visible {
		return ""
	}

	dialogW := min(wm.width*60/100, 80)
	if dialogW < 40 {
		dialogW = 40
	}
	contentW := max(dialogW-6, 24)

	// Title
	title := lipgloss.NewStyle().Foreground(wm.theme.Mauve).Bold(true).
		Render("Watch Manager")
	subtitle := lipgloss.NewStyle().Foreground(wm.theme.Overlay1).
		Render("  (manage watched resource types)")

	// Tabs
	activeTab := lipgloss.NewStyle().Foreground(wm.theme.Blue).Bold(true).Underline(true)
	inactiveTab := lipgloss.NewStyle().Foreground(wm.theme.Overlay1)
	watchingLabel := inactiveTab.Render("Watching")
	addLabel := inactiveTab.Render("Add Type")
	if wm.tab == wmTabWatching {
		watchingLabel = activeTab.Render("Watching")
	} else {
		addLabel = activeTab.Render("Add Type")
	}
	tabLine := "  " + watchingLabel + "    " + addLabel

	sep := lipgloss.NewStyle().Foreground(wm.theme.Surface1).
		Render(strings.Repeat("─", contentW))

	var body string
	if wm.tab == wmTabWatching {
		body = wm.viewWatching(contentW)
	} else {
		body = wm.viewAdd(contentW)
	}

	// Footer
	var footerText string
	if wm.tab == wmTabWatching {
		footerText = fmt.Sprintf("  %d types watched  Tab:add types  d:unwatch  Esc:close",
			len(wm.watchMatches))
	} else {
		footerText = fmt.Sprintf("  %d types available  Tab:back  Enter:add  Esc:close",
			len(wm.addMatches))
	}
	footer := lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Render(footerText)

	content := title + subtitle + "\n" +
		tabLine + "\n" +
		sep + "\n" +
		body + "\n" +
		sep + "\n" +
		footer

	dialog := lipgloss.NewStyle().
		Background(wm.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(wm.theme.Mauve).
		Padding(1, 2).
		Width(dialogW - 2).
		Render(content)

	return dialog
}

func (wm *WatchManager) viewWatching(contentW int) string {
	// Search input
	searchPrefix := lipgloss.NewStyle().Foreground(wm.theme.Mauve).Bold(true).Render("> ")
	queryDisplay := wm.watchFilter
	if queryDisplay == "" {
		queryDisplay = lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("Filter watched types...")
	} else {
		queryDisplay = lipgloss.NewStyle().Foreground(wm.theme.Text).Render(queryDisplay)
	}
	cursor := lipgloss.NewStyle().Background(wm.theme.Text).Foreground(wm.theme.Base).Render(" ")
	searchLine := searchPrefix + queryDisplay + cursor

	// Chrome lines: border(2) + padding(2) + title(1) + tabs(1) + sep(1) + search(1) + sep(1) + footer(1) = 10
	maxVisible := 12
	maxFit := max(wm.height-10, 3)
	if maxVisible > maxFit {
		maxVisible = maxFit
	}

	nameStyle := lipgloss.NewStyle().Foreground(wm.theme.Text)
	nameHighlight := lipgloss.NewStyle().Foreground(wm.theme.Yellow).Bold(true)

	var lines []string
	startIdx := 0
	if wm.watchCursor >= maxVisible {
		startIdx = wm.watchCursor - maxVisible + 1
	}

	for i := startIdx; i < len(wm.watchMatches) && len(lines) < maxVisible; i++ {
		match := wm.watchMatches[i]
		if match.Index >= len(wm.watched) {
			continue
		}
		entry := wm.watched[match.Index]
		isSelected := i == wm.watchCursor

		name := renderFuzzyHighlight(entry.Kind, match.MatchedIndexes, nameStyle, nameHighlight)

		// Show resource count and revision count
		countText := lipgloss.NewStyle().Foreground(wm.theme.Overlay1).
			Render(fmt.Sprintf("%d resources, %d revs", entry.ResourceCount, entry.RevisionCount))

		line := " " + name
		lineW := lipgloss.Width(line)
		infoW := lipgloss.Width(countText)
		gap := max(contentW-lineW-infoW, 1)
		line += strings.Repeat(" ", gap) + countText

		padded := PadRight(line, contentW)
		if isSelected {
			padded = lipgloss.NewStyle().Background(wm.theme.Surface0).Bold(true).Render(padded)
		}
		lines = append(lines, padded)
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("  No watched types"))
	}

	return searchLine + "\n" + strings.Join(lines, "\n")
}

func (wm *WatchManager) viewAdd(contentW int) string {
	// Search input
	searchPrefix := lipgloss.NewStyle().Foreground(wm.theme.Green).Bold(true).Render("> ")
	queryDisplay := wm.addQuery
	if queryDisplay == "" {
		queryDisplay = lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("Search resource types...")
	} else {
		queryDisplay = lipgloss.NewStyle().Foreground(wm.theme.Text).Render(queryDisplay)
	}
	cursor := lipgloss.NewStyle().Background(wm.theme.Text).Foreground(wm.theme.Base).Render(" ")
	searchLine := searchPrefix + queryDisplay + cursor

	// Chrome lines: border(2) + padding(2) + title(1) + tabs(1) + sep(1) + search(1) + sep(1) + footer(1) = 10
	maxVisible := 12
	maxFit := max(wm.height-10, 3)
	if maxVisible > maxFit {
		maxVisible = maxFit
	}

	nameStyle := lipgloss.NewStyle().Foreground(wm.theme.Text)
	nameHighlight := lipgloss.NewStyle().Foreground(wm.theme.Yellow).Bold(true)

	var lines []string
	startIdx := 0
	if wm.addCursor >= maxVisible {
		startIdx = wm.addCursor - maxVisible + 1
	}

	for i := startIdx; i < len(wm.addMatches) && len(lines) < maxVisible; i++ {
		match := wm.addMatches[i]
		if match.Index >= len(wm.available) {
			continue
		}
		rk := wm.available[match.Index]
		isSelected := i == wm.addCursor

		name := renderFuzzyHighlight(rk.Kind, match.MatchedIndexes, nameStyle, nameHighlight)

		// API version and scope info
		apiText := lipgloss.NewStyle().Foreground(wm.theme.Overlay1).Render(rk.APIVersion)
		scopeText := ""
		if rk.Namespaced {
			scopeText = lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Render("namespaced")
		} else {
			scopeText = lipgloss.NewStyle().Foreground(wm.theme.Peach).Render("cluster")
		}
		info := apiText + "  " + scopeText

		line := " " + name
		lineW := lipgloss.Width(line)
		infoW := lipgloss.Width(info)
		gap := max(contentW-lineW-infoW, 1)
		line += strings.Repeat(" ", gap) + info

		padded := PadRight(line, contentW)
		if isSelected {
			padded = lipgloss.NewStyle().Background(wm.theme.Surface0).Bold(true).Render(padded)
		}
		lines = append(lines, padded)
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("  No matching resource types"))
	}

	return searchLine + "\n" + strings.Join(lines, "\n")
}

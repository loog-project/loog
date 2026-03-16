package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/loog-project/loog/internal/resource"
)

// fuzzyMatchAll performs a fuzzy search on names. When the query is empty all
// names are returned as identity matches. The cursor is clamped to the result
// length.
func fuzzyMatchAll(query string, names []string, cursor int) ([]fuzzy.Match, int) {
	var matches []fuzzy.Match
	if query == "" {
		matches = make([]fuzzy.Match, len(names))
		for i, name := range names {
			matches[i] = fuzzy.Match{Str: name, Index: i}
		}
	} else {
		matches = fuzzy.Find(query, names)
	}
	if cursor >= len(matches) {
		cursor = 0
	}
	return matches, cursor
}

// renderListLine builds a single padded, optionally highlighted line with a
// left-aligned label and right-aligned info string.
func renderListLine(name, info string, width int, selected bool, bgColor lipgloss.Color) string {
	line := " " + name
	lineW := lipgloss.Width(line)
	infoW := lipgloss.Width(info)
	gap := max(width-lineW-infoW, 1)
	line += strings.Repeat(" ", gap) + info

	padded := PadRight(line, width)
	if selected {
		padded = lipgloss.NewStyle().Background(bgColor).Bold(true).Render(padded)
	}
	return padded
}

// CommandPalette is a fuzzy-searchable modal overlay.
type CommandPalette struct {
	width, height int
	theme         Theme
	registry      *CommandRegistry
	queryInput    textinput.Model
	cursor        int
	visible       bool
	matches       []fuzzy.Match
}

func NewCommandPalette(theme Theme, registry *CommandRegistry) *CommandPalette {
	return &CommandPalette{
		theme:      theme,
		registry:   registry,
		queryInput: newSearchInput("> ", "Type to search...", theme, theme.Blue),
	}
}

func (cp *CommandPalette) SetSize(w, h int) {
	cp.width = w
	cp.height = h
}

func (cp *CommandPalette) IsVisible() bool {
	return cp.visible
}

func (cp *CommandPalette) Show() tea.Cmd {
	cp.visible = true
	cp.queryInput.SetValue("")
	cmd := cp.queryInput.Focus()
	cp.cursor = 0
	cp.updateMatches()
	return cmd
}

func (cp *CommandPalette) Hide() {
	cp.visible = false
	cp.queryInput.SetValue("")
	cp.queryInput.Blur()
	cp.cursor = 0
}

func (cp *CommandPalette) updateMatches() {
	cp.matches, cp.cursor = fuzzyMatchAll(cp.queryInput.Value(), cp.registry.Names(), cp.cursor)
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
		default:
			var cmd tea.Cmd
			cp.queryInput, cmd = cp.queryInput.Update(msg)
			cp.updateMatches()
			return cmd
		}
	default:
		var cmd tea.Cmd
		cp.queryInput, cmd = cp.queryInput.Update(msg)
		return cmd
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

	// Search input (prompt, text, cursor, and placeholder handled by textinput.View)
	searchLine := cp.queryInput.View()

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

// QuickJump is a fuzzy resource finder overlay triggered by //.
type QuickJump struct {
	width, height int
	theme         Theme
	visible       bool
	queryInput    textinput.Model
	cursor        int
	resources     []*resource.Data
	names         []string // precomputed KindName() for fuzzy matching
	matches       []fuzzy.Match
}

func NewQuickJump(theme Theme) *QuickJump {
	return &QuickJump{
		theme:      theme,
		queryInput: newSearchInput("// ", "Fuzzy search resources...", theme, theme.Green),
	}
}

func (qs *QuickJump) SetSize(w, h int) {
	qs.width = w
	qs.height = h
}

func (qs *QuickJump) IsVisible() bool {
	return qs.visible
}

func (qs *QuickJump) Show(resources []*resource.Data) tea.Cmd {
	qs.visible = true
	qs.queryInput.SetValue("")
	cmd := qs.queryInput.Focus()
	qs.cursor = 0
	qs.resources = resources
	qs.names = make([]string, len(resources))
	for i, rd := range resources {
		qs.names[i] = rd.Resource.KindName()
	}
	qs.updateMatches()
	return cmd
}

func (qs *QuickJump) Hide() {
	qs.visible = false
	qs.queryInput.SetValue("")
	qs.queryInput.Blur()
	qs.cursor = 0
}

func (qs *QuickJump) updateMatches() {
	qs.matches, qs.cursor = fuzzyMatchAll(qs.queryInput.Value(), qs.names, qs.cursor)
}

func (qs *QuickJump) Update(msg tea.Msg) tea.Cmd {
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
		default:
			var cmd tea.Cmd
			qs.queryInput, cmd = qs.queryInput.Update(msg)
			qs.updateMatches()
			return cmd
		}
	default:
		var cmd tea.Cmd
		qs.queryInput, cmd = qs.queryInput.Update(msg)
		return cmd
	}
	return nil
}

func (qs *QuickJump) View() string {
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
		Render("Quick Jump")

	searchLine := qs.queryInput.View()

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
		matchLines = append(matchLines, renderListLine(name, info, contentW, isSelected, qs.theme.Surface0))
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
	watched          []watchedKindEntry
	watchCursor      int
	watchFilterInput textinput.Model
	watchNames       []string
	watchMatches     []fuzzy.Match

	// Add tab
	available     []resource.Kind
	addCursor     int
	addQueryInput textinput.Model
	addNames      []string
	addMatches    []fuzzy.Match
}

func NewWatchManager(theme Theme) *WatchManager {
	return &WatchManager{
		theme:            theme,
		watchFilterInput: newSearchInput("> ", "Filter watched types...", theme, theme.Mauve),
		addQueryInput:    newSearchInput("> ", "Search resource types...", theme, theme.Green),
	}
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
func (wm *WatchManager) Show(store Store, unwatchedKinds []resource.Kind) tea.Cmd {
	wm.visible = true
	wm.tab = wmTabWatching
	wm.watchCursor = 0
	wm.watchFilterInput.SetValue("")
	cmd := wm.watchFilterInput.Focus()
	wm.addCursor = 0
	wm.addQueryInput.SetValue("")

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
	return cmd
}

func (wm *WatchManager) Hide() {
	wm.visible = false
	wm.watchFilterInput.Blur()
	wm.addQueryInput.Blur()
}

func (wm *WatchManager) updateWatchMatches() {
	wm.watchMatches, wm.watchCursor = fuzzyMatchAll(wm.watchFilterInput.Value(), wm.watchNames, wm.watchCursor)
}

func (wm *WatchManager) updateAddMatches() {
	wm.addMatches, wm.addCursor = fuzzyMatchAll(wm.addQueryInput.Value(), wm.addNames, wm.addCursor)
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
			var cmd tea.Cmd
			if wm.tab == wmTabWatching {
				wm.tab = wmTabAdd
				wm.watchFilterInput.Blur()
				cmd = wm.addQueryInput.Focus()
			} else {
				wm.tab = wmTabWatching
				wm.addQueryInput.Blur()
				cmd = wm.watchFilterInput.Focus()
			}
			return cmd
		}

		if wm.tab == wmTabWatching {
			return wm.updateWatching(msg)
		}
		return wm.updateAdd(msg)
	default:
		if wm.tab == wmTabWatching {
			return wm.updateWatching(msg)
		}
		return wm.updateAdd(msg)
	}
}

func (wm *WatchManager) updateWatching(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			wm.Hide()
			return Cmd(HideOverlayMsg{})
		default:
			var cmd tea.Cmd
			wm.watchFilterInput, cmd = wm.watchFilterInput.Update(msg)
			wm.updateWatchMatches()
			return cmd
		}
	default:
		var cmd tea.Cmd
		wm.watchFilterInput, cmd = wm.watchFilterInput.Update(msg)
		return cmd
	}
	return nil
}

func (wm *WatchManager) updateAdd(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
		default:
			var cmd tea.Cmd
			wm.addQueryInput, cmd = wm.addQueryInput.Update(msg)
			wm.updateAddMatches()
			return cmd
		}
	default:
		var cmd tea.Cmd
		wm.addQueryInput, cmd = wm.addQueryInput.Update(msg)
		return cmd
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
	searchLine := wm.watchFilterInput.View()

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

		lines = append(lines, renderListLine(name, countText, contentW, isSelected, wm.theme.Surface0))
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("  No watched types"))
	}

	return searchLine + "\n" + strings.Join(lines, "\n")
}

func (wm *WatchManager) viewAdd(contentW int) string {
	// Search input
	searchLine := wm.addQueryInput.View()

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

		lines = append(lines, renderListLine(name, info, contentW, isSelected, wm.theme.Surface0))
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(wm.theme.Overlay0).Italic(true).
			Render("  No matching resource types"))
	}

	return searchLine + "\n" + strings.Join(lines, "\n")
}

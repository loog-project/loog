package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Debug Log Ring Buffer ───

// LogLevel categorizes debug log messages.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DBG"
	case LogInfo:
		return "INF"
	case LogWarn:
		return "WRN"
	case LogError:
		return "ERR"
	default:
		return "???"
	}
}

// LogEntry is a single debug log line.
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Source  string // component name (e.g. "app", "explorer", "revlist")
	Message string
}

// DebugLog is a thread-safe ring buffer of log entries.
type DebugLog struct {
	mu      sync.Mutex
	entries []LogEntry
	maxSize int
}

// NewDebugLog creates a ring buffer with the given capacity.
func NewDebugLog(maxSize int) *DebugLog {
	return &DebugLog{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// Log appends an entry to the ring buffer.
func (dl *DebugLog) Log(level LogLevel, source, format string, args ...any) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now(),
		Level:   level,
		Source:  source,
		Message: fmt.Sprintf(format, args...),
	}

	if len(dl.entries) >= dl.maxSize {
		// Shift out oldest
		copy(dl.entries, dl.entries[1:])
		dl.entries[len(dl.entries)-1] = entry
	} else {
		dl.entries = append(dl.entries, entry)
	}
}

// Debug logs at debug level.
func (dl *DebugLog) Debug(source, format string, args ...any) {
	dl.Log(LogDebug, source, format, args...)
}

// Info logs at info level.
func (dl *DebugLog) Info(source, format string, args ...any) {
	dl.Log(LogInfo, source, format, args...)
}

// Warn logs at warn level.
func (dl *DebugLog) Warn(source, format string, args ...any) {
	dl.Log(LogWarn, source, format, args...)
}

// Error logs at error level.
func (dl *DebugLog) Error(source, format string, args ...any) {
	dl.Log(LogError, source, format, args...)
}

// Entries returns a snapshot of all entries.
func (dl *DebugLog) Entries() []LogEntry {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	out := make([]LogEntry, len(dl.entries))
	copy(out, dl.entries)
	return out
}

// Len returns the current number of entries.
func (dl *DebugLog) Len() int {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	return len(dl.entries)
}

// Clear removes all entries.
func (dl *DebugLog) Clear() {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	dl.entries = dl.entries[:0]
}

// ─── Debug Log Viewer Overlay ───

// DebugLogViewer is a scrollable overlay that shows the debug ring buffer.
type DebugLogViewer struct {
	width, height int
	theme         Theme
	visible       bool
	log           *DebugLog
	viewport      viewport.Model
	follow        bool // auto-scroll to bottom
	levelFilter   LogLevel
	filterAll     bool // show all levels when true
}

func NewDebugLogViewer(theme Theme, log *DebugLog) *DebugLogViewer {
	vp := viewport.New(0, 0)
	return &DebugLogViewer{
		theme:       theme,
		log:         log,
		viewport:    vp,
		follow:      true,
		filterAll:   true,
		levelFilter: LogDebug,
	}
}

func (dlv *DebugLogViewer) SetSize(w, h int) {
	dlv.width = w
	dlv.height = h
	dialogW := w * 75 / 100
	if dialogW > 120 {
		dialogW = 120
	}
	if dialogW < 50 {
		dialogW = 50
	}
	dialogH := h * 70 / 100
	if dialogH < 10 {
		dialogH = 10
	}
	dlv.viewport.Width = dialogW - 6
	dlv.viewport.Height = dialogH - 8
}

func (dlv *DebugLogViewer) IsVisible() bool { return dlv.visible }
func (dlv *DebugLogViewer) Show() {
	dlv.visible = true
	dlv.refresh()
}
func (dlv *DebugLogViewer) Hide() { dlv.visible = false }

func (dlv *DebugLogViewer) refresh() {
	entries := dlv.log.Entries()
	var lines []string

	for _, e := range entries {
		if !dlv.filterAll && e.Level < dlv.levelFilter {
			continue
		}

		timeStr := e.Time.Format("15:04:05.000")
		var levelStyle lipgloss.Style
		switch e.Level {
		case LogDebug:
			levelStyle = lipgloss.NewStyle().Foreground(dlv.theme.Overlay1)
		case LogInfo:
			levelStyle = lipgloss.NewStyle().Foreground(dlv.theme.Blue)
		case LogWarn:
			levelStyle = lipgloss.NewStyle().Foreground(dlv.theme.Yellow)
		case LogError:
			levelStyle = lipgloss.NewStyle().Foreground(dlv.theme.Red).Bold(true)
		}

		timeRendered := lipgloss.NewStyle().Foreground(dlv.theme.Overlay0).Render(timeStr)
		levelRendered := levelStyle.Render(fmt.Sprintf("%-3s", e.Level.String()))
		sourceRendered := lipgloss.NewStyle().Foreground(dlv.theme.Teal).Render(fmt.Sprintf("%-10s", e.Source))
		msgRendered := lipgloss.NewStyle().Foreground(dlv.theme.Text).Render(e.Message)

		lines = append(lines, fmt.Sprintf("%s %s %s %s", timeRendered, levelRendered, sourceRendered, msgRendered))
	}

	if len(lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(dlv.theme.Overlay0).Italic(true).
			Render("  No log entries yet. Actions will appear here."))
	}

	dlv.viewport.SetContent(strings.Join(lines, "\n"))
	if dlv.follow {
		dlv.viewport.GotoBottom()
	}
}

func (dlv *DebugLogViewer) Update(msg tea.Msg) tea.Cmd {
	if !dlv.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "f6", "alt+6":
			dlv.Hide()
			return Cmd(HideOverlayMsg{})
		case "f":
			dlv.follow = !dlv.follow
			if dlv.follow {
				dlv.viewport.GotoBottom()
			}
		case "c":
			dlv.log.Clear()
			dlv.refresh()
		case "1":
			dlv.filterAll = true
			dlv.refresh()
		case "2":
			dlv.filterAll = false
			dlv.levelFilter = LogInfo
			dlv.refresh()
		case "3":
			dlv.filterAll = false
			dlv.levelFilter = LogWarn
			dlv.refresh()
		case "4":
			dlv.filterAll = false
			dlv.levelFilter = LogError
			dlv.refresh()
		default:
			var cmd tea.Cmd
			dlv.viewport, cmd = dlv.viewport.Update(msg)
			return cmd
		}
	}

	return nil
}

func (dlv *DebugLogViewer) View() string {
	if !dlv.visible {
		return ""
	}

	dlv.refresh() // Always refresh to show latest logs

	dialogW := dlv.width * 75 / 100
	if dialogW > 120 {
		dialogW = 120
	}
	if dialogW < 50 {
		dialogW = 50
	}
	innerW := dialogW - 6

	title := lipgloss.NewStyle().Foreground(dlv.theme.Lavender).Bold(true).
		Render("Debug Log")

	countStr := lipgloss.NewStyle().Foreground(dlv.theme.Overlay0).
		Render(fmt.Sprintf("  %d entries", dlv.log.Len()))

	followBadge := ""
	if dlv.follow {
		followBadge = "  " + lipgloss.NewStyle().Foreground(dlv.theme.Teal).Bold(true).Render("[FOLLOW]")
	}

	filterBadge := ""
	if !dlv.filterAll {
		filterBadge = "  " + lipgloss.NewStyle().Foreground(dlv.theme.Yellow).Render("[≥"+dlv.levelFilter.String()+"]")
	}

	header := title + countStr + followBadge + filterBadge

	sep := lipgloss.NewStyle().Foreground(dlv.theme.Surface1).
		Render(strings.Repeat("─", innerW))

	footer := lipgloss.NewStyle().Foreground(dlv.theme.Overlay0).Italic(true).
		Render("  f=follow  c=clear  1=all 2=≥info 3=≥warn 4=≥error  Esc=close")

	content := header + "\n" + sep + "\n" + dlv.viewport.View() + "\n" + sep + "\n" + footer

	dialog := lipgloss.NewStyle().
		Background(dlv.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dlv.theme.Lavender).
		Padding(1, 2).
		Width(dialogW).
		Render(content)

	return dialog
}

// ─── Developer Console ───

// DevConsole is a simple command-line console for development/debugging.
type DevConsole struct {
	width, height int
	theme         Theme
	visible       bool
	input         string
	cursor        int
	history       []string
	historyIdx    int
	output        []string // rendered output lines
	viewport      viewport.Model
	store         Store
	log           *DebugLog
	pendingCmd    tea.Cmd // command to return from Update after execute()
}

func NewDevConsole(theme Theme, store Store, log *DebugLog) *DevConsole {
	vp := viewport.New(0, 0)
	return &DevConsole{
		theme:      theme,
		store:      store,
		log:        log,
		viewport:   vp,
		historyIdx: -1,
	}
}

func (dc *DevConsole) SetSize(w, h int) {
	dc.width = w
	dc.height = h
	dialogW := w * 70 / 100
	if dialogW > 100 {
		dialogW = 100
	}
	if dialogW < 50 {
		dialogW = 50
	}
	dialogH := h * 60 / 100
	if dialogH < 10 {
		dialogH = 10
	}
	dc.viewport.Width = dialogW - 6
	dc.viewport.Height = dialogH - 8
}

func (dc *DevConsole) IsVisible() bool { return dc.visible }
func (dc *DevConsole) Show() {
	dc.visible = true
	dc.input = ""
	dc.cursor = 0
	dc.historyIdx = -1
}
func (dc *DevConsole) Hide() { dc.visible = false }

func (dc *DevConsole) execute(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}

	dc.history = append(dc.history, cmd)
	dc.historyIdx = -1

	promptLine := lipgloss.NewStyle().Foreground(dc.theme.Green).Bold(true).Render("> ") +
		lipgloss.NewStyle().Foreground(dc.theme.Text).Render(cmd)
	dc.output = append(dc.output, promptLine)

	parts := strings.Fields(cmd)
	command := parts[0]

	switch command {
	case "help":
		dc.outputLines([]string{
			"Available commands:",
			"  help                     Show this help",
			"  status                   Show app state summary",
			"  resources                List all resources with UIDs",
			"  revisions <uid>          List revisions for a resource",
			"  store                    Dump store stats",
			"  kinds                    List resource kinds and counts",
			"  starred                  List starred resources",
			"  log <message>            Write a debug log entry",
			"  clear                    Clear console output",
			"  sim start|stop           Start/stop simulation",
			"  inspect <uid> [rev-idx]  Show raw revision data (database format)",
		})

	case "status":
		dc.outputLines([]string{
			fmt.Sprintf("Resources:  %d", dc.store.TotalResourceCount()),
			fmt.Sprintf("Revisions:  %d", dc.store.TotalRevisionCount()),
			fmt.Sprintf("Timeline:   %d entries", len(dc.store.Timeline())),
			fmt.Sprintf("Starred:    %d", len(dc.store.StarredResources())),
			fmt.Sprintf("Watched:    %d kinds", len(dc.store.WatchedKinds())),
			fmt.Sprintf("Log buffer: %d entries", dc.log.Len()),
		})

	case "resources":
		dc.store.ForEachResource(func(uid string, rd *ResourceData) {
			star := " "
			if rd.Resource.Starred {
				star = "★"
			}
			dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
				fmt.Sprintf("  %s %s  %-30s  %d revs", star, uid[:12], rd.Resource.KindName(), len(rd.Revisions))))
		})

	case "revisions":
		if len(parts) < 2 {
			dc.outputError("Usage: revisions <uid-prefix>")
			break
		}
		prefix := parts[1]
		found := false
		dc.store.ForEachResource(func(uid string, rd *ResourceData) {
			if strings.HasPrefix(uid, prefix) {
				found = true
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Blue).Bold(true).Render(
					"  "+rd.Resource.KindName()))
				for i, rev := range rd.Revisions {
					dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
						fmt.Sprintf("    [%d] %s  %-8s  %s  keys=%d",
							i, rev.ID.String(), rev.EventType, rev.Time.Format("15:04:05.000"), len(rev.Object))))
				}
			}
		})
		if !found {
			dc.outputError("No resource found with UID prefix: " + prefix)
		}

	case "store":
		dc.outputLines([]string{
			fmt.Sprintf("Total resources: %d", dc.store.TotalResourceCount()),
			fmt.Sprintf("Total revisions: %d", dc.store.TotalRevisionCount()),
			fmt.Sprintf("Kind groups:     %d", len(dc.store.KindGroups())),
			fmt.Sprintf("Timeline events: %d", len(dc.store.Timeline())),
			fmt.Sprintf("Watched kinds:   %v", dc.store.WatchedKinds()),
		})

	case "kinds":
		kindCounts := make(map[string]int)
		dc.store.ForEachResource(func(_ string, rd *ResourceData) {
			kindCounts[rd.Resource.Kind]++
		})
		for kind, count := range kindCounts {
			dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
				fmt.Sprintf("  %-20s %d", kind, count)))
		}

	case "starred":
		for _, rd := range dc.store.StarredResources() {
			dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Yellow).Render(
				"  ★ "+rd.Resource.KindName()))
		}

	case "log":
		if len(parts) < 2 {
			dc.outputError("Usage: log <message>")
			break
		}
		msg := strings.Join(parts[1:], " ")
		dc.log.Info("console", "%s", msg)
		dc.outputSuccess("Logged: " + msg)

	case "clear":
		dc.output = nil

	case "sim":
		if len(parts) < 2 {
			dc.outputError("Usage: sim start|stop")
			break
		}
		switch parts[1] {
		case "start":
			dc.outputSuccess("Simulation started")
			dc.pendingCmd = Cmd(TogglePauseMsg{})
		case "stop":
			dc.outputSuccess("Simulation stopped")
			dc.pendingCmd = Cmd(TogglePauseMsg{})
		default:
			dc.outputError("Unknown sim subcommand: " + parts[1])
		}

	case "inspect":
		if len(parts) < 2 {
			dc.outputError("Usage: inspect <uid-prefix> [revision-index]")
			break
		}
		prefix := parts[1]
		revIdx := -1
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &revIdx)
		}
		found := false
		dc.store.ForEachResource(func(uid string, rd *ResourceData) {
			if strings.HasPrefix(uid, prefix) {
				found = true
				idx := revIdx
				if idx < 0 {
					idx = len(rd.Revisions) - 1
				}
				if idx >= len(rd.Revisions) {
					dc.outputError(fmt.Sprintf("Revision index %d out of range (0-%d)", idx, len(rd.Revisions)-1))
					return
				}
				rev := rd.Revisions[idx]
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Blue).Bold(true).Render(
					fmt.Sprintf("  %s  rev[%d] = %s", rd.Resource.KindName(), idx, rev.ID.String())))
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
					fmt.Sprintf("  UID:       %s", uid)))
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
					fmt.Sprintf("  Event:     %s", rev.EventType)))
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
					fmt.Sprintf("  Time:      %s", rev.Time.Format(time.RFC3339Nano))))
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
					fmt.Sprintf("  Object:    %d top-level keys", len(rev.Object))))
				if rev.Patch != nil {
					dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
						fmt.Sprintf("  Patch:     %d top-level keys", len(rev.Patch))))
				} else {
					dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay0).Render(
						"  Patch:     nil"))
				}
				// Dump raw object keys
				dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Teal).Render(
					"  Object keys:"))
				for k, v := range rev.Object {
					valStr := fmt.Sprintf("%T", v)
					if s, ok := v.(string); ok && len(s) < 60 {
						valStr = fmt.Sprintf("%q", s)
					} else if m, ok := v.(map[string]any); ok {
						valStr = fmt.Sprintf("map[%d keys]", len(m))
					} else if sl, ok := v.([]any); ok {
						valStr = fmt.Sprintf("list[%d items]", len(sl))
					}
					dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render(
						fmt.Sprintf("    %-20s %s", k+":", valStr)))
				}
			}
		})
		if !found {
			dc.outputError("No resource found with UID prefix: " + prefix)
		}

	default:
		dc.outputError("Unknown command: " + command + "  (type 'help' for available commands)")
	}

	// Update viewport
	dc.viewport.SetContent(strings.Join(dc.output, "\n"))
	dc.viewport.GotoBottom()
}

func (dc *DevConsole) outputLines(lines []string) {
	for _, l := range lines {
		dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Overlay1).Render("  "+l))
	}
}

func (dc *DevConsole) outputError(msg string) {
	dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Red).Render("  error: "+msg))
}

func (dc *DevConsole) outputSuccess(msg string) {
	dc.output = append(dc.output, lipgloss.NewStyle().Foreground(dc.theme.Green).Render("  "+msg))
}

func (dc *DevConsole) Update(msg tea.Msg) tea.Cmd {
	if !dc.visible {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			dc.Hide()
			return Cmd(HideOverlayMsg{})
		case "enter":
			dc.execute(dc.input)
			dc.input = ""
			dc.cursor = 0
			// Return any pending command from the executed console command
			if dc.pendingCmd != nil {
				cmd := dc.pendingCmd
				dc.pendingCmd = nil
				return cmd
			}
		case "backspace":
			if dc.cursor > 0 {
				runes := []rune(dc.input)
				dc.input = string(runes[:dc.cursor-1]) + string(runes[dc.cursor:])
				dc.cursor--
			}
		case "ctrl+u":
			dc.input = ""
			dc.cursor = 0
		case "up":
			if len(dc.history) > 0 {
				if dc.historyIdx < 0 {
					dc.historyIdx = len(dc.history) - 1
				} else if dc.historyIdx > 0 {
					dc.historyIdx--
				}
				dc.input = dc.history[dc.historyIdx]
				dc.cursor = len([]rune(dc.input))
			}
		case "down":
			if dc.historyIdx >= 0 {
				dc.historyIdx++
				if dc.historyIdx >= len(dc.history) {
					dc.historyIdx = -1
					dc.input = ""
				} else {
					dc.input = dc.history[dc.historyIdx]
				}
				dc.cursor = len([]rune(dc.input))
			}
		case "ctrl+a", "home":
			dc.cursor = 0
		case "ctrl+e", "end":
			dc.cursor = len([]rune(dc.input))
		case "left":
			if dc.cursor > 0 {
				dc.cursor--
			}
		case "right":
			if dc.cursor < len([]rune(dc.input)) {
				dc.cursor++
			}
		default:
			// Only insert single printable characters (not control sequences like "tab", "space", "alt+x")
			runes := []rune(msg.String())
			if len(runes) == 1 && msg.String() == string(runes[0]) {
				inputRunes := []rune(dc.input)
				dc.input = string(inputRunes[:dc.cursor]) + msg.String() + string(inputRunes[dc.cursor:])
				dc.cursor += 1
			}
		}
	}

	return nil
}

func (dc *DevConsole) View() string {
	if !dc.visible {
		return ""
	}

	dialogW := dc.width * 70 / 100
	if dialogW > 100 {
		dialogW = 100
	}
	if dialogW < 50 {
		dialogW = 50
	}
	innerW := dialogW - 6

	title := lipgloss.NewStyle().Foreground(dc.theme.Green).Bold(true).
		Render("Developer Console")
	subtitle := lipgloss.NewStyle().Foreground(dc.theme.Overlay0).
		Render("  type 'help' for commands")

	sep := lipgloss.NewStyle().Foreground(dc.theme.Surface1).
		Render(strings.Repeat("─", innerW))

	// Input line with accurate cursor position
	prompt := lipgloss.NewStyle().Foreground(dc.theme.Green).Bold(true).Render("> ")
	textStyle := lipgloss.NewStyle().Foreground(dc.theme.Text)
	cursorStyle := lipgloss.NewStyle().Background(dc.theme.Text).Foreground(dc.theme.Base)
	var inputRendered string
	if dc.input == "" {
		placeholder := lipgloss.NewStyle().Foreground(dc.theme.Overlay0).Italic(true).Render("enter command...")
		inputRendered = prompt + placeholder + cursorStyle.Render(" ")
	} else {
		runes := []rune(dc.input)
		before := string(runes[:dc.cursor])
		if dc.cursor < len(runes) {
			atCursor := string(runes[dc.cursor : dc.cursor+1])
			after := string(runes[dc.cursor+1:])
			inputRendered = prompt + textStyle.Render(before) + cursorStyle.Render(atCursor) + textStyle.Render(after)
		} else {
			inputRendered = prompt + textStyle.Render(before) + cursorStyle.Render(" ")
		}
	}

	content := title + subtitle + "\n" + sep + "\n" + dc.viewport.View() + "\n" + sep + "\n" + inputRendered

	dialog := lipgloss.NewStyle().
		Background(dc.theme.Mantle).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dc.theme.Green).
		Padding(1, 2).
		Width(dialogW).
		Render(content)

	return dialog
}

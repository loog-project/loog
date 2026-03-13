package tui

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys defines keybindings that work from any context.
type GlobalKeys struct {
	Quit           key.Binding
	CommandPalette key.Binding
	Help           key.Binding
	Filter         key.Binding
	FocusLeft      key.Binding
	FocusMiddle    key.Binding
	FocusRight     key.Binding
	NextPanel      key.Binding
	PrevPanel      key.Binding
	ViewExplorer   key.Binding
	ViewTimeline   key.Binding
	ViewWatchlist  key.Binding
	ViewCompare    key.Binding
	Fullscreen     key.Binding
	AutoScroll     key.Binding
	WindowMode     key.Binding
	PauseRecording key.Binding
	FreezeView     key.Binding
	WatchManager   key.Binding
}

var GlobalKeyMap = GlobalKeys{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	CommandPalette: key.NewBinding(
		key.WithKeys("ctrl+k"),
		key.WithHelp("ctrl+k", "commands"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	FocusLeft: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "panel 1"),
	),
	FocusMiddle: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "panel 2"),
	),
	FocusRight: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3", "panel 3"),
	),
	NextPanel: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next panel"),
	),
	PrevPanel: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev panel"),
	),
	ViewExplorer: key.NewBinding(
		key.WithKeys("f1", "alt+1"),
		key.WithHelp("F1", "explorer"),
	),
	ViewTimeline: key.NewBinding(
		key.WithKeys("f2", "alt+2"),
		key.WithHelp("F2", "timeline"),
	),
	ViewWatchlist: key.NewBinding(
		key.WithKeys("f3", "alt+3"),
		key.WithHelp("F3", "watchlist"),
	),
	ViewCompare: key.NewBinding(
		key.WithKeys("f4", "alt+4"),
		key.WithHelp("F4", "compare"),
	),
	Fullscreen: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "fullscreen"),
	),
	AutoScroll: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "auto-scroll"),
	),
	WindowMode: key.NewBinding(
		key.WithKeys("w"),
		key.WithHelp("w", "window mode"),
	),
	PauseRecording: key.NewBinding(
		key.WithKeys("P"),
		key.WithHelp("P", "pause recording"),
	),
	FreezeView: key.NewBinding(
		key.WithKeys("f5", "alt+5"),
		key.WithHelp("F5", "freeze view"),
	),
	WatchManager: key.NewBinding(
		key.WithKeys("W"),
		key.WithHelp("W", "watch manager"),
	),
}

// ListKeys defines keybindings for navigable lists.
type ListKeys struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Select   key.Binding
	Star     key.Binding
	Expand   key.Binding
}

var ListKeyMap = ListKeys{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/dn", "down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("ctrl+u", "pgup"),
		key.WithHelp("ctrl+u", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("ctrl+d", "pgdown"),
		key.WithHelp("ctrl+d", "page dn"),
	),
	Home: key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g", "top"),
	),
	End: key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G", "bottom"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Star: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "star"),
	),
	Expand: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter", "expand"),
	),
}

// DetailKeys defines keybindings for the detail/diff view.
type DetailKeys struct {
	ScrollUp     key.Binding
	ScrollDown   key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	ModeDiff     key.Binding
	ModeObject   key.Binding
	ModePatch    key.Binding
	ModeJSON     key.Binding
	Export       key.Binding
	Copy         key.Binding
	CompareWith  key.Binding
	JumpTimeline key.Binding
	PrevRevision key.Binding
	NextRevision key.Binding
}

var DetailKeyMap = DetailKeys{
	ScrollUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/dn", "scroll down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("ctrl+u", "pgup"),
		key.WithHelp("ctrl+u", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("ctrl+d", "pgdown"),
		key.WithHelp("ctrl+d", "page dn"),
	),
	ModeDiff: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "diff"),
	),
	ModeObject: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "object"),
	),
	ModePatch: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "patch"),
	),
	ModeJSON: key.NewBinding(
		key.WithKeys("J"),
		key.WithHelp("J", "json"),
	),
	Export: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "export"),
	),
	Copy: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "copy"),
	),
	CompareWith: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "compare"),
	),
	JumpTimeline: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "timeline"),
	),
	PrevRevision: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "prev rev"),
	),
	NextRevision: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "next rev"),
	),
}

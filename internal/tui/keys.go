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
	DebugLog       key.Binding
	DevConsole     key.Binding
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
	DebugLog: key.NewBinding(
		key.WithKeys("f6", "alt+6"),
		key.WithHelp("F6", "debug log"),
	),
	DevConsole: key.NewBinding(
		key.WithKeys(":"),
		key.WithHelp(":", "dev console"),
	),
}

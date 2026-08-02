package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Switching to a view must focus its primary (left) pane, so list keys work
// immediately. Regression: focus state persisted across view switches, so
// returning to the timeline with the detail pane previously focused made R
// and navigation silently no-op.
func TestSwitchView_FocusesLeftPane(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	m, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = m.(*App)

	pressR := func() {
		m, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
		app = m.(*App)
	}

	app.switchView(TimelineView)
	if !app.timeline.timeline.focused {
		t.Fatal("timeline list should be focused after switching to the timeline view")
	}

	// R works from the resources (left) pane.
	pressR()
	if !app.timeline.timeline.Reversed() {
		t.Fatal("R should toggle reverse when the timeline pane is focused")
	}

	// Tab to the detail pane, then leave and return to the timeline view.
	m, _ = app.Update(tea.KeyMsg{Type: tea.KeyTab})
	app = m.(*App)
	if app.timeline.timeline.focused {
		t.Fatal("timeline list should not be focused after tabbing to detail")
	}
	app.switchView(ExplorerView)
	app.switchView(TimelineView)

	// Focus must be back on the list pane so R works again without a manual Tab.
	if !app.timeline.timeline.focused {
		t.Fatal("returning to the timeline view should re-focus the list pane")
	}
	wasReversed := app.timeline.timeline.Reversed()
	pressR()
	if app.timeline.timeline.Reversed() == wasReversed {
		t.Fatal("R should toggle reverse after returning to the timeline view")
	}
}

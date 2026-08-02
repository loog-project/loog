package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
)

func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// TestWatchManager_UnwatchStaysOpen verifies that unwatching a type emits a
// RemoveWatchKindMsg and keeps the manager open for further edits.
func TestWatchManager_UnwatchStaysOpen(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	wm.Show(newDataStore(), nil) // watched: Deployment, Service

	msg := runCmd(wm.updateWatching(tea.KeyMsg{Type: tea.KeyEnter}))
	rm, ok := msg.(RemoveWatchKindMsg)
	if !ok {
		t.Fatalf("expected RemoveWatchKindMsg, got %T", msg)
	}
	if rm.Kind == "" {
		t.Error("RemoveWatchKindMsg has empty kind")
	}
	if !wm.IsVisible() {
		t.Error("watch manager should stay open after unwatch")
	}
}

// TestWatchManager_AddStaysOpen verifies that adding a type emits an
// AddWatchKindMsg and keeps the manager open.
func TestWatchManager_AddStaysOpen(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	unwatched := []resource.Kind{
		{Kind: "ConfigMap", APIVersion: "v1", Resource: "configmaps", Namespaced: true},
	}
	wm.Show(newDataStore(), unwatched)
	// Move to the Add tab.
	wm.Update(tea.KeyMsg{Type: tea.KeyTab})

	msg := runCmd(wm.updateAdd(tea.KeyMsg{Type: tea.KeyEnter}))
	add, ok := msg.(AddWatchKindMsg)
	if !ok {
		t.Fatalf("expected AddWatchKindMsg, got %T", msg)
	}
	if add.Kind.Kind != "ConfigMap" {
		t.Errorf("added kind = %q, want ConfigMap", add.Kind.Kind)
	}
	if !wm.IsVisible() {
		t.Error("watch manager should stay open after add")
	}
}

// TestWatchManager_FilterAcceptsLetterKeys verifies that typing letters that
// used to be action shortcuts (d, j, k) filters instead of triggering actions.
func TestWatchManager_FilterAcceptsLetterKeys(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	wm.Show(newDataStore(), nil)

	for _, r := range []rune{'d', 'j', 'k'} {
		msg := runCmd(wm.updateWatching(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}))
		if _, ok := msg.(RemoveWatchKindMsg); ok {
			t.Fatalf("typing %q triggered unwatch instead of filtering", string(r))
		}
	}
	if got := wm.watchFilterInput.Value(); got != "djk" {
		t.Errorf("filter value = %q, want %q", got, "djk")
	}
	if !wm.IsVisible() {
		t.Error("watch manager should stay open while filtering")
	}
}

// TestWatchManager_RefreshPreservesTabAndFilter verifies Refresh repopulates
// lists without resetting the active tab or filter text.
func TestWatchManager_RefreshPreservesTabAndFilter(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	wm.Show(newDataStore(), nil)
	wm.Update(tea.KeyMsg{Type: tea.KeyTab}) // Add tab
	wm.addQueryInput.SetValue("cm")

	wm.Refresh(newDataStore(), []resource.Kind{
		{Kind: "ConfigMap", APIVersion: "v1", Resource: "configmaps", Namespaced: true},
	})

	if wm.tab != wmTabAdd {
		t.Error("Refresh should preserve the active tab")
	}
	if wm.addQueryInput.Value() != "cm" {
		t.Error("Refresh should preserve the filter text")
	}
	if len(wm.addNames) != 1 || wm.addNames[0] != "ConfigMap" {
		t.Errorf("Refresh did not update available kinds: %v", wm.addNames)
	}
}

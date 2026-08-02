package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/loog-project/loog/internal/resource"
)

func init() {
	lipgloss.SetColorProfile(termenv.TrueColor)
}

// ---------------------------------------------------------------------------
// Helper: build sample resource data for tests
// ---------------------------------------------------------------------------

func sampleResource(kind, name, ns, uid string, revCount int) *resource.Data {
	rd := &resource.Data{
		Resource: resource.Resource{
			Kind:      kind,
			Name:      name,
			Namespace: ns,
			UID:       uid,
		},
	}
	for i := range revCount {
		rd.Revisions = append(rd.Revisions, resource.Revision{
			ID:        resource.RevisionID(i + 1),
			EventType: resource.EventModified,
			Time:      time.Now().Add(-time.Duration(revCount-i) * time.Minute),
			Object:    map[string]any{"kind": kind, "metadata": map[string]any{"name": name}},
		})
	}
	return rd
}

func sampleKindGroups() []*resource.KindGroup {
	return []*resource.KindGroup{
		{
			Kind: "Deployment",
			Resources: []*resource.Data{
				sampleResource("Deployment", "nginx", "default", "uid-1", 3),
				sampleResource("Deployment", "redis", "default", "uid-2", 2),
			},
		},
		{
			Kind: "Service",
			Resources: []*resource.Data{
				sampleResource("Service", "web", "default", "uid-3", 1),
			},
		},
	}
}

func sampleTimeline() []resource.TimelineEntry {
	groups := sampleKindGroups()
	var entries []resource.TimelineEntry
	for _, g := range groups {
		for _, rd := range g.Resources {
			for _, rev := range rd.Revisions {
				entries = append(entries, resource.TimelineEntry{
					Resource: rd.Resource,
					Revision: rev,
				})
			}
		}
	}
	return entries
}

// ---------------------------------------------------------------------------
// dataStore: a test Store implementation with real data
// ---------------------------------------------------------------------------

type dataStore struct {
	resources  map[string]*resource.Data
	kindGroups []*resource.KindGroup
	timeline   []resource.TimelineEntry
}

func newDataStore() *dataStore {
	groups := sampleKindGroups()
	resources := map[string]*resource.Data{}
	for _, g := range groups {
		for _, rd := range g.Resources {
			resources[rd.Resource.UID] = rd
		}
	}
	return &dataStore{
		resources:  resources,
		kindGroups: groups,
		timeline:   sampleTimeline(),
	}
}

func (s *dataStore) AllResources() []*resource.Data {
	out := make([]*resource.Data, 0, len(s.resources))
	for _, rd := range s.resources {
		out = append(out, rd)
	}
	return out
}
func (s *dataStore) StarredResources() []*resource.Data                   { return nil }
func (s *dataStore) GetResource(uid string) *resource.Data                { return s.resources[uid] }
func (s *dataStore) TotalResourceCount() int                              { return len(s.resources) }
func (s *dataStore) TotalRevisionCount() int                              { return len(s.timeline) }
func (s *dataStore) FilterResources(string) []*resource.Data              { return s.AllResources() }
func (s *dataStore) FilterTimeline(string, bool) []resource.TimelineEntry { return s.timeline }
func (s *dataStore) Timeline() []resource.TimelineEntry                   { return s.timeline }
func (s *dataStore) KindGroups() []*resource.KindGroup                    { return s.kindGroups }
func (s *dataStore) WatchedKinds() []string                               { return []string{"Deployment", "Service"} }
func (s *dataStore) ResourceCountByKind(k string) int {
	for _, g := range s.kindGroups {
		if g.Kind == k {
			return len(g.Resources)
		}
	}
	return 0
}
func (s *dataStore) RevisionCountByKind(string) int              { return 0 }
func (s *dataStore) UnwatchedKinds() []resource.Kind             { return nil }
func (s *dataStore) AddWatchKind(resource.Kind) []*resource.Data { return nil }
func (s *dataStore) RemoveWatchKind(string)                      {}
func (s *dataStore) ToggleStar(string) bool                      { return false }
func (s *dataStore) AddRevision(string, resource.Revision)       {}
func (s *dataStore) RebuildKindGroups()                          {}
func (s *dataStore) ForEachResource(fn func(string, *resource.Data)) {
	for uid, rd := range s.resources {
		fn(uid, rd)
	}
}

// ---------------------------------------------------------------------------
// Header tests
// ---------------------------------------------------------------------------

func TestHeader_Dimensions(t *testing.T) {
	for _, w := range []int{40, 80, 120, 200} {
		h := NewHeader(CatppuccinMocha)
		h.SetSize(w)
		h.SetView(ExplorerView)
		out := h.View()
		_, outH := rendered(out)
		if outH != 1 {
			t.Errorf("Header width=%d: height=%d, want 1", w, outH)
		}
		outW := lipgloss.Width(out)
		if outW > w {
			t.Errorf("Header width=%d: visual width=%d, exceeds max", w, outW)
		}
	}
}

func TestHeader_ZeroWidth(t *testing.T) {
	h := NewHeader(CatppuccinMocha)
	h.SetSize(0)
	if h.View() != "" {
		t.Error("Header should return empty for width=0")
	}
}

func TestHeader_AllViewTabs(t *testing.T) {
	for _, v := range AllViews() {
		h := NewHeader(CatppuccinMocha)
		h.SetSize(80)
		h.SetView(v)
		out := h.View()
		if out == "" {
			t.Errorf("Header should render for view %s", v)
		}
	}
}

func TestHeader_FrozenAndRecording(t *testing.T) {
	h := NewHeader(CatppuccinMocha)
	h.SetSize(120)
	h.SetView(ExplorerView)
	h.SetFrozen(true)
	h.SetRecording(true)
	out := h.View()
	if lipgloss.Width(out) > 120 {
		t.Error("Header overflows with frozen+recording indicators")
	}
}

// ---------------------------------------------------------------------------
// StatusBar tests
// ---------------------------------------------------------------------------

func TestStatusBar_Dimensions(t *testing.T) {
	for _, w := range []int{80, 120, 200} {
		sb := NewStatusBar(CatppuccinMocha)
		sb.SetSize(w)
		sb.SetCounts(10, 50, 2)
		sb.SetResourceInfo("Deployment/nginx")
		sb.SetRevisionInfo("5 revisions")
		sb.SetHint("ctrl+k cmds  ? help")
		out := sb.View()
		_, outH := rendered(out)
		if outH != 2 {
			t.Errorf("StatusBar width=%d: height=%d, want 2", w, outH)
		}
		lines := strings.Split(out, "\n")
		for i, line := range lines {
			lw := lipgloss.Width(line)
			if lw > w {
				t.Errorf("StatusBar width=%d line %d: visual width=%d, exceeds max", w, i, lw)
			}
		}
	}
}

func TestStatusBar_ZeroWidth(t *testing.T) {
	sb := NewStatusBar(CatppuccinMocha)
	sb.SetSize(0)
	if sb.View() != "" {
		t.Error("StatusBar should return empty for width=0")
	}
}

func TestStatusBar_LongResourceInfo(t *testing.T) {
	sb := NewStatusBar(CatppuccinMocha)
	sb.SetSize(80)
	sb.SetResourceInfo(strings.Repeat("VeryLongResourceName", 10))
	sb.SetRevisionInfo("100 revisions")
	sb.SetHint("lots of hints here")
	out := sb.View()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if lipgloss.Width(line) > 80 {
			t.Errorf("StatusBar line %d overflows with long resource info", i)
		}
	}
}

func TestStatusBar_ErrorStatus(t *testing.T) {
	sb := NewStatusBar(CatppuccinMocha)
	sb.SetSize(80)
	sb.SetStatus("Something went wrong", true)
	out := sb.View()
	if out == "" {
		t.Error("StatusBar should render error status")
	}
}

// ---------------------------------------------------------------------------
// ExplorerViewComponent tests
// ---------------------------------------------------------------------------

func TestExplorerView_Dimensions(t *testing.T) {
	// The 3-column explorer requires at minimum ~80 columns to render properly.
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		w, h := size[0], size[1]
		ev := NewExplorerViewComponent(CatppuccinMocha)
		ev.SetSize(w, h)
		ev.SetGroups(sampleKindGroups())
		out := ev.View()
		assertDimensions(t, "explorer", out, w, h)
	}
}

func TestExplorerView_EmptyData(t *testing.T) {
	ev := NewExplorerViewComponent(CatppuccinMocha)
	ev.SetSize(80, 24)
	ev.SetGroups(nil)
	out := ev.View()
	assertDimensions(t, "explorer empty", out, 80, 24)
}

func TestExplorerView_PanelCycling(t *testing.T) {
	ev := NewExplorerViewComponent(CatppuccinMocha)
	ev.SetSize(120, 40)
	ev.SetGroups(sampleKindGroups())

	ev.SetFocusPanel(PanelLeft)
	out1 := ev.View()
	ev.NextPanel() // -> Middle
	out2 := ev.View()
	ev.NextPanel() // -> Right
	out3 := ev.View()

	if out1 == out2 || out2 == out3 {
		t.Error("different panel focus should produce different output (border highlight changes)")
	}
}

func TestExplorerView_Fullscreen(t *testing.T) {
	ev := NewExplorerViewComponent(CatppuccinMocha)
	ev.SetSize(120, 40)
	ev.SetGroups(sampleKindGroups())

	for _, panel := range []PanelID{PanelLeft, PanelMiddle, PanelRight} {
		ev.SetFocusPanel(panel)
		out := ev.ViewFullscreen(120, 40)
		assertDimensions(t, "explorer fullscreen", out, 120, 40)
	}
}

// ---------------------------------------------------------------------------
// TimelineViewComponent tests
// ---------------------------------------------------------------------------

func TestTimelineView_Dimensions(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		w, h := size[0], size[1]
		tv := NewTimelineViewComponent(CatppuccinMocha)
		tv.SetSize(w, h)
		tv.SetEntries(sampleTimeline())
		out := tv.View()
		assertDimensions(t, "timeline", out, w, h)
	}
}

func TestTimelineView_EmptyEntries(t *testing.T) {
	tv := NewTimelineViewComponent(CatppuccinMocha)
	tv.SetSize(80, 24)
	tv.SetEntries(nil)
	out := tv.View()
	assertDimensions(t, "timeline empty", out, 80, 24)
}

func TestTimelineView_Fullscreen(t *testing.T) {
	tv := NewTimelineViewComponent(CatppuccinMocha)
	tv.SetSize(120, 40)
	tv.SetEntries(sampleTimeline())

	for _, panel := range []PanelID{PanelLeft, PanelRight} {
		tv.SetFocusPanel(panel)
		out := tv.ViewFullscreen(120, 40)
		assertDimensions(t, "timeline fullscreen", out, 120, 40)
	}
}

// ---------------------------------------------------------------------------
// CompareViewComponent tests
// ---------------------------------------------------------------------------

func TestCompareView_Dimensions(t *testing.T) {
	cv := NewCompareViewComponent(CatppuccinMocha)
	cv.SetSize(120, 40)
	out := cv.View()
	assertDimensions(t, "compare empty", out, 120, 40)
}

func TestCompareView_WithItems(t *testing.T) {
	cv := NewCompareViewComponent(CatppuccinMocha)
	cv.SetSize(120, 40)
	rd := sampleResource("Deployment", "nginx", "default", "uid-1", 3)
	cv.AddItem(resource.CompareItem{Resource: rd.Resource, Revision: rd.Revisions[0]})
	cv.AddItem(resource.CompareItem{Resource: rd.Resource, Revision: rd.Revisions[2]})
	out := cv.View()
	assertDimensions(t, "compare with items", out, 120, 40)
}

func TestCompareView_Fullscreen(t *testing.T) {
	cv := NewCompareViewComponent(CatppuccinMocha)
	cv.SetSize(120, 40)
	out := cv.ViewFullscreen(120, 40)
	assertDimensions(t, "compare fullscreen", out, 120, 40)
}

// ---------------------------------------------------------------------------
// CommandPalette tests
// ---------------------------------------------------------------------------

func TestCommandPalette_HiddenReturnsEmpty(t *testing.T) {
	cp := NewCommandPalette(CatppuccinMocha, NewCommandRegistry())
	cp.SetSize(120, 40)
	if cp.View() != "" {
		t.Error("hidden command palette should return empty")
	}
}

func TestCommandPalette_VisibleDimensions(t *testing.T) {
	cp := NewCommandPalette(CatppuccinMocha, NewCommandRegistry())
	cp.SetSize(120, 40)
	cp.Show()
	out := cp.View()
	w, h := rendered(out)
	if w > 120 || h > 40 {
		t.Errorf("command palette %dx%d exceeds viewport 120x40", w, h)
	}
	if w == 0 || h == 0 {
		t.Error("visible command palette should have non-zero dimensions")
	}
}

func TestCommandPalette_EscHides(t *testing.T) {
	cp := NewCommandPalette(CatppuccinMocha, NewCommandRegistry())
	cp.SetSize(120, 40)
	cp.Show()
	cp.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cp.IsVisible() {
		t.Error("esc should hide command palette")
	}
}

func TestCommandPalette_Navigation(t *testing.T) {
	cp := NewCommandPalette(CatppuccinMocha, NewCommandRegistry())
	cp.SetSize(120, 40)
	cp.Show()

	// Navigate down
	cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cp.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", cp.cursor)
	}

	// Navigate up
	cp.Update(tea.KeyMsg{Type: tea.KeyUp})
	if cp.cursor != 0 {
		t.Errorf("cursor after up = %d, want 0", cp.cursor)
	}
}

func TestCommandPalette_SmallViewport(t *testing.T) {
	cp := NewCommandPalette(CatppuccinMocha, NewCommandRegistry())
	cp.SetSize(120, 30) // realistic viewport
	cp.Show()
	out := cp.View()
	w, h := rendered(out)
	if w > 120 || h > 30 {
		t.Errorf("command palette %dx%d exceeds viewport 120x30", w, h)
	}
}

// ---------------------------------------------------------------------------
// QuickJump tests
// ---------------------------------------------------------------------------

func TestQuickJump_HiddenReturnsEmpty(t *testing.T) {
	qj := NewQuickJump(CatppuccinMocha)
	qj.SetSize(120, 40)
	if qj.View() != "" {
		t.Error("hidden quick jump should return empty")
	}
}

func TestQuickJump_VisibleWithResources(t *testing.T) {
	qj := NewQuickJump(CatppuccinMocha)
	qj.SetSize(120, 40)
	resources := newDataStore().AllResources()
	qj.Show(resources)
	out := qj.View()
	w, h := rendered(out)
	if w > 120 || h > 40 {
		t.Errorf("quick jump %dx%d exceeds viewport", w, h)
	}
}

func TestQuickJump_EmptyResources(t *testing.T) {
	qj := NewQuickJump(CatppuccinMocha)
	qj.SetSize(80, 30)
	qj.Show(nil)
	out := qj.View()
	if out == "" {
		t.Error("quick jump should render even with no resources")
	}
}

// ---------------------------------------------------------------------------
// WatchManager tests
// ---------------------------------------------------------------------------

func TestWatchManager_HiddenReturnsEmpty(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	if wm.View() != "" {
		t.Error("hidden watch manager should return empty")
	}
}

func TestWatchManager_VisibleDimensions(t *testing.T) {
	wm := NewWatchManager(CatppuccinMocha)
	wm.SetSize(120, 40)
	store := newDataStore()
	wm.Show(store, nil)
	out := wm.View()
	w, h := rendered(out)
	if w > 120 || h > 40 {
		t.Errorf("watch manager %dx%d exceeds viewport", w, h)
	}
}

// ---------------------------------------------------------------------------
// Full App integration tests
// ---------------------------------------------------------------------------

func TestApp_InitAndRender(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*App)
	out := app.View()
	assertDimensions(t, "app initial", out, 120, 40)
}

func TestApp_AllViewsRender(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*App)

	for _, v := range AllViews() {
		app.switchView(v)
		out := app.View()
		assertDimensions(t, "app "+v.String(), out, 120, 40)
	}
}

func TestApp_SmallTerminal(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	// 80x20 is the practical minimum for the 3-column explorer layout
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	app = model.(*App)
	out := app.View()
	assertDimensions(t, "app small", out, 80, 20)
}

func TestApp_LargeTerminal(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 300, Height: 80})
	app = model.(*App)
	out := app.View()
	assertDimensions(t, "app large", out, 300, 80)
}

func TestApp_ResizePreservesView(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*App)

	app.switchView(TimelineView)
	model, _ = app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(*App)
	if app.activeView != TimelineView {
		t.Error("resize should preserve active view")
	}
	out := app.View()
	assertDimensions(t, "app resized", out, 80, 24)
}

func TestApp_FullscreenNoOverflow(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*App)

	app.fullscreen = true
	for _, panel := range []PanelID{PanelLeft, PanelMiddle, PanelRight} {
		app.explorer.SetFocusPanel(panel)
		out := app.View()
		assertDimensions(t, "app fullscreen explorer", out, 120, 40)
	}
}

func TestApp_NotReadyReturnsInitializing(t *testing.T) {
	store := newDataStore()
	app := NewApp(store)
	out := app.View()
	if !strings.Contains(out, "Initializing") {
		t.Error("app should show 'Initializing' before WindowSizeMsg")
	}
}

// ---------------------------------------------------------------------------
// Edge cases: many resources, long names, unicode
// ---------------------------------------------------------------------------

func TestApp_ManyResources(t *testing.T) {
	groups := []*resource.KindGroup{{Kind: "Pod"}}
	for i := range 200 {
		name := strings.Repeat("a", 30) + string(rune('0'+i%10))
		groups[0].Resources = append(groups[0].Resources,
			sampleResource("Pod", name, "default", "uid-"+name, 5))
	}
	resources := map[string]*resource.Data{}
	var timeline []resource.TimelineEntry
	for _, rd := range groups[0].Resources {
		resources[rd.Resource.UID] = rd
		for _, rev := range rd.Revisions {
			timeline = append(timeline, resource.TimelineEntry{
				Resource: rd.Resource,
				Revision: rev,
			})
		}
	}
	store := &dataStore{resources: resources, kindGroups: groups, timeline: timeline}

	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(*App)

	for _, v := range AllViews() {
		app.switchView(v)
		out := app.View()
		assertDimensions(t, "many resources "+v.String(), out, 120, 40)
	}
}

func TestApp_LongResourceNames(t *testing.T) {
	longName := strings.Repeat("very-long-resource-name-", 20)
	groups := []*resource.KindGroup{{
		Kind: "ConfigMap",
		Resources: []*resource.Data{
			sampleResource("ConfigMap", longName, "extremely-long-namespace-name", "uid-long", 10),
		},
	}}
	resources := map[string]*resource.Data{}
	for _, rd := range groups[0].Resources {
		resources[rd.Resource.UID] = rd
	}
	store := &dataStore{resources: resources, kindGroups: groups, timeline: nil}

	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(*App)

	out := app.View()
	assertDimensions(t, "long names", out, 80, 24)
}

func TestApp_UnicodeResourceNames(t *testing.T) {
	groups := []*resource.KindGroup{{
		Kind: "Pod",
		Resources: []*resource.Data{
			sampleResource("Pod", "日本語テスト", "名前空間", "uid-unicode", 3),
			sampleResource("Pod", "émojis-🎉🚀", "default", "uid-emoji", 2),
		},
	}}
	resources := map[string]*resource.Data{}
	for _, rd := range groups[0].Resources {
		resources[rd.Resource.UID] = rd
	}
	store := &dataStore{resources: resources, kindGroups: groups, timeline: nil}

	app := NewApp(store)
	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = model.(*App)

	out := app.View()
	assertDimensions(t, "unicode names", out, 100, 30)
}

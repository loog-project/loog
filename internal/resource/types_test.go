package resource

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ShortName
// ---------------------------------------------------------------------------

func TestShortName_MaxLenZero(t *testing.T) {
	r := Resource{Name: "nginx"}
	if got := r.ShortName(0); got != "nginx" {
		t.Errorf("ShortName(0) = %q, want %q", got, "nginx")
	}
}

func TestShortName_MaxLenNegative(t *testing.T) {
	r := Resource{Name: "nginx"}
	if got := r.ShortName(-5); got != "nginx" {
		t.Errorf("ShortName(-5) = %q, want %q", got, "nginx")
	}
}

func TestShortName_MaxLenOne(t *testing.T) {
	r := Resource{Name: "nginx"}
	if got := r.ShortName(1); got != "…" {
		t.Errorf("ShortName(1) = %q, want %q", got, "…")
	}
}

func TestShortName_MaxLenThree(t *testing.T) {
	r := Resource{Name: "nginx"}
	got := r.ShortName(3)
	// "nginx" is 5 runes, maxLen=3 → 2 runes + "…"
	want := "ng…"
	if got != want {
		t.Errorf("ShortName(3) = %q, want %q", got, want)
	}
}

func TestShortName_ExactLength(t *testing.T) {
	r := Resource{Name: "nginx"}
	if got := r.ShortName(5); got != "nginx" {
		t.Errorf("ShortName(5) = %q, want %q", got, "nginx")
	}
}

func TestShortName_LongerThanName(t *testing.T) {
	r := Resource{Name: "nginx"}
	if got := r.ShortName(100); got != "nginx" {
		t.Errorf("ShortName(100) = %q, want %q", got, "nginx")
	}
}

func TestShortName_MultiByteRunes(t *testing.T) {
	r := Resource{Name: "日本語テスト"} // 6 runes, each 3 bytes
	got := r.ShortName(4)
	want := "日本語…"
	if got != want {
		t.Errorf("ShortName(4) = %q, want %q", got, want)
	}
}

func TestShortName_MultiByteExact(t *testing.T) {
	r := Resource{Name: "日本語"}
	if got := r.ShortName(3); got != "日本語" {
		t.Errorf("ShortName(3) = %q, want %q", got, "日本語")
	}
}

func TestShortName_EmptyName(t *testing.T) {
	r := Resource{Name: ""}
	if got := r.ShortName(5); got != "" {
		t.Errorf("ShortName(5) on empty = %q, want %q", got, "")
	}
}

// ---------------------------------------------------------------------------
// KindName
// ---------------------------------------------------------------------------

func TestKindName_Basic(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "nginx-abc"}
	want := "Pod/nginx-abc"
	if got := r.KindName(); got != want {
		t.Errorf("KindName() = %q, want %q", got, want)
	}
}

func TestKindName_EmptyFields(t *testing.T) {
	r := Resource{}
	if got := r.KindName(); got != "/" {
		t.Errorf("KindName() = %q, want %q", got, "/")
	}
}

// ---------------------------------------------------------------------------
// CreationTime
// ---------------------------------------------------------------------------

func TestCreationTime_NoRevisions(t *testing.T) {
	rd := &Data{Revisions: nil}
	if got := rd.CreationTime(); !got.IsZero() {
		t.Errorf("CreationTime() with no revisions = %v, want zero", got)
	}
}

func TestCreationTime_NoMetadata(t *testing.T) {
	rd := &Data{Revisions: []Revision{{Object: map[string]any{"kind": "Pod"}}}}
	if got := rd.CreationTime(); !got.IsZero() {
		t.Errorf("CreationTime() with no metadata = %v, want zero", got)
	}
}

func TestCreationTime_ValidTimestamp(t *testing.T) {
	ts := "2024-01-15T10:30:00Z"
	rd := &Data{Revisions: []Revision{{
		Object: map[string]any{
			"metadata": map[string]any{
				"creationTimestamp": ts,
			},
		},
	}}}
	got := rd.CreationTime()
	want, _ := time.Parse(time.RFC3339, ts)
	if !got.Equal(want) {
		t.Errorf("CreationTime() = %v, want %v", got, want)
	}
}

func TestCreationTime_InvalidTimestamp(t *testing.T) {
	rd := &Data{Revisions: []Revision{{
		Object: map[string]any{
			"metadata": map[string]any{
				"creationTimestamp": "not-a-date",
			},
		},
	}}}
	if got := rd.CreationTime(); !got.IsZero() {
		t.Errorf("CreationTime() with invalid ts = %v, want zero", got)
	}
}

func TestCreationTime_NilObject(t *testing.T) {
	rd := &Data{Revisions: []Revision{{Object: nil}}}
	if got := rd.CreationTime(); !got.IsZero() {
		t.Errorf("CreationTime() with nil object = %v, want zero", got)
	}
}

// ---------------------------------------------------------------------------
// LatestRevision
// ---------------------------------------------------------------------------

func TestLatestRevision_Empty(t *testing.T) {
	rd := &Data{}
	if got := rd.LatestRevision(); got != nil {
		t.Errorf("LatestRevision() on empty = %v, want nil", got)
	}
}

func TestLatestRevision_Single(t *testing.T) {
	rev := Revision{ID: 1}
	rd := &Data{Revisions: []Revision{rev}}
	got := rd.LatestRevision()
	if got == nil || got.ID != 1 {
		t.Errorf("LatestRevision() = %v, want ID=1", got)
	}
}

func TestLatestRevision_Multiple(t *testing.T) {
	rd := &Data{Revisions: []Revision{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}}
	got := rd.LatestRevision()
	if got == nil || got.ID != 3 {
		t.Errorf("LatestRevision() = %v, want ID=3", got)
	}
}

// ---------------------------------------------------------------------------
// ChangeFrequency
// ---------------------------------------------------------------------------

func TestChangeFrequency_ZeroRevisions(t *testing.T) {
	rd := &Data{}
	if got := rd.ChangeFrequency(); got != 0 {
		t.Errorf("ChangeFrequency() with 0 revisions = %v, want 0", got)
	}
}

func TestChangeFrequency_OneRevision(t *testing.T) {
	rd := &Data{Revisions: []Revision{{Time: time.Now()}}}
	if got := rd.ChangeFrequency(); got != 0 {
		t.Errorf("ChangeFrequency() with 1 revision = %v, want 0", got)
	}
}

func TestChangeFrequency_CloseRevisions(t *testing.T) {
	now := time.Now()
	rd := &Data{Revisions: []Revision{
		{Time: now},
		{Time: now.Add(30 * time.Second)},
		{Time: now.Add(60 * time.Second)},
	}}
	// 2 changes in 1 minute = 2.0 changes/min
	got := rd.ChangeFrequency()
	if got < 1.99 || got > 2.01 {
		t.Errorf("ChangeFrequency() = %v, want ~2.0", got)
	}
}

func TestChangeFrequency_SameTime(t *testing.T) {
	now := time.Now()
	rd := &Data{Revisions: []Revision{
		{Time: now},
		{Time: now},
	}}
	// duration=0, should return 0
	if got := rd.ChangeFrequency(); got != 0 {
		t.Errorf("ChangeFrequency() with same time = %v, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// RelativeTime
// ---------------------------------------------------------------------------

func TestRelativeTime_Now(t *testing.T) {
	got := RelativeTime(time.Now())
	if got != "now" {
		t.Errorf("RelativeTime(now) = %q, want %q", got, "now")
	}
}

func TestRelativeTime_Seconds(t *testing.T) {
	got := RelativeTime(time.Now().Add(-30 * time.Second))
	if got != "30s" {
		t.Errorf("RelativeTime(-30s) = %q, want %q", got, "30s")
	}
}

func TestRelativeTime_Minutes(t *testing.T) {
	got := RelativeTime(time.Now().Add(-5 * time.Minute))
	if got != "5m" {
		t.Errorf("RelativeTime(-5m) = %q, want %q", got, "5m")
	}
}

func TestRelativeTime_Hours(t *testing.T) {
	got := RelativeTime(time.Now().Add(-3 * time.Hour))
	if got != "3h" {
		t.Errorf("RelativeTime(-3h) = %q, want %q", got, "3h")
	}
}

func TestRelativeTime_Days(t *testing.T) {
	got := RelativeTime(time.Now().Add(-48 * time.Hour))
	if got != "2d" {
		t.Errorf("RelativeTime(-48h) = %q, want %q", got, "2d")
	}
}

func TestRelativeTime_Future(t *testing.T) {
	got := RelativeTime(time.Now().Add(1 * time.Hour))
	if got != "future" {
		t.Errorf("RelativeTime(+1h) = %q, want %q", got, "future")
	}
}

// ---------------------------------------------------------------------------
// MatchesSubstring
// ---------------------------------------------------------------------------

func TestMatchesSubstring_EmptyQuery(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "nginx", Namespace: "default"}
	if !MatchesSubstring("", r) {
		t.Error("MatchesSubstring(\"\") should be true")
	}
}

func TestMatchesSubstring_NameMatch(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "nginx-abc", Namespace: "default"}
	if !MatchesSubstring("nginx", r) {
		t.Error("should match name substring")
	}
}

func TestMatchesSubstring_KindMatch(t *testing.T) {
	r := Resource{Kind: "Deployment", Name: "app", Namespace: "default"}
	if !MatchesSubstring("deploy", r) {
		t.Error("should match kind substring (case-insensitive)")
	}
}

func TestMatchesSubstring_NamespaceMatch(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "app", Namespace: "kube-system"}
	if !MatchesSubstring("kube-sys", r) {
		t.Error("should match namespace substring")
	}
}

func TestMatchesSubstring_CaseInsensitive(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "MyNginx", Namespace: "default"}
	if !MatchesSubstring("mynginx", r) {
		t.Error("should match case-insensitively")
	}
}

func TestMatchesSubstring_NoMatch(t *testing.T) {
	r := Resource{Kind: "Pod", Name: "nginx", Namespace: "default"}
	if MatchesSubstring("grafana", r) {
		t.Error("should not match unrelated query")
	}
}

// ---------------------------------------------------------------------------
// BuildKindGroups
// ---------------------------------------------------------------------------

func TestBuildKindGroups_Empty(t *testing.T) {
	groups := BuildKindGroups(nil)
	if len(groups) != 0 {
		t.Errorf("BuildKindGroups(nil) returned %d groups, want 0", len(groups))
	}
}

func TestBuildKindGroups_SingleKind(t *testing.T) {
	data := []*Data{
		{Resource: Resource{Kind: "Pod", Name: "b"}},
		{Resource: Resource{Kind: "Pod", Name: "a"}},
	}
	groups := BuildKindGroups(data)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	if groups[0].Kind != "Pod" {
		t.Errorf("group kind = %q, want %q", groups[0].Kind, "Pod")
	}
	// Resources should be sorted by name
	if groups[0].Resources[0].Resource.Name != "a" {
		t.Errorf("first resource = %q, want %q", groups[0].Resources[0].Resource.Name, "a")
	}
}

func TestBuildKindGroups_MultipleKinds_Ordering(t *testing.T) {
	data := []*Data{
		{Resource: Resource{Kind: "Secret", Name: "s1"}},
		{Resource: Resource{Kind: "Pod", Name: "p1"}},
		{Resource: Resource{Kind: "Service", Name: "svc1"}},
	}
	groups := BuildKindGroups(data)
	if len(groups) != 3 {
		t.Fatalf("got %d groups, want 3", len(groups))
	}
	// Expected order: Pod(0), Service(4), Secret(7)
	if groups[0].Kind != "Pod" {
		t.Errorf("groups[0].Kind = %q, want Pod", groups[0].Kind)
	}
	if groups[1].Kind != "Service" {
		t.Errorf("groups[1].Kind = %q, want Service", groups[1].Kind)
	}
	if groups[2].Kind != "Secret" {
		t.Errorf("groups[2].Kind = %q, want Secret", groups[2].Kind)
	}
}

func TestBuildKindGroups_MyAppRemoved(t *testing.T) {
	data := []*Data{
		{Resource: Resource{Kind: "MyApp", Name: "x"}},
		{Resource: Resource{Kind: "Pod", Name: "p"}},
	}
	groups := BuildKindGroups(data)
	// MyApp is unknown kind, should sort after known kinds alphabetically
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
	if groups[0].Kind != "Pod" {
		t.Errorf("groups[0].Kind = %q, want Pod (known kind first)", groups[0].Kind)
	}
	if groups[1].Kind != "MyApp" {
		t.Errorf("groups[1].Kind = %q, want MyApp (unknown kind last)", groups[1].Kind)
	}
}

func TestBuildKindGroups_ResourcesSortedByName(t *testing.T) {
	data := []*Data{
		{Resource: Resource{Kind: "Pod", Name: "z-pod"}},
		{Resource: Resource{Kind: "Pod", Name: "a-pod"}},
		{Resource: Resource{Kind: "Pod", Name: "m-pod"}},
	}
	groups := BuildKindGroups(data)
	names := make([]string, len(groups[0].Resources))
	for i, rd := range groups[0].Resources {
		names[i] = rd.Resource.Name
	}
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("resources not sorted: %v", names)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// GroupTimelineByBurst
// ---------------------------------------------------------------------------

func TestGroupTimelineByBurst_Empty(t *testing.T) {
	result := GroupTimelineByBurst(nil, time.Second)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestGroupTimelineByBurst_SingleEntry(t *testing.T) {
	entries := []TimelineEntry{
		{Resource: Resource{Name: "a"}, Revision: Revision{Time: time.Now()}},
	}
	result := GroupTimelineByBurst(entries, time.Second)
	if len(result) != 1 {
		t.Fatalf("got %d items, want 1", len(result))
	}
	if _, ok := result[0].(TimelineEntry); !ok {
		t.Error("single entry should remain a TimelineEntry, not a BurstGroup")
	}
}

func TestGroupTimelineByBurst_BurstDetection(t *testing.T) {
	now := time.Now()
	entries := []TimelineEntry{
		{Resource: Resource{Name: "a"}, Revision: Revision{Time: now}},
		{Resource: Resource{Name: "b"}, Revision: Revision{Time: now.Add(-100 * time.Millisecond)}},
		{Resource: Resource{Name: "c"}, Revision: Revision{Time: now.Add(-200 * time.Millisecond)}},
	}
	result := GroupTimelineByBurst(entries, time.Second)
	if len(result) != 1 {
		t.Fatalf("got %d items, want 1 burst group", len(result))
	}
	bg, ok := result[0].(BurstGroup)
	if !ok {
		t.Fatal("expected BurstGroup")
	}
	if len(bg.Entries) != 3 {
		t.Errorf("burst has %d entries, want 3", len(bg.Entries))
	}
}

func TestGroupTimelineByBurst_NoBurst(t *testing.T) {
	now := time.Now()
	entries := []TimelineEntry{
		{Resource: Resource{Name: "a"}, Revision: Revision{Time: now}},
		{Resource: Resource{Name: "b"}, Revision: Revision{Time: now.Add(-10 * time.Second)}},
		{Resource: Resource{Name: "c"}, Revision: Revision{Time: now.Add(-20 * time.Second)}},
	}
	result := GroupTimelineByBurst(entries, time.Second)
	if len(result) != 3 {
		t.Fatalf("got %d items, want 3 individual entries", len(result))
	}
	for i, item := range result {
		if _, ok := item.(TimelineEntry); !ok {
			t.Errorf("item[%d] should be TimelineEntry, got %T", i, item)
		}
	}
}

// ---------------------------------------------------------------------------
// CloneMap
// ---------------------------------------------------------------------------

func TestCloneMap_Nil(t *testing.T) {
	if got := CloneMap(nil); got != nil {
		t.Errorf("CloneMap(nil) = %v, want nil", got)
	}
}

func TestCloneMap_Empty(t *testing.T) {
	got := CloneMap(map[string]any{})
	if got == nil || len(got) != 0 {
		t.Errorf("CloneMap(empty) = %v, want empty map", got)
	}
}

func TestCloneMap_NestedMaps(t *testing.T) {
	inner := map[string]any{"key": "value"}
	original := map[string]any{"nested": inner}
	cloned := CloneMap(original)

	// Modify original
	inner["key"] = "changed"

	// Clone should be independent
	clonedInner, ok := cloned["nested"].(map[string]any)
	if !ok {
		t.Fatal("cloned nested is not map[string]any")
	}
	if clonedInner["key"] != "value" {
		t.Errorf("clone was mutated: got %q, want %q", clonedInner["key"], "value")
	}
}

func TestCloneMap_NestedSlices(t *testing.T) {
	original := map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			"scalar",
		},
	}
	cloned := CloneMap(original)

	// Modify original slice element
	origItems := original["items"].([]any)
	origItems[0].(map[string]any)["name"] = "changed"

	clonedItems := cloned["items"].([]any)
	clonedMap := clonedItems[0].(map[string]any)
	if clonedMap["name"] != "a" {
		t.Errorf("clone slice map was mutated: got %q, want %q", clonedMap["name"], "a")
	}
	if clonedItems[1] != "scalar" {
		t.Errorf("scalar in slice = %v, want %q", clonedItems[1], "scalar")
	}
}

func TestCloneMap_DeeplyNestedSlices(t *testing.T) {
	// []any containing []any — the bug that was fixed
	original := map[string]any{
		"matrix": []any{
			[]any{"a", "b"},
			[]any{
				map[string]any{"deep": "val"},
			},
		},
	}
	cloned := CloneMap(original)

	// Modify the deeply nested map
	origMatrix := original["matrix"].([]any)
	origInner := origMatrix[1].([]any)
	origInner[0].(map[string]any)["deep"] = "changed"

	clonedMatrix := cloned["matrix"].([]any)
	clonedInner := clonedMatrix[1].([]any)
	clonedDeep := clonedInner[0].(map[string]any)
	if clonedDeep["deep"] != "val" {
		t.Errorf("deeply nested clone was mutated: got %q, want %q", clonedDeep["deep"], "val")
	}

	// Check the simple inner slice is independent too
	clonedFirst := clonedMatrix[0].([]any)
	if clonedFirst[0] != "a" || clonedFirst[1] != "b" {
		t.Errorf("inner slice values wrong: %v", clonedFirst)
	}
}

func TestCloneMap_ScalarValues(t *testing.T) {
	original := map[string]any{
		"str":  "hello",
		"num":  42,
		"flag": true,
		"nil":  nil,
	}
	cloned := CloneMap(original)
	if cloned["str"] != "hello" || cloned["num"] != 42 || cloned["flag"] != true || cloned["nil"] != nil {
		t.Errorf("scalar values not preserved: %v", cloned)
	}
}

// ---------------------------------------------------------------------------
// EventType.Symbol
// ---------------------------------------------------------------------------

func TestEventTypeSymbol(t *testing.T) {
	tests := []struct {
		et   EventType
		want string
	}{
		{EventAdded, "+"},
		{EventModified, "~"},
		{EventDeleted, "-"},
		{EventType("UNKNOWN"), "?"},
	}
	for _, tt := range tests {
		if got := tt.et.Symbol(); got != tt.want {
			t.Errorf("EventType(%q).Symbol() = %q, want %q", tt.et, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// RevisionCount
// ---------------------------------------------------------------------------

func TestRevisionCount(t *testing.T) {
	rd := &Data{Revisions: []Revision{{}, {}, {}}}
	if got := rd.RevisionCount(); got != 3 {
		t.Errorf("RevisionCount() = %d, want 3", got)
	}
}

// ---------------------------------------------------------------------------
// FormatTimestamp
// ---------------------------------------------------------------------------

func TestFormatTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 15, 14, 32, 5, 0, time.UTC)
	got := FormatTimestamp(ts)
	if got != "14:32:05" {
		t.Errorf("FormatTimestamp() = %q, want %q", got, "14:32:05")
	}
}

// ---------------------------------------------------------------------------
// Kind methods
// ---------------------------------------------------------------------------

func TestKind_String(t *testing.T) {
	k := Kind{Kind: "Pod", APIVersion: "v1", Resource: "pods"}
	if got := k.String(); got != "Pod" {
		t.Errorf("Kind.String() = %q, want %q", got, "Pod")
	}
}

func TestKind_GVR(t *testing.T) {
	k := Kind{Kind: "Deployment", APIVersion: "apps/v1", Resource: "deployments"}
	want := "apps/v1/deployments"
	if got := k.GVR(); got != want {
		t.Errorf("Kind.GVR() = %q, want %q", got, want)
	}
}

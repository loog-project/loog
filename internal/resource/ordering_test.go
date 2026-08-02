package resource

import (
	"testing"
	"time"
)

// resourceVersion must decide order for near-simultaneous events, even when the
// wall-clock times are inverted (as they can be across independent watches).
func TestCompareRevisionsNewestFirst_ResourceVersionWins(t *testing.T) {
	base := time.Now()
	// A happened first (lower rv) but was OBSERVED later (later time).
	a := Revision{ResourceVersion: 100, Time: base.Add(50 * time.Millisecond)}
	// B happened second (higher rv) but was observed first (earlier time).
	b := Revision{ResourceVersion: 105, Time: base}

	// Newest-first: B (higher rv) must sort before A despite its earlier time.
	if got := CompareRevisionsNewestFirst(b, a); got >= 0 {
		t.Errorf("expected B (rv 105) before A (rv 100), got %d", got)
	}
	if got := CompareRevisionsNewestFirst(a, b); got <= 0 {
		t.Errorf("expected A after B, got %d", got)
	}
}

func TestCompareRevisionsNewestFirst_FallsBackToTime(t *testing.T) {
	base := time.Now()
	// No resourceVersion -> order by time, newest first.
	newer := Revision{Time: base.Add(time.Second)}
	older := Revision{Time: base}
	if got := CompareRevisionsNewestFirst(newer, older); got >= 0 {
		t.Errorf("expected newer before older by time, got %d", got)
	}
	// One side missing rv -> also falls back to time.
	mixed := Revision{ResourceVersion: 7, Time: base}
	if got := CompareRevisionsNewestFirst(mixed, newer); got <= 0 {
		t.Errorf("expected time fallback to put newer first, got %d", got)
	}
}

func TestSortTimelineNewestFirst_CausalOrder(t *testing.T) {
	base := time.Now()
	// Simulate the reported scenario: B observed before A, but A is causally
	// first (lower rv). Ready events land within the same instant.
	entries := []TimelineEntry{
		{Revision: Revision{ResourceVersion: 205, Time: base}},                       // B ready (observed first)
		{Revision: Revision{ResourceVersion: 200, Time: base.Add(30 * time.Millisecond)}}, // A ready (observed later)
	}
	SortTimelineNewestFirst(entries)

	// Newest-first: B (rv 205, causally later) must be first, A second.
	if entries[0].Revision.ResourceVersion != 205 || entries[1].Revision.ResourceVersion != 200 {
		t.Errorf("causal order wrong: got %d then %d",
			entries[0].Revision.ResourceVersion, entries[1].Revision.ResourceVersion)
	}
}

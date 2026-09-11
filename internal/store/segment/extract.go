package segment

import (
	"context"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
)

// farFuture stands in for an open-ended "To" bound.
var farFuture = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

// ExtractOptions configures a time-range extraction.
type ExtractOptions struct {
	// From/To bound the window (inclusive). A zero From means "from the
	// beginning"; a zero To means "up to now".
	From time.Time
	To   time.Time

	// Filter, when non-nil, is an expr-lang program (compiled against
	// util.EventEntryEnv) that decides whether an object is included. Only the
	// Object is populated during extraction (there is no live watch Event).
	Filter *vm.Program

	// ExcludeActive is the path of a segment that must not be opened (the
	// recorder's active file). Ignored when empty.
	ExcludeActive string

	// Bolt controls durability/compression of the produced .loog file.
	Bolt bboltStore.Options
}

// Stats summarizes what an extraction produced.
type Stats struct {
	SegmentsScanned  int
	RevisionsScanned int
	RevisionsWritten int
	Resources        int
}

// Extract reads the segments in dir that overlap [From,To], reconstructs each
// object's state over time, and writes every revision whose timestamp falls in
// the window into a fresh .loog file at outPath. Original timestamps and delete
// markers are preserved, so the result replays exactly like the live capture
// for that window. The output is written as self-contained snapshots, so it can
// be browsed with `loog --replay outPath`.
func Extract(ctx context.Context, dir, outPath string, opts ExtractOptions) (Stats, error) {
	var stats Stats

	from := opts.From
	to := opts.To
	if to.IsZero() {
		to = time.Now()
	}

	all, err := ListSegments(dir, opts.ExcludeActive)
	if err != nil {
		return stats, fmt.Errorf("list segments: %w", err)
	}
	selected := selectOverlapping(all, from, to)
	if len(selected) == 0 {
		return stats, fmt.Errorf("no segments overlap the requested window")
	}

	out, err := bboltStore.NewWithOptions(outPath, opts.Bolt)
	if err != nil {
		return stats, fmt.Errorf("create output store: %w", err)
	}
	defer func() { _ = out.Close() }()

	seenResources := make(map[string]struct{})

	for _, seg := range selected {
		stats.SegmentsScanned++
		if err := extractSegment(ctx, seg, out, from, to, opts.Filter, &stats, seenResources); err != nil {
			return stats, fmt.Errorf("extract %s: %w", seg.Name, err)
		}
	}
	stats.Resources = len(seenResources)
	return stats, nil
}

// selectOverlapping returns the segments whose [Start, nextStart) window
// intersects [from, to]. Over-selection is harmless because each revision is
// still filtered by its own timestamp; we err toward including a segment.
func selectOverlapping(segs []Info, from, to time.Time) []Info {
	lo := from // zero => matches everything at/after the epoch
	hi := to
	if hi.IsZero() {
		hi = farFuture
	}
	var out []Info
	for i, seg := range segs {
		end := farFuture
		if i+1 < len(segs) {
			end = segs[i+1].Start
		}
		// overlap if seg.Start <= hi AND end > lo
		if !seg.Start.After(hi) && end.After(lo) {
			out = append(out, seg)
		}
	}
	return out
}

// extractSegment reconstructs object state within one segment and writes the
// in-window revisions to out. Every revision in the segment is applied to keep
// reconstruction correct, but only those inside [from,to] are emitted.
func extractSegment(
	ctx context.Context,
	seg Info,
	out *bboltStore.Store,
	from, to time.Time,
	filter *vm.Program,
	stats *Stats,
	seenResources map[string]struct{},
) error {
	src, err := bboltStore.NewWithOptions(seg.Path, bboltStore.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("open segment: %w", err)
	}
	defer func() { _ = src.Close() }()

	// Per-segment reconstruction state. Reset per segment because rotation makes
	// each segment start with fresh snapshots for every object it touches.
	state := make(map[string]diffmap.DiffMap)

	walkErr := src.WalkObjectRevisions(func(
		uid string,
		_ store.RevisionID,
		snapshot *store.Snapshot,
		patch *store.Patch,
	) bool {
		stats.RevisionsScanned++

		var obj diffmap.DiffMap
		var t time.Time
		var deleted bool
		eventType := resource.EventModified

		switch {
		case snapshot != nil:
			obj = resource.CloneMap(snapshot.Object)
			t = snapshot.Time
			deleted = snapshot.Deleted
			if snapshot.PreviousID == 0 {
				eventType = resource.EventAdded
			}
		case patch != nil:
			base := state[uid]
			if base == nil {
				log.Warn().Str("uid", uid).Str("segment", seg.Name).
					Msg("Patch without a base snapshot during extraction; skipping")
				return true
			}
			obj = resource.CloneMap(base)
			diffmap.Apply(obj, patch.Patch)
			t = patch.Time
			deleted = patch.Deleted
		default:
			return true
		}

		state[uid] = obj
		if deleted {
			eventType = resource.EventDeleted
		}

		if t.Before(from) || t.After(to) {
			return true
		}
		if !passesFilter(filter, obj) {
			return true
		}

		// PreviousID encodes the event type for replay's buildRevision:
		// 0 -> ADDED, non-zero -> MODIFIED, Deleted flag -> DELETED.
		var prevID store.RevisionID
		if eventType != resource.EventAdded {
			prevID = 1
		}
		outSnap := store.Snapshot{
			PreviousID: prevID,
			Object:     resource.CloneMap(obj),
			Time:       t,
			Deleted:    deleted,
		}
		if err := out.SetSnapshot(ctx, uid, &outSnap); err != nil {
			log.Error().Err(err).Str("uid", uid).Msg("Error writing extracted revision")
			return true
		}
		stats.RevisionsWritten++
		seenResources[uid] = struct{}{}
		return true
	})
	return walkErr
}

func passesFilter(filter *vm.Program, obj diffmap.DiffMap) bool {
	if filter == nil {
		return true
	}
	u := &unstructured.Unstructured{Object: obj}
	res, err := expr.Run(filter, util.EventEntryEnv{Object: u})
	if err != nil {
		log.Error().Err(err).Msg("Filter expression error during extraction; including object")
		return true
	}
	b, ok := res.(bool)
	if !ok {
		log.Error().Msgf("Filter returned %T, want bool; including object", res)
		return true
	}
	return b
}

// Package segment implements a time-segmented, self-rotating revision store on
// top of the bbolt store. It is the storage backend for loog's in-cluster
// recorder: it writes changes into an active segment file, rolls to a
// new file every Interval, and prunes segments older than Retention.
package segment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
)

const (
	filePrefix = "loog-"
	fileSuffix = ".loog"
	// segTimeLayout is a lexicographically sortable, filesystem-safe timestamp.
	// Millisecond precision keeps names unique across fast rotations (tests).
	segTimeLayout = "20060102T150405.000Z0700"

	defaultInterval  = time.Hour
	defaultRetention = 14 * 24 * time.Hour
)

// Options controls rotation, retention, and the durability of each segment.
type Options struct {
	// Interval is how long an active segment collects before rolling to a new
	// file. Defaults to 1h.
	Interval time.Duration

	// Retention deletes rolled segments whose time window ends before
	// now-Retention. Zero keeps everything. Defaults to 30 days.
	Retention time.Duration

	// MaxSegments, when > 0, additionally caps the number of retained segments
	// (oldest deleted first). Useful as a hard disk-usage guard.
	MaxSegments int

	// Bolt is passed to each underlying segment file (durability, compression).
	Bolt bboltStore.Options
}

func (o *Options) withDefaults() {
	if o.Interval <= 0 {
		o.Interval = defaultInterval
	}
	if o.Retention == 0 {
		o.Retention = defaultRetention
	}
}

// Store is a segmented [store.ResourcePatchStore]. All reads and writes go to
// the currently active segment; use MaybeRotate/Rotate to advance segments.
type Store struct {
	dir  string
	opts Options

	mu          sync.Mutex
	active      *bboltStore.Store
	activePath  string
	activeStart time.Time
}

var _ store.ResourcePatchStore = (*Store)(nil)

// Open prepares dir and starts a fresh active segment. Starting fresh (rather
// than resuming the newest file) keeps segments self-contained across recorder
// restarts, since informers re-list all objects on startup and thus re-snapshot
// them into the new segment.
func Open(dir string, opts Options) (*Store, error) {
	opts.withDefaults()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create segment dir: %w", err)
	}
	s := &Store{dir: dir, opts: opts}
	if err := s.openNewSegment(time.Now()); err != nil {
		return nil, err
	}
	s.prune()
	return s, nil
}

// Dir returns the directory holding the segment files.
func (s *Store) Dir() string { return s.dir }

// ActivePath returns the path of the segment currently being written. Readers
// (extraction, HTTP API) must skip it: bbolt takes an exclusive lock on the
// open read-write file, so a second read-only open in the same process fails.
func (s *Store) ActivePath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activePath
}

func (s *Store) openNewSegment(now time.Time) error {
	path := filepath.Join(s.dir, filePrefix+now.UTC().Format(segTimeLayout)+fileSuffix)
	for i := 0; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		now = now.Add(time.Millisecond)
		path = filepath.Join(s.dir, filePrefix+now.UTC().Format(segTimeLayout)+fileSuffix)
		if i > 1000 {
			return fmt.Errorf("cannot find free segment name in %s", s.dir)
		}
	}

	st, err := bboltStore.NewWithOptions(path, s.opts.Bolt)
	if err != nil {
		return fmt.Errorf("open segment %s: %w", path, err)
	}
	s.active = st
	s.activePath = path
	s.activeStart = now
	log.Info().Str("segment", filepath.Base(path)).Msg("Opened new segment")
	return nil
}

// MaybeRotate rolls to a new segment if the active one has been open for at
// least Interval. It reports whether a rotation happened so the caller can
// reset the tracker cache. Safe to call frequently.
func (s *Store) MaybeRotate() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.activeStart) < s.opts.Interval {
		return false, nil
	}
	return true, s.rotateLocked()
}

// Rotate forces an immediate roll to a new segment regardless of Interval.
func (s *Store) Rotate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rotateLocked()
}

func (s *Store) rotateLocked() error {
	if s.active != nil {
		if err := s.active.Close(); err != nil {
			log.Error().Err(err).Str("segment", s.activePath).Msg("Error closing segment on rotate")
		}
	}
	if err := s.openNewSegment(time.Now()); err != nil {
		return err
	}
	s.prune()
	return nil
}

// prune deletes rolled segments older than Retention and, if MaxSegments is
// set, the oldest segments beyond that count. The active segment is never
// pruned.
func (s *Store) prune() {
	segs, err := listSegmentFiles(s.dir)
	if err != nil {
		log.Error().Err(err).Msg("Error listing segments for prune")
		return
	}

	var cutoff time.Time
	if s.opts.Retention > 0 {
		cutoff = time.Now().Add(-s.opts.Retention)
	}

	for i, seg := range segs {
		if seg.Path == s.activePath {
			continue
		}
		end := time.Now()
		if i+1 < len(segs) {
			end = segs[i+1].Start
		}
		tooOld := !cutoff.IsZero() && end.Before(cutoff)
		if tooOld {
			s.deleteSegment(seg.Path)
		}
	}

	if s.opts.MaxSegments > 0 {
		remaining, _ := listSegmentFiles(s.dir)
		excess := len(remaining) - s.opts.MaxSegments
		for i := 0; i < len(remaining) && excess > 0; i++ {
			if remaining[i].Path == s.activePath {
				continue
			}
			s.deleteSegment(remaining[i].Path)
			excess--
		}
	}
}

func (s *Store) deleteSegment(path string) {
	if err := os.Remove(path); err != nil {
		log.Error().Err(err).Str("segment", path).Msg("Error deleting expired segment")
		return
	}
	log.Info().Str("segment", filepath.Base(path)).Msg("Pruned expired segment")
}

// --- store.ResourcePatchStore delegation ---------------------------------

func (s *Store) Get(ctx context.Context, objectID string, revID store.RevisionID) (
	*store.Snapshot,
	*store.Patch,
	error,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.Get(ctx, objectID, revID)
}

func (s *Store) SetSnapshot(ctx context.Context, objectID string, snap *store.Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.SetSnapshot(ctx, objectID, snap)
}

func (s *Store) SetPatch(ctx context.Context, objectID string, p *store.Patch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.SetPatch(ctx, objectID, p)
}

func (s *Store) GetLatestRevision(ctx context.Context, objectID string) (store.RevisionID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.GetLatestRevision(ctx, objectID)
}

func (s *Store) WalkObjectRevisions(yield func(string, store.RevisionID, *store.Snapshot, *store.Patch) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active.WalkObjectRevisions(yield)
}

// Close flushes and closes the active segment.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil
	}
	err := s.active.Close()
	s.active = nil
	return err
}

// --- segment discovery ----------------------------------------------------

// Info describes one segment file on disk.
type Info struct {
	Path  string    `json:"path"`
	Name  string    `json:"name"`
	Start time.Time `json:"start"`
	// End is the start of the following segment (or zero for the newest one).
	End       time.Time `json:"end,omitempty"`
	SizeBytes int64     `json:"sizeBytes"`
}

// ListSegments returns the segments in dir sorted oldest-first, with End filled
// in from the following segment's start. Files in exclude (by absolute path)
// are omitted; pass the recorder's ActivePath so callers never try to open the
// file that is currently locked for writing.
func ListSegments(dir string, exclude ...string) ([]Info, error) {
	segs, err := listSegmentFiles(dir)
	if err != nil {
		return nil, err
	}
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		if abs, aerr := filepath.Abs(e); aerr == nil {
			skip[abs] = struct{}{}
		}
	}

	out := make([]Info, 0, len(segs))
	for i, seg := range segs {
		if abs, aerr := filepath.Abs(seg.Path); aerr == nil {
			if _, ok := skip[abs]; ok {
				continue
			}
		}
		info := seg
		if i+1 < len(segs) {
			info.End = segs[i+1].Start
		}
		if fi, statErr := os.Stat(seg.Path); statErr == nil {
			info.SizeBytes = fi.Size()
		}
		out = append(out, info)
	}
	return out, nil
}

// listSegmentFiles returns every parseable segment file in dir sorted by start
// time (End and SizeBytes are not filled in here).
func listSegmentFiles(dir string) ([]Info, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var segs []Info
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		start, ok := parseSegmentTime(e.Name())
		if !ok {
			continue
		}
		segs = append(segs, Info{
			Path:  filepath.Join(dir, e.Name()),
			Name:  e.Name(),
			Start: start,
		})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].Start.Before(segs[j].Start) })
	return segs, nil
}

func parseSegmentTime(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
		return time.Time{}, false
	}
	mid := name[len(filePrefix) : len(name)-len(fileSuffix)]
	t, err := time.Parse(segTimeLayout, mid)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

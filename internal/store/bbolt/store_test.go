package bbolt

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/pkg/diffmap"
)

var (
	ctx = context.Background()
	id  = "object-uid"
)

// equalMaps compares two DiffMaps with tolerance for msgpack numeric type
// round-tripping. Msgpack decodes integers into the smallest fitting type
// (e.g. int -> int8), so reflect.DeepEqual fails even though values match.
func equalMaps(t *testing.T, got, want diffmap.DiffMap) bool {
	t.Helper()
	return equalAny(t, "", got, want)
}

func equalAny(t *testing.T, path string, got, want any) bool {
	t.Helper()

	// nil
	if got == nil && want == nil {
		return true
	}
	if (got == nil) != (want == nil) {
		t.Errorf("%s: got=%v want=%v (nil mismatch)", path, got, want)
		return false
	}

	// both DiffMap / map[string]any
	gm, gok := toMap(got)
	wm, wok := toMap(want)
	if gok && wok {
		if len(gm) != len(wm) {
			t.Errorf("%s: map len got=%d want=%d", path, len(gm), len(wm))
			return false
		}
		ok := true
		for k, wv := range wm {
			gv, has := gm[k]
			if !has {
				t.Errorf("%s: missing key %q", path, k)
				ok = false
				continue
			}
			if !equalAny(t, path+"."+k, gv, wv) {
				ok = false
			}
		}
		return ok
	}

	gn, gnOk := toFloat64(got)
	wn, wnOk := toFloat64(want)
	if gnOk && wnOk {
		if gn != wn {
			t.Errorf("%s: got=%v want=%v (numeric mismatch)", path, got, want)
			return false
		}
		return true
	}

	// fallback: reflect
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got=%v (%T) want=%v (%T)", path, got, got, want, want)
		return false
	}
	return true
}

func toMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	return nil, false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// helper to open a store with options and register cleanup.
func openStore(t *testing.T, opts Options) *Store {
	t.Helper()
	s, err := NewWithOptions(filepath.Join(t.TempDir(), "test.bb"), opts)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ---------------------------------------------------------------------------
// Basic functionality
// ---------------------------------------------------------------------------

func TestNewAndBuckets(t *testing.T) {
	dir := t.TempDir()
	s, err := New(filepath.Join(dir, "db.bb"), nil, false)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	info1, _ := os.Stat(s.db.Path())
	if info1.Size() == 0 {
		t.Fatal("DB file should not be empty")
	}
}

func TestSnapshotPatchRoundtrip(t *testing.T) {
	s := openStore(t, Options{})

	snap := &store.Snapshot{Object: diffmap.DiffMap{"foo": "bar"}}
	if err := s.SetSnapshot(ctx, id, snap); err != nil {
		t.Fatalf("set snapshot: %v", err)
	}
	if snap.ID != 0 {
		t.Fatalf("first snapshot should have ID 0, got %d", snap.ID)
	}

	latest, err := s.GetLatestRevision(ctx, id)
	if err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if latest != 0 {
		t.Fatalf("latest want 0, got %d", latest)
	}

	patch1 := &store.Patch{PreviousID: snap.ID, Patch: diffmap.DiffMap{"foo": "baz"}}
	if err := s.SetPatch(ctx, id, patch1); err != nil {
		t.Fatalf("set patch1: %v", err)
	}
	if patch1.ID != 1 {
		t.Fatalf("patch1 should receive ID 1, got %d", patch1.ID)
	}

	patch2 := &store.Patch{PreviousID: patch1.ID, Patch: diffmap.DiffMap{"bar": 42}}
	_ = s.SetPatch(ctx, id, patch2)

	if latest, _ := s.GetLatestRevision(ctx, id); latest != 2 {
		t.Fatalf("latest want 2, got %d", latest)
	}

	sn0, p0, err := s.Get(ctx, id, 0)
	if err != nil || p0 != nil || sn0 == nil {
		t.Fatalf("rev0: want snapshot, got %+v / %+v / err=%v", sn0, p0, err)
	}
	sn1, p1, _ := s.Get(ctx, id, 1)
	if sn1 != nil || p1 == nil || p1.ID != 1 {
		t.Fatalf("rev1 not patch1")
	}
	_, p2, _ := s.Get(ctx, id, 2)
	if p2 == nil || p2.ID != 2 {
		t.Fatalf("rev2 not patch2")
	}
}

func TestConcurrentClaims(t *testing.T) {
	s := openStore(t, Options{})

	errs := make(chan error, 20)
	for i := range 20 {
		go func() {
			errs <- s.SetSnapshot(ctx, id, &store.Snapshot{Object: diffmap.DiffMap{"x": i}})
		}()
	}
	for range 20 {
		if e := <-errs; e != nil {
			t.Fatalf("concurrent SetSnapshot failed: %v", e)
		}
	}
	if latest, _ := s.GetLatestRevision(ctx, id); latest != 19 {
		t.Fatalf("after 20 writes, latest should be 19, got %d", latest)
	}
}

func TestPersistedValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.bb")
	s, _ := New(path, nil, false)
	_ = s.SetSnapshot(ctx, id, &store.Snapshot{Object: diffmap.DiffMap{"k": "v"}})
	_ = s.Close()

	blob, _ := os.ReadFile(path)
	if !bytes.Contains(blob, []byte{0x81}) {
		t.Fatalf("file does not appear to contain msgpack map header")
	}
}

// ---------------------------------------------------------------------------
// Data integrity: write many revisions, read every one back, deep-compare.
// Runs once for each store mode to catch pool/compression bugs.
// ---------------------------------------------------------------------------

func TestDataIntegrity(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
		{"periodic_sync", Options{Durable: true, SyncInterval: 10 * time.Millisecond}},
		{"periodic_sync_compressed", Options{Durable: true, SyncInterval: 10 * time.Millisecond, Compress: true}},
	}

	const revisions = 64

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			s := openStore(t, mode.opts)

			// keep expected data to verify later
			type entry struct {
				isSnapshot bool
				data       diffmap.DiffMap
				prevID     store.RevisionID
			}
			expected := make([]entry, revisions)

			for i := range revisions {
				data := diffmap.DiffMap{
					"index":  i,
					"value":  strings.Repeat("v", i+1),
					"nested": diffmap.DiffMap{"deep": float64(i) * 1.5},
				}

				if i%8 == 0 {
					snap := &store.Snapshot{
						Object: data,
					}
					if i > 0 {
						snap.PreviousID = store.RevisionID(i - 1)
					}
					if err := s.SetSnapshot(ctx, id, snap); err != nil {
						t.Fatalf("rev %d: set snapshot: %v", i, err)
					}
					if int(snap.ID) != i {
						t.Fatalf("rev %d: ID want %d got %d", i, i, snap.ID)
					}
					expected[i] = entry{isSnapshot: true, data: data, prevID: snap.PreviousID}
				} else {
					p := &store.Patch{
						PreviousID: store.RevisionID(i - 1),
						Patch:      data,
						Time:       time.Now(),
					}
					if err := s.SetPatch(ctx, id, p); err != nil {
						t.Fatalf("rev %d: set patch: %v", i, err)
					}
					if int(p.ID) != i {
						t.Fatalf("rev %d: ID want %d got %d", i, i, p.ID)
					}
					expected[i] = entry{isSnapshot: false, data: data, prevID: p.PreviousID}
				}
			}

			// read back every revision and verify
			for i, exp := range expected {
				snap, patch, err := s.Get(ctx, id, store.RevisionID(i))
				if err != nil {
					t.Fatalf("rev %d: get: %v", i, err)
				}

				if exp.isSnapshot {
					if snap == nil {
						t.Fatalf("rev %d: expected snapshot, got patch", i)
					}
					if patch != nil {
						t.Fatalf("rev %d: expected nil patch alongside snapshot", i)
					}
					if snap.ID != store.RevisionID(i) {
						t.Fatalf("rev %d: snapshot ID = %d", i, snap.ID)
					}
					if snap.PreviousID != exp.prevID {
						t.Fatalf("rev %d: snapshot PreviousID want %d got %d", i, exp.prevID, snap.PreviousID)
					}
					if !equalMaps(t, snap.Object, exp.data) {
						t.Fatalf("rev %d: snapshot data mismatch", i)
					}
				} else {
					if patch == nil {
						t.Fatalf("rev %d: expected patch, got snapshot", i)
					}
					if snap != nil {
						t.Fatalf("rev %d: expected nil snapshot alongside patch", i)
					}
					if patch.ID != store.RevisionID(i) {
						t.Fatalf("rev %d: patch ID = %d", i, patch.ID)
					}
					if patch.PreviousID != exp.prevID {
						t.Fatalf("rev %d: patch PreviousID want %d got %d", i, exp.prevID, patch.PreviousID)
					}
					if !equalMaps(t, patch.Patch, exp.data) {
						t.Fatalf("rev %d: patch data mismatch", i)
					}
				}
			}

			// verify latest
			latest, err := s.GetLatestRevision(ctx, id)
			if err != nil {
				t.Fatalf("get latest: %v", err)
			}
			if int(latest) != revisions-1 {
				t.Fatalf("latest want %d got %d", revisions-1, latest)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pool safety under concurrent access: stresses all pooled buffers at once.
// The race detector catches any cross-goroutine buffer corruption.
// ---------------------------------------------------------------------------

func TestPoolSafety_ConcurrentWriteRead(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			s := openStore(t, mode.opts)

			const objects = 8
			const revisionsPerObject = 30
			uids := make([]string, objects)
			for i := range objects {
				uids[i] = "uid-" + strconv.Itoa(i)
			}

			// concurrent writes
			var wg sync.WaitGroup
			for _, uid := range uids {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := range revisionsPerObject {
						data := diffmap.DiffMap{
							"obj":   uid,
							"seq":   j,
							"pad":   strings.Repeat("x", 100),
							"inner": diffmap.DiffMap{"n": j},
						}
						if j == 0 {
							err := s.SetSnapshot(ctx, uid, &store.Snapshot{Object: data})
							if err != nil {
								t.Errorf("snapshot %s/%d: %v", uid, j, err)
							}
						} else {
							err := s.SetPatch(ctx, uid, &store.Patch{
								PreviousID: store.RevisionID(j - 1),
								Patch:      data,
							})
							if err != nil {
								t.Errorf("patch %s/%d: %v", uid, j, err)
							}
						}
					}
				}()
			}
			wg.Wait()

			// concurrent reads while more writes happen
			var wg2 sync.WaitGroup
			for _, uid := range uids {
				// reader
				wg2.Add(1)
				go func() {
					defer wg2.Done()
					for j := range revisionsPerObject {
						snap, patch, err := s.Get(ctx, uid, store.RevisionID(j))
						if err != nil {
							t.Errorf("get %s/%d: %v", uid, j, err)
							return
						}
						if snap == nil && patch == nil {
							t.Errorf("get %s/%d: both nil", uid, j)
							return
						}
						// verify the data belongs to this object
						var data diffmap.DiffMap
						if snap != nil {
							data = snap.Object
						} else {
							data = patch.Patch
						}
						if data["obj"] != uid {
							t.Errorf("get %s/%d: wrong uid in data: %v", uid, j, data["obj"])
						}
					}
				}()
			}
			wg2.Wait()

			// verify all revisions for each object
			for _, uid := range uids {
				latest, err := s.GetLatestRevision(ctx, uid)
				if err != nil {
					t.Fatalf("latest for %s: %v", uid, err)
				}
				if int(latest) != revisionsPerObject-1 {
					t.Fatalf("latest for %s: want %d got %d", uid, revisionsPerObject-1, latest)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Compression round-trip with various payload shapes.
// Checks that s2 compression introduces no artifacts.
// ---------------------------------------------------------------------------

func TestCompression_PayloadShapes(t *testing.T) {
	s := openStore(t, Options{Compress: true})

	cases := []struct {
		name string
		data diffmap.DiffMap
	}{
		{"empty_map", diffmap.DiffMap{}},
		{"single_key", diffmap.DiffMap{"a": "b"}},
		{
			"numeric_values", diffmap.DiffMap{
				"int": 42, "int64": int64(math.MaxInt64), "float": 3.14,
			},
		},
		{
			"bool_and_nil", diffmap.DiffMap{
				"yes": true, "no": false, "null": nil,
			},
		},
		{
			"deeply_nested", diffmap.DiffMap{
				"l1": diffmap.DiffMap{
					"l2": diffmap.DiffMap{
						"l3": diffmap.DiffMap{
							"l4": diffmap.DiffMap{"leaf": "deep"},
						},
					},
				},
			},
		},
		{
			"large_string_value", diffmap.DiffMap{
				"big": strings.Repeat("abcdef0123456789", 1000),
			},
		},
		{
			"many_keys", func() diffmap.DiffMap {
				m := make(diffmap.DiffMap, 200)
				for i := range 200 {
					m["key-"+strconv.Itoa(i)] = strconv.Itoa(i)
				}
				return m
			}(),
		},
		{
			"mixed_types", diffmap.DiffMap{
				"s": "hello", "i": 1, "i64": int64(2),
				"f": 1.5, "b": true, "n": nil,
				"m": diffmap.DiffMap{"x": "y"},
			},
		},
		{
			"repetitive_data", func() diffmap.DiffMap {
				// highly compressible
				m := make(diffmap.DiffMap, 50)
				for i := range 50 {
					m["k"+strconv.Itoa(i)] = "same-value-repeated"
				}
				return m
			}(),
		},
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uid := "shape-" + strconv.Itoa(i)

			// write as snapshot
			snap := &store.Snapshot{Object: tc.data, Time: time.Now()}
			if err := s.SetSnapshot(ctx, uid, snap); err != nil {
				t.Fatalf("set snapshot: %v", err)
			}

			// write as patch
			patch := &store.Patch{PreviousID: snap.ID, Patch: tc.data, Time: time.Now()}
			if err := s.SetPatch(ctx, uid, patch); err != nil {
				t.Fatalf("set patch: %v", err)
			}

			// read back snapshot
			gotSnap, _, err := s.Get(ctx, uid, snap.ID)
			if err != nil {
				t.Fatalf("get snapshot: %v", err)
			}
			if !equalMaps(t, gotSnap.Object, tc.data) {
				t.Fatalf("snapshot mismatch")
			}

			// read back patch
			_, gotPatch, err := s.Get(ctx, uid, patch.ID)
			if err != nil {
				t.Fatalf("get patch: %v", err)
			}
			if !equalMaps(t, gotPatch.Patch, tc.data) {
				t.Fatalf("patch mismatch")
			}

			// verify metadata survived
			if gotSnap.ID != snap.ID {
				t.Fatalf("snapshot ID: want %d got %d", snap.ID, gotSnap.ID)
			}
			if gotPatch.ID != patch.ID {
				t.Fatalf("patch ID: want %d got %d", patch.ID, gotPatch.ID)
			}
			if gotPatch.PreviousID != patch.PreviousID {
				t.Fatalf("patch PreviousID: want %d got %d", patch.PreviousID, gotPatch.PreviousID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Multi-object key isolation: different UIDs must never cross-contaminate.
// ---------------------------------------------------------------------------

func TestMultiObjectIsolation(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			s := openStore(t, mode.opts)

			const objects = 10
			const revisions = 16

			// write unique data per object
			for i := range objects {
				uid := "obj-" + strconv.Itoa(i)
				for j := range revisions {
					data := diffmap.DiffMap{
						"owner": uid,
						"seq":   j,
					}
					if j == 0 {
						_ = s.SetSnapshot(ctx, uid, &store.Snapshot{Object: data})
					} else {
						_ = s.SetPatch(ctx, uid, &store.Patch{
							PreviousID: store.RevisionID(j - 1),
							Patch:      data,
						})
					}
				}
			}

			// verify isolation: every entry must contain the correct owner
			for i := range objects {
				uid := "obj-" + strconv.Itoa(i)
				for j := range revisions {
					snap, patch, err := s.Get(ctx, uid, store.RevisionID(j))
					if err != nil {
						t.Fatalf("get %s/%d: %v", uid, j, err)
					}

					var data diffmap.DiffMap
					if snap != nil {
						data = snap.Object
					} else {
						data = patch.Patch
					}

					if data["owner"] != uid {
						t.Fatalf("cross-contamination: %s/%d has owner=%v", uid, j, data["owner"])
					}
					if gotSeq, ok := toFloat64(data["seq"]); !ok || gotSeq != float64(j) {
						t.Fatalf("wrong seq: %s/%d has seq=%v", uid, j, data["seq"])
					}
				}

				latest, _ := s.GetLatestRevision(ctx, uid)
				if int(latest) != revisions-1 {
					t.Fatalf("latest for %s: want %d got %d", uid, revisions-1, latest)
				}
			}

			// walk and verify all entries
			seen := map[string]int{}
			err := s.WalkObjectRevisions(func(
				uid string,
				revID store.RevisionID,
				snap *store.Snapshot,
				patch *store.Patch,
			) bool {
				var data diffmap.DiffMap
				if snap != nil {
					data = snap.Object
				} else {
					data = patch.Patch
				}
				if data["owner"] != uid {
					t.Errorf("walk: uid=%s rev=%d has owner=%v", uid, revID, data["owner"])
				}
				seen[uid]++
				return true
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			if len(seen) != objects {
				t.Fatalf("walk: expected %d objects, saw %d", objects, len(seen))
			}
			for uid, count := range seen {
				if count != revisions {
					t.Fatalf("walk: %s had %d revisions, want %d", uid, count, revisions)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DB persistence: close and reopen, verify data survives.
// ---------------------------------------------------------------------------

func TestPersistence_CloseReopen(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
		{"periodic_sync", Options{Durable: true, SyncInterval: 10 * time.Millisecond}},
		{"periodic_sync_compressed", Options{Durable: true, SyncInterval: 10 * time.Millisecond, Compress: true}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "persist.bb")

			// phase 1: write
			s1, err := NewWithOptions(path, mode.opts)
			if err != nil {
				t.Fatalf("open phase 1: %v", err)
			}

			snap := &store.Snapshot{
				Object: diffmap.DiffMap{"persistent": "data", "n": 42},
				Time:   time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
			}
			if err := s1.SetSnapshot(ctx, id, snap); err != nil {
				t.Fatalf("phase 1 snapshot: %v", err)
			}

			patch := &store.Patch{
				PreviousID: snap.ID,
				Patch:      diffmap.DiffMap{"n": 43},
				Time:       time.Date(2025, 6, 15, 12, 1, 0, 0, time.UTC),
			}
			if err := s1.SetPatch(ctx, id, patch); err != nil {
				t.Fatalf("phase 1 patch: %v", err)
			}

			// wait for a sync tick if periodic
			if mode.opts.SyncInterval > 0 {
				time.Sleep(mode.opts.SyncInterval * 2)
			}
			if err := s1.Close(); err != nil {
				t.Fatalf("close phase 1: %v", err)
			}

			// phase 2: reopen and verify
			s2, err := NewWithOptions(path, mode.opts)
			if err != nil {
				t.Fatalf("open phase 2: %v", err)
			}
			defer func() { _ = s2.Close() }()

			latest, err := s2.GetLatestRevision(ctx, id)
			if err != nil {
				t.Fatalf("phase 2 latest: %v", err)
			}
			if latest != 1 {
				t.Fatalf("phase 2 latest: want 1 got %d", latest)
			}

			gotSnap, _, err := s2.Get(ctx, id, 0)
			if err != nil {
				t.Fatalf("phase 2 get rev 0: %v", err)
			}
			if gotSnap.Object["persistent"] != "data" {
				t.Fatalf("phase 2 snapshot data wrong: %v", gotSnap.Object)
			}
			if n, ok := toFloat64(gotSnap.Object["n"]); !ok || n != 42 {
				t.Fatalf("phase 2 snapshot n: want 42 got %v", gotSnap.Object["n"])
			}

			_, gotPatch, err := s2.Get(ctx, id, 1)
			if err != nil {
				t.Fatalf("phase 2 get rev 1: %v", err)
			}
			if n, ok := toFloat64(gotPatch.Patch["n"]); !ok || n != 43 {
				t.Fatalf("phase 2 patch n: want 43 got %v", gotPatch.Patch["n"])
			}
			if gotPatch.PreviousID != 0 {
				t.Fatalf("phase 2 patch PreviousID: want 0 got %d", gotPatch.PreviousID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Walk completeness: every entry returned by Walk matches individual Get.
// ---------------------------------------------------------------------------

func TestWalkMatchesGet(t *testing.T) {
	modes := []struct {
		name string
		opts Options
	}{
		{"plain", Options{}},
		{"compressed", Options{Compress: true}},
	}

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			s := openStore(t, mode.opts)

			// write 3 objects, 10 revisions each (mix of snaps and patches)
			for i := range 3 {
				uid := "walk-" + strconv.Itoa(i)
				for j := range 10 {
					data := diffmap.DiffMap{"uid": uid, "j": j}
					if j%5 == 0 {
						_ = s.SetSnapshot(ctx, uid, &store.Snapshot{Object: data})
					} else {
						_ = s.SetPatch(ctx, uid, &store.Patch{
							PreviousID: store.RevisionID(j - 1),
							Patch:      data,
						})
					}
				}
			}

			// walk and compare each entry with a direct Get
			err := s.WalkObjectRevisions(func(
				uid string,
				revID store.RevisionID,
				wSnap *store.Snapshot,
				wPatch *store.Patch,
			) bool {
				gSnap, gPatch, err := s.Get(ctx, uid, revID)
				if err != nil {
					t.Errorf("Get(%s, %d) during walk: %v", uid, revID, err)
					return false
				}

				// both should agree on type
				if (wSnap != nil) != (gSnap != nil) {
					t.Errorf("%s/%d: walk says snapshot=%v, Get says snapshot=%v", uid, revID, wSnap != nil, gSnap != nil)
					return false
				}
				if (wPatch != nil) != (gPatch != nil) {
					t.Errorf("%s/%d: walk says patch=%v, Get says patch=%v", uid, revID, wPatch != nil, gPatch != nil)
					return false
				}

				if wSnap != nil {
					if !equalMaps(t, wSnap.Object, gSnap.Object) {
						t.Errorf("%s/%d: snapshot data mismatch", uid, revID)
					}
					if wSnap.ID != gSnap.ID || wSnap.PreviousID != gSnap.PreviousID {
						t.Errorf("%s/%d: snapshot metadata mismatch", uid, revID)
					}
				}
				if wPatch != nil {
					if !equalMaps(t, wPatch.Patch, gPatch.Patch) {
						t.Errorf("%s/%d: patch data mismatch", uid, revID)
					}
					if wPatch.ID != gPatch.ID || wPatch.PreviousID != gPatch.PreviousID {
						t.Errorf("%s/%d: patch metadata mismatch", uid, revID)
					}
				}
				return true
			})
			if err != nil {
				t.Fatalf("walk error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge cases: empty maps, nil values, very long UIDs, missing revisions.
// ---------------------------------------------------------------------------

func TestEdgeCases(t *testing.T) {
	t.Run("empty_diffmap", func(t *testing.T) {
		s := openStore(t, Options{Compress: true})

		snap := &store.Snapshot{Object: diffmap.DiffMap{}}
		if err := s.SetSnapshot(ctx, id, snap); err != nil {
			t.Fatalf("set: %v", err)
		}
		got, _, err := s.Get(ctx, id, 0)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if len(got.Object) != 0 {
			t.Fatalf("expected empty map, got %v", got.Object)
		}
	})

	t.Run("nil_patch_field", func(t *testing.T) {
		s := openStore(t, Options{Compress: true})

		snap := &store.Snapshot{Object: diffmap.DiffMap{"a": 1}}
		_ = s.SetSnapshot(ctx, id, snap)

		p := &store.Patch{PreviousID: 0, Patch: nil}
		if err := s.SetPatch(ctx, id, p); err != nil {
			t.Fatalf("set: %v", err)
		}
		_, got, err := s.Get(ctx, id, 1)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if len(got.Patch) != 0 {
			t.Fatalf("expected nil/empty patch, got %v", got.Patch)
		}
	})

	t.Run("very_long_uid", func(t *testing.T) {
		s := openStore(t, Options{Compress: true})

		longUID := strings.Repeat("a", 1000)
		snap := &store.Snapshot{Object: diffmap.DiffMap{"uid": longUID}}
		if err := s.SetSnapshot(ctx, longUID, snap); err != nil {
			t.Fatalf("set: %v", err)
		}
		got, _, err := s.Get(ctx, longUID, 0)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Object["uid"] != longUID {
			t.Fatalf("UID mismatch after roundtrip")
		}
	})

	t.Run("get_missing_revision", func(t *testing.T) {
		s := openStore(t, Options{})
		_, _, err := s.Get(ctx, "nonexistent", 999)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("get_latest_missing_object", func(t *testing.T) {
		s := openStore(t, Options{})
		_, err := s.GetLatestRevision(ctx, "nonexistent")
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("walk_empty_store", func(t *testing.T) {
		s := openStore(t, Options{})
		count := 0
		err := s.WalkObjectRevisions(func(_ string, _ store.RevisionID, _ *store.Snapshot, _ *store.Patch) bool {
			count++
			return true
		})
		if err != nil {
			t.Fatalf("walk error: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 entries, got %d", count)
		}
	})
}

// ---------------------------------------------------------------------------
// Codec pool correctness: rapid sequential marshal/unmarshal must not
// produce cross-contamination between calls.
// ---------------------------------------------------------------------------

func TestCodecPoolCorrectness(t *testing.T) {
	codec := store.DefaultCodec

	type testCase struct {
		snap store.Snapshot
		pat  store.Patch
	}

	cases := make([]testCase, 50)
	for i := range cases {
		cases[i] = testCase{
			snap: store.Snapshot{
				ID:         store.RevisionID(i),
				PreviousID: store.RevisionID(i + 100),
				Object:     diffmap.DiffMap{"i": i, "s": strings.Repeat("x", i+1)},
				Time:       time.Date(2025, 1, 1+i, 0, 0, 0, 0, time.UTC),
			},
			pat: store.Patch{
				ID:         store.RevisionID(i + 1000),
				PreviousID: store.RevisionID(i + 2000),
				Patch:      diffmap.DiffMap{"p": i, "d": float64(i) * 0.1},
				Time:       time.Date(2025, 6, 1+i, 0, 0, 0, 0, time.UTC),
			},
		}
	}

	// marshal all, then unmarshal all, verify no mixing
	snapBytes := make([][]byte, len(cases))
	patchBytes := make([][]byte, len(cases))
	for i, tc := range cases {
		var err error
		snapBytes[i], err = codec.Marshal(&tc.snap)
		if err != nil {
			t.Fatalf("marshal snap %d: %v", i, err)
		}
		patchBytes[i], err = codec.Marshal(&tc.pat)
		if err != nil {
			t.Fatalf("marshal patch %d: %v", i, err)
		}
	}

	for i, tc := range cases {
		var gotSnap store.Snapshot
		if err := codec.Unmarshal(snapBytes[i], &gotSnap); err != nil {
			t.Fatalf("unmarshal snap %d: %v", i, err)
		}
		if gotSnap.ID != tc.snap.ID {
			t.Fatalf("snap %d ID: want %d got %d", i, tc.snap.ID, gotSnap.ID)
		}
		if gotSnap.PreviousID != tc.snap.PreviousID {
			t.Fatalf("snap %d PreviousID: want %d got %d", i, tc.snap.PreviousID, gotSnap.PreviousID)
		}
		if !equalMaps(t, gotSnap.Object, tc.snap.Object) {
			t.Fatalf("snap %d Object mismatch", i)
		}

		var gotPatch store.Patch
		if err := codec.Unmarshal(patchBytes[i], &gotPatch); err != nil {
			t.Fatalf("unmarshal patch %d: %v", i, err)
		}
		if gotPatch.ID != tc.pat.ID {
			t.Fatalf("patch %d ID: want %d got %d", i, tc.pat.ID, gotPatch.ID)
		}
		if !equalMaps(t, gotPatch.Patch, tc.pat.Patch) {
			t.Fatalf("patch %d Patch mismatch", i)
		}
	}
}

// TestCodecPoolConcurrency fires many goroutines doing marshal+unmarshal
// on different data to check for pool cross-contamination under contention.
func TestCodecPoolConcurrency(t *testing.T) {
	codec := store.DefaultCodec
	const goroutines = 16
	const iterations = 200

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range iterations {
				unique := g*iterations + i
				snap := store.Snapshot{
					ID:     store.RevisionID(unique),
					Object: diffmap.DiffMap{"g": g, "i": i, "u": unique},
				}
				data, err := codec.Marshal(&snap)
				if err != nil {
					t.Errorf("g%d i%d: marshal: %v", g, i, err)
					return
				}
				var got store.Snapshot
				if err := codec.Unmarshal(data, &got); err != nil {
					t.Errorf("g%d i%d: unmarshal: %v", g, i, err)
					return
				}
				if got.ID != snap.ID {
					t.Errorf("g%d i%d: ID want %d got %d", g, i, snap.ID, got.ID)
					return
				}
				if !equalMaps(t, got.Object, snap.Object) {
					t.Errorf("g%d i%d: Object mismatch: got %v want %v", g, i, got.Object, snap.Object)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Periodic sync mode: verify writes are actually persisted to disk.
// ---------------------------------------------------------------------------

func TestPeriodicSync_DataPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync.bb")

	s, err := NewWithOptions(path, Options{
		Durable:      true,
		SyncInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// write some data
	for i := range 20 {
		_ = s.SetPatch(ctx, id, &store.Patch{
			PreviousID: store.RevisionID(i),
			Patch:      diffmap.DiffMap{"val": i},
		})
	}

	// wait for sync to happen
	time.Sleep(30 * time.Millisecond)
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// reopen and verify
	s2, err := NewWithOptions(path, Options{
		Durable:      true,
		SyncInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = s2.Close() }()

	latest, err := s2.GetLatestRevision(ctx, id)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest != 19 {
		t.Fatalf("latest: want 19 got %d", latest)
	}

	// spot check a few values
	for _, rev := range []int{0, 5, 10, 19} {
		_, p, err := s2.Get(ctx, id, store.RevisionID(rev))
		if err != nil {
			t.Fatalf("get rev %d: %v", rev, err)
		}
		if p == nil {
			t.Fatalf("rev %d: expected patch", rev)
		}
		if v, ok := toFloat64(p.Patch["val"]); !ok || v != float64(rev) {
			t.Fatalf("rev %d: val want %d got %v", rev, rev, p.Patch["val"])
		}
	}
}

// ---------------------------------------------------------------------------
// Compressed store: verify DB is actually smaller than uncompressed.
// ---------------------------------------------------------------------------

func TestCompression_ReducesSize(t *testing.T) {
	dir := t.TempDir()
	pathPlain := filepath.Join(dir, "plain.bb")
	pathComp := filepath.Join(dir, "compressed.bb")

	write := func(path string, compress bool) {
		s, err := NewWithOptions(path, Options{Compress: compress})
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		// write highly compressible data
		for i := range 50 {
			data := diffmap.DiffMap{
				"kind":       "ConfigMap",
				"apiVersion": "v1",
				"metadata": diffmap.DiffMap{
					"name":      "my-config-" + strconv.Itoa(i),
					"namespace": "default",
					"uid":       "uid-" + strconv.Itoa(i),
				},
				"data": diffmap.DiffMap{
					"config.yaml": strings.Repeat("key: value\n", 50),
				},
			}
			_ = s.SetSnapshot(ctx, "uid-"+strconv.Itoa(i), &store.Snapshot{Object: data})
		}
		_ = s.Close()
	}

	write(pathPlain, false)
	write(pathComp, true)

	plainInfo, _ := os.Stat(pathPlain)
	compInfo, _ := os.Stat(pathComp)

	if compInfo.Size() >= plainInfo.Size() {
		t.Fatalf("compressed (%d bytes) should be smaller than plain (%d bytes)",
			compInfo.Size(), plainInfo.Size())
	}

	ratio := float64(plainInfo.Size()) / float64(compInfo.Size())
	t.Logf("compression ratio: %.1fx (%d bytes -> %d bytes)", ratio, plainInfo.Size(), compInfo.Size())
}

// == Benchmarks ============================================================

func benchStoreOpts(b *testing.B, opts Options) *Store {
	b.Helper()
	s, err := NewWithOptions(filepath.Join(b.TempDir(), "bench.bb"), opts)
	if err != nil {
		b.Fatalf("open: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

func benchStore(b *testing.B, durable bool) *Store {
	b.Helper()
	return benchStoreOpts(b, Options{Durable: durable})
}

func largePatch() *store.Patch {
	m := make(diffmap.DiffMap, 20)
	for i := range 20 {
		m["field-"+strconv.Itoa(i)] = strings.Repeat("x", 50)
	}
	return &store.Patch{PreviousID: 0, Patch: m, Time: time.Now()}
}

func largeSnapshot() *store.Snapshot {
	m := make(diffmap.DiffMap, 500)
	for i := range 500 {
		v := strings.Repeat(string(rune(i%26+65)), 26)
		m[v] = v
	}
	return &store.Snapshot{PreviousID: 0, Object: m, Time: time.Now()}
}

func BenchmarkMarshalPatch(b *testing.B) {
	p := largePatch()
	codec := store.DefaultCodec
	for range b.N {
		if _, err := codec.Marshal(p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalSnapshot(b *testing.B) {
	s := largeSnapshot()
	codec := store.DefaultCodec
	for range b.N {
		if _, err := codec.Marshal(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalPatch(b *testing.B) {
	p := largePatch()
	codec := store.DefaultCodec
	data, _ := codec.Marshal(p)
	for range b.N {
		var out store.Patch
		if err := codec.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalSnapshot(b *testing.B) {
	s := largeSnapshot()
	codec := store.DefaultCodec
	data, _ := codec.Marshal(s)
	for range b.N {
		var out store.Snapshot
		if err := codec.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkKeyObjectRevision(b *testing.B) {
	uid := "550e8400-e29b-41d4-a716-446655440000"
	for range b.N {
		_ = keyObjectRevision(uid, 12345)
	}
}

func BenchmarkSetPatch(b *testing.B) {
	s := benchStore(b, false)
	p := largePatch()
	for range b.N {
		p.ID = 0
		if err := s.SetPatch(ctx, id, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetPatch_Compressed(b *testing.B) {
	s := benchStoreOpts(b, Options{Compress: true})
	p := largePatch()
	for range b.N {
		p.ID = 0
		if err := s.SetPatch(ctx, id, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetPatch_Durable(b *testing.B) {
	s := benchStore(b, true)
	p := largePatch()
	for range b.N {
		p.ID = 0
		if err := s.SetPatch(ctx, id, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetPatch_PeriodicSync(b *testing.B) {
	s := benchStoreOpts(b, Options{Durable: true, SyncInterval: 50 * time.Millisecond})
	p := largePatch()
	for range b.N {
		p.ID = 0
		if err := s.SetPatch(ctx, id, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetPatch_PeriodicSyncCompressed(b *testing.B) {
	s := benchStoreOpts(b, Options{Durable: true, SyncInterval: 50 * time.Millisecond, Compress: true})
	p := largePatch()
	for range b.N {
		p.ID = 0
		if err := s.SetPatch(ctx, id, p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetSnapshot(b *testing.B) {
	s := benchStore(b, false)
	snap := largeSnapshot()
	for range b.N {
		snap.ID = 0
		if err := s.SetSnapshot(ctx, id, snap); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSetSnapshot_Compressed(b *testing.B) {
	s := benchStoreOpts(b, Options{Compress: true})
	snap := largeSnapshot()
	for range b.N {
		snap.ID = 0
		if err := s.SetSnapshot(ctx, id, snap); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGet(b *testing.B) {
	s := benchStore(b, false)
	for range 100 {
		if err := s.SetPatch(ctx, id, largePatch()); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := range b.N {
		rev := store.RevisionID(i % 100)
		if _, _, err := s.Get(ctx, id, rev); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGet_Compressed(b *testing.B) {
	s := benchStoreOpts(b, Options{Compress: true})
	for range 100 {
		if err := s.SetPatch(ctx, id, largePatch()); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := range b.N {
		rev := store.RevisionID(i % 100)
		if _, _, err := s.Get(ctx, id, rev); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetLatestRevision(b *testing.B) {
	s := benchStore(b, false)
	_ = s.SetPatch(ctx, id, largePatch())
	b.ResetTimer()
	for range b.N {
		_, _ = s.GetLatestRevision(ctx, id)
	}
}

func BenchmarkWalkObjectRevisions(b *testing.B) {
	s := benchStore(b, false)
	for range 200 {
		if err := s.SetPatch(ctx, id, largePatch()); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for range b.N {
		_ = s.WalkObjectRevisions(func(_ string, _ store.RevisionID, _ *store.Snapshot, _ *store.Patch) bool {
			return true
		})
	}
}

func BenchmarkFileSize(b *testing.B) {
	const writes = 200

	run := func(name string, opts Options) {
		b.Run(name, func(b *testing.B) {
			for range b.N {
				dir := b.TempDir()
				path := filepath.Join(dir, "bench.bb")
				s, err := NewWithOptions(path, opts)
				if err != nil {
					b.Fatal(err)
				}
				for i := range writes {
					if i%8 == 0 {
						_ = s.SetSnapshot(ctx, id, largeSnapshot())
					} else {
						_ = s.SetPatch(ctx, id, largePatch())
					}
				}
				_ = s.Close()
				if fi, err := os.Stat(path); err == nil {
					b.ReportMetric(float64(fi.Size())/1024, "KB_db")
				}
			}
		})
	}

	run("plain", Options{})
	run("compressed", Options{Compress: true})
}

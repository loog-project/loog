package service_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/loog-project/loog/internal/service"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/pkg/diffmap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// == Tests =================================================================

func mustNewSvc(t *testing.T, snapshotEvery uint64, durable, withCache bool) (*service.TrackerService, *bboltStore.Store) {
	t.Helper()
	st, err := bboltStore.New(t.TempDir()+"/db.bb", nil, durable)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	svc := service.NewTrackerService(st, snapshotEvery, withCache)
	t.Cleanup(func() { _ = svc.Close(); _ = st.Close() })
	return svc, st
}

func newCM(uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: diffmap.DiffMap{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": diffmap.DiffMap{
				"uid":       uid,
				"namespace": "default",
				"name":      "cm",
			},
			"data": diffmap.DiffMap{"val": "x"},
		},
	}
}

func TestCommitRestore_RotationAndDiff(t *testing.T) {
	ctx := context.Background()
	svc, raw := mustNewSvc(t, 4, true, true)

	uid := "uid-rot"
	obj := newCM(uid)

	// first commit → snapshot rev0
	rev0, _ := svc.Commit(ctx, uid, obj.DeepCopy())
	if rev0 != 0 {
		t.Fatalf("want rev0=0, got %d", rev0)
	}

	// mutate three times → three patches
	for i := 1; i <= 3; i++ {
		obj.Object["data"].(diffmap.DiffMap)["val"] = "x" + strconv.Itoa(i)
		rev, _ := svc.Commit(ctx, uid, obj.DeepCopy())
		if int(rev) != i {
			t.Fatalf("commit %d got rev=%d", i, rev)
		}

		// verify patch stored contains only delta (no full snapshot)
		_, p, _ := raw.Get(ctx, uid, rev)
		if p == nil || p.Patch == nil {
			t.Fatalf("rev%d expected patch, got snapshot", rev)
		}
		if len(p.Patch) != 1 || p.Patch["data"].(diffmap.DiffMap)["val"] != "x"+strconv.Itoa(i) {
			t.Fatalf("patch diff wrong at rev%d: %#v", rev, p.Patch)
		}
	}

	// 4th commit should create snapshot
	obj.Object["data"].(diffmap.DiffMap)["val"] = "final"
	rev4, _ := svc.Commit(ctx, uid, obj.DeepCopy())
	if rev4 != 4 {
		t.Fatalf("snapshot expected at rev4, got %d", rev4)
	}
	snap4, p4, _ := raw.Get(ctx, uid, rev4)
	if snap4 == nil || p4 != nil {
		t.Fatalf("rev4 should be snapshot, got patch=%v snap=%v", p4, snap4)
	}

	// restore revision 2 and verify content
	snap2, _ := svc.Restore(ctx, uid, 2)
	got := snap2.Object["data"].(diffmap.DiffMap)["val"]
	if got != "x2" {
		t.Fatalf("restore rev2 wrong: want x2, got %v", got)
	}
}

func TestHotCache_FastPath(t *testing.T) {
	ctx := context.Background()
	svc, _ := mustNewSvc(t, 8, true, true)

	uid := "uid-cache"
	obj := newCM(uid)

	// first commit fills cache
	_, _ = svc.Commit(ctx, uid, obj.DeepCopy())

	// mutate & commit again
	obj.Object["data"].(diffmap.DiffMap)["val"] = "cached"
	_, _ = svc.Commit(ctx, uid, obj.DeepCopy())

	// inspect private cache length via reflection
	val := reflect.ValueOf(svc).Elem().FieldByName("cache")
	keys := reflect.Indirect(val).FieldByName("data").MapKeys()
	if len(keys) != 1 {
		t.Fatalf("cache entry missing, got %d", len(keys))
	}
}

func TestTrackerService_ConcurrentCommits(t *testing.T) {
	ctx := context.Background()
	svc, raw := mustNewSvc(t, 8, true, false)

	uid := "uid-conc"
	obj := newCM(uid)

	const workers = 12
	const loops = 40

	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(id int) {
			defer wg.Done()

			local := obj.DeepCopy()
			for i := 0; i < loops; i++ {
				local.Object["data"].(diffmap.DiffMap)["val"] = id*100 + i
				if _, err := svc.Commit(ctx, uid, local); err != nil {
					t.Errorf("worker %d: %v", id, err)
				}
			}
		}(w)
	}
	wg.Wait()

	// latest revision = workers*loops -1
	want := workers*loops - 1
	latest, _ := raw.GetLatestRevision(ctx, uid)
	if int(latest) != want {
		t.Fatalf("latest want %d got %d", want, latest)
	}

	// leak check: janitor + GC + main should stay under 8 goroutines
	if g := runtime.NumGoroutine(); g > 8 {
		t.Fatalf("goroutine leak: %d still running", g)
	}
}

// == Benchmarks ============================================================

// snapshot every 1

func BenchmarkCommit_SnapshotEvery1DurableNoCache(b *testing.B) {
	benchCommit(b, 1, true, false)
}

func BenchmarkCommit_SnapshotEvery1DurableWithCache(b *testing.B) {
	benchCommit(b, 1, true, true)
}

func BenchmarkCommit_SnapshotEvery1(b *testing.B) {
	benchCommit(b, 1, false, false)
}

// snapshot every 2

func BenchmarkCommit_SnapshotEvery2DurableNoCache(b *testing.B) {
	benchCommit(b, 2, true, false)
}

func BenchmarkCommit_SnapshotEvery2DurableWithCache(b *testing.B) {
	benchCommit(b, 2, true, true)
}

func BenchmarkCommit_SnapshotEvery2(b *testing.B) {
	benchCommit(b, 2, false, false)
}

// snapshot every 4

func BenchmarkCommit_SnapshotEvery4DurableNoCache(b *testing.B) {
	benchCommit(b, 4, true, false)
}

func BenchmarkCommit_SnapshotEvery4DurableWithCache(b *testing.B) {
	benchCommit(b, 4, true, true)
}

func BenchmarkCommit_SnapshotEvery4(b *testing.B) {
	benchCommit(b, 4, false, false)
}

// snapshot every 8

func BenchmarkCommit_SnapshotEvery8DurableNoCache(b *testing.B) {
	benchCommit(b, 8, true, false)
}

func BenchmarkCommit_SnapshotEvery8DurableWithCache(b *testing.B) {
	benchCommit(b, 8, true, true)
}

func BenchmarkCommit_SnapshotEvery8(b *testing.B) {
	benchCommit(b, 8, false, false)
}

// snapshot every 16

func BenchmarkCommit_SnapshotEvery16DurableNoCache(b *testing.B) {
	benchCommit(b, 16, true, false)
}

func BenchmarkCommit_SnapshotEvery16DurableWithCache(b *testing.B) {
	benchCommit(b, 16, true, true)
}

func BenchmarkCommit_SnapshotEvery16(b *testing.B) {
	benchCommit(b, 16, false, false)
}

// snapshot every 32

func BenchmarkCommit_SnapshotEvery32DurableNoCache(b *testing.B) {
	benchCommit(b, 32, true, false)
}

func BenchmarkCommit_SnapshotEvery32DurableWithCache(b *testing.B) {
	benchCommit(b, 32, true, true)
}

func BenchmarkCommit_SnapshotEvery32(b *testing.B) {
	benchCommit(b, 32, false, false)
}

// snapshot every 64

func BenchmarkCommit_SnapshotEvery64DurableNoCache(b *testing.B) {
	benchCommit(b, 64, true, false)
}

func BenchmarkCommit_SnapshotEvery64DurableWithCache(b *testing.B) {
	benchCommit(b, 64, true, true)
}

func BenchmarkCommit_SnapshotEvery64(b *testing.B) {
	benchCommit(b, 64, false, false)
}

// benchCommit is the shared benchmark body.
func benchCommit(b *testing.B, snapshotEvery uint64, durable, withCache bool) {
	tempDir := b.TempDir()
	dbPath := fmt.Sprintf("%s/bench-%d.db", tempDir, snapshotEvery)

	db, err := bboltStore.New(dbPath, nil, durable)
	if err != nil {
		b.Fatalf("init store: %v", err)
	}
	defer func(store *bboltStore.Store) {
		_ = store.Close()
	}(db)

	svc := service.NewTrackerService(db, snapshotEvery, withCache)

	// make this object large
	m := map[string]any{}
	for i := 0; i < 500; i++ {
		v := strings.Repeat(string(rune(i+65)), 26)
		m[v] = v
	}

	// base object – simple unstructured with metadata.name mutated each loop.
	base := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"uid":        "bench-uid",
			"namespace":  "default",
			"name":       "cm-0",
			"generation": int64(1),
		},
		"data": m,
	}}

	objectID := "bench-uid"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// mutate name + generation each commit
		meta := base.Object["metadata"].(map[string]any)
		meta["name"] = "cm-" + strconv.Itoa(i)
		meta["generation"] = int64(i + 1)

		if _, err := svc.Commit(b.Context(), objectID, base); err != nil {
			b.Fatalf("commit error: %v", err)
		}
	}
	b.StopTimer()

	// record file size for visibility
	if fi, err := os.Stat(dbPath); err == nil {
		b.ReportMetric(float64(fi.Size())/1e3, "KB_db")
	} else {
		b.Fatalf("stat db file: %v", err)
	}
}

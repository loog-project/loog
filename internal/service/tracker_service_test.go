package service_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/loog-project/loog/internal/service"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func BenchmarkCommit_SnapshotEvery1Durable(b *testing.B) {
	benchCommit(b, 1, true)
}

func BenchmarkCommit_SnapshotEvery1(b *testing.B) {
	benchCommit(b, 1, false)
}

func BenchmarkCommit_SnapshotEvery2Durable(b *testing.B) {
	benchCommit(b, 2, true)
}

func BenchmarkCommit_SnapshotEvery2(b *testing.B) {
	benchCommit(b, 2, false)
}

func BenchmarkCommit_SnapshotEvery4Durable(b *testing.B) {
	benchCommit(b, 4, true)
}

func BenchmarkCommit_SnapshotEvery4(b *testing.B) {
	benchCommit(b, 4, false)
}

func BenchmarkCommit_SnapshotEvery8Durable(b *testing.B) {
	benchCommit(b, 8, true)
}

func BenchmarkCommit_SnapshotEvery8(b *testing.B) {
	benchCommit(b, 8, false)
}

func BenchmarkCommit_SnapshotEvery16Durable(b *testing.B) {
	benchCommit(b, 16, true)
}

func BenchmarkCommit_SnapshotEvery16(b *testing.B) {
	benchCommit(b, 16, false)
}

func BenchmarkCommit_SnapshotEvery32Durable(b *testing.B) {
	benchCommit(b, 32, true)
}

func BenchmarkCommit_SnapshotEvery32(b *testing.B) {
	benchCommit(b, 32, false)
}

// benchCommit is the shared benchmark body.
func benchCommit(b *testing.B, snapshotEvery uint64, durable bool) {
	tempDir := b.TempDir()
	dbPath := fmt.Sprintf("%s/bench-%d.db", tempDir, snapshotEvery)

	store, err := bboltStore.New(dbPath, nil, durable)
	if err != nil {
		b.Fatalf("init store: %v", err)
	}
	defer func(store *bboltStore.Store) {
		_ = store.Close()
	}(store)

	svc := service.NewTrackerService(store, snapshotEvery)

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

package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/store/segment"
	"github.com/loog-project/loog/pkg/diffmap"
)

// writeSegmentDir creates a directory with one closed segment file containing a
// couple of timestamped snapshots, then returns a live store opened on that dir
// (which starts a fresh active segment; the pre-written data is in a rolled one).
func writeSegmentDir(t *testing.T) *segment.Store {
	t.Helper()
	dir := t.TempDir()

	st, err := segment.Open(dir, segment.Options{Interval: time.Hour, Bolt: bboltStore.Options{Durable: true}})
	if err != nil {
		t.Fatalf("open segment store: %v", err)
	}
	now := time.Now().UTC()
	snap := func(uid, val string, ts time.Time, prev store.RevisionID) {
		s := &store.Snapshot{
			PreviousID: prev,
			Object: diffmap.DiffMap{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   diffmap.DiffMap{"uid": uid, "name": "cm-" + uid, "namespace": "default"},
				"data":       diffmap.DiffMap{"val": val},
			},
			Time: ts,
		}
		if err := st.SetSnapshot(context.Background(), uid, s); err != nil {
			t.Fatalf("set snapshot: %v", err)
		}
	}
	// Recent-past timestamps so a "since=1h" window (ending now) includes them.
	snap("a", "1", now.Add(-3*time.Minute), 0)
	snap("a", "2", now.Add(-2*time.Minute), 1)
	snap("b", "1", now.Add(-1*time.Minute), 0)
	// Roll so the data lands in a rolled segment; the store keeps a fresh
	// (empty) active segment. The store stays open (live) for the server.
	if err := st.Rotate(); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestServer_Segments(t *testing.T) {
	st := writeSegmentDir(t)
	srv := httptest.NewServer(NewServer(st, "All()", false, zerolog.Nop()).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/segments")
	if err != nil {
		t.Fatalf("GET /segments: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body segmentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Segments) != 1 {
		t.Fatalf("expected 1 rolled segment, got %d", len(body.Segments))
	}
}

func TestServer_Extract(t *testing.T) {
	st := writeSegmentDir(t)
	srv := httptest.NewServer(NewServer(st, "All()", false, zerolog.Nop()).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/extract?since=1h")
	if err != nil {
		t.Fatalf("GET /extract: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, string(b))
	}
	if got := resp.Header.Get("X-Loog-Revisions"); got != "3" {
		t.Fatalf("X-Loog-Revisions = %q, want 3", got)
	}
	if got := resp.Header.Get("X-Loog-Resources"); got != "2" {
		t.Fatalf("X-Loog-Resources = %q, want 2", got)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("empty extract body")
	}
}

func TestServer_ExtractBadRange(t *testing.T) {
	st := writeSegmentDir(t)
	srv := httptest.NewServer(NewServer(st, "All()", false, zerolog.Nop()).Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/extract?since=notaduration")
	if err != nil {
		t.Fatalf("GET /extract: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

package bbolt

import (
	"context"
	"testing"

	"github.com/loog-project/loog/internal/store"
)

// A store written with compression must be readable even when reopened without
// the Compress flag set (this is what --replay does: it can't know the original
// setting). Previously this produced "msgpack: unexpected code=... decoding map
// length" because the reader fed S2 bytes straight to the decoder.
func TestStore_CompressionAutodetect(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name       string
		writeCompr bool
	}{
		{"written compressed", true},
		{"written uncompressed", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir() + "/capture.loog"

			w, err := NewWithOptions(path, Options{Compress: tc.writeCompr})
			if err != nil {
				t.Fatalf("open write: %v", err)
			}
			snap := &store.Snapshot{
				Object: map[string]any{
					"kind":     "ConfigMap",
					"metadata": map[string]any{"uid": "u1", "name": "example-name"},
					"data":     map[string]any{"key": "some-value-to-compress"},
				},
			}
			if err := w.SetSnapshot(ctx, "u1", snap); err != nil {
				t.Fatalf("SetSnapshot: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("close write: %v", err)
			}

			// Reopen with the OPPOSITE compress flag, as replay (Compress:false)
			// would for a compressed file. Auto-detection must make reads work.
			r, err := NewWithOptions(path, Options{Compress: !tc.writeCompr, ReadOnly: true})
			if err != nil {
				t.Fatalf("open read: %v", err)
			}
			defer func() { _ = r.Close() }()

			var got *store.Snapshot
			err = r.WalkObjectRevisions(func(_ string, _ store.RevisionID, s *store.Snapshot, _ *store.Patch) bool {
				if s != nil {
					got = s
				}
				return true
			})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			if got == nil {
				t.Fatal("no snapshot read back")
			}
			meta, _ := got.Object["metadata"].(map[string]any)
			if meta == nil || meta["name"] != "example-name" {
				t.Errorf("round-trip mismatch: %#v", got.Object)
			}
		})
	}
}

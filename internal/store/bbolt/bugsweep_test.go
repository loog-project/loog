package bbolt

import (
	"testing"
	"time"
)

// Close must be idempotent even when a periodic sync goroutine is running,
// and must not panic on a second call (previously close-of-closed-channel).
func TestStore_CloseIdempotentWithSync(t *testing.T) {
	s, err := NewWithOptions(t.TempDir()+"/db.bb", Options{
		Durable:      true,
		SyncInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Let the sync goroutine tick at least once.
	time.Sleep(15 * time.Millisecond)

	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second Close must be a no-op, not a panic.
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

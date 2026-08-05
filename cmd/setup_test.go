package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupProduction_ErrorPathDoesNotPanic guards against a regression where
// setupProduction's error paths did:
//
//	return cleanup, nil, nil, nil, nil, err
//
// which zeroed the named-return variables (trackerService, rps, m). The
// cleanup closures capture those variables, so a subsequent `defer cleanup()`
// in the caller would deref nil and panic instead of releasing resources and
// surfacing the real error.
func TestSetupProduction_ErrorPathDoesNotPanic(t *testing.T) {
	dir := t.TempDir()

	// Save + restore every package flag setupProduction reads.
	saved := struct {
		filterExpr       string
		kubeConfigPath   string
		outputFile       string
		snapshotInterval uint64
		noDurableSync    bool
		disableCache     bool
		disableCompress  bool
		headlessMode     bool
	}{
		filterExpr:       filterExpr,
		kubeConfigPath:   kubeConfigPath,
		outputFile:       outputFile,
		snapshotInterval: snapshotInterval,
		noDurableSync:    noDurableSync,
		disableCache:     disableCache,
		disableCompress:  disableCompress,
		headlessMode:     headlessMode,
	}
	t.Cleanup(func() {
		filterExpr = saved.filterExpr
		kubeConfigPath = saved.kubeConfigPath
		outputFile = saved.outputFile
		snapshotInterval = saved.snapshotInterval
		noDurableSync = saved.noDurableSync
		disableCache = saved.disableCache
		disableCompress = saved.disableCompress
		headlessMode = saved.headlessMode
	})

	filterExpr = "All()"
	kubeConfigPath = filepath.Join(dir, "definitely-not-a-kubeconfig")
	outputFile = filepath.Join(dir, "capture.loog")
	snapshotInterval = 8
	noDurableSync = false
	disableCache = false
	disableCompress = false
	headlessMode = false

	cleanup, _, ts, rps, _, err := setupProduction(context.Background(), nil)
	if err == nil {
		t.Fatalf("expected setupProduction to fail with a bogus kubeconfig")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "kubeconfig") {
		t.Fatalf("expected kubeconfig error, got: %v", err)
	}
	if cleanup == nil {
		t.Fatalf("cleanup must be non-nil so callers can safely `defer cleanup()`")
	}

	// The two named returns that MUST NOT be zeroed on error. Their cleanup
	// closures capture them; nil'ing them is what caused the panic.
	if ts == nil {
		t.Errorf("trackerService returned nil after failure - cleanup closure would panic")
	}
	if rps == nil {
		t.Errorf("rps returned nil after failure - cleanup closure would panic")
	}

	// The real regression assertion: cleanup must run to completion without
	// panicking, no matter which step failed.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("cleanup panicked: %v", r)
		}
	}()
	cleanup()

	// Idempotent: a second cleanup call (e.g. via nested defer) must also
	// be safe. Close methods on the store and tracker service are guarded by
	// sync.Once and a nil-receiver check.
	cleanup()
}

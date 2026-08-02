package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// resetFlags clears the package-level flag vars that validateArgsAndFlags reads,
// so each case starts from a known state.
func resetFlags() {
	outputFile = ""
	appendOutput = false
	replayFile = ""
	headlessMode = false
	simulateMode = false
}

func TestValidateArgsAndFlags(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "capture.loog")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "does-not-exist.loog")
	fresh := filepath.Join(dir, "new.loog")

	tests := []struct {
		name    string
		setup   func()
		args    []string
		wantErr bool
	}{
		{
			name:    "replay existing file",
			setup:   func() { replayFile = existing },
			wantErr: false,
		},
		{
			name:    "replay missing file",
			setup:   func() { replayFile = missing },
			wantErr: true,
		},
		{
			name:    "replay with output is rejected",
			setup:   func() { replayFile = existing; outputFile = fresh },
			wantErr: true,
		},
		{
			name:    "replay with resource args is rejected",
			setup:   func() { replayFile = existing },
			args:    []string{"v1/pods"},
			wantErr: true,
		},
		{
			name:    "output to new file",
			setup:   func() { outputFile = fresh },
			wantErr: false,
		},
		{
			name:    "output to existing file without append is rejected",
			setup:   func() { outputFile = existing },
			wantErr: true,
		},
		{
			name:    "output to existing file with append",
			setup:   func() { outputFile = existing; appendOutput = true },
			wantErr: false,
		},
		{
			name:    "no args and no output is rejected",
			setup:   func() {},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			tt.setup()
			err := validateArgsAndFlags(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateArgsAndFlags() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
	resetFlags()
}

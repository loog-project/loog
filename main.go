package main

import "github.com/loog-project/loog/cmd"

// Build metadata, injected at release time via -ldflags. Defaults are used for
// `go install` / local builds. See .goreleaser.yaml and the Makefile.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	cmd.Execute()
}

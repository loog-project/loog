package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build metadata, populated by SetVersionInfo from main's -ldflags values.
var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// SetVersionInfo records the build metadata injected via -ldflags in main and
// wires it into `loog --version`.
func SetVersionInfo(version, commit, date string) {
	buildVersion = version
	buildCommit = commit
	buildDate = date
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("loog {{ .Version }}\n")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit, and build information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(),
			"loog %s\n  commit:  %s\n  built:   %s\n  go:      %s\n  os/arch: %s/%s\n",
			buildVersion, buildCommit, buildDate,
			runtime.Version(), runtime.GOOS, runtime.GOARCH,
		)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

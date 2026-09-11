package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/spf13/cobra"

	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/store/segment"
	"github.com/loog-project/loog/internal/util"
)

var (
	extractDir      string
	extractFrom     string
	extractTo       string
	extractSince    string
	extractFilter   string
	extractOutput   string
	extractReplay   bool
	extractCompress bool
)

var extractCmd = &cobra.Command{
	Use:   "extract [FLAGS]",
	Short: "Extract a time range from recorded segments into a single .loog file",
	Long: `Extract reads the rotating .loog segments produced by 'loog record' (in --dir)
and writes every change within the requested time window into a single, portable
.loog file. Timestamps and delete markers are preserved, so the result replays
exactly like the original capture for that window.

Use this against a directory you can read directly (a mounted PVC, an rsync'd
copy, or 'kubectl cp' of the recorder's data dir). To pull a range from a running
in-cluster recorder over HTTP instead, use 'loog fetch'.

Examples:
  # Everything from the last 2 hours into out.loog and open it
  loog extract --dir /data --since 2h -o out.loog --replay

  # A precise window, only the "payments" namespace
  loog extract --dir /data \
    --from 2026-09-10T14:00:00Z --to 2026-09-10T15:00:00Z \
    -f 'Namespaces("payments")' -o incident.loog`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runExtract(cmd.Context())
	},
}

func init() {
	extractCmd.Flags().StringVar(&extractDir, "dir", "",
		"Directory containing recorded .loog segment files (required)")
	extractCmd.Flags().StringVar(&extractFrom, "from", "",
		"Start of the window (RFC3339 or 'YYYY-MM-DD HH:MM:SS'); empty = beginning")
	extractCmd.Flags().StringVar(&extractTo, "to", "",
		"End of the window (RFC3339 or 'YYYY-MM-DD HH:MM:SS'); empty = now")
	extractCmd.Flags().StringVar(&extractSince, "since", "",
		"Relative window ending now, e.g. 2h or 30m (overrides --from/--to)")
	extractCmd.Flags().StringVarP(&extractFilter, "filter", "f", "All()",
		"Filter expression selecting which resources to include")
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "",
		"Output .loog file (default: a temporary file)")
	extractCmd.Flags().BoolVar(&extractReplay, "replay", false,
		"Open the extracted file in the TUI after extraction")
	extractCmd.Flags().BoolVar(&extractCompress, "compress", true,
		"Compress the output .loog with s2")

	_ = extractCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(extractCmd)
}

func runExtract(ctx context.Context) error {
	setupDebugLogger()

	if info, err := os.Stat(extractDir); err != nil || !info.IsDir() {
		return fmt.Errorf("--dir %q is not a directory", extractDir)
	}

	from, to, err := segment.ResolveRange(extractFrom, extractTo, extractSince)
	if err != nil {
		return err
	}

	var program *vm.Program
	if extractFilter != "" && extractFilter != "All()" {
		program, err = expr.Compile(extractFilter, expr.Env(util.EventEntryEnv{}), expr.AsBool())
		if err != nil {
			return fmt.Errorf("compiling filter expression: %w", err)
		}
	}

	out := extractOutput
	if out == "" {
		f, terr := os.CreateTemp("", "loog-extract-*.loog")
		if terr != nil {
			return fmt.Errorf("creating temp output: %w", terr)
		}
		out = f.Name()
		_ = f.Close()
		_ = os.Remove(out) // let bbolt create it fresh
	}

	stats, err := segment.Extract(ctx, extractDir, out, segment.ExtractOptions{
		From:   from,
		To:     to,
		Filter: program,
		Bolt:   bboltStore.Options{Durable: true, Compress: extractCompress},
	})
	if err != nil {
		return fmt.Errorf("extracting: %w", err)
	}

	setupLog.Info().
		Str("output", out).
		Int("segments", stats.SegmentsScanned).
		Int("revisions", stats.RevisionsWritten).
		Int("resources", stats.Resources).
		Msg("Extraction complete")
	_, _ = fmt.Fprintf(os.Stderr, "Wrote %d revisions across %d resources to %s\n",
		stats.RevisionsWritten, stats.Resources, out)

	if extractReplay {
		// runReplayMode reads these package-level vars.
		replayFile = out
		filterExpr = "All()"
		return runReplayMode()
	}
	return nil
}

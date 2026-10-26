package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	fetchServer string
	fetchFrom   string
	fetchTo     string
	fetchSince  string
	fetchFilter string
	fetchOutput string
	fetchReplay bool
)

var fetchCmd = &cobra.Command{
	Use:   "fetch [FLAGS]",
	Short: "Fetch a time range from a running in-cluster recorder over HTTP",
	Long: `Fetch pulls a clipped .loog file from a 'loog record --http' recorder for a
given time window and saves it locally, optionally opening it in the TUI.

Point --server at the recorder's retrieval API. In-cluster, the simplest path is
a port-forward:

  kubectl -n loog port-forward svc/loog-recorder 8080:8080
  loog fetch --server localhost:8080 --since 1h -o out.loog --replay

Examples:
  # Last 30 minutes, browse immediately
  loog fetch --server localhost:8080 --since 30m --replay

  # A precise incident window, only one namespace
  loog fetch --server localhost:8080 \
    --from 2026-09-10T14:00:00Z --to 2026-09-10T15:00:00Z \
    -f 'Namespaces("payments")' -o incident.loog`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runFetch(cmd.Context())
	},
}

func init() {
	fetchCmd.Flags().StringVar(&fetchServer, "server", "",
		"Recorder retrieval API address, e.g. localhost:8080 (required)")
	fetchCmd.Flags().StringVar(&fetchFrom, "from", "",
		"Start of the window (RFC3339 or 'YYYY-MM-DD HH:MM:SS'); empty = beginning")
	fetchCmd.Flags().StringVar(&fetchTo, "to", "",
		"End of the window (RFC3339 or 'YYYY-MM-DD HH:MM:SS'); empty = now")
	fetchCmd.Flags().StringVar(&fetchSince, "since", "",
		"Relative window ending now, e.g. 2h or 30m (overrides --from/--to)")
	fetchCmd.Flags().StringVarP(&fetchFilter, "filter", "f", "",
		"Filter expression (defaults to the recorder's own filter when empty)")
	fetchCmd.Flags().StringVarP(&fetchOutput, "output", "o", "",
		"Output .loog file (default: a temporary file)")
	fetchCmd.Flags().BoolVar(&fetchReplay, "replay", false,
		"Open the fetched file in the TUI after downloading")

	_ = fetchCmd.MarkFlagRequired("server")
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(ctx context.Context) error {
	setupDebugLogger()

	endpoint, err := buildExtractURL(fetchServer, fetchFrom, fetchTo, fetchSince, fetchFilter)
	if err != nil {
		return err
	}

	out := fetchOutput
	if out == "" {
		f, terr := os.CreateTemp("", "loog-fetch-*.loog")
		if terr != nil {
			return fmt.Errorf("creating temp output: %w", terr)
		}
		out = f.Name()
		_ = f.Close()
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("requesting extract: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	written, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("saving extract: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing output: %w", closeErr)
	}

	revs := resp.Header.Get("X-Loog-Revisions")
	res := resp.Header.Get("X-Loog-Resources")
	_, _ = fmt.Fprintf(os.Stderr, "Fetched %d bytes (%s revisions, %s resources) to %s\n",
		written, orDash(revs), orDash(res), out)

	if fetchReplay {
		replayFile = out
		filterExpr = "All()"
		return runReplayMode()
	}
	return nil
}

// buildExtractURL normalizes the server address (adding http:// if no scheme)
// and appends the /extract query parameters.
func buildExtractURL(server, from, to, since, filter string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("--server is required")
	}
	if !strings.Contains(server, "://") {
		//goland:noinspection HttpUrlsUsage
		server = "http://" + server
	}
	base, err := url.Parse(server)
	if err != nil {
		return "", fmt.Errorf("invalid --server %q: %w", server, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/extract"

	q := base.Query()
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	if since != "" {
		q.Set("since", since)
	}
	if filter != "" {
		q.Set("filter", filter)
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

func orDash(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/loog-project/loog/internal/httpapi"
	"github.com/loog-project/loog/internal/service"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/store/segment"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/mux"
)

var (
	recordDir         string
	recordInterval    time.Duration
	recordRetention   time.Duration
	recordMaxSegments int
	recordFilter      string
	recordSnapshotIvl uint64
	recordNoDurable   bool
	recordNoCompress  bool
	recordHTTPAddr    string
)

var recordCmd = &cobra.Command{
	Use:   "record [FLAGS] RESOURCES...",
	Short: "Continuously record resource changes into rotating .loog segments",
	Long: `Record runs a headless, always-on collector that watches the given resources
and writes every change into time-segmented .loog files in --dir. Segments roll
every --interval and are pruned after --retention, giving a fixed-size rolling
window of cluster history.

Each segment is self-contained, so a specific time frame can be pulled back out
cheaply with 'loog extract' (local files) or 'loog fetch' (over the HTTP API
enabled with --http). This command is meant to run as an in-cluster Deployment;
see deploy/ for manifests.

Examples:
  # Record Deployments and ConfigMaps, hourly segments, 30-day retention
  loog record --dir /data apps/v1/deployments v1/configmaps

  # Also serve the retrieval API so 'loog fetch' can pull time ranges
  loog record --dir /data --http :8080 apps/v1/deployments`,
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: gvrCompletion,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRecord(cmd.Context(), args)
	},
}

func init() {
	recordCmd.Flags().StringVar(&recordDir, "dir", "/var/lib/loog",
		"Directory to store rotating .loog segment files")
	recordCmd.Flags().DurationVar(&recordInterval, "interval", time.Hour,
		"How often to roll to a new segment file")
	recordCmd.Flags().DurationVar(&recordRetention, "retention", 30*24*time.Hour,
		"Delete segments older than this (0 keeps everything)")
	recordCmd.Flags().IntVar(&recordMaxSegments, "max-segments", 0,
		"Hard cap on retained segments (0 = unlimited); oldest deleted first")
	recordCmd.Flags().StringVarP(&recordFilter, "filter", "f", "All()",
		"Filter expression selecting which resources to record")
	recordCmd.Flags().Uint64VarP(&recordSnapshotIvl, "snapshot-interval", "s", 8,
		"Create a full snapshot after this many patches")
	recordCmd.Flags().BoolVar(&recordNoDurable, "no-durable-sync", false,
		"Skip periodic fsync (faster, unsafe on crashes)")
	recordCmd.Flags().BoolVar(&recordNoCompress, "no-compress", false,
		"Disable s2 compression for stored payloads")
	recordCmd.Flags().StringVar(&recordHTTPAddr, "http", "",
		"If set, serve the retrieval API on this address (e.g. :8080)")

	rootCmd.AddCommand(recordCmd)
}

func runRecord(ctx context.Context, args []string) error {
	setupDebugLogger()
	setupLog.Info().
		Str("dir", recordDir).
		Dur("interval", recordInterval).
		Dur("retention", recordRetention).
		Msg("Starting loog recorder")

	filterProgram, err := expr.Compile(recordFilter, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		return fmt.Errorf("compiling filter expression: %w", err)
	}

	boltOpts := bboltStore.Options{
		Durable:      !recordNoDurable,
		SyncInterval: 50 * time.Millisecond,
		Compress:     !recordNoCompress,
	}
	segStore, err := segment.Open(recordDir, segment.Options{
		Interval:    recordInterval,
		Retention:   recordRetention,
		MaxSegments: recordMaxSegments,
		Bolt:        boltOpts,
	})
	if err != nil {
		return fmt.Errorf("opening segment store: %w", err)
	}
	defer func() { _ = segStore.Close() }()

	trackerService := service.NewTrackerService(segStore, recordSnapshotIvl, true)
	defer func() { _ = trackerService.Close() }()

	cfg, err := restConfigForKubeconfig(kubeConfigPath, kubeContext)
	if err != nil {
		return fmt.Errorf("loading kubeconfig: %w", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating dynamic client: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	m, err := mux.New(ctx, dyn)
	if err != nil {
		return fmt.Errorf("creating dynamic mux: %w", err)
	}
	defer m.Stop()

	watched := 0
	for _, a := range args {
		gvr, gerr := util.ParseGroupVersionResource(a)
		if gerr != nil {
			return fmt.Errorf("invalid resource %q: %w", a, gerr)
		}
		if aerr := m.Add(gvr); aerr != nil {
			// A single missing/unservable CRD must not take down a recorder
			// watching dozens of kinds. Warn and carry on; fail only if nothing
			// at all could be watched.
			setupLog.Warn().Err(aerr).Str("gvr", gvr.String()).
				Msg("Could not watch resource (not installed or version not served?), skipping")
			continue
		}
		watched++
		setupLog.Info().Str("gvr", gvr.String()).Msg("Watching resource")
	}
	if watched == 0 {
		return fmt.Errorf("none of the %d requested resources could be watched", len(args))
	}

	// Optional retrieval API. It reads only rolled segments and excludes the
	// active file (which bbolt keeps exclusively locked).
	var httpServer *http.Server
	if recordHTTPAddr != "" {
		api := httpapi.NewServer(segStore.Dir(), segStore.ActivePath, recordFilter, !recordNoCompress)
		httpServer = &http.Server{Addr: recordHTTPAddr, Handler: api.Handler()}
		go func() {
			setupLog.Info().Str("addr", recordHTTPAddr).Msg("Serving retrieval API")
			if serveErr := httpServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				log.Error().Err(serveErr).Msg("Retrieval API server error")
			}
		}()
	}

	// Stop cleanly on SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		setupLog.Info().Msg("Received interrupt, stopping recorder...")
		cancel()
	}()

	runRecorderLoop(ctx, m, trackerService, segStore, filterProgram)

	if httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}
	setupLog.Info().Msg("Recorder stopped, bye!")
	return nil
}

// runRecorderLoop consumes watch events and commits them into the active
// segment, rolling over on schedule. Rotation and commits run in this single
// goroutine, so resetting the tracker cache on rotation is race-free.
func runRecorderLoop(
	ctx context.Context,
	m *mux.Mux,
	trackerService *service.TrackerService,
	segStore *segment.Store,
	filterProgram *vm.Program,
) {
	// Check for rotation at least this often even when no events arrive, so a
	// quiet cluster still rolls segments roughly on schedule.
	checkEvery := max(min(recordInterval/4, 30*time.Second), time.Second)
	ticker := time.NewTicker(checkEvery)
	defer ticker.Stop()

	rotate := func() {
		if rotated, err := segStore.MaybeRotate(); err != nil {
			log.Error().Err(err).Msg("Error rotating segment")
		} else if rotated {
			trackerService.ResetCache()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rotate()
		case ev, ok := <-m.Events():
			if !ok {
				return
			}
			rotate()

			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				log.Warn().Msgf("Expected *unstructured.Unstructured, got %T", ev.Object)
				continue
			}

			pass, err := expr.Run(filterProgram, util.EventEntryEnv{Event: ev, Object: obj})
			if err != nil {
				log.Error().Err(err).Msg("Filter expression error")
				continue
			}
			if passBool, _ := pass.(bool); !passBool {
				continue
			}

			obj.SetManagedFields(nil)
			uid := string(obj.GetUID())

			if ev.Type == watch.Deleted {
				_, err = trackerService.CommitDelete(ctx, uid, obj)
			} else {
				_, err = trackerService.Commit(ctx, uid, obj)
			}
			if err != nil {
				var dupErr service.DuplicateResourceVersionError
				if errors.As(err, &dupErr) {
					continue
				}
				log.Error().Err(err).
					Str("kind", obj.GetKind()).
					Str("name", obj.GetName()).
					Msg("Error committing revision")
			}
		}
	}
}

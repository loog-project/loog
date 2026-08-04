package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/loog-project/loog/internal/adapter"
	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/simulation"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/tui"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
	"github.com/loog-project/loog/pkg/mux"
)

var (
	// persistent flags
	cfgFile          string
	kubeConfigPath   string
	enableDebugMode  bool
	truncateDebugLog bool

	// local flags
	outputFile       string
	noDurableSync    bool
	disableCache     bool
	disableCompress  bool
	snapshotInterval uint64
	filterExpr       string
	headlessMode     bool
	simulateMode     bool
	appendOutput     bool
	replayFile       string
)

var rootCmd = &cobra.Command{
	Use:   "loog [FLAGS] [RESOURCES...]",
	Short: "Kubernetes Resource History Viewer",
	Long: `Loog is an interactive or headless tool that watches arbitrary Kubernetes
resources and records every change as either a snapshot or patch. You can explore
those revisions in a Terminal UI or collect them head-less for further analysis`,
	Args:              cobra.ArbitraryArgs,
	ValidArgsFunction: gvrCompletion,
	PreRunE:           validateArgsAndFlags,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), args)
	},
}

var setupLog = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().
	Timestamp().
	Caller().
	Logger()

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	cobra.OnInitialize(initConfig)

	// global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.loog.yaml)")
	rootCmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", "",
		"Path to the kubeconfig file (overrides $KUBECONFIG; defaults to $KUBECONFIG or $HOME/.kube/config)")
	rootCmd.PersistentFlags().BoolVar(&enableDebugMode, "debug", false,
		"Enable debug mode, which will print additional information to the debug.log file")
	rootCmd.PersistentFlags().BoolVar(&truncateDebugLog, "truncate-debug", false,
		"Truncate the debug.log file on startup, if it exists")

	// loog command flags
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "",
		"Path to the *.loog output file (default: temporary file)")
	rootCmd.Flags().StringVarP(&filterExpr, "filter", "f", "All()",
		"Filter expression to select which resources to store (default: all resources)")
	rootCmd.Flags().BoolVarP(&headlessMode, "headless", "H", false,
		"Run in headless mode, without TUI. Useful for collecting revisions only.")
	rootCmd.Flags().BoolVar(&noDurableSync, "no-durable-sync", false,
		"Skip fsync on every commit to improve throughput (unsafe on crashes)")
	rootCmd.Flags().BoolVar(&disableCache, "disable-cache", false,
		"Disable in‑memory cache layer for the revision store")
	rootCmd.Flags().BoolVar(&disableCompress, "no-compress", false,
		"Disable s2 compression for stored payloads (larger DB but slightly less CPU)")
	rootCmd.Flags().Uint64VarP(&snapshotInterval, "snapshot-interval", "s", 8,
		"Create a full snapshot after this many patches (default 8)")
	rootCmd.Flags().BoolVar(&simulateMode, "simulate", false,
		"Run with simulated data instead of connecting to Kubernetes")
	rootCmd.Flags().BoolVar(&appendOutput, "append", false,
		"Allow --output to resume an existing .loog file instead of refusing it")
	rootCmd.Flags().StringVar(&replayFile, "replay", "",
		"Open an existing .loog file read-only and browse it, without connecting to Kubernetes")

	// allow some flags to be set via environment variables / config file
	mustBind("kubeconfig",
		viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig")))
	mustBind("debug",
		viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")))
	mustBind("truncate-debug",
		viper.BindPFlag("truncate-debug", rootCmd.PersistentFlags().Lookup("truncate-debug")))
	mustBind("no-durable-sync",
		viper.BindPFlag("no-durable-sync", rootCmd.Flags().Lookup("no-durable-sync")))
	mustBind("disable-cache",
		viper.BindPFlag("disable-cache", rootCmd.Flags().Lookup("disable-cache")))
	mustBind("snapshot-interval",
		viper.BindPFlag("snapshot-interval", rootCmd.Flags().Lookup("snapshot-interval")))
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".loog")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		setupLog.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
	}
}

// run is the main entry point for the command execution.
func run(ctx context.Context, args []string) error {
	setupDebugLogger()

	// Replay mode: open an existing capture read-only and browse it.
	if replayFile != "" {
		return runReplayMode()
	}

	// Simulate mode: skip Kubernetes entirely, use simulated data
	if simulateMode {
		return runSimulateMode()
	}

	// Production mode: connect to Kubernetes
	cleanup, prog, trackerService, rps, m, err := setupProduction(ctx, args)
	defer cleanup()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	if headlessMode {
		runHeadless(ctx, cancel, &wg, m, trackerService, rps, prog)
	} else {
		runInteractive(ctx, cancel, &wg, m, trackerService, rps, prog)
	}

	wg.Wait()
	setupLog.Info().Msg("Collector stopped, bye!")
	return nil
}

// setupDebugLogger configures the global zerolog logger.
func setupDebugLogger() {
	if enableDebugMode {
		setupLog.Info().Msg("Debug mode is enabled, setting up debug logger...")

		fileMode := os.O_CREATE | os.O_WRONLY
		if truncateDebugLog {
			fileMode |= os.O_TRUNC
		} else {
			fileMode |= os.O_APPEND
		}
		logFile, logError := os.OpenFile("debug.log", fileMode, 0o644)
		if logError != nil {
			setupLog.Fatal().Err(logError).Msg("Error opening debug log file")
		}
		// Note: logFile is intentionally not closed here; it lives for the process lifetime.
		// Go's runtime will close it on exit.

		log.Logger = zerolog.New(logFile).With().
			Timestamp().
			Caller().
			Logger().
			Level(zerolog.DebugLevel)
	} else {
		log.Logger = zerolog.Nop()
	}
}

// runSimulateMode starts the TUI with simulated data.
func runSimulateMode() error {
	setupLog.Info().Msg("Running in simulate mode with generated data")

	klog.SetOutput(io.Discard)
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("cannot open %s: %w", os.DevNull, err)
	}
	defer func(devNull *os.File) {
		_ = devNull.Close()
	}(devNull)
	savedStderr := os.Stderr
	os.Stderr = devNull
	defer func() { os.Stderr = savedStderr }()

	simStore := simulation.New()
	app := tui.NewApp(simStore, tui.WithSimulator(simStore))
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		setupLog.Error().Err(err).Msg("Error running TUI program")
	}
	return nil
}

// runReplayMode opens an existing capture read-only and browses it in the TUI.
// It never connects to Kubernetes and never writes to the file.
func runReplayMode() error {
	setupLog.Info().Str("file", replayFile).Msg("Replaying capture (read-only)")

	// Keep klog and stderr off the TUI, same as simulate mode.
	klog.SetOutput(io.Discard)
	defer klog.SetOutput(io.Discard)
	if devNull, derr := os.Open(os.DevNull); derr == nil {
		savedStderr := os.Stderr
		os.Stderr = devNull
		defer func() {
			os.Stderr = savedStderr
			_ = devNull.Close()
		}()
	}

	rps, err := bboltStore.NewWithOptions(replayFile, bboltStore.Options{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("opening replay file: %w", err)
	}
	defer func() { _ = rps.Close() }()

	filterProgram, err := expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		return fmt.Errorf("compiling filter expression: %w", err)
	}

	liveStore := adapter.NewLiveStore()
	// A throwaway tracker service (no cache) just satisfies loadHistoryFromDB's
	// WarmCache call; nothing is committed in replay.
	trackerService := service.NewTrackerService(rps, snapshotInterval, false)
	defer func() { _ = trackerService.Close() }()

	// Program is nil: we populate the store up-front, before the TUI runs.
	handler := &adapter.TUIRevisionHandler{Store: liveStore}
	if loadErr := loadHistoryFromDB(trackerService, rps, filterProgram, handler); loadErr != nil {
		return fmt.Errorf("loading capture: %w", loadErr)
	}
	liveStore.SortTimeline()
	liveStore.RebuildKindGroups()

	// No recording, no simulator, no watch callbacks: a pure browse session.
	app := tui.NewApp(liveStore)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, runErr := p.Run(); runErr != nil {
		setupLog.Error().Err(runErr).Msg("Error running TUI program")
	}
	return nil
}

// setupProduction initializes the output file, filter, store, kube client, and mux.
// It returns a cleanup function, the compiled filter, tracker service, store, and mux.
func setupProduction(ctx context.Context, args []string) (
	cleanup func(),
	prog *vm.Program,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	m *mux.Mux,
	err error,
) {
	var cleanups []func()
	cleanup = func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	if outputFile == "" {
		file, fileErr := os.CreateTemp("", "loog-output-*.loog")
		if fileErr != nil {
			return cleanup, nil, nil, nil, nil, fmt.Errorf("cannot create temp file: %w", fileErr)
		}
		outputFile = file.Name()
		_ = file.Close() // bbolt reopens by path

		if headlessMode {
			// In headless mode the collected file IS the deliverable, so keep
			// it and tell the user where it landed instead of deleting it.
			setupLog.Info().Msgf("No output file specified; collecting to: %s", outputFile)
		} else {
			cleanups = append(cleanups, func() {
				if removeErr := os.Remove(outputFile); removeErr != nil {
					setupLog.Err(removeErr).Msg("Cannot remove temp file")
				}
			})
			setupLog.Info().Msgf("No output file specified, using temporary file: %s", outputFile)
		}
	}

	setupLog.Info().
		Str("expression", filterExpr).
		Msg("Compiling filter expression...")
	prog, err = expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		return cleanup, nil, nil, nil, nil, fmt.Errorf("error compiling filter expression: %w", err)
	}

	setupLog.Info().
		Str("store-file", outputFile).
		Msg("Preparing object revision store...")
	rps, err = bboltStore.NewWithOptions(outputFile, bboltStore.Options{
		Durable:      !noDurableSync,
		SyncInterval: 50 * time.Millisecond,
		Compress:     !disableCompress,
	})
	if err != nil {
		return cleanup, nil, nil, nil, nil, fmt.Errorf("error preparing store: %w", err)
	}
	cleanups = append(cleanups, func() { _ = rps.Close() })
	trackerService = service.NewTrackerService(rps, snapshotInterval, !disableCache)
	cleanups = append(cleanups, func() { _ = trackerService.Close() })

	setupLog.Info().Msg("Preparing dynamic Kubernetes watch client...")
	cfg, cfgErr := restConfigForKubeconfig(kubeConfigPath)
	if cfgErr != nil {
		err = fmt.Errorf("error loading kubeconfig: %w", cfgErr)
		return
	}
	dyn, dynErr := dynamic.NewForConfig(cfg)
	if dynErr != nil {
		return cleanup, nil, nil, nil, nil, fmt.Errorf("error creating dynamic watch client: %w", dynErr)
	}

	m, err = mux.New(ctx, dyn)
	if err != nil {
		return cleanup, nil, nil, nil, nil, fmt.Errorf("error creating dynamic mux: %w", err)
	}
	cleanups = append(cleanups, func() { m.Stop() })

	for _, r := range args {
		gvr, gvrParseErr := util.ParseGroupVersionResource(r)
		if gvrParseErr != nil {
			return cleanup, nil, nil, nil, nil, fmt.Errorf("cannot parse argument '%s' to GVR: %w", r, gvrParseErr)
		}
		if muxAddErr := m.Add(gvr); muxAddErr != nil {
			return cleanup, nil, nil, nil, nil, fmt.Errorf("cannot add GVR '%s' to dynamic mux: %w", gvr, muxAddErr)
		}
	}

	return cleanup, prog, trackerService, rps, m, nil
}

// runHeadless runs the collector without a TUI, waiting for SIGINT.
func runHeadless(
	ctx context.Context,
	cancel context.CancelFunc,
	wg *sync.WaitGroup,
	m *mux.Mux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	prog *vm.Program,
) {
	setupLog.Info().Msg("Running in headless mode, using no-op revision handler")

	wg.Go(func() {
		runCollector(ctx, m, trackerService, rps, prog, &noOpRevisionHandler{})
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(c)
	<-c

	setupLog.Info().Msg("Received interrupt signal, stopping collector...")
	cancel()
}

// runInteractive starts the TUI with a LiveStore and runs the collector in background.
func runInteractive(
	ctx context.Context,
	cancel context.CancelFunc,
	wg *sync.WaitGroup,
	m *mux.Mux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	prog *vm.Program,
) {
	setupLog.Info().Msg("Running in interactive mode with new TUI")

	liveStore := adapter.NewLiveStore()
	app := tui.NewApp(liveStore,
		tui.WithRecording(),
		tui.WithWatchCallbacks(
			func(rk resource.Kind) {
				gvr, err := util.ParseGroupVersionResource(rk.GVR())
				if err != nil {
					log.Error().Err(err).Str("gvr", rk.GVR()).Msg("Cannot parse GVR for watch add")
					return
				}
				// m.Add blocks until the informer's cache syncs. This callback
				// runs on the bubbletea event loop, so blocking here would
				// freeze the whole UI (indefinitely if the GVR never syncs).
				// Run it in the background; events arrive via the collector.
				go func() {
					if err := m.Add(gvr); err != nil {
						log.Error().Err(err).Str("kind", rk.Kind).Msg("Cannot add watch to mux")
					} else {
						log.Info().Str("kind", rk.Kind).Str("gvr", rk.GVR()).Msg("Added dynamic watch")
					}
				}()
			},
			func(kind string) {
				log.Info().Str("kind", kind).Msg("Watch kind removed from TUI (mux removal requires GVR)")
			},
		),
	)
	program := tea.NewProgram(app, tea.WithAltScreen())

	// Redirect klog (k8s client-go) into the TUI's debug log viewer
	logCapture := &tuiLogWriter{program: program}
	klog.SetOutput(logCapture)
	// Stop routing klog into the (dead) program once the TUI exits.
	defer klog.SetOutput(io.Discard)

	savedStderr := os.Stderr
	if devNull, derr := os.Open(os.DevNull); derr == nil {
		os.Stderr = devNull
		// Deferred so stderr is restored and the fd closed even if program.Run panics.
		defer func() {
			os.Stderr = savedStderr
			_ = devNull.Close()
		}()
	}

	// Discover cluster resource kinds in the background for WatchManager
	go func() {
		kinds, err := loadClusterResourceKinds(kubeConfigPath)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover cluster resource kinds for WatchManager")
			return
		}
		liveStore.SetUnwatchedKinds(kinds)
	}()

	handler := &adapter.TUIRevisionHandler{
		Store:   liveStore,
		Program: program,
	}

	// Register the goroutines with the WaitGroup before launching them,
	// ensuring wg.Wait() in the caller cannot return prematurely.
	wg.Add(2)
	go func() {
		defer wg.Done()
		program.Send(nil) // wait until program is ready
		if historyErr := loadHistoryFromDB(trackerService, rps, prog, handler); historyErr != nil {
			log.Error().Err(historyErr).Msg("Error loading history from database")
		}
		// History is walked one resource at a time, so the timeline arrives
		// grouped by resource. Sort it chronologically and refresh once.
		liveStore.SortTimeline()
		program.Send(adapter.LiveRevisionMsg{})
	}()
	go func() {
		defer wg.Done()
		runCollector(ctx, m, trackerService, rps, prog, handler)
	}()

	if _, teaErr := program.Run(); teaErr != nil {
		setupLog.Error().Err(teaErr).Msg("Error running TUI program")
	}

	setupLog.Info().Msg("TUI program exited, stopping collector")
	cancel()
}

// revisionHandler is the handler used by the collector to handle revisions.
type revisionHandler interface {
	HandleRevision(
		obj *unstructured.Unstructured,
		revisionID store.RevisionID,
		snapshot *store.Snapshot,
		patch *store.Patch,
	) error
}

// tuiLogWriter is an io.Writer that captures lines written by external
// libraries (klog, stray stderr) and forwards them to the TUI as
// ExternalLogMsg via program.Send. Error-level keywords trigger a toast.
type tuiLogWriter struct {
	program *tea.Program
	buf     []byte
}

// maxLogBuf is the maximum size of the internal line buffer.
// Lines longer than this are silently discarded to prevent unbounded growth.
const maxLogBuf = 64 * 1024

func (w *tuiLogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)

	// Prevent unbounded growth if a caller writes enormous data without
	// newlines (e.g., a binary blob).
	if len(w.buf) > maxLogBuf {
		w.buf = w.buf[len(w.buf)-maxLogBuf:]
	}

	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:idx]))
		w.buf = w.buf[idx+1:]
		if line == "" {
			continue
		}
		isErr := strings.Contains(line, "ERROR") ||
			strings.Contains(line, "FATAL") ||
			strings.Contains(line, "error") ||
			strings.Contains(line, "fatal")
		w.program.Send(tui.ExternalLogMsg{Text: line, IsError: isErr})
	}
	return len(p), nil
}

var _ revisionHandler = (*noOpRevisionHandler)(nil)

// noOpRevisionHandler is a no-op implementation of the revisionHandler.
// It just logs the revision and does nothing else.
type noOpRevisionHandler struct{}

func (n noOpRevisionHandler) HandleRevision(
	obj *unstructured.Unstructured,
	revisionID store.RevisionID,
	_ *store.Snapshot,
	_ *store.Patch,
) error {
	log.Debug().
		Str("revision-id", revisionID.String()).
		Str("namespace", obj.GetNamespace()).
		Str("name", obj.GetName()).
		Str("kind", obj.GetKind()).
		Msg("Storing revision...")

	return nil
}

// runCollector runs the collector that listens to events from the dynamic mux
func runCollector(
	ctx context.Context,
	m *mux.Mux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	filterExprProgram *vm.Program,
	handler revisionHandler,
) {
	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-m.Events():
			if !ok {
				return
			}

			l := log.With().
				Str("event-type", string(ev.Type)).
				Logger()

			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				l.Warn().Msgf("Expected unstructured.Unstructured, got %T", ev.Object)
				continue
			}

			// make sure we want to store this object
			pass, err := expr.Run(filterExprProgram, util.EventEntryEnv{
				Event:  ev,
				Object: obj,
			})
			if err != nil {
				l.Error().Err(err).Msg("Error executing filter expression")
				continue
			}
			passBool, ok := pass.(bool)
			if !ok {
				l.Error().Msgf("Filter expression returned %T instead of bool", pass)
				continue
			}
			if !passBool {
				continue
			}

			l = l.With().
				Str("namespace", obj.GetNamespace()).
				Str("name", obj.GetName()).
				Str("kind", obj.GetKind()).
				Logger()

			l.Debug().Msg("Processing event...")

			// empty managed fields before committing as they only clutter and we in 99/100 cases don't need them
			obj.SetManagedFields(nil)
			revisionID, err := trackerService.Commit(ctx, string(obj.GetUID()), obj)
			if err != nil {
				var dupErr service.DuplicateResourceVersionError
				if errors.As(err, &dupErr) {
					l.Debug().Msgf("Resource version %s is already present in revision %d, skipping commit",
						obj.GetResourceVersion(), revisionID)
					continue
				}
				l.Error().Err(err).Msg("Error committing to tracker service")
				continue
			}

			snapshot, patch, err := rps.Get(ctx, string(obj.GetUID()), revisionID)
			if err != nil {
				l.Error().Err(err).Msgf("Error loading snapshot/patch for revision %s", revisionID.String())
				continue
			}

			if handleErr := handler.HandleRevision(obj, revisionID, snapshot, patch); handleErr != nil {
				l.Error().Err(handleErr).Msg("Error handling revision")
			}
		}
	}
}

func loadHistoryFromDB(
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	filterExprProgram *vm.Program,
	handler revisionHandler,
) error {
	objectRevisionState := map[string]*store.Snapshot{}
	err := rps.WalkObjectRevisions(func(
		objectUID string,
		revisionID store.RevisionID,
		snapshot *store.Snapshot,
		patch *store.Patch,
	) bool {
		var current *store.Snapshot
		if snapshot != nil {
			// full snapshot: deep-clone so we own the map and later patches
			// don't alias nested sub-maps.
			current = &store.Snapshot{
				ID:     revisionID,
				Object: resource.CloneMap(snapshot.Object),
				Time:   snapshot.Time,
			}
		} else {
			// patch: apply on top of last state
			prev := objectRevisionState[objectUID]
			if prev == nil {
				log.Warn().
					Str("objectUID", objectUID).
					Stringer("revisionID", revisionID).
					Msg("Patch arrived before any snapshot; skipping")
				return true
			}
			// Deep-clone the previous state so Apply doesn't mutate it
			// through shared nested map references.
			base := resource.CloneMap(prev.Object)
			diffmap.Apply(base, patch.Patch)
			current = &store.Snapshot{
				ID:     revisionID,
				Object: base,
				Time:   patch.Time,
			}
		}
		objectRevisionState[objectUID] = current
		trackerService.WarmCache(objectUID, current.Object, current.ID)
		unstructuredObj := &unstructured.Unstructured{Object: current.Object}

		// make sure we want to track this object
		pass, err := expr.Run(filterExprProgram, util.EventEntryEnv{Object: unstructuredObj})
		if err != nil {
			log.Error().Err(err).Msgf("Error executing filter expression for historic object %s/%s/%s",
				unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj.GetKind())
			return true
		}
		passBool, ok := pass.(bool)
		if !ok {
			log.Error().Msgf("Filter expression returned %T instead of bool for historic object %s/%s/%s",
				pass, unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj.GetKind())
			return true
		}
		if !passBool {
			return true
		}

		// Pass the original snapshot/patch (not the reconstructed `current`) so
		// the handler derives the correct event type and PreviousID. `current`
		// always has PreviousID 0, which made every historic revision look ADDED.
		// The full reconstructed object travels in unstructuredObj.
		if handleErr := handler.HandleRevision(unstructuredObj, revisionID, snapshot, patch); handleErr != nil {
			log.Error().Err(handleErr).Msg("Error handling historic revision")
		}
		return true
	})
	return err
}

func validateArgsAndFlags(_ *cobra.Command, args []string) error {
	// Replay mode browses an existing file read-only; it can't be combined
	// with any of the collection/output flags.
	if replayFile != "" {
		if len(args) > 0 || outputFile != "" || appendOutput || headlessMode || simulateMode {
			return fmt.Errorf(
				"--replay cannot be combined with resource args, --output, --append, --headless, or --simulate")
		}
		if info, err := os.Stat(replayFile); err != nil || info.IsDir() {
			return fmt.Errorf("--replay file %q does not exist or is not a file", replayFile)
		}
		return nil
	}

	// Simulate mode doesn't need resource args or output file
	if simulateMode {
		return nil
	}

	if len(args) == 0 && outputFile == "" {
		return fmt.Errorf(
			"at least one resource argument or the --output flag must be provided (you may provide both)")
	}

	// Guard against silently appending to (and pre-loading) an existing capture.
	// Resuming is opt-in via --append; browsing read-only is --replay.
	if outputFile != "" && !appendOutput {
		if info, err := os.Stat(outputFile); err == nil && !info.IsDir() {
			return fmt.Errorf(
				"output file %q already exists; use --append to resume it or --replay to browse it read-only",
				outputFile)
		}
	}

	// validate each provided resource argument
	for _, a := range args {
		if _, err := util.ParseGroupVersionResource(a); err != nil {
			return fmt.Errorf("invalid resource argument %q: %w", a, err)
		}
	}

	return nil
}

func mustBind(flagName string, err error) {
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to bind flag %s", flagName)
	}
}

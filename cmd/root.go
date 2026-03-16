package cmd

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/loog-project/loog/internal/adapter"
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
	defaultKube := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKube = filepath.Join(home, ".kube", "config")
	}
	rootCmd.PersistentFlags().StringVar(&kubeConfigPath, "kubeconfig", defaultKube,
		"Path to the kubeconfig file (defaults to $HOME/.kube/config)")
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

	ctx, cancel := context.WithCancel(context.Background())
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

	simStore := simulation.New()
	app := tui.NewApp(simStore, tui.WithSimulator(simStore))
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		setupLog.Error().Err(err).Msg("Error running TUI program")
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
			setupLog.Fatal().Err(fileErr).Msg("Cannot create temp file")
		}
		cleanups = append(cleanups, func() {
			_ = file.Close()
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				setupLog.Err(removeErr).Msg("Cannot remove temp file")
			}
		})
		outputFile = file.Name()
		setupLog.Info().Msgf("No output file specified, using temporary file: %s", outputFile)
	}

	setupLog.Info().
		Str("expression", filterExpr).
		Msg("Compiling filter expression...")
	prog, err = expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error compiling filter expression")
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
		setupLog.Fatal().Err(err).Msg("Error preparing store")
	}
	trackerService = service.NewTrackerService(rps, snapshotInterval, !disableCache)

	setupLog.Info().Msg("Preparing dynamic Kubernetes watch client...")
	cfg, cfgErr := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if cfgErr != nil {
		setupLog.Fatal().Err(cfgErr).Msg("Error loading kubeconfig")
	}
	dyn, dynErr := dynamic.NewForConfig(cfg)
	if dynErr != nil {
		setupLog.Fatal().Err(dynErr).Msg("Error creating dynamic watch client")
	}

	m, err = mux.New(ctx, dyn)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error creating dynamic mux")
	}
	cleanups = append(cleanups, func() { m.Stop() })

	for _, r := range args {
		gvr, gvrParseErr := util.ParseGroupVersionResource(r)
		if gvrParseErr != nil {
			setupLog.Fatal().Err(gvrParseErr).Msgf("Cannot parse argument '%s' to GVR", r)
		}
		if muxAddErr := m.Add(gvr); muxAddErr != nil {
			setupLog.Fatal().Err(muxAddErr).Msgf("Cannot add GVR '%s' to dynamic mux", gvr)
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
	signal.Notify(c, os.Interrupt)
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
			func(rk tui.ResourceKind) {
				gvr, err := util.ParseGroupVersionResource(rk.GVR())
				if err != nil {
					log.Error().Err(err).Str("gvr", rk.GVR()).Msg("Cannot parse GVR for watch add")
					return
				}
				if err := m.Add(gvr); err != nil {
					log.Error().Err(err).Str("kind", rk.Kind).Msg("Cannot add watch to mux")
				} else {
					log.Info().Str("kind", rk.Kind).Str("gvr", rk.GVR()).Msg("Added dynamic watch")
				}
			},
			func(kind string) {
				log.Info().Str("kind", kind).Msg("Watch kind removed from TUI (mux removal requires GVR)")
			},
		),
	)
	program := tea.NewProgram(app, tea.WithAltScreen())

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

	wg.Add(2)
	go func() {
		program.Send(nil) // wait until program is ready

		go func() {
			if historyErr := loadHistoryFromDB(trackerService, rps, prog, handler); historyErr != nil {
				log.Error().Err(historyErr).Msg("Error loading history from database")
			}
			wg.Done()
		}()

		go func() {
			runCollector(ctx, m, trackerService, rps, prog, handler)
			wg.Done()
		}()
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
			if !pass.(bool) {
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
				if errors.As(err, &service.DuplicateResourceVersionError{}) {
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
			// full snapshot: start anew
			diffMap := make(diffmap.DiffMap)
			maps.Copy(diffMap, snapshot.Object)
			current = &store.Snapshot{
				ID:     revisionID,
				Object: diffMap,
				Time:   snapshot.Time,
			}
		} else {
			// patch: apply on top of last state
			base := make(diffmap.DiffMap)
			maps.Copy(base, objectRevisionState[objectUID].Object)
			diffmap.Apply(base, patch.Patch)
			current = &store.Snapshot{
				ID:     revisionID,
				Object: base,
				Time:   patch.Time,
			}
		}
		objectRevisionState[objectUID] = current
		trackerService.WarmCache(objectUID, current)
		unstructuredObj := &unstructured.Unstructured{Object: current.Object}

		// make sure we want to track this object
		pass, err := expr.Run(filterExprProgram, util.EventEntryEnv{Object: unstructuredObj})
		if err != nil {
			log.Error().Err(err).Msgf("Error executing filter expression for historic object %s/%s/%s",
				unstructuredObj.GetNamespace(), unstructuredObj.GetName(), unstructuredObj.GetKind())
			return true
		}
		if !pass.(bool) {
			return true
		}

		if handleErr := handler.HandleRevision(unstructuredObj, revisionID, current, patch); handleErr != nil {
			log.Error().Err(handleErr).Msg("Error handling historic revision")
		}
		return true
	})
	return err
}

func validateArgsAndFlags(_ *cobra.Command, args []string) error {
	// Simulate mode doesn't need resource args or output file
	if simulateMode {
		return nil
	}

	if len(args) == 0 && outputFile == "" {
		return fmt.Errorf(
			"at least one resource argument or the --output flag must be provided (you may provide both)")
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

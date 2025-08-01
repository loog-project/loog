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

	"github.com/loog-project/loog/internal/dynamicmux"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/ui"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
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
	snapshotInterval uint64
	filterExpr       string
	headlessMode     bool
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
	rootCmd.Flags().Uint64VarP(&snapshotInterval, "snapshot-interval", "s", 8,
		"Create a full snapshot after this many patches (default 8)")

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

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		setupLog.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
	}
}

// run is the main entry point for the command execution.
func run(ctx context.Context, args []string) error {

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
		defer func(logFile *os.File) {
			err := logFile.Close()
			if err != nil {
				setupLog.Error().Err(err).Msg("Error closing debug log file")
			}
		}(logFile)

		log.Logger = zerolog.New(logFile).With().
			Timestamp().
			Caller().
			Logger().
			Level(zerolog.DebugLevel)
	} else {
		// by default, we shouldn't log anything as this would break our TUI.
		log.Logger = zerolog.Nop()
	}

	if outputFile == "" {
		file, err := os.CreateTemp("", "loog-output-*.loog")
		if err != nil {
			setupLog.Fatal().Err(err).Msg("Cannot create temp file")
		}
		defer func() {
			_ = file.Close()
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				setupLog.Err(removeErr).Msg("Cannot remove temp file")
			}
		}()
		outputFile = file.Name()

		setupLog.Info().Msgf("No output file specified, using temporary file: %s", outputFile)
	}

	setupLog.Info().
		Str("expression", filterExpr).
		Msg("Compiling filter expression...")
	prog, err := expr.Compile(filterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error compiling filter expression")
	}

	setupLog.Info().
		Str("store-file", outputFile).
		Msg("Preparing object revision store...")
	rps, err := bboltStore.New(outputFile, nil, !noDurableSync)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error preparing store")
	}
	trackerService := service.NewTrackerService(rps, snapshotInterval, !disableCache)

	setupLog.Info().Msg("Preparing dynamic Kubernetes watch client...")
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error loading kubeconfig")
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error creating dynamic watch client")
	}

	// closing this context will stop the dynamic mux and the collector
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux, err := dynamicmux.New(ctx, dyn)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error creating dynamic mux")
	}
	defer mux.Stop()

	// add default resources from the arguments
	for _, r := range args {
		gvr, gvrParseErr := util.ParseGroupVersionResource(r)
		if gvrParseErr != nil {
			setupLog.Fatal().Err(gvrParseErr).Msgf("Cannot parse argument '%s' to GVR", r)
		}
		if muxAddErr := mux.Add(gvr); muxAddErr != nil {
			setupLog.Fatal().Err(muxAddErr).Msgf("Cannot add GVR '%s' to dynamic mux", gvr)
		}
	}

	var wg sync.WaitGroup

	if headlessMode {
		// headless mode: we will not use the UI, but just collect revisions and store them in the store.
		setupLog.Info().Msg("Running in headless mode, using no-op revision handler")

		wg.Add(1)
		go func() {
			runCollector(ctx, mux, trackerService, rps, prog, &noOpRevisionHandler{})
			wg.Done()
		}()

		// we use [signal.Notify] instead of [signal.NotifyContext] here so we can re-use the ctx for the TUI.
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c

		setupLog.Info().Msg("Received interrupt signal, stopping collector...")
		cancel()
	} else {
		// interactive mode: we will use the UI to display revisions and allow user interaction
		setupLog.Info().Msg("Running in interactive mode, using UI revision handler")

		root := ui.NewRoot(ui.DarkTheme, ui.NewListView(trackerService, rps))
		program := tea.NewProgram(root)

		handler := &uiRevisionHandler{
			program: program,
		}

		// run collector
		wg.Add(2) // 2 because we will run two goroutines: one for loading history and one for collecting revisions
		go func() {
			// wait until the program is ready to receive commands, so we don't skip any commits
			program.Send(nil)

			// after the program is ready, we can start load historic data
			go func() {
				if historyErr := loadHistoryFromDB(trackerService, rps, prog, handler); historyErr != nil {
					log.Error().Err(historyErr).Msg("Error loading history from database")
				}
				wg.Done()
			}()

			// and start collecting revisions
			go func() {
				runCollector(ctx, mux, trackerService, rps, prog, handler)
				wg.Done()
			}()
		}()

		if _, teaErr := program.Run(); teaErr != nil {
			setupLog.Error().Err(teaErr).Msg("Error running TUI program")
		}

		setupLog.Info().Msg("TUI program exited, stopping collector")
		cancel()
	}

	wg.Wait()
	setupLog.Info().Msg("Collector stopped, bye!")

	return nil
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

	// nothing to do in headless mode, as we are just storing revisions in the collector
	return nil
}

var _ revisionHandler = (*uiRevisionHandler)(nil)

// uiRevisionHandler is an implementation of the revisionHandler that sends
// a command to the TUI program to display the revision in the UI.
type uiRevisionHandler struct {
	program *tea.Program
}

func (u *uiRevisionHandler) HandleRevision(
	obj *unstructured.Unstructured,
	revisionID store.RevisionID,
	snapshot *store.Snapshot,
	patch *store.Patch,
) error {
	u.program.Send(ui.NewCommitCommand(
		string(obj.GetUID()),
		obj.GetKind(),
		obj.GetName(),
		obj.GetNamespace(),
		revisionID,
		snapshot,
		patch,
	))
	return nil
}

// runCollector runs the collector that listens to events from the dynamic mux
func runCollector(
	ctx context.Context,
	mux *dynamicmux.Mux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	filterExprProgram *vm.Program,
	handler revisionHandler,
) {
	for {
		select {
		case <-ctx.Done():
			return

		case ev := <-mux.Events():
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

func validateArgsAndFlags(cmd *cobra.Command, args []string) error {
	if len(args) == 0 && outputFile == "" {
		return fmt.Errorf("either specify at least one resource to watch or set the --output flag")
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

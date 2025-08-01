package main

import (
	"context"
	"errors"
	"flag"
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
	flagEnableDebugLog   bool
	flagTruncateDebugLog bool

	flagKubeconfig       string
	flagOutFile          string
	flagNotDurable       bool
	flagNoCache          bool
	flagSnapshotEvery    uint64
	flagFilterExpression string
	flagNonInteractive   bool
)

func init() {
	flag.BoolVar(&flagEnableDebugLog, "debug", false, "enable debug logging to debug.log")
	flag.BoolVar(&flagTruncateDebugLog, "truncate", false, "truncate debug.log instead of appending to it")
	flag.StringVar(&flagOutFile, "out", "", "dump output file")
	flag.BoolVar(&flagNotDurable, "not-durable", false, "if set to true, the store won't fsync every commit")
	flag.BoolVar(&flagNoCache, "no-cache", false, "if set to true, the store won't cache the data")
	flag.Uint64Var(&flagSnapshotEvery, "snapshot-every", 8, "patches until snapshot")
	flag.StringVar(&flagFilterExpression, "filter-expr", "All()", "filter for objects to process")
	flag.BoolVar(&flagNonInteractive, "non-interactive", false, "set to true to disable the UI")
	if h := homedir.HomeDir(); h != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(h, ".kube", "config"), "")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "")
	}
	flag.Parse()
}

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	// setupLog is a secondary logger only used for setup and initialization.
	// when you want to log something later when the TUI is running, it writes only to the debug.log file
	// if this was enabled by the [debugMode] flag.
	setupLog := zerolog.New(os.Stderr).With().
		Timestamp().
		Caller().
		Logger()
	if flagEnableDebugLog {
		setupLog.Info().Msg("Debug mode is enabled, setting up debug logger...")

		fileMode := os.O_CREATE | os.O_WRONLY
		if flagTruncateDebugLog {
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

	if flagOutFile == "" {
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
		flagOutFile = file.Name()

		setupLog.Info().Msgf("No output file specified, using temporary file: %s", flagOutFile)
	}

	setupLog.Info().
		Str("expression", flagFilterExpression).
		Msg("Compiling filter expression...")
	prog, err := expr.Compile(flagFilterExpression, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error compiling filter expression")
	}

	setupLog.Info().
		Str("store-file", flagOutFile).
		Msg("Preparing object revision store...")
	rps, err := bboltStore.New(flagOutFile, nil, !flagNotDurable)
	if err != nil {
		setupLog.Fatal().Err(err).Msg("Error preparing store")
	}
	trackerService := service.NewTrackerService(rps, flagSnapshotEvery, !flagNoCache)

	setupLog.Info().Msg("Preparing dynamic Kubernetes watch client...")
	cfg, err := clientcmd.BuildConfigFromFlags("", flagKubeconfig)
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
	for _, r := range flag.Args() {
		gvr, gvrParseErr := util.ParseGroupVersionResource(r)
		if gvrParseErr != nil {
			setupLog.Fatal().Err(gvrParseErr).Msgf("Cannot parse argument '%s' to GVR", r)
		}
		if muxAddErr := mux.Add(gvr); muxAddErr != nil {
			setupLog.Fatal().Err(muxAddErr).Msgf("Cannot add GVR '%s' to dynamic mux", gvr)
		}
	}

	var wg sync.WaitGroup

	if flagNonInteractive {
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

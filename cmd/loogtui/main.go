package main

import (
	"context"
	"flag"
	"log"
	"maps"
	"os"
	"os/signal"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
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
	flagKubeconfig       string
	flagOutFile          string
	flagResources        util.StringSliceFlag
	flagNotDurable       bool
	flagNoCache          bool
	flagSnapshotEvery    uint64
	flagFilterExpression string
	flagNonInteractive   bool
)

func init() {
	flag.StringVar(&flagOutFile, "out", "", "dump output file")
	flag.BoolVar(&flagNotDurable, "not-durable", false, "if set to true, the store won't fsync every commit")
	flag.BoolVar(&flagNoCache, "no-cache", false, "if set to true, the store won't cache the data")
	flag.Uint64Var(&flagSnapshotEvery, "snapshot-every", 8, "patches until snapshot")
	flag.StringVar(&flagFilterExpression, "filter-expr", "All()", "expr filter")
	flag.BoolVar(&flagNonInteractive, "non-interactive", false, "set to true to disable the UI")
	flag.Var(&flagResources, "resource", "<group>/<version>/<resource> (repeatable)")
	if h := homedir.HomeDir(); h != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(h, ".kube", "config"), "")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "")
	}
}

func main() {
	flag.Parse()
	if flagOutFile == "" {
		file, err := os.CreateTemp("", "loog-output-*.loog")
		if err != nil {
			log.Fatal("Cannot create temp file:", err)
			return
		}
		defer func() {
			_ = file.Close()
			if removeErr := os.Remove(file.Name()); removeErr != nil {
				log.Println("Cannot remove temp file:", removeErr)
			}
		}()
		flagOutFile = file.Name()
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log.Println("Compiling filter expression...")
	prog, err := expr.Compile(flagFilterExpression, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatal("Cannot compile filter expression:", err)
		return
	}

	log.Println("Creating store...")
	rps, err := bboltStore.New(flagOutFile, nil, !flagNotDurable)
	if err != nil {
		log.Fatal("Cannot create store:", err)
		return
	}
	trackerService := service.NewTrackerService(rps, flagSnapshotEvery, !flagNoCache)

	log.Println("Creating dynamic client...")
	cfg, err := clientcmd.BuildConfigFromFlags("", flagKubeconfig)
	if err != nil {
		log.Fatal("Cannot load kubeconfig:", err)
		return
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatal("Cannot create dynamic client:", err)
		return
	}

	mux, err := dynamicmux.New(ctx, dyn)
	if err != nil {
		log.Fatal("Cannot create dynamic mux:", err)
		return
	}
	defer mux.Stop()

	log.Println("Starting dynamic mux...")
	// add default resources from -resource flags
	for _, r := range flagResources {
		gvr, err := util.ParseGroupVersionResource(r)
		if err != nil {
			log.Fatal("Cannot parse resource:", err, "input:", r)
			return
		}
		if err := mux.Add(gvr); err != nil {
			log.Fatal("Cannot add resource to dynamic mux:", err, "input:", r)
			return
		}
	}

	var program *tea.Program
	var uiLogger ui.Logger

	if flagNonInteractive {
		log.Println("Running in non-interactive mode...")
		uiLogger = ui.StdLogger{}
	} else {
		log.Println("Building UI...")
		logger := ui.NewUILogger()
		root := ui.NewRoot(ui.DarkTheme, logger, ui.NewListView(trackerService, rps))
		program = tea.NewProgram(root)
		logger.Attach(program)

		uiLogger = logger
	}

	log.Println("Starting dynamic watches...")
	go runCollector(ctx, program, mux, trackerService, rps, prog, uiLogger)

	// TODO(future): load database on startup

	if !flagNonInteractive {
		log.Println("Starting UI...")
		if _, err := program.Run(); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("Running in non-interactive mode, press Ctrl+C to exit...")
		<-ctx.Done()
	}
}

func runCollector(
	ctx context.Context,
	p *tea.Program,
	mux *dynamicmux.Mux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	program *vm.Program,
	logSink ui.Logger,
) {
	if err := loadHistoryFromDB(rps, trackerService, p); err != nil {
		p.Send(ui.NewAlert("when walking object revisions", err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			logSink.Infof("collector", "stopping")
			log.Println("Stopping collector...")
			return
		case ev := <-mux.Events():
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			// make sure we want to track this object
			pass, err := expr.Run(program, util.EventEntryEnv{Event: ev, Object: obj})
			if err != nil {
				logSink.Errorf("collector", "when executing filter expression: %s", err)
				continue
			}
			if !pass.(bool) {
				continue
			}

			logSink.Infof("collector", "%s: %s/%s/%s",
				ev.Type,
				obj.GetNamespace(), obj.GetName(), obj.GetKind())

			// empty managed fields as they only clutter and we in 99/100 cases don't need them
			obj.SetManagedFields(nil)
			rev, err := trackerService.Commit(ctx, string(obj.GetUID()), obj)
			if err != nil {
				logSink.Errorf("collector", "when committing to tracker service: %s", err)
				continue
			}

			if p == nil {
				// we're running in non-interactive mode, so we don't need to send the commit command
				continue
			}

			// read
			snapshot, patch, err := rps.Get(ctx, string(obj.GetUID()), rev)
			if err != nil {
				logSink.Errorf("collector", "when reading tracked object from store: %s", err)
				continue
			}

			p.Send(ui.NewCommitCommand(
				string(obj.GetUID()),
				obj.GetKind(),
				obj.GetName(),
				obj.GetNamespace(),
				rev,
				snapshot,
				patch,
			))
		}
	}
}

func loadHistoryFromDB(rps store.ResourcePatchStore, trackerService *service.TrackerService, p *tea.Program) error {
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
		p.Send(ui.NewCommitCommand(
			objectUID,
			unstructuredObj.GetKind(),
			unstructuredObj.GetName(),
			unstructuredObj.GetNamespace(),
			revisionID,
			current,
			patch,
		))
		return true
	})
	return err
}

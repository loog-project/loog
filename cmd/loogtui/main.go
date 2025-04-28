package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/ui"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/internal/watch"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	flagKubeconfig       string
	flagOutFile          string
	flagResources        util.StringSliceFlag
	flagNotDurable       bool
	flagNoCache          bool
	flagSnapshotEvery    uint64
	flagFilterExpression string
)

func init() {
	flag.StringVar(&flagOutFile, "out", "output.loog", "dump output file")
	flag.BoolVar(&flagNotDurable, "not-durable", false, "if set to true, the store won't fsync every commit")
	flag.BoolVar(&flagNoCache, "no-cache", false, "if set to true, the store won't cache the data")
	flag.Uint64Var(&flagSnapshotEvery, "snapshot-every", 8, "patches until snapshot")
	flag.StringVar(&flagFilterExpression, "filter-expr", "All()", "expr filter")
	flag.Var(&flagResources, "resource", "<group>/<version>/<resource> (repeatable)")
	if h := homedir.HomeDir(); h != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(h, ".kube", "config"), "")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "")
	}
}

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	prog, err := expr.Compile(flagFilterExpression, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatal("Cannot compile filter expression:", err)
		return
	}

	rps, err := bboltStore.New(flagOutFile, nil, !flagNotDurable)
	if err != nil {
		log.Fatal("Cannot create store:", err)
		return
	}
	trackerService := service.NewTrackerService(rps, flagSnapshotEvery, !flagNoCache)

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
	mux := watch.New(ctx, dyn, v1.ListOptions{})
	defer mux.Stop()

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

	root := ui.NewRoot(ui.NewListView(trackerService, rps))
	program := tea.NewProgram(root)

	go runCollector(ctx, program, mux, trackerService, rps, prog)

	// TODO(future): load database on startup

	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

func runCollector(
	ctx context.Context,
	p *tea.Program,
	mux *watch.DynamicMux,
	trackerService *service.TrackerService,
	rps store.ResourcePatchStore,
	program *vm.Program,
) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping collector...")
			return
		case ev := <-mux.ResultChan():
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}

			// make sure we want to track this object
			pass, err := expr.Run(program, util.EventEntryEnv{Event: ev, Object: obj})
			if err != nil {
				p.Send(ui.NewAlert("when executing filter expression", err))
				continue
			}
			if !pass.(bool) {
				continue
			}

			// empty managed fields as they only clutter and we in 99/100 cases don't need them
			obj.SetManagedFields(nil)
			rev, err := trackerService.Commit(ctx, string(obj.GetUID()), obj)
			if err != nil {
				p.Send(ui.NewAlert("when committing to tracker service", err))
				continue
			}

			// read
			snapshot, patch, err := rps.Get(ctx, string(obj.GetUID()), rev)
			if err != nil {
				p.Send(ui.NewAlert("when reading tracked object from store", err))
				continue
			}

			p.Send(ui.NewCommitCommand(time.Now(), obj, rev, snapshot, patch))
		}
	}
}

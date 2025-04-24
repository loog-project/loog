package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/expr-lang/expr"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/util"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var (
		flagKubeconfig  string
		flagFilterExpr  string
		flagVerbose     bool
		flagContextSize int

		flagResources util.StringSliceFlag

		flagDataDir       string
		flagSyncWrites    bool
		flagSnapshotEvery uint64
	)
	flag.StringVar(&flagFilterExpr, "filter-expr", "All()", "Expression to filter the resource to watch")
	flag.BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	flag.IntVar(&flagContextSize, "context-size", 3, "Number of lines to show in context for diffs")

	flag.Var(&flagResources, "resource", "Resource to watch (can be specified multiple times)")

	flag.StringVar(&flagDataDir, "out", "output.bb", "File to store the object revisions")
	flag.BoolVar(&flagSyncWrites, "sync-writes", true, "Enable sync writes for the database")
	flag.Uint64Var(&flagSnapshotEvery, "snapshot-every", 3, "Number of patches to store before taking a snapshot")

	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	}
	flag.Parse()

	// Parse the resources
	var gvrs []schema.GroupVersionResource
	for _, resource := range flagResources {
		gvr, err := util.ParseGroupVersionResource(resource)
		if err != nil {
			log.Fatalf("Error parsing resource: %v", err)
			return
		}
		gvrs = append(gvrs, gvr)
	}
	config, err := clientcmd.BuildConfigFromFlags("", flagKubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
		return
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic client: %v", err)
		return
	}

	// Parse the filter expression
	log.Println("Compiling expression:", flagFilterExpr, "...")
	program, err := expr.Compile(flagFilterExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatalf("Error compiling expression: %v", err)
		return
	}
	log.Println("Expression compiled successfully")

	rps, err := bboltStore.New(flagDataDir, nil)
	if err != nil {
		panic("failed to create bbolt store: " + err.Error())
	}
	defer func(rps store.ResourcePatchStore) {
		_ = rps.Close()
	}(rps)

	svc := service.NewTrackerService(rps, flagSnapshotEvery)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	watcher, err := util.NewMultiWatcher(ctx, dyn, gvrs, v1.ListOptions{})
	if err != nil {
		log.Fatalf("Error creating multi-watcher: %v", err)
		return
	}
	defer watcher.Stop()

	log.Println("Watching resources:", gvrs, "...")
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Bye from event loop!")
				return
			case event := <-watcher.ResultChan():
				unstr, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					fmt.Println("Skipping event", event.Type, "because object is not unstructured:")
					spew.Dump(event)
					continue
				}

				// check if event should be included
				output, err := expr.Run(program, util.EventEntryEnv{
					Event:  event,
					Object: unstr,
				})
				if err != nil {
					log.Println("[WARN] Error evaluating expression, skipping:", err)
					continue
				}
				if !output.(bool) {
					if flagVerbose {
						log.Println("[INFO] Skipping event because expression evaluated to false")
					}
					continue
				}

				log.Println(event.Type, "::", unstr.GetName(), "(", unstr.GetUID(), ") in", unstr.GetNamespace())

				unstr.SetManagedFields(nil)
				if rid, err := svc.Commit(ctx, string(unstr.GetUID()), unstr); err != nil {
					log.Println("[ERROR] Error committing object:", err)
					continue
				} else {
					log.Println(" >> committed revision:", rid)
				}
			}
		}
	}()

	// Wait for the context to be done
	log.Println("Press Ctrl+C to exit")
	<-ctx.Done()
}

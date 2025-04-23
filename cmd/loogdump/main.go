package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/expr-lang/expr"
	"github.com/sergi/go-diff/diffmatchpatch"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	red   = "\x1b[9;90m"
	green = "\x1b[32m"
	reset = "\x1b[0m"
)

type EventEntry struct {
	EventType  watch.EventType
	ReceivedAt time.Time

	Name               types.NamespacedName
	ResourceGeneration int64
	ResourceVersion    string

	Object *unstructured.Unstructured
}

type EventEntryEnv struct {
	Event  watch.Event
	Object *unstructured.Unstructured
}

func (e EventEntryEnv) All() bool {
	return true
}

func (e EventEntryEnv) None() bool {
	return false
}

func (e EventEntryEnv) Namespaces(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, val := range vals {
		if val == e.Object.GetNamespace() {
			return true
		}
	}
	return false
}

func (e EventEntryEnv) Namespace(vals ...string) bool {
	return e.Namespaces(vals...)
}

func (e EventEntryEnv) Names(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, val := range vals {
		if val == e.Object.GetName() {
			return true
		}
	}
	return false
}

func (e EventEntryEnv) Name(vals ...string) bool {
	return e.Names(vals...)
}

func (e EventEntryEnv) Namespaced(namespace, name string) bool {
	return e.Object.GetNamespace() == namespace && e.Object.GetName() == name
}

func mustEncode(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

func performWrite(entries []EventEntry) error {
	f, err := os.OpenFile("dump.json", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func main() {
	var (
		flagGroup       string
		flagVersion     string
		flagKind        string
		flagKubeconfig  string
		flagExpr        string
		flagVerbose     bool
		flagContextSize int
	)
	flag.StringVar(&flagGroup, "group", "", "Group of the resource to watch")
	flag.StringVar(&flagVersion, "version", "v1", "Version of the resource to watch")
	flag.StringVar(&flagKind, "kind", "", "Kind of the resource to watch")
	flag.StringVar(&flagExpr, "expr", "true", "Expression to filter the resource to watch")
	flag.BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	flag.IntVar(&flagContextSize, "context-size", 3, "Number of lines to show in context for diffs")

	if home := homedir.HomeDir(); home != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(home, ".kube", "config"), "Path to the kubeconfig file")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	}
	flag.Parse()

	// compile expression
	log.Println("Compiling expression:", flagExpr, "...")
	program, err := expr.Compile(flagExpr, expr.Env(EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatalf("Error compiling expression: %v", err)
		return
	}
	log.Println("Expression compiled successfully")

	log.Println("Connecting to Kubernetes cluster...")
	config, err := clientcmd.BuildConfigFromFlags("", flagKubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
		return
	}

	// TODO(future): check if GVK is valid
	gvr := schema.GroupVersionResource{
		Group:    flagGroup,
		Version:  flagVersion,
		Resource: flagKind,
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating dynamic client: %v", err)
		return
	}

	log.Println("Watching", gvr, "...")
	w, err := dyn.Resource(gvr).Watch(context.TODO(), v1.ListOptions{})
	if err != nil {
		log.Fatalf("Error watching resource: %v", err)
		return
	}
	//
	var entries []EventEntry

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	result := make(chan struct{})

	// writer
	// TODO(future): make this fancier
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := performWrite(entries); err != nil {
					log.Printf("Error writing entries: %v", err)
				}
			case <-ctx.Done():
				log.Println("Stopping writer, writing remaining entries")
				if err := performWrite(entries); err != nil {
					log.Printf("Error writing entries: %v", err)
				}
				result <- struct{}{}
				return
			}
		}
	}()

	last := make(map[string]EventEntry)
	dmp := diffmatchpatch.New()

	// reader
	// TODO(future): make this fancier
	go func() {
		for event := range w.ResultChan() {
			unstr, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				spew.Dump(event)
				fmt.Println("Skipping event", event.Type, "because object is not unstructured")
				continue
			}

			// check if event should be included
			output, err := expr.Run(program, EventEntryEnv{
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

			unstr.SetManagedFields(nil)
			entry := EventEntry{
				EventType:  event.Type,
				ReceivedAt: time.Now(),
				Name: types.NamespacedName{
					Namespace: unstr.GetNamespace(),
					Name:      unstr.GetName(),
				},
				ResourceGeneration: unstr.GetGeneration(),
				ResourceVersion:    unstr.GetResourceVersion(),
				Object:             unstr,
			}
			entries = append(entries, entry)

			fmt.Println(event.Type, "::", unstr.GetName(), "@", unstr.GetNamespace())

			uid := string(unstr.GetUID())
			lastEntry, lastOk := last[uid]
			last[uid] = entry

			wasSuppressed := false

			// compare current and last object
			if lastOk {
				prevCopy := *entry.Object
				prevCopy.SetResourceVersion("")
				prevCopy.SetManagedFields(nil)
				afterCopy := *lastEntry.Object
				afterCopy.SetResourceVersion("")
				afterCopy.SetManagedFields(nil)

				previous, after := mustEncode(prevCopy), mustEncode(afterCopy)
				if previous == after {
					fmt.Println("< No change in object >")
				} else {
					diffs := dmp.DiffMain(after, previous, false)
					diffs = dmp.DiffCleanupSemantic(diffs)

					colored := DiffPrettyText(diffs)

					lines := strings.Split(colored, "\n")

					keep := make(map[int]struct{}, len(lines))
					for i, ln := range lines {
						if strings.Contains(ln, `"lastReconciledAt": `) {
							wasSuppressed = true
							continue
						}
						if strings.Contains(ln, red) || strings.Contains(ln, green) {
							for j := i - flagContextSize; j <= i+flagContextSize; j++ {
								if j >= 0 && j < len(lines) {
									keep[j] = struct{}{}
								}
							}
						}
					}
					gap := false
					for i, ln := range lines {
						if _, ok := keep[i]; !ok {
							gap = true
							continue
						}
						if gap {
							fmt.Println("    | ...") // visual separator for large omitted block
							gap = false
						}
						fmt.Printf("%3d | %s%s\n", i+1, ln, reset)
						wasSuppressed = false
					}
				}
			} else {
				fmt.Println("< First seen >")
			}

			if wasSuppressed {
				fmt.Println("< Output suppressed >")
			}
			fmt.Println()
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)
	<-c

	log.Println("Received interrupt signal, stopping watcher")
	cancel()
	<-result
}

func DiffPrettyText(diffs []diffmatchpatch.Diff) string {
	var buff bytes.Buffer
	for _, diff := range diffs {
		text := diff.Text

		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			_, _ = buff.WriteString(green)
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString(reset)

		case diffmatchpatch.DiffDelete:
			_, _ = buff.WriteString(red)
			_, _ = buff.WriteString(text)
			_, _ = buff.WriteString(reset)

		case diffmatchpatch.DiffEqual:
			_, _ = buff.WriteString(text)
		}
	}
	return buff.String()
}

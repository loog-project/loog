package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"github.com/loog-project/loog/internal/dynamicmux"
	"github.com/loog-project/loog/internal/util"
)

const (
	purple    = lipgloss.Color("99")
	orange    = lipgloss.Color("214")
	gray      = lipgloss.Color("245")
	lightGray = lipgloss.Color("241")
)

var (
	RandomKindColors = []lipgloss.Color{
		lipgloss.Color("12"),
		lipgloss.Color("4"),
		lipgloss.Color("13"),
		lipgloss.Color("5"),
		lipgloss.Color("14"),
		lipgloss.Color("4"),
	}

	HeaderStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true).
			Align(lipgloss.Center)

	CellStyle = lipgloss.NewStyle().
			Padding(0, 1)
	OddRowStyle = CellStyle.
			Foreground(gray)
	EvenRowStyle = CellStyle.
			Foreground(lightGray)

	ActivityStyle = lipgloss.NewStyle().
			Foreground(orange).
			Bold(true)

	ChangedStyle = lipgloss.NewStyle().
			Foreground(purple).
			Bold(true)
)

func tableStyleFunc(row, _ int) lipgloss.Style {
	var style lipgloss.Style

	switch {
	case row == table.HeaderRow:
		return HeaderStyle
	case row%2 == 0:
		style = EvenRowStyle
	default:
		style = OddRowStyle
	}
	return style
}

var (
	flagKubeconfig       string
	flagResources        util.StringSliceFlag
	flagOrder            util.StringSliceFlag
	flagFilterExpression string
)

type object struct {
	lastUpdate time.Time

	gvr       schema.GroupVersionResource
	uid       string
	kind      string
	namespace string

	name          string
	nameChangedAt time.Time

	status          string
	statusChangedAt time.Time
}

type model struct {
	objects   map[string]object
	kindOrder map[string]int
	colors    map[string]lipgloss.Color

	quitting bool
}

type tickMsg struct{}

type updateMsg struct {
	object object
}

func tick() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}
	case tickMsg:
		cmds = append(cmds, tick())
	case updateMsg:
		if strings.TrimSpace(msg.object.kind) == "" {
			return m, nil
		}
		if oldObject, oldObjectFound := m.objects[msg.object.uid]; oldObjectFound {
			if msg.object.status != oldObject.status {
				msg.object.statusChangedAt = time.Now()
			}
			if msg.object.name != oldObject.name {
				msg.object.nameChangedAt = time.Now()
			}
		} else {
			msg.object.statusChangedAt = time.Now()
			msg.object.nameChangedAt = time.Now()
		}
		m.objects[msg.object.uid] = msg.object
	}
	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	t := table.New().StyleFunc(tableStyleFunc)
	t.Headers("Kind", "Namespace", "Name", "Status", "Last Update")

	objects := make([]object, 0, len(m.objects))
	for _, o := range m.objects {
		objects = append(objects, o)
	}

	slices.SortFunc(objects, func(a, b object) int {
		aOrder, aExists := m.kindOrder[a.kind]
		bOrder, bExists := m.kindOrder[b.kind]

		if aExists && bExists {
			if aOrder != bOrder {
				return aOrder - bOrder
			}
		} else if aExists {
			return -1
		} else if bExists {
			return 1
		}

		if a.kind != b.kind {
			return strings.Compare(a.kind, b.kind)
		}
		return strings.Compare(a.uid, b.uid)
	})

	prevKind := ""
	for _, o := range objects {
		timeStr := humanize.Time(o.lastUpdate)
		if time.Now().Sub(o.lastUpdate) < 3*time.Second {
			timeStr = ActivityStyle.Render(timeStr)
		}

		color := gray

		color, hasColor := m.colors[o.kind]
		if !hasColor {
			color = RandomKindColors[len(m.colors)%len(RandomKindColors)]
			m.colors[o.kind] = color
		}

		nameStr := o.name
		if time.Now().Sub(o.nameChangedAt) < 10*time.Second {
			nameStr = ChangedStyle.Render(nameStr)
		}

		statusStr := o.status
		if time.Now().Sub(o.statusChangedAt) < 10*time.Second {
			statusStr = ChangedStyle.Render(statusStr)
		}

		if prevKind != "" && o.kind != prevKind {
			t.Row()
		}
		prevKind = o.kind

		t.Row(
			lipgloss.NewStyle().Foreground(color).Render(o.kind),
			o.namespace,
			nameStr,
			statusStr,
			timeStr,
		)
	}

	return "\n" + t.Render()
}
func init() {
	flag.StringVar(&flagFilterExpression, "filter-expr", "All()", "expr filter")
	flag.Var(&flagResources, "resource", "<group>/<version>/<resource> (repeatable)")
	flag.Var(&flagOrder, "order", "<kind> (repeatable)")
	if h := homedir.HomeDir(); h != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(h, ".kube", "config"), "")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "")
	}
	flag.Parse()
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	prog, err := expr.Compile(flagFilterExpression, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatal("Cannot compile filter expression:", err)
		return
	}

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

	order := make(map[string]int)
	for i, r := range flagOrder {
		order[r] = i
	}

	program := tea.NewProgram(model{
		objects:   make(map[string]object),
		kindOrder: order,
		colors:    make(map[string]lipgloss.Color),
	})

	go runCollector(ctx, program, mux, prog)

	if _, err := program.Run(); err != nil {
		log.Fatal(err)
	}
}

func runCollector(ctx context.Context, program *tea.Program, mux *dynamicmux.Mux, prog *vm.Program) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-mux.Events():
			obj, ok := ev.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			obj.SetManagedFields(nil)

			// make sure we want to track this object
			pass, err := expr.Run(prog, util.EventEntryEnv{Event: ev, Object: obj})
			if err != nil {
				log.Println("when executing filter expression:", err)
				continue
			}
			if !pass.(bool) {
				continue
			}

			status := "unknown"

			// TODO: refactor in the future :))))
			if statusVal, ok := obj.Object["status"]; ok {
				if statusMap, ok := statusVal.(map[string]any); ok {
					if state, ok := statusMap["state"]; ok {
						if statusStr, ok := state.(string); ok {
							status = statusStr
						} else {
							status = "status state not a string"
						}
					} else {
						status = "status state not found"
					}
				} else {
					status = "status not a map"
				}
			} else {
				status = "status not found"
			}

			o := object{
				lastUpdate: time.Now(),
				uid:        string(obj.GetUID()),
				kind:       obj.GetKind(),
				namespace:  obj.GetNamespace(),
				name:       obj.GetName(),
				status:     status,
			}

			program.Send(updateMsg{object: o})
		}
	}
}

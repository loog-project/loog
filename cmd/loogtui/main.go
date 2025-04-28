package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/expr-lang/expr"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	bboltStore "github.com/loog-project/loog/internal/store/bbolt"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
	"github.com/loog-project/loog/pkg/diffpreview"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	modeMaterializedObjectPretty = iota
	modeMaterializedObjectYAML
	modePatchPretty
	modePatchYAML
)

/* ------------------------------------------------------------------ */
/*                       flags & bootstrapping                        */
/* ------------------------------------------------------------------ */

var (
	flagKubeconfig string
	flagDB         string
	flagResources  util.StringSliceFlag
	flagSync       bool
	flagSnap       uint64
	flagExpr       string
)

func init() {
	flag.StringVar(&flagDB, "db", "output.bb", "bolt database file")
	flag.BoolVar(&flagSync, "sync-writes", true, "fsync every commit")
	flag.Uint64Var(&flagSnap, "snapshot-every", 8, "patches until snapshot")
	flag.StringVar(&flagExpr, "filter-expr", "All()", "expr filter")
	flag.Var(&flagResources, "resource", "group/version/resource (repeatable)")
	if h := homedir.HomeDir(); h != "" {
		flag.StringVar(&flagKubeconfig, "kubeconfig", filepath.Join(h, ".kube", "config"), "")
	} else {
		flag.StringVar(&flagKubeconfig, "kubeconfig", "", "")
	}
}

type commitMsg struct {
	Kind     string
	Res      string
	UID      string
	Time     time.Time
	Rev      store.RevisionID
	Patch    *store.Patch
	Snapshot *store.Snapshot
	Object   *unstructured.Unstructured
}

/* ------------------------------------------------------------------ */

func main() {
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	db, _ := bboltStore.New(flagDB, nil, flagSync)
	svc := service.NewTrackerService(db, flagSnap, true)

	cfg, _ := clientcmd.BuildConfigFromFlags("", flagKubeconfig)
	dyn, _ := dynamic.NewForConfig(cfg)

	var gvrs []schema.GroupVersionResource
	for _, r := range flagResources {
		g, _ := util.ParseGroupVersionResource(r)
		gvrs = append(gvrs, g)
	}
	watcher, err := util.NewMultiWatcher(ctx, dyn, gvrs, v1.ListOptions{})
	if err != nil {
		log.Fatal(err)
		return
	}
	defer watcher.Stop()

	prog, err := expr.Compile(flagExpr, expr.Env(util.EventEntryEnv{}), expr.AsBool())
	if err != nil {
		log.Fatal("Cannot compile filter:", err)
		return
	}

	commits := make(chan commitMsg, 512)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-watcher.ResultChan():
				obj, ok := ev.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}
				pass, _ := expr.Run(prog, util.EventEntryEnv{Event: ev, Object: obj})
				if !pass.(bool) {
					continue
				}

				obj.SetManagedFields(nil)
				rev, err := svc.Commit(ctx, string(obj.GetUID()), obj)
				if err != nil {
					fmt.Println("\n\n\n\n\nError committing:", err)
					continue
				}

				// read
				snapshot, patch, err := db.Get(ctx, string(obj.GetUID()), rev)
				if err != nil {
					fmt.Println("\n\n\n\n\nError reading:", err)
					continue
				}

				commits <- commitMsg{
					Kind:     obj.GetKind(),
					Res:      fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()),
					UID:      string(obj.GetUID()),
					Time:     time.Now(),
					Rev:      rev,
					Snapshot: snapshot,
					Patch:    patch,
					Object:   obj,
				}
			}
		}
	}()

	if err := tea.NewProgram(newModel(commits, svc, db)).Start(); err != nil {
		log.Fatal(err)
	}
}

/* ------------------------------------------------------------------ */
/*                         tree data structs                          */
/* ------------------------------------------------------------------ */

type revInfo struct {
	id  store.RevisionID
	msg commitMsg
}

type resEntry struct {
	uid      string
	lastSeen time.Time
	revs     []revInfo
	open     bool
}
type kindEntry struct {
	open bool
	res  map[string]*resEntry
}

/* ------------------------------------------------------------------ */
/*                              model                                 */
/* ------------------------------------------------------------------ */

type tickMsg struct{}

type model struct {
	commits <-chan commitMsg
	svc     *service.TrackerService
	store   store.ResourcePatchStore

	left  viewport.Model
	right viewport.Model

	focusRight             bool
	currentRightRenderMode int

	showHighlight bool

	kinds  map[string]*kindEntry
	order  []string
	cursor int
}

/* styles */
var (
	stKind  = lipgloss.NewStyle().Bold(true)
	stNS    = lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))
	stHot   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8700"))
	stDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#666"))
	stRev   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7d56f4"))
	stCur   = lipgloss.NewStyle().Foreground(lipgloss.Color("#00afff"))
	boxLeft = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	boxRsel = boxLeft.Copy().BorderForeground(lipgloss.Color("#00afff"))
	boxR    = boxLeft.Copy().BorderForeground(lipgloss.Color("#666"))
)

func newModel(ch <-chan commitMsg, svc *service.TrackerService, st store.ResourcePatchStore) *model {
	return &model{
		commits: ch, svc: svc, store: st,
		left:          viewport.New(0, 0),
		right:         viewport.New(0, 0),
		kinds:         make(map[string]*kindEntry),
		showHighlight: true,
	}
}

func nextCommit(ch <-chan commitMsg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(nextCommit(m.commits), tick())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	commands := []tea.Cmd{}

	switch v := msg.(type) {

	case tea.WindowSizeMsg:
		m.left.Width, m.left.Height = v.Width/2, v.Height-3
		m.right.Width, m.right.Height = v.Width-m.left.Width-6, v.Height-3

	case tea.KeyMsg:
		switch v.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.focusRight = !m.focusRight
		case "p":
			if m.currentRightRenderMode == modePatchPretty {
				m.currentRightRenderMode = modePatchYAML
			} else if m.currentRightRenderMode == modePatchYAML {
				m.currentRightRenderMode = modeMaterializedObjectPretty
			} else {
				m.currentRightRenderMode = (m.currentRightRenderMode + 1) % 4
			}
			m.renderRight()
		case "P":
			if m.currentRightRenderMode == modePatchYAML || m.currentRightRenderMode == modePatchPretty {
				m.currentRightRenderMode = modeMaterializedObjectYAML
			} else {
				m.currentRightRenderMode = modePatchPretty
			}
		case "h":
			m.showHighlight = !m.showHighlight
			m.renderRight()
		default:
			if m.focusRight {
				m.scrollRight(v)
				m.renderRight()
			} else {
				m.navigateLeft(v)
			}
		}

	case commitMsg:
		m.ingest(v)
		commands = append(commands, nextCommit(m.commits))

	case tickMsg:
		// refresh fade
		commands = append(commands, tick())
	}

	m.renderLeft()
	m.renderRight()

	return m, tea.Batch(commands...)
}

func (m *model) View() string {
	var rBox, lBox string
	if m.focusRight {
		lBox = boxR.Render(m.left.View())
		rBox = boxRsel.Render(m.right.View())
	} else {
		lBox = boxRsel.Render(m.left.View())
		rBox = boxR.Render(m.right.View())
	}
	bindings := []string{"↑↓ move/scroll", "←/→/⏎ toggle", "TAB focus"}
	//if m.focusRight {

	current := "unknown"
	switch m.currentRightRenderMode {
	case modeMaterializedObjectPretty:
		current = "materialized object (pretty)"
	case modeMaterializedObjectYAML:
		current = "materialized object (yaml)"
	case modePatchPretty:
		current = "patch (pretty)"
	case modePatchYAML:
		current = "patch (yaml)"
	}

	bindings = append(bindings, "p toggle patch (current: "+current+")")
	bindings = append(bindings, "h toggle highlight")
	//}
	bindings = append(bindings, "q quit")
	help := stDim.Render(strings.Join(bindings, "  "))
	return lipgloss.JoinHorizontal(lipgloss.Top, lBox, rBox) + "\n" + help
}

/* ------------------------------------------------------------------ */
/*                     left-pane logic                                */
/* ------------------------------------------------------------------ */

func (m *model) navigateLeft(k tea.KeyMsg) {
	switch k.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.ensureVisible()
		}
	case "down", "j":
		if m.cursor < m.totalLines()-1 {
			m.cursor++
			m.ensureVisible()
		}
	case "left":
		m.toggle(false)
	case "right", "enter", " ":
		m.toggle(true)
	}
}

func (m *model) ensureVisible() {
	if m.cursor < m.left.YOffset {
		m.left.YOffset = m.cursor
	}
	if m.cursor >= m.left.YOffset+m.left.Height {
		m.left.YOffset = m.cursor - m.left.Height + 1
	}
}

func (m *model) ingest(c commitMsg) {
	ke := m.kinds[c.Kind]
	if ke == nil {
		ke = &kindEntry{open: true, res: map[string]*resEntry{}}
		m.kinds[c.Kind] = ke
		m.order = append(m.order, c.Kind)
		slices.Sort(m.order)
	}
	re := ke.res[c.Res]
	if re == nil {
		re = &resEntry{uid: c.UID}
		ke.res[c.Res] = re
	}
	re.revs = append(re.revs, revInfo{
		id:  c.Rev,
		msg: c,
	})
	re.lastSeen = c.Time
}

func (m *model) toggle(exp bool) {
	line := 0
	for _, k := range m.order {
		if line == m.cursor {
			m.kinds[k].open = exp
			return
		}
		line++
		ke := m.kinds[k]
		if !ke.open {
			continue
		}
		for _, res := range sortedKeys(ke.res) {
			re := ke.res[res]
			if line == m.cursor {
				re.open = !re.open
				return
			}
			line++
			if re.open {
				line += len(re.revs)
			}
		}
	}
}

func (m *model) totalLines() int {
	n := 0
	for _, k := range m.order {
		n++
		ke := m.kinds[k]
		if !ke.open {
			continue
		}
		for _, r := range sortedKeys(ke.res) {
			n++
			if ke.res[r].open {
				n += len(ke.res[r].revs)
			}
		}
	}
	return n
}

func (m *model) renderLeft() {
	var sb strings.Builder
	now := time.Now()
	line := 0
	for _, kind := range m.order {
		_, _ = fmt.Fprintf(&sb, "%s %s %s\n",
			sel(line == m.cursor),
			expand(m.kinds[kind].open), stKind.Render(kind))

		line++
		ke := m.kinds[kind]
		if !ke.open {
			continue
		}

		for _, res := range sortedKeys(ke.res) {
			re := ke.res[res]
			style := stDim

			if now.Sub(re.lastSeen) < 3*time.Second {
				style = stHot
			}

			ns, name, _ := strings.Cut(res, "/")
			if len(ns) > 12 {
				ns = ns[len(ns)-12:] + "..."
			}

			info := fmt.Sprintf("[%d] %s", len(re.revs), relAge(now.Sub(re.lastSeen)))
			_, _ = fmt.Fprintf(&sb, "%s   %s %-36s %s\n",
				sel(line == m.cursor), expand(re.open),
				style.Render(stNS.Render(ns)+"/"+name),
				style.Render(info))

			line++
			if re.open {
				for _, r := range re.revs {
					age := now.Sub(r.msg.Time)

					idStr := ""
					if age <= 3*time.Second {
						idStr = stHot.Render(r.msg.Rev.String())
					} else {
						idStr = stRev.Render(r.msg.Rev.String())
					}

					typeStr := ""
					if r.msg.Patch != nil {
						typeStr = stDim.Render("patch")
					} else {
						typeStr = stCur.Render("snapshot")
					}

					_, _ = fmt.Fprintf(&sb, "%s       • %s [%s] (%s)\n",
						sel(line == m.cursor),
						idStr,
						typeStr,
						stDim.Render(relAge(time.Now().Sub(r.msg.Time))))

					line++
				}
			}
		}
	}
	m.left.SetContent(sb.String())
}

/* ------------------------------------------------------------------ */
/*                  right-pane scrolling & rendering                  */
/* ------------------------------------------------------------------ */

func setUnchanged(a *diffpreview.AnnotatedNode) {
	a.Change = diffpreview.Unchanged
	for _, child := range a.Children {
		setUnchanged(child)
	}
}

func (m *model) scrollRight(k tea.KeyMsg) {
	switch k.String() {
	case "up", "k":
		m.right.ScrollUp(1)
	case "down", "j":
		m.right.ScrollDown(1)
	case "right", "l":
		m.right.ScrollRight(1)
	case "left", "h":
		m.right.ScrollLeft(1)
	}
}

func (m *model) renderRight() {
	rev := m.currentSelection()
	if rev == nil {
		m.right.SetContent(stDim.Render("no revision selected"))
		return
	}

	var view any
	asJson := true

	curSnap, err := m.svc.Restore(context.Background(), rev.msg.UID, rev.msg.Rev)
	if err != nil {
		m.right.SetContent(stDim.Render("error reading revision: " + err.Error()))
	}

	prevSnap, err := m.svc.Restore(context.Background(), rev.msg.UID, rev.msg.Rev-1)
	if err != nil {
		m.right.SetContent(stDim.Render("error reading previous revision: " + err.Error()))
	}
	var other diffmap.DiffMap
	if prevSnap != nil {
		other = prevSnap.Object
	}

	switch m.currentRightRenderMode {
	case modePatchPretty:
		diff := diffmap.Diff(other, curSnap.Object)
		if diff != nil {
			previewDiff := diffpreview.Diff(other, curSnap.Object)
			view = diffpreview.RenderYAML(previewDiff, diffpreview.DarkTheme, diffpreview.RenderOptions{
				IndentSize:                2,
				EnableBackgroundHighlight: m.showHighlight,
			})
			asJson = false
		} else {
			view = stDim.Render("no difference between versions")
			asJson = false
		}
	case modePatchYAML:
		diff := diffmap.Diff(other, curSnap.Object)
		if diff != nil {
			view = diff
		} else {
			view = stDim.Render("no difference between versions")
			asJson = false
		}
	case modeMaterializedObjectPretty:
		diff := diffpreview.DiffRecursive(other, curSnap.Object)
		view = diffpreview.RenderYAML(diff, diffpreview.DarkTheme, diffpreview.RenderOptions{
			IndentSize:                2,
			EnableBackgroundHighlight: m.showHighlight,
		})
		asJson = false
	default:
		view = curSnap.Object
	}

	//if m.showPatch {
	//	if prevSnap != nil {
	//		diff := diffmap.Diff(prevSnap.Object, curSnap.Object)
	//		if diff != nil {
	//			view = diff
	//
	//			previewDiff := diffpreview.Diff(curSnap.Object, prevSnap.Object)
	//			view = diffpreview.RenderYAML(previewDiff, diffpreview.DarkTheme, diffpreview.RenderOptions{
	//				IndentSize:                2,
	//				EnableBackgroundHighlight: m.showHighlight,
	//			})
	//			asJson = false
	//		} else {
	//			view = stDim.Render("no difference between versions")
	//			asJson = false
	//		}
	//	} else {
	//		view = "no previous version"
	//	}
	//} else {
	//	view = curSnap.Object
	//
	//	var other diffmap.DiffMap
	//	if prevSnap != nil {
	//		other = prevSnap.Object
	//	}
	//
	//	diff := diffpreview.DiffRecursive(curSnap.Object, other)
	//	view = diffpreview.RenderYAML(diff, diffpreview.DarkTheme, diffpreview.RenderOptions{
	//		IndentSize:                2,
	//		EnableBackgroundHighlight: m.showHighlight,
	//	})
	//	asJson = false

	//spew.Dump(view)
	//asJson = false
	//}

	if asJson || view == nil {
		b, _ := json.MarshalIndent(view, "", "  ")
		m.right.SetContent(string(b))
	} else {
		str, ok := view.(string)
		if ok {
			m.right.SetContent(str)
		} else {
			m.right.SetContent(fmt.Sprintf("cannot render %T, value: %#v", view, view))
		}
	}
}

/* selection mapping */
func (m *model) currentSelection() *revInfo {
	line := 0
	for _, k := range m.order {
		if line == m.cursor {
			return nil
		}
		line++
		ke := m.kinds[k]
		if !ke.open {
			continue
		}
		for _, r := range sortedKeys(ke.res) {
			if line == m.cursor {
				return nil
			}
			line++
			re := ke.res[r]
			if re.open {
				for i := range re.revs {
					if line == m.cursor {
						return &re.revs[i]
					}
					line++
				}
			}
		}
	}
	return nil
}

/* utils */
func sel(b bool) string {
	if b {
		return stCur.Render("▶")
	}
	return " "
}
func expand(b bool) string {
	if b {
		return "▾"
	}
	return "▸"
}
func relAge(d time.Duration) string {
	if d < time.Second {
		return "now"
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

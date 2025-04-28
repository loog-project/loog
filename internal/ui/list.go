package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/loog-project/loog/internal/service"
	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/internal/util"
	"github.com/loog-project/loog/pkg/diffmap"
	"github.com/loog-project/loog/pkg/diffpreview"
)

const (
	arrowDown      = "▾"
	arrowRight     = "▸"
	pageScrollSkip = 5

	whereRevisionBanner = `
         .-"-.      
       _/_-.-_\_   WHERE 
      / __} {__ \     REVISION
     / //  "  \\ \      ???
    / / \'---'/ \ \`

	cannotShowRevisionBanner = `
        .-"-.
      _/.-.-.\_
     ( ( o o ) ) FUCK
      |/  "  \|     CANNOT
       \ .-. /          DISPLAY
      /       \`
)

type renderMode uint

const (
	modeShowObjectPretty = iota
	modeShowObjectJSON
	modeShowPatchPretty
	modeShowPatchJSON

	_modeMax // only a helper to get the number of modes
)

func (r renderMode) String() string {
	switch r {
	case modeShowObjectPretty:
		return "object (pretty)"
	case modeShowObjectJSON:
		return "object (json)"
	case modeShowPatchPretty:
		return "patch (pretty)"
	case modeShowPatchJSON:
		return "patch (json)"
	default:
		return "unknown"
	}
}

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

type ListView struct {
	Size
	eventChan chan<- tea.Msg

	trackerService *service.TrackerService
	rps            store.ResourcePatchStore

	left, right viewport.Model

	// tree data
	kinds map[string]*kindEntry
	order []string

	// ui state
	cursor     int
	focusRight bool
	renderMode renderMode
	highlight  bool
}

var _ View = (*ListView)(nil)

func NewListView(trackingService *service.TrackerService, rps store.ResourcePatchStore) *ListView {
	list := ListView{
		trackerService: trackingService,
		rps:            rps,

		left:  viewport.New(5, 5), // will be overwritten by SetSize
		right: viewport.New(5, 5), // will be overwritten by SetSize

		kinds:     make(map[string]*kindEntry),
		highlight: true,
	}
	return &list
}

func (lv *ListView) Breadcrumb() string {
	return "list"
}

func (lv *ListView) SetSize(width, height int) {
	lv.Size.SetSize(width, height)

	lv.left.Width, lv.left.Height = width/2-2, height-1
	lv.right.Width, lv.right.Height = width-(width/2-2)-4, height-1
}

func (lv *ListView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch v := msg.(type) {
	case commitMsg:
		lv.ingest(v)

	case tickMsg:
		// only re-render fade; handled in View()

	case tea.KeyMsg:
		if !lv.handleKey(v) {
			return lv, tea.Quit
		}

	case tea.MouseMsg: /* ignore */
	}

	lv.renderLeft()
	lv.renderRight()

	return lv, nil
}

func (lv *ListView) View() string {
	leftBox := util.Ternary(lv.focusRight, BorderIdle, BorderActive).Render(lv.left.View())
	rightBox := util.Ternary(lv.focusRight, BorderActive, BorderIdle).Render(lv.right.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (lv *ListView) KeyMap() string {
	return fmt.Sprintf("[mode: %s] [HL: %s] %s",
		StyleCur.Render(lv.renderMode.String()),
		util.Ternary(lv.highlight,
			StyleCur.Render("enabled"),
			StyleDim.Render("disabled")),
		NewShortcuts().
			// general shortcuts
			Add("TAB", "focus").
			Add("p", "change format").
			Add("h", util.Ternary(lv.highlight, "disable", "enable")+" highlight").
			Add("q", "quit").

			// left-only shortcuts
			AddIf(!lv.focusRight, "↑↓", "move").
			AddIf(!lv.focusRight, "pgup", "scroll up").
			AddIf(!lv.focusRight, "pgdn", "scroll down").
			AddIf(!lv.focusRight, "←→", "collapse").
			AddIf(!lv.focusRight, "⏎", "toggle").

			// right-only shortcuts
			AddIf(lv.focusRight, "↑↓←→", "move").
			Render())
}

/* ---------- listView helpers ---------- */

func (lv *ListView) handleKey(k tea.KeyMsg) bool {
	switch k.String() {
	case "q", "ctrl+c":
		return false // bubble up
	case "tab":
		lv.focusRight = !lv.focusRight
	case "p":
		lv.renderMode = (lv.renderMode + 1) % _modeMax
	case "h":
		lv.highlight = !lv.highlight
	default:
		if lv.focusRight {
			lv.scrollRight(k)
		} else {
			lv.navigateLeft(k)
		}
	}
	return true
}

func (lv *ListView) navigateLeft(k tea.KeyMsg) {
	switch k.String() {
	case "up", "k":
		if lv.cursor > 0 {
			lv.cursor--
			lv.keepVisible()
		}
	case "down", "j":
		if lv.cursor < lv.totalLines()-1 {
			lv.cursor++
			lv.keepVisible()
		}
	case "pgup":
		if lv.cursor > 0 {
			lv.cursor = int(math.Max(0, float64(lv.cursor-pageScrollSkip)))
			lv.keepVisible()
		}
	case "pgdown":
		if lv.cursor < lv.totalLines()-1 {
			lv.cursor = int(math.Min(float64(lv.totalLines()-1), float64(lv.cursor+pageScrollSkip)))
			lv.keepVisible()
		}
	case "left":
		lv.toggle(false)
	case "right", "enter", " ":
		lv.toggle(true)
	}
}

func (lv *ListView) keepVisible() {
	if lv.cursor < lv.left.YOffset {
		lv.left.YOffset = lv.cursor
	}
	if lv.cursor >= lv.left.YOffset+lv.left.Height {
		lv.left.YOffset = lv.cursor - lv.left.Height + 1
	}
}

func (lv *ListView) scrollRight(k tea.KeyMsg) {
	switch k.String() {
	case "up", "k":
		lv.right.ScrollUp(1)
	case "down", "j":
		lv.right.ScrollDown(1)
	}
}

/* ingest new commit */
func (lv *ListView) ingest(c commitMsg) {
	kind := c.Object.GetKind()
	res := fmt.Sprintf("%s::%s", c.Object.GetNamespace(), c.Object.GetName())
	uid := string(c.Object.GetUID())
	rev := c.Revision

	ke := lv.kinds[kind]
	if ke == nil {
		ke = &kindEntry{open: true, res: map[string]*resEntry{}}
		lv.kinds[kind] = ke
		lv.order = append(lv.order, kind)
		slices.Sort(lv.order)
	}
	re := ke.res[res]
	if re == nil {
		re = &resEntry{uid: uid}
		ke.res[res] = re
	}
	re.revs = append(re.revs, revInfo{id: rev, msg: c})
	re.lastSeen = c.Time
}

/* tree toggles */
func (lv *ListView) toggle(exp bool) {
	line := 0
	for _, k := range lv.order {
		if line == lv.cursor {
			lv.kinds[k].open = exp
			return
		}
		line++
		ke := lv.kinds[k]
		if !ke.open {
			continue
		}
		for _, r := range sortedKeys(ke.res) {
			re := ke.res[r]
			if line == lv.cursor {
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

func (lv *ListView) totalLines() int {
	n := 0
	for _, k := range lv.order {
		n++
		ke := lv.kinds[k]
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

/* render left pane */
func (lv *ListView) renderLeft() {
	var b strings.Builder
	now := time.Now()
	line := 0

	for _, kind := range lv.order {
		kindEntryInfo := lv.kinds[kind]

		isSelected := lv.cursor == line
		isExpanded := kindEntryInfo.open

		_, _ = fmt.Fprintf(&b, "%s %s %s\n",
			util.Ternary(isSelected, StyleCur.Render(arrowRight), " "),
			util.Ternary(isExpanded, arrowDown, arrowRight),
			StyleKind.Render(kind),
		)

		line++
		if !isExpanded {
			continue
		}

		for _, res := range sortedKeys(kindEntryInfo.res) {
			resourceEntry := kindEntryInfo.res[res]

			isSelected := lv.cursor == line
			isExpanded := resourceEntry.open

			// orange blink if recently seen
			style := StyleDim
			if now.Sub(resourceEntry.lastSeen) < 3*time.Second {
				style = StyleHot
			}

			ns, name, _ := strings.Cut(res, "/")
			if len(ns) > 12 {
				ns = "..." + ns[len(ns)-11:]
			}
			info := fmt.Sprintf("[%d] %s",
				len(resourceEntry.revs),
				elapsedTime(now.Sub(resourceEntry.lastSeen)))

			_, _ = fmt.Fprintf(&b, "%s   %s %-32s %s\n",
				util.Ternary(isSelected, StyleCur.Render(arrowRight), " "),
				util.Ternary(isExpanded, arrowDown, arrowRight),
				style.Render(StyleNS.Render(ns)+"/"+name), style.Render(info))
			line++

			if resourceEntry.open {
				for _, rv := range resourceEntry.revs {
					isSelected := lv.cursor == line

					revisionKind := "snap"
					if rv.msg.Patch != nil {
						revisionKind = "patch"
					}

					_, _ = fmt.Fprintf(&b, "       • %s%s%s [%s] %s\n",
						util.Ternary(isSelected, StyleCur.Render("["), " "),
						util.Ternary(isSelected, StyleCur, StyleRev).Render(rv.id.String()),
						util.Ternary(isSelected, StyleCur.Render("]"), " "),
						StyleDim.Render(revisionKind),
						StyleDim.Render(elapsedTime(now.Sub(rv.msg.Time))))
					line++
				}
			}
		}
	}
	lv.left.SetContent(b.String())
}

/* render right pane */
func (lv *ListView) renderRight() {
	rev := lv.currentSelection()
	if rev == nil {
		lv.right.SetContent(StyleDim.Render(whereRevisionBanner))
		return
	}

	uid := string(rev.msg.Object.GetUID())
	curSnap, err := lv.trackerService.Restore(context.Background(), uid, rev.msg.Revision)
	if err != nil {
		// that's fatal :/
		lv.eventChan <- NewAlertCommand("when restoring snapshot", err)
		lv.right.SetContent(StyleDim.Render(cannotShowRevisionBanner))
		return
	}

	prevSnap, err := lv.trackerService.Restore(context.Background(), uid, rev.msg.Revision-1)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		lv.eventChan <- NewAlertCommand("when restoring previous snapshot", err)
		// but we can still show the current one
	}

	var previousObject diffmap.DiffMap
	if prevSnap != nil {
		previousObject = prevSnap.Object
	}

	var (
		asJSON any
		asStr  string
	)

	switch lv.renderMode {
	case modeShowObjectJSON:
		// simplest case: just display pretty-printed JSON of the object
		asJSON = curSnap.Object

	case modeShowObjectPretty:
		// show the object in a pretty-printed format
		diff := diffpreview.DiffRecursive(previousObject, curSnap.Object)
		asStr = diffpreview.RenderYAML(diff, diffpreview.DarkTheme, diffpreview.RenderOptions{
			IndentSize:                2,
			EnableBackgroundHighlight: lv.highlight,
		})

	case modeShowPatchJSON:
		// show the patch in JSON format
		diff := diffmap.Diff(previousObject, curSnap.Object)
		if diff != nil {
			asJSON = diff
		} else {
			asStr = StyleDim.Render("no difference between versions")
		}

	case modeShowPatchPretty:
		diff := diffmap.Diff(previousObject, curSnap.Object)
		if diff != nil {
			previewDiff := diffpreview.Diff(previousObject, curSnap.Object)
			asStr = diffpreview.RenderYAML(previewDiff, diffpreview.DarkTheme, diffpreview.RenderOptions{
				IndentSize:                2,
				EnableBackgroundHighlight: lv.highlight,
			})
		} else {
			asStr = StyleDim.Render("no difference between versions")
		}

	default:
		// this should never happen, but just in case
		asStr = "I have no idea what to show you here"
	}

	if asStr != "" {
		lv.right.SetContent(asStr)
		return
	}
	j, err := json.MarshalIndent(asJSON, "", "  ")
	if err != nil {
		lv.right.SetContent(StyleDim.Render("error marshalling: " + err.Error()))
		return
	}
	lv.right.SetContent(string(j))
}

/* current selection */
func (lv *ListView) currentSelection() *revInfo {
	line := 0
	for _, k := range lv.order {
		if line == lv.cursor {
			return nil
		}
		line++
		ke := lv.kinds[k]
		if !ke.open {
			continue
		}
		for _, r := range sortedKeys(ke.res) {
			if line == lv.cursor {
				return nil
			}
			line++
			re := ke.res[r]
			if re.open {
				for i := range re.revs {
					if line == lv.cursor {
						return &re.revs[i]
					}
					line++
				}
			}
		}
	}
	return nil
}

/*=====================================================================*/
/*                       8. helpers / UI bits                          */
/*=====================================================================*/

func elapsedTime(d time.Duration) string {
	if d < time.Second {
		return "now"
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

func sortedKeys[K ~string, V any](m map[K]V) []K {
	ks := make([]K, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

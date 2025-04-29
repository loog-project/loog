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
	"github.com/loog-project/loog/pkg/diffmap"
	"github.com/loog-project/loog/pkg/diffpreview"
)

const (
	arrowDown  = "▾"
	arrowRight = "▸"

	pageScrollSkip = 5
	sizeSkip       = 2

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
	Base

	trackerService *service.TrackerService
	rps            store.ResourcePatchStore

	left, right viewport.Model
	leftExtra   int

	// tree data
	kinds map[string]*kindEntry
	order []string

	// ui state
	cursor     int
	focusRight bool
	renderMode renderMode
	fullscreen bool
	highlight  bool
}

var _ View = (*ListView)(nil)

func NewListView(trackingService *service.TrackerService, rps store.ResourcePatchStore) *ListView {
	list := ListView{
		trackerService: trackingService,
		rps:            rps,

		left:      viewport.New(5, 5), // will be overwritten by SetSize
		right:     viewport.New(5, 5), // will be overwritten by SetSize
		leftExtra: 0,

		kinds:      make(map[string]*kindEntry),
		highlight:  false, // highlight is disabled by default
		fullscreen: false, // fullscreen is disabled by default
	}
	return &list
}

func (lv *ListView) Breadcrumb() string {
	return "list"
}

func (lv *ListView) calculateViewportSizes() {
	if lv.fullscreen {
		lv.right.Width = lv.Width
		lv.right.Height = lv.Height + 1 // no status bar
	} else {
		leftWidth := (lv.Width/2 + lv.leftExtra) - 2
		lv.left.Width, lv.left.Height = leftWidth, lv.Height-1

		rightWidth := lv.Width - leftWidth - 4
		lv.right.Width, lv.right.Height = rightWidth, lv.Height-1
	}
}

// SetSize sets the size of the left and right panes.
// It is overridden from the Base struct to be able to set the size of the left and right panes
// based on the current mode (fullscreen or not).
func (lv *ListView) SetSize(width, height int) {
	lv.Base.SetSize(width, height)
	lv.calculateViewportSizes()
}

func (lv *ListView) Update(msg tea.Msg) (View, tea.Cmd) {
	switch v := msg.(type) {
	case commitMsg:
		lv.ingest(v)

	case tickMsg:
		// only re-render fade; handled in View()

	case tea.KeyMsg:
		if cmd := lv.handleKey(v); cmd != nil {
			return lv, cmd
		}

	case tea.MouseMsg: /* ignore */
	}

	if cmd := lv.renderLeft(); cmd != nil {
		return lv, cmd
	}
	if cmd := lv.renderRight(); cmd != nil {
		return lv, cmd
	}

	return lv, nil
}

func (lv *ListView) View() string {
	if lv.fullscreen {
		return lv.right.View()
	}
	leftBox := ternary(lv.focusRight, lv.Theme.BorderIdleContainerStyle, lv.Theme.BorderActiveContainerStyle).
		Render(lv.left.View())
	rightBox := ternary(lv.focusRight, lv.Theme.BorderActiveContainerStyle, lv.Theme.BorderIdleContainerStyle).
		Render(lv.right.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
}

func (lv *ListView) KeyMap() string {
	return fmt.Sprintf("[mode: %s] %s",
		lv.Theme.PrimaryTextStyle.Render(lv.renderMode.String()),
		NewShortcuts().
			// general shortcuts
			Add("q", "quit").
			Add("⇥", "focus").
			Add("p", "patch").
			Add("h", "highlight "+ternary(lv.highlight, "off", "on")).

			// left-only shortcuts
			AddIf(!lv.focusRight, "↑/↓/pgup/pgdn", "scroll").
			AddIf(!lv.focusRight, "←/→", "collapse").
			AddIf(!lv.focusRight, "⏎", "toggle").

			// right-only shortcuts
			AddIf(lv.focusRight, "↑/↓/←/→", "move").
			AddIf(lv.focusRight, "f", "fullscreen").
			Render(lv.Theme))
}

/* ---------- listView helpers ---------- */

func (lv *ListView) handleKey(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "tab":
		lv.focusRight = !lv.focusRight
	case "p":
		lv.renderMode = (lv.renderMode + 1) % _modeMax
	case "h":
		lv.highlight = !lv.highlight
	case "f":
		lv.fullscreen = !lv.fullscreen
		lv.SetSize(lv.Width, lv.Height)
	case "+":
		maxExtra := (lv.Width / 2) - 8
		if lv.leftExtra < maxExtra {
			lv.leftExtra = int(math.Min(float64(lv.leftExtra+sizeSkip), float64(maxExtra)))
			lv.calculateViewportSizes()
		}
	case "-":
		minExtra := -(lv.Width / 2) + 8
		if lv.leftExtra > minExtra {
			lv.leftExtra = int(math.Max(float64(lv.leftExtra-sizeSkip), float64(minExtra)))
			lv.calculateViewportSizes()
		}
	default:
		if lv.focusRight {
			return scrollViewport(k, &lv.right)
		} else {
			return lv.navigateLeft(k)
		}
	}
	return nil
}

func (lv *ListView) navigateLeft(k tea.KeyMsg) tea.Cmd {
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
	case "right", "enter", "l", " ":
		lv.toggle(true)
	}
	return nil
}

func (lv *ListView) keepVisible() {
	if lv.cursor < lv.left.YOffset {
		lv.left.YOffset = lv.cursor
	}
	if lv.cursor >= lv.left.YOffset+lv.left.Height {
		lv.left.YOffset = lv.cursor - lv.left.Height + 1
	}
}

func scrollViewport(k tea.KeyMsg, vp *viewport.Model) tea.Cmd {
	switch k.String() {
	case "up", "k":
		vp.ScrollUp(1)
	case "down", "j":
		vp.ScrollDown(1)
	case "pgup":
		vp.PageUp()
	case "pgdown":
		vp.PageDown()
	case "left":
		vp.ScrollLeft(1)
	case "right":
		vp.ScrollRight(1)
	}
	return nil
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
func (lv *ListView) renderLeft() tea.Cmd {
	var b strings.Builder
	now := time.Now()
	line := 0

	for _, kind := range lv.order {
		kindEntryInfo := lv.kinds[kind]

		isSelected := lv.cursor == line
		isExpanded := kindEntryInfo.open

		_, _ = fmt.Fprintf(&b, "%s %s %s\n",
			ternary(isSelected, lv.Theme.ListCurrentArrowTextStyle.Render(arrowRight), " "),
			ternary(isExpanded, arrowDown, arrowRight),
			lv.Theme.ListKindNameTextStyle.Render(kind),
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
			style := lv.Theme.MutedTextStyle
			if now.Sub(resourceEntry.lastSeen) < 3*time.Second {
				style = lv.Theme.ListActivityTextStyle
			}

			ns, name, _ := strings.Cut(res, "::")
			if len(ns) > 12 {
				ns = "..." + ns[len(ns)-11:]
			}
			info := fmt.Sprintf("[%d] %s",
				len(resourceEntry.revs),
				elapsedTime(now.Sub(resourceEntry.lastSeen)))

			_, _ = fmt.Fprintf(&b, "%s   %s %-32s %s\n",
				ternary(isSelected, lv.Theme.ListCurrentArrowTextStyle.Render(arrowRight), " "),
				ternary(isExpanded, arrowDown, arrowRight),
				style.Render(lv.Theme.ListNamespaceTextStyle.Render(ns)+"/"+name), style.Render(info))
			line++

			if resourceEntry.open {
				for _, rv := range resourceEntry.revs {
					isSelected := lv.cursor == line

					revisionKind := "snap"
					if rv.msg.Patch != nil {
						revisionKind = "patch"
					}

					_, _ = fmt.Fprintf(&b, "       • %s%s%s [%s] %s\n",
						ternary(isSelected, lv.Theme.ListCurrentArrowTextStyle.Render("["), " "),
						ternary(isSelected, lv.Theme.ListCurrentArrowTextStyle, lv.Theme.ListRevisionTextStyle).
							Render(rv.id.String()),
						ternary(isSelected, lv.Theme.ListCurrentArrowTextStyle.Render("]"), " "),
						lv.Theme.MutedTextStyle.Render(revisionKind),
						lv.Theme.MutedTextStyle.Render(elapsedTime(now.Sub(rv.msg.Time))))
					line++
				}
			}
		}
	}
	lv.left.SetContent(b.String())
	return nil
}

// TODO: only re-render if the revision changes
func (lv *ListView) renderRight() tea.Cmd {
	rev := lv.currentSelection()
	if rev == nil {
		lv.right.SetContent(lv.Theme.MutedTextStyle.Render(whereRevisionBanner))
		return nil
	}

	uid := string(rev.msg.Object.GetUID())
	curSnap, err := lv.trackerService.Restore(context.Background(), uid, rev.msg.Revision)
	if err != nil {
		// that's fatal :/
		return PushAlert("when restoring snapshot", err)
	}

	prevSnap, err := lv.trackerService.Restore(context.Background(), uid, rev.msg.Revision-1)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return PushAlert("when restoring previous snapshot", err)
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
			asStr = lv.Theme.MutedTextStyle.Render("no difference between versions")
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
			asStr = lv.Theme.MutedTextStyle.Render("no difference between versions")
		}

	default:
		// this should never happen, but just in case
		asStr = "I have no idea what to show you here"
	}

	if asStr != "" {
		lv.right.SetContent(asStr)
		return nil
	}
	j, err := json.MarshalIndent(asJSON, "", "  ")
	if err != nil {
		lv.right.SetContent(cannotShowRevisionBanner + "\n\n" +
			lv.Theme.ErrorTextStyle.Render("error marshalling: "+err.Error()))
		return nil
	}
	lv.right.SetContent(string(j))
	return nil
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

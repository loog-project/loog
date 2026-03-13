package views

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/internal/ui/bus"
	"github.com/loog-project/loog/internal/ui/core"
)

type SelectionChangedMsg struct {
	ObjectUID string
	Revision  store.RevisionID
}

type revInfo struct {
	uid string
	id  store.RevisionID
}
type resEntry struct {
	revs []revInfo
	open bool
}
type kindEntry struct {
	open bool
	res  map[string]*resEntry
}

type selectorView struct {
	core.Sizer
	core.Themer

	cursor int

	kinds map[string]*kindEntry
	order []string
}

func NewSelectorView() core.View {
	return &selectorView{
		kinds: make(map[string]*kindEntry),
	}
}

func (sv *selectorView) Init() tea.Cmd {
	return core.Noop
}

func (sv *selectorView) Update(msg tea.Msg) (core.View, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.CommitMessage:
		sv.ingest(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if sv.cursor > 0 {
				sv.cursor--
				return sv, sv.emitChange()
			}
		case "down", "j":
			if sv.cursor < sv.totalLines()-1 {
				sv.cursor++
				return sv, sv.emitChange()
			}
		case "right", "l", "enter":
			sv.toggle(true)
		case "left", "h":
			sv.toggle(false)
		}
	}
	return sv, nil
}

func (sv *selectorView) View() string {
	var b strings.Builder
	line := 0

	for _, kind := range sv.order {
		ke := sv.kinds[kind]
		selected := line == sv.cursor
		_, _ = fmt.Fprintf(&b, "%s %s\n",
			sv.arrow(selected),
			sv.Theme.ListKindNameTextStyle.Render(kind),
		)
		line++
		if !ke.open {
			continue
		}
		for resKey, re := range ke.res {
			selected := line == sv.cursor
			_, _ = fmt.Fprintf(&b, " %s %s [%d]\n",
				sv.arrow(selected),
				sv.Theme.ListRevisionTextStyle.Render(resKey),
				len(re.revs),
			)
			line++
			if re.open {
				for _, r := range re.revs {
					selected := line == sv.cursor
					_, _ = fmt.Fprintf(&b, "   %s %04x\n", sv.arrow(selected), uint64(r.id))
					line++
				}
			}
		}
	}
	return b.String()
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (sv *selectorView) arrow(selected bool) string {
	if selected {
		return sv.Theme.ListCurrentArrowTextStyle.Render("▶")
	}
	return " "
}

func (sv *selectorView) ingest(msg bus.CommitMessage) {
	ke := sv.kinds[msg.Kind]
	if ke == nil {
		ke = &kindEntry{open: true, res: map[string]*resEntry{}}
		sv.kinds[msg.Kind] = ke
		sv.order = append(sv.order, msg.Kind)
		slices.Sort(sv.order)
	}

	key := fmt.Sprintf("%s/%s", msg.Namespace, msg.Name)
	re := ke.res[key]
	if re == nil {
		re = &resEntry{open: false}
		ke.res[key] = re
	}
	re.revs = append(re.revs, revInfo{uid: msg.UID, id: msg.Revision})
}

func (sv *selectorView) toggle(open bool) {
	line := 0
	for _, k := range sv.order {
		if line == sv.cursor {
			sv.kinds[k].open = open
			return
		}
		line++

		ke := sv.kinds[k]
		if !ke.open {
			continue
		}

		for _, re := range ke.res {
			if line == sv.cursor {
				re.open = open
				return
			}
			line++

			if re.open {
				line += len(re.revs)
			}
		}
	}
}

func (sv *selectorView) totalLines() int {
	n := 0
	for _, k := range sv.order {
		n++
		ke := sv.kinds[k]
		if !ke.open {
			continue
		}
		for _, re := range ke.res {
			n++
			if re.open {
				n += len(re.revs)
			}
		}
	}
	return n
}

func (sv *selectorView) current() (string, store.RevisionID) {
	line := 0
	for _, k := range sv.order {
		if line == sv.cursor {
			return "", 0
		}
		line++
		ke := sv.kinds[k]
		if !ke.open {
			continue
		}
		for _, re := range ke.res {
			if line == sv.cursor {
				return "", 0
			}
			line++
			if re.open {
				for _, rv := range re.revs {
					if line == sv.cursor {
						return rv.uid, rv.id
					}
					line++
				}
			}
		}
	}
	return "", 0
}

func (sv *selectorView) emitChange() tea.Cmd {
	uid, rev := sv.current()
	return func() tea.Msg {
		return SelectionChangedMsg{ObjectUID: uid, Revision: rev}
	}
}

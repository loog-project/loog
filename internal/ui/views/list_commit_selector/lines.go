package list_commit_selector

import (
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/core"
	"github.com/loog-project/loog/internal/ui/theme"
)

type line interface {
	Key() string
	View(theme theme.Theme, isSelected bool) string
}

type handleKey interface {
	HandleKey(m *model, key tea.KeyMsg) (core.View, tea.Cmd)
}

type onSelect interface {
	OnSelect(m *model) (core.View, tea.Cmd)
}

type kindHeaderLine struct {
	kind string
}

func (l kindHeaderLine) Key() string {
	return Join("kind", l.kind)
}

func (l kindHeaderLine) View(_ theme.Theme, isSelected bool) string {
	var bob strings.Builder
	if isSelected {
		bob.WriteRune('[')
	} else {
		bob.WriteRune(' ')
	}
	bob.WriteString(l.kind)
	if isSelected {
		bob.WriteRune(']')
	} else {
		bob.WriteRune(' ')
	}
	return bob.String()
}

func (l kindHeaderLine) HandleKey(m *model, key tea.KeyMsg) (core.View, tea.Cmd) {
	if key.String() == "enter" {

	}
	return m, core.Noop
}

type resourceNameLine struct {
	parent       line
	resourceName string

	lastActivity  time.Time
	revisionCount uint
}

func (l resourceNameLine) Key() string {
	return Join(l.parent.Key(), "resource", l.resourceName)
}

func (l resourceNameLine) View(_ theme.Theme, isSelected bool) string {
	var bob strings.Builder
	bob.WriteString("  ")
	if isSelected {
		bob.WriteRune('[')
	} else {
		bob.WriteRune(' ')
	}
	bob.WriteString(l.resourceName)
	if l.revisionCount > 0 {
		bob.WriteString(" (" + strconv.Itoa(int(l.revisionCount)) + " revisions)")
	}
	if !l.lastActivity.IsZero() {
		bob.WriteString(" (last activity: " + l.lastActivity.Format(time.RFC3339) + ")")
	}
	if isSelected {
		bob.WriteRune(']')
	} else {
		bob.WriteRune(' ')
	}
	return bob.String()
}

type revisionLine struct {
	parent line

	revisionID   string
	lastActivity time.Time
}

func (l revisionLine) Key() string {
	return Join(l.parent.Key(), "revision", l.revisionID)
}

func (l revisionLine) View(_ theme.Theme, isSelected bool) string {
	var bob strings.Builder
	bob.WriteString("    ")
	if isSelected {
		bob.WriteRune('[')
	} else {
		bob.WriteRune(' ')
	}
	bob.WriteString(l.revisionID)
	if !l.lastActivity.IsZero() {
		bob.WriteString(" (last activity: " + l.lastActivity.Format(time.RFC3339) + ")")
	}
	if isSelected {
		bob.WriteRune(']')
	} else {
		bob.WriteRune(' ')
	}
	return bob.String()
}

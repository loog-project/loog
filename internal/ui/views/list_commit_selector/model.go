package list_commit_selector

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/internal/ui/bus"
	"github.com/loog-project/loog/internal/ui/core"
)

type keyMap struct {
	LineUp   key.Binding
	LineDown key.Binding
}

var defaultKeyMap = keyMap{
	LineDown: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("j/down", "move down"),
	),
	LineUp: key.NewBinding(
		key.WithKeys("k", "up"),
		key.WithHelp("k/up", "move up"),
	),
}

func NewModel() core.View {
	m := model{
		keyMap:    defaultKeyMap,
		revisions: []patchOrSnapshot{},
	}

	m.modifyLines(func(lines *[]line) {
		*lines = append(*lines, kindHeaderLine{
			kind: "Instances",
		})
		*lines = append(*lines, resourceNameLine{
			parentKind:    "Instances",
			resourceName:  "tom-mihai-uwe-daniel",
			lastActivity:  time.Now(),
			revisionCount: 2,
		})
		*lines = append(*lines, revisionLine{
			parentKind:     "Instances",
			parentResource: "tom-mihai-uwe-daniel",
			revisionID:     "a",
			lastActivity:   time.Now().Add(-time.Hour),
		})
		*lines = append(*lines, revisionLine{
			parentKind:     "Instances",
			parentResource: "tom-mihai-uwe-daniel",
			revisionID:     "b",
			lastActivity:   time.Now().Add(-2 * time.Hour),
		})

		*lines = append(*lines, kindHeaderLine{
			kind: "ServiceBindings",
		})
		*lines = append(*lines, resourceNameLine{
			parentKind:    "ServiceBindings",
			resourceName:  "my-service-binding",
			lastActivity:  time.Now().Add(-3 * time.Hour),
			revisionCount: 1,
		})
		*lines = append(*lines, revisionLine{
			parentKind:     "ServiceBindings",
			parentResource: "my-service-binding",
			revisionID:     "a",
			lastActivity:   time.Now().Add(-3 * time.Hour),
		})
		*lines = append(*lines, kindHeaderLine{
			kind: "Spaces",
		})
	})

	return &m
}

type patchOrSnapshot struct {
	patch    *store.Patch
	snapshot *store.Snapshot
}

var _ core.View = (*model)(nil)

type model struct {
	core.Sizer
	core.Themer
	
	keyMap    keyMap
	revisions []patchOrSnapshot

	lines         []line
	lineCursorKey string
}

func (m *model) modifyLines(mutate func(*[]line)) {
	mutate(&m.lines)
	if m.lineCursorKey == "" && len(m.lines) > 0 {
		m.lineCursorKey = m.lines[0].Key()
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) currentLineIndex() (int, bool) {
	for i, line := range m.lines {
		if line.Key() == m.lineCursorKey {
			return i, true
		}
	}
	return -1, false // Not found
}

func (m *model) Update(msg tea.Msg) (core.View, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.LineDown):
			currentLineIndex, found := m.currentLineIndex()
			if !found {
				return m, bus.Emit(bus.ErrorMessage{
					Title:   "List Error",
					Message: "Could not find the current line to move down.",
				})
			}
			nextLineIndex := currentLineIndex + 1
			if nextLineIndex >= len(m.lines) {
				return m, nil // No more lines to move down to
			}
			m.lineCursorKey = m.lines[nextLineIndex].Key()
			return m, nil

		case key.Matches(msg, m.keyMap.LineUp):
			currentLineIndex, found := m.currentLineIndex()
			if !found {
				return m, bus.Emit(bus.ErrorMessage{
					Title:   "List Error",
					Message: "Could not find the current line to move up.",
				})
			}
			prevLineIndex := currentLineIndex - 1
			if prevLineIndex < 0 {
				return m, nil // No more lines to move up to
			}
			m.lineCursorKey = m.lines[prevLineIndex].Key()
			return m, nil
		}
	}
	return m, nil
}

func (m *model) View() string {
	lines := make([]string, len(m.lines))

	for i, line := range m.lines {
		isActive := m.lineCursorKey == line.Key()
		lines[i] = line.View(isActive)
	}

	return strings.Join(lines, "\n")
}

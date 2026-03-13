package main

import (
	"crypto/rand"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Debounce[T any] struct {
	tag int
}

var (
	keyQuit = key.NewBinding(key.WithKeys("q"))
	keyDown = key.NewBinding(key.WithKeys("down", "j"))
	keyUp   = key.NewBinding(key.WithKeys("up", "k"))

	selected = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)
var _ tea.Model = model{}

type model struct {
	items    []string
	cursor   uint
	selected string

	latestTag     int
	debounceTimer time.Timer
}

func initModel() model {
	items := make([]string, 0, 50)
	for i := 0; i < cap(items); i++ {
		items = append(items, rand.Text())
	}
	return model{
		items:  items,
		cursor: 0,
	}
}

type switchMsg struct {
	selected string
	tag      int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keyQuit):
			return m, tea.Quit
		case key.Matches(msg, keyDown):
			if m.cursor < uint(len(m.items)-1) {
				m.cursor++
				m.latestTag++
				// return m, Debounce(500 * time.Millisecond, func() tea.Msg {
				// 	return switchMsg{selected: ...}
				// }
				return m, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
					return switchMsg{
						selected: m.items[m.cursor],
						tag:      m.latestTag,
					}
				})
			}
		case key.Matches(msg, keyUp):
			if m.cursor > 0 {
				m.cursor--
				m.latestTag++
				return m, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
					return switchMsg{
						selected: m.items[m.cursor],
						tag:      m.latestTag,
					}
				})
			}
		}
	case switchMsg:
		if msg.tag == m.latestTag {
			m.selected = msg.selected
		}
	}
	return m, nil
}

func (m model) View() string {
	view := "Items:\n"

	for i, item := range m.items {
		style := lipgloss.NewStyle()
		if item == m.selected {
			style = selected
		}
		var txt string
		if i == int(m.cursor) {
			txt = "> " + item
		} else {
			txt = "  " + item
		}
		view += style.Render(txt) + "\n"
	}
	view += "\n\nSelected: " + m.selected
	view += "\n\nPress q to quit.\n"
	return view
}

func main() {
	m := initModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		panic(err)
	}
}

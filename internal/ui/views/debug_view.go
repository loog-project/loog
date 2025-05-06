package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/core"
)

type debugView struct {
	commits []SelectionChangedMsg
}

func NewDebugView() core.View {
	return &debugView{}
}

func (dv *debugView) Init() tea.Cmd {
	return core.Noop
}

func (dv *debugView) Update(msg tea.Msg) (core.View, tea.Cmd) {
	switch msg := msg.(type) {
	case SelectionChangedMsg:
		dv.commits = append(dv.commits, msg)
	}
	return dv, core.Noop
}

func (dv *debugView) View() string {
	var bob strings.Builder

	for _, commit := range dv.commits {
		bob.WriteString(fmt.Sprintf("%#v", commit))
		bob.WriteString("\n")
	}

	return bob.String()
}

package core

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/ui/theme"
)

type View interface {
	Init() tea.Cmd
	Update(tea.Msg) (View, tea.Cmd)
	View() string
}

type Sizeable interface {
	SetSize(width, height int)
}

type Themeable interface {
	SetTheme(theme theme.Theme)
}

// wrapView is a wrapper for a View that implements the Model interface
type wrapView struct {
	tea.Model
}

func (w *wrapView) Init() tea.Cmd {
	return w.Model.Init()
}

func (w *wrapView) Update(msg tea.Msg) (View, tea.Cmd) {
	var cmd tea.Cmd
	w.Model, cmd = w.Model.Update(msg)
	return w, cmd
}

func (w *wrapView) View() string {
	return w.Model.View()
}

func Wrap(model tea.Model) View {
	return &wrapView{Model: model}
}

// primitiveView is a wrapper for a View that just returns some string
type primitiveView struct {
	content string
}

func (p *primitiveView) Init() tea.Cmd {
	return nil
}

func (p *primitiveView) Update(msg tea.Msg) (View, tea.Cmd) {
	return p, nil
}

func (p *primitiveView) View() string {
	return p.content
}

func Primitive(content string) View {
	return &primitiveView{content: content}
}

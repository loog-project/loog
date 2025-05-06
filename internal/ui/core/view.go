package core

import (
	tea "github.com/charmbracelet/bubbletea"
)

type View interface {
	Init() tea.Cmd
	Update(tea.Msg) (View, tea.Cmd)
	View() string
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

// Primitive is a wrapper for a View that just returns some string
type Primitive string

func (p Primitive) Init() tea.Cmd {
	return nil
}

func (p Primitive) Update(msg tea.Msg) (View, tea.Cmd) {
	return p, nil
}

func (p Primitive) View() string {
	return string(p)
}

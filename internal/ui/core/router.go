package core

import "github.com/loog-project/loog/internal/ui/theme"

type Router struct {
	stack []View
}

func NewRouter(initialView View) *Router {
	return &Router{
		stack: []View{initialView},
	}
}

func (r *Router) Current() View {
	return r.stack[len(r.stack)-1]
}

func (r *Router) Push(view View) {
	r.stack = append(r.stack, view)
}

func (r *Router) Pop() {
	if len(r.stack) > 1 {
		r.stack = r.stack[:len(r.stack)-1]
	}
}

func (r *Router) Replace(newView View) {
	r.stack[len(r.stack)-1] = newView
}

func (r *Router) PushWindowMeta(newTheme theme.Theme, width, height int) {
	for _, view := range r.stack {
		if s, ok := view.(Sizeable); ok {
			s.SetSize(width, height)
		}
		if t, ok := view.(Themeable); ok {
			t.SetTheme(newTheme)
		}
	}
}

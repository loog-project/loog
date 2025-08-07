package zack

// Router is a simple stacked-based router for managing views in a terminal application.
type Router[T any] struct {
	stack []T
}

// NewRouter creates a new Router with an initial view.
// The initial view is pushed onto the stack and cannot be removed (as one view is required).
// You can, however, replace it with another view using the [Replace] method.
//
// Note: currently, the Router only serves as a simple stack, but we may extend it in the future
// to support more complex routing features, such as named routes, parameters, etc.
func NewRouter[T any](initialModel T) *Router[T] {
	return &Router[T]{
		stack: []T{initialModel},
	}
}

// Peek returns the current view at the top of the stack without removing it.
func (r *Router[T]) Peek() T {
	return r.stack[len(r.stack)-1]
}

// Push adds a new view to the top of the stack.
func (r *Router[T]) Push(model T) {
	r.stack = append(r.stack, model)
}

// Pop removes the top view from the stack, unless it's the only view left.
func (r *Router[T]) Pop() {
	if len(r.stack) > 1 {
		r.stack = r.stack[:len(r.stack)-1]
	}
}

func (r *Router[T]) Replace(newModel T) {
	r.stack[len(r.stack)-1] = newModel
}

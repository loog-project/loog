package core

import "sync"

type State[T any] struct {
	value T
	mu    sync.RWMutex
}

func NewState[T any](value T) *State[T] {
	return &State[T]{value: value}
}

func (s *State[T]) Get() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

func (s *State[T]) Set(value T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value = value
}

package ui

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// AppState represents the global application state
type AppState struct {
}

// Action represents a state change
type Action interface {
	Type() string
}

// StateReducer processes actions and returns new state
type StateReducer func(state AppState, action Action) AppState

// StateManager manages global application state
type StateManager struct {
	mu       sync.RWMutex
	state    AppState
	eventBus EventBus
	reducer  StateReducer
}

// NewStateManager creates a new state manager
func NewStateManager(eventBus EventBus) *StateManager {
	return &StateManager{
		eventBus: eventBus,
		state:    AppState{},
		reducer:  defaultReducer,
	}
}

// State returns a copy of the current state
func (sm *StateManager) State() AppState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// Dispatch processes an action and updates state
func (sm *StateManager) Dispatch(action Action) tea.Cmd {
	sm.mu.Lock()
	oldState := sm.state
	sm.state = sm.reducer(sm.state, action)
	changed := !statesEqual(oldState, sm.state)
	sm.mu.Unlock()

	if changed {
		// Publish state change event
		event := &StateChangedEvent{
			BaseEvent: BaseEvent{
				eventType: EventTypeStateChanged,
				timestamp: time.Now(),
			},
			OldState: oldState,
			NewState: sm.state,
			Action:   action,
		}
		return sm.eventBus.Publish(event)
	}

	return nil
}

// Subscribe to state changes
func (sm *StateManager) Subscribe(handler func(state AppState, action Action) tea.Cmd) Subscription {
	return sm.eventBus.Subscribe(EventTypeStateChanged, func(event Event) tea.Cmd {
		if sce, ok := event.(*StateChangedEvent); ok {
			return handler(sce.NewState, sce.Action)
		}
		return nil
	})
}

type StateConnector struct {
	manager      *StateManager
	eventBus     EventBus
	subscription Subscription
}

// Connect creates a state connector for a component
func Connect(component Component, manager *StateManager, eventBus EventBus) *StateConnector {
	return &StateConnector{
		manager:  manager,
		eventBus: eventBus,
		subscription: manager.Subscribe(func(state AppState, action Action) tea.Cmd {
			if sa, ok := component.(StateAware); ok {
				return sa.OnStateChange(state, action)
			}
			return nil
		}),
	}
}

// Disconnect removes the state connector's subscription
func (c *StateConnector) Disconnect() {
	c.eventBus.Unsubscribe(c.subscription)
}

// Dispatch is an alias for StateManager's Dispatch method
func (c *StateConnector) Dispatch(action Action) tea.Cmd {
	return c.manager.Dispatch(action)
}

// State is an alias for StateManager's State method
func (c *StateConnector) State() AppState {
	return c.manager.State()
}

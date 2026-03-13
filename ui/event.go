package ui

import (
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/store"
)

// EventType represents different types of events in the system
type EventType string

const (
	EventTypeRevisionSelected EventType = "revision.selected"
	EventTypeRevisionAdded    EventType = "revision.added"
	EventTypeFocusChanged     EventType = "focus.changed"
	EventTypeStateChanged     EventType = "state.changed"
)

type Event interface {
	Type() EventType
	Timestamp() time.Time
}

type BaseEvent struct {
	eventType EventType
	timestamp time.Time
}

func (e BaseEvent) Type() EventType {
	return e.eventType
}

func (e BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// =====================================================================================================================

type RevisionSelectedEvent struct {
	BaseEvent
	ObjectID   string
	RevisionID store.RevisionID
}

type RevisionAddedEvent struct {
	BaseEvent
	Revision any // TODO: RevisionData
}

type FocusChangedEvent struct {
	BaseEvent
	ComponentID any // TODO: ComponentID
	Focused     bool
}

type StateChangedEvent struct {
	BaseEvent
	OldState AppState
	NewState AppState
	Action   Action
}

// =====================================================================================================================

// EventHandler processes events and returns a command to execute
type EventHandler func(event Event) tea.Cmd

// Subscription represents an active event subscription
type Subscription struct {
	id        string
	eventType EventType
	handler   EventHandler
}

// EventBus manages event distribution
type EventBus interface {
	Subscribe(eventType EventType, handler EventHandler) Subscription
	Unsubscribe(sub Subscription) (unsubscribed bool)
	Publish(event Event) tea.Cmd
}

// DefaultEventBus is the standard implementation.
type DefaultEventBus struct {
	mu            sync.RWMutex
	subscriptions map[EventType][]Subscription
	nextID        int
}

func NewEventBus() EventBus {
	return &DefaultEventBus{
		subscriptions: make(map[EventType][]Subscription),
	}
}

// Subscribe registers a new event handler for a specific event type.
//
// Note that the order of event handling is not guaranteed.
// If you subscribe multiple handlers for the same event type,
// they may be executed in any order when the event is published.
func (bus *DefaultEventBus) Subscribe(eventType EventType, handler EventHandler) Subscription {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.nextID++
	sub := Subscription{
		id:        fmt.Sprintf("sub-%d", bus.nextID),
		eventType: eventType,
		handler:   handler,
	}

	bus.subscriptions[eventType] = append(bus.subscriptions[eventType], sub)
	return sub
}

// Unsubscribe removes an event handler subscription.
// It returns true if the subscription was found and removed, false otherwise.
func (bus *DefaultEventBus) Unsubscribe(sub Subscription) bool {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	subs := bus.subscriptions[sub.eventType]
	for i, s := range subs {
		if s.id == sub.id {
			// remove subscription
			bus.subscriptions[sub.eventType] = append(subs[:i], subs[i+1:]...)
			return true
		}
	}

	// subscription not found
	return false
}

// Publish sends an event to all registered handlers for its type.
func (bus *DefaultEventBus) Publish(event Event) tea.Cmd {
	bus.mu.RLock()
	subs := bus.subscriptions[event.Type()]

	// copy subscriptions to avoid holding lock during handler execution
	handlers := make([]EventHandler, len(subs))
	for i, sub := range subs {
		handlers[i] = sub.handler
	}
	bus.mu.RUnlock()

	// Execute handlers and collect commands
	var commands []tea.Cmd
	for _, handler := range handlers {
		if cmd := handler(event); cmd != nil {
			commands = append(commands, cmd)
		}
	}

	return tea.Batch(commands...)
}

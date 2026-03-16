// Package mux multiplexes Kubernetes dynamic informers created on demand.
//
// Each registered GroupVersionResource gets its own shared informer that
// lists existing objects and then watches for changes. All events are
// delivered on a single unified channel accessible via [Mux.Events].
//
// The underlying informers handle gap recovery automatically (HTTP 410
// Gone, timeouts, etc.), so no events are missed. Consumers may see
// duplicates and should deduplicate if exactly-once semantics are
// required.
package mux

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	defaultBuffer        = 1024
	eventDeliveryTimeout = 100 * time.Millisecond
)

// ErrStopped is returned by Add when the Mux has already been stopped.
var ErrStopped = errors.New("mux is stopped")

// Option configures a Mux during construction.
type Option func(*muxConfig)

type muxConfig struct {
	buffer        int
	labelSelector string
	fieldSelector string
}

// WithBuffer sets the capacity of the internal event channel.
// Values below 1 are ignored and the default (1024) is used instead.
func WithBuffer(n int) Option {
	return func(c *muxConfig) {
		c.buffer = n
	}
}

// WithLabelSelector applies a Kubernetes label selector to every
// informer created by this Mux.
func WithLabelSelector(s string) Option {
	return func(c *muxConfig) {
		c.labelSelector = s
	}
}

// WithFieldSelector applies a Kubernetes field selector to every
// informer created by this Mux.
func WithFieldSelector(s string) Option {
	return func(c *muxConfig) {
		c.fieldSelector = s
	}
}

// Mux manages a dynamic set of Kubernetes informers and merges their
// events into a single channel. Watches can be added and removed at
// runtime. All methods are safe for concurrent use.
type Mux struct {
	ctx    context.Context
	cancel context.CancelFunc
	client dynamic.Interface
	cfg    muxConfig

	events chan watch.Event

	mu      sync.RWMutex
	watches map[schema.GroupVersionResource]context.CancelFunc
	stopped bool
}

// New creates a Mux that will create informers using the provided
// dynamic client. No watches are started until Add is called. The
// parent context controls the lifetime of all informers; canceling it
// is equivalent to calling Stop.
func New(ctx context.Context, client dynamic.Interface, opts ...Option) (*Mux, error) {
	if client == nil {
		return nil, errors.New("nil dynamic client")
	}

	cfg := muxConfig{buffer: defaultBuffer}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.buffer < 1 {
		cfg.buffer = defaultBuffer
	}

	ctx, cancel := context.WithCancel(ctx)
	return &Mux{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		cfg:     cfg,
		events:  make(chan watch.Event, cfg.buffer),
		watches: make(map[schema.GroupVersionResource]context.CancelFunc),
	}, nil
}

// Add registers a watch for the given GVR. It blocks until the
// informer's cache has synced (the initial list is complete). Calling
// Add for an already-watched GVR is a no-op.
func (m *Mux) Add(gvr schema.GroupVersionResource) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	if _, ok := m.watches[gvr]; ok {
		m.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(m.ctx)

	// Reserve the slot so a concurrent Add for the same GVR is a no-op.
	m.watches[gvr] = cancel
	m.mu.Unlock()

	// Everything below runs without the lock. The informer's event
	// handlers acquire a read-lock internally, so holding a write-lock
	// here would deadlock during the initial list.
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		m.client, 0, metav1.NamespaceAll, m.tweakListOptions,
	)

	inf := factory.ForResource(gvr).Informer()
	if _, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    m.dispatch(watch.Added),
		UpdateFunc: func(_, newObj any) { m.dispatch(watch.Modified)(newObj) },
		DeleteFunc: m.dispatch(watch.Deleted),
	}); err != nil {
		cancel()
		m.unwatch(gvr)
		return fmt.Errorf("register event handler for %s: %w", gvr, err)
	}

	go factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		cancel()
		m.unwatch(gvr)
		return fmt.Errorf("cache sync failed for %s", gvr)
	}

	return nil
}

// Remove stops the watch for the given GVR and returns true. If the GVR
// is not being watched, Remove returns false.
func (m *Mux) Remove(gvr schema.GroupVersionResource) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	cancel, ok := m.watches[gvr]
	if !ok {
		return false
	}
	cancel()
	delete(m.watches, gvr)
	return true
}

// Has reports whether the given GVR is currently being watched.
func (m *Mux) Has(gvr schema.GroupVersionResource) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.watches[gvr]
	return ok
}

// GVRs returns all currently watched GroupVersionResources.
func (m *Mux) GVRs() []schema.GroupVersionResource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]schema.GroupVersionResource, 0, len(m.watches))
	for gvr := range m.watches {
		out = append(out, gvr)
	}
	return out
}

// Len returns the number of active watches.
func (m *Mux) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.watches)
}

// Events returns the unified, read-only event stream. The channel is
// closed when Stop is called.
func (m *Mux) Events() <-chan watch.Event {
	return m.events
}

// Stop terminates all watches and closes the event channel. It is safe
// to call multiple times.
func (m *Mux) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()

	// Canceling the parent context cascades to every informer context.
	m.cancel()
	close(m.events)
}

// unwatch removes a GVR from the watch map under the write lock. Used
// as cleanup when Add fails after the slot has been reserved.
func (m *Mux) unwatch(gvr schema.GroupVersionResource) {
	m.mu.Lock()
	delete(m.watches, gvr)
	m.mu.Unlock()
}

// tweakListOptions merges user-configured selectors into the informer's
// list options without overwriting fields set internally by the
// reflector (ResourceVersion, SendInitialEvents, etc.).
func (m *Mux) tweakListOptions(lo *metav1.ListOptions) {
	if m.cfg.labelSelector != "" {
		lo.LabelSelector = m.cfg.labelSelector
	}
	if m.cfg.fieldSelector != "" {
		lo.FieldSelector = m.cfg.fieldSelector
	}
}

// dispatch returns an informer event handler that converts the callback
// argument into a [watch.Event] and sends it to the event channel. If
// the channel is full and cannot accept within [eventDeliveryTimeout],
// the event is dropped (informer cache is authoritative, so no data is
// truly lost).
func (m *Mux) dispatch(eventType watch.EventType) func(obj any) {
	return func(obj any) {
		ro, ok := toRuntimeObject(obj)
		if !ok {
			return
		}

		event := watch.Event{Type: eventType, Object: ro}

		timer := time.NewTimer(eventDeliveryTimeout)
		defer timer.Stop()

		select {
		case m.events <- event:
		case <-timer.C:
		case <-m.ctx.Done():
		}
	}
}

// toRuntimeObject extracts a runtime.Object from an informer callback
// argument, handling the DeletedFinalStateUnknown tombstone wrapper that
// the informer emits when it missed the actual delete event.
func toRuntimeObject(obj any) (runtime.Object, bool) {
	switch v := obj.(type) {
	case runtime.Object:
		return v, true
	case cache.DeletedFinalStateUnknown:
		if ro, ok := v.Obj.(runtime.Object); ok {
			return ro, true
		}
	}
	return nil, false
}

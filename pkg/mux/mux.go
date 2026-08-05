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
	wg     sync.WaitGroup // tracks in-flight dispatch calls

	mu      sync.RWMutex
	watches map[schema.GroupVersionResource]*watchEntry
	stopped bool
}

// watchEntry holds the per-GVR cancel func and a channel that is closed once
// the initial Add for that GVR has finished (whether it synced or failed).
type watchEntry struct {
	cancel context.CancelFunc
	synced chan struct{}
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
		watches: make(map[schema.GroupVersionResource]*watchEntry),
	}, nil
}

// Add registers a watch for the given GVR. It blocks until the
// informer's cache has synced (the initial list is complete). Calling
// Add for an already-watched GVR (including a concurrent Add for the
// same GVR) blocks until that watch has synced, then returns.
func (m *Mux) Add(gvr schema.GroupVersionResource) error {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return ErrStopped
	}
	if existing, ok := m.watches[gvr]; ok {
		synced := existing.synced
		m.mu.Unlock()
		// Honour the "blocks until synced" contract for this caller too,
		// instead of returning before the first Add finished its initial list.
		select {
		case <-synced:
		case <-m.ctx.Done():
			return ErrStopped
		}
		if m.Has(gvr) {
			return nil
		}
		return fmt.Errorf("concurrent add for %s did not complete", gvr)
	}

	ctx, cancel := context.WithCancel(m.ctx)

	// Reserve the slot so a concurrent Add for the same GVR waits on synced.
	entry := &watchEntry{cancel: cancel, synced: make(chan struct{})}
	m.watches[gvr] = entry
	m.mu.Unlock()

	// Always unblock any waiters, whether this Add succeeds or fails. Waiters
	// re-check Has() after waking to distinguish success from failure.
	defer close(entry.synced)

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

	// Start is non-blocking (it launches the informer goroutines and
	// returns), so call it directly; wrapping it in `go` would let
	// WaitForCacheSync run before Start.
	factory.Start(ctx.Done())

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

	entry, ok := m.watches[gvr]
	if !ok {
		return false
	}
	entry.cancel()
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
	if m == nil {
		return
	}
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()

	// Canceling the parent context cascades to every informer context
	// and unblocks any dispatch calls waiting on a send.
	m.cancel()

	// Wait for all in-flight dispatch calls to return before closing
	// the channel so no goroutine can send on a closed channel.
	m.wg.Wait()
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
		// Acquire a read-lock to register with the WaitGroup. This
		// pairs with the write-lock in Stop: once Stop sets
		// m.stopped=true and releases the lock, no new dispatch can
		// call wg.Add, so the subsequent wg.Wait is safe.
		m.mu.RLock()
		if m.stopped {
			m.mu.RUnlock()
			return
		}
		m.wg.Add(1)
		m.mu.RUnlock()
		defer m.wg.Done()

		ro, ok := toRuntimeObject(obj)
		if !ok {
			return
		}

		event := watch.Event{Type: eventType, Object: ro}

		// Fast path: non-blocking send avoids allocating a timer when
		// the channel has capacity.
		select {
		case m.events <- event:
			return
		default:
		}

		// Channel full – apply backpressure with timeout.
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

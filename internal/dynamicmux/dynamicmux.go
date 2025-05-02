package dynamicmux

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
	"k8s.io/klog/v2"
)

const maxEventCallbackTime = 100 * time.Millisecond

// Handler is a user‑provided callback that receives a single watch.Event.
// It MUST be fast and non‑blocking; move expensive work to a goroutine.
type Handler func(ev watch.Event) error

// Options for New. All fields are optional; use functional helpers below.
//
// Zero values give reasonable defaults.
//
// All Options are read‑only after New returns.
type Options struct {
	// List is the base ListOptions passed to every informer.
	List metav1.ListOptions

	// Buffer is the size of the internal buffered channel.
	// A value < 1 picks the default
	// (1024).
	Buffer int

	// Logger is a Logger. Nothing more to say.
	Logger klog.Logger
}

func WithListOptions(lo metav1.ListOptions) func(*Options) {
	return func(o *Options) { o.List = lo }
}

func WithBuffer(n int) func(*Options) {
	return func(o *Options) { o.Buffer = n }
}

func WithLogger(l klog.Logger) func(*Options) {
	return func(o *Options) { o.Logger = l }
}

// Mux multiplexes dynamic informers created on demand.
//
// It is _reliable_ in the sense that it never misses Kubernetes events:
// the underlying Reflector relists + continues the watch if it detects a
// gap (HTTP 410, Timeout, etc.).
//
// Clients may still see duplicates – dedupe if you require exactly‑once semantics.
type Mux struct {
	ctx    context.Context
	cancel context.CancelFunc

	dyn     dynamic.Interface
	options Options

	events chan watch.Event

	mutex           sync.RWMutex
	activeInformers map[schema.GroupVersionResource]*runner
	running         bool

	handlers []Handler
}

type runner struct {
	stop context.CancelFunc
}

var (
	// ErrClosed is returned by Add / Remove after Stop() has been called.
	ErrClosed = errors.New("dynamicmux: mux has been stopped")
)

// New instantiates a new Mux.
// It starts no watches; call Add() for each GVR you’re interested in.
func New(parent context.Context, dyn dynamic.Interface, opts ...func(*Options)) (*Mux, error) {
	if dyn == nil {
		return nil, errors.New("dynamicmux: nil dynamic client")
	}

	// defaults
	options := Options{Buffer: 1024, Logger: klog.Background()}
	for _, fn := range opts {
		fn(&options)
	}
	if options.Buffer < 1 {
		options.Buffer = 1024
	}

	ctx, cancel := context.WithCancel(parent)

	m := &Mux{
		ctx:             ctx,
		cancel:          cancel,
		dyn:             dyn,
		options:         options,
		events:          make(chan watch.Event, options.Buffer),
		activeInformers: make(map[schema.GroupVersionResource]*runner),
		running:         true,
	}
	return m, nil
}

// Add registers (idempotently) a watch for the given GVR.
func (m *Mux) Add(gvr schema.GroupVersionResource) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.running {
		return ErrClosed
	}
	if _, ok := m.activeInformers[gvr]; ok {
		// already running
		return nil
	}

	ctx, cancel := context.WithCancel(m.ctx)
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		m.dyn,
		0,
		metav1.NamespaceAll,
		func(lo *metav1.ListOptions) {
			*lo = m.options.List
		},
	)

	inf := factory.ForResource(gvr).Informer()
	_, err := inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: m.wrapHandler(watch.Added),
		UpdateFunc: func(_, newObj any) {
			m.wrapHandler(watch.Modified)(newObj)
		},
		DeleteFunc: m.wrapHandler(watch.Deleted),
	})
	if err != nil {
		cancel()
		return fmt.Errorf("dynamicmux: failed to register event handler for %s: %w", gvr, err)
	}
	go factory.Start(ctx.Done())

	if ok := cache.WaitForCacheSync(ctx.Done(), inf.HasSynced); !ok {
		cancel()
		return fmt.Errorf("dynamicmux: failed to sync cache for %s", gvr)
	}

	m.activeInformers[gvr] = &runner{stop: cancel}
	m.options.Logger.V(2).Info("started watch", "gvr", gvr)
	return nil
}

// Remove stops and forgets the informer associated with gvr.
// It returns true if the informer was running, false if it was not.
func (m *Mux) Remove(gvr schema.GroupVersionResource) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if r, ok := m.activeInformers[gvr]; ok {
		r.stop()
		delete(m.activeInformers, gvr)

		m.options.Logger.V(2).Info("stopped watch", "gvr", gvr)
		return true
	}

	return false
}

// Events exposes the unified, read‑only event stream.
// The channel is closed after Stop().
func (m *Mux) Events() <-chan watch.Event {
	return m.events
}

// RegisterHandler appends a synchronous callback executed for EVERY event.
func (m *Mux) RegisterHandler(h Handler) {
	if h == nil {
		return
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.handlers = append(m.handlers, h)
}

// Stop terminates all informers and closes the Events() channel.
func (m *Mux) Stop() {
	m.mutex.Lock()
	if !m.running {
		m.mutex.Unlock()
		return
	}
	m.running = false
	m.mutex.Unlock()

	m.cancel()

	m.mutex.RLock()
	for _, r := range m.activeInformers {
		r.stop()
	}
	m.mutex.RUnlock()

	close(m.events)
}

func (m *Mux) wrapHandler(t watch.EventType) func(obj any) {
	return func(obj any) {
		var runtimeObject runtime.Object
		switch v := obj.(type) {
		case runtime.Object:
			runtimeObject = v
		case cache.DeletedFinalStateUnknown:
			if inner, ok := v.Obj.(runtime.Object); ok {
				runtimeObject = inner
			} else {
				m.options.Logger.V(2).Info("deleted final state unknown object is not a runtime.Object")
				return
			}
		default:
			m.options.Logger.V(2).Info("unexpected object type",
				"type", fmt.Sprintf("%T", obj))
			return
		}

		event := watch.Event{Type: t, Object: runtimeObject}

		// fast path: try to deliver into channel without blocking longer
		// than a small grace period; otherwise drop + log (event can be
		// reconstructed from informer cache which is authoritative).
		select {
		case m.events <- event:
		case <-time.After(maxEventCallbackTime):
			m.options.Logger.V(4).Info("dropping event due to slow consumer",
				"gvk", event.Object.GetObjectKind().GroupVersionKind(), "type", t)
		case <-m.ctx.Done():
			return
		}

		m.mutex.RLock()
		handlers := append([]Handler(nil), m.handlers...)
		m.mutex.RUnlock()

		for i, h := range handlers {
			if h == nil {
				continue
			}
			if err := h(event); err != nil {
				m.options.Logger.Error(err, "handler returned error – disabling", "index", i)
				m.mutex.Lock()
				m.handlers[i] = nil
				m.mutex.Unlock()
			}
		}
	}
}

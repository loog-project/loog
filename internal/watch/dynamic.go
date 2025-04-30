package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

var ErrAlreadyTerminated = fmt.Errorf("dynamic mux terminated")

// DynamicMux multiplexes a set of dynamic watches that can be added or
// removed at runtime.
type DynamicMux struct {
	ctx       context.Context
	dyn       dynamic.Interface
	options   metav1.ListOptions
	eventChan chan watch.Event

	mutex      sync.RWMutex
	watchers   map[schema.GroupVersionResource]watch.Interface
	terminated bool
}

// New creates an empty DynamicMux and starts no watches yet.
func New(
	ctx context.Context,
	dyn dynamic.Interface,
	options metav1.ListOptions,
) *DynamicMux {
	return &DynamicMux{
		ctx:       ctx,
		dyn:       dyn,
		options:   options,
		eventChan: make(chan watch.Event, 1_024),
		watchers:  make(map[schema.GroupVersionResource]watch.Interface),
	}
}

// ResultChan returns the multiplexed event channel.
func (m *DynamicMux) ResultChan() <-chan watch.Event {
	return m.eventChan
}

// Stop terminates all child watches and closes the event channel.
func (m *DynamicMux) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.terminated {
		return
	}
	for _, w := range m.watchers {
		w.Stop()
	}
	m.terminated = true
	close(m.eventChan)
}

// Add starts watching the provided GVR if it is not already watched.
func (m *DynamicMux) Add(gvr schema.GroupVersionResource) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.terminated {
		return ErrAlreadyTerminated
	}
	if _, exists := m.watchers[gvr]; exists {
		return nil // already watching
	}

	watcher, err := m.dyn.Resource(gvr).Watch(m.ctx, m.options)
	if err != nil {
		return fmt.Errorf("watch %s: %w", gvr, err)
	}
	m.watchers[gvr] = watcher

	start := time.Now()
	go func(w watch.Interface) {
		for {
			select {
			case <-m.ctx.Done():
				w.Stop()
				return
			case ev, ok := <-w.ResultChan():
				if !ok {
					panic("watcher closed unexpectedly for " + gvr.String() + " after " + time.Since(start).String())
					return
				}
				m.eventChan <- ev
			}
		}
	}(watcher)

	return nil
}

// Remove stops and forgets the watch for the given GVR.
func (m *DynamicMux) Remove(gvr schema.GroupVersionResource) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if watcher, exists := m.watchers[gvr]; exists {
		watcher.Stop()
		delete(m.watchers, gvr)
	}
}

// GetWatchedGVRs returns the current watchers.
func (m *DynamicMux) GetWatchedGVRs() []schema.GroupVersionResource {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	gvrs := make([]schema.GroupVersionResource, 0, len(m.watchers))
	for gvr := range m.watchers {
		gvrs = append(gvrs, gvr)
	}

	return gvrs
}

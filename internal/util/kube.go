package util

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func ParseGroupVersionResource(gv string) (schema.GroupVersionResource, error) {
	parts := strings.Split(gv, "/")
	if len(parts) == 2 {
		// assume it's a resource without a group (e.g. "v1/pods")
		return schema.GroupVersionResource{
			Version:  parts[0],
			Resource: parts[1],
		}, nil
	}
	if len(parts) == 3 {
		// assume it's a resource with a group (e.g. "apps/v1/deployments")
		return schema.GroupVersionResource{
			Group:    parts[0],
			Version:  parts[1],
			Resource: parts[2],
		}, nil
	}
	// no idea...
	return schema.GroupVersionResource{}, fmt.Errorf("invalid group/version/resource format: %s", gv)
}

type MultiWatcher struct {
	Watchers   []watch.Interface
	resultChan <-chan watch.Event
}

func (m MultiWatcher) Stop() {
	for _, watcher := range m.Watchers {
		watcher.Stop()
	}
}

func (m MultiWatcher) ResultChan() <-chan watch.Event {
	return m.resultChan
}

func NewMultiWatcher(ctx context.Context, dyn *dynamic.DynamicClient, gvks []schema.GroupVersionResource, options v1.ListOptions) (*MultiWatcher, error) {
	watchers := make([]watch.Interface, len(gvks))
	for i, gvk := range gvks {
		watcher, err := dyn.Resource(gvk).Watch(ctx, options)
		if err != nil {
			return nil, fmt.Errorf("failed to watch %s: %w", gvk, err)
		}
		watchers[i] = watcher
	}
	resultChan := make(chan watch.Event)
	for _, watcher := range watchers {
		go func(w watch.Interface) {
			for {
				select {
				case <-ctx.Done():
					w.Stop()
					return
				case event := <-w.ResultChan():
					resultChan <- event
				}
			}
		}(watcher)
	}
	return &MultiWatcher{
		Watchers:   watchers,
		resultChan: resultChan,
	}, nil
}

var _ watch.Interface = (*MultiWatcher)(nil)

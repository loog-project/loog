package util

import (
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type EventEntry struct {
	EventType  watch.EventType
	ReceivedAt time.Time

	Name               types.NamespacedName
	ResourceGeneration int64
	ResourceVersion    string

	Object *unstructured.Unstructured
}

type EventEntryEnv struct {
	Event  watch.Event
	Object *unstructured.Unstructured
}

func (e EventEntryEnv) All() bool {
	return true
}

func (e EventEntryEnv) None() bool {
	return false
}

func (e EventEntryEnv) Namespaces(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, val := range vals {
		if val == e.Object.GetNamespace() {
			return true
		}
	}
	return false
}

func (e EventEntryEnv) Namespace(vals ...string) bool {
	return e.Namespaces(vals...)
}

func (e EventEntryEnv) Names(vals ...string) bool {
	if len(vals) == 0 {
		return true
	}
	for _, val := range vals {
		if val == e.Object.GetName() {
			return true
		}
	}
	return false
}

func (e EventEntryEnv) Name(vals ...string) bool {
	return e.Names(vals...)
}

func (e EventEntryEnv) Namespaced(namespace, name string) bool {
	return e.Object.GetNamespace() == namespace && e.Object.GetName() == name
}

func (e EventEntryEnv) LabelExists(labelKeys ...string) bool {
	if len(labelKeys) == 0 {
		return true
	}
	labels := e.Object.GetLabels()
	if labels == nil {
		return false
	}
	for _, key := range labelKeys {
		if _, exists := labels[key]; !exists {
			return false
		}
	}
	return true
}

func (e EventEntryEnv) Label(key, value string) bool {
	if e.Object.GetLabels() == nil {
		return false
	}
	val, exists := e.Object.GetLabels()[key]
	if !exists {
		return false
	}
	return val == value
}

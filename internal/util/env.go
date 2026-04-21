package util

import (
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// EventEntryEnv is the expression environment exposed to the user-supplied
// filter expression (--filter flag). Methods are callable from within
// expr-lang expressions such as `Namespaces("default", "kube-system")`.
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
	if e.Object == nil || len(vals) == 0 {
		return true
	}
	return slices.Contains(vals, e.Object.GetNamespace())
}

func (e EventEntryEnv) Namespace(vals ...string) bool {
	return e.Namespaces(vals...)
}

func (e EventEntryEnv) Names(vals ...string) bool {
	if e.Object == nil || len(vals) == 0 {
		return true
	}
	return slices.Contains(vals, e.Object.GetName())
}

func (e EventEntryEnv) Name(vals ...string) bool {
	return e.Names(vals...)
}

func (e EventEntryEnv) Namespaced(namespace, name string) bool {
	if e.Object == nil {
		return false
	}
	return e.Object.GetNamespace() == namespace && e.Object.GetName() == name
}

func (e EventEntryEnv) LabelExists(labelKeys ...string) bool {
	if e.Object == nil || len(labelKeys) == 0 {
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
	if e.Object == nil {
		return false
	}
	labels := e.Object.GetLabels()
	if labels == nil {
		return false
	}
	val, exists := labels[key]
	return exists && val == value
}

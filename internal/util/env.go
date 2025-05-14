package util

import (
	"encoding/json"
	"strings"
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

func (e EventEntryEnv) ToLower(s string) string {
	return strings.ToLower(s)
}

func (e EventEntryEnv) ToUpper(s string) string {
	return strings.ToUpper(s)
}

func (e EventEntryEnv) StartsWith(haystack, needle string) bool {
	return strings.HasPrefix(haystack, needle)
}

func (e EventEntryEnv) StartsWithFold(haystack, needle string) bool {
	return strings.HasPrefix(strings.ToLower(haystack), strings.ToLower(needle))
}

func (e EventEntryEnv) EndsWith(haystack, needle string) bool {
	return strings.HasSuffix(haystack, needle)
}

func (e EventEntryEnv) EndsWithFold(haystack, needle string) bool {
	return strings.HasSuffix(strings.ToLower(haystack), strings.ToLower(needle))
}

func (e EventEntryEnv) Contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func (e EventEntryEnv) ContainsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func (e EventEntryEnv) ForKind(kind string, v bool) bool {
	if e.Object.GetKind() == kind {
		return v
	}
	return true
}

func (e EventEntryEnv) SpecContains(val string) bool {
	specAny, ok := e.Object.Object["spec"]
	if !ok {
		return false
	}
	specStr, err := json.Marshal(specAny)
	if err != nil {
		panic("oh oh!")
	}
	return strings.Contains(strings.ToLower(string(specStr)), strings.ToLower(val))
}

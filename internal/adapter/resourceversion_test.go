package adapter

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/loog-project/loog/internal/store"
)

// buildRevision must capture metadata.resourceVersion as a parsed uint64 so the
// timeline can order near-simultaneous events causally.
func TestBuildRevision_CapturesResourceVersion(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"kind": "ConfigMap",
		"metadata": map[string]any{
			"uid":             "u1",
			"name":            "cm",
			"resourceVersion": "123456",
		},
	}}
	rev := buildRevision(obj, 1, &store.Snapshot{PreviousID: 0}, nil)
	if rev.ResourceVersion != 123456 {
		t.Errorf("ResourceVersion = %d, want 123456", rev.ResourceVersion)
	}
}

func TestBuildRevision_MissingResourceVersionIsZero(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"kind":     "ConfigMap",
		"metadata": map[string]any{"uid": "u1", "name": "cm"},
	}}
	rev := buildRevision(obj, 1, &store.Snapshot{PreviousID: 0}, nil)
	if rev.ResourceVersion != 0 {
		t.Errorf("ResourceVersion = %d, want 0 when absent", rev.ResourceVersion)
	}
}

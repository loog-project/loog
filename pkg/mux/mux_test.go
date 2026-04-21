package mux

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
)

var (
	podGVR        = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	deploymentGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
)

func testPod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func testDeployment(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

// gvrListKinds maps every GVR used in the test suite to its
// corresponding list kind. Required by the fake dynamic client.
var gvrListKinds = map[schema.GroupVersionResource]string{
	podGVR:        "PodList",
	deploymentGVR: "DeploymentList",
}

// newTestMux creates a Mux backed by a fake dynamic client pre-loaded
// with the given objects. The Mux and its context are torn down when the
// test completes.
func newTestMux(t *testing.T, objects ...runtime.Object) (*Mux, *fakedynamic.FakeDynamicClient) {
	t.Helper()

	client := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), gvrListKinds, objects...,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	m, err := New(ctx, client)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(m.Stop)
	return m, client
}

// drainEvents reads exactly n events from ch (or fails after timeout).
func drainEvents(t *testing.T, ch <-chan watch.Event, n int, timeout time.Duration) []watch.Event {
	t.Helper()

	var events []watch.Event
	deadline := time.After(timeout)
	for range n {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed after %d/%d events", len(events), n)
			}
			events = append(events, ev)
		case <-deadline:
			t.Fatalf("timed out after receiving %d/%d events", len(events), n)
		}
	}
	return events
}

// eventNames extracts sorted object names from a slice of events.
func eventNames(events []watch.Event) []string {
	names := make([]string, len(events))
	for i, ev := range events {
		u, ok := ev.Object.(*unstructured.Unstructured)
		if ok {
			names[i] = u.GetName()
		}
	}
	sort.Strings(names)
	return names
}

func TestNew(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	m, err := New(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.Stop()

	if m.Events() == nil {
		t.Fatal("Events() returned nil")
	}
	if m.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", m.Len())
	}
}

func TestNew_NilClient(t *testing.T) {
	_, err := New(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestNew_WithBuffer(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	m, err := New(context.Background(), client, WithBuffer(42))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.Stop()

	if got := cap(m.events); got != 42 {
		t.Fatalf("buffer capacity: got %d, want 42", got)
	}
}

func TestNew_ZeroBuffer_UsesDefault(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	m, err := New(context.Background(), client, WithBuffer(0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.Stop()

	if got := cap(m.events); got != defaultBuffer {
		t.Fatalf("buffer capacity: got %d, want %d", got, defaultBuffer)
	}
}

func TestNew_NegativeBuffer_UsesDefault(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	m, err := New(context.Background(), client, WithBuffer(-5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.Stop()

	if got := cap(m.events); got != defaultBuffer {
		t.Fatalf("buffer capacity: got %d, want %d", got, defaultBuffer)
	}
}

func TestNew_MultipleOptions(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	m, err := New(context.Background(), client,
		WithBuffer(64),
		WithLabelSelector("app=web"),
		WithFieldSelector("metadata.name=test"),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer m.Stop()

	if m.cfg.buffer != 64 {
		t.Fatalf("buffer: got %d, want 64", m.cfg.buffer)
	}
	if m.cfg.labelSelector != "app=web" {
		t.Fatalf("labelSelector: got %q, want %q", m.cfg.labelSelector, "app=web")
	}
	if m.cfg.fieldSelector != "metadata.name=test" {
		t.Fatalf("fieldSelector: got %q, want %q", m.cfg.fieldSelector, "metadata.name=test")
	}
}

func TestAdd(t *testing.T) {
	m, _ := newTestMux(t)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !m.Has(podGVR) {
		t.Fatal("Has(podGVR) = false after Add")
	}
	if m.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", m.Len())
	}
}

func TestAdd_DeliversInitialListEvents(t *testing.T) {
	pods := []runtime.Object{
		testPod("alpha", "default"),
		testPod("bravo", "default"),
		testPod("charlie", "kube-system"),
	}
	m, _ := newTestMux(t, pods...)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	events := drainEvents(t, m.Events(), 3, 5*time.Second)
	for _, ev := range events {
		if ev.Type != watch.Added {
			t.Errorf("event type: got %s, want %s", ev.Type, watch.Added)
		}
	}

	names := eventNames(events)
	want := []string{"alpha", "bravo", "charlie"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("names: got %v, want %v", names, want)
	}
}

func TestAdd_Idempotent(t *testing.T) {
	m, _ := newTestMux(t, testPod("p1", "default"))

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("first Add: %v", err)
	}
	// Drain the initial event so we can detect duplicates later.
	drainEvents(t, m.Events(), 1, 5*time.Second)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("second Add: %v", err)
	}
	if m.Len() != 1 {
		t.Fatalf("Len: got %d, want 1", m.Len())
	}

	// A second Add must not deliver duplicate events.
	select {
	case ev := <-m.Events():
		t.Fatalf("unexpected event after idempotent Add: %+v", ev)
	case <-time.After(200 * time.Millisecond):
		// good
	}
}

func TestAdd_AfterStop(t *testing.T) {
	m, _ := newTestMux(t)
	m.Stop()

	err := m.Add(podGVR)
	if !errors.Is(err, ErrStopped) {
		t.Fatalf("Add after Stop: got %v, want ErrStopped", err)
	}
}

func TestAdd_MultipleGVRs(t *testing.T) {
	m, _ := newTestMux(t,
		testPod("pod-1", "default"),
		testDeployment("deploy-1", "default"),
	)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add pods: %v", err)
	}
	if err := m.Add(deploymentGVR); err != nil {
		t.Fatalf("Add deployments: %v", err)
	}

	if m.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", m.Len())
	}
	if !m.Has(podGVR) || !m.Has(deploymentGVR) {
		t.Fatal("Has returned false for a watched GVR")
	}

	// Should receive events from both GVRs.
	events := drainEvents(t, m.Events(), 2, 5*time.Second)
	names := eventNames(events)
	if !containsAll(names, "deploy-1", "pod-1") {
		t.Fatalf("names: got %v, want [deploy-1 pod-1]", names)
	}
}

func TestAdd_CanceledContext(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately canceled

	m, err := New(ctx, client)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Stop()

	err = m.Add(podGVR)
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

func TestRemove(t *testing.T) {
	m, _ := newTestMux(t)
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !m.Remove(podGVR) {
		t.Fatal("Remove: got false, want true")
	}
	if m.Has(podGVR) {
		t.Fatal("Has(podGVR) = true after Remove")
	}
	if m.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", m.Len())
	}
}

func TestRemove_Nonexistent(t *testing.T) {
	m, _ := newTestMux(t)

	if m.Remove(podGVR) {
		t.Fatal("Remove nonexistent: got true, want false")
	}
}

func TestRemove_Idempotent(t *testing.T) {
	m, _ := newTestMux(t)
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !m.Remove(podGVR) {
		t.Fatal("first Remove: got false, want true")
	}
	if m.Remove(podGVR) {
		t.Fatal("second Remove: got true, want false")
	}
}

func TestRemove_ThenReAdd(t *testing.T) {
	m, _ := newTestMux(t, testPod("p1", "default"))

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("initial Add: %v", err)
	}
	drainEvents(t, m.Events(), 1, 5*time.Second)

	m.Remove(podGVR)

	// Re-adding the same GVR should work and deliver events again.
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("re-Add: %v", err)
	}

	events := drainEvents(t, m.Events(), 1, 5*time.Second)
	if events[0].Type != watch.Added {
		t.Fatalf("event type after re-add: got %s, want %s", events[0].Type, watch.Added)
	}
}

func TestHas(t *testing.T) {
	m, _ := newTestMux(t)

	if m.Has(podGVR) {
		t.Fatal("Has before Add should be false")
	}

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !m.Has(podGVR) {
		t.Fatal("Has after Add should be true")
	}
	if m.Has(deploymentGVR) {
		t.Fatal("Has for unwatched GVR should be false")
	}
}

func TestGVRs_Empty(t *testing.T) {
	m, _ := newTestMux(t)

	gvrs := m.GVRs()
	if len(gvrs) != 0 {
		t.Fatalf("GVRs: got %v, want empty", gvrs)
	}
}

func TestGVRs(t *testing.T) {
	m, _ := newTestMux(t)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add pods: %v", err)
	}
	if err := m.Add(deploymentGVR); err != nil {
		t.Fatalf("Add deployments: %v", err)
	}

	gvrs := m.GVRs()
	if len(gvrs) != 2 {
		t.Fatalf("GVRs: got %d entries, want 2", len(gvrs))
	}
	if !containsGVR(gvrs, podGVR) || !containsGVR(gvrs, deploymentGVR) {
		t.Fatalf("GVRs: got %v, want both pods and deployments", gvrs)
	}
}

func TestLen_Lifecycle(t *testing.T) {
	m, _ := newTestMux(t)

	if m.Len() != 0 {
		t.Fatalf("initial Len: got %d, want 0", m.Len())
	}

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add pods: %v", err)
	}
	if m.Len() != 1 {
		t.Fatalf("after Add pods: got %d, want 1", m.Len())
	}

	if err := m.Add(deploymentGVR); err != nil {
		t.Fatalf("Add deployments: %v", err)
	}
	if m.Len() != 2 {
		t.Fatalf("after Add deployments: got %d, want 2", m.Len())
	}

	m.Remove(podGVR)
	if m.Len() != 1 {
		t.Fatalf("after Remove pods: got %d, want 1", m.Len())
	}
}

func TestEvents_DynamicUpdate(t *testing.T) {
	pod := testPod("my-pod", "default")
	m, client := newTestMux(t, pod)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Consume the initial Add event.
	drainEvents(t, m.Events(), 1, 5*time.Second)

	// Update the pod via the fake client.
	updated := testPod("my-pod", "default")
	updated.Object["spec"] = map[string]any{"nodeName": "node-1"}
	if _, err := client.Resource(podGVR).Namespace("default").Update(
		context.Background(), updated, metav1.UpdateOptions{},
	); err != nil {
		t.Fatalf("Update: %v", err)
	}

	events := drainEvents(t, m.Events(), 1, 5*time.Second)
	if events[0].Type != watch.Modified {
		t.Fatalf("event type: got %s, want %s", events[0].Type, watch.Modified)
	}
}

func TestEvents_DynamicDelete(t *testing.T) {
	pod := testPod("doomed", "default")
	m, client := newTestMux(t, pod)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}
	drainEvents(t, m.Events(), 1, 5*time.Second)

	if err := client.Resource(podGVR).Namespace("default").Delete(
		context.Background(), "doomed", metav1.DeleteOptions{},
	); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	events := drainEvents(t, m.Events(), 1, 5*time.Second)
	if events[0].Type != watch.Deleted {
		t.Fatalf("event type: got %s, want %s", events[0].Type, watch.Deleted)
	}
}

func TestEvents_MultipleGVRsInterleaved(t *testing.T) {
	m, _ := newTestMux(t,
		testPod("p1", "default"),
		testPod("p2", "default"),
		testDeployment("d1", "default"),
	)

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add pods: %v", err)
	}
	if err := m.Add(deploymentGVR); err != nil {
		t.Fatalf("Add deployments: %v", err)
	}

	events := drainEvents(t, m.Events(), 3, 5*time.Second)
	names := eventNames(events)
	if !containsAll(names, "d1", "p1", "p2") {
		t.Fatalf("names: got %v, want [d1 p1 p2]", names)
	}
}

func TestEvents_ChannelClosedOnStop(t *testing.T) {
	m, _ := newTestMux(t)
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m.Stop()

	// Reading from a closed channel should immediately return the zero
	// value with ok=false.
	_, ok := <-m.Events()
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestStop_Idempotent(t *testing.T) {
	m, _ := newTestMux(t)

	// Calling Stop multiple times must not panic.
	m.Stop()
	m.Stop()
	m.Stop()
}

func TestStop_AddAfterStop(t *testing.T) {
	m, _ := newTestMux(t)
	m.Stop()

	if err := m.Add(podGVR); !errors.Is(err, ErrStopped) {
		t.Fatalf("got %v, want ErrStopped", err)
	}
}

func TestStop_RemoveAfterStop(t *testing.T) {
	m, _ := newTestMux(t)
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	m.Stop()

	// Remove after Stop should be safe (returns false since stopped
	// mux clears nothing extra). The watch map still has the entry but
	// the context is canceled. This is acceptable.
	_ = m.Remove(podGVR)
}

// TestStop_ConcurrentDispatchNoPanic verifies that calling Stop while
// dispatch goroutines are actively sending events does not cause a
// "send on closed channel" panic.
func TestStop_ConcurrentDispatchNoPanic(t *testing.T) {
	client := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), gvrListKinds,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a tiny buffer so the channel fills up quickly and dispatch
	// goroutines are more likely to be mid-send when Stop is called.
	m, err := New(ctx, client, WithBuffer(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Fire many events concurrently to maximize the chance of a
	// dispatch in flight when Stop is called.
	var wg sync.WaitGroup
	for i := range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pod := testPod(fmt.Sprintf("pod-%d", i), "default")
			_, _ = client.Resource(podGVR).Namespace("default").Create(
				context.Background(), pod, metav1.CreateOptions{},
			)
		}()
	}

	// Give some events time to start flowing, then stop.
	time.Sleep(10 * time.Millisecond)
	m.Stop()

	// Wait for all creators to finish (they may error after Stop,
	// that's fine).
	wg.Wait()

	// If we reach here without a panic, the race is fixed.
	// Verify the channel is closed.
	_, ok := <-m.Events()
	if ok {
		// Drain remaining buffered events.
		for range m.Events() {
		}
	}
}

func TestConcurrentAddRemove(t *testing.T) {
	gvrs := make([]schema.GroupVersionResource, 20)
	listKinds := make(map[schema.GroupVersionResource]string, 20)
	for i := range 20 {
		gvr := schema.GroupVersionResource{
			Version:  "v1",
			Resource: fmt.Sprintf("resource%ds", i),
		}
		gvrs[i] = gvr
		listKinds[gvr] = fmt.Sprintf("Resource%dList", i)
	}

	client := fakedynamic.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), listKinds,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	m, err := New(ctx, client)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer m.Stop()

	var wg sync.WaitGroup
	for _, gvr := range gvrs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Add(gvr)
			m.Remove(gvr)
		}()
	}
	wg.Wait()
}

func TestConcurrentHasLen(t *testing.T) {
	m, _ := newTestMux(t)
	if err := m.Add(podGVR); err != nil {
		t.Fatalf("Add: %v", err)
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Has(podGVR)
			m.Len()
			m.GVRs()
		}()
	}
	wg.Wait()
}

func TestConcurrentAddSameGVR(t *testing.T) {
	m, _ := newTestMux(t)

	errs := make(chan error, 10)
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.Add(podGVR); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Add: %v", err)
	}

	if m.Len() != 1 {
		t.Fatalf("Len after concurrent Add: got %d, want 1", m.Len())
	}
}

func TestToRuntimeObject_Unstructured(t *testing.T) {
	obj := testPod("test", "default")
	ro, ok := toRuntimeObject(obj)
	if !ok {
		t.Fatal("expected ok=true for *unstructured.Unstructured")
	}
	if ro != obj {
		t.Fatal("returned object should be the same pointer")
	}
}

func TestToRuntimeObject_DeletedFinalState(t *testing.T) {
	inner := testPod("deleted", "default")
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/deleted",
		Obj: inner,
	}

	ro, ok := toRuntimeObject(tombstone)
	if !ok {
		t.Fatal("expected ok=true for DeletedFinalStateUnknown")
	}
	if ro != inner {
		t.Fatal("returned object should be the inner pod")
	}
}

func TestToRuntimeObject_DeletedFinalState_NonRuntimeObj(t *testing.T) {
	tombstone := cache.DeletedFinalStateUnknown{
		Key: "default/unknown",
		Obj: "not a runtime.Object",
	}

	_, ok := toRuntimeObject(tombstone)
	if ok {
		t.Fatal("expected ok=false for non-runtime.Object inside tombstone")
	}
}

func TestToRuntimeObject_UnknownType(t *testing.T) {
	_, ok := toRuntimeObject("just a string")
	if ok {
		t.Fatal("expected ok=false for unknown type")
	}
}

func TestToRuntimeObject_Nil(t *testing.T) {
	_, ok := toRuntimeObject(nil)
	if ok {
		t.Fatal("expected ok=false for nil")
	}
}

func TestTweakListOptions_Empty(t *testing.T) {
	m := &Mux{cfg: muxConfig{}}
	lo := &metav1.ListOptions{
		ResourceVersion: "12345",
	}
	m.tweakListOptions(lo)

	// Must not touch fields it doesn't own.
	if lo.ResourceVersion != "12345" {
		t.Fatalf("ResourceVersion clobbered: got %q", lo.ResourceVersion)
	}
	if lo.LabelSelector != "" {
		t.Fatalf("LabelSelector unexpectedly set: %q", lo.LabelSelector)
	}
}

func TestTweakListOptions_MergesSelectors(t *testing.T) {
	m := &Mux{
		cfg: muxConfig{
			labelSelector: "app=web",
			fieldSelector: "metadata.name=test",
		},
	}
	lo := &metav1.ListOptions{
		ResourceVersion: "99",
	}
	m.tweakListOptions(lo)

	if lo.LabelSelector != "app=web" {
		t.Fatalf("LabelSelector: got %q, want %q", lo.LabelSelector, "app=web")
	}
	if lo.FieldSelector != "metadata.name=test" {
		t.Fatalf("FieldSelector: got %q, want %q", lo.FieldSelector, "metadata.name=test")
	}
	if lo.ResourceVersion != "99" {
		t.Fatal("ResourceVersion must not be overwritten by tweakListOptions")
	}
}

func containsAll(haystack []string, needles ...string) bool {
	set := make(map[string]bool, len(haystack))
	for _, s := range haystack {
		set[s] = true
	}
	for _, n := range needles {
		if !set[n] {
			return false
		}
	}
	return true
}

func containsGVR(list []schema.GroupVersionResource, target schema.GroupVersionResource) bool {
	for _, g := range list {
		if g == target {
			return true
		}
	}
	return false
}

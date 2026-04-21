package diffpreview

import (
	"testing"
)

// --- ChangeType --------------------------------------------------------

func TestDiff_BothEmpty(t *testing.T) {
	node := Diff(map[string]any{}, map[string]any{})
	if len(node.Children) != 0 {
		t.Fatalf("expected no children, got %d", len(node.Children))
	}
}

func TestDiff_BothNil(t *testing.T) {
	node := Diff(nil, nil)
	if len(node.Children) != 0 {
		t.Fatalf("expected no children for nil inputs, got %d", len(node.Children))
	}
}

// --- Scalar changes ----------------------------------------------------

func TestDiff_AddedKey(t *testing.T) {
	a := map[string]any{}
	b := map[string]any{"name": "pod-1"}

	node := Diff(a, b)
	child, ok := node.Children["name"]
	if !ok {
		t.Fatal("expected 'name' child")
	}
	if child.Change != Added {
		t.Fatalf("expected Added, got %v", child.Change)
	}
	if child.Value != "pod-1" {
		t.Fatalf("expected value 'pod-1', got %v", child.Value)
	}
}

func TestDiff_RemovedKey(t *testing.T) {
	a := map[string]any{"name": "pod-1"}
	b := map[string]any{}

	node := Diff(a, b)
	child := node.Children["name"]
	if child.Change != Removed {
		t.Fatalf("expected Removed, got %v", child.Change)
	}
	if child.Value != "pod-1" {
		t.Fatalf("expected value 'pod-1', got %v", child.Value)
	}
}

func TestDiff_ModifiedScalar(t *testing.T) {
	a := map[string]any{"replicas": 1}
	b := map[string]any{"replicas": 3}

	node := Diff(a, b)
	child := node.Children["replicas"]
	if child.Change != Modified {
		t.Fatalf("expected Modified, got %v", child.Change)
	}
	if child.Value != 3 {
		t.Fatalf("expected new value 3, got %v", child.Value)
	}
}

func TestDiff_UnchangedScalar(t *testing.T) {
	a := map[string]any{"kind": "Pod"}
	b := map[string]any{"kind": "Pod"}

	node := Diff(a, b)
	child := node.Children["kind"]
	if child.Change != Unchanged {
		t.Fatalf("expected Unchanged, got %v", child.Change)
	}
	if child.Value != "Pod" {
		t.Fatalf("expected value 'Pod', got %v", child.Value)
	}
}

func TestDiff_ScalarTypes(t *testing.T) {
	a := map[string]any{
		"str":   "hello",
		"num":   42,
		"float": 3.14,
		"flag":  true,
		"empty": nil,
	}
	b := map[string]any{
		"str":   "hello",
		"num":   42,
		"float": 3.14,
		"flag":  true,
		"empty": nil,
	}

	node := Diff(a, b)
	for key, child := range node.Children {
		if child.Change != Unchanged {
			t.Errorf("key %q: expected Unchanged, got %v", key, child.Change)
		}
	}
}

func TestDiff_TypeChange(t *testing.T) {
	a := map[string]any{"value": "text"}
	b := map[string]any{"value": 42}

	node := Diff(a, b)
	child := node.Children["value"]
	if child.Change != Modified {
		t.Fatalf("expected Modified for type change, got %v", child.Change)
	}
	if child.Value != 42 {
		t.Fatalf("expected new value 42, got %v", child.Value)
	}
}

// --- Nested maps -------------------------------------------------------

func TestDiff_NestedMapUnchanged(t *testing.T) {
	inner := map[string]any{"cpu": "100m", "memory": "128Mi"}
	a := map[string]any{"resources": inner}
	b := map[string]any{"resources": inner}

	node := Diff(a, b)
	res := node.Children["resources"]
	if res.Children == nil {
		t.Fatal("expected nested map Children")
	}
	for key, child := range res.Children {
		if child.Change != Unchanged {
			t.Errorf("resources.%s: expected Unchanged, got %v", key, child.Change)
		}
	}
}

func TestDiff_NestedMapModified(t *testing.T) {
	a := map[string]any{
		"metadata": map[string]any{
			"name":      "pod-1",
			"namespace": "default",
		},
	}
	b := map[string]any{
		"metadata": map[string]any{
			"name":      "pod-1",
			"namespace": "kube-system",
		},
	}

	node := Diff(a, b)
	meta := node.Children["metadata"]
	if meta.Children["name"].Change != Unchanged {
		t.Error("name should be Unchanged")
	}
	ns := meta.Children["namespace"]
	if ns.Change != Modified {
		t.Errorf("expected Modified, got %v", ns.Change)
	}
	if ns.Value != "kube-system" {
		t.Errorf("expected new value 'kube-system', got %v", ns.Value)
	}
}

func TestDiff_NestedMapAdded(t *testing.T) {
	a := map[string]any{"metadata": map[string]any{"name": "pod-1"}}
	b := map[string]any{
		"metadata": map[string]any{
			"name": "pod-1",
			"labels": map[string]any{
				"app": "nginx",
			},
		},
	}

	node := Diff(a, b)
	labels := node.Children["metadata"].Children["labels"]
	if labels == nil {
		t.Fatal("expected labels child")
	}
	app := labels.Children["app"]
	if app.Change != Added {
		t.Fatalf("expected Added, got %v", app.Change)
	}
}

func TestDiff_NestedMapRemoved(t *testing.T) {
	a := map[string]any{
		"spec": map[string]any{
			"replicas": 3,
			"selector": map[string]any{"app": "web"},
		},
	}
	b := map[string]any{
		"spec": map[string]any{
			"replicas": 3,
		},
	}

	node := Diff(a, b)
	sel := node.Children["spec"].Children["selector"]
	if sel == nil {
		t.Fatal("expected selector child")
	}
	// The whole subtree should be Removed.
	if !hasChanges(sel) {
		t.Fatal("expected changes in removed subtree")
	}
	app := sel.Children["app"]
	if app.Change != Removed {
		t.Fatalf("expected Removed, got %v", app.Change)
	}
}

// --- Deeply nested -----------------------------------------------------

func TestDiff_DeeplyNested(t *testing.T) {
	a := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "old",
				},
			},
		},
	}
	b := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{
					"d": "new",
				},
			},
		},
	}

	node := Diff(a, b)
	d := node.Children["a"].Children["b"].Children["c"].Children["d"]
	if d.Change != Modified {
		t.Fatalf("expected Modified at depth 4, got %v", d.Change)
	}
	if d.Value != "new" {
		t.Fatalf("expected 'new', got %v", d.Value)
	}
}

// --- List diffing: positional ------------------------------------------

func TestDiff_ListPositional_Unchanged(t *testing.T) {
	a := map[string]any{"ports": []any{80, 443}}
	b := map[string]any{"ports": []any{80, 443}}

	node := Diff(a, b)
	ports := node.Children["ports"]
	if ports.List == nil {
		t.Fatal("expected List")
	}
	if len(ports.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(ports.List))
	}
	for i, item := range ports.List {
		if item.Change != Unchanged {
			t.Errorf("item %d: expected Unchanged, got %v", i, item.Change)
		}
	}
}

func TestDiff_ListPositional_Modified(t *testing.T) {
	a := map[string]any{"args": []any{"--verbose", "--port=8080"}}
	b := map[string]any{"args": []any{"--verbose", "--port=9090"}}

	node := Diff(a, b)
	args := node.Children["args"]
	if args.List[0].Change != Unchanged {
		t.Error("first arg should be Unchanged")
	}
	if args.List[1].Change != Modified {
		t.Error("second arg should be Modified")
	}
	if args.List[1].Value != "--port=9090" {
		t.Errorf("expected '--port=9090', got %v", args.List[1].Value)
	}
}

func TestDiff_ListPositional_Added(t *testing.T) {
	a := map[string]any{"tags": []any{"v1"}}
	b := map[string]any{"tags": []any{"v1", "v2"}}

	node := Diff(a, b)
	tags := node.Children["tags"]
	if len(tags.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(tags.List))
	}
	if tags.List[0].Change != Unchanged {
		t.Error("first tag should be Unchanged")
	}
	if tags.List[1].Change != Added {
		t.Error("second tag should be Added")
	}
}

func TestDiff_ListPositional_Removed(t *testing.T) {
	a := map[string]any{"tags": []any{"v1", "v2"}}
	b := map[string]any{"tags": []any{"v1"}}

	node := Diff(a, b)
	tags := node.Children["tags"]
	if len(tags.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(tags.List))
	}
	if tags.List[1].Change != Removed {
		t.Error("second tag should be Removed")
	}
}

func TestDiff_ListPositional_Empty(t *testing.T) {
	a := map[string]any{"items": []any{}}
	b := map[string]any{"items": []any{}}

	node := Diff(a, b)
	items := node.Children["items"]
	if items.List == nil {
		t.Fatal("expected List (possibly empty)")
	}
	if len(items.List) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items.List))
	}
}

func TestDiff_ListPositional_OneEmptyOneNot(t *testing.T) {
	a := map[string]any{"items": []any{}}
	b := map[string]any{"items": []any{"new"}}

	node := Diff(a, b)
	items := node.Children["items"]
	if len(items.List) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items.List))
	}
	if items.List[0].Change != Added {
		t.Errorf("expected Added, got %v", items.List[0].Change)
	}
}

// --- List diffing: key-based matching ----------------------------------

func TestDiff_ListByKey_MatchByName(t *testing.T) {
	a := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx:1.19"},
			map[string]any{"name": "sidecar", "image": "envoy:1.0"},
		},
	}
	b := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx:1.21"},
			map[string]any{"name": "sidecar", "image": "envoy:1.0"},
		},
	}

	node := Diff(a, b)
	containers := node.Children["containers"]
	if containers.List == nil {
		t.Fatal("expected List for containers")
	}

	// "app" container should have Modified image
	app := containers.List[0]
	if app.Children == nil {
		t.Fatal("expected Children in first list item")
	}
	img := app.Children["image"]
	if img.Change != Modified {
		t.Errorf("expected Modified image, got %v", img.Change)
	}
	if img.Value != "nginx:1.21" {
		t.Errorf("expected 'nginx:1.21', got %v", img.Value)
	}

	// "sidecar" should be unchanged
	sidecar := containers.List[1]
	if hasChanges(sidecar) {
		t.Error("expected sidecar to be fully unchanged")
	}
}

func TestDiff_ListByKey_Added(t *testing.T) {
	a := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx"},
		},
	}
	b := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx"},
			map[string]any{"name": "sidecar", "image": "envoy"},
		},
	}

	node := Diff(a, b)
	containers := node.Children["containers"]
	if len(containers.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(containers.List))
	}

	sidecar := containers.List[1]
	if sidecar.Children["name"].Change != Added {
		t.Error("expected Added for new container name")
	}
}

func TestDiff_ListByKey_Removed(t *testing.T) {
	a := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx"},
			map[string]any{"name": "sidecar", "image": "envoy"},
		},
	}
	b := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx"},
		},
	}

	node := Diff(a, b)
	containers := node.Children["containers"]
	if len(containers.List) != 2 {
		t.Fatalf("expected 2 items, got %d", len(containers.List))
	}

	sidecar := containers.List[1]
	if sidecar.Children["name"].Change != Removed {
		t.Errorf("expected Removed for deleted container, got %v", sidecar.Children["name"].Change)
	}
}

func TestDiff_ListByKey_Reordered(t *testing.T) {
	a := map[string]any{
		"containers": []any{
			map[string]any{"name": "app", "image": "nginx"},
			map[string]any{"name": "sidecar", "image": "envoy"},
		},
	}
	b := map[string]any{
		"containers": []any{
			map[string]any{"name": "sidecar", "image": "envoy"},
			map[string]any{"name": "app", "image": "nginx"},
		},
	}

	node := Diff(a, b)
	containers := node.Children["containers"]
	// Key-based matching should pair by name regardless of order.
	// Both containers are unchanged.
	for i, item := range containers.List {
		if hasChanges(item) {
			t.Errorf("item %d: expected no changes after reorder", i)
		}
	}
}

// --- List diffing: nested maps within lists ----------------------------

func TestDiff_ListOfMaps_NestedChange(t *testing.T) {
	a := map[string]any{
		"volumes": []any{
			map[string]any{
				"name": "data",
				"hostPath": map[string]any{
					"path": "/var/data",
					"type": "Directory",
				},
			},
		},
	}
	b := map[string]any{
		"volumes": []any{
			map[string]any{
				"name": "data",
				"hostPath": map[string]any{
					"path": "/mnt/data",
					"type": "Directory",
				},
			},
		},
	}

	node := Diff(a, b)
	vol := node.Children["volumes"].List[0]
	hp := vol.Children["hostPath"]
	if hp.Children["path"].Change != Modified {
		t.Error("expected hostPath.path to be Modified")
	}
	if hp.Children["type"].Change != Unchanged {
		t.Error("expected hostPath.type to be Unchanged")
	}
}

// --- Mixed changes (multiple operations at once) -----------------------

func TestDiff_MixedChanges(t *testing.T) {
	a := map[string]any{
		"keep":   "same",
		"modify": "old",
		"remove": "gone",
	}
	b := map[string]any{
		"keep":   "same",
		"modify": "new",
		"add":    "fresh",
	}

	node := Diff(a, b)

	if node.Children["keep"].Change != Unchanged {
		t.Error("keep should be Unchanged")
	}
	if node.Children["modify"].Change != Modified {
		t.Error("modify should be Modified")
	}
	if node.Children["remove"].Change != Removed {
		t.Error("remove should be Removed")
	}
	if node.Children["add"].Change != Added {
		t.Error("add should be Added")
	}
}

// --- hasChanges --------------------------------------------------------

func TestHasChanges(t *testing.T) {
	unchanged := buildFullNode(map[string]any{"a": 1, "b": 2}, Unchanged)
	if hasChanges(unchanged) {
		t.Error("expected no changes in unchanged tree")
	}

	added := buildFullNode(map[string]any{"a": 1}, Added)
	if !hasChanges(added) {
		t.Error("expected changes in added tree")
	}

	// A tree with one changed leaf deep inside.
	mixed := Diff(
		map[string]any{"a": map[string]any{"b": "old"}},
		map[string]any{"a": map[string]any{"b": "new"}},
	)
	if !hasChanges(mixed) {
		t.Error("expected changes in mixed tree")
	}
}

// --- buildFullNode -----------------------------------------------------

func TestBuildFullNode_Scalar(t *testing.T) {
	node := buildFullNode("hello", Added)
	if node.Change != Added || node.Value != "hello" {
		t.Errorf("unexpected scalar node: %+v", node)
	}
}

func TestBuildFullNode_Map(t *testing.T) {
	node := buildFullNode(map[string]any{"a": "b"}, Removed)
	if node.Change != Removed {
		t.Errorf("expected Removed, got %v", node.Change)
	}
	if node.Children["a"].Change != Removed {
		t.Error("child should inherit Removed")
	}
}

func TestBuildFullNode_List(t *testing.T) {
	node := buildFullNode([]any{1, 2, 3}, Added)
	if node.Change != Added {
		t.Errorf("expected Added, got %v", node.Change)
	}
	if len(node.List) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(node.List))
	}
	for i, item := range node.List {
		if item.Change != Added {
			t.Errorf("list item %d: expected Added, got %v", i, item.Change)
		}
	}
}

func TestBuildFullNode_NestedMapInList(t *testing.T) {
	val := []any{map[string]any{"x": 1}}
	node := buildFullNode(val, Added)
	inner := node.List[0]
	if inner.Children == nil {
		t.Fatal("expected map Children in list element")
	}
	if inner.Children["x"].Change != Added {
		t.Error("nested map child should be Added")
	}
}

func TestBuildFullNode_Nil(t *testing.T) {
	node := buildFullNode(nil, Unchanged)
	if node.Value != nil || node.Change != Unchanged {
		t.Errorf("unexpected nil node: %+v", node)
	}
}

// --- unionKeys ---------------------------------------------------------

func TestUnionKeys(t *testing.T) {
	a := map[string]any{"b": 1, "a": 2}
	b := map[string]any{"c": 3, "a": 4}

	keys := unionKeys(a, b)
	expected := []string{"a", "b", "c"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, keys)
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("index %d: expected %q, got %q", i, expected[i], k)
		}
	}
}

func TestUnionKeys_Empty(t *testing.T) {
	keys := unionKeys(nil, nil)
	if len(keys) != 0 {
		t.Fatalf("expected empty, got %v", keys)
	}
}

// --- findMatchKey ------------------------------------------------------

func TestFindMatchKey_Name(t *testing.T) {
	a := []any{map[string]any{"name": "a"}}
	b := []any{map[string]any{"name": "b"}}
	if key := findMatchKey(a, b); key != "name" {
		t.Errorf("expected 'name', got %q", key)
	}
}

func TestFindMatchKey_NoCommonKey(t *testing.T) {
	a := []any{map[string]any{"foo": 1}}
	b := []any{map[string]any{"bar": 2}}
	if key := findMatchKey(a, b); key != "" {
		t.Errorf("expected empty, got %q", key)
	}
}

func TestFindMatchKey_ScalarList(t *testing.T) {
	a := []any{1, 2, 3}
	b := []any{4, 5, 6}
	if key := findMatchKey(a, b); key != "" {
		t.Errorf("expected empty for scalar lists, got %q", key)
	}
}

func TestFindMatchKey_Priority(t *testing.T) {
	// Both "name" and "type" present: "name" has higher priority.
	a := []any{map[string]any{"name": "a", "type": "x"}}
	b := []any{map[string]any{"name": "b", "type": "y"}}
	if key := findMatchKey(a, b); key != "name" {
		t.Errorf("expected 'name' (higher priority), got %q", key)
	}
}

// --- allMapsHaveKey ----------------------------------------------------

func TestAllMapsHaveKey(t *testing.T) {
	list := []any{
		map[string]any{"name": "a"},
		map[string]any{"name": "b"},
	}
	if !allMapsHaveKey(list, "name") {
		t.Error("expected true")
	}
	if allMapsHaveKey(list, "missing") {
		t.Error("expected false for missing key")
	}
}

func TestAllMapsHaveKey_EmptyList(t *testing.T) {
	if allMapsHaveKey([]any{}, "name") {
		t.Error("expected false for empty list")
	}
}

func TestAllMapsHaveKey_OnlyScalars(t *testing.T) {
	if allMapsHaveKey([]any{1, "two"}, "name") {
		t.Error("expected false for all-scalar list")
	}
}

// --- Edge cases --------------------------------------------------------

func TestDiff_MapToScalar(t *testing.T) {
	// a has a nested map, b has a scalar at the same key -> type change.
	a := map[string]any{"value": map[string]any{"nested": true}}
	b := map[string]any{"value": "flat"}

	node := Diff(a, b)
	v := node.Children["value"]
	if v.Change != Modified {
		t.Fatalf("expected Modified for map→scalar change, got %v", v.Change)
	}
}

func TestDiff_ScalarToMap(t *testing.T) {
	a := map[string]any{"value": "flat"}
	b := map[string]any{"value": map[string]any{"nested": true}}

	node := Diff(a, b)
	v := node.Children["value"]
	// When types differ and both are present, it's Modified.
	if v.Change != Modified {
		t.Fatalf("expected Modified for scalar→map change, got %v", v.Change)
	}
}

func TestDiff_ListToScalar(t *testing.T) {
	a := map[string]any{"value": []any{1, 2}}
	b := map[string]any{"value": "text"}

	node := Diff(a, b)
	if node.Children["value"].Change != Modified {
		t.Error("expected Modified for list→scalar")
	}
}

func TestDiff_NilValues(t *testing.T) {
	a := map[string]any{"x": nil}
	b := map[string]any{"x": nil}

	node := Diff(a, b)
	if node.Children["x"].Change != Unchanged {
		t.Error("nil == nil should be Unchanged")
	}
}

func TestDiff_NilToValue(t *testing.T) {
	a := map[string]any{"x": nil}
	b := map[string]any{"x": "now-set"}

	node := Diff(a, b)
	if node.Children["x"].Change != Modified {
		t.Error("nil → value should be Modified")
	}
}

// --- Kubernetes-like full object diff ----------------------------------

func TestDiff_KubernetesDeployment(t *testing.T) {
	a := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
			"labels": map[string]any{
				"app":     "web",
				"version": "v1",
			},
		},
		"spec": map[string]any{
			"replicas": 2,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "web",
							"image": "nginx:1.19",
							"ports": []any{
								map[string]any{"containerPort": 80},
							},
						},
					},
				},
			},
		},
	}

	b := map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":      "web",
			"namespace": "default",
			"labels": map[string]any{
				"app":     "web",
				"version": "v2",
			},
			"annotations": map[string]any{
				"deploy-time": "2024-01-01",
			},
		},
		"spec": map[string]any{
			"replicas": 3,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "web",
							"image": "nginx:1.21",
							"ports": []any{
								map[string]any{"containerPort": 80},
								map[string]any{"containerPort": 443},
							},
						},
					},
				},
			},
		},
	}

	node := Diff(a, b)

	// apiVersion and kind unchanged
	if node.Children["apiVersion"].Change != Unchanged {
		t.Error("apiVersion should be Unchanged")
	}
	if node.Children["kind"].Change != Unchanged {
		t.Error("kind should be Unchanged")
	}

	// metadata.labels.version modified
	ver := node.Children["metadata"].Children["labels"].Children["version"]
	if ver.Change != Modified {
		t.Errorf("version label: expected Modified, got %v", ver.Change)
	}

	// metadata.annotations added
	ann := node.Children["metadata"].Children["annotations"]
	if ann == nil {
		t.Fatal("expected annotations child")
	}
	if ann.Children["deploy-time"].Change != Added {
		t.Error("deploy-time annotation should be Added")
	}

	// spec.replicas modified
	rep := node.Children["spec"].Children["replicas"]
	if rep.Change != Modified {
		t.Errorf("replicas: expected Modified, got %v", rep.Change)
	}
	if rep.Value != 3 {
		t.Errorf("replicas: expected 3, got %v", rep.Value)
	}

	// container image modified
	containers := node.Children["spec"].Children["template"].Children["spec"].Children["containers"]
	web := containers.List[0]
	img := web.Children["image"]
	if img.Change != Modified {
		t.Errorf("container image: expected Modified, got %v", img.Change)
	}

	// port 443 added
	ports := web.Children["ports"]
	if len(ports.List) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(ports.List))
	}
	port443 := ports.List[1]
	if port443.Children["containerPort"].Change != Added {
		t.Error("port 443 should be Added")
	}
}

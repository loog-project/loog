// Package simulation provides simulated Kubernetes resource data for the TUI.
// It creates realistic resource histories with various scenarios (rollouts, CrashLoopBackOff,
// operator reconcile bursts, reconcile loops, configuration drift) and provides live
// simulation via tea.Cmd that generates new revisions at random intervals.
//
// The package provides a Store (implementing tui.Store) and a Simulator (implementing
// tui.Simulator). Use New() to create a fully-populated Store, then pass it to
// tui.NewApp() with tui.WithSimulator().
package simulation

import (
	"sort"
	"time"

	"github.com/loog-project/loog/internal/resource"
)

// New creates a realistic set of Kubernetes resources with revision histories
// for the --simulate mode. The returned Store implements tui.Store and tui.Simulator.
func New() *Store {
	now := time.Now()
	resources := make(map[string]*resource.ResourceData)

	addResource := func(uid, kind, name, ns string, starred bool, revisions []resource.Revision) {
		resources[uid] = &resource.ResourceData{
			Resource: resource.Resource{
				UID:       uid,
				Kind:      kind,
				Name:      name,
				Namespace: ns,
				Starred:   starred,
			},
			Revisions: revisions,
		}
	}

	// ─── SCENARIO 1: nginx deployment rollout (default namespace) ───

	addResource("uid-pod-nginx-1", "Pod", "nginx-7d4f8b-abc12", "default", true, []resource.Revision{
		{
			ID: 0x0001, EventType: resource.EventAdded, Time: now.Add(-12 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name": "nginx-7d4f8b-abc12", "namespace": "default", "uid": "uid-pod-nginx-1",
					"labels":          map[string]any{"app": "nginx", "pod-template-hash": "7d4f8b"},
					"ownerReferences": []any{map[string]any{"kind": "ReplicaSet", "name": "nginx-deployment-7d4f8b"}},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx:1.24",
							"ports": []any{map[string]any{"containerPort": float64(80)}},
						},
					},
				},
				"status": map[string]any{"phase": "Pending", "conditions": []any{}},
			},
		},
		{
			ID:         0x0003,
			PreviousID: 0x0001,
			EventType:  resource.EventModified,
			Time:       now.Add(-11 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name": "nginx-7d4f8b-abc12", "namespace": "default", "uid": "uid-pod-nginx-1",
					"labels": map[string]any{"app": "nginx", "pod-template-hash": "7d4f8b"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx:1.24",
							"ports": []any{map[string]any{"containerPort": float64(80)}},
						},
					},
				},
				"status": map[string]any{
					"phase":      "Running",
					"podIP":      "10.0.1.15",
					"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
				},
			},
			Patch: map[string]any{"status": map[string]any{"phase": "Running", "podIP": "10.0.1.15"}},
		},
		{
			ID:         0x000a,
			PreviousID: 0x0003,
			EventType:  resource.EventModified,
			Time:       now.Add(-3 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name": "nginx-7d4f8b-abc12", "namespace": "default", "uid": "uid-pod-nginx-1",
					"labels": map[string]any{"app": "nginx", "pod-template-hash": "7d4f8b"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "nginx",
							"image": "nginx:1.25",
							"ports": []any{map[string]any{"containerPort": float64(80)}},
						},
					},
				},
				"status": map[string]any{
					"phase":      "Running",
					"podIP":      "10.0.1.15",
					"conditions": []any{map[string]any{"type": "Ready", "status": "True"}},
				},
			},
			Patch: map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"image": "nginx:1.25"}}}},
		},
	})

	addResource("uid-pod-nginx-2", "Pod", "nginx-7d4f8b-def34", "default", false, []resource.Revision{
		{
			ID: 0x0002, EventType: resource.EventAdded, Time: now.Add(-12 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "nginx-7d4f8b-def34", "namespace": "default"},
				"spec":     map[string]any{"containers": []any{map[string]any{"name": "nginx", "image": "nginx:1.24"}}},
				"status":   map[string]any{"phase": "Pending"},
			},
		},
		{
			ID:         0x0004,
			PreviousID: 0x0002,
			EventType:  resource.EventModified,
			Time:       now.Add(-11 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "nginx-7d4f8b-def34", "namespace": "default"},
				"spec":     map[string]any{"containers": []any{map[string]any{"name": "nginx", "image": "nginx:1.24"}}},
				"status":   map[string]any{"phase": "Running", "podIP": "10.0.1.16"},
			},
			Patch: map[string]any{"status": map[string]any{"phase": "Running", "podIP": "10.0.1.16"}},
		},
	})

	addResource("uid-pod-nginx-3", "Pod", "nginx-7d4f8b-ghi56", "default", false, []resource.Revision{
		{
			ID: 0x0005, EventType: resource.EventAdded, Time: now.Add(-10 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "nginx-7d4f8b-ghi56", "namespace": "default"},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":      "nginx",
							"image":     "nginx:1.24",
							"resources": map[string]any{"limits": map[string]any{"memory": "64Mi"}},
						},
					},
				},
				"status": map[string]any{"phase": "Running"},
			},
		},
		{
			ID:         0x0008,
			PreviousID: 0x0005,
			EventType:  resource.EventModified,
			Time:       now.Add(-7 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "nginx-7d4f8b-ghi56", "namespace": "default"},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":      "nginx",
							"image":     "nginx:1.24",
							"resources": map[string]any{"limits": map[string]any{"memory": "64Mi"}},
						},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name": "nginx",
							"state": map[string]any{
								"terminated": map[string]any{
									"reason":   "OOMKilled",
									"exitCode": float64(137),
								},
							},
						},
					},
				},
			},
			Patch: map[string]any{"status": map[string]any{"containerStatuses": []any{map[string]any{"state": map[string]any{"terminated": map[string]any{"reason": "OOMKilled"}}}}}},
		},
		{
			ID:         0x0009,
			PreviousID: 0x0008,
			EventType:  resource.EventModified,
			Time:       now.Add(-6 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{"name": "nginx-7d4f8b-ghi56", "namespace": "default"},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":      "nginx",
							"image":     "nginx:1.24",
							"resources": map[string]any{"limits": map[string]any{"memory": "64Mi"}},
						},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name":         "nginx",
							"restartCount": float64(1),
							"state":        map[string]any{"running": map[string]any{}},
						},
					},
				},
			},
			Patch: map[string]any{
				"status": map[string]any{
					"containerStatuses": []any{
						map[string]any{
							"restartCount": float64(1),
							"state":        map[string]any{"running": map[string]any{}},
						},
					},
				},
			},
		},
	})

	addResource("uid-deploy-nginx", "Deployment", "nginx-deployment", "default", true, []resource.Revision{
		{
			ID: 0x0006, EventType: resource.EventAdded, Time: now.Add(-13 * time.Minute), Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{"name": "nginx-deployment", "namespace": "default"},
				"spec": map[string]any{
					"replicas": float64(1),
					"selector": map[string]any{"matchLabels": map[string]any{"app": "nginx"}},
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "nginx",
									"image": "nginx:1.24",
								},
							},
						},
					},
				},
				"status": map[string]any{"readyReplicas": float64(0), "replicas": float64(1)},
			},
		},
		{
			ID:         0x0007,
			PreviousID: 0x0006,
			EventType:  resource.EventModified,
			Time:       now.Add(-11 * time.Minute),
			Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{"name": "nginx-deployment", "namespace": "default"},
				"spec": map[string]any{
					"replicas": float64(3),
					"selector": map[string]any{"matchLabels": map[string]any{"app": "nginx"}},
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "nginx",
									"image": "nginx:1.24",
								},
							},
						},
					},
				},
				"status": map[string]any{"readyReplicas": float64(3), "replicas": float64(3)},
			},
			Patch: map[string]any{
				"spec":   map[string]any{"replicas": float64(3)},
				"status": map[string]any{"readyReplicas": float64(3), "replicas": float64(3)},
			},
		},
		{
			ID:         0x000b,
			PreviousID: 0x0007,
			EventType:  resource.EventModified,
			Time:       now.Add(-3 * time.Minute),
			Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{"name": "nginx-deployment", "namespace": "default"},
				"spec": map[string]any{
					"replicas": float64(3),
					"selector": map[string]any{"matchLabels": map[string]any{"app": "nginx"}},
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "nginx",
									"image": "nginx:1.25",
								},
							},
						},
					},
				},
				"status": map[string]any{
					"readyReplicas":   float64(3),
					"replicas":        float64(3),
					"updatedReplicas": float64(3),
				},
			},
			Patch: map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{"containers": []any{map[string]any{"image": "nginx:1.25"}}}}}},
		},
	})

	addResource("uid-svc-nginx", "Service", "nginx-svc", "default", false, []resource.Revision{
		{
			ID: 0x000c, EventType: resource.EventAdded, Time: now.Add(-13 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Service",
				"metadata": map[string]any{"name": "nginx-svc", "namespace": "default"},
				"spec": map[string]any{
					"type":     "ClusterIP",
					"ports":    []any{map[string]any{"port": float64(80), "targetPort": float64(80)}},
					"selector": map[string]any{"app": "nginx"},
				},
			},
		},
		{
			ID:         0x000d,
			PreviousID: 0x000c,
			EventType:  resource.EventModified,
			Time:       now.Add(-2 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Service",
				"metadata": map[string]any{"name": "nginx-svc", "namespace": "default"},
				"spec": map[string]any{
					"type":     "ClusterIP",
					"ports":    []any{map[string]any{"port": float64(8080), "targetPort": float64(80)}},
					"selector": map[string]any{"app": "nginx"},
				},
			},
			Patch: map[string]any{"spec": map[string]any{"ports": []any{map[string]any{"port": float64(8080)}}}},
		},
	})

	addResource("uid-cm-nginx", "ConfigMap", "nginx-config", "default", false, []resource.Revision{
		{
			ID: 0x000e, EventType: resource.EventAdded, Time: now.Add(-13 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "nginx-config", "namespace": "default"},
				"data":     map[string]any{"nginx.conf": "server { listen 80; }"},
			},
		},
		{
			ID:         0x000f,
			PreviousID: 0x000e,
			EventType:  resource.EventModified,
			Time:       now.Add(-5 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "nginx-config", "namespace": "default"},
				"data": map[string]any{
					"nginx.conf": "server { listen 80; gzip on; }",
					"extra.conf": "# extra config",
				},
			},
			Patch: map[string]any{
				"data": map[string]any{
					"nginx.conf": "server { listen 80; gzip on; }",
					"extra.conf": "# extra config",
				},
			},
		},
	})

	// ─── SCENARIO 2: api-server with CrashLoopBackOff ───

	addResource("uid-pod-api", "Pod", "api-server-6b7c-xyz99", "default", false, []resource.Revision{
		{
			ID: 0x0010, EventType: resource.EventAdded, Time: now.Add(-15 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name":      "api-server-6b7c-xyz99",
					"namespace": "default",
					"labels":    map[string]any{"app": "api-server"},
				},
				"spec":   map[string]any{"containers": []any{map[string]any{"name": "api", "image": "myapp/api:v2.1.0"}}},
				"status": map[string]any{"phase": "Running"},
			},
		},
		{
			ID:         0x0011,
			PreviousID: 0x0010,
			EventType:  resource.EventModified,
			Time:       now.Add(-9 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name":      "api-server-6b7c-xyz99",
					"namespace": "default",
					"labels":    map[string]any{"app": "api-server"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "api",
							"image": "myapp/api:v2.1.0",
						},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name":         "api",
							"state":        map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
							"restartCount": float64(3),
						},
					},
				},
			},
			Patch: map[string]any{
				"status": map[string]any{
					"containerStatuses": []any{
						map[string]any{
							"state":        map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
							"restartCount": float64(3),
						},
					},
				},
			},
		},
		{
			ID:         0x0014,
			PreviousID: 0x0011,
			EventType:  resource.EventModified,
			Time:       now.Add(-4 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name":      "api-server-6b7c-xyz99",
					"namespace": "default",
					"labels":    map[string]any{"app": "api-server"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "api",
							"image": "myapp/api:v2.1.1",
						},
					},
				},
				"status": map[string]any{
					"phase": "Running",
					"containerStatuses": []any{
						map[string]any{
							"name":         "api",
							"state":        map[string]any{"running": map[string]any{}},
							"restartCount": float64(4),
						},
					},
				},
			},
			Patch: map[string]any{
				"spec":   map[string]any{"containers": []any{map[string]any{"image": "myapp/api:v2.1.1"}}},
				"status": map[string]any{"containerStatuses": []any{map[string]any{"state": map[string]any{"running": map[string]any{}}}}},
			},
		},
	})

	addResource("uid-deploy-api", "Deployment", "api-server", "default", false, []resource.Revision{
		{
			ID: 0x0012, EventType: resource.EventAdded, Time: now.Add(-15 * time.Minute), Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{"name": "api-server", "namespace": "default"},
				"spec": map[string]any{
					"replicas": float64(2),
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "api",
									"image": "myapp/api:v2.1.0",
								},
							},
						},
					},
				},
				"status": map[string]any{"readyReplicas": float64(2), "replicas": float64(2)},
			},
		},
	})

	addResource("uid-svc-api", "Service", "api-svc", "default", false, []resource.Revision{
		{
			ID: 0x0013, EventType: resource.EventAdded, Time: now.Add(-15 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Service",
				"metadata": map[string]any{"name": "api-svc", "namespace": "default"},
				"spec": map[string]any{
					"type":     "ClusterIP",
					"ports":    []any{map[string]any{"port": float64(8080), "targetPort": float64(8080)}},
					"selector": map[string]any{"app": "api-server"},
				},
			},
		},
	})

	// ─── SCENARIO 3: kube-system resources ───

	addResource("uid-pod-coredns", "Pod", "coredns-5644d8b9d-mn123", "kube-system", false, []resource.Revision{
		{
			ID: 0x0015, EventType: resource.EventAdded, Time: now.Add(-30 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]any{
					"name":      "coredns-5644d8b9d-mn123",
					"namespace": "kube-system",
					"labels":    map[string]any{"k8s-app": "kube-dns"},
				},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "coredns",
							"image": "registry.k8s.io/coredns/coredns:v1.11.1",
						},
					},
				},
				"status": map[string]any{"phase": "Running", "podIP": "10.0.0.2"},
			},
		},
	})

	addResource("uid-cm-coredns", "ConfigMap", "coredns-config", "kube-system", false, []resource.Revision{
		{
			ID: 0x0016, EventType: resource.EventAdded, Time: now.Add(-30 * time.Minute), Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "coredns-config", "namespace": "kube-system"},
				"data":     map[string]any{"Corefile": ".:53 {\n    errors\n    health\n    kubernetes cluster.local\n    forward . /etc/resolv.conf\n}"},
			},
		},
		{
			ID:         0x0017,
			PreviousID: 0x0016,
			EventType:  resource.EventModified,
			Time:       now.Add(-8 * time.Minute),
			Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{"name": "coredns-config", "namespace": "kube-system"},
				"data":     map[string]any{"Corefile": ".:53 {\n    errors\n    health\n    kubernetes cluster.local\n    forward . 8.8.8.8 8.8.4.4\n}"},
			},
			Patch: map[string]any{"data": map[string]any{"Corefile": ".:53 {\n    errors\n    health\n    kubernetes cluster.local\n    forward . 8.8.8.8 8.8.4.4\n}"}},
		},
	})

	// ─── SCENARIO 4: Operator reconcile burst ───

	burstBase := now.Add(-1 * time.Minute)

	addResource("uid-crd-myapp", "MyApp", "production", "operators", true, []resource.Revision{
		{
			ID: 0x0020, EventType: resource.EventAdded, Time: burstBase.Add(-10 * time.Minute), Object: map[string]any{
				"apiVersion": "myapp.example.com/v1", "kind": "MyApp",
				"metadata": map[string]any{"name": "production", "namespace": "operators"},
				"spec":     map[string]any{"replicas": float64(2), "image": "myapp/web:v1.0.0", "port": float64(3000)},
				"status":   map[string]any{"phase": "Pending", "conditions": []any{}},
			},
		},
		{
			ID:         0x0025,
			PreviousID: 0x0020,
			EventType:  resource.EventModified,
			Time:       burstBase,
			Object: map[string]any{
				"apiVersion": "myapp.example.com/v1", "kind": "MyApp",
				"metadata": map[string]any{"name": "production", "namespace": "operators"},
				"spec":     map[string]any{"replicas": float64(3), "image": "myapp/web:v1.1.0", "port": float64(3000)},
				"status":   map[string]any{"phase": "Reconciling"},
			},
			Patch: map[string]any{
				"spec":   map[string]any{"replicas": float64(3), "image": "myapp/web:v1.1.0"},
				"status": map[string]any{"phase": "Reconciling"},
			},
		},
		{
			ID:         0x002a,
			PreviousID: 0x0025,
			EventType:  resource.EventModified,
			Time:       burstBase.Add(5 * time.Second),
			Object: map[string]any{
				"apiVersion": "myapp.example.com/v1", "kind": "MyApp",
				"metadata": map[string]any{"name": "production", "namespace": "operators"},
				"spec":     map[string]any{"replicas": float64(3), "image": "myapp/web:v1.1.0", "port": float64(3000)},
				"status": map[string]any{
					"phase":      "Ready",
					"conditions": []any{map[string]any{"type": "Available", "status": "True"}},
				},
			},
			Patch: map[string]any{"status": map[string]any{"phase": "Ready"}},
		},
	})

	addResource("uid-deploy-myapp", "Deployment", "myapp-web", "operators", false, []resource.Revision{
		{
			ID: 0x0026, EventType: resource.EventAdded, Time: burstBase.Add(1 * time.Second), Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{
					"name":            "myapp-web",
					"namespace":       "operators",
					"ownerReferences": []any{map[string]any{"kind": "MyApp", "name": "production"}},
				},
				"spec": map[string]any{
					"replicas": float64(3),
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "web",
									"image": "myapp/web:v1.1.0",
								},
							},
						},
					},
				},
				"status": map[string]any{"readyReplicas": float64(0), "replicas": float64(3)},
			},
		},
		{
			ID:         0x0029,
			PreviousID: 0x0026,
			EventType:  resource.EventModified,
			Time:       burstBase.Add(4 * time.Second),
			Object: map[string]any{
				"apiVersion": "apps/v1", "kind": "Deployment",
				"metadata": map[string]any{
					"name":            "myapp-web",
					"namespace":       "operators",
					"ownerReferences": []any{map[string]any{"kind": "MyApp", "name": "production"}},
				},
				"spec": map[string]any{
					"replicas": float64(3),
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "web",
									"image": "myapp/web:v1.1.0",
								},
							},
						},
					},
				},
				"status": map[string]any{"readyReplicas": float64(3), "replicas": float64(3)},
			},
			Patch: map[string]any{"status": map[string]any{"readyReplicas": float64(3)}},
		},
	})

	addResource("uid-svc-myapp", "Service", "myapp-svc", "operators", false, []resource.Revision{
		{
			ID: 0x0027, EventType: resource.EventAdded, Time: burstBase.Add(2 * time.Second), Object: map[string]any{
				"apiVersion": "v1", "kind": "Service",
				"metadata": map[string]any{
					"name":            "myapp-svc",
					"namespace":       "operators",
					"ownerReferences": []any{map[string]any{"kind": "MyApp", "name": "production"}},
				},
				"spec": map[string]any{
					"type":     "ClusterIP",
					"ports":    []any{map[string]any{"port": float64(3000), "targetPort": float64(3000)}},
					"selector": map[string]any{"app": "myapp-web"},
				},
			},
		},
	})

	addResource("uid-cm-myapp", "ConfigMap", "myapp-config", "operators", false, []resource.Revision{
		{
			ID: 0x0028, EventType: resource.EventAdded, Time: burstBase.Add(3 * time.Second), Object: map[string]any{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]any{
					"name":            "myapp-config",
					"namespace":       "operators",
					"ownerReferences": []any{map[string]any{"kind": "MyApp", "name": "production"}},
				},
				"data": map[string]any{"DATABASE_URL": "postgres://db:5432/myapp", "LOG_LEVEL": "info"},
			},
		},
	})

	// ─── SCENARIO 5: Reconcile loop ───

	loopObj1 := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{
			"name": "flaky-operator-deploy", "namespace": "default",
			"annotations": map[string]any{"operator.example.com/reconcile-hash": "abc123"},
		},
		"spec":   map[string]any{"replicas": float64(1)},
		"status": map[string]any{"readyReplicas": float64(1)},
	}
	loopObj2 := map[string]any{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]any{
			"name": "flaky-operator-deploy", "namespace": "default",
			"annotations": map[string]any{"operator.example.com/reconcile-hash": "def456"},
		},
		"spec":   map[string]any{"replicas": float64(1)},
		"status": map[string]any{"readyReplicas": float64(1)},
	}

	loopRevisions := make([]resource.Revision, 0, 8)
	loopBase := now.Add(-2 * time.Minute)
	for i := range 8 {
		obj := loopObj1
		if i%2 == 1 {
			obj = loopObj2
		}
		rev := resource.Revision{
			ID:        resource.RevisionID(0x0030 + i),
			EventType: resource.EventModified,
			Time:      loopBase.Add(time.Duration(i*15) * time.Second),
			Object:    obj,
			Patch:     map[string]any{"metadata": map[string]any{"annotations": map[string]any{"operator.example.com/reconcile-hash": "changed"}}},
		}
		if i == 0 {
			rev.EventType = resource.EventAdded
			rev.Patch = nil
		} else {
			rev.PreviousID = resource.RevisionID(0x0030 + i - 1)
		}
		loopRevisions = append(loopRevisions, rev)
	}
	addResource("uid-deploy-flaky", "Deployment", "flaky-operator-deploy", "default", false, loopRevisions)

	// ─── Build timeline (newest first) ───
	var timeline []resource.TimelineEntry
	for _, rd := range resources {
		for _, rev := range rd.Revisions {
			timeline = append(timeline, resource.TimelineEntry{
				Resource: rd.Resource,
				Revision: rev,
			})
		}
	}
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Revision.Time.After(timeline[j].Revision.Time)
	})

	// ─── Build kind groups ───
	allResources := make([]*resource.ResourceData, 0, len(resources))
	for _, rd := range resources {
		allResources = append(allResources, rd)
	}
	kindGroups := resource.BuildKindGroups(allResources)

	// ─── Cluster Resource Types ───
	clusterResourceTypes := []resource.ResourceKind{
		{Kind: "Secret", APIVersion: "v1", Resource: "secrets", Namespaced: true},
		{Kind: "Ingress", APIVersion: "networking.k8s.io/v1", Resource: "ingresses", Namespaced: true},
		{Kind: "CronJob", APIVersion: "batch/v1", Resource: "cronjobs", Namespaced: true},
		{Kind: "HPA", APIVersion: "autoscaling/v2", Resource: "horizontalpodautoscalers", Namespaced: true},
		{Kind: "PodDisruptionBudget", APIVersion: "policy/v1", Resource: "poddisruptionbudgets", Namespaced: true},
		{Kind: "Namespace", APIVersion: "v1", Resource: "namespaces", Namespaced: false},
		{Kind: "ServiceAccount", APIVersion: "v1", Resource: "serviceaccounts", Namespaced: true},
		{Kind: "ClusterRole", APIVersion: "rbac.authorization.k8s.io/v1", Resource: "clusterroles", Namespaced: false},
		{Kind: "Endpoints", APIVersion: "v1", Resource: "endpoints", Namespaced: true},
		{Kind: "PersistentVolume", APIVersion: "v1", Resource: "persistentvolumes", Namespaced: false},
		{Kind: "PersistentVolumeClaim", APIVersion: "v1", Resource: "persistentvolumeclaims", Namespaced: true},
		{Kind: "NetworkPolicy", APIVersion: "networking.k8s.io/v1", Resource: "networkpolicies", Namespaced: true},
		{Kind: "Job", APIVersion: "batch/v1", Resource: "jobs", Namespaced: true},
		{Kind: "DaemonSet", APIVersion: "apps/v1", Resource: "daemonsets", Namespaced: true},
		{Kind: "StatefulSet", APIVersion: "apps/v1", Resource: "statefulsets", Namespaced: true},
		{Kind: "Role", APIVersion: "rbac.authorization.k8s.io/v1", Resource: "roles", Namespaced: true},
		{Kind: "RoleBinding", APIVersion: "rbac.authorization.k8s.io/v1", Resource: "rolebindings", Namespaced: true},
		{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
			Resource:   "clusterrolebindings",
			Namespaced: false,
		},
		{Kind: "LimitRange", APIVersion: "v1", Resource: "limitranges", Namespaced: true},
		{Kind: "ResourceQuota", APIVersion: "v1", Resource: "resourcequotas", Namespaced: true},
	}

	return NewStore(resources, timeline, kindGroups, clusterResourceTypes)
}

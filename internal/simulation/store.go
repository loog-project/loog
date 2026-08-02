package simulation

import (
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/loog-project/loog/internal/resource"
	"github.com/loog-project/loog/internal/tui"
)

// Store is an in-memory tui.Store implementation for simulation and testing.
// It also implements tui.Simulator so it can generate live data.
type Store struct {
	mu sync.RWMutex

	resources            map[string]*resource.Data
	timeline             []resource.TimelineEntry
	kindGroups           []*resource.KindGroup
	clusterResourceTypes []resource.Kind
	totalRevisions       int    // cached count of all revisions across resources
	nextID               uint64 // monotonic source of unique revision IDs
}

// Compile-time check that Store implements both interfaces.
var (
	_ tui.Store     = (*Store)(nil)
	_ tui.Simulator = (*Store)(nil)
)

// NewStore creates a Store from pre-built data.
func NewStore(
	resources map[string]*resource.Data,
	timeline []resource.TimelineEntry,
	kindGroups []*resource.KindGroup,
	clusterResourceTypes []resource.Kind,
) *Store {
	if resources == nil {
		resources = make(map[string]*resource.Data)
	}
	totalRevs := 0
	var maxID uint64
	for _, rd := range resources {
		totalRevs += len(rd.Revisions)
		for _, rev := range rd.Revisions {
			if uint64(rev.ID) > maxID {
				maxID = uint64(rev.ID)
			}
		}
	}
	return &Store{
		resources:            resources,
		timeline:             timeline,
		kindGroups:           kindGroups,
		clusterResourceTypes: clusterResourceTypes,
		totalRevisions:       totalRevs,
		nextID:               maxID + 1,
	}
}

// nextRevisionIDLocked returns a unique, monotonically increasing revision ID.
// The caller must hold s.mu.
func (s *Store) nextRevisionIDLocked() resource.RevisionID {
	if s.nextID == 0 {
		s.nextID = 1
	}
	id := s.nextID
	s.nextID++
	return resource.RevisionID(id)
}

func (s *Store) AllResources() []*resource.Data {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.allResourcesLocked()
}

func (s *Store) StarredResources() []*resource.Data {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*resource.Data
	for _, rd := range s.resources {
		if rd.Resource.Starred {
			result = append(result, rd)
		}
	}
	return result
}

func (s *Store) GetResource(uid string) *resource.Data {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resources[uid]
}

func (s *Store) TotalResourceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.resources)
}

func (s *Store) TotalRevisionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.totalRevisions
}

func (s *Store) FilterResources(expr string) []*resource.Data {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if expr == "" {
		return s.allResourcesLocked()
	}
	lower := strings.ToLower(expr)
	var result []*resource.Data
	for _, rd := range s.resources {
		if resource.MatchesSubstring(lower, rd.Resource) {
			result = append(result, rd)
		}
	}
	return result
}

func (s *Store) FilterTimeline(expr string, starredOnly bool) []resource.TimelineEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if expr == "" && !starredOnly {
		result := make([]resource.TimelineEntry, len(s.timeline))
		copy(result, s.timeline)
		return result
	}
	lower := strings.ToLower(expr)
	var result []resource.TimelineEntry
	for _, e := range s.timeline {
		if starredOnly && !e.Resource.Starred {
			continue
		}
		if expr != "" && !resource.MatchesSubstring(lower, e.Resource) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (s *Store) Timeline() []resource.TimelineEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]resource.TimelineEntry, len(s.timeline))
	copy(result, s.timeline)
	return result
}

func (s *Store) KindGroups() []*resource.KindGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*resource.KindGroup, len(s.kindGroups))
	copy(out, s.kindGroups)
	return out
}

func (s *Store) WatchedKinds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.watchedKindsLocked()
}

func (s *Store) watchedKindsLocked() []string {
	kindSet := make(map[string]bool)
	for _, rd := range s.resources {
		kindSet[rd.Resource.Kind] = true
	}
	kinds := make([]string, 0, len(kindSet))
	for k := range kindSet {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func (s *Store) ResourceCountByKind(kind string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, rd := range s.resources {
		if rd.Resource.Kind == kind {
			count++
		}
	}
	return count
}

func (s *Store) RevisionCountByKind(kind string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, rd := range s.resources {
		if rd.Resource.Kind == kind {
			count += len(rd.Revisions)
		}
	}
	return count
}

func (s *Store) UnwatchedKinds() []resource.Kind {
	s.mu.RLock()
	defer s.mu.RUnlock()

	watched := s.watchedKindsLocked()
	watchedSet := make(map[string]bool, len(watched))
	for _, k := range watched {
		watchedSet[k] = true
	}
	var result []resource.Kind
	for _, rk := range s.clusterResourceTypes {
		if !watchedSet[rk.Kind] {
			result = append(result, rk)
		}
	}
	return result
}

func (s *Store) AddWatchKind(rk resource.Kind) []*resource.Data {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var created []*resource.Data

	dummyResources := generateDummyResourcesForKind(rk, now)
	for _, r := range dummyResources {
		initRev := resource.Revision{
			ID:        s.nextRevisionIDLocked(),
			EventType: resource.EventAdded,
			Time:      now.Add(-time.Duration(len(created)) * 500 * time.Millisecond),
			Object: map[string]any{
				"apiVersion": rk.APIVersion,
				"kind":       rk.Kind,
				"metadata":   r.meta,
				"spec":       r.spec,
				"status":     r.status,
			},
		}

		rd := &resource.Data{
			Resource: resource.Resource{
				UID:       r.uid,
				Kind:      rk.Kind,
				Name:      r.name,
				Namespace: r.namespace,
			},
			Revisions: []resource.Revision{initRev},
		}
		s.resources[r.uid] = rd
		s.totalRevisions++
		created = append(created, rd)

		s.timeline = append(s.timeline, resource.TimelineEntry{
			Resource: rd.Resource,
			Revision: initRev,
		})
	}

	sort.Slice(s.timeline, func(i, j int) bool {
		return s.timeline[i].Revision.Time.After(s.timeline[j].Revision.Time)
	})
	return created
}

func (s *Store) RemoveWatchKind(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var toRemove []string
	for uid, rd := range s.resources {
		if rd.Resource.Kind == kind {
			s.totalRevisions -= len(rd.Revisions)
			toRemove = append(toRemove, uid)
		}
	}
	for _, uid := range toRemove {
		delete(s.resources, uid)
	}

	removeSet := make(map[string]bool, len(toRemove))
	for _, uid := range toRemove {
		removeSet[uid] = true
	}
	filtered := make([]resource.TimelineEntry, 0, len(s.timeline)-len(toRemove))
	for _, e := range s.timeline {
		if !removeSet[e.Resource.UID] {
			filtered = append(filtered, e)
		}
	}
	s.timeline = filtered
}

func (s *Store) ToggleStar(uid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, ok := s.resources[uid]
	if !ok {
		return false
	}
	rd.Resource.Starred = !rd.Resource.Starred
	for i := range s.timeline {
		if s.timeline[i].Resource.UID == uid {
			s.timeline[i].Resource.Starred = rd.Resource.Starred
		}
	}
	return rd.Resource.Starred
}

func (s *Store) AddRevision(resourceUID string, rev resource.Revision) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, ok := s.resources[resourceUID]
	if !ok {
		return
	}
	rd.Revisions = append(rd.Revisions, rev)
	s.totalRevisions++

	// Timeline is sorted newest-first. New simulation revisions are always
	// the most recent, so prepending maintains sort order without re-sorting.
	s.timeline = append(s.timeline, resource.TimelineEntry{})
	copy(s.timeline[1:], s.timeline)
	s.timeline[0] = resource.TimelineEntry{
		Resource: rd.Resource,
		Revision: rev,
	}
}

func (s *Store) RebuildKindGroups() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.kindGroups = resource.BuildKindGroups(s.allResourcesLocked())
}

func (s *Store) ForEachResource(fn func(uid string, rd *resource.Data)) {
	// Snapshot under the lock, then invoke the callback outside it to avoid
	// holding the lock across arbitrary caller work and to prevent RWMutex
	// re-entry deadlocks if fn calls back into the store.
	s.mu.RLock()
	type entry struct {
		uid string
		rd  *resource.Data
	}
	snapshot := make([]entry, 0, len(s.resources))
	for uid, rd := range s.resources {
		snapshot = append(snapshot, entry{uid, rd})
	}
	s.mu.RUnlock()

	for _, e := range snapshot {
		fn(e.uid, e.rd)
	}
}

// allResourcesLocked returns all resources sorted by kind/name.
// Must be called with at least s.mu.RLock held.
func (s *Store) allResourcesLocked() []*resource.Data {
	result := make([]*resource.Data, 0, len(s.resources))
	for _, rd := range s.resources {
		result = append(result, rd)
	}
	resource.SortByKindName(result)
	return result
}

// ScheduleNextTick returns a tea.Cmd that generates a SimulationTickMsg
// for a random resource after a 3-5 second delay. The epoch is echoed back
// in the message so the App can discard ticks from a superseded chain.
func (s *Store) ScheduleNextTick(epoch uint64) tea.Cmd {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.resources) == 0 {
		return nil
	}
	uids := make([]string, 0, len(s.resources))
	for uid := range s.resources {
		uids = append(uids, uid)
	}
	return func() tea.Msg {
		delay := time.Duration(3+rand.Intn(3)) * time.Second
		time.Sleep(delay)
		uid := uids[rand.Intn(len(uids))]
		return tui.SimulationTickMsg{ResourceUID: uid, Epoch: epoch}
	}
}

// GenerateRevision creates a new revision for the given resource. It is tagged
// ADDED for a resource's first revision and MODIFIED thereafter.
func (s *Store) GenerateRevision(rd *resource.Data) resource.Revision {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	latest := rd.LatestRevision()

	var newObj map[string]any
	if latest != nil && latest.Object != nil {
		newObj = resource.CloneMap(latest.Object)
	} else {
		newObj = map[string]any{
			"apiVersion": "v1",
			"kind":       rd.Resource.Kind,
			"metadata":   map[string]any{"name": rd.Resource.Name},
		}
	}

	meta, _ := newObj["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		newObj["metadata"] = meta
	}
	annotations, _ := meta["annotations"].(map[string]any)
	if annotations == nil {
		annotations = map[string]any{}
		meta["annotations"] = annotations
	}
	annotations["loog.dev/last-seen"] = now.Format(time.RFC3339)

	var prevID resource.RevisionID
	eventType := resource.EventModified
	if latest != nil {
		prevID = latest.ID
	} else {
		// No prior revision: this is the first time we see the resource.
		eventType = resource.EventAdded
	}
	newID := s.nextRevisionIDLocked()

	return resource.Revision{
		ID:         newID,
		PreviousID: prevID,
		EventType:  eventType,
		Time:       now,
		Object:     newObj,
		Patch: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					"loog.dev/last-seen": now.Format(time.RFC3339),
				},
			},
		},
	}
}

type dummyResourceSpec struct {
	uid       string
	name      string
	namespace string
	meta      map[string]any
	spec      map[string]any
	status    map[string]any
}

func generateDummyResourcesForKind(rk resource.Kind, now time.Time) []dummyResourceSpec {
	ns := "default"
	if !rk.Namespaced {
		ns = ""
	}
	uid := func(suffix string) string {
		return "watched-" + strings.ToLower(rk.Kind) + "-" + suffix
	}
	meta := func(name, namespace string) map[string]any {
		m := map[string]any{
			"name":              name,
			"creationTimestamp": now.Add(-1 * time.Hour).Format(time.RFC3339),
		}
		if namespace != "" {
			m["namespace"] = namespace
		}
		return m
	}

	switch rk.Kind {
	case "Secret":
		return []dummyResourceSpec{
			{
				uid: uid("tls-cert"), name: "tls-cert", namespace: ns,
				meta: meta("tls-cert", ns), spec: map[string]any{"type": "kubernetes.io/tls"},
				status: map[string]any{},
			},
			{
				uid: uid("db-creds"), name: "db-credentials", namespace: ns,
				meta: meta("db-credentials", ns), spec: map[string]any{"type": "Opaque"},
				status: map[string]any{},
			},
		}
	case "Ingress":
		return []dummyResourceSpec{
			{
				uid: uid("api-gw"), name: "api-gateway", namespace: ns,
				meta: meta("api-gateway", ns),
				spec: map[string]any{
					"rules": []any{
						map[string]any{
							"host": "api.example.com",
							"http": map[string]any{
								"paths": []any{
									map[string]any{
										"path": "/",
										"backend": map[string]any{
											"service": map[string]any{
												"name": "api-svc",
												"port": float64(8080),
											},
										},
									},
								},
							},
						},
					},
				},
				status: map[string]any{"loadBalancer": map[string]any{}},
			},
			{
				uid: uid("web-fe"), name: "web-frontend", namespace: ns,
				meta:   meta("web-frontend", ns),
				spec:   map[string]any{"rules": []any{map[string]any{"host": "www.example.com"}}},
				status: map[string]any{"loadBalancer": map[string]any{}},
			},
		}
	case "CronJob":
		return []dummyResourceSpec{
			{
				uid: uid("db-backup"), name: "db-backup", namespace: ns,
				meta: meta("db-backup", ns),
				spec: map[string]any{
					"schedule": "0 2 * * *",
					"jobTemplate": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "backup",
											"image": "postgres:16",
										},
									},
								},
							},
						},
					},
				},
				status: map[string]any{"lastScheduleTime": now.Add(-2 * time.Hour).Format(time.RFC3339)},
			},
			{
				uid: uid("log-cleanup"), name: "log-cleanup", namespace: "kube-system",
				meta: meta("log-cleanup", "kube-system"),
				spec: map[string]any{
					"schedule": "0 */6 * * *",
					"jobTemplate": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "cleaner",
											"image": "busybox:latest",
										},
									},
								},
							},
						},
					},
				},
				status: map[string]any{},
			},
		}
	case "HPA":
		return []dummyResourceSpec{
			{
				uid: uid("nginx-as"), name: "nginx-autoscaler", namespace: ns,
				meta: meta("nginx-autoscaler", ns),
				spec: map[string]any{
					"scaleTargetRef":                 map[string]any{"kind": "Deployment", "name": "nginx-deployment"},
					"minReplicas":                    float64(1),
					"maxReplicas":                    float64(10),
					"targetCPUUtilizationPercentage": float64(50),
				},
				status: map[string]any{"currentReplicas": float64(3), "desiredReplicas": float64(3)},
			},
		}
	case "PodDisruptionBudget":
		return []dummyResourceSpec{
			{
				uid: uid("nginx-pdb"), name: "nginx-pdb", namespace: ns,
				meta: meta("nginx-pdb", ns),
				spec: map[string]any{
					"minAvailable": float64(1),
					"selector":     map[string]any{"matchLabels": map[string]any{"app": "nginx"}},
				},
				status: map[string]any{"currentHealthy": float64(3), "desiredHealthy": float64(1)},
			},
		}
	case "Namespace":
		return []dummyResourceSpec{
			{
				uid: uid("monitoring"), name: "monitoring", namespace: "",
				meta: meta("monitoring", ""), spec: map[string]any{},
				status: map[string]any{"phase": "Active"},
			},
			{
				uid: uid("staging"), name: "staging", namespace: "",
				meta: meta("staging", ""), spec: map[string]any{},
				status: map[string]any{"phase": "Active"},
			},
		}
	case "ServiceAccount":
		return []dummyResourceSpec{
			{
				uid: uid("deploy-bot"), name: "deploy-bot", namespace: ns,
				meta:   meta("deploy-bot", ns),
				spec:   map[string]any{"automountServiceAccountToken": true},
				status: map[string]any{},
			},
		}
	case "ClusterRole":
		return []dummyResourceSpec{
			{
				uid: uid("custom-admin"), name: "custom-admin", namespace: "",
				meta: meta("custom-admin", ""),
				spec: map[string]any{
					"rules": []any{
						map[string]any{
							"apiGroups": []any{"*"},
							"resources": []any{"*"},
							"verbs":     []any{"*"},
						},
					},
				},
				status: map[string]any{},
			},
		}
	case "Endpoints":
		return []dummyResourceSpec{
			{
				uid: uid("nginx-ep"), name: "nginx-svc", namespace: ns,
				meta: meta("nginx-svc", ns),
				spec: map[string]any{
					"subsets": []any{
						map[string]any{
							"addresses": []any{
								map[string]any{"ip": "10.0.1.15"},
								map[string]any{"ip": "10.0.1.16"},
							}, "ports": []any{map[string]any{"port": float64(80)}},
						},
					},
				},
				status: map[string]any{},
			},
		}
	case "PersistentVolume":
		return []dummyResourceSpec{
			{
				uid: uid("data-pv"), name: "data-pv-01", namespace: "",
				meta: meta("data-pv-01", ""),
				spec: map[string]any{
					"capacity":         map[string]any{"storage": "100Gi"},
					"accessModes":      []any{"ReadWriteOnce"},
					"storageClassName": "standard",
				},
				status: map[string]any{"phase": "Bound"},
			},
		}
	case "PersistentVolumeClaim":
		return []dummyResourceSpec{
			{
				uid: uid("data-pvc"), name: "data-pvc", namespace: ns,
				meta: meta("data-pvc", ns),
				spec: map[string]any{
					"accessModes": []any{"ReadWriteOnce"},
					"resources":   map[string]any{"requests": map[string]any{"storage": "50Gi"}},
				},
				status: map[string]any{"phase": "Bound", "capacity": map[string]any{"storage": "50Gi"}},
			},
		}
	case "NetworkPolicy":
		return []dummyResourceSpec{
			{
				uid: uid("default-deny"), name: "default-deny", namespace: ns,
				meta:   meta("default-deny", ns),
				spec:   map[string]any{"podSelector": map[string]any{}, "policyTypes": []any{"Ingress", "Egress"}},
				status: map[string]any{},
			},
		}
	case "Job":
		return []dummyResourceSpec{
			{
				uid: uid("db-migrate"), name: "db-migrate-v2", namespace: ns,
				meta: meta("db-migrate-v2", ns),
				spec: map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name":  "migrate",
									"image": "myapp/migrate:v2",
								},
							}, "restartPolicy": "Never",
						},
					},
				},
				status: map[string]any{
					"succeeded":      float64(1),
					"completionTime": now.Add(-30 * time.Minute).Format(time.RFC3339),
				},
			},
		}
	default:
		return []dummyResourceSpec{
			{
				uid: uid("instance-1"), name: rk.Kind + "-instance-1", namespace: ns,
				meta:   meta(rk.Kind+"-instance-1", ns),
				spec:   map[string]any{"placeholder": "(auto-generated)"},
				status: map[string]any{"phase": "Active"},
			},
		}
	}
}

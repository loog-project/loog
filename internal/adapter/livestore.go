// Package adapter bridges the production backend (TrackerService, ResourcePatchStore)
// to the new TUI's Store interface. It provides LiveStore (thread-safe, in-memory)
// and TUIRevisionHandler (collector → TUI message bridge).
package adapter

import (
	"sort"
	"strings"
	"sync"

	"github.com/loog-project/loog/internal/resource"
)

// LiveStore implements tui.Store backed by in-memory maps.
// Thread-safe: the collector goroutine writes (IngestRevision), the TUI goroutine reads.
type LiveStore struct {
	mu sync.RWMutex

	// Core data indexed by UID
	resources map[string]*resource.ResourceData

	// Timeline: newest-first
	timeline []resource.TimelineEntry

	// Precomputed kind groups (rebuilt on demand via RebuildKindGroups)
	kindGroups []*resource.KindGroup

	// Watched kinds (e.g., "Pod", "Deployment")
	watchedKinds map[string]bool

	// Unwatched kinds available on the cluster (populated externally)
	unwatchedKinds []resource.ResourceKind

	// Cached totals for fast access
	totalRevisions int
}

// NewLiveStore creates an empty LiveStore.
func NewLiveStore() *LiveStore {
	return &LiveStore{
		resources:    make(map[string]*resource.ResourceData),
		watchedKinds: make(map[string]bool),
	}
}

// ── Ingestion (called from collector goroutine) ──

// IngestRevision adds a revision for a resource. If the resource doesn't exist yet,
// it is created. This is thread-safe and designed for high-throughput ingestion.
func (s *LiveStore) IngestRevision(
	uid, kind, name, namespace string,
	rev resource.Revision,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, exists := s.resources[uid]
	if !exists {
		rd = &resource.ResourceData{
			Resource: resource.Resource{
				UID:       uid,
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
		}
		s.resources[uid] = rd
		s.watchedKinds[kind] = true
	}

	// Append revision (oldest-first order: new revisions go at the end)
	rd.Revisions = append(rd.Revisions, rev)
	s.totalRevisions++

	// Prepend to timeline (newest-first)
	entry := resource.TimelineEntry{
		Resource: rd.Resource,
		Revision: rev,
	}
	s.timeline = append(s.timeline, resource.TimelineEntry{}) // grow
	copy(s.timeline[1:], s.timeline)
	s.timeline[0] = entry
}

// ── Query Methods (tui.Store interface) ──

func (s *LiveStore) AllResources() []*resource.ResourceData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*resource.ResourceData, 0, len(s.resources))
	for _, rd := range s.resources {
		result = append(result, rd)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resource.Kind != result[j].Resource.Kind {
			return result[i].Resource.Kind < result[j].Resource.Kind
		}
		return result[i].Resource.Name < result[j].Resource.Name
	})
	return result
}

func (s *LiveStore) StarredResources() []*resource.ResourceData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*resource.ResourceData
	for _, rd := range s.resources {
		if rd.Resource.Starred {
			result = append(result, rd)
		}
	}
	return result
}

func (s *LiveStore) GetResource(uid string) *resource.ResourceData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resources[uid]
}

func (s *LiveStore) TotalResourceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.resources)
}

func (s *LiveStore) TotalRevisionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalRevisions
}

func (s *LiveStore) FilterResources(expr string) []*resource.ResourceData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if expr == "" {
		return s.allResourcesLocked()
	}

	prog := resource.CompileFilter(expr)
	lower := strings.ToLower(expr)
	var result []*resource.ResourceData
	for _, rd := range s.resources {
		if resource.MatchesFilterOrSubstring(prog, rd.Resource, lower) {
			result = append(result, rd)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resource.Kind != result[j].Resource.Kind {
			return result[i].Resource.Kind < result[j].Resource.Kind
		}
		return result[i].Resource.Name < result[j].Resource.Name
	})
	return result
}

func (s *LiveStore) FilterTimeline(expr string, starredOnly bool) []resource.TimelineEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if expr == "" && !starredOnly {
		// Return a copy
		result := make([]resource.TimelineEntry, len(s.timeline))
		copy(result, s.timeline)
		return result
	}

	prog := resource.CompileFilter(expr)
	lower := strings.ToLower(expr)
	var result []resource.TimelineEntry
	for _, e := range s.timeline {
		if starredOnly {
			rd := s.resources[e.Resource.UID]
			if rd == nil || !rd.Resource.Starred {
				continue
			}
		}
		if expr != "" && !resource.MatchesFilterOrSubstring(prog, e.Resource, lower) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (s *LiveStore) Timeline() []resource.TimelineEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]resource.TimelineEntry, len(s.timeline))
	copy(result, s.timeline)
	return result
}

func (s *LiveStore) KindGroups() []*resource.KindGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return cached kind groups
	return s.kindGroups
}

func (s *LiveStore) WatchedKinds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kinds := make([]string, 0, len(s.watchedKinds))
	for k := range s.watchedKinds {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func (s *LiveStore) ResourceCountByKind(kind string) int {
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

func (s *LiveStore) RevisionCountByKind(kind string) int {
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

func (s *LiveStore) UnwatchedKinds() []resource.ResourceKind {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// In production mode, unwatched kinds could be populated from API discovery.
	// For now, return whatever was set externally.
	return s.unwatchedKinds
}

// ── Mutation Methods (tui.Store interface) ──

func (s *LiveStore) AddWatchKind(rk resource.ResourceKind) []*resource.ResourceData {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.watchedKinds[rk.Kind] = true

	// Remove from unwatched list
	filtered := s.unwatchedKinds[:0]
	for _, uk := range s.unwatchedKinds {
		if uk.Kind != rk.Kind {
			filtered = append(filtered, uk)
		}
	}
	s.unwatchedKinds = filtered

	// In production mode, adding a watch kind triggers mux.Add() which is
	// handled externally by the caller. We don't create synthetic resources here.
	return nil
}

func (s *LiveStore) RemoveWatchKind(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.watchedKinds, kind)

	// Remove all resources of this kind
	for uid, rd := range s.resources {
		if rd.Resource.Kind == kind {
			s.totalRevisions -= len(rd.Revisions)
			delete(s.resources, uid)
		}
	}

	// Remove timeline entries for this kind
	filtered := s.timeline[:0]
	for _, e := range s.timeline {
		if e.Resource.Kind != kind {
			filtered = append(filtered, e)
		}
	}
	s.timeline = filtered
}

func (s *LiveStore) ToggleStar(uid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, ok := s.resources[uid]
	if !ok {
		return false
	}
	rd.Resource.Starred = !rd.Resource.Starred
	return rd.Resource.Starred
}

func (s *LiveStore) AddRevision(resourceUID string, rev resource.Revision) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, ok := s.resources[resourceUID]
	if !ok {
		return
	}

	rd.Revisions = append(rd.Revisions, rev)
	s.totalRevisions++

	entry := resource.TimelineEntry{
		Resource: rd.Resource,
		Revision: rev,
	}
	s.timeline = append(s.timeline, resource.TimelineEntry{})
	copy(s.timeline[1:], s.timeline)
	s.timeline[0] = entry
}

func (s *LiveStore) RebuildKindGroups() {
	s.mu.Lock()
	defer s.mu.Unlock()

	all := s.allResourcesLocked()
	s.kindGroups = resource.BuildKindGroups(all)
}

func (s *LiveStore) ForEachResource(fn func(uid string, rd *resource.ResourceData)) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for uid, rd := range s.resources {
		fn(uid, rd)
	}
}

// ── External Configuration ──

// SetUnwatchedKinds sets the list of unwatched resource kinds available on the cluster.
func (s *LiveStore) SetUnwatchedKinds(kinds []resource.ResourceKind) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unwatchedKinds = kinds
}

// ── Internal helpers ──

func (s *LiveStore) allResourcesLocked() []*resource.ResourceData {
	result := make([]*resource.ResourceData, 0, len(s.resources))
	for _, rd := range s.resources {
		result = append(result, rd)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Resource.Kind != result[j].Resource.Kind {
			return result[i].Resource.Kind < result[j].Resource.Kind
		}
		return result[i].Resource.Name < result[j].Resource.Name
	})
	return result
}

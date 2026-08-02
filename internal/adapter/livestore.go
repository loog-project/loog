// Package adapter bridges the production backend (TrackerService, ResourcePatchStore)
// to the new TUI's Store interface. It provides LiveStore (thread-safe, in-memory)
// and TUIRevisionHandler (collector -> TUI message bridge).
package adapter

import (
	"slices"
	"strings"
	"sync"

	"github.com/loog-project/loog/internal/resource"
)

// LiveStore implements tui.Store backed by in-memory maps.
// Thread-safe: the collector goroutine writes (IngestRevision), the TUI goroutine reads.
type LiveStore struct {
	mu sync.RWMutex

	// Core data indexed by UID
	resources map[string]*resource.Data

	// Timeline: newest-first
	timeline []resource.TimelineEntry

	// Precomputed kind groups (rebuilt on demand via RebuildKindGroups)
	kindGroups []*resource.KindGroup

	// Watched kinds (e.g., "Pod", "Deployment")
	watchedKinds map[string]bool

	// Unwatched kinds available on the cluster (populated externally)
	unwatchedKinds []resource.Kind

	// Cached totals for fast access
	totalRevisions int
}

// NewLiveStore creates an empty LiveStore.
func NewLiveStore() *LiveStore {
	return &LiveStore{
		resources:    make(map[string]*resource.Data),
		watchedKinds: make(map[string]bool),
	}
}

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
		rd = &resource.Data{
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

	// Create a new Data snapshot with the appended revision (copy-on-write).
	newData := s.appendRevision(uid, rd, rev)
	s.totalRevisions++

	// Prepend to timeline (newest-first)
	entry := resource.TimelineEntry{
		Resource: newData.Resource,
		Revision: rev,
	}
	s.timeline = append(s.timeline, resource.TimelineEntry{}) // grow
	copy(s.timeline[1:], s.timeline)
	s.timeline[0] = entry
}

func (s *LiveStore) AllResources() []*resource.Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allResourcesLocked()
}

func (s *LiveStore) StarredResources() []*resource.Data {
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

func (s *LiveStore) GetResource(uid string) *resource.Data {
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

func (s *LiveStore) FilterResources(expr string) []*resource.Data {
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
	resource.SortByKindName(result)
	return result
}

func (s *LiveStore) FilterTimeline(expr string, starredOnly bool) []resource.TimelineEntry {
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
		if starredOnly {
			rd := s.resources[e.Resource.UID]
			if rd == nil || !rd.Resource.Starred {
				continue
			}
		}
		if expr != "" && !resource.MatchesSubstring(lower, e.Resource) {
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

	// Return a shallow copy of the slice so a concurrent RebuildKindGroups
	// (which reassigns s.kindGroups) cannot race with the caller iterating it.
	out := make([]*resource.KindGroup, len(s.kindGroups))
	copy(out, s.kindGroups)
	return out
}

func (s *LiveStore) WatchedKinds() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	kinds := make([]string, 0, len(s.watchedKinds))
	for k := range s.watchedKinds {
		kinds = append(kinds, k)
	}
	slices.Sort(kinds)
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

func (s *LiveStore) UnwatchedKinds() []resource.Kind {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy: callers (e.g. the watch manager) retain this slice across
	// render frames, and AddWatchKind must not mutate the array underneath them.
	out := make([]resource.Kind, len(s.unwatchedKinds))
	copy(out, s.unwatchedKinds)
	return out
}

func (s *LiveStore) AddWatchKind(rk resource.Kind) []*resource.Data {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.watchedKinds[rk.Kind] = true

	// Remove from unwatched list. Allocate a fresh slice instead of reusing the
	// backing array with [:0]; the old array may still be held by the UI.
	filtered := make([]resource.Kind, 0, len(s.unwatchedKinds))
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
	filtered := make([]resource.TimelineEntry, 0, len(s.timeline))
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
	// Create a NEW Data with the toggled Starred flag instead of mutating in-place.
	newResource := rd.Resource
	newResource.Starred = !newResource.Starred
	newData := &resource.Data{
		Resource:  newResource,
		Revisions: rd.Revisions,
	}
	s.resources[uid] = newData
	for i := range s.timeline {
		if s.timeline[i].Resource.UID == uid {
			s.timeline[i].Resource.Starred = newResource.Starred
		}
	}
	return newResource.Starred
}

func (s *LiveStore) AddRevision(resourceUID string, rev resource.Revision) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rd, ok := s.resources[resourceUID]
	if !ok {
		return
	}

	// Create a new Data snapshot with the appended revision (copy-on-write).
	newData := s.appendRevision(resourceUID, rd, rev)
	s.totalRevisions++

	entry := resource.TimelineEntry{
		Resource: newData.Resource,
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

func (s *LiveStore) ForEachResource(fn func(uid string, rd *resource.Data)) {
	// Snapshot under the lock, then invoke the callback outside it. This keeps
	// the write lock off the collector's ingestion path while the callback does
	// potentially slow work (e.g. rendering), and avoids RWMutex re-entry
	// deadlocks if fn calls back into the store.
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

// SetUnwatchedKinds sets the list of unwatched resource kinds available on the cluster.
func (s *LiveStore) SetUnwatchedKinds(kinds []resource.Kind) {
	// Copy so the store owns its backing array independent of the caller.
	owned := make([]resource.Kind, len(kinds))
	copy(owned, kinds)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.unwatchedKinds = owned
}

func (s *LiveStore) allResourcesLocked() []*resource.Data {
	result := make([]*resource.Data, 0, len(s.resources))
	for _, rd := range s.resources {
		result = append(result, rd)
	}
	resource.SortByKindName(result)
	return result
}

// appendRevision creates a new *resource.Data with the revision appended,
// stores it in the map, and returns it. The old pointer remains valid for
// anyone who already holds it (copy-on-write).
func (s *LiveStore) appendRevision(uid string, rd *resource.Data, rev resource.Revision) *resource.Data {
	newRevisions := make([]resource.Revision, len(rd.Revisions)+1)
	copy(newRevisions, rd.Revisions)
	newRevisions[len(rd.Revisions)] = rev
	newData := &resource.Data{
		Resource:  rd.Resource,
		Revisions: newRevisions,
	}
	s.resources[uid] = newData
	return newData
}

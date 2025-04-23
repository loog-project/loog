package service

import (
	"context"
	"fmt"

	"github.com/loog-project/loog/internal/patch"
	"github.com/loog-project/loog/internal/store"
	"github.com/vmihailenco/msgpack/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Service struct {
	rps           store.ResourcePatchStore
	snapshotEvery uint64
}

// New creates a new Service instance with the given store and snapshot interval.
func New(rps store.ResourcePatchStore, snapshotEvery uint64) *Service {
	return &Service{
		rps:           rps,
		snapshotEvery: snapshotEvery,
	}
}

// Get retrieves the object with the given uid and revision from the store.
func (s *Service) Get(ctx context.Context, uid string, revision uint64) (*unstructured.Unstructured, error) {
	snapshotData, snapshotRevision, err := s.rps.LoadSnapshot(ctx, uid, revision)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}
	var snapshot unstructured.Unstructured
	err = msgpack.Unmarshal(snapshotData, &snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	if snapshotRevision == revision {
		// if the snapshot revision is the same as the requested revision, we can return it
		return &snapshot, nil
	}
	// TODO: implement me!
	return nil, fmt.Errorf("not implemented")
}

// Commit commits the given object to the store with the given uid and revision.
func (s *Service) Commit(ctx context.Context, uid string, obj *unstructured.Unstructured) error {
	latestRevision, err := s.rps.LatestRevision(ctx, uid)
	if err != nil {
		return fmt.Errorf("failed to get latest revision: %w", err)
	}
	newRevision := latestRevision + 1
	isSnapshot := newRevision == 1 || newRevision%s.snapshotEvery == 0

	var toEncode any
	if isSnapshot {
		// if it's a snapshot, we can just store the object like it is
		toEncode = obj
	} else {
		// if it's a patch, we need to store the diff
		previous, err := s.Get(ctx, uid, latestRevision)
		if err != nil {
			return fmt.Errorf("failed to get previous revision: %w", err)
		}
		diff, err := patch.Create(previous, obj)
		if err != nil {
			return fmt.Errorf("failed to create patch: %w", err)
		}
		toEncode = diff
	}

	payload, err := msgpack.Marshal(toEncode)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	err = s.rps.Commit(ctx, uid, newRevision, isSnapshot, payload)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

// Close closes the underlying store.
func (s *Service) Close() error {
	return s.rps.Close()
}

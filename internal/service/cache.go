package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/loog-project/loog/internal/store"
	"github.com/loog-project/loog/pkg/diffmap"
)

const (
	cacheSweepEvery   = 10 * time.Second // janitor wake-up
	ttlBase           = 40 * time.Second // cold entry expires after this
	ttlHitBonus       = 4 * time.Second  // each extra read adds this much TTL
	maxTrackedEntries = 100_000          // hard memory cap (~few 100 MB)
)

// trackerState is a cache entry for a tracked object.
type trackerState struct {
	obj      diffmap.DiffMap
	rev      store.RevisionID
	lastRead int64 // unix-nsec; atomic
	hitCount uint32
}

// stateCache is a cache of trackerState objects.
type stateCache struct {
	mu     sync.RWMutex
	data   map[string]*trackerState
	stopCh chan struct{}
}

// newStateCache returns a new state cache with a janitor that evicts cold entries.
func newStateCache() *stateCache {
	c := &stateCache{
		data:   make(map[string]*trackerState, 1024),
		stopCh: make(chan struct{}),
	}
	go c.janitor()
	return c
}

// close stops the janitor and clears the cache.
func (c *stateCache) close() {
	close(c.stopCh)
	c.mu.Lock()
	for _, e := range c.data {
		e.obj = nil
	}
	c.data = nil
	c.mu.Unlock()
}

func (c *stateCache) evictCold() {
	now := time.Now()

	c.mu.Lock()
	for k, e := range c.data {
		age := now.Sub(time.Unix(0, atomic.LoadInt64(&e.lastRead)))
		ttl := ttlBase + time.Duration(atomic.LoadUint32(&e.hitCount))*ttlHitBonus
		if age > ttl {
			delete(c.data, k)
		} else {
			// decay hit counter so “old” popularity fades
			if hc := atomic.LoadUint32(&e.hitCount); hc > 0 {
				atomic.StoreUint32(&e.hitCount, hc/2)
			}
		}
	}
	c.mu.Unlock()
}

// janitor evicts cold entries.  Cheap O(n) scan every 10 s.
func (c *stateCache) janitor() {
	ticker := time.NewTicker(cacheSweepEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.evictCold()
		case <-c.stopCh:
			return
		}
	}
}

// get returns nil on a miss.
func (c *stateCache) get(uid string) *trackerState {
	c.mu.RLock()
	entry := c.data[uid]
	c.mu.RUnlock()

	if entry == nil {
		return nil
	}

	atomic.AddUint32(&entry.hitCount, 1)
	atomic.StoreInt64(&entry.lastRead, time.Now().UnixNano())
	return entry
}

// set overwrites (or creates) the entry.
func (c *stateCache) set(uid string, ts *trackerState) {
	c.mu.Lock()
	if len(c.data) < maxTrackedEntries {
		c.data[uid] = ts
	}
	c.mu.Unlock()
	atomic.StoreInt64(&ts.lastRead, time.Now().UnixNano())
}

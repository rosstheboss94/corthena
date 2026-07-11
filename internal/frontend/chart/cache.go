package chart

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrCacheBudget reports a frame that cannot fit in the configured cache.
	ErrCacheBudget = errors.New("chart cache byte budget exceeded")
	// ErrServiceBusy reports explicit bounded-queue backpressure.
	ErrServiceBusy = errors.New("chart service busy")
	// ErrServiceClosed reports use after service shutdown.
	ErrServiceClosed = errors.New("chart service closed")
)

// Query is the stable chart-frame cache and request key.
type Query struct {
	SeriesKey  string
	MinimumX   float64
	MaximumX   float64
	Resolution int
}

// Validate rejects ambiguous or non-finite cache keys.
func (query Query) Validate() error {
	if query.SeriesKey == "" || !finite(query.MinimumX) || !finite(query.MaximumX) || query.MaximumX <= query.MinimumX || query.Resolution <= 0 {
		return fmt.Errorf("%w: invalid chart query", ErrInvalidData)
	}
	return nil
}

type cacheEntry struct {
	key   Query
	frame Frame
	bytes uint64
	newer *cacheEntry
	older *cacheEntry
}

// CacheStats is a stable byte-accounting snapshot.
type CacheStats struct {
	Budget    uint64
	Used      uint64
	Entries   int
	Hits      uint64
	Misses    uint64
	Evictions uint64
}

// FrameCache is a concurrency-safe, byte-bounded least-recently-used cache.
// It owns deep frame copies and returns deep copies, so eviction can never
// mutate or invalidate a frame already published to the UI thread.
type FrameCache struct {
	mu        sync.Mutex
	budget    uint64
	used      uint64
	entries   map[Query]*cacheEntry
	newest    *cacheEntry
	oldest    *cacheEntry
	hits      uint64
	misses    uint64
	evictions uint64
}

// NewFrameCache constructs an empty cache with an exact byte budget.
func NewFrameCache(budget uint64) (*FrameCache, error) {
	if budget == 0 {
		return nil, fmt.Errorf("%w: budget must be positive", ErrCacheBudget)
	}
	return &FrameCache{budget: budget, entries: make(map[Query]*cacheEntry)}, nil
}

// Get returns an owned immutable copy and promotes the key to most recent.
func (cache *FrameCache) Get(key Query) (Frame, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	entry, found := cache.entries[key]
	if !found {
		cache.misses++
		return Frame{}, false
	}
	cache.hits++
	cache.promote(entry)
	return entry.frame.Clone(), true
}

// Put inserts an owned deep copy, evicting least-recent entries until the
// exact byte bound is satisfied. Oversized values leave the cache unchanged.
func (cache *FrameCache) Put(key Query, frame Frame) error {
	if err := key.Validate(); err != nil {
		return err
	}
	owned := frame.Clone()
	bytes := owned.ByteSize()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if bytes > cache.budget {
		return fmt.Errorf("%w: frame uses %d bytes with budget %d", ErrCacheBudget, bytes, cache.budget)
	}
	if existing, found := cache.entries[key]; found {
		cache.remove(existing, false)
	}
	for cache.used+bytes > cache.budget && cache.oldest != nil {
		cache.remove(cache.oldest, true)
	}
	entry := &cacheEntry{key: key, frame: owned, bytes: bytes}
	cache.entries[key] = entry
	cache.used += bytes
	cache.insertNewest(entry)
	return nil
}

// Stats returns deterministic cache accounting.
func (cache *FrameCache) Stats() CacheStats {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return CacheStats{Budget: cache.budget, Used: cache.used, Entries: len(cache.entries), Hits: cache.hits, Misses: cache.misses, Evictions: cache.evictions}
}

func (cache *FrameCache) promote(entry *cacheEntry) {
	if cache.newest == entry {
		return
	}
	cache.unlink(entry)
	cache.insertNewest(entry)
}

func (cache *FrameCache) insertNewest(entry *cacheEntry) {
	entry.newer = nil
	entry.older = cache.newest
	if cache.newest != nil {
		cache.newest.newer = entry
	}
	cache.newest = entry
	if cache.oldest == nil {
		cache.oldest = entry
	}
}

func (cache *FrameCache) unlink(entry *cacheEntry) {
	if entry.newer != nil {
		entry.newer.older = entry.older
	} else {
		cache.newest = entry.older
	}
	if entry.older != nil {
		entry.older.newer = entry.newer
	} else {
		cache.oldest = entry.newer
	}
	entry.newer = nil
	entry.older = nil
}

func (cache *FrameCache) remove(entry *cacheEntry, eviction bool) {
	cache.unlink(entry)
	delete(cache.entries, entry.key)
	cache.used -= entry.bytes
	if eviction {
		cache.evictions++
	}
}

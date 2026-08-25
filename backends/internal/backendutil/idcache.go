package backendutil

import (
	"sync"
	"time"
)

// IDCache is a TTL cache from string keys to int64 IDs. Backends use it to
// memoize name→ID resolutions (labels, users) so repeated writes do not
// rescan. The zero value is not usable; construct with NewIDCache.
type IDCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]idCacheEntry
}

type idCacheEntry struct {
	id        int64
	expiresAt time.Time
}

// NewIDCache constructs an IDCache with the given TTL (TTL <= 0 never
// expires entries).
func NewIDCache(ttl time.Duration) *IDCache {
	return &IDCache{ttl: ttl, entries: map[string]idCacheEntry{}}
}

// Get returns the cached ID for key when present and fresh.
func (c *IDCache) Get(key string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || (c.ttl > 0 && !time.Now().Before(e.expiresAt)) {
		return 0, false
	}
	return e.id, true
}

// Put stores id under key with a fresh TTL.
func (c *IDCache) Put(key string, id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = time.Now().Add(c.ttl)
	}
	c.entries[key] = idCacheEntry{id: id, expiresAt: expiresAt}
}

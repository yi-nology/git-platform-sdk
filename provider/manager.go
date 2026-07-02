package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type cachedProvider struct {
	provider  Provider
	createdAt time.Time
}

// Stats reports cache hit/miss counters and the current size. Counters are
// atomically incremented and safe to read concurrently.
type Stats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
}

// Manager provides a caching layer over Provider creation.
//
// It automatically detects the platform from clone URLs and reuses existing
// Provider instances within the TTL window. The cache key is derived from
// platform + baseURL + a SHA-256 hash of the token, so different tokens map
// to different entries without leaking the token itself in logs or memory
// dumps.
type Manager struct {
	providers map[string]cachedProvider
	mu        sync.RWMutex
	ttl       time.Duration
	maxSize   int // 0 = unlimited

	// stats counters (atomic)
	hits      atomic.Int64
	misses    atomic.Int64
	evictions atomic.Int64

	// hasher produces the token portion of the cache key. Defaults to a
	// trimmed SHA-256 hash. Override via WithHasher for testing or for
	// integrating with an external secret manager.
	hasher func(token string) string

	// janitor state
	janitorOnce sync.Once
	janitorStop chan struct{}
}

// ManagerOption configures a Manager at construction time.
type ManagerOption func(*Manager)

// WithMaxSize caps the cache at n entries. When the cap is reached, the
// oldest entry is evicted before a new one is inserted.
func WithMaxSize(n int) ManagerOption {
	return func(m *Manager) { m.maxSize = n }
}

// WithHasher overrides the default SHA-256 token hasher. Useful for tests
// that want deterministic, human-readable keys.
func WithHasher(h func(token string) string) ManagerOption {
	return func(m *Manager) { m.hasher = h }
}

// NewManager creates a new Provider Manager with the given TTL.
// A TTL of 0 means providers never expire (until the process exits).
func NewManager(ttl time.Duration, opts ...ManagerOption) *Manager {
	m := &Manager{
		providers: make(map[string]cachedProvider),
		ttl:       ttl,
		hasher:    defaultHasher,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// defaultHasher returns the first 16 hex characters of SHA-256(token). The
// truncation keeps cache keys short while still providing 64 bits of entropy
// (collision probability ~10^-19 for 1000 keys).
func defaultHasher(token string) string {
	return HashToken(token)
}

// HashToken returns the first 16 hex characters of SHA-256(token). It is the
// default token hasher used by Manager and is exported so callers can compute
// cache keys for diagnostics or external caches.
//
// An empty token hashes to an empty string (no anonymous cache entries).
func HashToken(token string) string {
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:16]
}

// GetByURL detects the platform from the clone URL and returns a cached or
// newly created Provider.
func (m *Manager) GetByURL(cloneURL, token string) (Provider, error) {
	result, err := DetectPlatform(cloneURL)
	if err != nil {
		return nil, fmt.Errorf("detect platform: %w", err)
	}
	return m.Get(Config{
		Platform: result.Platform,
		BaseURL:  result.BaseURL,
		Token:    token,
	})
}

// Get returns a cached or newly created Provider for the given config. The
// cache key is platform + baseURL + hash(token), so the same (platform,
// baseURL) with a different token gets a distinct entry.
func (m *Manager) Get(cfg Config) (Provider, error) {
	key := m.buildKey(cfg)

	m.mu.RLock()
	if cp, ok := m.providers[key]; ok && (m.ttl == 0 || time.Since(cp.createdAt) < m.ttl) {
		m.mu.RUnlock()
		m.hits.Add(1)
		return cp.provider, nil
	}
	m.mu.RUnlock()

	m.misses.Add(1)
	p, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m.mu.Lock()
	// Check again under write lock in case another goroutine raced ahead.
	if cp, ok := m.providers[key]; ok && (m.ttl == 0 || time.Since(cp.createdAt) < m.ttl) {
		m.mu.Unlock()
		return cp.provider, nil
	}
	// Enforce max size via simple LRU-ish eviction (oldest first).
	if m.maxSize > 0 && len(m.providers) >= m.maxSize {
		m.evictOldestLocked()
	}
	m.providers[key] = cachedProvider{provider: p, createdAt: now}
	m.mu.Unlock()

	return p, nil
}

// evictOldestLocked removes the entry with the smallest createdAt. Caller
// must hold m.mu in write mode.
func (m *Manager) evictOldestLocked() {
	var oldestKey string
	var oldestTime time.Time
	for k, cp := range m.providers {
		if oldestKey == "" || cp.createdAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = cp.createdAt
		}
	}
	if oldestKey != "" {
		delete(m.providers, oldestKey)
		m.evictions.Add(1)
	}
}

// Remove removes a cached Provider by config.
func (m *Manager) Remove(cfg Config) {
	key := m.buildKey(cfg)
	m.mu.Lock()
	delete(m.providers, key)
	m.mu.Unlock()
}

// Purge removes all cached Providers.
func (m *Manager) Purge() {
	m.mu.Lock()
	m.providers = make(map[string]cachedProvider)
	m.mu.Unlock()
}

// Len returns the number of cached Providers.
func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.providers)
}

// Stats returns a snapshot of cache counters. The counters continue to
// accumulate across calls; reset them with ResetStats.
func (m *Manager) Stats() Stats {
	m.mu.RLock()
	size := len(m.providers)
	m.mu.RUnlock()
	return Stats{
		Hits:      m.hits.Load(),
		Misses:    m.misses.Load(),
		Evictions: m.evictions.Load(),
		Size:      size,
	}
}

// ResetStats zeroes the hit/miss/eviction counters. Cache entries are not
// affected.
func (m *Manager) ResetStats() {
	m.hits.Store(0)
	m.misses.Store(0)
	m.evictions.Store(0)
}

// Cleanup removes expired entries from the cache. Safe to call manually; also
// invoked periodically by StartJanitor.
func (m *Manager) Cleanup() {
	if m.ttl == 0 {
		return
	}
	m.mu.Lock()
	cutoff := time.Now().Add(-m.ttl)
	for k, cp := range m.providers {
		if cp.createdAt.Before(cutoff) {
			delete(m.providers, k)
			m.evictions.Add(1)
		}
	}
	m.mu.Unlock()
}

// StartJanitor launches a background goroutine that calls Cleanup every
// interval until ctx is cancelled or Stop is called. Calling StartJanitor
// more than once is a no-op (the first call wins).
//
// This is useful for long-running services that want stale providers to be
// reclaimed without explicit Cleanup calls. Short-lived CLIs should skip it.
func (m *Manager) StartJanitor(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	m.janitorOnce.Do(func() {
		m.janitorStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-m.janitorStop:
					return
				case <-ticker.C:
					m.Cleanup()
				}
			}
		}()
	})
}

// Stop halts the background janitor started by StartJanitor. It is safe to
// call when no janitor is running. After Stop, StartJanitor can be called
// again to launch a fresh janitor.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.janitorStop != nil {
		close(m.janitorStop)
		m.janitorStop = nil
		m.janitorOnce = sync.Once{}
	}
}

// buildKey derives a stable cache key from the config. The token is passed
// through the configured hasher (SHA-256 by default) so the raw token never
// appears in the key. This avoids accidental leakage via logs, error
// messages, or heap dumps.
func (m *Manager) buildKey(cfg Config) string {
	return fmt.Sprintf("%s:%s:%s", cfg.Platform, cfg.BaseURL, m.hasher(cfg.Token))
}

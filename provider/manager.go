package provider

import (
	"fmt"
	"sync"
	"time"
)

type cachedProvider struct {
	provider  Provider
	createdAt time.Time
}

// Manager provides a caching layer over Provider creation.
// It automatically detects the platform from clone URLs and reuses
// existing Provider instances within the TTL window.
type Manager struct {
	providers map[string]cachedProvider
	mu        sync.RWMutex
	ttl       time.Duration
}

// NewManager creates a new Provider Manager with the given TTL.
// A TTL of 0 means providers never expire (until the process exits).
func NewManager(ttl time.Duration) *Manager {
	return &Manager{
		providers: make(map[string]cachedProvider),
		ttl:       ttl,
	}
}

// GetByURL detects the platform from the clone URL and returns a cached
// or newly created Provider. The cache key is derived from the platform
// and owner/repo combination.
func (m *Manager) GetByURL(cloneURL, token string) (Provider, error) {
	result, err := DetectPlatform(cloneURL)
	if err != nil {
		return nil, fmt.Errorf("detect platform: %w", err)
	}

	cfg := Config{
		Platform: result.Platform,
		BaseURL:  result.BaseURL,
		Token:    token,
	}

	return m.Get(cfg)
}

// Get returns a cached or newly created Provider for the given config.
// The cache key is derived from platform + baseURL + token prefix.
func (m *Manager) Get(cfg Config) (Provider, error) {
	key := m.buildKey(cfg)

	m.mu.RLock()
	if cp, ok := m.providers[key]; ok {
		if m.ttl == 0 || time.Since(cp.createdAt) < m.ttl {
			m.mu.RUnlock()
			return cp.provider, nil
		}
	}
	m.mu.RUnlock()

	p, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.providers[key] = cachedProvider{provider: p, createdAt: time.Now()}
	m.mu.Unlock()

	return p, nil
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

// Cleanup removes expired entries from the cache.
func (m *Manager) Cleanup() {
	if m.ttl == 0 {
		return
	}
	m.mu.Lock()
	for k, cp := range m.providers {
		if time.Since(cp.createdAt) >= m.ttl {
			delete(m.providers, k)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) buildKey(cfg Config) string {
	tokenPrefix := ""
	if len(cfg.Token) >= 8 {
		tokenPrefix = cfg.Token[:8]
	} else if cfg.Token != "" {
		tokenPrefix = cfg.Token
	}
	return fmt.Sprintf("%s:%s:%s", cfg.Platform, cfg.BaseURL, tokenPrefix)
}

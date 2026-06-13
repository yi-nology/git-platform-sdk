package provider

import (
	"fmt"
	"sync"
)

// ProviderConstructor is a function that creates a Provider from a Config.
type ProviderConstructor func(cfg Config) (Provider, error)

var (
	registry   = map[Platform]ProviderConstructor{}
	registryMu sync.RWMutex
)

// Register registers a provider constructor for a platform.
// This is typically called from init() functions in platform implementation files.
func Register(p Platform, ctor ProviderConstructor) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[p]; exists {
		panic(fmt.Sprintf("provider: platform %q already registered", p))
	}
	registry[p] = ctor
}

// MustRegister is like Register but panics if the platform is already registered.
func MustRegister(p Platform, ctor ProviderConstructor) {
	Register(p, ctor)
}

// RegisteredPlatforms returns a list of all registered platforms.
func RegisteredPlatforms() []Platform {
	registryMu.RLock()
	defer registryMu.RUnlock()
	platforms := make([]Platform, 0, len(registry))
	for p := range registry {
		platforms = append(platforms, p)
	}
	return platforms
}

// IsRegistered checks if a platform has been registered.
func IsRegistered(p Platform) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[p]
	return ok
}

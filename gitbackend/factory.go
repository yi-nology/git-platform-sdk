package gitbackend

import (
	"fmt"
	"sync"
)

// GitBackendConstructor creates a GitBackend instance.
type GitBackendConstructor func(opts Options) (GitBackend, error)

// Options holds configuration for creating a GitBackend.
type Options struct {
	Type   string // "native", "gogit", or "" for auto-detect
	Logger Logger
}

var (
	backendRegistry   = map[string]GitBackendConstructor{}
	backendRegistryMu sync.RWMutex
)

// Register registers a GitBackend constructor.
func Register(name string, ctor GitBackendConstructor) {
	backendRegistryMu.Lock()
	defer backendRegistryMu.Unlock()
	if _, exists := backendRegistry[name]; exists {
		panic(fmt.Sprintf("gitbackend: %q already registered", name))
	}
	backendRegistry[name] = ctor
}

// NewGitBackend creates a GitBackend using the registry.
// If opts.Type is empty, it auto-detects (native first, fallback to gogit).
func NewGitBackend(opts Options) (GitBackend, error) {
	backendRegistryMu.RLock()
	defer backendRegistryMu.RUnlock()

	if opts.Type != "" {
		ctor, ok := backendRegistry[opts.Type]
		if !ok {
			return nil, fmt.Errorf("gitbackend: unknown type %q", opts.Type)
		}
		return ctor(opts)
	}

	// Auto-detect: try native first, fallback to gogit
	if ctor, ok := backendRegistry["native"]; ok {
		backend, err := ctor(opts)
		if err == nil {
			return backend, nil
		}
	}
	if ctor, ok := backendRegistry["gogit"]; ok {
		return ctor(opts)
	}
	return nil, fmt.Errorf("gitbackend: no backends available")
}

func init() {
	Register("native", func(opts Options) (GitBackend, error) {
		return NewNativeGitBackend(opts)
	})
	Register("gogit", func(opts Options) (GitBackend, error) {
		return NewGoGitBackend(opts), nil
	})
}

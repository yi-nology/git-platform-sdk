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

	nativeCtor, hasNative := backendRegistry["native"]
	gogitCtor, hasGogit := backendRegistry["gogit"]
	return newAutoBackend(opts, nativeCtor, hasNative, gogitCtor, hasGogit)
}

// newAutoBackend implements the auto-detect fallback: the native backend when
// its constructor succeeds, the feature-reduced gogit backend otherwise. The
// degradation is logged at Warn level so a missing git binary never silently
// shrinks the backend's capability set.
func newAutoBackend(opts Options, nativeCtor GitBackendConstructor, hasNative bool, gogitCtor GitBackendConstructor, hasGogit bool) (GitBackend, error) {
	if hasNative {
		backend, err := nativeCtor(opts)
		if err == nil {
			return backend, nil
		}
		if opts.Logger != nil {
			opts.Logger.Warn("gitbackend: native backend unavailable, falling back to the feature-reduced gogit backend", "error", err)
		}
	}
	if hasGogit {
		return gogitCtor(opts)
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

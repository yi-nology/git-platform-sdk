package provider

import "fmt"

// Config holds the configuration for creating a Provider.
type Config struct {
	Platform Platform
	BaseURL  string
	Token    string
	SkipTLS  bool

	// Logger for provider operations. Defaults to a no-op logger.
	Logger Logger
	// RetryConfig for automatic retry on transient failures. nil means no retry.
	RetryConfig *RetryConfig
	// Hooks for request/response lifecycle interception.
	Hooks *Hooks
}

// NewProvider creates a Provider for the given platform using the registry.
// Returns ErrPlatformNotSupported if the platform is not registered.
//
// Platform backends are registered via init() functions. Import
// "github.com/yi-nology/git-platform-sdk/backends/all" with a blank
// identifier to register every platform shipped with the SDK.
func NewProvider(cfg Config) (Provider, error) {
	registryMu.RLock()
	ctor, ok := registry[cfg.Platform]
	registryMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPlatformNotSupported, cfg.Platform)
	}
	return ctor(cfg)
}

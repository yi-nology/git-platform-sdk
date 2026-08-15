package gitbackend

import "github.com/yi-nology/git-platform-sdk/provider"

// Logger reuses the provider.Logger interface.
// Consumers can inject the same logger for both provider and gitbackend.
type Logger = provider.Logger

// NewNoopLogger returns a Logger that discards all output.
func NewNoopLogger() Logger {
	return provider.NewNoopLogger()
}

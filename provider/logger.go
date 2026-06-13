package provider

// Logger is a minimal logging interface compatible with most Go logging libraries.
// Implementations include slog, zap, zerolog, logrus, etc.
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// noopLogger is a Logger that discards all output.
type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

// NewNoopLogger returns a Logger that discards all output.
func NewNoopLogger() Logger { return noopLogger{} }

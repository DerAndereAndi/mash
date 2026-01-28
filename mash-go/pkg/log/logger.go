package log

// Logger is the interface applications implement to receive protocol log events.
// Pass nil or NoopLogger to disable logging.
type Logger interface {
	// Log records a protocol event. Implementations must be thread-safe.
	// The event should be processed quickly or queued; blocking affects performance.
	Log(event Event)
}

// NoopLogger discards all events. Use when logging is disabled.
// NoopLogger is safe for concurrent use and usable as a zero value.
type NoopLogger struct{}

// Log discards the event.
func (NoopLogger) Log(Event) {}

// Compile-time interface satisfaction check.
var _ Logger = NoopLogger{}

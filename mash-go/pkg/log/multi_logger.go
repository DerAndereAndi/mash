package log

// MultiLogger sends events to multiple loggers.
// Useful when you want both console output (via SlogAdapter)
// and file output (via FileLogger) simultaneously.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a MultiLogger that sends events to all provided loggers.
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Log sends the event to all configured loggers.
func (m *MultiLogger) Log(event Event) {
	for _, l := range m.loggers {
		l.Log(event)
	}
}

// Compile-time interface satisfaction check.
var _ Logger = (*MultiLogger)(nil)

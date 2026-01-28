package log

import (
	"os"
	"sync"

	"github.com/fxamacker/cbor/v2"
)

// FileLogger writes protocol events to a file in CBOR format.
// It is safe for concurrent use from multiple goroutines.
type FileLogger struct {
	file    *os.File
	encoder *cbor.Encoder
	mu      sync.Mutex
	closed  bool
}

// NewFileLogger creates a new FileLogger that writes to the specified path.
// If the file exists, new events are appended. The file is created with
// permissions 0644 if it doesn't exist.
func NewFileLogger(path string) (*FileLogger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileLogger{
		file:    f,
		encoder: NewEncoder(f),
	}, nil
}

// Log writes an event to the log file.
// This method is safe for concurrent use.
func (l *FileLogger) Log(event Event) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return
	}

	// Ignore encoding errors - logging should not disrupt the application
	_ = l.encoder.Encode(event)
}

// Close closes the log file.
// It is safe to call Close multiple times.
// After Close is called, subsequent Log calls are silently ignored.
func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	return l.file.Close()
}

// Compile-time interface satisfaction check.
var _ Logger = (*FileLogger)(nil)

package log

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestFileLoggerCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}
	defer logger.Close()

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("log file was not created")
	}
}

func TestFileLoggerWritesCBOR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}

	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
		Frame: &FrameEvent{
			Size: 100,
			Data: []byte{1, 2, 3},
		},
	}

	logger.Log(event)
	logger.Close()

	// Read the file and decode
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("log file is empty")
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("failed to decode event: %v", err)
	}

	if decoded.ConnectionID != event.ConnectionID {
		t.Errorf("ConnectionID: got %q, want %q", decoded.ConnectionID, event.ConnectionID)
	}
	if decoded.Frame == nil {
		t.Error("Frame is nil")
	} else if decoded.Frame.Size != event.Frame.Size {
		t.Errorf("Frame.Size: got %d, want %d", decoded.Frame.Size, event.Frame.Size)
	}
}

func TestFileLoggerAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	// Write first event
	logger1, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}

	logger1.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-1",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	})
	logger1.Close()

	// Get file size after first write
	info1, _ := os.Stat(path)
	size1 := info1.Size()

	// Open again and write second event
	logger2, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger second open failed: %v", err)
	}

	logger2.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-2",
		Direction:    DirectionOut,
		Layer:        LayerWire,
		Category:     CategoryMessage,
	})
	logger2.Close()

	// File should be larger
	info2, _ := os.Stat(path)
	size2 := info2.Size()

	if size2 <= size1 {
		t.Errorf("file did not grow: size before=%d, size after=%d", size1, size2)
	}

	// Read all events back
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Decode both events using streaming decoder
	decoder := NewDecoder(bytesReader(data))
	var events []Event
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			break
		}
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].ConnectionID != "conn-1" {
		t.Errorf("first event ConnectionID: got %q, want %q", events[0].ConnectionID, "conn-1")
	}
	if events[1].ConnectionID != "conn-2" {
		t.Errorf("second event ConnectionID: got %q, want %q", events[1].ConnectionID, "conn-2")
	}
}

func TestFileLoggerThreadSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}
	defer logger.Close()

	const numGoroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				logger.Log(Event{
					Timestamp:    time.Now(),
					ConnectionID: "conn-" + string(rune('A'+id)),
					Direction:    DirectionIn,
					Layer:        LayerTransport,
					Category:     CategoryMessage,
				})
			}
		}(i)
	}

	wg.Wait()
	logger.Close()

	// Count events in file
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	decoder := NewDecoder(bytesReader(data))
	count := 0
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			break
		}
		count++
	}

	expectedCount := numGoroutines * eventsPerGoroutine
	if count != expectedCount {
		t.Errorf("event count: got %d, want %d", count, expectedCount)
	}
}

func TestFileLoggerClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.mlog")

	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger failed: %v", err)
	}

	// Write an event
	logger.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	})

	// Close should not error
	if err := logger.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Double close should not panic or error
	if err := logger.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}

	// Logging after close should not panic
	logger.Log(Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-456",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	})
}

func TestFileLoggerInterfaceSatisfaction(t *testing.T) {
	// Compile-time check that FileLogger satisfies Logger interface
	var _ Logger = (*FileLogger)(nil)
}

// bytesReader wraps a byte slice as an io.Reader
type bytesReaderT struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) *bytesReaderT {
	return &bytesReaderT{data: data}
}

func (r *bytesReaderT) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, os.ErrClosed
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

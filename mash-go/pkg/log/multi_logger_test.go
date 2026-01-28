package log

import (
	"testing"
	"time"
)

// mockLogger records events for testing
type mockLogger struct {
	events []Event
}

func (m *mockLogger) Log(event Event) {
	m.events = append(m.events, event)
}

func TestMultiLoggerCallsAll(t *testing.T) {
	mock1 := &mockLogger{}
	mock2 := &mockLogger{}
	mock3 := &mockLogger{}

	multi := NewMultiLogger(mock1, mock2, mock3)

	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	}

	multi.Log(event)

	// All loggers should have received the event
	for i, mock := range []*mockLogger{mock1, mock2, mock3} {
		if len(mock.events) != 1 {
			t.Errorf("logger %d: got %d events, want 1", i, len(mock.events))
			continue
		}
		if mock.events[0].ConnectionID != "conn-123" {
			t.Errorf("logger %d: ConnectionID = %q, want %q", i, mock.events[0].ConnectionID, "conn-123")
		}
	}
}

func TestMultiLoggerEmptyList(t *testing.T) {
	multi := NewMultiLogger()

	// Should not panic with empty logger list
	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	}

	multi.Log(event)
}

func TestMultiLoggerSingleLogger(t *testing.T) {
	mock := &mockLogger{}
	multi := NewMultiLogger(mock)

	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-456",
		Direction:    DirectionOut,
		Layer:        LayerWire,
		Category:     CategoryMessage,
	}

	multi.Log(event)

	if len(mock.events) != 1 {
		t.Fatalf("got %d events, want 1", len(mock.events))
	}
	if mock.events[0].ConnectionID != "conn-456" {
		t.Errorf("ConnectionID = %q, want %q", mock.events[0].ConnectionID, "conn-456")
	}
}

func TestMultiLoggerInterfaceSatisfaction(t *testing.T) {
	var _ Logger = (*MultiLogger)(nil)
}

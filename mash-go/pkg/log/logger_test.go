package log

import (
	"testing"
	"time"
)

func TestNoopLoggerDoesNotPanic(t *testing.T) {
	logger := NoopLogger{}

	// Should not panic with any event type
	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "test-conn",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	}

	// Test with nil payloads
	logger.Log(event)

	// Test with frame payload
	event.Frame = &FrameEvent{Size: 100, Data: []byte{1, 2, 3}}
	logger.Log(event)

	// Test with message payload
	event.Frame = nil
	event.Message = &MessageEvent{Type: MessageTypeRequest, MessageID: 1}
	logger.Log(event)

	// Test with state change payload
	event.Message = nil
	event.StateChange = &StateChangeEvent{Entity: StateEntityConnection, NewState: "connected"}
	logger.Log(event)

	// Test with control message payload
	event.StateChange = nil
	event.ControlMsg = &ControlMsgEvent{Type: ControlMsgPing}
	logger.Log(event)

	// Test with error payload
	event.ControlMsg = nil
	event.Error = &ErrorEventData{Message: "test error"}
	logger.Log(event)
}

func TestLoggerInterfaceSatisfaction(t *testing.T) {
	// Compile-time check that NoopLogger satisfies Logger interface
	var _ Logger = NoopLogger{}
	var _ Logger = &NoopLogger{}
}

func TestNoopLoggerIsZeroValue(t *testing.T) {
	// NoopLogger should be usable as zero value
	var logger NoopLogger
	logger.Log(Event{})
}

package log

import (
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

func TestEventCBORRoundTrip(t *testing.T) {
	ts := time.Date(2026, 1, 28, 10, 15, 32, 123456789, time.UTC)
	original := Event{
		Timestamp:    ts,
		ConnectionID: "abc12345-def6-7890-abcd-ef1234567890",
		Direction:    DirectionOut,
		Layer:        LayerWire,
		Category:     CategoryMessage,
		LocalRole:    RoleDevice,
		RemoteAddr:   "192.168.1.100:8443",
		DeviceID:     "device-001",
		ZoneID:       "zone-local",
	}

	data, err := EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}

	// Compare fields
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if decoded.ConnectionID != original.ConnectionID {
		t.Errorf("ConnectionID: got %q, want %q", decoded.ConnectionID, original.ConnectionID)
	}
	if decoded.Direction != original.Direction {
		t.Errorf("Direction: got %v, want %v", decoded.Direction, original.Direction)
	}
	if decoded.Layer != original.Layer {
		t.Errorf("Layer: got %v, want %v", decoded.Layer, original.Layer)
	}
	if decoded.Category != original.Category {
		t.Errorf("Category: got %v, want %v", decoded.Category, original.Category)
	}
	if decoded.LocalRole != original.LocalRole {
		t.Errorf("LocalRole: got %v, want %v", decoded.LocalRole, original.LocalRole)
	}
	if decoded.RemoteAddr != original.RemoteAddr {
		t.Errorf("RemoteAddr: got %q, want %q", decoded.RemoteAddr, original.RemoteAddr)
	}
	if decoded.DeviceID != original.DeviceID {
		t.Errorf("DeviceID: got %q, want %q", decoded.DeviceID, original.DeviceID)
	}
	if decoded.ZoneID != original.ZoneID {
		t.Errorf("ZoneID: got %q, want %q", decoded.ZoneID, original.ZoneID)
	}
}

func TestFrameEventCBORRoundTrip(t *testing.T) {
	original := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
		Frame: &FrameEvent{
			Size:      256,
			Data:      []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			Truncated: true,
		},
	}

	data, err := EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}

	if decoded.Frame == nil {
		t.Fatal("Frame is nil")
	}
	if decoded.Frame.Size != original.Frame.Size {
		t.Errorf("Frame.Size: got %d, want %d", decoded.Frame.Size, original.Frame.Size)
	}
	if string(decoded.Frame.Data) != string(original.Frame.Data) {
		t.Errorf("Frame.Data: got %v, want %v", decoded.Frame.Data, original.Frame.Data)
	}
	if decoded.Frame.Truncated != original.Frame.Truncated {
		t.Errorf("Frame.Truncated: got %v, want %v", decoded.Frame.Truncated, original.Frame.Truncated)
	}
}

func TestMessageEventCBORRoundTrip(t *testing.T) {
	op := wire.OpRead
	endpoint := uint8(1)
	feature := uint8(3)
	status := wire.StatusSuccess
	subID := uint32(42)
	processingTime := 2 * time.Millisecond

	tests := []struct {
		name string
		msg  *MessageEvent
	}{
		{
			name: "request",
			msg: &MessageEvent{
				Type:       MessageTypeRequest,
				MessageID:  100,
				Operation:  &op,
				EndpointID: &endpoint,
				FeatureID:  &feature,
				Payload:    map[string]any{"attrs": []any{1, 2, 5}},
			},
		},
		{
			name: "response",
			msg: &MessageEvent{
				Type:           MessageTypeResponse,
				MessageID:      100,
				Status:         &status,
				Payload:        map[string]any{"value": 42},
				ProcessingTime: &processingTime,
			},
		},
		{
			name: "notification",
			msg: &MessageEvent{
				Type:           MessageTypeNotification,
				MessageID:      0,
				SubscriptionID: &subID,
				Payload:        map[string]any{"changed": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := Event{
				Timestamp:    time.Now(),
				ConnectionID: "conn-123",
				Direction:    DirectionOut,
				Layer:        LayerWire,
				Category:     CategoryMessage,
				Message:      tt.msg,
			}

			data, err := EncodeEvent(original)
			if err != nil {
				t.Fatalf("EncodeEvent failed: %v", err)
			}

			decoded, err := DecodeEvent(data)
			if err != nil {
				t.Fatalf("DecodeEvent failed: %v", err)
			}

			if decoded.Message == nil {
				t.Fatal("Message is nil")
			}
			if decoded.Message.Type != tt.msg.Type {
				t.Errorf("Message.Type: got %v, want %v", decoded.Message.Type, tt.msg.Type)
			}
			if decoded.Message.MessageID != tt.msg.MessageID {
				t.Errorf("Message.MessageID: got %d, want %d", decoded.Message.MessageID, tt.msg.MessageID)
			}
		})
	}
}

func TestStateChangeEventCBORRoundTrip(t *testing.T) {
	original := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerService,
		Category:     CategoryState,
		StateChange: &StateChangeEvent{
			Entity:   StateEntityConnection,
			OldState: "connecting",
			NewState: "connected",
			Reason:   "TLS handshake complete",
		},
	}

	data, err := EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}

	if decoded.StateChange == nil {
		t.Fatal("StateChange is nil")
	}
	if decoded.StateChange.Entity != original.StateChange.Entity {
		t.Errorf("StateChange.Entity: got %v, want %v", decoded.StateChange.Entity, original.StateChange.Entity)
	}
	if decoded.StateChange.OldState != original.StateChange.OldState {
		t.Errorf("StateChange.OldState: got %q, want %q", decoded.StateChange.OldState, original.StateChange.OldState)
	}
	if decoded.StateChange.NewState != original.StateChange.NewState {
		t.Errorf("StateChange.NewState: got %q, want %q", decoded.StateChange.NewState, original.StateChange.NewState)
	}
	if decoded.StateChange.Reason != original.StateChange.Reason {
		t.Errorf("StateChange.Reason: got %q, want %q", decoded.StateChange.Reason, original.StateChange.Reason)
	}
}

func TestControlMsgEventCBORRoundTrip(t *testing.T) {
	closeReason := uint8(0)

	tests := []struct {
		name string
		ctrl *ControlMsgEvent
	}{
		{
			name: "ping",
			ctrl: &ControlMsgEvent{Type: ControlMsgPing},
		},
		{
			name: "pong",
			ctrl: &ControlMsgEvent{Type: ControlMsgPong},
		},
		{
			name: "close",
			ctrl: &ControlMsgEvent{Type: ControlMsgClose, CloseReason: &closeReason},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := Event{
				Timestamp:    time.Now(),
				ConnectionID: "conn-123",
				Direction:    DirectionIn,
				Layer:        LayerTransport,
				Category:     CategoryControl,
				ControlMsg:   tt.ctrl,
			}

			data, err := EncodeEvent(original)
			if err != nil {
				t.Fatalf("EncodeEvent failed: %v", err)
			}

			decoded, err := DecodeEvent(data)
			if err != nil {
				t.Fatalf("DecodeEvent failed: %v", err)
			}

			if decoded.ControlMsg == nil {
				t.Fatal("ControlMsg is nil")
			}
			if decoded.ControlMsg.Type != tt.ctrl.Type {
				t.Errorf("ControlMsg.Type: got %v, want %v", decoded.ControlMsg.Type, tt.ctrl.Type)
			}
		})
	}
}

func TestErrorEventCBORRoundTrip(t *testing.T) {
	code := 42

	original := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerWire,
		Category:     CategoryError,
		Error: &ErrorEventData{
			Layer:   LayerWire,
			Message: "failed to decode message",
			Code:    &code,
			Context: "HandleRequest",
		},
	}

	data, err := EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	decoded, err := DecodeEvent(data)
	if err != nil {
		t.Fatalf("DecodeEvent failed: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("Error is nil")
	}
	if decoded.Error.Layer != original.Error.Layer {
		t.Errorf("Error.Layer: got %v, want %v", decoded.Error.Layer, original.Error.Layer)
	}
	if decoded.Error.Message != original.Error.Message {
		t.Errorf("Error.Message: got %q, want %q", decoded.Error.Message, original.Error.Message)
	}
	if decoded.Error.Code == nil || *decoded.Error.Code != *original.Error.Code {
		t.Errorf("Error.Code: got %v, want %v", decoded.Error.Code, original.Error.Code)
	}
	if decoded.Error.Context != original.Error.Context {
		t.Errorf("Error.Context: got %q, want %q", decoded.Error.Context, original.Error.Context)
	}
}

func TestEventCBORUsesIntegerKeys(t *testing.T) {
	event := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-123",
		Direction:    DirectionIn,
		Layer:        LayerTransport,
		Category:     CategoryMessage,
	}

	data, err := EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	// Decode to generic map and verify keys are integers
	var rawMap map[uint64]any
	if err := logDecMode.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("failed to decode as map: %v", err)
	}

	// Should have integer keys 1, 2, 3, 4, 5 etc.
	expectedKeys := []uint64{1, 2, 3, 4, 5}
	for _, key := range expectedKeys {
		if _, ok := rawMap[key]; !ok {
			t.Errorf("expected integer key %d not found in encoded data", key)
		}
	}

	// Verify no string keys
	var stringMap map[string]any
	if err := logDecMode.Unmarshal(data, &stringMap); err == nil && len(stringMap) > 0 {
		t.Error("encoded data contains string keys, expected integer keys only")
	}
}

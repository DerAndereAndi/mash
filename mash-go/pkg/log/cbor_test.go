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

func TestCategorySnapshotString(t *testing.T) {
	if got := CategorySnapshot.String(); got != "SNAPSHOT" {
		t.Errorf("CategorySnapshot.String() = %q, want %q", got, "SNAPSHOT")
	}
}

func TestSnapshotEventCBORRoundTrip(t *testing.T) {
	original := Event{
		Timestamp:    time.Date(2026, 2, 2, 14, 30, 0, 0, time.UTC),
		ConnectionID: "conn-snap-001",
		Direction:    DirectionOut,
		Layer:        LayerService,
		Category:     CategorySnapshot,
		LocalRole:    RoleDevice,
		DeviceID:     "device-001",
		ZoneID:       "zone-local",
		Snapshot: &CapabilitySnapshotEvent{
			Local: &DeviceSnapshot{
				DeviceID:    "device-001",
				SpecVersion: "1.0",
				Endpoints: []EndpointSnapshot{
					{
						ID:   0,
						Type: 0x00, // DEVICE_ROOT
						Features: []FeatureSnapshot{
							{
								ID:            1, // DeviceInfo
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 3, 10, 12, 20, 21},
							},
						},
					},
					{
						ID:    1,
						Type:  0x05, // EV_CHARGER
						Label: "Wallbox",
						Features: []FeatureSnapshot{
							{
								ID:            3, // Electrical
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 3, 4},
							},
							{
								ID:            4, // Measurement
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 3},
								CommandList:   []uint8{1},
							},
						},
					},
				},
				UseCases: []UseCaseSnapshot{
					{
						EndpointID: 1,
						ID:         100,
						Major:      1,
						Minor:      0,
						Scenarios:  0x07,
					},
				},
			},
			Remote: &DeviceSnapshot{
				DeviceID:    "controller-001",
				SpecVersion: "1.0",
				Endpoints: []EndpointSnapshot{
					{
						ID:   0,
						Type: 0x00,
						Features: []FeatureSnapshot{
							{
								ID:            1,
								FeatureMap:    0x0001,
								AttributeList: []uint16{1, 2, 12},
							},
						},
					},
				},
			},
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

	if decoded.Category != CategorySnapshot {
		t.Errorf("Category: got %v, want %v", decoded.Category, CategorySnapshot)
	}
	if decoded.Snapshot == nil {
		t.Fatal("Snapshot is nil")
	}

	// Verify Local snapshot
	local := decoded.Snapshot.Local
	if local == nil {
		t.Fatal("Snapshot.Local is nil")
	}
	if local.DeviceID != "device-001" {
		t.Errorf("Local.DeviceID: got %q, want %q", local.DeviceID, "device-001")
	}
	if local.SpecVersion != "1.0" {
		t.Errorf("Local.SpecVersion: got %q, want %q", local.SpecVersion, "1.0")
	}
	if len(local.Endpoints) != 2 {
		t.Fatalf("Local.Endpoints: got %d, want 2", len(local.Endpoints))
	}

	// Endpoint 0
	ep0 := local.Endpoints[0]
	if ep0.ID != 0 || ep0.Type != 0x00 {
		t.Errorf("Endpoint 0: got ID=%d Type=0x%02x, want ID=0 Type=0x00", ep0.ID, ep0.Type)
	}
	if len(ep0.Features) != 1 {
		t.Fatalf("Endpoint 0 features: got %d, want 1", len(ep0.Features))
	}
	if ep0.Features[0].ID != 1 {
		t.Errorf("Endpoint 0 feature ID: got %d, want 1", ep0.Features[0].ID)
	}
	if len(ep0.Features[0].AttributeList) != 7 {
		t.Errorf("Endpoint 0 feature attributeList: got %d attrs, want 7", len(ep0.Features[0].AttributeList))
	}

	// Endpoint 1
	ep1 := local.Endpoints[1]
	if ep1.ID != 1 || ep1.Type != 0x05 {
		t.Errorf("Endpoint 1: got ID=%d Type=0x%02x, want ID=1 Type=0x05", ep1.ID, ep1.Type)
	}
	if ep1.Label != "Wallbox" {
		t.Errorf("Endpoint 1 Label: got %q, want %q", ep1.Label, "Wallbox")
	}
	if len(ep1.Features) != 2 {
		t.Fatalf("Endpoint 1 features: got %d, want 2", len(ep1.Features))
	}
	if ep1.Features[1].CommandList == nil || len(ep1.Features[1].CommandList) != 1 {
		t.Errorf("Endpoint 1 feature 1 commandList: got %v, want [1]", ep1.Features[1].CommandList)
	}

	// Use cases
	if len(local.UseCases) != 1 {
		t.Fatalf("Local.UseCases: got %d, want 1", len(local.UseCases))
	}
	uc := local.UseCases[0]
	if uc.EndpointID != 1 || uc.ID != 100 || uc.Major != 1 || uc.Minor != 0 || uc.Scenarios != 0x07 {
		t.Errorf("UseCase: got %+v", uc)
	}

	// Verify Remote snapshot
	remote := decoded.Snapshot.Remote
	if remote == nil {
		t.Fatal("Snapshot.Remote is nil")
	}
	if remote.DeviceID != "controller-001" {
		t.Errorf("Remote.DeviceID: got %q, want %q", remote.DeviceID, "controller-001")
	}
	if len(remote.Endpoints) != 1 {
		t.Errorf("Remote.Endpoints: got %d, want 1", len(remote.Endpoints))
	}
}

func TestSnapshotEventCBORRoundTrip_NilRemote(t *testing.T) {
	original := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-snap-002",
		Direction:    DirectionOut,
		Layer:        LayerService,
		Category:     CategorySnapshot,
		Snapshot: &CapabilitySnapshotEvent{
			Local: &DeviceSnapshot{
				DeviceID: "device-002",
				Endpoints: []EndpointSnapshot{
					{
						ID:   0,
						Type: 0x00,
						Features: []FeatureSnapshot{
							{ID: 1, FeatureMap: 0x0001, AttributeList: []uint16{1, 2}},
						},
					},
				},
			},
			Remote: nil,
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

	if decoded.Snapshot == nil {
		t.Fatal("Snapshot is nil")
	}
	if decoded.Snapshot.Local == nil {
		t.Fatal("Snapshot.Local is nil")
	}
	if decoded.Snapshot.Local.DeviceID != "device-002" {
		t.Errorf("Local.DeviceID: got %q, want %q", decoded.Snapshot.Local.DeviceID, "device-002")
	}
	if decoded.Snapshot.Remote != nil {
		t.Errorf("Snapshot.Remote: got %v, want nil", decoded.Snapshot.Remote)
	}
}

func TestSnapshotEvent_BackwardCompat(t *testing.T) {
	// Encode an event with a Snapshot field
	original := Event{
		Timestamp:    time.Now(),
		ConnectionID: "conn-snap-003",
		Direction:    DirectionOut,
		Layer:        LayerService,
		Category:     CategorySnapshot,
		Snapshot: &CapabilitySnapshotEvent{
			Local: &DeviceSnapshot{
				DeviceID: "device-003",
				Endpoints: []EndpointSnapshot{
					{ID: 0, Type: 0x00, Features: []FeatureSnapshot{
						{ID: 1, FeatureMap: 0x0001, AttributeList: []uint16{1}},
					}},
				},
			},
		},
	}

	data, err := EncodeEvent(original)
	if err != nil {
		t.Fatalf("EncodeEvent failed: %v", err)
	}

	// Decode into a struct without the Snapshot field (simulating an older reader).
	// The CBOR decoder is configured with ExtraDecErrorNone, so unknown keys
	// (key 15 = Snapshot) are silently ignored.
	type OldEvent struct {
		Timestamp    time.Time        `cbor:"1,keyasint"`
		ConnectionID string           `cbor:"2,keyasint"`
		Direction    Direction        `cbor:"3,keyasint"`
		Layer        Layer            `cbor:"4,keyasint"`
		Category     Category         `cbor:"5,keyasint"`
		LocalRole    Role             `cbor:"6,keyasint,omitempty"`
		RemoteAddr   string           `cbor:"7,keyasint,omitempty"`
		DeviceID     string           `cbor:"8,keyasint,omitempty"`
		ZoneID       string           `cbor:"9,keyasint,omitempty"`
		Frame        *FrameEvent      `cbor:"10,keyasint,omitempty"`
		Message      *MessageEvent    `cbor:"11,keyasint,omitempty"`
		StateChange  *StateChangeEvent `cbor:"12,keyasint,omitempty"`
		ControlMsg   *ControlMsgEvent `cbor:"13,keyasint,omitempty"`
		Error        *ErrorEventData  `cbor:"14,keyasint,omitempty"`
		// No Snapshot field -- simulates older version
	}

	var old OldEvent
	if err := logDecMode.Unmarshal(data, &old); err != nil {
		t.Fatalf("decoding into OldEvent (without Snapshot) should succeed, got: %v", err)
	}

	if old.ConnectionID != "conn-snap-003" {
		t.Errorf("ConnectionID: got %q, want %q", old.ConnectionID, "conn-snap-003")
	}
	// Category 4 still decodes fine -- it's just a uint8
	if old.Category != CategorySnapshot {
		t.Errorf("Category: got %v, want %v", old.Category, CategorySnapshot)
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

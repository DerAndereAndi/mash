package wire

import (
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		req  Request
	}{
		{
			name: "read request",
			req: Request{
				MessageID:  1,
				Operation:  OpRead,
				EndpointID: 1,
				FeatureID:  2,
				Payload:    []uint16{1, 2, 3},
			},
		},
		{
			name: "write request",
			req: Request{
				MessageID:  2,
				Operation:  OpWrite,
				EndpointID: 1,
				FeatureID:  3,
				Payload: map[uint16]any{
					21: int64(6000000),
				},
			},
		},
		{
			name: "subscribe request",
			req: Request{
				MessageID:  3,
				Operation:  OpSubscribe,
				EndpointID: 1,
				FeatureID:  2,
				Payload: SubscribePayload{
					AttributeIDs: []uint16{1, 2, 3},
					MinInterval:  1000,
					MaxInterval:  60000,
				},
			},
		},
		{
			name: "invoke request",
			req: Request{
				MessageID:  4,
				Operation:  OpInvoke,
				EndpointID: 1,
				FeatureID:  3,
				Payload: InvokePayload{
					CommandID: 1,
					Parameters: map[uint8]any{
						1: int64(6000000),
						4: uint8(2),
					},
				},
			},
		},
		{
			name: "read all attributes",
			req: Request{
				MessageID:  5,
				Operation:  OpRead,
				EndpointID: 1,
				FeatureID:  2,
				Payload:    []uint16{}, // empty = read all
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := EncodeRequest(&tt.req)
			if err != nil {
				t.Fatalf("EncodeRequest failed: %v", err)
			}

			// Decode
			decoded, err := DecodeRequest(data)
			if err != nil {
				t.Fatalf("DecodeRequest failed: %v", err)
			}

			// Verify basic fields
			if decoded.MessageID != tt.req.MessageID {
				t.Errorf("MessageID mismatch: got %d, want %d", decoded.MessageID, tt.req.MessageID)
			}
			if decoded.Operation != tt.req.Operation {
				t.Errorf("Operation mismatch: got %v, want %v", decoded.Operation, tt.req.Operation)
			}
			if decoded.EndpointID != tt.req.EndpointID {
				t.Errorf("EndpointID mismatch: got %d, want %d", decoded.EndpointID, tt.req.EndpointID)
			}
			if decoded.FeatureID != tt.req.FeatureID {
				t.Errorf("FeatureID mismatch: got %d, want %d", decoded.FeatureID, tt.req.FeatureID)
			}
		})
	}
}

func TestResponseRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		resp Response
	}{
		{
			name: "success response",
			resp: Response{
				MessageID: 1,
				Status:    StatusSuccess,
				Payload: map[uint16]any{
					1: int64(5000000),
					2: int64(200000),
				},
			},
		},
		{
			name: "error response",
			resp: Response{
				MessageID: 2,
				Status:    StatusInvalidParameter,
				Payload: ErrorPayload{
					Message: "consumptionLimit must be >= 0",
				},
			},
		},
		{
			name: "subscribe response",
			resp: Response{
				MessageID: 3,
				Status:    StatusSuccess,
				Payload: SubscribeResponsePayload{
					SubscriptionID: 5001,
					CurrentValues: map[uint16]any{
						1: int64(5000000),
						2: int64(200000),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodeResponse(&tt.resp)
			if err != nil {
				t.Fatalf("EncodeResponse failed: %v", err)
			}

			decoded, err := DecodeResponse(data)
			if err != nil {
				t.Fatalf("DecodeResponse failed: %v", err)
			}

			if decoded.MessageID != tt.resp.MessageID {
				t.Errorf("MessageID mismatch: got %d, want %d", decoded.MessageID, tt.resp.MessageID)
			}
			if decoded.Status != tt.resp.Status {
				t.Errorf("Status mismatch: got %v, want %v", decoded.Status, tt.resp.Status)
			}
		})
	}
}

func TestNotificationRoundTrip(t *testing.T) {
	notif := Notification{
		SubscriptionID: 5001,
		EndpointID:     1,
		FeatureID:      2,
		Changes: map[uint16]any{
			1: int64(5500000),
		},
	}

	data, err := EncodeNotification(&notif)
	if err != nil {
		t.Fatalf("EncodeNotification failed: %v", err)
	}

	decoded, err := DecodeNotification(data)
	if err != nil {
		t.Fatalf("DecodeNotification failed: %v", err)
	}

	if decoded.SubscriptionID != notif.SubscriptionID {
		t.Errorf("SubscriptionID mismatch: got %d, want %d", decoded.SubscriptionID, notif.SubscriptionID)
	}
	if decoded.EndpointID != notif.EndpointID {
		t.Errorf("EndpointID mismatch: got %d, want %d", decoded.EndpointID, notif.EndpointID)
	}
	if decoded.FeatureID != notif.FeatureID {
		t.Errorf("FeatureID mismatch: got %d, want %d", decoded.FeatureID, notif.FeatureID)
	}
}

func TestControlMessageRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		msg  ControlMessage
	}{
		{
			name: "ping",
			msg:  ControlMessage{Type: ControlPing, Sequence: 1},
		},
		{
			name: "pong",
			msg:  ControlMessage{Type: ControlPong, Sequence: 1},
		},
		{
			name: "close",
			msg:  ControlMessage{Type: ControlClose},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := EncodeControlMessage(&tt.msg)
			if err != nil {
				t.Fatalf("EncodeControlMessage failed: %v", err)
			}

			decoded, err := DecodeControlMessage(data)
			if err != nil {
				t.Fatalf("DecodeControlMessage failed: %v", err)
			}

			if decoded.Type != tt.msg.Type {
				t.Errorf("Type mismatch: got %v, want %v", decoded.Type, tt.msg.Type)
			}
			if decoded.Sequence != tt.msg.Sequence {
				t.Errorf("Sequence mismatch: got %d, want %d", decoded.Sequence, tt.msg.Sequence)
			}
		})
	}
}

func TestRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name: "valid request",
			req: Request{
				MessageID:  1,
				Operation:  OpRead,
				EndpointID: 1,
				FeatureID:  2,
			},
			wantErr: false,
		},
		{
			name: "messageId 0 reserved",
			req: Request{
				MessageID:  0,
				Operation:  OpRead,
				EndpointID: 1,
				FeatureID:  2,
			},
			wantErr: true,
		},
		{
			name: "invalid operation",
			req: Request{
				MessageID:  1,
				Operation:  Operation(99),
				EndpointID: 1,
				FeatureID:  2,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNullableVsAbsent(t *testing.T) {
	// Test that null values are preserved (not treated as absent)
	payload := map[uint16]any{
		1: uint64(5000000), // Has value
		2: nil,             // Explicitly null
		// Key 3 is absent
	}

	data, err := Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded map[uint16]any
	if err := Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Check value present (CBOR decodes positive integers as uint64)
	if v, ok := decoded[1]; !ok {
		t.Errorf("Key 1 should be present")
	} else if v != uint64(5000000) {
		t.Errorf("Key 1: got %v (%T), want 5000000", v, v)
	}

	// Check null preserved
	if v, ok := decoded[2]; !ok {
		t.Errorf("Key 2 should be present (with null value)")
	} else if v != nil {
		t.Errorf("Key 2: got %v, want nil", v)
	}

	// Check absent key
	if _, ok := decoded[3]; ok {
		t.Errorf("Key 3 should be absent")
	}
}

func TestCBORCompactness(t *testing.T) {
	// Verify that CBOR with integer keys is reasonably compact
	req := Request{
		MessageID:  12345,
		Operation:  OpRead,
		EndpointID: 1,
		FeatureID:  2,
		Payload:    []uint16{1, 2, 3},
	}

	data, err := EncodeRequest(&req)
	if err != nil {
		t.Fatalf("EncodeRequest failed: %v", err)
	}

	// Should be much smaller than JSON equivalent
	// JSON: {"1":12345,"2":1,"3":1,"4":2,"5":[1,2,3]} = ~40 bytes
	// CBOR with integer keys should be ~15-20 bytes
	if len(data) > 30 {
		t.Errorf("CBOR encoding too large: %d bytes (expected < 30)", len(data))
	}

	t.Logf("CBOR size: %d bytes", len(data))
}

func TestUnknownFieldsIgnored(t *testing.T) {
	// Test forward compatibility: unknown fields should be ignored
	// This simulates receiving a message from a newer protocol version

	// Create a map with extra fields
	msg := map[int]any{
		1:  uint32(1),         // messageId
		2:  uint8(1),          // operation
		3:  uint8(1),          // endpointId
		4:  uint8(2),          // featureId
		5:  []uint16{1, 2, 3}, // payload
		99: "future field",    // unknown field from future version
	}

	data, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Should decode without error, ignoring unknown field
	decoded, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest should succeed with unknown fields: %v", err)
	}

	if decoded.MessageID != 1 {
		t.Errorf("MessageID mismatch: got %d, want 1", decoded.MessageID)
	}
}

func TestClone(t *testing.T) {
	original := Request{
		MessageID:  1,
		Operation:  OpRead,
		EndpointID: 1,
		FeatureID:  2,
		Payload:    []uint16{1, 2, 3},
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	if cloned.MessageID != original.MessageID {
		t.Errorf("MessageID mismatch")
	}
	if cloned.Operation != original.Operation {
		t.Errorf("Operation mismatch")
	}
}

func TestEqual(t *testing.T) {
	a := Request{
		MessageID:  1,
		Operation:  OpRead,
		EndpointID: 1,
		FeatureID:  2,
	}
	b := Request{
		MessageID:  1,
		Operation:  OpRead,
		EndpointID: 1,
		FeatureID:  2,
	}
	c := Request{
		MessageID:  2, // different
		Operation:  OpRead,
		EndpointID: 1,
		FeatureID:  2,
	}

	if !Equal(a, b) {
		t.Errorf("Equal(a, b) should be true")
	}
	if Equal(a, c) {
		t.Errorf("Equal(a, c) should be false")
	}
}

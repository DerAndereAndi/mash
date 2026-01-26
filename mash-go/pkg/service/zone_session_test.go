package service

import (
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// createTestDevice creates a device with DeviceInfo feature for testing.
func createTestDevice() *model.Device {
	device := model.NewDevice("test-device", 0x1234, 0x5678)

	// Get endpoint 0 (DEVICE_ROOT) and add DeviceInfo feature
	endpoint, _ := device.GetEndpoint(0)
	deviceInfo := model.NewFeature(model.FeatureDeviceInfo, 1)
	endpoint.AddFeature(deviceInfo)

	return device
}

// mockSendableConnection implements Sendable and tracks sent messages.
type mockSendableConnection struct {
	mu       sync.Mutex
	sent     [][]byte
	sendErr  error
	closed   bool
}

func newMockSendableConnection() *mockSendableConnection {
	return &mockSendableConnection{}
}

func (m *mockSendableConnection) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, data)
	return nil
}

func (m *mockSendableConnection) SentMessages() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]byte, len(m.sent))
	copy(result, m.sent)
	return result
}

func (m *mockSendableConnection) SetSendError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendErr = err
}

func TestZoneSession_HandleReadRequest(t *testing.T) {
	// Create a device with a feature
	device := createTestDevice()

	// Create mock connection
	conn := newMockSendableConnection()

	// Create zone session
	session := NewZoneSession("zone-1", conn, device)

	// Create a read request for DeviceInfo (endpoint 0, feature 6)
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo), // DeviceInfo feature = 6
		Payload:    nil,                            // Read all attributes
	}

	// Encode the request
	reqData, err := wire.EncodeRequest(req)
	if err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	// Handle the message
	session.OnMessage(reqData)

	// Wait for response to be sent
	time.Sleep(50 * time.Millisecond)

	// Check that a response was sent
	sent := conn.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(sent))
	}

	// Decode the response
	resp, err := wire.DecodeResponse(sent[0])
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.MessageID != req.MessageID {
		t.Errorf("Response MessageID mismatch: got %d, want %d", resp.MessageID, req.MessageID)
	}

	if resp.Status != wire.StatusSuccess {
		t.Errorf("Expected success status, got %v", resp.Status)
	}

	// Payload should contain device attributes
	if resp.Payload == nil {
		t.Error("Expected payload with attributes")
	}
}

func TestZoneSession_HandleReadRequest_InvalidEndpoint(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)

	// Request for non-existent endpoint
	req := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpRead,
		EndpointID: 99, // Non-existent
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}

	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)

	time.Sleep(50 * time.Millisecond)

	sent := conn.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(sent))
	}

	resp, _ := wire.DecodeResponse(sent[0])
	if resp.Status != wire.StatusInvalidEndpoint {
		t.Errorf("Expected InvalidEndpoint status, got %v", resp.Status)
	}
}

func TestZoneSession_HandleWriteRequest(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)

	// Write request - note: most DeviceInfo attributes are read-only
	// This test verifies the flow even if the write is rejected
	req := &wire.Request{
		MessageID:  3,
		Operation:  wire.OpWrite,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload:    wire.WritePayload{1: "new-value"},
	}

	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)

	time.Sleep(50 * time.Millisecond)

	sent := conn.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(sent))
	}

	resp, _ := wire.DecodeResponse(sent[0])
	// DeviceInfo attributes are read-only, so we expect an error
	if resp.Status == wire.StatusSuccess {
		t.Log("Write unexpectedly succeeded")
	}
	// The important thing is we got a valid response
	if resp.MessageID != req.MessageID {
		t.Errorf("Response MessageID mismatch: got %d, want %d", resp.MessageID, req.MessageID)
	}
}

func TestZoneSession_HandleSubscribeRequest(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)

	// Subscribe request
	req := &wire.Request{
		MessageID:  4,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload:    &wire.SubscribePayload{},
	}

	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)

	time.Sleep(50 * time.Millisecond)

	sent := conn.SentMessages()
	if len(sent) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(sent))
	}

	resp, _ := wire.DecodeResponse(sent[0])
	if resp.Status != wire.StatusSuccess {
		t.Errorf("Expected success status, got %v", resp.Status)
	}

	// Check subscription count
	if session.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", session.SubscriptionCount())
	}
}

func TestZoneSession_Close(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)

	// Create a subscription first
	req := &wire.Request{
		MessageID:  5,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload:    &wire.SubscribePayload{},
	}

	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)
	time.Sleep(50 * time.Millisecond)

	if session.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription before close, got %d", session.SubscriptionCount())
	}

	// Close the session
	session.Close()

	// Subscriptions should be cleared
	if session.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after close, got %d", session.SubscriptionCount())
	}
}

func TestZoneSession_ZoneID(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("my-zone-id", conn, device)

	if session.ZoneID() != "my-zone-id" {
		t.Errorf("Expected zone ID 'my-zone-id', got %q", session.ZoneID())
	}
}

func TestZoneSession_HandleMultipleRequests(t *testing.T) {
	device := createTestDevice()
	conn := newMockSendableConnection()
	session := NewZoneSession("zone-1", conn, device)

	// Send multiple read requests
	for i := uint32(1); i <= 3; i++ {
		req := &wire.Request{
			MessageID:  i,
			Operation:  wire.OpRead,
			EndpointID: 0,
			FeatureID:  uint8(model.FeatureDeviceInfo),
		}
		reqData, _ := wire.EncodeRequest(req)
		session.OnMessage(reqData)
	}

	time.Sleep(100 * time.Millisecond)

	// Should have 3 responses
	sent := conn.SentMessages()
	if len(sent) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(sent))
	}

	// Verify each response
	for i, data := range sent {
		resp, err := wire.DecodeResponse(data)
		if err != nil {
			t.Errorf("Failed to decode response %d: %v", i, err)
			continue
		}
		if resp.Status != wire.StatusSuccess {
			t.Errorf("Response %d: expected success, got %v", i, resp.Status)
		}
	}
}

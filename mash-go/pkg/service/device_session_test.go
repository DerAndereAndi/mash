package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// testCapturingLogger records log events for testing.
type testCapturingLogger struct {
	mu     sync.Mutex
	events []log.Event
}

func newTestCapturingLogger() *testCapturingLogger {
	return &testCapturingLogger{}
}

func (l *testCapturingLogger) Log(event log.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *testCapturingLogger) Events() []log.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]log.Event, len(l.events))
	copy(result, l.events)
	return result
}

// mockResponseConnection simulates a device connection by echoing back responses.
type mockResponseConnection struct {
	mu        sync.Mutex
	sent      [][]byte
	handler   *ProtocolHandler
	onMessage func([]byte) // Callback for outgoing messages
}

func newMockResponseConnection(device *model.Device) *mockResponseConnection {
	return &mockResponseConnection{
		handler: NewProtocolHandler(device),
	}
}

func (m *mockResponseConnection) Send(data []byte) error {
	m.mu.Lock()
	m.sent = append(m.sent, data)
	handler := m.handler
	onMessage := m.onMessage
	m.mu.Unlock()

	// Check if this is a request (we need to respond to it)
	msgType, err := wire.PeekMessageType(data)
	if err != nil || msgType != wire.MessageTypeRequest {
		return nil
	}

	// Decode request
	req, err := wire.DecodeRequest(data)
	if err != nil {
		return nil
	}

	// Process through handler to get response
	resp := handler.HandleRequest(req)

	// Encode response
	respData, err := wire.EncodeResponse(resp)
	if err != nil {
		return nil
	}

	// Deliver response asynchronously (simulates network)
	if onMessage != nil {
		go func() {
			time.Sleep(1 * time.Millisecond)
			onMessage(respData)
		}()
	}

	return nil
}

func (m *mockResponseConnection) SetOnMessage(fn func([]byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onMessage = fn
}

func TestDeviceSession_Read(t *testing.T) {
	// Create a device with DeviceInfo
	device := createTestDevice()

	// Create mock connection that simulates device responses
	conn := newMockResponseConnection(device)

	// Create device session
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// Wire up the response handler
	conn.SetOnMessage(session.OnMessage)

	// Read from DeviceInfo feature
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attrs, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if attrs == nil {
		t.Fatal("Expected attributes map, got nil")
	}

	// Should have at least the global attributes
	if len(attrs) == 0 {
		t.Error("Expected at least some attributes")
	}
}

func TestDeviceSession_Read_InvalidEndpoint(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := session.Read(ctx, 99, uint8(model.FeatureDeviceInfo), nil)
	if err == nil {
		t.Fatal("Expected error for invalid endpoint")
	}

	// Should be a StatusError from the interaction package
	if _, ok := err.(*interaction.StatusError); !ok {
		t.Errorf("Expected interaction.StatusError, got %T", err)
	}
}

func TestDeviceSession_Write(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to write (will fail since DeviceInfo is read-only, but flow should work)
	_, err := session.Write(ctx, 0, uint8(model.FeatureDeviceInfo), map[uint16]any{1: "test"})
	if err == nil {
		t.Log("Write succeeded (unexpected but not an error)")
	} else {
		// This is expected - DeviceInfo attributes are read-only
		t.Logf("Write returned error as expected: %v", err)
	}
}

func TestDeviceSession_Subscribe(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subID, priming, err := session.Subscribe(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if subID == 0 {
		t.Error("Expected non-zero subscription ID")
	}

	if priming == nil {
		t.Error("Expected priming report")
	} else {
		t.Logf("Got subscription ID %d with %d priming attributes", subID, len(priming))
	}
}

func TestDeviceSession_HandleNotification(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// Set up notification handler
	var receivedNotif *wire.Notification
	var notifMu sync.Mutex
	session.SetNotificationHandler(func(notif *wire.Notification) {
		notifMu.Lock()
		receivedNotif = notif
		notifMu.Unlock()
	})

	// Simulate receiving a notification
	notif := &wire.Notification{
		SubscriptionID: 1,
		EndpointID:     0,
		FeatureID:      uint8(model.FeatureDeviceInfo),
		Changes: map[uint16]any{
			1: "changed-value",
		},
	}

	notifData, err := wire.EncodeNotification(notif)
	if err != nil {
		t.Fatalf("Failed to encode notification: %v", err)
	}

	// Deliver the notification
	session.OnMessage(notifData)

	// Wait for handler to be called
	time.Sleep(50 * time.Millisecond)

	notifMu.Lock()
	received := receivedNotif
	notifMu.Unlock()

	if received == nil {
		t.Fatal("Expected notification to be received")
	}

	if received.SubscriptionID != 1 {
		t.Errorf("Expected subscription ID 1, got %d", received.SubscriptionID)
	}

	if len(received.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(received.Changes))
	}
}

func TestDeviceSession_DeviceID(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("my-device-123", conn)
	defer session.Close()

	if session.DeviceID() != "my-device-123" {
		t.Errorf("Expected device ID 'my-device-123', got %q", session.DeviceID())
	}
}

func TestDeviceSession_Close(t *testing.T) {
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	conn.SetOnMessage(session.OnMessage)

	// Close the session
	session.Close()

	// Operations should fail after close
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err == nil {
		t.Error("Expected error after close")
	}
}

// ============================================================================
// Phase 4: Bidirectional Support Tests
// ============================================================================

// mockDeviceSessionConn captures sent messages and supports bidirectional traffic.
// Named differently from mockBidirectionalConnection in zone_session_test.go.
type mockDeviceSessionConn struct {
	mu   sync.Mutex
	sent [][]byte
}

func (m *mockDeviceSessionConn) Send(data []byte) error {
	m.mu.Lock()
	m.sent = append(m.sent, data)
	m.mu.Unlock()
	return nil
}

func (m *mockDeviceSessionConn) GetSent() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([][]byte, len(m.sent))
	copy(result, m.sent)
	return result
}

func (m *mockDeviceSessionConn) ClearSent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
}

func TestDeviceSession_HandleIncomingRequest_NoHandler(t *testing.T) {
	// When DeviceSession has no ProtocolHandler configured,
	// incoming requests should receive StatusUnsupported
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// Send a Read request to the session (as if from the device)
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}
	reqData, err := wire.EncodeRequest(req)
	if err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	// Deliver the request
	session.OnMessage(reqData)

	// Wait for response to be sent
	time.Sleep(10 * time.Millisecond)

	// Should have sent a response
	sent := conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("Expected a response to be sent")
	}

	// Decode the response
	resp, err := wire.DecodeResponse(sent[0])
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be StatusUnsupported since no handler is configured
	if resp.Status != wire.StatusUnsupported {
		t.Errorf("Expected StatusUnsupported, got %v", resp.Status)
	}
	if resp.MessageID != 1 {
		t.Errorf("Expected MessageID 1, got %d", resp.MessageID)
	}
}

func TestDeviceSession_HandleIncomingRequest_WithHandler(t *testing.T) {
	// When DeviceSession has a ProtocolHandler configured with a device model,
	// incoming requests should be processed and receive valid responses
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// Create a device model for the controller to expose
	controllerDevice := createTestDevice() // Has endpoint 0 with DeviceInfo

	// Configure the session with a ProtocolHandler
	session.SetExposedDevice(controllerDevice)

	// Send a Read request for DeviceInfo
	req := &wire.Request{
		MessageID:  42,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}
	reqData, err := wire.EncodeRequest(req)
	if err != nil {
		t.Fatalf("Failed to encode request: %v", err)
	}

	// Deliver the request
	session.OnMessage(reqData)

	// Wait for response to be sent
	time.Sleep(10 * time.Millisecond)

	// Should have sent a response
	sent := conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("Expected a response to be sent")
	}

	// Decode the response
	resp, err := wire.DecodeResponse(sent[0])
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be successful
	if resp.Status != wire.StatusSuccess {
		t.Errorf("Expected StatusSuccess, got %v", resp.Status)
	}
	if resp.MessageID != 42 {
		t.Errorf("Expected MessageID 42, got %d", resp.MessageID)
	}

	// Payload should be a map (ReadResponsePayload is map[uint16]any)
	// The codec may return it as map[uint16]any or map[interface{}]interface{}
	if resp.Payload == nil {
		t.Fatal("Expected payload in response")
	}
}

func TestDeviceSession_HandleIncomingRequest_InvalidEndpoint(t *testing.T) {
	// When request targets an invalid endpoint, should return error
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// Configure with a device model
	session.SetExposedDevice(createTestDevice())

	// Request for non-existent endpoint
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpRead,
		EndpointID: 99, // Does not exist
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}
	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)

	time.Sleep(10 * time.Millisecond)

	sent := conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("Expected a response")
	}

	resp, _ := wire.DecodeResponse(sent[0])
	if resp.Status == wire.StatusSuccess {
		t.Error("Expected error status for invalid endpoint")
	}
}

func TestDeviceSession_HandleIncomingRequest_Subscribe(t *testing.T) {
	// Controller can accept subscriptions from devices
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	session.SetExposedDevice(createTestDevice())

	// Subscribe request
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}
	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)

	time.Sleep(10 * time.Millisecond)

	sent := conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("Expected a response")
	}

	resp, _ := wire.DecodeResponse(sent[0])
	if resp.Status != wire.StatusSuccess {
		t.Errorf("Expected StatusSuccess, got %v", resp.Status)
	}

	// Payload should contain subscription ID (CBOR returns raw map)
	subID := extractSubscriptionID(t, resp.Payload)
	if subID == 0 {
		t.Error("Expected non-zero subscription ID")
	}
}

// extractSubscriptionID extracts the subscription ID from a subscribe response payload.
// The CBOR decoder returns map[interface{}]interface{} instead of typed struct.
func extractSubscriptionID(t *testing.T, payload any) uint32 {
	t.Helper()
	if payload == nil {
		return 0
	}

	// Try typed struct first (in case codec improves)
	if subPayload, ok := payload.(*wire.SubscribeResponsePayload); ok {
		return subPayload.SubscriptionID
	}

	// Handle raw map from CBOR decoder
	// Key 1 = subscriptionId (see wire.SubscribeResponsePayload CBOR tags)
	m, ok := payload.(map[interface{}]interface{})
	if !ok {
		t.Fatalf("Unexpected payload type: %T", payload)
		return 0
	}

	// Find subscription ID at key 1 (int or uint)
	for k, v := range m {
		var keyInt int
		switch kv := k.(type) {
		case int:
			keyInt = kv
		case int64:
			keyInt = int(kv)
		case uint64:
			keyInt = int(kv)
		default:
			continue
		}
		if keyInt == 1 { // Key 1 = SubscriptionID
			switch vv := v.(type) {
			case uint64:
				return uint32(vv)
			case int64:
				return uint32(vv)
			case int:
				return uint32(vv)
			}
		}
	}
	return 0
}

func TestDeviceSession_SendNotification(t *testing.T) {
	// Controller can send notifications to device for subscriptions
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	session.SetExposedDevice(createTestDevice())

	// First, device subscribes
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
	}
	reqData, _ := wire.EncodeRequest(req)
	session.OnMessage(reqData)
	time.Sleep(10 * time.Millisecond)

	// Get the subscription ID from response
	sent := conn.GetSent()
	resp, _ := wire.DecodeResponse(sent[0])
	subID := extractSubscriptionID(t, resp.Payload)
	if subID == 0 {
		t.Fatal("Failed to get subscription ID from response")
	}

	conn.ClearSent()

	// Now send a notification using the session's method
	err := session.SendNotification(&wire.Notification{
		SubscriptionID: subID,
		EndpointID:     0,
		FeatureID:      uint8(model.FeatureDeviceInfo),
		Changes: map[uint16]any{
			1: "updated-value",
		},
	})
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	// Should have sent a notification
	sent = conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("Expected notification to be sent")
	}

	// Verify it's a notification message
	msgType, _ := wire.PeekMessageType(sent[0])
	if msgType != wire.MessageTypeNotification {
		t.Errorf("Expected notification message type, got %v", msgType)
	}
}

func TestDeviceSession_ExistingResponseHandlingStillWorks(t *testing.T) {
	// Ensure adding request handling doesn't break existing response handling
	device := createTestDevice()
	conn := newMockResponseConnection(device)
	session := NewDeviceSession("device-1", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	// This is the existing test flow - should still work
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attrs, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if attrs == nil {
		t.Fatal("Expected attributes")
	}
}

func TestDeviceSession_ExistingNotificationHandlingStillWorks(t *testing.T) {
	// Ensure adding request handling doesn't break existing notification handling
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	var received *wire.Notification
	var mu sync.Mutex
	session.SetNotificationHandler(func(n *wire.Notification) {
		mu.Lock()
		received = n
		mu.Unlock()
	})

	// Simulate receiving a notification (from device to controller)
	notif := &wire.Notification{
		SubscriptionID: 123,
		EndpointID:     1,
		FeatureID:      5,
		Changes:        map[uint16]any{1: "value"},
	}
	notifData, _ := wire.EncodeNotification(notif)
	session.OnMessage(notifData)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("Expected notification to be received")
	}
	if received.SubscriptionID != 123 {
		t.Errorf("Expected subscription ID 123, got %d", received.SubscriptionID)
	}
}

// =============================================================================
// DeviceSession Protocol Logging Tests
// =============================================================================

func TestDeviceSession_LogsIncomingResponse(t *testing.T) {
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	logger := newTestCapturingLogger()
	session.SetProtocolLogger(logger, "conn-sess-resp")

	// Simulate receiving a response
	resp := &wire.Response{
		MessageID: 42,
		Status:    wire.StatusSuccess,
		Payload:   map[uint16]any{1: "test"},
	}
	respData, _ := wire.EncodeResponse(resp)
	session.OnMessage(respData)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	events := logger.Events()
	if len(events) == 0 {
		t.Fatal("Expected at least one log event for response")
	}

	// Find the response event
	var respEvent *log.Event
	for i := range events {
		if events[i].Message != nil && events[i].Message.Type == log.MessageTypeResponse {
			respEvent = &events[i]
			break
		}
	}

	if respEvent == nil {
		t.Fatal("Expected to find response event in log")
	}

	// Verify event properties
	if respEvent.Direction != log.DirectionIn {
		t.Errorf("Response should have Direction=In, got %s", respEvent.Direction)
	}
	if respEvent.Layer != log.LayerWire {
		t.Errorf("Response should have Layer=Wire, got %s", respEvent.Layer)
	}
	if respEvent.ConnectionID != "conn-sess-resp" {
		t.Errorf("Response should have ConnectionID='conn-sess-resp', got '%s'", respEvent.ConnectionID)
	}
	if respEvent.Message.MessageID != 42 {
		t.Errorf("Response should have MessageID=42, got %d", respEvent.Message.MessageID)
	}
	if respEvent.Message.Status == nil || *respEvent.Message.Status != wire.StatusSuccess {
		t.Errorf("Response should have Status=Success")
	}
}

func TestDeviceSession_LogsIncomingNotification(t *testing.T) {
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	logger := newTestCapturingLogger()
	session.SetProtocolLogger(logger, "conn-sess-notif")

	// Simulate receiving a notification
	notif := &wire.Notification{
		SubscriptionID: 999,
		EndpointID:     1,
		FeatureID:      5,
		Changes:        map[uint16]any{1: "changed-value"},
	}
	notifData, _ := wire.EncodeNotification(notif)
	session.OnMessage(notifData)

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	events := logger.Events()
	if len(events) == 0 {
		t.Fatal("Expected at least one log event for notification")
	}

	// Find the notification event
	var notifEvent *log.Event
	for i := range events {
		if events[i].Message != nil && events[i].Message.Type == log.MessageTypeNotification {
			notifEvent = &events[i]
			break
		}
	}

	if notifEvent == nil {
		t.Fatal("Expected to find notification event in log")
	}

	// Verify event properties
	if notifEvent.Direction != log.DirectionIn {
		t.Errorf("Notification should have Direction=In, got %s", notifEvent.Direction)
	}
	if notifEvent.Layer != log.LayerWire {
		t.Errorf("Notification should have Layer=Wire, got %s", notifEvent.Layer)
	}
	if notifEvent.ConnectionID != "conn-sess-notif" {
		t.Errorf("Notification should have ConnectionID='conn-sess-notif', got '%s'", notifEvent.ConnectionID)
	}
	if notifEvent.Message.SubscriptionID == nil || *notifEvent.Message.SubscriptionID != 999 {
		t.Errorf("Notification should have SubscriptionID=999")
	}
}

func TestDeviceSession_NoProtocolLoggerNoPanic(t *testing.T) {
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("device-1", conn)
	defer session.Close()

	// No protocol logger set - should not panic

	// Simulate receiving a response
	resp := &wire.Response{
		MessageID: 1,
		Status:    wire.StatusSuccess,
	}
	respData, _ := wire.EncodeResponse(resp)
	session.OnMessage(respData)

	// Simulate receiving a notification
	notif := &wire.Notification{
		SubscriptionID: 1,
		EndpointID:     1,
		FeatureID:      1,
		Changes:        map[uint16]any{1: "value"},
	}
	notifData, _ := wire.EncodeNotification(notif)
	session.OnMessage(notifData)

	// If we get here without panic, the test passes
}

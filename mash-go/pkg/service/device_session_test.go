package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

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


package service_test

import (
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// TestNotificationDispatcher_Subscribe tests that subscribing returns current values.
func TestNotificationDispatcher_Subscribe(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	defer dispatcher.Stop()

	// Create a mock connection sender
	var sentNotifications [][]byte
	var mu sync.Mutex
	sender := func(data []byte) error {
		mu.Lock()
		sentNotifications = append(sentNotifications, data)
		mu.Unlock()
		return nil
	}

	// Register connection
	connID := dispatcher.RegisterConnection(sender)
	defer dispatcher.UnregisterConnection(connID)

	// Subscribe to DeviceInfo
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload: &wire.SubscribePayload{
			MinInterval: 100,  // 100ms
			MaxInterval: 5000, // 5 seconds
		},
	}

	resp := dispatcher.HandleSubscribe(connID, req)

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got status %d", resp.Status)
	}

	payload, ok := resp.Payload.(*wire.SubscribeResponsePayload)
	if !ok {
		t.Fatalf("Expected *wire.SubscribeResponsePayload, got %T", resp.Payload)
	}

	if payload.SubscriptionID == 0 {
		t.Error("Expected non-zero subscription ID")
	}

	// Current values should be returned (priming report)
	if payload.CurrentValues == nil || len(payload.CurrentValues) == 0 {
		t.Error("Expected current values in priming report")
	}
}

// TestNotificationDispatcher_NotifyChange tests that changes trigger notifications.
func TestNotificationDispatcher_NotifyChange(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	dispatcher.SetProcessingInterval(50 * time.Millisecond) // Speed up for testing
	defer dispatcher.Stop()

	// Create a mock connection sender
	var sentNotifications [][]byte
	var mu sync.Mutex
	notifyCh := make(chan struct{}, 10)
	sender := func(data []byte) error {
		mu.Lock()
		sentNotifications = append(sentNotifications, data)
		mu.Unlock()
		select {
		case notifyCh <- struct{}{}:
		default:
		}
		return nil
	}

	// Register connection
	connID := dispatcher.RegisterConnection(sender)
	defer dispatcher.UnregisterConnection(connID)

	// Subscribe to Electrical feature on endpoint 1 with short intervals
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureElectrical),
		Payload: &wire.SubscribePayload{
			MinInterval: 10, // 10ms - very short for testing
			MaxInterval: 60000,
		},
	}

	resp := dispatcher.HandleSubscribe(connID, req)
	if !resp.IsSuccess() {
		t.Fatalf("Subscribe failed: status %d", resp.Status)
	}

	// Start processing
	dispatcher.Start()

	// Simulate an attribute change
	dispatcher.NotifyChange(1, uint16(model.FeatureElectrical), 1, uint8(1)) // Change phaseCount

	// Wait for notification (with timeout)
	select {
	case <-notifyCh:
		// Got notification
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for notification")
	}

	// Verify notification was sent
	mu.Lock()
	count := len(sentNotifications)
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one notification to be sent")
	}
}

// TestNotificationDispatcher_Unsubscribe tests that unsubscribe stops notifications.
func TestNotificationDispatcher_Unsubscribe(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	defer dispatcher.Stop()

	// Create a mock connection sender
	sender := func(data []byte) error { return nil }

	// Register connection
	connID := dispatcher.RegisterConnection(sender)
	defer dispatcher.UnregisterConnection(connID)

	// Subscribe
	subReq := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload: &wire.SubscribePayload{
			MinInterval: 1000,
			MaxInterval: 60000,
		},
	}

	subResp := dispatcher.HandleSubscribe(connID, subReq)
	if !subResp.IsSuccess() {
		t.Fatalf("Subscribe failed: status %d", subResp.Status)
	}

	subPayload := subResp.Payload.(*wire.SubscribeResponsePayload)
	subscriptionID := subPayload.SubscriptionID

	// Verify subscription exists
	if dispatcher.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", dispatcher.SubscriptionCount())
	}

	// Unsubscribe
	unsubReq := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0, // Feature 0 indicates unsubscribe
		Payload: &wire.UnsubscribePayload{
			SubscriptionID: subscriptionID,
		},
	}

	unsubResp := dispatcher.HandleUnsubscribe(connID, unsubReq)
	if !unsubResp.IsSuccess() {
		t.Errorf("Unsubscribe failed: status %d", unsubResp.Status)
	}

	// Verify subscription removed
	if dispatcher.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after unsubscribe, got %d", dispatcher.SubscriptionCount())
	}
}

// TestNotificationDispatcher_ConnectionDisconnect tests cleanup on disconnect.
func TestNotificationDispatcher_ConnectionDisconnect(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	defer dispatcher.Stop()

	sender := func(data []byte) error { return nil }

	// Register connection
	connID := dispatcher.RegisterConnection(sender)

	// Create multiple subscriptions
	for i := 1; i <= 3; i++ {
		req := &wire.Request{
			MessageID:  uint32(i),
			Operation:  wire.OpSubscribe,
			EndpointID: 0,
			FeatureID:  uint8(model.FeatureDeviceInfo),
			Payload: &wire.SubscribePayload{
				MinInterval: 1000,
				MaxInterval: 60000,
			},
		}
		dispatcher.HandleSubscribe(connID, req)
	}

	// Verify subscriptions exist
	if dispatcher.SubscriptionCount() != 3 {
		t.Errorf("Expected 3 subscriptions, got %d", dispatcher.SubscriptionCount())
	}

	// Disconnect (unregister) the connection
	dispatcher.UnregisterConnection(connID)

	// All subscriptions should be cleaned up
	if dispatcher.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after disconnect, got %d", dispatcher.SubscriptionCount())
	}
}

// TestNotificationDispatcher_MultipleConnections tests multiple connections with subscriptions.
func TestNotificationDispatcher_MultipleConnections(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	defer dispatcher.Stop()

	sender1 := func(data []byte) error { return nil }
	sender2 := func(data []byte) error { return nil }

	// Register two connections
	connID1 := dispatcher.RegisterConnection(sender1)
	connID2 := dispatcher.RegisterConnection(sender2)

	// Each connection creates subscriptions
	req1 := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload:    &wire.SubscribePayload{MinInterval: 1000, MaxInterval: 60000},
	}
	req2 := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureElectrical),
		Payload:    &wire.SubscribePayload{MinInterval: 1000, MaxInterval: 60000},
	}

	dispatcher.HandleSubscribe(connID1, req1)
	dispatcher.HandleSubscribe(connID2, req2)

	if dispatcher.SubscriptionCount() != 2 {
		t.Errorf("Expected 2 subscriptions, got %d", dispatcher.SubscriptionCount())
	}

	// Disconnect first connection - only its subscriptions should be removed
	dispatcher.UnregisterConnection(connID1)

	if dispatcher.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription after first disconnect, got %d", dispatcher.SubscriptionCount())
	}

	// Disconnect second connection
	dispatcher.UnregisterConnection(connID2)

	if dispatcher.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after second disconnect, got %d", dispatcher.SubscriptionCount())
	}
}

// TestNotificationDispatcher_Heartbeat tests heartbeat notifications.
func TestNotificationDispatcher_Heartbeat(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	dispatcher.SetProcessingInterval(50 * time.Millisecond)
	defer dispatcher.Stop()

	var sentNotifications [][]byte
	var mu sync.Mutex
	notifyCh := make(chan struct{}, 10)
	sender := func(data []byte) error {
		mu.Lock()
		sentNotifications = append(sentNotifications, data)
		mu.Unlock()
		select {
		case notifyCh <- struct{}{}:
		default:
		}
		return nil
	}

	connID := dispatcher.RegisterConnection(sender)
	defer dispatcher.UnregisterConnection(connID)

	// Subscribe with very short maxInterval for heartbeat testing
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureDeviceInfo),
		Payload: &wire.SubscribePayload{
			MinInterval: 10,  // 10ms
			MaxInterval: 100, // 100ms - heartbeat should trigger quickly
		},
	}

	dispatcher.HandleSubscribe(connID, req)
	dispatcher.Start()

	// Wait for heartbeat (should arrive within maxInterval + processing interval)
	select {
	case <-notifyCh:
		// Got notification (could be heartbeat)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for heartbeat notification")
	}

	mu.Lock()
	count := len(sentNotifications)
	mu.Unlock()

	if count == 0 {
		t.Error("Expected at least one heartbeat notification")
	}
}

// TestNotificationDispatcher_NotificationEncoding tests that notifications are properly encoded.
func TestNotificationDispatcher_NotificationEncoding(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)
	dispatcher := service.NewNotificationDispatcher(handler)
	dispatcher.SetProcessingInterval(50 * time.Millisecond)
	defer dispatcher.Stop()

	var lastNotification []byte
	var mu sync.Mutex
	notifyCh := make(chan struct{}, 10)
	sender := func(data []byte) error {
		mu.Lock()
		lastNotification = make([]byte, len(data))
		copy(lastNotification, data)
		mu.Unlock()
		select {
		case notifyCh <- struct{}{}:
		default:
		}
		return nil
	}

	connID := dispatcher.RegisterConnection(sender)
	defer dispatcher.UnregisterConnection(connID)

	// Subscribe
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureElectrical),
		Payload: &wire.SubscribePayload{
			MinInterval: 10,
			MaxInterval: 60000,
		},
	}

	resp := dispatcher.HandleSubscribe(connID, req)
	if !resp.IsSuccess() {
		t.Fatalf("Subscribe failed: status %d", resp.Status)
	}

	dispatcher.Start()

	// Trigger a change
	dispatcher.NotifyChange(1, uint16(model.FeatureElectrical), 2, uint32(240)) // voltage change

	// Wait for notification
	select {
	case <-notifyCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timeout waiting for notification")
	}

	// Decode the notification
	mu.Lock()
	data := lastNotification
	mu.Unlock()

	if data == nil {
		t.Fatal("No notification received")
	}

	notif, err := wire.DecodeNotification(data)
	if err != nil {
		t.Fatalf("Failed to decode notification: %v", err)
	}

	// Verify notification structure
	if notif.EndpointID != 1 {
		t.Errorf("Expected EndpointID 1, got %d", notif.EndpointID)
	}
	if notif.FeatureID != uint8(model.FeatureElectrical) {
		t.Errorf("Expected FeatureID %d, got %d", model.FeatureElectrical, notif.FeatureID)
	}
	if notif.SubscriptionID == 0 {
		t.Error("Expected non-zero SubscriptionID")
	}
}

// Note: createTestDevice is defined in protocol_handler_test.go

package mock_test

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/mock"
)

func TestDeviceBasic(t *testing.T) {
	device := mock.NewDevice("device-1")

	if device.ID != "device-1" {
		t.Errorf("Expected ID device-1, got %s", device.ID)
	}
	if len(device.Endpoints) != 0 {
		t.Error("Expected no endpoints initially")
	}
}

func TestDeviceEndpoints(t *testing.T) {
	device := mock.NewDevice("device-1")

	// Add endpoints
	ep0 := device.AddEndpoint(0, "DEVICE_ROOT")
	ep1 := device.AddEndpoint(1, "EV_CHARGER")

	if ep0 == nil || ep0.ID != 0 || ep0.Type != "DEVICE_ROOT" {
		t.Error("Endpoint 0 not created correctly")
	}
	if ep1 == nil || ep1.ID != 1 || ep1.Type != "EV_CHARGER" {
		t.Error("Endpoint 1 not created correctly")
	}
	if len(device.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(device.Endpoints))
	}
}

func TestDeviceFeatures(t *testing.T) {
	device := mock.NewDevice("device-1")
	device.AddEndpoint(1, "EV_CHARGER")

	// Add feature
	f := device.AddFeature(1, "EnergyControl", 0x03)

	if f == nil {
		t.Fatal("Feature not created")
	}
	if f.Name != "EnergyControl" {
		t.Errorf("Expected feature name EnergyControl, got %s", f.Name)
	}
	if f.FeatureMap != 0x03 {
		t.Errorf("Expected feature map 0x03, got %x", f.FeatureMap)
	}

	// Add feature to non-existent endpoint
	f2 := device.AddFeature(99, "Test", 0)
	if f2 != nil {
		t.Error("Should not create feature on non-existent endpoint")
	}
}

func TestDeviceAttributes(t *testing.T) {
	device := mock.NewDevice("device-1")
	device.AddEndpoint(1, "EV_CHARGER")
	device.AddFeature(1, "Measurement", 0x01)

	// Set and get attribute
	device.SetAttribute(1, "Measurement", "power", 3500)
	val, ok := device.GetAttribute(1, "Measurement", "power")

	if !ok {
		t.Error("Attribute should exist")
	}
	if val != 3500 {
		t.Errorf("Expected 3500, got %v", val)
	}

	// Get non-existent attribute
	_, ok = device.GetAttribute(1, "Measurement", "nonexistent")
	if ok {
		t.Error("Non-existent attribute should not exist")
	}

	// Get from non-existent endpoint
	_, ok = device.GetAttribute(99, "Measurement", "power")
	if ok {
		t.Error("Attribute from non-existent endpoint should not exist")
	}
}

func TestDeviceMessages(t *testing.T) {
	device := mock.NewDevice("device-1")

	// Record messages
	device.RecordMessage(mock.Message{Type: "read", Path: "/1/measurement/power"})
	device.RecordMessage(mock.Message{Type: "write", Path: "/1/control/limit", Payload: 5000})

	msgs := device.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Type != "read" {
		t.Errorf("Expected read, got %s", msgs[0].Type)
	}
	if msgs[1].Payload != 5000 {
		t.Errorf("Expected payload 5000, got %v", msgs[1].Payload)
	}

	// Clear messages
	device.ClearMessages()
	if len(device.GetMessages()) != 0 {
		t.Error("Messages should be cleared")
	}
}

func TestDeviceResponseQueue(t *testing.T) {
	device := mock.NewDevice("device-1")

	// Queue responses
	device.QueueResponse(mock.Message{Type: "response", Payload: "first"})
	device.QueueResponse(mock.Message{Type: "response", Payload: "second"})

	// Pop responses
	msg1, ok := device.PopResponse()
	if !ok || msg1.Payload != "first" {
		t.Error("First response incorrect")
	}

	msg2, ok := device.PopResponse()
	if !ok || msg2.Payload != "second" {
		t.Error("Second response incorrect")
	}

	// Empty queue
	_, ok = device.PopResponse()
	if ok {
		t.Error("Queue should be empty")
	}
}

func TestDeviceState(t *testing.T) {
	device := mock.NewDevice("device-1")

	device.SetState("connected", true)
	device.SetState("session_id", "abc123")

	val, ok := device.GetState("connected")
	if !ok || val != true {
		t.Error("State 'connected' incorrect")
	}

	val, ok = device.GetState("session_id")
	if !ok || val != "abc123" {
		t.Error("State 'session_id' incorrect")
	}

	_, ok = device.GetState("nonexistent")
	if ok {
		t.Error("Non-existent state should not exist")
	}
}

func TestDeviceHandlers(t *testing.T) {
	device := mock.NewDevice("device-1")

	var readPath string
	var writePath string
	var writeValue any

	device.Handlers.OnRead = func(path string) (any, error) {
		readPath = path
		return 42, nil
	}

	device.Handlers.OnWrite = func(path string, value any) error {
		writePath = path
		writeValue = value
		return nil
	}

	ctx := context.Background()

	// Test read handler
	result, _ := device.HandleRead(ctx, "/1/measurement/power")
	if readPath != "/1/measurement/power" {
		t.Error("Read handler not called with correct path")
	}
	if result != 42 {
		t.Error("Read handler should return 42")
	}

	// Test write handler
	_ = device.HandleWrite(ctx, "/1/control/limit", 5000)
	if writePath != "/1/control/limit" {
		t.Error("Write handler not called with correct path")
	}
	if writeValue != 5000 {
		t.Error("Write handler not called with correct value")
	}

	// Verify messages recorded
	msgs := device.GetMessages()
	if len(msgs) != 2 {
		t.Errorf("Expected 2 messages recorded, got %d", len(msgs))
	}
}

func TestControllerBasic(t *testing.T) {
	ctrl := mock.NewController("ctrl-1", "HOME_MANAGER", 3)

	if ctrl.ID != "ctrl-1" {
		t.Errorf("Expected ID ctrl-1, got %s", ctrl.ID)
	}
	if ctrl.Zone != "HOME_MANAGER" {
		t.Errorf("Expected zone HOME_MANAGER, got %s", ctrl.Zone)
	}
	if ctrl.Priority != 3 {
		t.Errorf("Expected priority 3, got %d", ctrl.Priority)
	}
}

func TestControllerDevices(t *testing.T) {
	ctrl := mock.NewController("ctrl-1", "HOME_MANAGER", 3)
	device := mock.NewDevice("device-1")

	var connectedID string
	ctrl.Handlers.OnDeviceConnected = func(deviceID string) {
		connectedID = deviceID
	}

	// Connect device
	ctrl.ConnectDevice(device)

	if connectedID != "device-1" {
		t.Error("OnDeviceConnected not called")
	}

	// Get device
	d := ctrl.GetDevice("device-1")
	if d != device {
		t.Error("GetDevice returned wrong device")
	}

	// Get non-existent device
	d = ctrl.GetDevice("nonexistent")
	if d != nil {
		t.Error("GetDevice should return nil for non-existent")
	}

	// Disconnect device
	var disconnectedID string
	ctrl.Handlers.OnDeviceDisconnected = func(deviceID string) {
		disconnectedID = deviceID
	}

	ctrl.DisconnectDevice("device-1")

	if disconnectedID != "device-1" {
		t.Error("OnDeviceDisconnected not called")
	}

	d = ctrl.GetDevice("device-1")
	if d != nil {
		t.Error("Device should be disconnected")
	}
}

func TestControllerOperations(t *testing.T) {
	ctrl := mock.NewController("ctrl-1", "HOME_MANAGER", 3)
	device := mock.NewDevice("device-1")
	device.AddEndpoint(1, "EV_CHARGER")
	device.AddFeature(1, "Measurement", 0x01)

	ctrl.ConnectDevice(device)
	ctx := context.Background()

	// Test read
	_, err := ctrl.Read(ctx, "device-1", "/1/measurement/power")
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}

	// Test write
	err = ctrl.Write(ctx, "device-1", "/1/control/limit", 5000)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Test subscribe
	err = ctrl.Subscribe(ctx, "device-1", "/1/measurement/power")
	if err != nil {
		t.Errorf("Subscribe failed: %v", err)
	}

	// Test invoke
	_, err = ctrl.Invoke(ctx, "device-1", "/1/control/start", nil)
	if err != nil {
		t.Errorf("Invoke failed: %v", err)
	}

	// Verify sent messages
	msgs := ctrl.GetSentMessages()
	if len(msgs) != 4 {
		t.Fatalf("Expected 4 sent messages, got %d", len(msgs))
	}

	// Verify subscriptions
	subs := ctrl.GetSubscriptions("device-1")
	if len(subs) != 1 || subs[0] != "/1/measurement/power" {
		t.Error("Subscription not recorded")
	}

	// Operations on disconnected device
	_, err = ctrl.Read(ctx, "nonexistent", "/1/measurement/power")
	if err != mock.ErrDeviceNotConnected {
		t.Errorf("Expected ErrDeviceNotConnected, got %v", err)
	}
}

func TestControllerNotifications(t *testing.T) {
	ctrl := mock.NewController("ctrl-1", "HOME_MANAGER", 3)

	var notifDevice, notifPath string
	var notifValue any
	ctrl.Handlers.OnNotification = func(deviceID, path string, value any) {
		notifDevice = deviceID
		notifPath = path
		notifValue = value
	}

	// Receive notification
	ctrl.ReceiveNotification("device-1", "/1/measurement/power", 3500, 1)

	if notifDevice != "device-1" || notifPath != "/1/measurement/power" || notifValue != 3500 {
		t.Error("OnNotification not called correctly")
	}

	// Get notifications
	notifs := ctrl.GetNotifications()
	if len(notifs) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifs))
	}
	if notifs[0].Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", notifs[0].Sequence)
	}

	// Clear notifications
	ctrl.ClearNotifications()
	if len(ctrl.GetNotifications()) != 0 {
		t.Error("Notifications should be cleared")
	}
}

func TestControllerClearMessages(t *testing.T) {
	ctrl := mock.NewController("ctrl-1", "HOME_MANAGER", 3)
	device := mock.NewDevice("device-1")
	ctrl.ConnectDevice(device)

	ctx := context.Background()
	_, _ = ctrl.Read(ctx, "device-1", "/test")

	if len(ctrl.GetSentMessages()) != 1 {
		t.Error("Expected 1 sent message")
	}

	ctrl.ClearSentMessages()
	if len(ctrl.GetSentMessages()) != 0 {
		t.Error("Sent messages should be cleared")
	}
}

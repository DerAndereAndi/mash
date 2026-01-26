package service

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// parseTestPort extracts the port number from an address string.
// The format is "IP:PORT" or "[::]:PORT".
func parseTestPort(addr string) uint16 {
	var port uint16
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			for j := i + 1; j < len(addr); j++ {
				port = port*10 + uint16(addr[j]-'0')
			}
			return port
		}
	}
	return 0
}

// TestE2E_CommissioningFlow tests the full commissioning flow:
// 1. Device starts and enters commissioning mode
// 2. Controller discovers and commissions with correct setup code
// 3. Both sides emit appropriate events
func TestE2E_CommissioningFlow(t *testing.T) {
	// === Setup Device ===
	device := model.NewDevice("evse-001", 0x1234, 0x5678)
	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"
	deviceConfig.Discriminator = 1234

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	// Track device events
	var deviceConnectedZoneID string
	var deviceEventMu sync.Mutex
	deviceSvc.OnEvent(func(e Event) {
		deviceEventMu.Lock()
		defer deviceEventMu.Unlock()
		if e.Type == EventConnected && e.ZoneID != "" {
			deviceConnectedZoneID = e.ZoneID
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start device
	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	// Enter commissioning mode
	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Get device address
	addr := deviceSvc.TLSAddr()
	if addr == nil {
		t.Fatal("Device TLS address is nil - TLS server not started")
	}
	t.Logf("Device listening on %s", addr.String())

	// === Setup Controller ===
	controllerConfig := validControllerConfig()
	controllerConfig.ZoneName = "Home Energy Manager"

	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	browser := newMockBrowser()
	controllerSvc.SetBrowser(browser)

	// Track controller events
	var controllerDeviceID string
	var controllerEventMu sync.Mutex
	controllerSvc.OnEvent(func(e Event) {
		controllerEventMu.Lock()
		defer controllerEventMu.Unlock()
		if e.Type == EventCommissioned && e.DeviceID != "" {
			controllerDeviceID = e.DeviceID
		}
	})

	// Start controller
	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// === Commission Device ===
	port := parseTestPort(addr.String())

	// Simulate discovery result (in real scenario, mDNS would provide this)
	discoveryService := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          port,
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	}

	t.Logf("Commissioning device at %s:%d", discoveryService.Host, discoveryService.Port)

	// Commission with correct setup code
	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	// === Verify Results ===
	if connectedDevice == nil {
		t.Fatal("Commission returned nil device")
	}

	if connectedDevice.ID == "" {
		t.Error("Device ID should not be empty after commissioning")
	}
	t.Logf("Commissioned device ID: %s", connectedDevice.ID)

	// Wait for events to propagate
	time.Sleep(200 * time.Millisecond)

	// Check device side
	deviceEventMu.Lock()
	deviceZoneID := deviceConnectedZoneID
	deviceEventMu.Unlock()

	if deviceZoneID == "" {
		t.Error("Device should have received EventConnected with zone ID")
	} else {
		t.Logf("Device connected zone ID: %s", deviceZoneID)
	}

	// Check controller side
	controllerEventMu.Lock()
	ctrlDeviceID := controllerDeviceID
	controllerEventMu.Unlock()

	if ctrlDeviceID == "" {
		t.Error("Controller should have received EventCommissioned with device ID")
	} else {
		t.Logf("Controller commissioned device ID: %s", ctrlDeviceID)
	}

	// Verify device state
	if deviceSvc.ZoneCount() != 1 {
		t.Errorf("Device should have 1 connected zone, got %d", deviceSvc.ZoneCount())
	}

	zone := deviceSvc.GetZone(deviceZoneID)
	if zone == nil {
		t.Error("Device should have zone record")
	} else if !zone.Connected {
		t.Error("Zone should be marked as connected")
	}

	// Verify controller state
	if controllerSvc.DeviceCount() != 1 {
		t.Errorf("Controller should have 1 device, got %d", controllerSvc.DeviceCount())
	}

	storedDevice := controllerSvc.GetDevice(ctrlDeviceID)
	if storedDevice == nil {
		t.Error("Controller should have device record")
	} else if !storedDevice.Connected {
		t.Error("Device should be marked as connected")
	}
}

// TestE2E_CommissioningFlowWrongCode verifies that commissioning fails with wrong code
// and leaves both sides in a clean state.
func TestE2E_CommissioningFlowWrongCode(t *testing.T) {
	// Setup device
	device := model.NewDevice("evse-002", 0x1234, 0x5678)
	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	// Track if device received any connected events (shouldn't)
	var deviceGotConnected bool
	var deviceEventMu sync.Mutex
	deviceSvc.OnEvent(func(e Event) {
		deviceEventMu.Lock()
		defer deviceEventMu.Unlock()
		if e.Type == EventConnected {
			deviceGotConnected = true
		}
	})

	ctx := context.Background()

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	if addr == nil {
		t.Fatal("Device TLS address is nil")
	}

	// Setup controller
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	browser := newMockBrowser()
	controllerSvc.SetBrowser(browser)

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Create discovery service
	port := parseTestPort(addr.String())

	discoveryService := &discovery.CommissionableService{
		Host:          "localhost",
		Port:          port,
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
	}

	// Attempt commissioning with WRONG code
	_, err = controllerSvc.Commission(ctx, discoveryService, "87654321")
	if err == nil {
		t.Fatal("Expected commissioning to fail with wrong setup code")
	}

	t.Logf("Commission correctly failed: %v", err)

	// Wait for any async events
	time.Sleep(100 * time.Millisecond)

	// Verify device state - should have no zones
	if deviceSvc.ZoneCount() != 0 {
		t.Errorf("Device should have 0 zones after failed commission, got %d", deviceSvc.ZoneCount())
	}

	deviceEventMu.Lock()
	gotConnected := deviceGotConnected
	deviceEventMu.Unlock()

	if gotConnected {
		t.Error("Device should NOT have received EventConnected with wrong setup code")
	}

	// Verify controller state - should have no devices
	if controllerSvc.DeviceCount() != 0 {
		t.Errorf("Controller should have 0 devices after failed commission, got %d", controllerSvc.DeviceCount())
	}
}

// TestE2E_MultipleCommissioning verifies that a device can be commissioned by multiple zones.
func TestE2E_MultipleCommissioning(t *testing.T) {
	// Setup device
	device := model.NewDevice("evse-003", 0x1234, 0x5678)
	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"
	deviceConfig.MaxZones = 5

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx := context.Background()

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	if addr == nil {
		t.Fatal("Device TLS address is nil")
	}

	port := parseTestPort(addr.String())

	discoveryService := &discovery.CommissionableService{
		Host:          "localhost",
		Port:          port,
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
	}

	// Commission from first controller
	controller1Config := validControllerConfig()
	controller1Config.ZoneName = "Home Manager 1"
	controller1, _ := NewControllerService(controller1Config)
	controller1.SetBrowser(newMockBrowser())
	_ = controller1.Start(ctx)
	defer func() { _ = controller1.Stop() }()

	device1, err := controller1.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("First commission failed: %v", err)
	}
	t.Logf("First controller commissioned device: %s", device1.ID)

	// Commission from second controller
	controller2Config := validControllerConfig()
	controller2Config.ZoneName = "Home Manager 2"
	controller2, _ := NewControllerService(controller2Config)
	controller2.SetBrowser(newMockBrowser())
	_ = controller2.Start(ctx)
	defer func() { _ = controller2.Stop() }()

	device2, err := controller2.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Second commission failed: %v", err)
	}
	t.Logf("Second controller commissioned device: %s", device2.ID)

	// Wait for events
	time.Sleep(200 * time.Millisecond)

	// Device should have 2 zones
	if deviceSvc.ZoneCount() != 2 {
		t.Errorf("Device should have 2 zones, got %d", deviceSvc.ZoneCount())
	}

	// Each controller should have 1 device
	if controller1.DeviceCount() != 1 || controller2.DeviceCount() != 1 {
		t.Errorf("Each controller should have 1 device, got %d and %d",
			controller1.DeviceCount(), controller2.DeviceCount())
	}
}

// TestE2E_OperationalRead tests reading attributes after commissioning.
func TestE2E_OperationalRead(t *testing.T) {
	// Setup device with DeviceInfo feature
	device := model.NewDevice("evse-read-001", 0x1234, 0x5678)

	// Add DeviceInfo feature to endpoint 0
	endpoint, _ := device.GetEndpoint(0)
	deviceInfo := model.NewFeature(model.FeatureDeviceInfo, 1)
	endpoint.AddFeature(deviceInfo)

	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	port := parseTestPort(addr.String())

	// Setup and start controller
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}
	controllerSvc.SetBrowser(newMockBrowser())

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Commission device
	discoveryService := &discovery.CommissionableService{
		Host:      "localhost",
		Port:      port,
		Addresses: []string{"127.0.0.1"},
	}

	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	// Wait for connection to be fully established
	time.Sleep(100 * time.Millisecond)

	// Get session for operational messaging
	session := controllerSvc.GetSession(connectedDevice.ID)
	if session == nil {
		t.Fatal("No session found for commissioned device")
	}

	// Read DeviceInfo attributes
	attrs, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if attrs == nil {
		t.Fatal("Expected attributes, got nil")
	}

	t.Logf("Read %d attributes from DeviceInfo", len(attrs))

	// Should have at least global attributes (clusterRevision, featureMap, etc.)
	if len(attrs) == 0 {
		t.Error("Expected at least some attributes")
	}
}

// TestE2E_OperationalSubscribe tests subscribing and receiving priming report.
func TestE2E_OperationalSubscribe(t *testing.T) {
	// Setup device with DeviceInfo feature
	device := model.NewDevice("evse-sub-001", 0x1234, 0x5678)

	endpoint, _ := device.GetEndpoint(0)
	deviceInfo := model.NewFeature(model.FeatureDeviceInfo, 1)
	endpoint.AddFeature(deviceInfo)

	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	port := parseTestPort(addr.String())

	// Setup and start controller
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}
	controllerSvc.SetBrowser(newMockBrowser())

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Commission device
	discoveryService := &discovery.CommissionableService{
		Host:      "localhost",
		Port:      port,
		Addresses: []string{"127.0.0.1"},
	}

	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	session := controllerSvc.GetSession(connectedDevice.ID)
	if session == nil {
		t.Fatal("No session found for commissioned device")
	}

	// Subscribe to DeviceInfo
	subID, priming, err := session.Subscribe(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if subID == 0 {
		t.Error("Expected non-zero subscription ID")
	}

	t.Logf("Subscription ID: %d", subID)

	if priming == nil {
		t.Error("Expected priming report")
	} else {
		t.Logf("Priming report has %d attributes", len(priming))
	}
}

// TestE2E_SubscriptionNotification tests receiving notifications on attribute changes.
func TestE2E_SubscriptionNotification(t *testing.T) {
	// Setup device with a feature that can have its attributes changed
	device := model.NewDevice("evse-notif-001", 0x1234, 0x5678)

	// Add DeviceInfo feature
	endpoint, _ := device.GetEndpoint(0)
	deviceInfo := model.NewFeature(model.FeatureDeviceInfo, 1)
	endpoint.AddFeature(deviceInfo)

	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := newMockAdvertiser()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	port := parseTestPort(addr.String())

	// Setup and start controller
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}
	controllerSvc.SetBrowser(newMockBrowser())

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Commission device
	discoveryService := &discovery.CommissionableService{
		Host:      "localhost",
		Port:      port,
		Addresses: []string{"127.0.0.1"},
	}

	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	session := controllerSvc.GetSession(connectedDevice.ID)
	if session == nil {
		t.Fatal("No session found for commissioned device")
	}

	// Set up notification handler
	var receivedNotif *wire.Notification
	var notifMu sync.Mutex
	notifReceived := make(chan struct{}, 1)

	session.SetNotificationHandler(func(notif *wire.Notification) {
		notifMu.Lock()
		receivedNotif = notif
		notifMu.Unlock()
		select {
		case notifReceived <- struct{}{}:
		default:
		}
	})

	// Subscribe to DeviceInfo
	subID, _, err := session.Subscribe(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	t.Logf("Subscribed with ID %d", subID)

	// NOTE: For a full notification test, we would need to:
	// 1. Change an attribute on the device side
	// 2. Have the device send a notification
	// 3. Verify the controller receives it
	//
	// Since DeviceInfo attributes are mostly read-only and we don't have
	// a notification dispatch mechanism wired up yet, this test verifies
	// that subscription works and handlers can be set up.

	// Verify no spurious notifications were received (none should be sent)
	notifMu.Lock()
	if receivedNotif != nil {
		t.Errorf("Unexpected notification received: %+v", receivedNotif)
	}
	notifMu.Unlock()

	t.Log("Subscription and notification handler setup successful")
}

// TestE2E_NotificationDelivery tests that attribute changes are delivered as notifications.
func TestE2E_NotificationDelivery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create device with Measurement feature
	device := createTestDevice()

	// Add a Measurement feature on endpoint 1 for testing notifications
	ep1 := model.NewEndpoint(1, model.EndpointEVCharger, "EV Charger")
	measurementFeature := model.NewFeature(model.FeatureMeasurement, 1)
	// Add ACActivePower attribute (ID 1)
	measurementFeature.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:      1, // MeasurementAttrACActivePower
		Name:    "ACActivePower",
		Type:    model.DataTypeInt64,
		Access:  model.AccessReadWrite, // Need write access for device-side updates
		Default: int64(0),
	}))
	ep1.AddFeature(measurementFeature)
	_ = device.AddEndpoint(ep1)

	// Create device service
	deviceConfig := validDeviceConfig()
	deviceConfig.Discriminator = 2345 // Max is 4095
	deviceConfig.SetupCode = "12345678"
	deviceConfig.ListenAddress = "127.0.0.1:0"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	addr := deviceSvc.TLSAddr()
	port := addr.(*net.TCPAddr).Port
	t.Logf("Device listening on port %d", port)

	// Enter commissioning mode
	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Create controller
	controllerConfig := DefaultControllerConfig()
	controllerConfig.ZoneName = "Test Controller"

	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}
	controllerSvc.SetBrowser(newMockBrowser())

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Commission device
	discoveryService := &discovery.CommissionableService{
		Host:      "localhost",
		Port:      uint16(port),
		Addresses: []string{"127.0.0.1"},
	}

	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	session := controllerSvc.GetSession(connectedDevice.ID)
	if session == nil {
		t.Fatal("No session found for commissioned device")
	}

	// Set up notification handler
	var receivedNotif *wire.Notification
	var notifMu sync.Mutex
	notifReceived := make(chan struct{}, 1)

	session.SetNotificationHandler(func(notif *wire.Notification) {
		notifMu.Lock()
		receivedNotif = notif
		notifMu.Unlock()
		select {
		case notifReceived <- struct{}{}:
		default:
		}
	})

	// Subscribe to Measurement feature on endpoint 1
	subID, priming, err := session.Subscribe(ctx, 1, uint8(model.FeatureMeasurement), nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	t.Logf("Subscribed with ID %d, priming: %v", subID, priming)

	// Trigger an attribute change on the device side
	const attrACActivePower = uint16(1)
	newPower := int64(5000000) // 5 kW

	if err := deviceSvc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrACActivePower, newPower); err != nil {
		t.Fatalf("NotifyAttributeChange failed: %v", err)
	}

	// Wait for notification to arrive
	select {
	case <-notifReceived:
		notifMu.Lock()
		if receivedNotif == nil {
			t.Fatal("Notification received signal but notif is nil")
		}
		if receivedNotif.SubscriptionID != subID {
			t.Errorf("Expected subscription ID %d, got %d", subID, receivedNotif.SubscriptionID)
		}
		if receivedNotif.EndpointID != 1 {
			t.Errorf("Expected endpoint ID 1, got %d", receivedNotif.EndpointID)
		}
		if receivedNotif.FeatureID != uint8(model.FeatureMeasurement) {
			t.Errorf("Expected feature ID %d, got %d", model.FeatureMeasurement, receivedNotif.FeatureID)
		}
		if val, ok := receivedNotif.Changes[attrACActivePower]; !ok {
			t.Error("Notification missing ACActivePower attribute")
		} else {
			t.Logf("Received notification: power = %v", val)
		}
		notifMu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for notification")
	}

	t.Log("Notification delivery test passed")
}

package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Test helpers

// makeCommissionableChannel creates a channel that emits the given services then closes.
func makeCommissionableChannel(services ...*discovery.CommissionableService) <-chan *discovery.CommissionableService {
	ch := make(chan *discovery.CommissionableService)
	go func() {
		defer close(ch)
		for _, svc := range services {
			ch <- svc
		}
	}()
	return ch
}

// makeCommissionableChannels creates added and removed channels for BrowseCommissionable mock.
// The added channel emits the given services, the removed channel is empty and closed.
func makeCommissionableChannels(services ...*discovery.CommissionableService) (<-chan *discovery.CommissionableService, <-chan *discovery.CommissionableService) {
	added := makeCommissionableChannel(services...)
	removed := make(chan *discovery.CommissionableService)
	close(removed)
	return added, removed
}

// makeOperationalChannel creates a channel that emits the given services then closes.
func makeOperationalChannel(services ...*discovery.OperationalService) <-chan *discovery.OperationalService {
	ch := make(chan *discovery.OperationalService)
	go func() {
		defer close(ch)
		for _, svc := range services {
			ch <- svc
		}
	}()
	return ch
}

// Test helpers

func validDeviceConfig() DeviceConfig {
	config := DefaultDeviceConfig()
	config.ListenAddress = "127.0.0.1:0" // Use dynamic port to avoid conflicts
	config.Discriminator = 1234
	config.SetupCode = "12345678"
	config.SerialNumber = "SN001"
	config.Brand = "TestBrand"
	config.Model = "TestModel"
	config.Categories = []discovery.DeviceCategory{discovery.CategoryEMobility}
	return config
}

func validControllerConfig() ControllerConfig {
	config := DefaultControllerConfig()
	config.ZoneName = "Test Zone"
	config.ZoneType = cert.ZoneTypeLocal
	return config
}

// DeviceService tests

func TestNewDeviceService(t *testing.T) {
	device := model.NewDevice("test-device-001", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	if svc.State() != StateIdle {
		t.Errorf("expected state IDLE, got %v", svc.State())
	}

	if svc.Device() != device {
		t.Error("Device() returned wrong device")
	}
}

func TestDeviceServiceInvalidConfig(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := DeviceConfig{} // Invalid - missing required fields

	_, err := NewDeviceService(device, config)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestDeviceServiceStartStop(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	ctx := context.Background()

	// Start
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if svc.State() != StateRunning {
		t.Errorf("expected state RUNNING, got %v", svc.State())
	}

	// Start again should fail
	if err := svc.Start(ctx); err != ErrAlreadyStarted {
		t.Errorf("expected ErrAlreadyStarted, got %v", err)
	}

	// Stop
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if svc.State() != StateStopped {
		t.Errorf("expected state STOPPED, got %v", svc.State())
	}

	// Stop again should fail
	if err := svc.Stop(); err != ErrNotStarted {
		t.Errorf("expected ErrNotStarted, got %v", err)
	}
}

func TestDeviceServiceZoneManagement(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Initial state
	if svc.ZoneCount() != 0 {
		t.Errorf("expected 0 zones, got %d", svc.ZoneCount())
	}

	// Connect a zone
	zoneID := "zone-001"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	if svc.ZoneCount() != 1 {
		t.Errorf("expected 1 zone, got %d", svc.ZoneCount())
	}

	zone := svc.GetZone(zoneID)
	if zone == nil {
		t.Fatal("GetZone returned nil")
	}

	if zone.ID != zoneID {
		t.Errorf("expected zone ID %s, got %s", zoneID, zone.ID)
	}

	if zone.Type != cert.ZoneTypeLocal {
		t.Errorf("expected zone type HomeManager, got %v", zone.Type)
	}

	if !zone.Connected {
		t.Error("expected zone to be connected")
	}

	// Disconnect zone
	svc.HandleZoneDisconnect(zoneID)

	zone = svc.GetZone(zoneID)
	if zone == nil {
		t.Fatal("GetZone returned nil after disconnect")
	}

	if zone.Connected {
		t.Error("expected zone to be disconnected")
	}

	// GetAllZones
	zones := svc.GetAllZones()
	if len(zones) != 1 {
		t.Errorf("expected 1 zone, got %d", len(zones))
	}
}

func TestDeviceServiceEventHandlers(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	var receivedEvents []Event
	var mu sync.Mutex

	svc.OnEvent(func(e Event) {
		mu.Lock()
		receivedEvents = append(receivedEvents, e)
		mu.Unlock()
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Trigger events
	svc.HandleZoneConnect("zone-001", cert.ZoneTypeLocal)
	svc.HandleZoneDisconnect("zone-001")

	// Wait for async handlers
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(receivedEvents))
	}

	// Check event types
	hasConnect := false
	hasDisconnect := false
	for _, e := range receivedEvents {
		if e.Type == EventConnected {
			hasConnect = true
		}
		if e.Type == EventDisconnected {
			hasDisconnect = true
		}
	}

	if !hasConnect {
		t.Error("expected EventConnected")
	}
	if !hasDisconnect {
		t.Error("expected EventDisconnected")
	}
}

func TestDeviceServiceCommissioningMode(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Exit commissioning mode
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Mock expectations are automatically verified by NewMockAdvertiser(t)
}

// ControllerService tests

func TestNewControllerService(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	if svc.State() != StateIdle {
		t.Errorf("expected state IDLE, got %v", svc.State())
	}

	if svc.ZoneName() != config.ZoneName {
		t.Errorf("expected zone name %s, got %s", config.ZoneName, svc.ZoneName())
	}
}

func TestControllerServiceInvalidConfig(t *testing.T) {
	config := ControllerConfig{} // Invalid - missing ZoneName

	_, err := NewControllerService(config)
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestControllerServiceStartStop(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	ctx := context.Background()

	// Start
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if svc.State() != StateRunning {
		t.Errorf("expected state RUNNING, got %v", svc.State())
	}

	// Start again should fail
	if err := svc.Start(ctx); err != ErrAlreadyStarted {
		t.Errorf("expected ErrAlreadyStarted, got %v", err)
	}

	// Stop
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if svc.State() != StateStopped {
		t.Errorf("expected state STOPPED, got %v", svc.State())
	}
}

func TestControllerServiceDeviceManagement(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Initial state
	if svc.DeviceCount() != 0 {
		t.Errorf("expected 0 devices, got %d", svc.DeviceCount())
	}

	// Manually add a device (simulating commissioning)
	svc.mu.Lock()
	deviceID := "device-001"
	svc.connectedDevices[deviceID] = &ConnectedDevice{
		ID:        deviceID,
		Host:      "evse-001.local",
		Port:      8443,
		Connected: true,
		LastSeen:  time.Now(),
	}
	svc.mu.Unlock()

	if svc.DeviceCount() != 1 {
		t.Errorf("expected 1 device, got %d", svc.DeviceCount())
	}

	device := svc.GetDevice(deviceID)
	if device == nil {
		t.Fatal("GetDevice returned nil")
	}

	if device.ID != deviceID {
		t.Errorf("expected device ID %s, got %s", deviceID, device.ID)
	}

	// GetAllDevices
	devices := svc.GetAllDevices()
	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}

	// Decommission
	if err := svc.Decommission(deviceID); err != nil {
		t.Fatalf("Decommission failed: %v", err)
	}

	if svc.DeviceCount() != 0 {
		t.Errorf("expected 0 devices after decommission, got %d", svc.DeviceCount())
	}

	// Decommission non-existent device
	if err := svc.Decommission("non-existent"); err != ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}
}

func TestControllerServiceDiscovery(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Test data
	device1 := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "evse-001.local",
		Port:          8443,
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	}
	device2 := &discovery.CommissionableService{
		InstanceName:  "MASH-5678",
		Host:          "inverter-001.local",
		Port:          8443,
		Discriminator: 5678,
		Categories:    []discovery.DeviceCategory{discovery.CategoryInverter},
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	// BrowseCommissionable is called multiple times (once per Discover call)
	// Use RunAndReturn to create a fresh channel for each call (channels can only be consumed once)
	browser.EXPECT().BrowseCommissionable(mock.Anything).
		RunAndReturn(func(_ context.Context) (<-chan *discovery.CommissionableService, <-chan *discovery.CommissionableService, error) {
			added, removed := makeCommissionableChannels(device1, device2)
			return added, removed, nil
		}).Times(2)
	browser.EXPECT().FindByDiscriminator(mock.Anything, uint16(5678)).
		Return(device2, nil).Once()
	browser.EXPECT().FindByDiscriminator(mock.Anything, uint16(9999)).
		Return(nil, discovery.ErrNotFound).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Discover all devices
	devices, err := svc.Discover(ctx, nil)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}

	// Discover with filter
	devices, err = svc.Discover(ctx, discovery.FilterByCategory(discovery.CategoryEMobility))
	if err != nil {
		t.Fatalf("Discover with filter failed: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("expected 1 device with filter, got %d", len(devices))
	}

	if devices[0].Discriminator != 1234 {
		t.Errorf("expected discriminator 1234, got %d", devices[0].Discriminator)
	}

	// Find by discriminator
	device, err := svc.DiscoverByDiscriminator(ctx, 5678)
	if err != nil {
		t.Fatalf("DiscoverByDiscriminator failed: %v", err)
	}

	if device.Discriminator != 5678 {
		t.Errorf("expected discriminator 5678, got %d", device.Discriminator)
	}

	// Find non-existent discriminator
	_, err = svc.DiscoverByDiscriminator(ctx, 9999)
	if err != discovery.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// Config validation tests

func TestDeviceConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*DeviceConfig)
		wantErr bool
	}{
		{
			name:    "valid config",
			modify:  func(_ *DeviceConfig) {},
			wantErr: false,
		},
		{
			name: "invalid discriminator",
			modify: func(c *DeviceConfig) {
				c.Discriminator = 5000 // > MaxDiscriminator (4095)
			},
			wantErr: true,
		},
		{
			name: "invalid setup code length",
			modify: func(c *DeviceConfig) {
				c.SetupCode = "1234" // Not 8 digits
			},
			wantErr: true,
		},
		{
			name: "missing serial number",
			modify: func(c *DeviceConfig) {
				c.SerialNumber = ""
			},
			wantErr: true,
		},
		{
			name: "missing brand",
			modify: func(c *DeviceConfig) {
				c.Brand = ""
			},
			wantErr: true,
		},
		{
			name: "missing model",
			modify: func(c *DeviceConfig) {
				c.Model = ""
			},
			wantErr: true,
		},
		{
			name: "no categories",
			modify: func(c *DeviceConfig) {
				c.Categories = nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validDeviceConfig()
			tt.modify(&config)

			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestControllerConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ControllerConfig)
		wantErr bool
	}{
		{
			name:    "valid config",
			modify:  func(_ *ControllerConfig) {},
			wantErr: false,
		},
		{
			name: "missing zone name",
			modify: func(c *ControllerConfig) {
				c.ZoneName = ""
			},
			wantErr: true,
		},
		{
			name: "invalid zone type - too low",
			modify: func(c *ControllerConfig) {
				c.ZoneType = 0 // Invalid (below GridOperator)
			},
			wantErr: true,
		},
		{
			name: "invalid zone type - too high",
			modify: func(c *ControllerConfig) {
				c.ZoneType = 10 // Invalid (above UserApp)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validControllerConfig()
			tt.modify(&config)

			err := config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Operational mDNS advertising tests

func TestDeviceServiceOperationalAdvertisingOnZoneConnect(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - capture the operational info for verification
	var capturedOpInfo *discovery.OperationalInfo
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).
		Run(func(_ context.Context, info *discovery.OperationalInfo) {
			capturedOpInfo = info
		}).Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Simulate zone connect (as happens after commissioning)
	zoneID := "a1b2c3d4e5f6a7b8"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Verify operational advertising was called with correct zone ID
	if capturedOpInfo == nil {
		t.Fatal("expected operational advertising to start after zone connect")
	}

	if capturedOpInfo.ZoneID != zoneID {
		t.Errorf("expected zone ID %s, got %s", zoneID, capturedOpInfo.ZoneID)
	}

	// Device ID should be set (derived from certificate fingerprint)
	if capturedOpInfo.DeviceID == "" {
		t.Error("expected device ID to be set in operational info")
	}
}

func TestDeviceServiceOperationalAdvertisingPersistsOnDisconnect(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - we expect AdvertiseOperational but NOT StopOperational
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Once()
	// Note: We explicitly do NOT expect StopOperational to be called on disconnect
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Simulate zone connect
	zoneID := "a1b2c3d4e5f6a7b8"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Simulate disconnect - operational advertising should PERSIST
	svc.HandleZoneDisconnect(zoneID)

	// If StopOperational was called, the mock would fail the test
}

func TestDeviceServiceMultipleZonesOperationalAdvertising(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - expect two AdvertiseOperational calls
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Times(2)
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect multiple zones
	zone1 := "zone1111111111111"
	zone2 := "zone2222222222222"

	svc.HandleZoneConnect(zone1, cert.ZoneTypeLocal)
	svc.HandleZoneConnect(zone2, cert.ZoneTypeGrid)

	// Mock expectations verify both zones were advertised
}

func TestDeviceServiceOperationalAdvertisingInfo(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.Brand = "TestBrand"
	config.Model = "TestModel"

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - capture the operational info for verification
	var capturedOpInfo *discovery.OperationalInfo
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).
		Run(func(_ context.Context, info *discovery.OperationalInfo) {
			capturedOpInfo = info
		}).Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Simulate zone connect
	zoneID := "a1b2c3d4e5f6a7b8"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Verify operational info contains expected fields
	if capturedOpInfo == nil {
		t.Fatal("expected operational info to exist")
	}

	// Check vendor:product ID format
	expectedVP := "1234:5678" // From device vendor/product IDs
	if capturedOpInfo.VendorProduct != expectedVP {
		t.Errorf("expected vendor:product %s, got %s", expectedVP, capturedOpInfo.VendorProduct)
	}

	// Endpoint count should match device
	if capturedOpInfo.EndpointCount != uint8(device.EndpointCount()) {
		t.Errorf("expected endpoint count %d, got %d", device.EndpointCount(), capturedOpInfo.EndpointCount)
	}
}

// Streaming discovery tests

func TestControllerServiceStartDiscovery(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Test data
	device1 := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "evse-001.local",
		Port:          8443,
		Discriminator: 1234,
	}
	device2 := &discovery.CommissionableService{
		InstanceName:  "MASH-5678",
		Host:          "inverter-001.local",
		Port:          8443,
		Discriminator: 5678,
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	// Use RunAndReturn to create a fresh channel each time (channels can only be consumed once)
	browser.EXPECT().BrowseCommissionable(mock.Anything).
		RunAndReturn(func(_ context.Context) (<-chan *discovery.CommissionableService, <-chan *discovery.CommissionableService, error) {
			added, removed := makeCommissionableChannels(device1, device2)
			return added, removed, nil
		}).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Track discovered devices via events
	var discoveredDevices []*discovery.CommissionableService
	var mu sync.Mutex
	svc.OnEvent(func(e Event) {
		if e.Type == EventDeviceDiscovered {
			mu.Lock()
			if svc, ok := e.DiscoveredService.(*discovery.CommissionableService); ok {
				discoveredDevices = append(discoveredDevices, svc)
			}
			mu.Unlock()
		}
	})

	// Start background discovery
	if err := svc.StartDiscovery(ctx, nil); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}

	// Wait for devices to be discovered
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(discoveredDevices)
	mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 discovered devices via events, got %d", count)
	}

	// Stop discovery
	svc.StopDiscovery()
}

func TestControllerServiceStartDiscoveryWithFilter(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Test data - both devices returned, filter applied by service
	device1 := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	}
	device2 := &discovery.CommissionableService{
		InstanceName:  "MASH-5678",
		Discriminator: 5678,
		Categories:    []discovery.DeviceCategory{discovery.CategoryInverter},
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	// Use RunAndReturn to create a fresh channel each time (channels can only be consumed once)
	browser.EXPECT().BrowseCommissionable(mock.Anything).
		RunAndReturn(func(_ context.Context) (<-chan *discovery.CommissionableService, <-chan *discovery.CommissionableService, error) {
			added, removed := makeCommissionableChannels(device1, device2)
			return added, removed, nil
		}).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Track discovered devices
	var discoveredDevices []*discovery.CommissionableService
	var mu sync.Mutex
	svc.OnEvent(func(e Event) {
		if e.Type == EventDeviceDiscovered {
			mu.Lock()
			if svc, ok := e.DiscoveredService.(*discovery.CommissionableService); ok {
				discoveredDevices = append(discoveredDevices, svc)
			}
			mu.Unlock()
		}
	})

	// Start discovery with filter - only EMobility
	filter := discovery.FilterByCategory(discovery.CategoryEMobility)
	if err := svc.StartDiscovery(ctx, filter); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(discoveredDevices)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 filtered device, got %d", count)
	}

	svc.StopDiscovery()
}

func TestControllerServiceStopDiscovery(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	// Create channels that stay open until we close them
	// This simulates a real browser that keeps the channels open for discovery
	discoveryCh := make(chan *discovery.CommissionableService)
	removedCh := make(chan *discovery.CommissionableService)
	browser.EXPECT().BrowseCommissionable(mock.Anything).
		RunAndReturn(func(_ context.Context) (<-chan *discovery.CommissionableService, <-chan *discovery.CommissionableService, error) {
			return discoveryCh, removedCh, nil
		}).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		close(discoveryCh) // Clean up the channels
		close(removedCh)
		_ = svc.Stop()
	}()

	// Start discovery
	if err := svc.StartDiscovery(ctx, nil); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}

	// Wait for the discovery goroutine to start and call BrowseCommissionable
	time.Sleep(50 * time.Millisecond)

	if !svc.IsDiscovering() {
		t.Error("expected IsDiscovering to be true")
	}

	// Stop discovery
	svc.StopDiscovery()

	// Give the goroutine time to process the cancellation
	time.Sleep(50 * time.Millisecond)

	if svc.IsDiscovering() {
		t.Error("expected IsDiscovering to be false after StopDiscovery")
	}
}

func TestEventDeviceDiscoveredString(t *testing.T) {
	if EventDeviceDiscovered.String() != "DEVICE_DISCOVERED" {
		t.Errorf("expected DEVICE_DISCOVERED, got %s", EventDeviceDiscovered.String())
	}
}

// TestControllerOperationalDiscoveryMatchesDeviceID verifies that operational discovery
// correctly matches devices by device ID. This is a unit test for the bug where
// device IDs were derived differently on controller vs device side.
func TestControllerOperationalDiscoveryMatchesDeviceID(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Test data
	knownDeviceID := "abc123def456789a"
	opDevice := &discovery.OperationalService{
		InstanceName: "zone123456789abc-abc123def456789a",
		Host:         "device.local",
		Port:         8443,
		ZoneID:       "zone123456789abc",
		DeviceID:     knownDeviceID,
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowseOperational(mock.Anything, "zone123456789abc").
		Return(makeOperationalChannel(opDevice), nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Manually set zone ID (normally set during commissioning)
	svc.mu.Lock()
	svc.zoneID = "zone123456789abc"
	svc.mu.Unlock()

	// Simulate a known device that was previously commissioned
	svc.mu.Lock()
	svc.connectedDevices[knownDeviceID] = &ConnectedDevice{
		ID:        knownDeviceID,
		Host:      "device.local",
		Port:      8443,
		Connected: false, // Currently disconnected
		LastSeen:  time.Now().Add(-1 * time.Minute),
	}
	svc.mu.Unlock()

	// Track events
	var rediscoveredDeviceID string
	var mu sync.Mutex
	eventReceived := make(chan struct{}, 1)
	svc.OnEvent(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Type == EventDeviceRediscovered {
			rediscoveredDeviceID = e.DeviceID
			select {
			case eventReceived <- struct{}{}:
			default:
			}
		}
	})

	// Start operational discovery
	if err := svc.StartOperationalDiscovery(ctx); err != nil {
		t.Fatalf("StartOperationalDiscovery failed: %v", err)
	}
	defer svc.StopOperationalDiscovery()

	// Wait for rediscovery event
	select {
	case <-eventReceived:
		mu.Lock()
		gotID := rediscoveredDeviceID
		mu.Unlock()

		if gotID != knownDeviceID {
			t.Errorf("expected rediscovered device ID %s, got %s", knownDeviceID, gotID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for EventDeviceRediscovered - device ID matching failed")
	}
}

// TestControllerOperationalDiscoveryIgnoresUnknownDevices verifies that operational
// discovery ignores devices that aren't in the controller's known device list.
func TestControllerOperationalDiscoveryIgnoresUnknownDevices(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Test data - device with unknown ID
	unknownDevice := &discovery.OperationalService{
		InstanceName: "zone123456789abc-unknowndevice123",
		Host:         "unknown.local",
		Port:         8443,
		ZoneID:       "zone123456789abc",
		DeviceID:     "unknowndevice123",
	}

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowseOperational(mock.Anything, "zone123456789abc").
		Return(makeOperationalChannel(unknownDevice), nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "zone123456789abc"
	svc.mu.Unlock()

	// Add a known device with one ID
	knownDeviceID := "known1234567890a"
	svc.mu.Lock()
	svc.connectedDevices[knownDeviceID] = &ConnectedDevice{
		ID:        knownDeviceID,
		Connected: false,
	}
	svc.mu.Unlock()

	// Track events
	var gotRediscovered bool
	var mu sync.Mutex
	svc.OnEvent(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		if e.Type == EventDeviceRediscovered {
			gotRediscovered = true
		}
	})

	// Start operational discovery
	if err := svc.StartOperationalDiscovery(ctx); err != nil {
		t.Fatalf("StartOperationalDiscovery failed: %v", err)
	}

	// Wait a bit - should NOT receive rediscovery event
	time.Sleep(200 * time.Millisecond)

	svc.StopOperationalDiscovery()

	mu.Lock()
	gotEvent := gotRediscovered
	mu.Unlock()

	if gotEvent {
		t.Error("should NOT have received EventDeviceRediscovered for unknown device")
	}
}

// State and event String() tests

func TestServiceStateString(t *testing.T) {
	tests := []struct {
		state ServiceState
		want  string
	}{
		{StateIdle, "IDLE"},
		{StateStarting, "STARTING"},
		{StateRunning, "RUNNING"},
		{StateStopping, "STOPPING"},
		{StateStopped, "STOPPED"},
		{ServiceState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("ServiceState(%d).String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		event EventType
		want  string
	}{
		{EventConnected, "CONNECTED"},
		{EventDisconnected, "DISCONNECTED"},
		{EventCommissioned, "COMMISSIONED"},
		{EventDecommissioned, "DECOMMISSIONED"},
		{EventValueChanged, "VALUE_CHANGED"},
		{EventFailsafeStarted, "FAILSAFE_STARTED"},
		{EventFailsafeTriggered, "FAILSAFE_TRIGGERED"},
		{EventFailsafeCleared, "FAILSAFE_CLEARED"},
		{EventCommissioningOpened, "COMMISSIONING_OPENED"},
		{EventCommissioningClosed, "COMMISSIONING_CLOSED"},
		{EventDeviceDiscovered, "DEVICE_DISCOVERED"},
		{EventZoneRemoved, "ZONE_REMOVED"},
		{EventZoneRestored, "ZONE_RESTORED"},
		{EventDeviceRediscovered, "DEVICE_REDISCOVERED"},
		{EventDeviceReconnected, "DEVICE_RECONNECTED"},
		{EventType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.event.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %s, want %s", tt.event, got, tt.want)
		}
	}
}

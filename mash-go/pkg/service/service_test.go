package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// mockAdvertiser is a test double for discovery.Advertiser.
type mockAdvertiser struct {
	mu                      sync.Mutex
	commissionableInfo      *discovery.CommissionableInfo
	operationalZones        map[string]*discovery.OperationalInfo
	commissionerZones       map[string]*discovery.CommissionerInfo
	isCommissioningActive   bool
}

func newMockAdvertiser() *mockAdvertiser {
	return &mockAdvertiser{
		operationalZones:  make(map[string]*discovery.OperationalInfo),
		commissionerZones: make(map[string]*discovery.CommissionerInfo),
	}
}

func (m *mockAdvertiser) AdvertiseCommissionable(_ context.Context, info *discovery.CommissionableInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionableInfo = info
	m.isCommissioningActive = true
	return nil
}

func (m *mockAdvertiser) StopCommissionable() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isCommissioningActive = false
	return nil
}

func (m *mockAdvertiser) AdvertiseOperational(_ context.Context, info *discovery.OperationalInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.operationalZones[info.ZoneID] = info
	return nil
}

func (m *mockAdvertiser) UpdateOperational(zoneID string, info *discovery.OperationalInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.operationalZones[zoneID] = info
	return nil
}

func (m *mockAdvertiser) StopOperational(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.operationalZones, zoneID)
	return nil
}

func (m *mockAdvertiser) AdvertiseCommissioner(_ context.Context, info *discovery.CommissionerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionerZones[info.ZoneID] = info
	return nil
}

func (m *mockAdvertiser) UpdateCommissioner(zoneID string, info *discovery.CommissionerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionerZones[zoneID] = info
	return nil
}

func (m *mockAdvertiser) StopCommissioner(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.commissionerZones, zoneID)
	return nil
}

func (m *mockAdvertiser) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionableInfo = nil
	m.isCommissioningActive = false
	m.operationalZones = make(map[string]*discovery.OperationalInfo)
	m.commissionerZones = make(map[string]*discovery.CommissionerInfo)
}

// mockBrowser is a test double for discovery.Browser.
type mockBrowser struct {
	mu                    sync.Mutex
	commissionableDevices []*discovery.CommissionableService
	operationalDevices    []*discovery.OperationalService
	commissioners         []*discovery.CommissionerService
}

func newMockBrowser() *mockBrowser {
	return &mockBrowser{}
}

func (m *mockBrowser) BrowseCommissionable(_ context.Context) (<-chan *discovery.CommissionableService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *discovery.CommissionableService)
	go func() {
		defer close(ch)
		for _, svc := range m.commissionableDevices {
			ch <- svc
		}
	}()
	return ch, nil
}

func (m *mockBrowser) BrowseOperational(_ context.Context, _ string) (<-chan *discovery.OperationalService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *discovery.OperationalService)
	go func() {
		defer close(ch)
		for _, svc := range m.operationalDevices {
			ch <- svc
		}
	}()
	return ch, nil
}

func (m *mockBrowser) BrowseCommissioners(_ context.Context) (<-chan *discovery.CommissionerService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan *discovery.CommissionerService)
	go func() {
		defer close(ch)
		for _, svc := range m.commissioners {
			ch <- svc
		}
	}()
	return ch, nil
}

func (m *mockBrowser) FindByDiscriminator(_ context.Context, discriminator uint16) (*discovery.CommissionableService, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, svc := range m.commissionableDevices {
		if svc.Discriminator == discriminator {
			return svc, nil
		}
	}
	return nil, discovery.ErrNotFound
}

func (m *mockBrowser) Stop() {}

func (m *mockBrowser) AddDevice(svc *discovery.CommissionableService) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionableDevices = append(m.commissionableDevices, svc)
}

// Test helpers

func validDeviceConfig() DeviceConfig {
	config := DefaultDeviceConfig()
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
	config.ZoneType = cert.ZoneTypeHomeManager
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
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeHomeManager)

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

	if zone.Type != cert.ZoneTypeHomeManager {
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
	svc.HandleZoneConnect("zone-001", cert.ZoneTypeHomeManager)
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
	advertiser := newMockAdvertiser()
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

	advertiser.mu.Lock()
	active := advertiser.isCommissioningActive
	advertiser.mu.Unlock()

	if !active {
		t.Error("expected commissioning to be active")
	}

	// Exit commissioning mode
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	advertiser.mu.Lock()
	active = advertiser.isCommissioningActive
	advertiser.mu.Unlock()

	if active {
		t.Error("expected commissioning to be inactive")
	}
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

	// Set up mock browser
	browser := newMockBrowser()
	browser.AddDevice(&discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "evse-001.local",
		Port:          8443,
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	})
	browser.AddDevice(&discovery.CommissionableService{
		InstanceName:  "MASH-5678",
		Host:          "inverter-001.local",
		Port:          8443,
		Discriminator: 5678,
		Categories:    []discovery.DeviceCategory{discovery.CategoryInverter},
	})
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
		{EventType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.event.String(); got != tt.want {
			t.Errorf("EventType(%d).String() = %s, want %s", tt.event, got, tt.want)
		}
	}
}

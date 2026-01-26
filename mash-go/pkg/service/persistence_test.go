package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/persistence"
)

func TestDeviceServiceSetStateStore(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	dir := t.TempDir()
	store := persistence.NewDeviceStateStore(filepath.Join(dir, "state.json"))

	svc.SetStateStore(store)

	// Verify it's set
	if svc.stateStore != store {
		t.Error("state store not set correctly")
	}
}

func TestDeviceServiceSaveLoadState(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser and start
	advertiser := newMockAdvertiser()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect a zone
	zoneID := "zone-persist-001"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeHomeManager)

	// Set up persistence
	dir := t.TempDir()
	store := persistence.NewDeviceStateStore(filepath.Join(dir, "state.json"))
	svc.SetStateStore(store)

	// Save state
	if err := svc.SaveState(); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Create new service and load state
	svc2, err := NewDeviceService(model.NewDevice("test-device", 0x1234, 0x5678), config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}
	svc2.SetStateStore(store)

	if err := svc2.LoadState(); err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	// Verify zone index was restored
	svc2.mu.RLock()
	idx, exists := svc2.zoneIndexMap[zoneID]
	svc2.mu.RUnlock()

	if !exists {
		t.Error("zone index map not restored")
	}
	if idx != 0 {
		t.Errorf("zone index = %d, want 0", idx)
	}
}

func TestDeviceServiceSaveStateWithFailsafeTimer(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser and start
	advertiser := newMockAdvertiser()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect a zone with a failsafe timer
	zoneID := "zone-failsafe-001"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeHomeManager)

	// Set failsafe timer using the existing method (this starts the timer)
	timer := svc.GetFailsafeTimer(zoneID)
	if timer == nil {
		t.Fatal("failsafe timer not created")
	}

	// Verify timer is running
	if timer.State() != failsafe.StateTimerRunning {
		t.Errorf("timer state = %v, want TIMER_RUNNING", timer.State())
	}

	// Set up persistence
	dir := t.TempDir()
	store := persistence.NewDeviceStateStore(filepath.Join(dir, "state.json"))
	svc.SetStateStore(store)

	// Save state
	if err := svc.SaveState(); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Load and verify failsafe state was persisted
	state, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load() error = %v", err)
	}

	if len(state.FailsafeState) != 1 {
		t.Errorf("expected 1 failsafe state, got %d", len(state.FailsafeState))
	}

	snap, exists := state.FailsafeState[zoneID]
	if !exists {
		t.Fatal("failsafe state for zone not found")
	}

	if snap.State != uint8(failsafe.StateTimerRunning) {
		t.Errorf("failsafe state = %d, want %d (TIMER_RUNNING)", snap.State, failsafe.StateTimerRunning)
	}
}

func TestDeviceServiceLoadStateNoStore(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// LoadState without store should be no-op (not error)
	if err := svc.LoadState(); err != nil {
		t.Errorf("LoadState() without store should not error, got %v", err)
	}
}

func TestDeviceServiceSaveStateNoStore(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// SaveState without store should be no-op (not error)
	if err := svc.SaveState(); err != nil {
		t.Errorf("SaveState() without store should not error, got %v", err)
	}
}

func TestControllerServiceSetStateStore(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	dir := t.TempDir()
	store := persistence.NewControllerStateStore(filepath.Join(dir, "state.json"))

	svc.SetStateStore(store)

	if svc.stateStore != store {
		t.Error("state store not set correctly")
	}
}

func TestControllerServiceSaveLoadState(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// Set up mock browser and start
	browser := newMockBrowser()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Set zone ID and add a device
	svc.mu.Lock()
	svc.zoneID = "my-zone-123"
	deviceID := "device-001"
	svc.connectedDevices[deviceID] = &ConnectedDevice{
		ID:        deviceID,
		Host:      "evse-001.local",
		Port:      8443,
		Connected: true,
		LastSeen:  time.Now(),
	}
	svc.mu.Unlock()

	// Set up persistence
	dir := t.TempDir()
	store := persistence.NewControllerStateStore(filepath.Join(dir, "state.json"))
	svc.SetStateStore(store)

	// Save state
	if err := svc.SaveState(); err != nil {
		t.Fatalf("SaveState() error = %v", err)
	}

	// Create new service and load state
	svc2, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}
	svc2.SetStateStore(store)

	if err := svc2.LoadState(); err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	// Verify zone ID was restored
	if svc2.ZoneID() != "my-zone-123" {
		t.Errorf("zone ID = %q, want %q", svc2.ZoneID(), "my-zone-123")
	}

	// Verify device list was restored
	if svc2.DeviceCount() != 1 {
		t.Errorf("device count = %d, want 1", svc2.DeviceCount())
	}
}

func TestControllerServiceLoadStateNoStore(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// LoadState without store should be no-op
	if err := svc.LoadState(); err != nil {
		t.Errorf("LoadState() without store should not error, got %v", err)
	}
}

func TestControllerServiceSaveStateNoStore(t *testing.T) {
	config := validControllerConfig()

	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	// SaveState without store should be no-op
	if err := svc.SaveState(); err != nil {
		t.Errorf("SaveState() without store should not error, got %v", err)
	}
}

package service

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/stretchr/testify/mock"
)

// createTestDeviceService creates a DeviceService for testing.
func createTestDeviceService(t *testing.T) *DeviceService {
	t.Helper()

	evse := examples.NewEVSE(examples.EVSEConfig{
		DeviceID:           "test-evse-123",
		VendorName:         "Test Vendor",
		ProductName:        "Test EVSE",
		SerialNumber:       "SN-12345",
		VendorID:           0x1234,
		ProductID:          0x0001,
		PhaseCount:         3,
		NominalVoltage:     230,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1380000,
	})

	cfg := DefaultDeviceConfig()
	cfg.ListenAddress = ":0" // Use dynamic port
	cfg.Discriminator = 1234
	cfg.SetupCode = "12345678"
	cfg.SerialNumber = "SN-12345"
	cfg.Brand = "Test Vendor"
	cfg.Model = "Test EVSE"
	cfg.Categories = []discovery.DeviceCategory{discovery.CategoryEMobility}
	cfg.FailsafeTimeout = 100 * time.Millisecond // Short for testing

	svc, err := NewDeviceService(evse.Device(), cfg)
	if err != nil {
		t.Fatalf("Failed to create DeviceService: %v", err)
	}

	return svc
}

func TestDeviceServiceRemoveZone(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Simulate a zone connection
	zoneID := "test-zone-001"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Verify zone is connected
	zone := svc.GetZone(zoneID)
	if zone == nil {
		t.Fatal("Zone should exist after HandleZoneConnect")
	}
	if !zone.Connected {
		t.Error("Zone should be connected")
	}

	// Remove the zone
	err := svc.RemoveZone(zoneID)
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Verify zone is gone
	zone = svc.GetZone(zoneID)
	if zone != nil {
		t.Error("Zone should not exist after RemoveZone")
	}

	// Verify zone count
	if svc.ZoneCount() != 0 {
		t.Errorf("ZoneCount = %d, want 0", svc.ZoneCount())
	}
}

func TestDeviceServiceRemoveZoneNotFound(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Try to remove non-existent zone
	err := svc.RemoveZone("non-existent-zone")
	if err == nil {
		t.Error("RemoveZone should return error for non-existent zone")
	}
}

func TestDeviceServiceRemoveZoneWithFailsafe(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Connect zone
	zoneID := "test-zone-002"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Note: Failsafe timer is not created automatically if config duration is invalid
	// (MinDuration is 2 hours). Instead, manually set a timer for testing.
	timer := failsafe.NewTimer()
	timer.SetDuration(2 * time.Hour) // Valid duration
	svc.SetFailsafeTimer(zoneID, timer)
	timer.Start()

	// Verify failsafe timer exists now
	timer = svc.GetFailsafeTimer(zoneID)
	if timer == nil {
		t.Fatal("Failsafe timer should exist after SetFailsafeTimer")
	}

	// Remove the zone
	err := svc.RemoveZone(zoneID)
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Verify failsafe timer was cleaned up
	timer = svc.GetFailsafeTimer(zoneID)
	if timer != nil {
		t.Error("Failsafe timer should be nil after RemoveZone")
	}
}

func TestDeviceServiceRemoveZoneEmitsEvent(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Track events
	var events []Event
	svc.OnEvent(func(e Event) {
		events = append(events, e)
	})

	// Connect and remove zone
	zoneID := "test-zone-003"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Clear events from connect
	events = nil

	err := svc.RemoveZone(zoneID)
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Wait a bit for async event delivery
	time.Sleep(10 * time.Millisecond)

	// Check for EventZoneRemoved
	var foundRemoved bool
	for _, e := range events {
		if e.Type == EventZoneRemoved && e.ZoneID == zoneID {
			foundRemoved = true
			break
		}
	}
	if !foundRemoved {
		t.Error("Expected EventZoneRemoved event")
	}
}

func TestDeviceServiceGetAllZonesAfterRemove(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Connect 3 zones
	svc.HandleZoneConnect("zone-1", cert.ZoneTypeGrid)
	svc.HandleZoneConnect("zone-2", cert.ZoneTypeLocal)
	svc.HandleZoneConnect("zone-3", cert.ZoneTypeLocal)

	if svc.ZoneCount() != 3 {
		t.Fatalf("ZoneCount = %d, want 3", svc.ZoneCount())
	}

	// Remove zone-2
	err := svc.RemoveZone("zone-2")
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Verify zone count
	if svc.ZoneCount() != 2 {
		t.Errorf("ZoneCount = %d, want 2", svc.ZoneCount())
	}

	// Verify remaining zones
	zones := svc.GetAllZones()
	var foundZone1, foundZone3 bool
	for _, z := range zones {
		if z.ID == "zone-1" {
			foundZone1 = true
		}
		if z.ID == "zone-3" {
			foundZone3 = true
		}
		if z.ID == "zone-2" {
			t.Error("zone-2 should have been removed")
		}
	}
	if !foundZone1 || !foundZone3 {
		t.Error("zone-1 and zone-3 should still exist")
	}
}

func TestDeviceServiceRemoveZoneCleanupSessionSubscriptions(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Connect zone
	zoneID := "test-zone-subs"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Note: Full subscription testing would require setting up a real session
	// with subscriptions. Here we just verify the remove doesn't panic.

	err := svc.RemoveZone(zoneID)
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}
}

func TestDeviceServiceRemoveZoneWhileFailsafeActive(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Connect zone
	zoneID := "test-zone-failsafe"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Create a short-duration failsafe timer and manually set it
	timer := failsafe.NewTimer()
	timer.SetDuration(10 * time.Millisecond)
	svc.SetFailsafeTimer(zoneID, timer)
	timer.Start()

	// Wait for failsafe to trigger
	time.Sleep(20 * time.Millisecond)

	// Zone should now be in failsafe state
	zone := svc.GetZone(zoneID)
	if zone == nil {
		t.Fatal("Zone should exist")
	}
	// Note: FailsafeActive might or might not be set depending on timing

	// Remove the zone - should work even if failsafe was active
	err := svc.RemoveZone(zoneID)
	if err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Verify zone is gone
	zone = svc.GetZone(zoneID)
	if zone != nil {
		t.Error("Zone should not exist after RemoveZone")
	}
}

func TestDeviceServiceRemoveLastZoneReentersCommissioning(t *testing.T) {
	svc := createTestDeviceService(t)

	// Set up mock advertiser so we can inspect commissioning state
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopOperational(mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Enter commissioning, then exit (simulates commissioning flow)
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Add a zone
	zoneID := "test-zone-recomm"
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Exit commissioning mode (as happens after commissioning completes)
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Verify not in commissioning mode
	if svc.discoveryManager.IsCommissioningMode() {
		t.Fatal("expected NOT to be in commissioning mode after ExitCommissioningMode")
	}

	// Remove the last zone
	if err := svc.RemoveZone(zoneID); err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Verify device re-entered commissioning mode (DEC-059)
	if !svc.discoveryManager.IsCommissioningMode() {
		t.Error("expected commissioning mode after removing last zone (DEC-059)")
	}
}

func TestDeviceServiceRemoveZoneWithRemainingZonesStaysOperational(t *testing.T) {
	svc := createTestDeviceService(t)

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopOperational(mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Add two zones
	svc.HandleZoneConnect("zone-1", cert.ZoneTypeGrid)
	svc.HandleZoneConnect("zone-2", cert.ZoneTypeLocal)

	// Exit commissioning mode
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Remove one zone -- DEC-059: device re-enters commissioning mode
	// when zone slots become available, even with other zones still connected.
	if err := svc.RemoveZone("zone-1"); err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	if !svc.discoveryManager.IsCommissioningMode() {
		t.Error("should enter commissioning mode when zone slots become available (DEC-059)")
	}
}

func TestDeviceServiceListZoneIDs(t *testing.T) {
	svc := createTestDeviceService(t)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Connect zones
	svc.HandleZoneConnect("zone-a", cert.ZoneTypeGrid)
	svc.HandleZoneConnect("zone-b", cert.ZoneTypeLocal)

	// Get zone IDs
	ids := svc.ListZoneIDs()
	if len(ids) != 2 {
		t.Errorf("len(ListZoneIDs()) = %d, want 2", len(ids))
	}

	// Check for expected IDs
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	if !idSet["zone-a"] || !idSet["zone-b"] {
		t.Errorf("Expected zone-a and zone-b in IDs, got %v", ids)
	}
}

func TestHandleZoneConnect_TestZone_RequiresEnableKey(t *testing.T) {
	t.Run("RejectedWithoutEnableKey", func(t *testing.T) {
		svc := createTestDeviceService(t)

		ctx := context.Background()
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		defer svc.Stop()

		// No enable-key configured -- TEST zone should be rejected
		svc.HandleZoneConnect("test-zone", cert.ZoneTypeTest)

		zone := svc.GetZone("test-zone")
		if zone != nil {
			t.Error("TEST zone should be rejected without valid enable-key")
		}
	})

	t.Run("AcceptedWithEnableKey", func(t *testing.T) {
		svc := createTestDeviceServiceWithTestMode(t) // Has valid enable-key

		ctx := context.Background()
		if err := svc.Start(ctx); err != nil {
			t.Fatalf("Start failed: %v", err)
		}
		defer svc.Stop()

		svc.HandleZoneConnect("test-zone", cert.ZoneTypeTest)

		zone := svc.GetZone("test-zone")
		if zone == nil {
			t.Fatal("TEST zone should be accepted with valid enable-key")
		}
		if zone.Type != cert.ZoneTypeTest {
			t.Errorf("zone.Type = %v, want ZoneTypeTest", zone.Type)
		}
	})
}

func TestEnableKey_AllowsTestZone_WithoutCountingAgainstMaxZones(t *testing.T) {
	svc := createTestDeviceServiceWithTestMode(t)
	// MaxZones should remain at 2 (GRID + LOCAL).
	// TEST zones are an extra observer slot enabled by valid enable-key.
	if svc.config.MaxZones != 2 {
		t.Errorf("config.MaxZones = %d, want 2 (TEST zones don't count against MaxZones)", svc.config.MaxZones)
	}
	// Verify enable-key is valid
	if !svc.isEnableKeyValid() {
		t.Error("expected isEnableKeyValid() to return true with configured enable-key")
	}
}

// createTestDeviceServiceWithTestMode creates a DeviceService with TestMode enabled.
func createTestDeviceServiceWithTestMode(t *testing.T) *DeviceService {
	t.Helper()

	evse := examples.NewEVSE(examples.EVSEConfig{
		DeviceID:           "test-evse-tm",
		VendorName:         "Test Vendor",
		ProductName:        "Test EVSE",
		SerialNumber:       "SN-TM-001",
		VendorID:           0x1234,
		ProductID:          0x0001,
		PhaseCount:         3,
		NominalVoltage:     230,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1380000,
	})

	cfg := DefaultDeviceConfig()
	cfg.ListenAddress = ":0"
	cfg.Discriminator = 1234
	cfg.SetupCode = "12345678"
	cfg.SerialNumber = "SN-TM-001"
	cfg.Brand = "Test Vendor"
	cfg.Model = "Test EVSE"
	cfg.Categories = []discovery.DeviceCategory{discovery.CategoryEMobility}
	cfg.FailsafeTimeout = 100 * time.Millisecond
	cfg.TestMode = true
	cfg.TestEnableKey = "00112233445566778899aabbccddeeff"

	svc, err := NewDeviceService(evse.Device(), cfg)
	if err != nil {
		t.Fatalf("Failed to create DeviceService: %v", err)
	}

	return svc
}

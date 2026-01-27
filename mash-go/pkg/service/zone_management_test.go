package service

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
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

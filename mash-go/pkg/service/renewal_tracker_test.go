package service_test

import (
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/service"
)

// TestRenewalTracker_Track verifies basic tracking functionality.
func TestRenewalTracker_Track(t *testing.T) {
	tracker := service.NewRenewalTracker()

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	tracker.Track("device-1", "zone-1", expiresAt, 1)

	info, ok := tracker.Get("device-1")
	if !ok {
		t.Fatal("Expected to find device-1")
	}

	if info.DeviceID != "device-1" {
		t.Errorf("Expected DeviceID device-1, got %s", info.DeviceID)
	}
	if info.ZoneID != "zone-1" {
		t.Errorf("Expected ZoneID zone-1, got %s", info.ZoneID)
	}
	if !info.ExpiresAt.Equal(expiresAt) {
		t.Errorf("Expected ExpiresAt %v, got %v", expiresAt, info.ExpiresAt)
	}
	if info.Sequence != 1 {
		t.Errorf("Expected Sequence 1, got %d", info.Sequence)
	}
	if info.NeedsRenewal() {
		t.Error("Device with 365 days until expiry should not need renewal")
	}
}

// TestRenewalTracker_TrackUpdate verifies updating existing device.
func TestRenewalTracker_TrackUpdate(t *testing.T) {
	tracker := service.NewRenewalTracker()

	// Initial tracking
	expiresAt1 := time.Now().Add(30 * 24 * time.Hour)
	tracker.Track("device-1", "zone-1", expiresAt1, 1)

	// Update with new expiry
	expiresAt2 := time.Now().Add(365 * 24 * time.Hour)
	renewedAt := time.Now()
	tracker.TrackRenewal("device-1", expiresAt2, 2, renewedAt)

	info, _ := tracker.Get("device-1")

	if !info.ExpiresAt.Equal(expiresAt2) {
		t.Errorf("Expected updated ExpiresAt %v, got %v", expiresAt2, info.ExpiresAt)
	}
	if info.Sequence != 2 {
		t.Errorf("Expected Sequence 2, got %d", info.Sequence)
	}
	if info.RenewedAt == nil || !info.RenewedAt.Equal(renewedAt) {
		t.Error("Expected RenewedAt to be set")
	}
}

// TestRenewalTracker_NeedsRenewal verifies renewal window detection.
func TestRenewalTracker_NeedsRenewal(t *testing.T) {
	tracker := service.NewRenewalTracker()

	// Device expiring in 25 days (within 30-day window)
	expiresSoon := time.Now().Add(25 * 24 * time.Hour)
	tracker.Track("device-soon", "zone-1", expiresSoon, 1)

	// Device expiring in 100 days (outside window)
	expiresLater := time.Now().Add(100 * 24 * time.Hour)
	tracker.Track("device-later", "zone-1", expiresLater, 1)

	// Device already expired
	expired := time.Now().Add(-1 * time.Hour)
	tracker.Track("device-expired", "zone-1", expired, 1)

	needsRenewal := tracker.DevicesNeedingRenewal()

	// Should include device-soon and device-expired, but not device-later
	if len(needsRenewal) != 2 {
		t.Fatalf("Expected 2 devices needing renewal, got %d", len(needsRenewal))
	}

	// Verify correct devices are included
	ids := make(map[string]bool)
	for _, info := range needsRenewal {
		ids[info.DeviceID] = true
	}

	if !ids["device-soon"] {
		t.Error("Expected device-soon to need renewal")
	}
	if !ids["device-expired"] {
		t.Error("Expected device-expired to need renewal")
	}
	if ids["device-later"] {
		t.Error("Did not expect device-later to need renewal")
	}
}

// TestRenewalTracker_NeedsRenewalIndividual tests individual device NeedsRenewal.
func TestRenewalTracker_NeedsRenewalIndividual(t *testing.T) {
	testCases := []struct {
		name         string
		daysUntil    int
		needsRenewal bool
	}{
		{"365 days", 365, false},
		{"31 days", 31, false},
		{"30 days", 30, true}, // Boundary
		{"29 days", 29, true},
		{"1 day", 1, true},
		{"0 days", 0, true},
		{"-1 day (expired)", -1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := &service.DeviceCertInfo{
				DeviceID:  "test",
				ExpiresAt: time.Now().Add(time.Duration(tc.daysUntil) * 24 * time.Hour),
			}

			if info.NeedsRenewal() != tc.needsRenewal {
				t.Errorf("Expected NeedsRenewal=%v for %d days, got %v",
					tc.needsRenewal, tc.daysUntil, info.NeedsRenewal())
			}
		})
	}
}

// TestRenewalTracker_DevicesNearExpiry verifies expiry warning detection.
func TestRenewalTracker_DevicesNearExpiry(t *testing.T) {
	tracker := service.NewRenewalTracker()

	// Device expiring in 5 days (within 7-day warning)
	expiresVerySoon := time.Now().Add(5 * 24 * time.Hour)
	tracker.Track("device-warning", "zone-1", expiresVerySoon, 1)

	// Device expiring in 15 days (outside 7-day warning, inside 30-day renewal)
	expiresSoon := time.Now().Add(15 * 24 * time.Hour)
	tracker.Track("device-soon", "zone-1", expiresSoon, 1)

	// Device expiring in 100 days
	expiresLater := time.Now().Add(100 * 24 * time.Hour)
	tracker.Track("device-later", "zone-1", expiresLater, 1)

	// Query with 7-day warning threshold
	warnings := tracker.DevicesNearExpiry(7 * 24 * time.Hour)

	if len(warnings) != 1 {
		t.Fatalf("Expected 1 device near expiry, got %d", len(warnings))
	}

	if warnings[0].DeviceID != "device-warning" {
		t.Errorf("Expected device-warning, got %s", warnings[0].DeviceID)
	}
}

// TestRenewalTracker_DaysUntilExpiry verifies days calculation.
func TestRenewalTracker_DaysUntilExpiry(t *testing.T) {
	// Note: Due to timing between Now() calls, we add a small buffer
	// to ensure we get full days.
	buffer := time.Minute

	testCases := []struct {
		name     string
		duration time.Duration
		expected int
	}{
		{"365 days", 365*24*time.Hour + buffer, 365},
		{"30 days", 30*24*time.Hour + buffer, 30},
		{"1 day", 24*time.Hour + buffer, 1},
		{"23 hours", 23 * time.Hour, 0}, // Less than a day
		{"expired 1 day ago", -24*time.Hour - buffer, -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			info := &service.DeviceCertInfo{
				DeviceID:  "test",
				ExpiresAt: time.Now().Add(tc.duration),
			}

			days := info.DaysUntilExpiry()
			if days != tc.expected {
				t.Errorf("Expected %d days, got %d", tc.expected, days)
			}
		})
	}
}

// TestRenewalTracker_Remove verifies device removal.
func TestRenewalTracker_Remove(t *testing.T) {
	tracker := service.NewRenewalTracker()

	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	tracker.Track("device-1", "zone-1", expiresAt, 1)

	tracker.Remove("device-1")

	_, ok := tracker.Get("device-1")
	if ok {
		t.Error("Expected device-1 to be removed")
	}
}

// TestRenewalTracker_All verifies listing all devices.
func TestRenewalTracker_All(t *testing.T) {
	tracker := service.NewRenewalTracker()

	tracker.Track("device-1", "zone-1", time.Now().Add(100*24*time.Hour), 1)
	tracker.Track("device-2", "zone-1", time.Now().Add(200*24*time.Hour), 1)
	tracker.Track("device-3", "zone-2", time.Now().Add(300*24*time.Hour), 1)

	all := tracker.All()
	if len(all) != 3 {
		t.Fatalf("Expected 3 devices, got %d", len(all))
	}

	ids := make(map[string]bool)
	for _, info := range all {
		ids[info.DeviceID] = true
	}

	for _, id := range []string{"device-1", "device-2", "device-3"} {
		if !ids[id] {
			t.Errorf("Expected to find %s", id)
		}
	}
}

// TestRenewalTracker_Concurrent verifies thread-safety.
func TestRenewalTracker_Concurrent(t *testing.T) {
	tracker := service.NewRenewalTracker()

	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Track("device-a", "zone-1", time.Now().Add(time.Duration(i)*time.Hour), uint32(i))
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			tracker.Get("device-a")
			tracker.DevicesNeedingRenewal()
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// No panic = success
}

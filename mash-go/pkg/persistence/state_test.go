package persistence

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDeviceStateStore(t *testing.T) {
	t.Run("NewDeviceStateStore", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "state.json"))
		if store == nil {
			t.Fatal("NewDeviceStateStore() returned nil")
		}
	})

	t.Run("SaveAndLoadEmpty", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "state.json"))

		state := &DeviceState{
			Version: 1,
			SavedAt: time.Now(),
		}

		if err := store.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.Version != 1 {
			t.Errorf("Version = %d, want 1", got.Version)
		}
	})

	t.Run("LoadNonExistent", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "nonexistent.json"))

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Should return nil (empty state) for non-existent file
		if got != nil {
			t.Errorf("Load() = %v, want nil for non-existent file", got)
		}
	})

	t.Run("ZoneMembershipRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "state.json"))

		state := &DeviceState{
			Version: 1,
			SavedAt: time.Now(),
			Zones: []ZoneMembership{
				{
					ZoneID:       "zone-1",
					ZoneType:     3,
					ControllerID: "ctrl-abc",
					JoinedAt:     time.Now().Add(-24 * time.Hour),
				},
				{
					ZoneID:       "zone-2",
					ZoneType:     4,
					ControllerID: "ctrl-xyz",
					JoinedAt:     time.Now().Add(-1 * time.Hour),
				},
			},
		}

		if err := store.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(got.Zones) != 2 {
			t.Fatalf("len(Zones) = %d, want 2", len(got.Zones))
		}

		if got.Zones[0].ZoneID != "zone-1" {
			t.Errorf("Zones[0].ZoneID = %q, want %q", got.Zones[0].ZoneID, "zone-1")
		}
		if got.Zones[1].ControllerID != "ctrl-xyz" {
			t.Errorf("Zones[1].ControllerID = %q, want %q", got.Zones[1].ControllerID, "ctrl-xyz")
		}
	})

	t.Run("FailsafeStateRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "state.json"))

		now := time.Now()
		state := &DeviceState{
			Version: 1,
			SavedAt: now,
			FailsafeState: map[string]FailsafeSnapshot{
				"zone-1": {
					State:     1, // StateTimerRunning
					Duration:  4 * time.Hour,
					StartedAt: now.Add(-1 * time.Hour),
					Remaining: 3 * time.Hour,
					Limits: FailsafeLimits{
						ConsumptionLimit:    5000,
						ProductionLimit:     -2000,
						HasConsumptionLimit: true,
						HasProductionLimit:  true,
					},
				},
			},
		}

		if err := store.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		snap, exists := got.FailsafeState["zone-1"]
		if !exists {
			t.Fatal("FailsafeState[zone-1] not found")
		}

		if snap.State != 1 {
			t.Errorf("State = %d, want 1", snap.State)
		}
		if snap.Duration != 4*time.Hour {
			t.Errorf("Duration = %v, want 4h", snap.Duration)
		}
		if snap.Limits.ConsumptionLimit != 5000 {
			t.Errorf("ConsumptionLimit = %d, want 5000", snap.Limits.ConsumptionLimit)
		}
	})

	t.Run("ZoneIndexMapRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewDeviceStateStore(filepath.Join(dir, "state.json"))

		state := &DeviceState{
			Version: 1,
			SavedAt: time.Now(),
			ZoneIndexMap: map[string]uint8{
				"zone-a": 0,
				"zone-b": 1,
				"zone-c": 2,
			},
		}

		if err := store.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(got.ZoneIndexMap) != 3 {
			t.Fatalf("len(ZoneIndexMap) = %d, want 3", len(got.ZoneIndexMap))
		}

		if got.ZoneIndexMap["zone-b"] != 1 {
			t.Errorf("ZoneIndexMap[zone-b] = %d, want 1", got.ZoneIndexMap["zone-b"])
		}
	})

	t.Run("Clear", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")
		store := NewDeviceStateStore(path)

		state := &DeviceState{
			Version: 1,
			Zones:   []ZoneMembership{{ZoneID: "zone-1"}},
		}
		_ = store.Save(state)

		if err := store.Clear(); err != nil {
			t.Fatalf("Clear() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() after Clear() error = %v", err)
		}

		if got != nil {
			t.Errorf("Load() after Clear() = %v, want nil", got)
		}
	})
}

func TestControllerStateStore(t *testing.T) {
	t.Run("DeviceListRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewControllerStateStore(filepath.Join(dir, "state.json"))

		state := &ControllerState{
			Version: 1,
			SavedAt: time.Now(),
			ZoneID:  "my-zone",
			Devices: []DeviceMembership{
				{
					DeviceID:    "dev-1",
					SKI:         "abc123",
					DeviceType:  "EVSE",
					JoinedAt:    time.Now(),
					LastSeenAt:  time.Now(),
				},
				{
					DeviceID:   "dev-2",
					SKI:        "xyz789",
					DeviceType: "INVERTER",
					JoinedAt:   time.Now(),
				},
			},
		}

		if err := store.Save(state); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		got, err := store.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got.ZoneID != "my-zone" {
			t.Errorf("ZoneID = %q, want %q", got.ZoneID, "my-zone")
		}
		if len(got.Devices) != 2 {
			t.Fatalf("len(Devices) = %d, want 2", len(got.Devices))
		}
		if got.Devices[0].DeviceType != "EVSE" {
			t.Errorf("Devices[0].DeviceType = %q, want %q", got.Devices[0].DeviceType, "EVSE")
		}
	})
}

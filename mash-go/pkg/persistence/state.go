package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateVersion is the current version of the state file format.
const StateVersion = 1

// DeviceState contains the runtime state for a MASH device.
type DeviceState struct {
	// Version is the state file format version.
	Version int `json:"version"`

	// SavedAt is when the state was last saved.
	SavedAt time.Time `json:"saved_at"`

	// Zones contains membership info for each zone the device belongs to.
	Zones []ZoneMembership `json:"zones,omitempty"`

	// FailsafeState contains failsafe timer snapshots by zone ID.
	// Failsafe timers are persisted so they can resume after restart.
	FailsafeState map[string]FailsafeSnapshot `json:"failsafe_state,omitempty"`

	// ZoneIndexMap maps zone IDs to their endpoint indices.
	// This ensures consistent endpoint assignments across restarts.
	ZoneIndexMap map[string]uint8 `json:"zone_index_map,omitempty"`
}

// ZoneMembership contains information about a zone the device belongs to.
type ZoneMembership struct {
	// ZoneID is the unique identifier for the zone.
	ZoneID string `json:"zone_id"`

	// ZoneType is the zone type (1=GRID, 2=LOCAL).
	ZoneType uint8 `json:"zone_type"`

	// ControllerID identifies the controller that commissioned this device.
	ControllerID string `json:"controller_id,omitempty"`

	// JoinedAt is when the device joined this zone.
	JoinedAt time.Time `json:"joined_at"`
}

// FailsafeSnapshot captures the failsafe timer state for persistence.
type FailsafeSnapshot struct {
	// State is the failsafe state (0=NORMAL, 1=TIMER_RUNNING, 2=FAILSAFE, 3=GRACE_PERIOD).
	State uint8 `json:"state"`

	// Duration is the configured failsafe duration.
	Duration time.Duration `json:"duration"`

	// StartedAt is when the timer was started.
	StartedAt time.Time `json:"started_at,omitempty"`

	// Remaining is how much time was remaining when saved.
	Remaining time.Duration `json:"remaining,omitempty"`

	// Limits are the failsafe power limits.
	Limits FailsafeLimits `json:"limits,omitempty"`
}

// FailsafeLimits mirrors failsafe.Limits for JSON serialization.
type FailsafeLimits struct {
	ConsumptionLimit    int64 `json:"consumption_limit,omitempty"`
	ProductionLimit     int64 `json:"production_limit,omitempty"`
	HasConsumptionLimit bool  `json:"has_consumption_limit,omitempty"`
	HasProductionLimit  bool  `json:"has_production_limit,omitempty"`
}

// DeviceStateStore manages persistence of device state to a JSON file.
type DeviceStateStore struct {
	mu   sync.Mutex
	path string
}

// NewDeviceStateStore creates a new device state store.
func NewDeviceStateStore(path string) *DeviceStateStore {
	return &DeviceStateStore{path: path}
}

// Save persists the device state to disk.
func (s *DeviceStateStore) Save(state *DeviceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure parent directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	state.Version = StateVersion
	if state.SavedAt.IsZero() {
		state.SavedAt = time.Now()
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// Load reads the device state from disk.
// Returns nil, nil if the file doesn't exist (empty state).
func (s *DeviceStateStore) Load() (*DeviceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state := &DeviceState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}

	return state, nil
}

// Clear removes the state file.
func (s *DeviceStateStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ControllerState contains the runtime state for a MASH controller.
type ControllerState struct {
	// Version is the state file format version.
	Version int `json:"version"`

	// SavedAt is when the state was last saved.
	SavedAt time.Time `json:"saved_at"`

	// ZoneID is the controller's zone ID.
	ZoneID string `json:"zone_id"`

	// Devices contains info about commissioned devices.
	Devices []DeviceMembership `json:"devices,omitempty"`
}

// DeviceMembership contains information about a commissioned device.
type DeviceMembership struct {
	// DeviceID is the unique device identifier.
	DeviceID string `json:"device_id"`

	// DeviceType is the device type (EVSE, INVERTER, etc.).
	DeviceType string `json:"device_type,omitempty"`

	// JoinedAt is when the device was commissioned.
	JoinedAt time.Time `json:"joined_at"`

	// LastSeenAt is when the device was last connected.
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
}

// ControllerStateStore manages persistence of controller state to a JSON file.
type ControllerStateStore struct {
	mu   sync.Mutex
	path string
}

// NewControllerStateStore creates a new controller state store.
func NewControllerStateStore(path string) *ControllerStateStore {
	return &ControllerStateStore{path: path}
}

// Save persists the controller state to disk.
func (s *ControllerStateStore) Save(state *ControllerState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	state.Version = StateVersion
	if state.SavedAt.IsZero() {
		state.SavedAt = time.Now()
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

// Load reads the controller state from disk.
// Returns nil, nil if the file doesn't exist (empty state).
func (s *ControllerStateStore) Load() (*ControllerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	state := &ControllerState{}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}

	return state, nil
}

// Clear removes the state file.
func (s *ControllerStateStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

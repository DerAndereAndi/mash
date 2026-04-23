package service

import (
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/mash-protocol/mash-go/pkg/persistence"
)

// SetCertStore sets the certificate store for persistence.
func (s *DeviceService) SetCertStore(store cert.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.certStore = store
}

// GetCertStore returns the certificate store.
func (s *DeviceService) GetCertStore() cert.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.certStore
}

// SetStateStore sets the state store for persistence.
func (s *DeviceService) SetStateStore(store *persistence.DeviceStateStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateStore = store
}

// SaveState persists the current device state.
// This should be called on graceful shutdown and after commissioning changes.
func (s *DeviceService) SaveState() error {
	s.mu.RLock()
	store := s.stateStore
	if store == nil {
		s.mu.RUnlock()
		return nil // No store configured, no-op
	}

	state := &persistence.DeviceState{
		SavedAt:       time.Now(),
		ZoneIndexMap:  make(map[string]uint8),
		FailsafeState: make(map[string]persistence.FailsafeSnapshot),
	}

	// Save zone index map
	for zoneID, idx := range s.zoneIndexMap {
		state.ZoneIndexMap[zoneID] = idx
	}

	// Save zone memberships
	for zoneID, cz := range s.connectedZones {
		zm := persistence.ZoneMembership{
			ZoneID:   zoneID,
			ZoneType: uint8(cz.Type),
			JoinedAt: cz.LastSeen, // Use LastSeen as proxy for JoinedAt
		}
		state.Zones = append(state.Zones, zm)
	}

	// Save failsafe timer states
	for zoneID, timer := range s.failsafeTimers {
		snap := timer.Snapshot()
		state.FailsafeState[zoneID] = persistence.FailsafeSnapshot{
			State:     uint8(snap.State),
			Duration:  snap.Duration,
			StartedAt: snap.StartedAt,
			Remaining: snap.Remaining,
			Limits: persistence.FailsafeLimits{
				ConsumptionLimit:    snap.Limits.ConsumptionLimit,
				ProductionLimit:     snap.Limits.ProductionLimit,
				HasConsumptionLimit: snap.Limits.HasConsumptionLimit,
				HasProductionLimit:  snap.Limits.HasProductionLimit,
			},
		}
	}

	s.mu.RUnlock()

	return store.Save(state)
}

// LoadState restores the device state from persistence.
// This should be called during Start() if a state store is configured.
func (s *DeviceService) LoadState() error {
	s.mu.Lock()
	store := s.stateStore
	s.mu.Unlock()

	if store == nil {
		return nil // No store configured, no-op
	}

	state, err := store.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return nil // No saved state
	}

	s.mu.Lock()

	// Track restored zones to emit events after unlock
	var restoredZones []string

	// Restore zone index map
	for zoneID, idx := range state.ZoneIndexMap {
		s.zoneIndexMap[zoneID] = idx
		if idx >= s.nextZoneIndex {
			s.nextZoneIndex = idx + 1
		}
	}

	// Restore zone memberships (marked as not connected since no active connection)
	if len(state.Zones) > 0 {
		// New format: zones are explicitly saved
		for _, zm := range state.Zones {
			zoneType := cert.ZoneType(zm.ZoneType)
			cz := &ConnectedZone{
				ID:        zm.ZoneID,
				Type:      zoneType,
				Priority:  zoneType.Priority(),
				Connected: false, // Not connected until controller reconnects
				LastSeen:  zm.JoinedAt,
			}
			s.connectedZones[zm.ZoneID] = cz
			restoredZones = append(restoredZones, zm.ZoneID)
		}
	} else if len(state.ZoneIndexMap) > 0 {
		// Backward compatibility: derive zones from zone_index_map
		// We don't have zone type info, assume Local as default
		for zoneID := range state.ZoneIndexMap {
			zoneType := cert.ZoneTypeLocal
			cz := &ConnectedZone{
				ID:        zoneID,
				Type:      zoneType,
				Priority:  zoneType.Priority(),
				Connected: false,
				LastSeen:  state.SavedAt,
			}
			s.connectedZones[zoneID] = cz
			restoredZones = append(restoredZones, zoneID)
		}
	}

	// Restore failsafe timers
	for zoneID, snap := range state.FailsafeState {
		// Only restore timers that were running or in failsafe
		if snap.State == uint8(failsafe.StateNormal) {
			continue
		}

		timer := failsafe.NewTimer()
		timerSnap := &failsafe.TimerSnapshot{
			State:     failsafe.State(snap.State),
			Duration:  snap.Duration,
			StartedAt: snap.StartedAt,
			Remaining: snap.Remaining,
			Limits: failsafe.Limits{
				ConsumptionLimit:    snap.Limits.ConsumptionLimit,
				ProductionLimit:     snap.Limits.ProductionLimit,
				HasConsumptionLimit: snap.Limits.HasConsumptionLimit,
				HasProductionLimit:  snap.Limits.HasProductionLimit,
			},
		}

		// Set up callback before restore
		zoneIDCopy := zoneID // Capture for closure
		timer.OnFailsafeEnter(func(_ failsafe.Limits) {
			s.handleFailsafe(zoneIDCopy)
		})

		if err := timer.Restore(timerSnap); err != nil {
			s.debugLog("LoadState: failed to restore failsafe timer",
				"zoneID", zoneID, "error", err)
			continue
		}

		s.failsafeTimers[zoneID] = timer
	}

	s.mu.Unlock()

	// Emit events for restored zones (after releasing lock)
	for _, zoneID := range restoredZones {
		s.emitEvent(Event{
			Type:   EventZoneRestored,
			ZoneID: zoneID,
		})
	}

	return nil
}

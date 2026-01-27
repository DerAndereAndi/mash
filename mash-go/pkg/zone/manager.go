package zone

import (
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

// Manager handles zone membership for a MASH device.
// It tracks which zones the device belongs to and manages
// connection state and value resolution.
type Manager struct {
	mu sync.RWMutex

	// zones holds all zone memberships keyed by zone ID.
	zones map[string]*Zone

	// callbacks for zone events.
	onZoneAdded   func(zone *Zone)
	onZoneRemoved func(zoneID string)
	onConnect     func(zoneID string)
	onDisconnect  func(zoneID string)
}

// NewManager creates a new zone manager.
func NewManager() *Manager {
	return &Manager{
		zones: make(map[string]*Zone),
	}
}

// AddZone adds a zone membership.
// Returns ErrZoneExists if the zone already exists.
// Returns ErrMaxZonesExceeded if already at maximum zones.
func (m *Manager) AddZone(zoneID string, zoneType cert.ZoneType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.zones[zoneID]; exists {
		return ErrZoneExists
	}

	if len(m.zones) >= MaxZones {
		return ErrMaxZonesExceeded
	}

	zone := &Zone{
		ID:             zoneID,
		Type:           zoneType,
		Connected:      false,
		CommissionedAt: time.Now(),
	}
	m.zones[zoneID] = zone

	if m.onZoneAdded != nil {
		m.onZoneAdded(zone)
	}

	return nil
}

// RemoveZone removes a zone membership.
// Returns ErrZoneNotFound if the zone doesn't exist.
func (m *Manager) RemoveZone(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.zones[zoneID]; !exists {
		return ErrZoneNotFound
	}

	delete(m.zones, zoneID)

	if m.onZoneRemoved != nil {
		m.onZoneRemoved(zoneID)
	}

	return nil
}

// GetZone returns a zone by ID.
func (m *Manager) GetZone(zoneID string) (*Zone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return nil, ErrZoneNotFound
	}
	return zone, nil
}

// HasZone returns true if the device belongs to the specified zone.
func (m *Manager) HasZone(zoneID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.zones[zoneID]
	return exists
}

// ListZones returns all zone IDs.
func (m *Manager) ListZones() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.zones))
	for id := range m.zones {
		ids = append(ids, id)
	}
	return ids
}

// ZoneCount returns the number of zones.
func (m *Manager) ZoneCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.zones)
}

// AllZones returns all zones.
func (m *Manager) AllZones() []*Zone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zones := make([]*Zone, 0, len(m.zones))
	for _, z := range m.zones {
		zones = append(zones, z)
	}
	return zones
}

// SetConnected marks a zone as connected.
func (m *Manager) SetConnected(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return ErrZoneNotFound
	}

	zone.Connected = true
	zone.LastSeen = time.Now()

	if m.onConnect != nil {
		m.onConnect(zoneID)
	}

	return nil
}

// SetDisconnected marks a zone as disconnected.
func (m *Manager) SetDisconnected(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return ErrZoneNotFound
	}

	zone.Connected = false

	if m.onDisconnect != nil {
		m.onDisconnect(zoneID)
	}

	return nil
}

// UpdateLastSeen updates the last seen timestamp for a zone.
func (m *Manager) UpdateLastSeen(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, exists := m.zones[zoneID]
	if !exists {
		return ErrZoneNotFound
	}

	zone.LastSeen = time.Now()
	return nil
}

// ConnectedZones returns all connected zone IDs.
func (m *Manager) ConnectedZones() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ids []string
	for id, zone := range m.zones {
		if zone.Connected {
			ids = append(ids, id)
		}
	}
	return ids
}

// HighestPriorityZone returns the zone with the highest priority (lowest number).
// Returns nil if no zones exist.
func (m *Manager) HighestPriorityZone() *Zone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var highest *Zone
	for _, zone := range m.zones {
		if highest == nil || zone.Priority() < highest.Priority() {
			highest = zone
		}
	}
	return highest
}

// HighestPriorityConnectedZone returns the connected zone with highest priority.
// Returns nil if no zones are connected.
func (m *Manager) HighestPriorityConnectedZone() *Zone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var highest *Zone
	for _, zone := range m.zones {
		if !zone.Connected {
			continue
		}
		if highest == nil || zone.Priority() < highest.Priority() {
			highest = zone
		}
	}
	return highest
}

// OnZoneAdded sets a callback for when a zone is added.
func (m *Manager) OnZoneAdded(fn func(zone *Zone)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onZoneAdded = fn
}

// OnZoneRemoved sets a callback for when a zone is removed.
func (m *Manager) OnZoneRemoved(fn func(zoneID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onZoneRemoved = fn
}

// OnConnect sets a callback for when a zone connects.
func (m *Manager) OnConnect(fn func(zoneID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onConnect = fn
}

// OnDisconnect sets a callback for when a zone disconnects.
func (m *Manager) OnDisconnect(fn func(zoneID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDisconnect = fn
}

// CanRemoveZone checks if a zone can remove another zone.
// A zone can only forcibly remove a zone with lower priority.
func (m *Manager) CanRemoveZone(requesterType cert.ZoneType, targetZoneID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targetZone, exists := m.zones[targetZoneID]
	if !exists {
		return false
	}

	// Can only force remove if strictly higher priority
	return requesterType.Priority() < targetZone.Priority()
}

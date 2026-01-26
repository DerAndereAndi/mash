package discovery

import (
	"context"
	"sync"
	"time"
)

// Advertiser provides mDNS service advertising capabilities.
type Advertiser interface {
	// AdvertiseCommissionable starts advertising a commissionable service.
	// The service will be advertised until StopCommissionable is called or
	// the commissioning window expires.
	AdvertiseCommissionable(ctx context.Context, info *CommissionableInfo) error

	// StopCommissionable stops advertising the commissionable service.
	StopCommissionable() error

	// AdvertiseOperational starts advertising an operational service for a zone.
	// Multiple zones can be advertised simultaneously.
	AdvertiseOperational(ctx context.Context, info *OperationalInfo) error

	// UpdateOperational updates TXT records for an operational service.
	UpdateOperational(zoneID string, info *OperationalInfo) error

	// StopOperational stops advertising operational service for a specific zone.
	StopOperational(zoneID string) error

	// AdvertiseCommissioner starts advertising a commissioner service.
	AdvertiseCommissioner(ctx context.Context, info *CommissionerInfo) error

	// UpdateCommissioner updates TXT records for a commissioner service.
	UpdateCommissioner(zoneID string, info *CommissionerInfo) error

	// StopCommissioner stops advertising commissioner service for a specific zone.
	StopCommissioner(zoneID string) error

	// StopAll stops all advertisements.
	StopAll()
}

// AdvertiserConfig configures advertiser behavior.
type AdvertiserConfig struct {
	// Interface specifies which network interface to use.
	// Empty string means all interfaces.
	Interface string

	// TTL is the DNS record TTL.
	// Default: 120 seconds.
	TTL time.Duration
}

// DefaultAdvertiserConfig returns the default advertiser configuration.
func DefaultAdvertiserConfig() AdvertiserConfig {
	return AdvertiserConfig{
		Interface: "",
		TTL:       120 * time.Second,
	}
}

// DiscoveryManager manages the device's discovery state machine.
type DiscoveryManager struct {
	mu sync.RWMutex

	state      DiscoveryState
	advertiser Advertiser

	// Commissionable info (used when in commissioning mode)
	commissionableInfo *CommissionableInfo

	// Operational zones
	operationalZones map[string]*OperationalInfo

	// Commissioner zones (for zone controllers)
	commissionerZones map[string]*CommissionerInfo

	// Commissioning window timer
	commissioningTimer *time.Timer

	// Callback for state changes
	onStateChange func(old, new DiscoveryState)
}

// NewDiscoveryManager creates a new discovery manager.
func NewDiscoveryManager(advertiser Advertiser) *DiscoveryManager {
	return &DiscoveryManager{
		state:             StateUnregistered,
		advertiser:        advertiser,
		operationalZones:  make(map[string]*OperationalInfo),
		commissionerZones: make(map[string]*CommissionerInfo),
	}
}

// State returns the current discovery state.
func (m *DiscoveryManager) State() DiscoveryState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// OnStateChange sets a callback for state changes.
func (m *DiscoveryManager) OnStateChange(fn func(old, new DiscoveryState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

// SetCommissionableInfo sets the device's commissionable information.
// This should be called before entering commissioning mode.
func (m *DiscoveryManager) SetCommissionableInfo(info *CommissionableInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commissionableInfo = info
}

// EnterCommissioningMode starts advertising the commissionable service.
// The service will be automatically stopped after the commissioning window expires.
func (m *DiscoveryManager) EnterCommissioningMode(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.commissionableInfo == nil {
		return ErrMissingRequired
	}

	// Start advertising
	if err := m.advertiser.AdvertiseCommissionable(ctx, m.commissionableInfo); err != nil {
		return err
	}

	// Set up commissioning window timer
	m.commissioningTimer = time.AfterFunc(CommissioningWindowDuration, func() {
		m.exitCommissioningModeInternal(false)
	})

	// Update state
	oldState := m.state
	if len(m.operationalZones) > 0 {
		m.state = StateOperationalCommissioning
	} else {
		m.state = StateCommissioningOpen
	}

	if m.onStateChange != nil && oldState != m.state {
		m.onStateChange(oldState, m.state)
	}

	return nil
}

// ExitCommissioningMode stops advertising the commissionable service.
// Call this on successful commissioning or user cancellation.
func (m *DiscoveryManager) ExitCommissioningMode() error {
	return m.exitCommissioningModeInternal(true)
}

// exitCommissioningModeInternal stops commissioning mode.
// userInitiated indicates whether this was triggered by user action (true) or
// by the commissioning window timeout (false). This can be used for logging
// or to notify callbacks with context about why commissioning ended.
func (m *DiscoveryManager) exitCommissioningModeInternal(userInitiated bool) error {
	_ = userInitiated // TODO: pass to onStateChange callback or logging

	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel timer if still running
	if m.commissioningTimer != nil {
		m.commissioningTimer.Stop()
		m.commissioningTimer = nil
	}

	// Stop advertising
	if err := m.advertiser.StopCommissionable(); err != nil {
		return err
	}

	// Update state
	oldState := m.state
	if len(m.operationalZones) > 0 {
		m.state = StateOperational
	} else {
		m.state = StateUncommissioned
	}

	if m.onStateChange != nil && oldState != m.state {
		m.onStateChange(oldState, m.state)
	}

	return nil
}

// AddZone adds an operational zone and starts advertising it.
func (m *DiscoveryManager) AddZone(ctx context.Context, info *OperationalInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate
	if info.ZoneID == "" || info.DeviceID == "" {
		return ErrMissingRequired
	}

	// Start advertising
	if err := m.advertiser.AdvertiseOperational(ctx, info); err != nil {
		return err
	}

	m.operationalZones[info.ZoneID] = info

	// Update state if this is first zone
	if len(m.operationalZones) == 1 && m.state == StateUncommissioned {
		oldState := m.state
		m.state = StateOperational
		if m.onStateChange != nil {
			m.onStateChange(oldState, m.state)
		}
	}

	return nil
}

// UpdateZone updates an operational zone's TXT records.
func (m *DiscoveryManager) UpdateZone(info *OperationalInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operationalZones[info.ZoneID]; !exists {
		return ErrNotFound
	}

	if err := m.advertiser.UpdateOperational(info.ZoneID, info); err != nil {
		return err
	}

	m.operationalZones[info.ZoneID] = info
	return nil
}

// RemoveZone stops advertising an operational zone.
func (m *DiscoveryManager) RemoveZone(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operationalZones[zoneID]; !exists {
		return ErrNotFound
	}

	if err := m.advertiser.StopOperational(zoneID); err != nil {
		return err
	}

	delete(m.operationalZones, zoneID)

	// Update state if all zones removed
	if len(m.operationalZones) == 0 {
		oldState := m.state
		switch m.state {
		case StateOperational:
			m.state = StateUncommissioned
		case StateOperationalCommissioning:
			m.state = StateCommissioningOpen
		}
		if m.onStateChange != nil && oldState != m.state {
			m.onStateChange(oldState, m.state)
		}
	}

	return nil
}

// ZoneCount returns the number of operational zones.
func (m *DiscoveryManager) ZoneCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.operationalZones)
}

// AddCommissionerZone adds a commissioner zone (for zone controllers).
func (m *DiscoveryManager) AddCommissionerZone(ctx context.Context, info *CommissionerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if info.ZoneID == "" || info.ZoneName == "" {
		return ErrMissingRequired
	}

	if err := m.advertiser.AdvertiseCommissioner(ctx, info); err != nil {
		return err
	}

	m.commissionerZones[info.ZoneID] = info
	return nil
}

// UpdateCommissionerZone updates a commissioner zone's TXT records.
func (m *DiscoveryManager) UpdateCommissionerZone(info *CommissionerInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.commissionerZones[info.ZoneID]; !exists {
		return ErrNotFound
	}

	if err := m.advertiser.UpdateCommissioner(info.ZoneID, info); err != nil {
		return err
	}

	m.commissionerZones[info.ZoneID] = info
	return nil
}

// RemoveCommissionerZone stops advertising a commissioner zone.
func (m *DiscoveryManager) RemoveCommissionerZone(zoneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.commissionerZones[zoneID]; !exists {
		return ErrNotFound
	}

	if err := m.advertiser.StopCommissioner(zoneID); err != nil {
		return err
	}

	delete(m.commissionerZones, zoneID)
	return nil
}

// Stop stops all advertising.
func (m *DiscoveryManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel commissioning timer
	if m.commissioningTimer != nil {
		m.commissioningTimer.Stop()
		m.commissioningTimer = nil
	}

	// Stop all advertising
	m.advertiser.StopAll()

	// Clear state
	m.operationalZones = make(map[string]*OperationalInfo)
	m.commissionerZones = make(map[string]*CommissionerInfo)

	oldState := m.state
	m.state = StateUnregistered

	if m.onStateChange != nil && oldState != m.state {
		m.onStateChange(oldState, m.state)
	}
}

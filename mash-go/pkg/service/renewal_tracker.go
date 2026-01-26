package service

import (
	"sync"
	"time"
)

// RenewalWindow is the default window before expiry when renewal should occur.
// Certificates expiring within this window are considered as needing renewal.
const RenewalWindow = 30 * 24 * time.Hour

// RenewalTracker tracks certificate expiry for connected devices.
// It is safe for concurrent use.
type RenewalTracker struct {
	mu      sync.RWMutex
	devices map[string]*DeviceCertInfo
}

// DeviceCertInfo holds certificate information for a device.
type DeviceCertInfo struct {
	// DeviceID is the device identifier.
	DeviceID string

	// ZoneID is the zone this certificate belongs to.
	ZoneID string

	// ExpiresAt is when the certificate expires.
	ExpiresAt time.Time

	// RenewedAt is when the certificate was last renewed (nil if never).
	RenewedAt *time.Time

	// Sequence is the certificate sequence number.
	Sequence uint32
}

// NeedsRenewal returns true if the certificate is within the renewal window.
func (i *DeviceCertInfo) NeedsRenewal() bool {
	return time.Now().Add(RenewalWindow).After(i.ExpiresAt)
}

// DaysUntilExpiry returns the number of days until the certificate expires.
// Returns negative values if already expired.
func (i *DeviceCertInfo) DaysUntilExpiry() int {
	return int(time.Until(i.ExpiresAt).Hours() / 24)
}

// IsExpired returns true if the certificate has expired.
func (i *DeviceCertInfo) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// NewRenewalTracker creates a new RenewalTracker.
func NewRenewalTracker() *RenewalTracker {
	return &RenewalTracker{
		devices: make(map[string]*DeviceCertInfo),
	}
}

// Track adds or updates certificate tracking for a device.
func (t *RenewalTracker) Track(deviceID, zoneID string, expiresAt time.Time, sequence uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.devices[deviceID] = &DeviceCertInfo{
		DeviceID:  deviceID,
		ZoneID:    zoneID,
		ExpiresAt: expiresAt,
		Sequence:  sequence,
	}
}

// TrackRenewal updates tracking after a renewal operation.
func (t *RenewalTracker) TrackRenewal(deviceID string, expiresAt time.Time, sequence uint32, renewedAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if info, ok := t.devices[deviceID]; ok {
		info.ExpiresAt = expiresAt
		info.Sequence = sequence
		info.RenewedAt = &renewedAt
	}
}

// Get returns certificate info for a device.
func (t *RenewalTracker) Get(deviceID string) (*DeviceCertInfo, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info, ok := t.devices[deviceID]
	if !ok {
		return nil, false
	}

	// Return a copy to prevent mutation
	copy := *info
	return &copy, true
}

// Remove stops tracking a device.
func (t *RenewalTracker) Remove(deviceID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.devices, deviceID)
}

// All returns info for all tracked devices.
func (t *RenewalTracker) All() []*DeviceCertInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*DeviceCertInfo, 0, len(t.devices))
	for _, info := range t.devices {
		copy := *info
		result = append(result, &copy)
	}
	return result
}

// DevicesNeedingRenewal returns devices within the renewal window.
func (t *RenewalTracker) DevicesNeedingRenewal() []*DeviceCertInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*DeviceCertInfo
	for _, info := range t.devices {
		if info.NeedsRenewal() {
			copy := *info
			result = append(result, &copy)
		}
	}
	return result
}

// DevicesNearExpiry returns devices within the specified warning window.
func (t *RenewalTracker) DevicesNearExpiry(warningWindow time.Duration) []*DeviceCertInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*DeviceCertInfo
	threshold := time.Now().Add(warningWindow)
	for _, info := range t.devices {
		if threshold.After(info.ExpiresAt) {
			copy := *info
			result = append(result, &copy)
		}
	}
	return result
}

// Count returns the number of tracked devices.
func (t *RenewalTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return len(t.devices)
}

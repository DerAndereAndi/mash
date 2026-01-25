package interaction

import (
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// Subscription represents an active attribute subscription.
type Subscription struct {
	mu sync.RWMutex

	// ID is the unique subscription identifier.
	ID uint32

	// EndpointID is the subscribed endpoint.
	EndpointID uint8

	// FeatureID is the subscribed feature.
	FeatureID uint8

	// AttributeIDs is the list of subscribed attributes (empty = all).
	AttributeIDs []uint16

	// MinInterval is the minimum time between notifications.
	MinInterval time.Duration

	// MaxInterval is the maximum time without a notification (heartbeat).
	MaxInterval time.Duration

	// LastNotify is the time of the last notification.
	LastNotify time.Time

	// Values stores the last known values for delta detection.
	Values map[uint16]any

	// PendingChanges accumulates changes between notifications (for minInterval coalescing).
	PendingChanges map[uint16]any
}

// IsSubscribedTo returns true if the attribute is part of this subscription.
func (s *Subscription) IsSubscribedTo(attrID uint16) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Empty list means all attributes
	if len(s.AttributeIDs) == 0 {
		return true
	}

	for _, id := range s.AttributeIDs {
		if id == attrID {
			return true
		}
	}
	return false
}

// CanNotify returns true if enough time has passed since the last notification.
func (s *Subscription) CanNotify() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastNotify) >= s.MinInterval
}

// NeedsHeartbeat returns true if a heartbeat is due.
func (s *Subscription) NeedsHeartbeat() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastNotify) >= s.MaxInterval
}

// MarkNotified records that a notification was sent.
func (s *Subscription) MarkNotified() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastNotify = time.Now()
	s.PendingChanges = nil // Clear pending
}

// RecordChange records an attribute change for later notification.
func (s *Subscription) RecordChange(attrID uint16, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.PendingChanges == nil {
		s.PendingChanges = make(map[uint16]any)
	}
	s.PendingChanges[attrID] = value
	s.Values[attrID] = value
}

// GetPendingChanges returns accumulated changes since last notification.
func (s *Subscription) GetPendingChanges() map[uint16]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.PendingChanges == nil {
		return nil
	}

	result := make(map[uint16]any, len(s.PendingChanges))
	for k, v := range s.PendingChanges {
		result[k] = v
	}
	return result
}

// GetCurrentValues returns the current known values.
func (s *Subscription) GetCurrentValues() map[uint16]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[uint16]any, len(s.Values))
	for k, v := range s.Values {
		result[k] = v
	}
	return result
}

// OnAttributeChanged implements model.FeatureSubscriber.
// Called when a subscribed feature's attribute changes.
func (s *Subscription) OnAttributeChanged(featureType model.FeatureType, attrID uint16, value any) {
	if !s.IsSubscribedTo(attrID) {
		return
	}
	s.RecordChange(attrID, value)
}

// TimeSinceLastNotify returns the duration since the last notification.
func (s *Subscription) TimeSinceLastNotify() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.LastNotify)
}

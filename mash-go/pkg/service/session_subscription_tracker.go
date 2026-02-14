package service

import (
	"sync"
	"sync/atomic"
)

// Subscription represents a subscription to feature attributes.
type Subscription struct {
	ID         uint32
	EndpointID uint8
	FeatureID  uint8
	Attributes []uint16 // Empty means all attributes
}

// SessionSubscriptionTracker manages bidirectional subscriptions with separate
// ID spaces for inbound (from remote to us) and outbound (from us to remote).
type SessionSubscriptionTracker struct {
	mu sync.RWMutex

	// Inbound subscriptions: from remote side to our features
	inbound       map[uint32]*Subscription
	nextInboundID uint32

	// Outbound subscriptions: from us to remote side's features
	outbound       map[uint32]*Subscription
	nextOutboundID uint32
}

// NewSessionSubscriptionTracker creates a new session subscription tracker.
func NewSessionSubscriptionTracker() *SessionSubscriptionTracker {
	return &SessionSubscriptionTracker{
		inbound:  make(map[uint32]*Subscription),
		outbound: make(map[uint32]*Subscription),
	}
}

// AddInbound adds an inbound subscription (from remote to our features).
// Returns the assigned subscription ID.
func (m *SessionSubscriptionTracker) AddInbound(endpointID, featureID uint8, attributes []uint16) uint32 {
	id := atomic.AddUint32(&m.nextInboundID, 1)

	sub := &Subscription{
		ID:         id,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Attributes: attributes,
	}

	m.mu.Lock()
	m.inbound[id] = sub
	m.mu.Unlock()

	return id
}

// AddOutbound adds an outbound subscription (from us to remote features).
// Returns the assigned subscription ID.
func (m *SessionSubscriptionTracker) AddOutbound(endpointID, featureID uint8, attributes []uint16) uint32 {
	id := atomic.AddUint32(&m.nextOutboundID, 1)

	sub := &Subscription{
		ID:         id,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Attributes: attributes,
	}

	m.mu.Lock()
	m.outbound[id] = sub
	m.mu.Unlock()

	return id
}

// RemoveInbound removes an inbound subscription by ID.
// Returns true if the subscription existed and was removed.
func (m *SessionSubscriptionTracker) RemoveInbound(subscriptionID uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.inbound[subscriptionID]
	if exists {
		delete(m.inbound, subscriptionID)
	}
	return exists
}

// RemoveOutbound removes an outbound subscription by ID.
// Returns true if the subscription existed and was removed.
func (m *SessionSubscriptionTracker) RemoveOutbound(subscriptionID uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.outbound[subscriptionID]
	if exists {
		delete(m.outbound, subscriptionID)
	}
	return exists
}

// GetInbound returns an inbound subscription by ID, or nil if not found.
func (m *SessionSubscriptionTracker) GetInbound(subscriptionID uint32) *Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.inbound[subscriptionID]
}

// GetOutbound returns an outbound subscription by ID, or nil if not found.
func (m *SessionSubscriptionTracker) GetOutbound(subscriptionID uint32) *Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.outbound[subscriptionID]
}

// ListInbound returns a copy of all inbound subscriptions.
func (m *SessionSubscriptionTracker) ListInbound() []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0, len(m.inbound))
	for _, sub := range m.inbound {
		result = append(result, sub)
	}
	return result
}

// ListOutbound returns a copy of all outbound subscriptions.
func (m *SessionSubscriptionTracker) ListOutbound() []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0, len(m.outbound))
	for _, sub := range m.outbound {
		result = append(result, sub)
	}
	return result
}

// GetMatchingInbound returns inbound subscriptions that match the given endpoint, feature, and attribute.
// A subscription matches if it is for the same endpoint and feature, and either:
// - The subscription has no attribute filter (empty Attributes slice means all attributes)
// - The subscription's attribute filter includes the specified attributeID
func (m *SessionSubscriptionTracker) GetMatchingInbound(endpointID, featureID uint8, attributeID uint16) []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matches []*Subscription
	for _, sub := range m.inbound {
		if sub.EndpointID != endpointID || sub.FeatureID != featureID {
			continue
		}

		// Empty Attributes means subscribed to all attributes
		if len(sub.Attributes) == 0 {
			matches = append(matches, sub)
			continue
		}

		// Check if specific attribute is in the subscription
		for _, subAttrID := range sub.Attributes {
			if subAttrID == attributeID {
				matches = append(matches, sub)
				break
			}
		}
	}
	return matches
}

// ClearInbound removes all inbound subscriptions.
// Used by TriggerResetTestState to stop notifications from leaking into the
// next test when sessions are reused.
func (m *SessionSubscriptionTracker) ClearInbound() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inbound = make(map[uint32]*Subscription)
}

// InboundCount returns the number of inbound subscriptions.
func (m *SessionSubscriptionTracker) InboundCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inbound)
}

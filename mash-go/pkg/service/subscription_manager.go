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

// SubscriptionManager manages bidirectional subscriptions with separate
// ID spaces for inbound (from remote to us) and outbound (from us to remote).
type SubscriptionManager struct {
	mu sync.RWMutex

	// Inbound subscriptions: from remote side to our features
	inbound       map[uint32]*Subscription
	nextInboundID uint32

	// Outbound subscriptions: from us to remote side's features
	outbound       map[uint32]*Subscription
	nextOutboundID uint32
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		inbound:  make(map[uint32]*Subscription),
		outbound: make(map[uint32]*Subscription),
	}
}

// AddInbound adds an inbound subscription (from remote to our features).
// Returns the assigned subscription ID.
func (m *SubscriptionManager) AddInbound(endpointID, featureID uint8, attributes []uint16) uint32 {
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
func (m *SubscriptionManager) AddOutbound(endpointID, featureID uint8, attributes []uint16) uint32 {
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
func (m *SubscriptionManager) RemoveInbound(subscriptionID uint32) bool {
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
func (m *SubscriptionManager) RemoveOutbound(subscriptionID uint32) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.outbound[subscriptionID]
	if exists {
		delete(m.outbound, subscriptionID)
	}
	return exists
}

// GetInbound returns an inbound subscription by ID, or nil if not found.
func (m *SubscriptionManager) GetInbound(subscriptionID uint32) *Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.inbound[subscriptionID]
}

// GetOutbound returns an outbound subscription by ID, or nil if not found.
func (m *SubscriptionManager) GetOutbound(subscriptionID uint32) *Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.outbound[subscriptionID]
}

// ListInbound returns a copy of all inbound subscriptions.
func (m *SubscriptionManager) ListInbound() []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0, len(m.inbound))
	for _, sub := range m.inbound {
		result = append(result, sub)
	}
	return result
}

// ListOutbound returns a copy of all outbound subscriptions.
func (m *SubscriptionManager) ListOutbound() []*Subscription {
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
func (m *SubscriptionManager) GetMatchingInbound(endpointID, featureID uint8, attributeID uint16) []*Subscription {
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
func (m *SubscriptionManager) ClearInbound() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inbound = make(map[uint32]*Subscription)
}

// InboundCount returns the number of inbound subscriptions.
func (m *SubscriptionManager) InboundCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inbound)
}

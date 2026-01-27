package service

import (
	"sync"
	"testing"
)

func TestSubscriptionManager_AddInbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add first subscription
	id1 := sm.AddInbound(1, 2, []uint16{100, 101})
	if id1 == 0 {
		t.Error("expected non-zero subscription ID")
	}

	// Add second subscription
	id2 := sm.AddInbound(1, 3, []uint16{200})
	if id2 == 0 {
		t.Error("expected non-zero subscription ID")
	}

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("subscription IDs should be unique: got %d and %d", id1, id2)
	}

	// Verify subscription data
	sub := sm.GetInbound(id1)
	if sub == nil {
		t.Fatal("expected to find subscription")
	}
	if sub.ID != id1 {
		t.Errorf("expected ID %d, got %d", id1, sub.ID)
	}
	if sub.EndpointID != 1 {
		t.Errorf("expected EndpointID 1, got %d", sub.EndpointID)
	}
	if sub.FeatureID != 2 {
		t.Errorf("expected FeatureID 2, got %d", sub.FeatureID)
	}
	if len(sub.Attributes) != 2 || sub.Attributes[0] != 100 || sub.Attributes[1] != 101 {
		t.Errorf("expected Attributes [100 101], got %v", sub.Attributes)
	}
}

func TestSubscriptionManager_AddOutbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add first subscription
	id1 := sm.AddOutbound(1, 2, []uint16{100, 101})
	if id1 == 0 {
		t.Error("expected non-zero subscription ID")
	}

	// Add second subscription
	id2 := sm.AddOutbound(1, 3, []uint16{200})
	if id2 == 0 {
		t.Error("expected non-zero subscription ID")
	}

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("subscription IDs should be unique: got %d and %d", id1, id2)
	}

	// Verify subscription data
	sub := sm.GetOutbound(id1)
	if sub == nil {
		t.Fatal("expected to find subscription")
	}
	if sub.ID != id1 {
		t.Errorf("expected ID %d, got %d", id1, sub.ID)
	}
	if sub.EndpointID != 1 {
		t.Errorf("expected EndpointID 1, got %d", sub.EndpointID)
	}
	if sub.FeatureID != 2 {
		t.Errorf("expected FeatureID 2, got %d", sub.FeatureID)
	}
}

func TestSubscriptionManager_IDSpacesIndependent(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add inbound subscription
	inboundID := sm.AddInbound(1, 2, []uint16{100})

	// Add outbound subscription
	outboundID := sm.AddOutbound(3, 4, []uint16{200})

	// IDs can be the same (different ID spaces)
	// This is allowed by design - we just verify both exist

	// Inbound subscription should not be found in outbound space
	if sm.GetOutbound(inboundID) != nil && inboundID != outboundID {
		t.Error("inbound subscription should not be found in outbound space")
	}

	// Outbound subscription should not be found in inbound space
	if sm.GetInbound(outboundID) != nil && inboundID != outboundID {
		t.Error("outbound subscription should not be found in inbound space")
	}

	// Both should exist in their respective spaces
	if sm.GetInbound(inboundID) == nil {
		t.Error("inbound subscription should exist")
	}
	if sm.GetOutbound(outboundID) == nil {
		t.Error("outbound subscription should exist")
	}
}

func TestSubscriptionManager_RemoveInbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add subscription
	id := sm.AddInbound(1, 2, []uint16{100})

	// Remove should return true for existing subscription
	if !sm.RemoveInbound(id) {
		t.Error("expected RemoveInbound to return true for existing subscription")
	}

	// Subscription should no longer exist
	if sm.GetInbound(id) != nil {
		t.Error("subscription should be removed")
	}

	// Remove again should return false
	if sm.RemoveInbound(id) {
		t.Error("expected RemoveInbound to return false for non-existing subscription")
	}

	// Remove non-existent ID should return false
	if sm.RemoveInbound(9999) {
		t.Error("expected RemoveInbound to return false for non-existing ID")
	}
}

func TestSubscriptionManager_RemoveOutbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add subscription
	id := sm.AddOutbound(1, 2, []uint16{100})

	// Remove should return true for existing subscription
	if !sm.RemoveOutbound(id) {
		t.Error("expected RemoveOutbound to return true for existing subscription")
	}

	// Subscription should no longer exist
	if sm.GetOutbound(id) != nil {
		t.Error("subscription should be removed")
	}

	// Remove again should return false
	if sm.RemoveOutbound(id) {
		t.Error("expected RemoveOutbound to return false for non-existing subscription")
	}

	// Remove non-existent ID should return false
	if sm.RemoveOutbound(9999) {
		t.Error("expected RemoveOutbound to return false for non-existing ID")
	}
}

func TestSubscriptionManager_GetInbound_NotFound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Get non-existent subscription
	sub := sm.GetInbound(9999)
	if sub != nil {
		t.Error("expected nil for non-existing subscription")
	}
}

func TestSubscriptionManager_GetOutbound_NotFound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Get non-existent subscription
	sub := sm.GetOutbound(9999)
	if sub != nil {
		t.Error("expected nil for non-existing subscription")
	}
}

func TestSubscriptionManager_ListInbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Empty list initially
	list := sm.ListInbound()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Add subscriptions
	id1 := sm.AddInbound(1, 2, []uint16{100})
	id2 := sm.AddInbound(3, 4, []uint16{200, 201})

	list = sm.ListInbound()
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}

	// Verify both are present
	ids := make(map[uint32]bool)
	for _, sub := range list {
		ids[sub.ID] = true
	}
	if !ids[id1] || !ids[id2] {
		t.Error("expected both subscription IDs in list")
	}
}

func TestSubscriptionManager_ListOutbound(t *testing.T) {
	sm := NewSubscriptionManager()

	// Empty list initially
	list := sm.ListOutbound()
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}

	// Add subscriptions
	id1 := sm.AddOutbound(1, 2, []uint16{100})
	id2 := sm.AddOutbound(3, 4, []uint16{200, 201})

	list = sm.ListOutbound()
	if len(list) != 2 {
		t.Errorf("expected 2 items, got %d", len(list))
	}

	// Verify both are present
	ids := make(map[uint32]bool)
	for _, sub := range list {
		ids[sub.ID] = true
	}
	if !ids[id1] || !ids[id2] {
		t.Error("expected both subscription IDs in list")
	}
}

func TestSubscriptionManager_ListDoesNotAffectInternal(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add subscription
	id := sm.AddInbound(1, 2, []uint16{100})

	// Get list and modify it
	list := sm.ListInbound()
	list[0] = nil

	// Original should be unaffected
	sub := sm.GetInbound(id)
	if sub == nil {
		t.Error("internal state should not be affected by list modification")
	}
}

func TestSubscriptionManager_EmptyAttributes(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add subscription with empty attributes (subscribe to all)
	id := sm.AddInbound(1, 2, nil)

	sub := sm.GetInbound(id)
	if sub == nil {
		t.Fatal("expected to find subscription")
	}
	if sub.Attributes != nil && len(sub.Attributes) != 0 {
		t.Errorf("expected nil or empty Attributes, got %v", sub.Attributes)
	}

	// Also test with empty slice
	id2 := sm.AddInbound(1, 2, []uint16{})

	sub2 := sm.GetInbound(id2)
	if sub2 == nil {
		t.Fatal("expected to find subscription")
	}
	if len(sub2.Attributes) != 0 {
		t.Errorf("expected empty Attributes, got %v", sub2.Attributes)
	}
}

func TestSubscriptionManager_ConcurrentAccess(t *testing.T) {
	sm := NewSubscriptionManager()

	const numGoroutines = 100
	const opsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // Half inbound, half outbound

	// Concurrent inbound operations
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				id := sm.AddInbound(uint8(i%255), uint8(j%255), []uint16{uint16(j)})
				_ = sm.GetInbound(id)
				_ = sm.ListInbound()
				if j%2 == 0 {
					sm.RemoveInbound(id)
				}
			}
		}(i)
	}

	// Concurrent outbound operations
	for i := 0; i < numGoroutines; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				id := sm.AddOutbound(uint8(i%255), uint8(j%255), []uint16{uint16(j)})
				_ = sm.GetOutbound(id)
				_ = sm.ListOutbound()
				if j%2 == 0 {
					sm.RemoveOutbound(id)
				}
			}
		}(i)
	}

	wg.Wait()

	// Just verify we didn't crash and can still operate
	id := sm.AddInbound(1, 2, []uint16{100})
	if sm.GetInbound(id) == nil {
		t.Error("expected to find subscription after concurrent test")
	}
}

func TestSubscriptionManager_IDsIncrement(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add multiple subscriptions and verify IDs increment
	id1 := sm.AddInbound(1, 2, []uint16{100})
	id2 := sm.AddInbound(1, 2, []uint16{101})
	id3 := sm.AddInbound(1, 2, []uint16{102})

	if id2 <= id1 || id3 <= id2 {
		t.Errorf("expected incrementing IDs, got %d, %d, %d", id1, id2, id3)
	}

	// Same for outbound
	oid1 := sm.AddOutbound(1, 2, []uint16{100})
	oid2 := sm.AddOutbound(1, 2, []uint16{101})
	oid3 := sm.AddOutbound(1, 2, []uint16{102})

	if oid2 <= oid1 || oid3 <= oid2 {
		t.Errorf("expected incrementing IDs, got %d, %d, %d", oid1, oid2, oid3)
	}
}

func TestSubscriptionManager_RemoveDoesNotAffectOther(t *testing.T) {
	sm := NewSubscriptionManager()

	// Add inbound and outbound with potentially same IDs
	inID := sm.AddInbound(1, 2, []uint16{100})
	outID := sm.AddOutbound(3, 4, []uint16{200})

	// Remove inbound should not affect outbound
	sm.RemoveInbound(inID)

	if sm.GetInbound(inID) != nil {
		t.Error("inbound should be removed")
	}
	if sm.GetOutbound(outID) == nil {
		t.Error("outbound should still exist")
	}

	// Remove outbound should work
	sm.RemoveOutbound(outID)
	if sm.GetOutbound(outID) != nil {
		t.Error("outbound should be removed")
	}
}

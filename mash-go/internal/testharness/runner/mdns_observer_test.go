package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// testBrowser: a minimal Browser implementation for observer tests.
// Tests control what the observer sees by sending into the exposed channels.
// ---------------------------------------------------------------------------

type testBrowser struct {
	mu      sync.Mutex
	stopped bool

	// Commissionable browse channels
	commAdded   chan *discovery.CommissionableService
	commRemoved chan *discovery.CommissionableService

	// Operational browse channel
	opAdded chan *discovery.OperationalService

	// Commissioner browse channel
	commrAdded chan *discovery.CommissionerService

	// Pairing request callback (set by BrowsePairingRequests)
	pairingCallback func(discovery.PairingRequestService)

	// Tracking which sessions were started
	commStarted  atomic.Bool
	opStarted    atomic.Bool
	commrStarted atomic.Bool
	stopCalled   atomic.Bool
}

func newTestBrowser() *testBrowser {
	return &testBrowser{
		commAdded:   make(chan *discovery.CommissionableService, 100),
		commRemoved: make(chan *discovery.CommissionableService, 100),
		opAdded:     make(chan *discovery.OperationalService, 100),
		commrAdded:  make(chan *discovery.CommissionerService, 100),
	}
}

func (b *testBrowser) BrowseCommissionable(ctx context.Context) (added, removed <-chan *discovery.CommissionableService, err error) {
	b.commStarted.Store(true)
	// Wrap channels so they close when ctx is cancelled
	addedOut := make(chan *discovery.CommissionableService)
	removedOut := make(chan *discovery.CommissionableService)
	go func() {
		defer close(addedOut)
		for {
			select {
			case <-ctx.Done():
				return
			case svc, ok := <-b.commAdded:
				if !ok {
					return
				}
				select {
				case addedOut <- svc:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	go func() {
		defer close(removedOut)
		for {
			select {
			case <-ctx.Done():
				return
			case svc, ok := <-b.commRemoved:
				if !ok {
					return
				}
				select {
				case removedOut <- svc:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return addedOut, removedOut, nil
}

func (b *testBrowser) BrowseOperational(ctx context.Context, _ string) (<-chan *discovery.OperationalService, error) {
	b.opStarted.Store(true)
	out := make(chan *discovery.OperationalService)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case svc, ok := <-b.opAdded:
				if !ok {
					return
				}
				select {
				case out <- svc:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

func (b *testBrowser) BrowseCommissioners(ctx context.Context) (<-chan *discovery.CommissionerService, error) {
	b.commrStarted.Store(true)
	out := make(chan *discovery.CommissionerService)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case svc, ok := <-b.commrAdded:
				if !ok {
					return
				}
				select {
				case out <- svc:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

func (b *testBrowser) BrowsePairingRequests(ctx context.Context, callback func(discovery.PairingRequestService)) error {
	b.mu.Lock()
	b.pairingCallback = callback
	b.mu.Unlock()
	return nil
}

func (b *testBrowser) FindByDiscriminator(_ context.Context, _ uint16) (*discovery.CommissionableService, error) {
	return nil, nil
}

func (b *testBrowser) FindAllByDiscriminator(_ context.Context, _ uint16) ([]*discovery.CommissionableService, error) {
	return nil, nil
}

func (b *testBrowser) Stop() {
	b.stopCalled.Store(true)
}

// sendPairingRequest invokes the stored callback if set.
func (b *testBrowser) sendPairingRequest(svc discovery.PairingRequestService) {
	b.mu.Lock()
	cb := b.pairingCallback
	b.mu.Unlock()
	if cb != nil {
		cb(svc)
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func noopDebugf(string, ...any) {}

// waitForCondition polls until f returns true, failing after timeout.
func waitForCondition(t *testing.T, timeout time.Duration, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

// ---------------------------------------------------------------------------
// Group A: Observer Core
// ---------------------------------------------------------------------------

func TestObserver_Snapshot_EmptyBeforeAnyBrowse(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Snapshot with a specific type should lazily start a session,
	// but return empty since no services have arrived yet.
	snap := obs.Snapshot("commissionable")
	assert.Empty(t, snap)
	assert.True(t, tb.commStarted.Load(), "commissionable session should have started lazily")
}

func TestObserver_Snapshot_AccumulatesCommissionable(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Send 2 commissionable services
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName:  "MASH-001",
		Host:          "device1.local",
		Port:          8443,
		Discriminator: 1234,
		Brand:         "TestBrand",
	}
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName:  "MASH-002",
		Host:          "device2.local",
		Port:          8443,
		Discriminator: 5678,
	}

	// Wait for both to appear
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 2
	})
	require.NoError(t, err)
	assert.Len(t, snap, 2)

	// Verify service details
	names := map[string]bool{}
	for _, svc := range snap {
		names[svc.InstanceName] = true
		assert.Equal(t, discovery.ServiceTypeCommissionable, svc.ServiceType)
	}
	assert.True(t, names["MASH-001"])
	assert.True(t, names["MASH-002"])
}

func TestObserver_Snapshot_AccumulatesOperational(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	tb.opAdded <- &discovery.OperationalService{
		InstanceName: "zone1-dev1",
		Host:         "device1.local",
		Port:         8443,
		ZoneID:       "AABB",
		DeviceID:     "1122",
	}
	tb.opAdded <- &discovery.OperationalService{
		InstanceName: "zone1-dev2",
		Host:         "device2.local",
		Port:         8443,
		ZoneID:       "AABB",
		DeviceID:     "3344",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "operational", func(svcs []discoveredService) bool {
		return len(svcs) >= 2
	})
	require.NoError(t, err)
	assert.Len(t, snap, 2)
	for _, svc := range snap {
		assert.Equal(t, discovery.ServiceTypeOperational, svc.ServiceType)
		assert.Equal(t, "AABB", svc.TXTRecords["ZI"])
	}
}

func TestObserver_Snapshot_FiltersByServiceType(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Send one commissionable and one operational
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-COMM", Discriminator: 1000,
	}

	// Wait for commissionable to arrive first
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	// Now send operational
	tb.opAdded <- &discovery.OperationalService{
		InstanceName: "zone-op", ZoneID: "AA", DeviceID: "BB",
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	_, err = obs.WaitFor(ctx2, "operational", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	// Filter by commissionable
	commSnap := obs.Snapshot("commissionable")
	assert.Len(t, commSnap, 1)
	assert.Equal(t, "MASH-COMM", commSnap[0].InstanceName)

	// Filter by operational
	opSnap := obs.Snapshot("operational")
	assert.Len(t, opSnap, 1)
	assert.Equal(t, "zone-op", opSnap[0].InstanceName)

	// All
	allSnap := obs.Snapshot("")
	assert.Len(t, allSnap, 2)
}

func TestObserver_Removal_UpdatesSnapshot(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Add a service
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-DEL", Discriminator: 100,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	// Remove it
	tb.commRemoved <- &discovery.CommissionableService{InstanceName: "MASH-DEL"}

	// Wait for removal
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	snap, err := obs.WaitFor(ctx2, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) == 0
	})
	require.NoError(t, err)
	assert.Empty(t, snap)
}

func TestObserver_Stop_ClosesAllSessions(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)

	// Trigger sessions for commissionable and operational
	_ = obs.Snapshot("commissionable")
	_ = obs.Snapshot("operational")

	waitForCondition(t, time.Second, func() bool {
		return tb.commStarted.Load() && tb.opStarted.Load()
	})

	obs.Stop()
	assert.True(t, tb.stopCalled.Load())

	// After stop, snapshot returns empty
	snap := obs.Snapshot("commissionable")
	assert.Empty(t, snap)
}

func TestObserver_LazySessionStart(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// No sessions started yet
	assert.False(t, tb.commStarted.Load())
	assert.False(t, tb.opStarted.Load())

	// Query commissionable -- only that session should start
	_ = obs.Snapshot("commissionable")
	waitForCondition(t, time.Second, func() bool { return tb.commStarted.Load() })
	assert.False(t, tb.opStarted.Load(), "operational session should NOT have started")

	// Query operational -- now it starts
	_ = obs.Snapshot("operational")
	waitForCondition(t, time.Second, func() bool { return tb.opStarted.Load() })
}

func TestObserver_SnapshotIsDeepCopy(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-DEEP", Discriminator: 42, Brand: "Original",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap1, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	// Mutate the returned snapshot
	snap1[0].TXTRecords["brand"] = "MUTATED"
	snap1[0].Addresses = append(snap1[0].Addresses, "1.2.3.4")

	// Get a fresh snapshot
	snap2 := obs.Snapshot("commissionable")
	require.Len(t, snap2, 1)
	assert.Equal(t, "Original", snap2[0].TXTRecords["brand"], "mutation should not affect internal state")
	assert.Empty(t, snap2[0].Addresses, "address mutation should not affect internal state")
}

// ---------------------------------------------------------------------------
// Group B: WaitFor Semantics
// ---------------------------------------------------------------------------

func TestObserver_WaitFor_ImmediatelySatisfied(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Pre-populate
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-PRE", Discriminator: 10,
	}

	// First call triggers session and should pick up the buffered service
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) > 0
	})
	require.NoError(t, err)
	assert.Len(t, snap, 1)
}

func TestObserver_WaitFor_BlocksUntilSatisfied(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Trigger session start
	_ = obs.Snapshot("commissionable")

	done := make(chan []discoveredService, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
			return len(svcs) >= 2
		})
		if err == nil {
			done <- snap
		}
	}()

	// Send first service after 50ms
	time.Sleep(50 * time.Millisecond)
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-W1", Discriminator: 1,
	}

	// Should still be waiting
	select {
	case <-done:
		t.Fatal("should not have returned with only 1 service")
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	// Send second service
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-W2", Discriminator: 2,
	}

	select {
	case snap := <-done:
		assert.Len(t, snap, 2)
	case <-time.After(2 * time.Second):
		t.Fatal("WaitFor should have returned after 2nd service")
	}
}

func TestObserver_WaitFor_ContextTimeout(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) > 0
	})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestObserver_WaitFor_ContextCancelled(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) > 0
	})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestObserver_WaitFor_ReEvaluatesOnRemoval(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Add 2 services
	tb.commAdded <- &discovery.CommissionableService{InstanceName: "MASH-R1", Discriminator: 1}
	tb.commAdded <- &discovery.CommissionableService{InstanceName: "MASH-R2", Discriminator: 2}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 2
	})
	require.NoError(t, err)

	// Now wait for len == 1
	done := make(chan []discoveredService, 1)
	go func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		snap, err := obs.WaitFor(ctx2, "commissionable", func(svcs []discoveredService) bool {
			return len(svcs) == 1
		})
		if err == nil {
			done <- snap
		}
	}()

	// Remove one service
	time.Sleep(50 * time.Millisecond)
	tb.commRemoved <- &discovery.CommissionableService{InstanceName: "MASH-R1"}

	select {
	case snap := <-done:
		assert.Len(t, snap, 1)
		assert.Equal(t, "MASH-R2", snap[0].InstanceName)
	case <-time.After(2 * time.Second):
		t.Fatal("WaitFor should have returned after removal")
	}
}

func TestObserver_WaitFor_ConcurrentCallers(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Trigger the session
	_ = obs.Snapshot("commissionable")

	var wg sync.WaitGroup
	results := make([]int, 3)

	// 3 goroutines waiting for different counts
	for i, threshold := range []int{1, 2, 3} {
		wg.Add(1)
		go func(idx, thr int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
				return len(svcs) >= thr
			})
			if err == nil {
				results[idx] = len(snap)
			}
		}(i, threshold)
	}

	// Send services one at a time
	time.Sleep(20 * time.Millisecond)
	tb.commAdded <- &discovery.CommissionableService{InstanceName: "C1", Discriminator: 1}
	time.Sleep(20 * time.Millisecond)
	tb.commAdded <- &discovery.CommissionableService{InstanceName: "C2", Discriminator: 2}
	time.Sleep(20 * time.Millisecond)
	tb.commAdded <- &discovery.CommissionableService{InstanceName: "C3", Discriminator: 3}

	wg.Wait()
	assert.GreaterOrEqual(t, results[0], 1)
	assert.GreaterOrEqual(t, results[1], 2)
	assert.GreaterOrEqual(t, results[2], 3)
}

// ---------------------------------------------------------------------------
// Group D: Edge Cases
// ---------------------------------------------------------------------------

func TestObserver_DuplicateService_Deduplicated(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Same InstanceName sent twice
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-DUP", Discriminator: 100, Brand: "First",
	}
	tb.commAdded <- &discovery.CommissionableService{
		InstanceName: "MASH-DUP", Discriminator: 100, Brand: "Second",
	}

	// Wait until the second update has been processed
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		if len(svcs) != 1 {
			return false
		}
		return svcs[0].TXTRecords["brand"] == "Second"
	})
	require.NoError(t, err)
	assert.Len(t, snap, 1, "duplicate should be deduplicated")
}

func TestObserver_ServiceUpdate_OverwritesOld(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	tb.opAdded <- &discovery.OperationalService{
		InstanceName: "zone-dev", ZoneID: "AAAA", DeviceID: "aaa",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := obs.WaitFor(ctx, "operational", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	// Update with new DeviceID
	tb.opAdded <- &discovery.OperationalService{
		InstanceName: "zone-dev", ZoneID: "AAAA", DeviceID: "bbb",
	}

	// Wait for the update
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	snap, err := obs.WaitFor(ctx2, "operational", func(svcs []discoveredService) bool {
		return len(svcs) == 1 && svcs[0].TXTRecords["DI"] == "bbb"
	})
	require.NoError(t, err)
	assert.Len(t, snap, 1)
	assert.Equal(t, "bbb", snap[0].TXTRecords["DI"])
}

func TestObserver_StopThenSnapshot_ReturnsEmpty(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)

	tb.commAdded <- &discovery.CommissionableService{InstanceName: "MASH-X", Discriminator: 1}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)

	obs.Stop()
	snap := obs.Snapshot("commissionable")
	assert.Empty(t, snap)
}

func TestObserver_CommissionerServices(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	tb.commrAdded <- &discovery.CommissionerService{
		InstanceName: "ctrl-1",
		ZoneName:     "Home",
		ZoneID:       "AABB",
		DeviceCount:  3,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "commissioner", func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)
	require.Len(t, snap, 1)
	assert.Equal(t, "ctrl-1", snap[0].InstanceName)
	assert.Equal(t, "Home", snap[0].TXTRecords["ZN"])
	assert.Equal(t, "AABB", snap[0].TXTRecords["ZI"])
	assert.Equal(t, "3", snap[0].TXTRecords["DC"])
}

func TestObserver_PairingRequestServices(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Start pairing request session by querying it
	_ = obs.Snapshot(discovery.ServiceTypePairingRequest)

	// Give time for BrowsePairingRequests to register the callback
	waitForCondition(t, time.Second, func() bool {
		tb.mu.Lock()
		defer tb.mu.Unlock()
		return tb.pairingCallback != nil
	})

	// Send a pairing request via callback
	tb.sendPairingRequest(discovery.PairingRequestService{
		InstanceName:  "pair-1",
		Discriminator: 4567,
		ZoneID:        "CCDD",
		ZoneName:      "Office",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, discovery.ServiceTypePairingRequest, func(svcs []discoveredService) bool {
		return len(svcs) >= 1
	})
	require.NoError(t, err)
	require.Len(t, snap, 1)
	assert.Equal(t, "pair-1", snap[0].InstanceName)
	assert.Equal(t, uint16(4567), snap[0].Discriminator)
	assert.Equal(t, "CCDD", snap[0].TXTRecords["ZI"])
}

func TestObserver_HighFrequencyUpdates(t *testing.T) {
	tb := newTestBrowser()
	obs := newMDNSObserver(tb, noopDebugf)
	defer obs.Stop()

	// Start session
	_ = obs.Snapshot("commissionable")

	// Rapidly send 100 services
	const count = 100
	for i := range count {
		tb.commAdded <- &discovery.CommissionableService{
			InstanceName:  fmt.Sprintf("MASH-%04d", i),
			Discriminator: uint16(i),
		}
	}

	// Wait for all to appear
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snap, err := obs.WaitFor(ctx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) >= count
	})
	require.NoError(t, err)
	assert.Len(t, snap, count)
}

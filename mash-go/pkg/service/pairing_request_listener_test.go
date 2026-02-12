package service

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// TestPairingRequestListener_ActiveUntilContextCancelled verifies that
// IsPairingRequestListening returns true while browsing is active, even when
// BrowsePairingRequests returns immediately (non-blocking).
//
// Bug: runPairingRequestListener sets pairingRequestActive = false as soon as
// BrowsePairingRequests returns. Since the real mDNS browser returns nil
// immediately (non-blocking), the active flag is cleared right away.
// The fix adds <-ctx.Done() after BrowsePairingRequests to block.
func TestPairingRequestListener_ActiveUntilContextCancelled(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser (minimal expectations for Start)
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser: BrowsePairingRequests returns nil immediately
	// (simulating real non-blocking behavior -- does NOT block on <-ctx.Done())
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Return(nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })

	// Wait for the goroutine to run
	time.Sleep(50 * time.Millisecond)

	// The listener should still be marked active. Before the fix,
	// BrowsePairingRequests returns immediately and runPairingRequestListener
	// sets pairingRequestActive = false, so this returns false.
	if !svc.IsPairingRequestListening() {
		t.Error("expected IsPairingRequestListening() == true while listener goroutine is running")
	}

	// Stop should transition to inactive
	if err := svc.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// After stop, the listener should no longer be active
	if svc.IsPairingRequestListening() {
		t.Error("expected IsPairingRequestListening() == false after Stop")
	}
}

// TestPairingRequestListener_NoDuplicateStart verifies that
// updatePairingRequestListening does not start duplicate listeners when
// called multiple times while a listener is already running.
//
// Bug: Because pairingRequestActive goes false immediately (browse returns
// non-blocking), updatePairingRequestListening sees active==false and starts
// another listener on every call.
func TestPairingRequestListener_NoDuplicateStart(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	svc.SetAdvertiser(advertiser)

	// Track how many times BrowsePairingRequests is called
	var browseCount atomic.Int32

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ func(discovery.PairingRequestService)) {
			browseCount.Add(1)
		}).
		Return(nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })

	// Wait for Start to launch the initial listener goroutine
	time.Sleep(50 * time.Millisecond)

	// Simulate zone changes that trigger updatePairingRequestListening
	svc.updatePairingRequestListening()
	svc.updatePairingRequestListening()
	svc.updatePairingRequestListening()

	// Wait for any additional goroutines to run
	time.Sleep(50 * time.Millisecond)

	count := browseCount.Load()
	if count != 1 {
		t.Errorf("expected BrowsePairingRequests to be called exactly 1 time, got %d", count)
	}
}

// TestPairingRequestListener_StopCancelsContext verifies that
// StopPairingRequestListening actually cancels the browse context when the
// listener is active.
//
// Bug: Because pairingRequestActive is already false (browse returned
// immediately), StopPairingRequestListening returns early without cancelling
// the context.
func TestPairingRequestListener_StopCancelsContext(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	svc.SetAdvertiser(advertiser)

	// Track whether the browse context was cancelled
	ctxCancelled := make(chan struct{}, 1)

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, _ func(discovery.PairingRequestService)) {
			// Monitor context cancellation in a goroutine
			go func() {
				<-ctx.Done()
				select {
				case ctxCancelled <- struct{}{}:
				default:
				}
			}()
			// Return immediately (non-blocking), simulating real behavior
		}).
		Return(nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })

	// Wait for listener goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Explicitly stop pairing request listening
	if err := svc.StopPairingRequestListening(); err != nil {
		t.Fatalf("StopPairingRequestListening failed: %v", err)
	}

	// Verify the browse context was cancelled.
	// Before the fix, StopPairingRequestListening returns early because
	// pairingRequestActive is already false, and pairingRequestCancel is
	// never called.
	select {
	case <-ctxCancelled:
		// Context was cancelled as expected
	case <-time.After(200 * time.Millisecond):
		t.Error("expected browse context to be cancelled after StopPairingRequestListening, but it was not")
	}
}

// TestPairingRequestListener_RestartAfterStop verifies that a listener can be
// stopped and restarted, and that IsPairingRequestListening reflects the
// correct state at each phase.
//
// Bug: After restart, the browse returns immediately and pairingRequestActive
// goes back to false, so IsPairingRequestListening returns false even though
// we just started it.
func TestPairingRequestListener_RestartAfterStop(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	svc.SetAdvertiser(advertiser)

	var browseCount atomic.Int32

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ func(discovery.PairingRequestService)) {
			browseCount.Add(1)
		}).
		Return(nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })

	// Wait for initial listener to start
	time.Sleep(50 * time.Millisecond)

	// Stop listening
	if err := svc.StopPairingRequestListening(); err != nil {
		t.Fatalf("StopPairingRequestListening failed: %v", err)
	}

	// Wait for goroutine to finish
	time.Sleep(50 * time.Millisecond)

	// Restart listening
	if err := svc.StartPairingRequestListening(svc.ctx); err != nil {
		t.Fatalf("StartPairingRequestListening failed: %v", err)
	}

	// Wait for second listener to start
	time.Sleep(50 * time.Millisecond)

	// After restart, the listener should be active. Before the fix,
	// BrowsePairingRequests returns immediately and pairingRequestActive is
	// set to false.
	if !svc.IsPairingRequestListening() {
		t.Error("expected IsPairingRequestListening() == true after restart")
	}

	count := browseCount.Load()
	if count != 2 {
		t.Errorf("expected BrowsePairingRequests to be called exactly 2 times (start + restart), got %d", count)
	}
}

// TestPairingRequestDiscovered_TestZonesDontBlockPairing verifies that
// TEST zones do not count against MaxZones when deciding whether to accept
// a pairing request. Only non-TEST zones should block pairing.
//
// Bug: handlePairingRequestDiscovered used len(s.connectedZones) >= MaxZones,
// which counts TEST zones. With MaxZones=2 and 2 TEST zones connected,
// pairing requests were incorrectly rejected. The fix uses
// nonTestZoneCountLocked() for consistency with updatePairingRequestListening.
func TestPairingRequestDiscovered_TestZonesDontBlockPairing(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.MaxZones = 2
	config.Discriminator = 3840

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up a dummy listener so ensureListenerStarted returns nil.
	// This allows EnterCommissioningMode to succeed and set commissioningOpen=true.
	dummyListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create dummy listener: %v", err)
	}
	t.Cleanup(func() { dummyListener.Close() })
	svc.listener = dummyListener

	// Put service in running state so EnterCommissioningMode proceeds.
	svc.state = StateRunning

	// Simulate 2 TEST zones connected (at MaxZones if counting all zones).
	svc.mu.Lock()
	svc.connectedZones["zone-a"] = &ConnectedZone{
		ID:        "zone-a",
		Type:      cert.ZoneTypeTest,
		Connected: true,
	}
	svc.connectedZones["zone-b"] = &ConnectedZone{
		ID:        "zone-b",
		Type:      cert.ZoneTypeTest,
		Connected: true,
	}
	svc.mu.Unlock()

	pairingReq := discovery.PairingRequestService{
		Discriminator: 3840,
		ZoneID:        "controller-zone",
	}

	// Call handlePairingRequestDiscovered with matching discriminator.
	// Before fix: len(connectedZones)==2 >= MaxZones==2 → "at max zones"
	//             → commissioningOpen stays false.
	// After fix:  nonTestZoneCount==0 < MaxZones==2 → EnterCommissioningMode
	//             → commissioningOpen=true.
	svc.handlePairingRequestDiscovered(pairingReq, 3840)

	if !svc.commissioningOpen.Load() {
		t.Error("expected commissioningOpen=true: TEST zones should not block pairing requests")
	}

	// Clean up: close commissioning window for next sub-test
	svc.commissioningOpen.Store(false)

	// Verify that non-TEST zones at max DO block pairing.
	svc.mu.Lock()
	svc.connectedZones = map[string]*ConnectedZone{
		"zone-grid": {
			ID:        "zone-grid",
			Type:      cert.ZoneTypeGrid,
			Connected: true,
		},
		"zone-local": {
			ID:        "zone-local",
			Type:      cert.ZoneTypeLocal,
			Connected: true,
		},
	}
	svc.mu.Unlock()

	svc.handlePairingRequestDiscovered(pairingReq, 3840)

	if svc.commissioningOpen.Load() {
		t.Error("expected commissioningOpen=false: non-TEST zones at max should block pairing")
	}
}

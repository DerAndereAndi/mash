package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

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

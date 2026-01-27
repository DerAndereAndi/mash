package discovery_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/stretchr/testify/mock"
)

// TestDiscoveryManagerCommissioningWindowDuration verifies that the commissioning
// window duration can be configured.
func TestDiscoveryManagerCommissioningWindowDuration(t *testing.T) {
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()

	dm := discovery.NewDiscoveryManager(advertiser)
	dm.SetCommissionableInfo(&discovery.CommissionableInfo{
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "TEST001",
		Brand:         "TestBrand",
		Model:         "TestModel",
		Port:          8443,
	})

	// Set a short commissioning window duration
	dm.SetCommissioningWindowDuration(50 * time.Millisecond)

	// Track timeout callback
	var timeoutCalled bool
	var timeoutMu sync.Mutex
	timeoutCh := make(chan struct{}, 1)
	dm.OnCommissioningTimeout(func() {
		timeoutMu.Lock()
		timeoutCalled = true
		timeoutMu.Unlock()
		select {
		case timeoutCh <- struct{}{}:
		default:
		}
	})

	// Enter commissioning mode
	ctx := context.Background()
	if err := dm.EnterCommissioningMode(ctx); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait for timeout
	select {
	case <-timeoutCh:
		// Timeout callback was called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for commissioning window to expire")
	}

	timeoutMu.Lock()
	defer timeoutMu.Unlock()
	if !timeoutCalled {
		t.Error("expected timeout callback to be called")
	}
}

// TestDiscoveryManagerDefaultCommissioningWindowDuration verifies that when no
// custom duration is set, the default 3-hour duration is used (tested indirectly
// by verifying no timeout within a short period).
func TestDiscoveryManagerDefaultCommissioningWindowDuration(t *testing.T) {
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()

	dm := discovery.NewDiscoveryManager(advertiser)
	dm.SetCommissionableInfo(&discovery.CommissionableInfo{
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "TEST001",
		Brand:         "TestBrand",
		Model:         "TestModel",
		Port:          8443,
	})

	// Do NOT set a custom duration - should use default (3 hours)

	// Track timeout callback
	var timeoutCalled bool
	var timeoutMu sync.Mutex
	dm.OnCommissioningTimeout(func() {
		timeoutMu.Lock()
		timeoutCalled = true
		timeoutMu.Unlock()
	})

	// Enter commissioning mode
	ctx := context.Background()
	if err := dm.EnterCommissioningMode(ctx); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait a short time - should NOT timeout (default is 3 hours)
	time.Sleep(100 * time.Millisecond)

	timeoutMu.Lock()
	called := timeoutCalled
	timeoutMu.Unlock()

	if called {
		t.Error("timeout callback should NOT have been called with default duration")
	}

	// Clean up - exit commissioning mode
	if err := dm.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}
}

// TestDiscoveryManagerCommissioningTimeoutCallsStopCommissionable verifies that
// when the commissioning window times out, StopCommissionable is called.
func TestDiscoveryManagerCommissioningTimeoutCallsStopCommissionable(t *testing.T) {
	stopCalled := make(chan struct{}, 1)
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Run(func() {
		select {
		case stopCalled <- struct{}{}:
		default:
		}
	}).Return(nil).Once()

	dm := discovery.NewDiscoveryManager(advertiser)
	dm.SetCommissionableInfo(&discovery.CommissionableInfo{
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "TEST001",
		Brand:         "TestBrand",
		Model:         "TestModel",
		Port:          8443,
	})

	// Set a short commissioning window duration
	dm.SetCommissioningWindowDuration(50 * time.Millisecond)

	// Enter commissioning mode
	ctx := context.Background()
	if err := dm.EnterCommissioningMode(ctx); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait for StopCommissionable to be called
	select {
	case <-stopCalled:
		// Success
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for StopCommissionable to be called")
	}
}

// TestDiscoveryManagerExitBeforeTimeoutCancelsTimer verifies that exiting
// commissioning mode before the timeout cancels the timer.
func TestDiscoveryManagerExitBeforeTimeoutCancelsTimer(t *testing.T) {
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once() // Only once for ExitCommissioningMode

	dm := discovery.NewDiscoveryManager(advertiser)
	dm.SetCommissionableInfo(&discovery.CommissionableInfo{
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "TEST001",
		Brand:         "TestBrand",
		Model:         "TestModel",
		Port:          8443,
	})

	// Set a short commissioning window duration
	dm.SetCommissioningWindowDuration(100 * time.Millisecond)

	// Track timeout callback
	var timeoutCount int
	var timeoutMu sync.Mutex
	dm.OnCommissioningTimeout(func() {
		timeoutMu.Lock()
		timeoutCount++
		timeoutMu.Unlock()
	})

	// Enter commissioning mode
	ctx := context.Background()
	if err := dm.EnterCommissioningMode(ctx); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Exit commissioning mode before timeout
	if err := dm.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Wait longer than the commissioning window
	time.Sleep(200 * time.Millisecond)

	timeoutMu.Lock()
	defer timeoutMu.Unlock()

	if timeoutCount > 0 {
		t.Errorf("expected no timeout callbacks after exit, got %d", timeoutCount)
	}
}

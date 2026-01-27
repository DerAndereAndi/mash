package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// TestCommissioningWindowTimeoutEmitsEvent verifies that when the commissioning
// window expires, an EventCommissioningClosed event is emitted with Reason "timeout".
func TestCommissioningWindowTimeoutEmitsEvent(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.CommissioningWindowDuration = 50 * time.Millisecond // Short duration for testing

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track events
	var receivedEvent *Event
	var eventMu sync.Mutex
	eventCh := make(chan struct{}, 1)
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningClosed {
			eventCopy := e
			receivedEvent = &eventCopy
			select {
			case eventCh <- struct{}{}:
			default:
			}
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait for the commissioning window to expire
	select {
	case <-eventCh:
		// Event received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for EventCommissioningClosed")
	}

	eventMu.Lock()
	defer eventMu.Unlock()

	if receivedEvent == nil {
		t.Fatal("expected to receive EventCommissioningClosed")
	}

	if receivedEvent.Type != EventCommissioningClosed {
		t.Errorf("expected EventCommissioningClosed, got %v", receivedEvent.Type)
	}

	if receivedEvent.Reason != "timeout" {
		t.Errorf("expected Reason='timeout', got %q", receivedEvent.Reason)
	}
}

// TestCommissioningWindowSuccessfulCommissionEmitsEvent verifies that when
// commissioning succeeds (ExitCommissioningMode is called), an EventCommissioningClosed
// event is emitted with Reason "commissioned".
func TestCommissioningWindowSuccessfulCommissionEmitsEvent(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track events
	var receivedEvent *Event
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningClosed {
			eventCopy := e
			receivedEvent = &eventCopy
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Exit commissioning mode (simulating successful commission)
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Wait for event to be processed
	time.Sleep(50 * time.Millisecond)

	eventMu.Lock()
	defer eventMu.Unlock()

	if receivedEvent == nil {
		t.Fatal("expected to receive EventCommissioningClosed")
	}

	if receivedEvent.Type != EventCommissioningClosed {
		t.Errorf("expected EventCommissioningClosed, got %v", receivedEvent.Type)
	}

	if receivedEvent.Reason != "commissioned" {
		t.Errorf("expected Reason='commissioned', got %q", receivedEvent.Reason)
	}
}

// TestCommissioningWindowTimeoutStopsMDNSAdvertisement verifies that when the
// commissioning window expires, the mDNS commissionable advertisement is stopped.
func TestCommissioningWindowTimeoutStopsMDNSAdvertisement(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.CommissioningWindowDuration = 50 * time.Millisecond // Short duration for testing

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - verify StopCommissionable is called
	stopCalled := make(chan struct{}, 1)
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Run(func() {
		select {
		case stopCalled <- struct{}{}:
		default:
		}
	}).Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait for StopCommissionable to be called
	select {
	case <-stopCalled:
		// StopCommissionable was called as expected
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for StopCommissionable to be called")
	}
}

// TestCommissioningWindowStateTransitions verifies the correct state transitions
// when the commissioning window expires.
func TestCommissioningWindowStateTransitions(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.CommissioningWindowDuration = 50 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track discovery state changes
	var stateChanges []discovery.DiscoveryState
	var stateMu sync.Mutex
	svc.mu.Lock()
	dm := svc.discoveryManager
	svc.mu.Unlock()
	dm.OnStateChange(func(_, new discovery.DiscoveryState) {
		stateMu.Lock()
		stateChanges = append(stateChanges, new)
		stateMu.Unlock()
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Wait for timeout and state transition
	time.Sleep(200 * time.Millisecond)

	stateMu.Lock()
	defer stateMu.Unlock()

	// Should have transitioned to CommissioningOpen then to Uncommissioned
	// (since no zones are connected)
	if len(stateChanges) < 2 {
		t.Fatalf("expected at least 2 state changes, got %d: %v", len(stateChanges), stateChanges)
	}

	// First state change should be to CommissioningOpen
	if stateChanges[0] != discovery.StateCommissioningOpen {
		t.Errorf("expected first state change to StateCommissioningOpen, got %v", stateChanges[0])
	}

	// Second state change should be to Uncommissioned (no zones connected)
	if stateChanges[1] != discovery.StateUncommissioned {
		t.Errorf("expected second state change to StateUncommissioned, got %v", stateChanges[1])
	}
}

// TestCommissioningWindowDoesNotFireAfterSuccessfulCommission verifies that
// the timeout callback does not fire if commissioning succeeds before the window expires.
func TestCommissioningWindowDoesNotFireAfterSuccessfulCommission(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.CommissioningWindowDuration = 100 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Once() // Only once for ExitCommissioningMode
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track events - count how many times we get "timeout"
	var timeoutCount int
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningClosed && e.Reason == "timeout" {
			timeoutCount++
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Exit commissioning mode before timeout (simulating successful commission)
	if err := svc.ExitCommissioningMode(); err != nil {
		t.Fatalf("ExitCommissioningMode failed: %v", err)
	}

	// Wait longer than the commissioning window
	time.Sleep(200 * time.Millisecond)

	eventMu.Lock()
	defer eventMu.Unlock()

	if timeoutCount > 0 {
		t.Errorf("expected no timeout events after successful commission, got %d", timeoutCount)
	}
}

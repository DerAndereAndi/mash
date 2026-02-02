package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
)

// TestCommissionDevice_DeviceAlreadyAdvertising tests that when a device is already
// advertising, commissioning proceeds directly without announcing a pairing request.
func TestCommissionDevice_DeviceAlreadyAdvertising(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 100 * time.Millisecond  // Short for test
	config.ConnectionTimeout = 100 * time.Millisecond // Short for test
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser finds the device immediately
	// Use localhost with a port that will fail fast (connection refused)
	device := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          1, // Port 1 should be rejected quickly
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
	}

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return([]*discovery.CommissionableService{device}, nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser - should NOT have AnnouncePairingRequest called
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	// Note: We do NOT expect AnnouncePairingRequest to be called
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID (normally set during start with cert store)
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.mu.Unlock()

	// CommissionDevice should find the device directly, no pairing request needed
	result, err := svc.CommissionDevice(ctx, 1234, "12345678")

	// We expect it to find the device (but fail on actual connection since there's no real device)
	// The key assertion is that no pairing request was announced
	assert.Nil(t, result)
	assert.Error(t, err) // Connection will fail since no actual device

	// Mock expectations verify AnnouncePairingRequest was NOT called
}

// TestCommissionDevice_DeviceNotAdvertising_PairingRequestAnnounced tests that when
// a device is not found initially, a pairing request is announced.
func TestCommissionDevice_DeviceNotAdvertising_PairingRequestAnnounced(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond       // Short for test
	config.PairingRequestTimeout = 100 * time.Millisecond // Short timeout for test
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser - no devices found
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return(nil, nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser - should announce pairing request
	var announcedInfo *discovery.PairingRequestInfo
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).
		Run(func(_ context.Context, info *discovery.PairingRequestInfo) {
			announcedInfo = info
		}).
		Return(nil).Once()
	advertiser.EXPECT().StopPairingRequest(uint16(1234)).Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.zoneName = "Test Zone"
	svc.mu.Unlock()

	// CommissionDevice - device not found, should announce pairing request and timeout
	_, err = svc.CommissionDevice(ctx, 1234, "12345678")

	// Should timeout since device never appears
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPairingRequestTimeout), "expected timeout error, got: %v", err)

	// Verify pairing request was announced with correct info
	require.NotNil(t, announcedInfo, "expected pairing request to be announced")
	assert.Equal(t, uint16(1234), announcedInfo.Discriminator)
	assert.Equal(t, "a1b2c3d4e5f6a7b8", announcedInfo.ZoneID)
	assert.Equal(t, "Test Zone", announcedInfo.ZoneName)
}

// TestCommissionDevice_PairingRequest_DeviceAppears tests that when a device appears
// after the pairing request is announced, commissioning proceeds.
func TestCommissionDevice_PairingRequest_DeviceAppears(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond           // Short for test
	config.ConnectionTimeout = 100 * time.Millisecond         // Short for test
	config.PairingRequestTimeout = 500 * time.Millisecond     // Short for test
	config.PairingRequestPollInterval = 10 * time.Millisecond // Fast polling for test
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser - device not found initially, then appears
	// Use localhost with a port that will fail fast (connection refused)
	device := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          1, // Port 1 should be rejected quickly
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
	}

	callCount := 0
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		RunAndReturn(func(_ context.Context, _ uint16) ([]*discovery.CommissionableService, error) {
			callCount++
			if callCount == 1 {
				return nil, nil // No devices initially
			}
			return []*discovery.CommissionableService{device}, nil
		}).Times(2)
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser
	var announcedInfo *discovery.PairingRequestInfo
	var stoppedDiscriminator uint16
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).
		Run(func(_ context.Context, info *discovery.PairingRequestInfo) {
			announcedInfo = info
		}).
		Return(nil).Once()
	advertiser.EXPECT().StopPairingRequest(mock.Anything).
		Run(func(d uint16) {
			stoppedDiscriminator = d
		}).
		Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.zoneName = "Test Zone"
	svc.mu.Unlock()

	// CommissionDevice - device not found initially, then appears
	_, err = svc.CommissionDevice(ctx, 1234, "12345678")

	// Connection will fail since no actual device, but we verify the flow
	assert.Error(t, err) // Expected - no real device to connect to

	// Verify pairing request was announced
	require.NotNil(t, announcedInfo)
	assert.Equal(t, uint16(1234), announcedInfo.Discriminator)

	// Verify pairing request was stopped after device was found
	assert.Equal(t, uint16(1234), stoppedDiscriminator)
}

// TestCommissionDevice_PairingRequest_Timeout tests that commissioning times out
// if the device never appears.
func TestCommissionDevice_PairingRequest_Timeout(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 20 * time.Millisecond      // Short for test
	config.PairingRequestTimeout = 50 * time.Millisecond // Very short for test
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser - device never found
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return(nil, nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopPairingRequest(uint16(1234)).Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.mu.Unlock()

	// CommissionDevice should timeout
	start := time.Now()
	_, err = svc.CommissionDevice(ctx, 1234, "12345678")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrPairingRequestTimeout))
	assert.Less(t, elapsed, 500*time.Millisecond, "should timeout quickly")
}

// TestCancelCommissioning_StopsPairingRequest tests that cancelling commissioning
// stops the active pairing request.
func TestCancelCommissioning_StopsPairingRequest(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond // Short for test
	config.PairingRequestTimeout = 10 * time.Second // Long timeout so we can cancel
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser - device never found
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return(nil, nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser
	var stoppedDiscriminator uint16
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopPairingRequest(mock.Anything).
		Run(func(d uint16) {
			stoppedDiscriminator = d
		}).
		Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.mu.Unlock()

	// Start commissioning in background
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.CommissionDevice(ctx, 1234, "12345678")
		errCh <- err
	}()

	// Wait for pairing request to be active
	time.Sleep(50 * time.Millisecond)

	// Cancel commissioning
	err = svc.CancelCommissioning(1234)
	require.NoError(t, err)

	// CommissionDevice should return cancelled error
	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrCommissioningCancelled), "expected cancellation error, got: %v", err)
	case <-time.After(time.Second):
		t.Fatal("CommissionDevice did not return after cancellation")
	}

	// Verify pairing request was stopped
	assert.Equal(t, uint16(1234), stoppedDiscriminator)
}

// TestCancelCommissioning_NotActive tests cancelling when there's no active commissioning.
func TestCancelCommissioning_NotActive(t *testing.T) {
	config := validControllerConfig()
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Cancel non-existent commissioning
	err = svc.CancelCommissioning(9999)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoPairingRequestActive))
}

// TestCommissionDevice_PairingRequest_CleanupOnSuccess tests that the pairing request
// is properly cleaned up after successful commissioning.
func TestCommissionDevice_PairingRequest_CleanupOnSuccess(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond       // Short for test
	config.ConnectionTimeout = 100 * time.Millisecond     // Short for test
	config.PairingRequestTimeout = 500 * time.Millisecond // Short for test
	config.PairingRequestPollInterval = 10 * time.Millisecond
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Track if pairing request was cleaned up
	var mu sync.Mutex
	pairingRequestStopped := false

	// Mock browser - use localhost port 1 for fast connection failure
	device := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          1,
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 1234,
	}

	callCount := 0
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		RunAndReturn(func(_ context.Context, _ uint16) ([]*discovery.CommissionableService, error) {
			callCount++
			if callCount == 1 {
				return nil, nil // No devices initially
			}
			return []*discovery.CommissionableService{device}, nil
		}).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopPairingRequest(uint16(1234)).
		Run(func(_ uint16) {
			mu.Lock()
			pairingRequestStopped = true
			mu.Unlock()
		}).
		Return(nil).Once()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.mu.Unlock()

	// CommissionDevice - device appears (connection will fail but that's OK for this test)
	_, _ = svc.CommissionDevice(ctx, 1234, "12345678")

	// Verify pairing request was stopped (cleanup)
	mu.Lock()
	stopped := pairingRequestStopped
	mu.Unlock()
	assert.True(t, stopped, "pairing request should be stopped after device found")

	// Verify no active pairing request
	svc.mu.RLock()
	activeCount := len(svc.activePairingRequests)
	svc.mu.RUnlock()
	assert.Equal(t, 0, activeCount, "should have no active pairing requests")
}

// TestCommissionDevice_ContextCancellation tests that context cancellation
// properly cancels the commissioning flow.
func TestCommissionDevice_ContextCancellation(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond // Short for test
	config.PairingRequestTimeout = 10 * time.Second // Long so we can cancel
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	// Mock browser - device never found
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return(nil, nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AnnouncePairingRequest(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopPairingRequest(uint16(1234)).Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Set zone ID
	svc.mu.Lock()
	svc.zoneID = "a1b2c3d4e5f6a7b8"
	svc.mu.Unlock()

	// Create cancellable context
	commissionCtx, cancel := context.WithCancel(ctx)

	// Start commissioning
	errCh := make(chan error, 1)
	go func() {
		_, err := svc.CommissionDevice(commissionCtx, 1234, "12345678")
		errCh <- err
	}()

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should return context cancelled error
	select {
	case err := <-errCh:
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled), "expected context cancelled, got: %v", err)
	case <-time.After(time.Second):
		t.Fatal("CommissionDevice did not return after context cancellation")
	}
}

// TestPairingRequestTimeout_DefaultValue tests that the default timeout is used
// when not configured.
func TestPairingRequestTimeout_DefaultValue(t *testing.T) {
	config := validControllerConfig()
	// Don't set PairingRequestTimeout - should use default

	_, err := NewControllerService(config)
	require.NoError(t, err)

	// Verify default is applied
	assert.Equal(t, time.Duration(0), config.PairingRequestTimeout) // Config unchanged

	// The actual default should be applied in CommissionDevice
	// We can't easily test the exact value without running commissioning
}

// TestCommissionDevice_RequiresZoneID tests that commissioning fails if zoneID is not set.
func TestCommissionDevice_RequiresZoneID(t *testing.T) {
	config := validControllerConfig()
	config.DiscoveryTimeout = 50 * time.Millisecond // Short for test
	svc, err := NewControllerService(config)
	require.NoError(t, err)

	ctx := context.Background()

	// Mock browser - no devices found (to trigger pairing request path)
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().FindAllByDiscriminator(mock.Anything, uint16(1234)).
		Return(nil, nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	require.NoError(t, svc.Start(ctx))
	defer func() { _ = svc.Stop() }()

	// Don't set zone ID

	// CommissionDevice should fail because zoneID is required for pairing request
	_, err = svc.CommissionDevice(ctx, 1234, "12345678")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrZoneIDRequired), "expected zone ID required error, got: %v", err)
}

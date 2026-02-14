package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// startDeviceWithEnableKey creates and starts a DeviceService with a valid
// enable-key for testing auto-reentry behavior.
func startDeviceWithEnableKey(t *testing.T) *DeviceService {
	t.Helper()

	device := model.NewDevice("reentry-test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.TestEnableKey = "00112233445566778899aabbccddeeff"

	svc, err := NewDeviceService(device, config)
	require.NoError(t, err)

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() { _ = svc.Stop() })

	return svc
}

// TestHandleZoneDisconnect_ReentersCommissioning verifies that when all zones
// are disconnected in enable-key mode, HandleZoneDisconnect calls
// EnterCommissioningMode to re-open the commissioning gate.
func TestHandleZoneDisconnect_ReentersCommissioning(t *testing.T) {
	svc := startDeviceWithEnableKey(t)

	require.NoError(t, svc.EnterCommissioningMode())
	assert.True(t, svc.commissioningOpen.Load(), "gate should be open after EnterCommissioningMode")

	zoneID := "a1b2c3d4e5f6a7b8"

	// Register zone (Connected=false) and then connect it
	svc.RegisterZoneAwaitingConnection(zoneID, cert.ZoneTypeTest)
	svc.mu.Lock()
	if cz := svc.connectedZones[zoneID]; cz != nil {
		cz.Connected = true
	}
	svc.mu.Unlock()

	// Manually close the commissioning gate to test that auto-reentry reopens it
	svc.commissioningOpen.Store(false)

	// Disconnect the zone: all zones now disconnected -> auto-reentry should fire
	svc.HandleZoneDisconnect(zoneID)

	assert.True(t, svc.commissioningOpen.Load(),
		"HandleZoneDisconnect should re-enter commissioning mode when all zones are disconnected")
}

// TestEnterCommissioningMode_IncrementsEpoch verifies that each call to
// EnterCommissioningMode bumps the commissioning epoch.
func TestEnterCommissioningMode_IncrementsEpoch(t *testing.T) {
	svc := startDeviceWithEnableKey(t)

	epoch0 := svc.commissioningEpoch.Load()

	require.NoError(t, svc.EnterCommissioningMode())
	epoch1 := svc.commissioningEpoch.Load()
	assert.Equal(t, epoch0+1, epoch1, "epoch should increment on first Enter")

	// Exit and re-enter
	require.NoError(t, svc.ExitCommissioningMode())
	require.NoError(t, svc.EnterCommissioningMode())
	epoch2 := svc.commissioningEpoch.Load()
	assert.Equal(t, epoch1+1, epoch2, "epoch should increment on second Enter")
}

// TestExitCommissioningForEpoch_SkipsWhenEpochChanged verifies that
// exitCommissioningForEpoch is a no-op when EnterCommissioningMode has been
// called since the epoch was captured. This is the mechanism that prevents
// a stale exit from overriding a concurrent auto-reentry.
func TestExitCommissioningForEpoch_SkipsWhenEpochChanged(t *testing.T) {
	svc := startDeviceWithEnableKey(t)

	require.NoError(t, svc.EnterCommissioningMode())
	staleEpoch := svc.commissioningEpoch.Load()

	// Simulate auto-reentry: exit then re-enter (bumps epoch)
	require.NoError(t, svc.ExitCommissioningMode())
	require.NoError(t, svc.EnterCommissioningMode())
	assert.True(t, svc.commissioningOpen.Load(), "gate should be open after re-enter")

	// The stale exit should be skipped because epoch changed
	err := svc.exitCommissioningForEpoch(staleEpoch)
	assert.NoError(t, err)
	assert.True(t, svc.commissioningOpen.Load(),
		"gate must remain open: stale exitCommissioningForEpoch should be a no-op")
}

// TestExitCommissioningForEpoch_ProceedsWhenEpochMatches verifies that
// exitCommissioningForEpoch closes the gate when the epoch is current.
func TestExitCommissioningForEpoch_ProceedsWhenEpochMatches(t *testing.T) {
	svc := startDeviceWithEnableKey(t)

	require.NoError(t, svc.EnterCommissioningMode())
	currentEpoch := svc.commissioningEpoch.Load()

	// No re-enter happened, so epoch matches
	err := svc.exitCommissioningForEpoch(currentEpoch)
	assert.NoError(t, err)
	assert.False(t, svc.commissioningOpen.Load(),
		"gate should be closed when epoch matches")
}

// TestEpochGuard_CommissionDisconnectCycle reproduces the race condition
// where ExitCommissioningMode fires AFTER HandleZoneDisconnect's auto-reentry,
// and verifies that the epoch guard prevents the stale exit from closing the gate.
//
// Timeline:
//  1. handleCommissioningConnection captures epoch, registers zone
//  2. handleOperationalConnection:   zone connects (Connected=true)
//  3. handleZoneSessionClose:        zone disconnects -> HandleZoneDisconnect
//     -> all disconnected -> EnterCommissioningMode (bumps epoch, opens gate)
//  4. handleCommissioningConnection: exitCommissioningForEpoch (stale epoch, SKIPPED)
func TestEpochGuard_CommissionDisconnectCycle(t *testing.T) {
	svc := startDeviceWithEnableKey(t)

	require.NoError(t, svc.EnterCommissioningMode())

	zoneID := "a1b2c3d4e5f6a7b8"

	// Step 1: Capture epoch before zone registration (what handleCommissioningConnection does)
	exitEpoch := svc.commissioningEpoch.Load()
	svc.RegisterZoneAwaitingConnection(zoneID, cert.ZoneTypeTest)

	// Step 2: Controller reconnects operationally
	svc.mu.Lock()
	if cz := svc.connectedZones[zoneID]; cz != nil {
		cz.Connected = true
	}
	svc.mu.Unlock()

	// Step 3: Controller disconnects -> auto-reentry opens the gate (bumps epoch)
	svc.HandleZoneDisconnect(zoneID)
	assert.True(t, svc.commissioningOpen.Load(),
		"after HandleZoneDisconnect auto-reentry, gate should be open")

	// Step 4: Stale exitCommissioningForEpoch should be skipped (epoch changed)
	err := svc.exitCommissioningForEpoch(exitEpoch)
	assert.NoError(t, err)
	assert.True(t, svc.commissioningOpen.Load(),
		"epoch guard must prevent stale exit from closing the gate")

	// Verify the device can still accept commissioning connections
	conn, err := capTryDialTLS(svc.CommissioningAddr())
	if err != nil {
		t.Fatalf("new commissioning TLS connection should be accepted: %v", err)
	}
	conn.Close()
}

// TestEpochGuard_NoReentry_ExitProceeds verifies that without auto-reentry
// (normal mode, no enable-key), exitCommissioningForEpoch correctly closes
// the gate because the epoch hasn't changed.
func TestEpochGuard_NoReentry_ExitProceeds(t *testing.T) {
	device := model.NewDevice("normal-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	// No TestEnableKey set

	svc, err := NewDeviceService(device, config)
	require.NoError(t, err)

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	require.NoError(t, svc.Start(context.Background()))
	defer func() { _ = svc.Stop() }()

	require.NoError(t, svc.EnterCommissioningMode())
	exitEpoch := svc.commissioningEpoch.Load()

	// No auto-reentry -> epoch unchanged -> exit proceeds
	err = svc.exitCommissioningForEpoch(exitEpoch)
	assert.NoError(t, err)
	assert.False(t, svc.commissioningOpen.Load(),
		"gate should be closed when epoch matches (no auto-reentry)")
}

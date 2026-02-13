package runner

import (
	"context"
	"fmt"
	mathrand "math/rand"
	"sort"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Precondition levels form a hierarchy:
//
//	Level 0: No relevant preconditions     -> no-op
//	Level 1: device_in_commissioning_mode  -> ensure disconnected (clean state)
//	Level 2: tls/connection_established    -> connect
//	Level 3: session/device_commissioned   -> connect + commission (PASE)
const (
	precondLevelNone          = 0
	precondLevelCommissioning = 1
	precondLevelConnected     = 2
	precondLevelCommissioned  = 3
)

// preconditionKeyLevels maps known precondition keys to their required level.
// simulationPreconditionKeys are precondition keys that should be stored in
// execution state so that handlers can adapt their behavior based on the
// simulated scenario (D2D, environment, capacity, etc.).
var simulationPreconditionKeys = map[string]bool{
	// D2D simulation.
	PrecondTwoDevicesSameZone:          true,
	PrecondTwoDevicesDifferentZones:    true,
	PrecondDeviceBCertExpired:          true,
	PrecondTwoDevicesSameDiscriminator: true,
	// Commissioning window simulation.
	PrecondCommissioningWindowAt95s: true,
	// Environment / capacity simulation.
	PrecondDeviceZonesFull:              true,
	PrecondNoDevicesAdvertising:         true,
	PrecondDeviceSRVPresent:             true,
	PrecondDeviceAAAAMissing:            true,
	PrecondDeviceAddressValid:           true,
	PrecondDevicePortClosed:             true,
	PrecondDeviceWillAppearAfterDelay:   true,
	PrecondFiveZonesConnected:           true,
	PrecondTwoZonesConnected:            true,
	PrecondDeviceListening:              true,
	PrecondDeviceInZone:                 true,
	PrecondDeviceInTwoZones:             true,
	PrecondZoneCreated:                  true,
	PrecondMultipleDevicesCommissioning: true,
	PrecondMultipleDevicesCommissioned:  true,
	PrecondMultipleControllersRunning:   true,
	// Device state simulation.
	PrecondDeviceReset:                true,
	PrecondDeviceHasGridZone:          true,
	PrecondDeviceHasLocalZone:         true,
	PrecondDeviceInLocalZone:          true,
	PrecondSessionPreviouslyConnected: true,
	PrecondDeviceHasOneZone:           true,
	PrecondDeviceHasAvailableZoneSlot: true,
	// State-machine simulation.
	PrecondControlState:          true,
	PrecondInitialControlState:   true,
	PrecondProcessState:          true,
	PrecondProcessCapable:        true,
	PrecondDeviceIsPausable:      true,
	PrecondDeviceIsStoppable:     true,
	PrecondFailsafeDurationShort: true,
	// Zone limit/removal simulation.
	PrecondZoneCount:                true,
	PrecondZoneCountAtLeast:         true,
	PrecondNoOtherZonesConnected:    true,
	PrecondAcceptsSetpoints:         true,
	PrecondTwoZonesWithLimits:       true,
	PrecondSecondZoneConnected:      true,
	PrecondNoExistingLimits:         true,
	PrecondZoneHasSetValues:         true,
	PrecondDeviceSupportsProduction: true,
	PrecondDeviceIsBidirectional:    true,
	PrecondDeviceSupportsAsymmetric: true,
}

var preconditionKeyLevels = map[string]int{
	// Level 0: Always-true environment preconditions (no setup needed).
	PrecondDeviceBooted:      precondLevelNone,
	PrecondControllerRunning: precondLevelNone,
	PrecondDeviceInNetwork:   precondLevelNone,
	PrecondDeviceListening:   precondLevelNone,

	// D2D simulation preconditions (no actual connection needed).
	PrecondTwoDevicesSameZone:          precondLevelNone,
	PrecondTwoDevicesDifferentZones:    precondLevelNone,
	PrecondDeviceBCertExpired:          precondLevelNone,
	PrecondTwoDevicesSameDiscriminator: precondLevelNone,

	// Controller preconditions (zone/cert state, no connection needed).
	PrecondZoneCreated:              precondLevelNone,
	PrecondControllerHasCert:        precondLevelNone,
	PrecondControllerCertNearExpiry: precondLevelNone,

	// Environment/negative-test preconditions (simulation, no connection needed).
	PrecondDeviceZonesFull:              precondLevelNone,
	PrecondNoDevicesAdvertising:         precondLevelNone,
	PrecondDeviceSRVPresent:             precondLevelNone,
	PrecondDeviceAAAAMissing:            precondLevelNone,
	PrecondDeviceAddressValid:           precondLevelNone,
	PrecondDevicePortClosed:             precondLevelNone,
	PrecondDeviceWillAppearAfterDelay:   precondLevelNone,
	PrecondFiveZonesConnected:           precondLevelNone,
	PrecondTwoZonesConnected:            precondLevelNone,
	PrecondDeviceInZone:                 precondLevelNone,
	PrecondDeviceInTwoZones:             precondLevelNone,
	PrecondMultipleDevicesCommissioning: precondLevelNone,
	PrecondMultipleDevicesCommissioned:  precondLevelNone,
	PrecondMultipleControllersRunning:   precondLevelNone,

	// Zone management test preconditions (runner-side state, no connection needed).
	PrecondNoZonesConfigured:   precondLevelNone,
	PrecondLocalZoneConfigured: precondLevelNone,
	PrecondTwoZonesConfigured:  precondLevelNone,
	PrecondSubscriptionActive:  precondLevelNone,

	PrecondDeviceInCommissioningMode: precondLevelCommissioning,
	PrecondDeviceUncommissioned:      precondLevelCommissioning,
	PrecondCommissioningWindowOpen:   precondLevelCommissioning,
	PrecondCommissioningWindowClosed: precondLevelCommissioning,
	PrecondCommissioningWindowAt95s:  precondLevelCommissioning,
	PrecondDeviceConnected:           precondLevelConnected,
	PrecondTLSConnectionEstablished:  precondLevelConnected,
	PrecondConnectionEstablished:     precondLevelConnected,
	PrecondDeviceCommissioned:        precondLevelCommissioned,
	PrecondSessionEstablished:        precondLevelCommissioned,

	// Device state preconditions (require commissioned session for read/write).
	PrecondDeviceReset:                precondLevelCommissioned,
	PrecondDeviceHasGridZone:          precondLevelCommissioned,
	PrecondDeviceHasLocalZone:         precondLevelCommissioned,
	PrecondDeviceInLocalZone:          precondLevelCommissioned,
	PrecondSessionPreviouslyConnected:  precondLevelCommissioned,
	PrecondFreshCommission:             precondLevelCommissioned,
	PrecondDeviceHasOneZone:            precondLevelCommissioned,
	PrecondDeviceHasAvailableZoneSlot:  precondLevelCommissioned,

	// State-machine preconditions (require commissioned session).
	PrecondControlState:          precondLevelCommissioned,
	PrecondInitialControlState:   precondLevelCommissioned,
	PrecondProcessState:          precondLevelCommissioned,
	PrecondProcessCapable:        precondLevelCommissioned,
	PrecondDeviceIsPausable:      precondLevelCommissioned,
	PrecondDeviceIsStoppable:     precondLevelCommissioned,
	PrecondFailsafeDurationShort: precondLevelCommissioned,

	// Zone limit/removal test preconditions.
	PrecondZoneCount:                precondLevelCommissioned,
	PrecondZoneCountAtLeast:         precondLevelCommissioned,
	PrecondNoOtherZonesConnected:    precondLevelCommissioned,
	PrecondAcceptsSetpoints:         precondLevelCommissioned,
	PrecondTwoZonesWithLimits:       precondLevelCommissioned,
	PrecondSecondZoneConnected:      precondLevelCommissioned,
	PrecondNoExistingLimits:         precondLevelCommissioned,
	PrecondZoneHasSetValues:         precondLevelCommissioned,
	PrecondDeviceSupportsProduction: precondLevelCommissioned,
	PrecondDeviceIsBidirectional:    precondLevelCommissioned,
	PrecondDeviceSupportsAsymmetric: precondLevelCommissioned,
}

// preconditionLevelFor determines the highest setup level needed for the given conditions.
func preconditionLevelFor(conditions []loader.Condition) int {
	level := precondLevelNone
	for _, cond := range conditions {
		for key, val := range cond {
			// Skip conditions explicitly set to false.
			if b, ok := val.(bool); ok && !b {
				continue
			}
			// Accept boolean true and non-boolean values (e.g., string
			// enum values like control_state: FAILSAFE).
			if l, ok := preconditionKeyLevels[key]; ok && l > level {
				level = l
			}
		}
	}
	return level
}

// preconditionLevel determines the highest setup level needed for the given conditions.
func (r *Runner) preconditionLevel(conditions []loader.Condition) int {
	return preconditionLevelFor(conditions)
}

// hasPrecondition checks if any condition in the list has the given key set to true.
func hasPrecondition(conditions []loader.Condition, key string) bool {
	for _, cond := range conditions {
		if b, ok := cond[key].(bool); ok && b {
			return true
		}
	}
	return false
}

// hasPreconditionInt checks if a precondition has the given key with an int value >= minVal.
func hasPreconditionInt(conditions []loader.Condition, key string, minVal int) bool {
	for _, cond := range conditions {
		if v, ok := cond[key].(int); ok && v >= minVal {
			return true
		}
	}
	return false
}

// hasPreconditionString checks if a precondition has the given key with a non-empty string value.
func hasPreconditionString(conditions []loader.Condition, key string) bool {
	for _, cond := range conditions {
		if s, ok := cond[key].(string); ok && s != "" {
			return true
		}
	}
	return false
}

// preconditionValue returns the string value of a precondition key, or "" if not found.
func preconditionValue(conditions []loader.Condition, key string) string {
	for _, cond := range conditions {
		if s, ok := cond[key].(string); ok {
			return s
		}
	}
	return ""
}

// needsFreshCommission checks if any condition explicitly requests a fresh commissioning cycle.
func needsFreshCommission(conditions []loader.Condition) bool {
	return hasPrecondition(conditions, PrecondFreshCommission)
}

// SortByPreconditionLevel sorts test cases by their required precondition level
// (lowest first). The sort is stable, preserving file order within the same level.
func SortByPreconditionLevel(cases []*loader.TestCase) {
	sort.SliceStable(cases, func(i, j int) bool {
		return preconditionLevelFor(cases[i].Preconditions) <
			preconditionLevelFor(cases[j].Preconditions)
	})
}

// ShuffleWithinLevels randomizes the order of test cases within each
// precondition level group. Cases must already be sorted by level.
// The seed is used for reproducibility.
func ShuffleWithinLevels(cases []*loader.TestCase, seed int64) {
	rng := mathrand.New(mathrand.NewSource(seed))
	i := 0
	for i < len(cases) {
		level := preconditionLevelFor(cases[i].Preconditions)
		j := i + 1
		for j < len(cases) && preconditionLevelFor(cases[j].Preconditions) == level {
			j++
		}
		// Shuffle cases[i:j]
		group := cases[i:j]
		rng.Shuffle(len(group), func(a, b int) {
			group[a], group[b] = group[b], group[a]
		})
		i = j
	}
}

// currentLevel returns the runner's current precondition level based on connection
// and commissioning state.
func (r *Runner) currentLevel() int {
	return r.coordinator.CurrentLevel()
}

// teardownTest is the callback registered with the engine.
// It delegates to the coordinator for test cleanup.
func (r *Runner) teardownTest(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) {
	r.coordinator.TeardownTest(ctx, tc, state)
}

// setupPreconditions is the callback registered with the engine.
// It delegates to the coordinator for all lifecycle orchestration.
func (r *Runner) setupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error {
	return r.coordinator.SetupPreconditions(ctx, tc, state)
}

// sendClearLimitInvoke sends a ClearLimit invoke (direction=nil, i.e. clear both)
// to endpoint 1 / EnergyControl on the device. Used by the no_existing_limits
// precondition to ensure the device has no stale limits from prior tests.
func (r *Runner) sendClearLimitInvoke(_ context.Context) error {
	if r.pool.Main() == nil || !r.pool.Main().isConnected() {
		return fmt.Errorf("no connection for ClearLimit")
	}

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureEnergyControl),
		Payload: &wire.InvokePayload{
			CommandID: features.EnergyControlCmdClearLimit,
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode ClearLimit: %w", err)
	}

	_, err = r.sendRequest(data, "clearLimit", req.MessageID)
	return err
}

// ensureConnected checks if already connected; if not, establishes a commissioning TLS connection.
func (r *Runner) ensureConnected(ctx context.Context, state *engine.ExecutionState) error {
	if r.pool.Main() != nil && r.pool.Main().isConnected() {
		return nil
	}

	// Create a synthetic step to drive handleConnect.
	step := &loader.Step{
		Action: "connect",
		Params: map[string]any{
			KeyCommissioning: true,
		},
	}

	outputs, err := r.handleConnect(ctx, step, state)
	if err != nil {
		return fmt.Errorf("precondition connect failed: %w", err)
	}

	// handleConnect returns connection_established in outputs even on TLS failure.
	if established, ok := outputs[KeyConnectionEstablished].(bool); ok && !established {
		errMsg, _ := outputs[KeyError].(string)
		return fmt.Errorf("precondition connect failed: %s", errMsg)
	}

	return nil
}


// closeActiveZoneConns closes runner-tracked zone connections from previous
// tests. The suite zone lives on suite.Conn() (outside the pool), so it is
// not affected. Use closeAllZoneConns for full cleanup (suite teardown).
func (r *Runner) closeActiveZoneConns() {
	r.closeActiveZoneConnsExcept("")
}

// closeAllZoneConns closes ALL zone connections including the suite zone.
// Used during suite teardown and fresh_commission precondition.
func (r *Runner) closeAllZoneConns() {
	r.closeActiveZoneConnsExcept("")
}

// closeActiveZoneConnsExcept closes all runner-tracked zone connections
// except the one matching exceptKey.
func (r *Runner) closeActiveZoneConnsExcept(exceptKey string) {
	closedAny := false
	for _, key := range r.pool.ZoneKeys() {
		if exceptKey != "" && key == exceptKey {
			r.debugf("closeActiveZoneConns: keeping suite zone %s", key)
			continue
		}
		conn := r.pool.Zone(key)
		if conn == nil {
			r.pool.UntrackZone(key)
			continue
		}
		r.debugf("closeActiveZoneConns: zone %s (state=%v tls=%v raw=%v)",
			key, conn.state, conn.tlsConn != nil, conn.conn != nil)
		// Send explicit RemoveZone before closing so the device can
		// synchronously process the zone removal and re-enter commissioning
		// mode quickly.
		if conn.framer != nil {
			zoneID := r.pool.ZoneID(key)
			if zoneID != "" {
				r.debugf("closeActiveZoneConns: sending RemoveZone for zone %s (zoneID=%s, state=%v)", key, zoneID, conn.state)
				r.sendRemoveZoneOnConn(conn, zoneID)
			}
		}
		if conn.tlsConn != nil || conn.conn != nil {
			closedAny = true
		}
		// Send ControlClose before TCP close.
		if conn.framer != nil {
			closeMsg := &wire.ControlMessage{Type: wire.ControlClose}
			if closeData, err := wire.EncodeControlMessage(closeMsg); err == nil {
				_ = conn.framer.WriteFrame(closeData)
			}
		}
		_ = conn.Close()
		conn.clearConnectionRefs()
		r.pool.UntrackZone(key)
	}
	if closedAny {
		r.paseState = nil
		r.lastDeviceConnClose = time.Now()
	}
}

// disconnectConnection closes the TCP connection but preserves crypto material
// (zoneCA, controllerCert, zoneCAPool) and suite zone identity. Used for L3->L1
// transitions where the connection will be re-established later.
func (r *Runner) disconnectConnection() {
	if r.pool.Main() != nil {
		if r.pool.Main().isConnected() || r.pool.Main().tlsConn != nil || r.pool.Main().conn != nil {
			r.debugf("disconnectConnection: closing (state=%v tls=%v raw=%v)",
				r.pool.Main().state, r.pool.Main().tlsConn != nil, r.pool.Main().conn != nil)
			// Send ControlClose before TCP close.
			if r.pool.Main().framer != nil {
				closeMsg := &wire.ControlMessage{Type: wire.ControlClose}
				if closeData, err := wire.EncodeControlMessage(closeMsg); err == nil {
					_ = r.pool.Main().framer.WriteFrame(closeData)
				}
			}
		}
		_ = r.pool.Main().Close()
		// Nil pointers after close. This is the full-cleanup path --
		// no goroutines should be referencing this connection's framer
		// because disconnectConnection is called between tests, not
		// mid-handler.
		r.pool.Main().clearConnectionRefs()
	}
	r.paseState = nil
}

// ensureDisconnected closes the connection AND clears all crypto material.
// Used when abandoning the suite zone entirely (suite teardown, fresh_commission).
// Clears both current AND suite crypto to prevent stale suite crypto from being
// restored by ensureCommissioned's session-reuse path. Without clearing suite
// crypto, a subsequent fresh commission creates new crypto but suiteZoneCAPool
// still points to the old CA, causing "unknown_ca" TLS failures when the
// session-reuse path restores the stale suite crypto.
func (r *Runner) ensureDisconnected() {
	r.disconnectConnection()
	r.zoneCA = nil
	r.controllerCert = nil
	r.zoneCAPool = nil
	r.issuedDeviceCert = nil
	// suite.Clear() closes the suite zone connection and nils all suite state.
	r.suite.Clear()
}

// sendRemoveZone sends a RemoveZone invoke to the device so it re-enters
// commissioning mode (DEC-059). Errors are ignored because the device may
// have already closed the connection.
func (r *Runner) sendRemoveZone() {
	if r.pool.Main() == nil || !r.pool.Main().isConnected() || r.pool.Main().framer == nil {
		return
	}
	if r.paseState == nil || !r.paseState.completed || r.paseState.sessionKey == nil {
		return
	}

	// Derive zone ID from shared secret (same derivation as device).
	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// Build RemoveZone invoke: endpoint 0, DeviceInfo feature (1), command 0x10.
	req := &wire.Request{
		MessageID:  r.pool.NextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  1, // DeviceInfo
		Payload: &wire.InvokePayload{
			CommandID:  16, // DeviceInfoCmdRemoveZone
			Parameters: map[string]any{"zoneId": zoneID},
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return
	}

	// Best-effort: send and read response with a short deadline to avoid
	// blocking forever when the device closes the connection (e.g., last
	// zone removed triggers commissioning mode).
	if err := r.pool.Main().framer.WriteFrame(data); err != nil {
		return
	}
	if r.pool.Main().tlsConn != nil {
		r.pool.Main().tlsConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	}
	_, _ = r.pool.Main().framer.ReadFrame()
	// Reset deadline for subsequent operations.
	if r.pool.Main().tlsConn != nil {
		r.pool.Main().tlsConn.SetReadDeadline(time.Time{})
	}
}

// sendRemoveZoneOnConn sends a RemoveZone invoke on a specific zone connection.
// Used by closeActiveZoneConns to explicitly remove zones before closing TCP
// connections, giving the device a synchronous signal instead of relying on
// async disconnect detection. A short read deadline prevents blocking when
// the device enters commissioning mode before responding (e.g., last zone).
func (r *Runner) sendRemoveZoneOnConn(conn *Connection, zoneID string) {
	req := &wire.Request{
		MessageID:  r.pool.NextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  1, // DeviceInfo
		Payload: &wire.InvokePayload{
			CommandID:  16, // DeviceInfoCmdRemoveZone
			Parameters: map[string]any{"zoneId": zoneID},
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return
	}

	if err := conn.framer.WriteFrame(data); err != nil {
		return
	}

	// Read response with short deadline. The response arrives in <2ms when
	// it's going to arrive at all. When the last zone is removed, the device
	// enters commissioning mode and may close the connection before
	// responding, so this deadline avoids a long unnecessary wait.
	if conn.tlsConn != nil {
		conn.tlsConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	}
	_, _ = conn.framer.ReadFrame()
}

// hasZoneOfType checks if a zone of the given type exists in zone state.
func hasZoneOfType(zs *zoneState, zoneType string) bool {
	for _, z := range zs.zones {
		if z.ZoneType == zoneType {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CommissioningOps interface implementation
// ---------------------------------------------------------------------------
// These exported wrappers allow the Coordinator to call back into Runner
// without knowing about Runner's internals. Each method delegates to the
// corresponding private method or field.

// EnsureConnected wraps ensureConnected.
func (r *Runner) EnsureConnected(ctx context.Context, state *engine.ExecutionState) error {
	return r.ensureConnected(ctx, state)
}

// EnsureCommissioned wraps ensureCommissioned.
func (r *Runner) EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error {
	return r.ensureCommissioned(ctx, state)
}

// DisconnectConnection wraps disconnectConnection.
func (r *Runner) DisconnectConnection() {
	r.disconnectConnection()
}

// EnsureDisconnected wraps ensureDisconnected.
func (r *Runner) EnsureDisconnected() {
	r.ensureDisconnected()
}

// ReconnectToZone wraps reconnectToZone.
func (r *Runner) ReconnectToZone(state *engine.ExecutionState) error {
	return r.reconnectToZone(state)
}

// ProbeSessionHealth wraps probeSessionHealth.
func (r *Runner) ProbeSessionHealth() error {
	return r.probeSessionHealth()
}

// WaitForCommissioningMode wraps waitForCommissioningMode.
func (r *Runner) WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error {
	return r.waitForCommissioningMode(ctx, timeout)
}

// SendRemoveZone wraps sendRemoveZone.
func (r *Runner) SendRemoveZone() {
	r.sendRemoveZone()
}

// SendRemoveZoneOnConn wraps sendRemoveZoneOnConn.
func (r *Runner) SendRemoveZoneOnConn(conn *Connection, zoneID string) {
	r.sendRemoveZoneOnConn(conn, zoneID)
}

// SendTriggerViaZone wraps sendTriggerViaZone.
func (r *Runner) SendTriggerViaZone(ctx context.Context, trigger uint64, state *engine.ExecutionState) error {
	return r.sendTriggerViaZone(ctx, trigger, state)
}

// SendClearLimitInvoke wraps sendClearLimitInvoke.
func (r *Runner) SendClearLimitInvoke(ctx context.Context) error {
	return r.sendClearLimitInvoke(ctx)
}

// PASEState returns the current PASE state.
func (r *Runner) PASEState() *PASEState {
	return r.paseState
}

// SetPASEState sets the current PASE state.
func (r *Runner) SetPASEState(ps *PASEState) {
	r.paseState = ps
}

// DeviceStateModified returns whether device state has been modified.
func (r *Runner) DeviceStateModified() bool {
	return r.deviceStateModified
}

// SetDeviceStateModified sets the device state modified flag.
func (r *Runner) SetDeviceStateModified(modified bool) {
	r.deviceStateModified = modified
}

// WorkingCrypto returns the current working crypto material.
func (r *Runner) WorkingCrypto() CryptoState {
	return CryptoState{
		ZoneCA:           r.zoneCA,
		ControllerCert:   r.controllerCert,
		ZoneCAPool:       r.zoneCAPool,
		IssuedDeviceCert: r.issuedDeviceCert,
	}
}

// SetWorkingCrypto replaces the working crypto material.
func (r *Runner) SetWorkingCrypto(crypto CryptoState) {
	r.zoneCA = crypto.ZoneCA
	r.controllerCert = crypto.ControllerCert
	r.zoneCAPool = crypto.ZoneCAPool
	r.issuedDeviceCert = crypto.IssuedDeviceCert
}

// ClearWorkingCrypto nils all working crypto fields.
func (r *Runner) ClearWorkingCrypto() {
	r.zoneCA = nil
	r.controllerCert = nil
	r.zoneCAPool = nil
	r.issuedDeviceCert = nil
}

// CommissionZoneType returns the current commission zone type.
func (r *Runner) CommissionZoneType() cert.ZoneType {
	return r.commissionZoneType
}

// SetCommissionZoneType sets the commission zone type.
func (r *Runner) SetCommissionZoneType(zt cert.ZoneType) {
	r.commissionZoneType = zt
}

// DiscoveredDiscriminator returns the mDNS-discovered discriminator.
func (r *Runner) DiscoveredDiscriminator() uint16 {
	return r.discoveredDiscriminator
}

// LastDeviceConnClose returns when zone connections were last closed.
func (r *Runner) LastDeviceConnClose() time.Time {
	return r.lastDeviceConnClose
}

// SetLastDeviceConnClose sets the last device connection close time.
func (r *Runner) SetLastDeviceConnClose(t time.Time) {
	r.lastDeviceConnClose = t
}

// IsSuiteZoneCommission wraps isSuiteZoneCommission.
func (r *Runner) IsSuiteZoneCommission() bool {
	return r.isSuiteZoneCommission()
}

// RequestDeviceState wraps requestDeviceState.
func (r *Runner) RequestDeviceState(ctx context.Context, state *engine.ExecutionState) DeviceStateSnapshot {
	return r.requestDeviceState(ctx, state)
}

// DebugSnapshot wraps debugSnapshot.
func (r *Runner) DebugSnapshot(label string) {
	r.debugSnapshot(label)
}

// HandlePreconditionCases processes the boolean precondition switch that sets
// up zone state, crypto, and multi-zone commissioning. Extracted from
// setupPreconditions so the coordinator can delegate it back to Runner.
func (r *Runner) HandlePreconditionCases(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState,
	preconds []loader.Condition, needsMultiZone *bool) error {
	for _, cond := range preconds {
		for key, val := range cond {
			b, ok := val.(bool)
			if !ok || !b {
				continue
			}
			switch key {
			case PrecondZoneCreated, PrecondControllerHasCert:
				// Create a default zone (generates Zone CA + controller cert).
				if r.zoneCA == nil {
					step := &loader.Step{Params: map[string]any{KeyZoneType: "LOCAL"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
			case PrecondDeviceHasGridZone:
				zs := getZoneState(state)
				if !hasZoneOfType(zs, ZoneTypeGrid) {
					step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeGrid, KeyZoneID: "GRID"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
				r.commissionZoneType = cert.ZoneTypeGrid
			case PrecondDeviceHasLocalZone:
				zs := getZoneState(state)
				if !hasZoneOfType(zs, ZoneTypeLocal) {
					step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "LOCAL"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
				r.commissionZoneType = cert.ZoneTypeLocal
			case PrecondDeviceInLocalZone:
				zs := getZoneState(state)
				if !hasZoneOfType(zs, ZoneTypeLocal) {
					step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "LOCAL"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
				if len(zs.zones) == 0 {
					zoneID := "sim-local-zone"
					zs.zones[zoneID] = &zoneInfo{
						ZoneID:         zoneID,
						ZoneType:       ZoneTypeLocal,
						Priority:       zonePriority[ZoneTypeLocal],
						Connected:      true,
						Metadata:       make(map[string]any),
						CommissionedAt: time.Now(),
					}
					zs.zoneOrder = append(zs.zoneOrder, zoneID)
					state.Set(StateLocalZoneID, zoneID)
				}
				r.commissionZoneType = cert.ZoneTypeLocal
			case PrecondNoZonesConfigured:
				zs := getZoneState(state)
				zs.zones = make(map[string]*zoneInfo)
				zs.zoneOrder = nil
			case PrecondLocalZoneConfigured:
				zs := getZoneState(state)
				if _, exists := zs.zones["zone-local-001"]; !exists {
					zs.zones["zone-local-001"] = &zoneInfo{
						ZoneID:         "zone-local-001",
						ZoneType:       ZoneTypeLocal,
						Priority:       zonePriority[ZoneTypeLocal],
						Connected:      false,
						Metadata:       make(map[string]any),
						CommissionedAt: time.Now(),
					}
					zs.zoneOrder = append(zs.zoneOrder, "zone-local-001")
				}
			case PrecondTwoZonesConfigured:
				zs := getZoneState(state)
				if _, exists := zs.zones["zone-grid-001"]; !exists {
					zs.zones["zone-grid-001"] = &zoneInfo{
						ZoneID:         "zone-grid-001",
						ZoneType:       ZoneTypeGrid,
						Priority:       zonePriority[ZoneTypeGrid],
						Connected:      false,
						Metadata:       make(map[string]any),
						CommissionedAt: time.Now(),
					}
					zs.zoneOrder = append(zs.zoneOrder, "zone-grid-001")
				}
				if _, exists := zs.zones["zone-local-001"]; !exists {
					zs.zones["zone-local-001"] = &zoneInfo{
						ZoneID:         "zone-local-001",
						ZoneType:       ZoneTypeLocal,
						Priority:       zonePriority[ZoneTypeLocal],
						Connected:      false,
						Metadata:       make(map[string]any),
						CommissionedAt: time.Now(),
					}
					zs.zoneOrder = append(zs.zoneOrder, "zone-local-001")
				}
			case PrecondCommissioningWindowClosed:
				state.Set(StateCommissioningActive, false)
			case PrecondDeviceInZone:
				zs := getZoneState(state)
				if len(zs.zones) == 0 {
					zoneID := "sim-local-zone"
					zs.zones[zoneID] = &zoneInfo{
						ZoneID:         zoneID,
						ZoneType:       ZoneTypeLocal,
						Priority:       zonePriority[ZoneTypeLocal],
						Connected:      true,
						Metadata:       make(map[string]any),
						CommissionedAt: time.Now(),
					}
					zs.zoneOrder = append(zs.zoneOrder, zoneID)
					state.Set(StateLocalZoneID, zoneID)
				}
			case PrecondCommissioningWindowAt95s:
				state.Set(StateCommWindowStart, time.Now().Add(-95*time.Second))
			case PrecondControllerCertNearExpiry:
				state.Set(StateCertDaysUntilExpiry, 29)
			case PrecondFiveZonesConnected:
				ct := getConnectionTracker(state)
				for _, name := range []string{"GRID", "BUILDING", "HOME", "USER1", "USER2"} {
					if _, exists := ct.zoneConnections[name]; !exists {
						ct.zoneConnections[name] = &Connection{state: ConnOperational}
					}
				}
			case PrecondTwoZonesConnected:
				*needsMultiZone = false // Already handled inline.
				ct := getConnectionTracker(state)
				zones := []struct {
					name string
					zt   cert.ZoneType
				}{
					{"GRID", cert.ZoneTypeGrid},
					{"LOCAL", cert.ZoneTypeLocal},
				}
				if r.config.Target != "" {
					r.debugf("two_zones_connected: commissioning against real device")
					if !r.lastDeviceConnClose.IsZero() {
						if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
							r.debugf("two_zones_connected: %v (continuing)", err)
						}
					}
					for i, z := range zones {
						if _, exists := ct.zoneConnections[z.name]; exists {
							r.debugf("two_zones_connected: zone %s already exists, skipping", z.name)
							continue
						}
						r.debugf("two_zones_connected: commissioning zone %s (type=%d)", z.name, z.zt)
						r.debugSnapshot("two_zones_connected BEFORE commission " + z.name)

						if r.pool.Main() != nil && r.pool.Main().isConnected() && r.paseState != nil && r.paseState.completed {
							r.debugf("two_zones_connected: sending RemoveZone before disconnect (zone %d)", i)
							r.sendRemoveZone()
						}

						savedPool := r.zoneCAPool
						r.disconnectConnection()
						r.zoneCA = nil
						r.controllerCert = nil
						r.zoneCAPool = savedPool
						r.issuedDeviceCert = nil

						r.commissionZoneType = z.zt

						if err := r.ensureCommissioned(ctx, state); err != nil {
							r.debugf("two_zones_connected: PASE FAILED for zone %s: %v", z.name, err)
							r.debugSnapshot("two_zones_connected AFTER PASE FAIL " + z.name)
							return fmt.Errorf("precondition two_zones_connected commission zone %s: %w", z.name, err)
						}

						r.debugf("two_zones_connected: zone %s commissioned successfully", z.name)

						zoneConn := r.pool.Main()
						ct.zoneConnections[z.name] = zoneConn
						state.Set(ZoneConnectionStateKey(z.name), zoneConn)

						if r.paseState != nil && r.paseState.sessionKey != nil {
							zID := deriveZoneIDFromSecret(r.paseState.sessionKey)
							r.pool.TrackZone(z.name, zoneConn, zID)

							var stateKey string
							switch z.zt {
							case cert.ZoneTypeGrid:
								stateKey = StateGridZoneID
							case cert.ZoneTypeLocal:
								stateKey = StateLocalZoneID
							case cert.ZoneTypeTest:
								stateKey = StateTestZoneID
							}
							if stateKey != "" {
								state.Set(stateKey, zID)
							}
						}

						if i == len(zones)-1 {
							if err := r.waitForOperationalReady(2 * time.Second); err != nil {
								r.debugf("two_zones_connected: %v (continuing)", err)
							}
						}

						r.pool.SetMain(&Connection{})

						if i < len(zones)-1 {
							if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
								r.debugf("two_zones_connected: %v (continuing)", err)
							}
						}
					}
					firstZone := zones[0]
					if zc, ok := ct.zoneConnections[firstZone.name]; ok && zc.isConnected() {
						r.pool.SetMain(zc)
						r.debugf("two_zones_connected: restored main conn from zone %s", firstZone.name)
					}
					if gridID, ok := state.Get(StateGridZoneID); ok {
						state.Set(StateCurrentZoneID, gridID)
					}
					if localID, ok := state.Get(StateLocalZoneID); ok {
						state.Set(StateOtherZoneID, localID)
					}
					r.commissionZoneType = 0
				} else {
					for _, z := range zones {
						if _, exists := ct.zoneConnections[z.name]; exists {
							continue
						}
						dummyConn := &Connection{state: ConnOperational}
						ct.zoneConnections[z.name] = dummyConn
						r.pool.TrackZone(z.name, dummyConn, z.name)
					}
				}
			case PrecondDeviceZonesFull:
				if r.config.Target != "" {
					r.debugf("device_zones_full: commissioning zones to fill device slots")
					ct := getConnectionTracker(state)

					if !r.lastDeviceConnClose.IsZero() {
						if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
							r.debugf("device_zones_full: %v (continuing)", err)
						}
					}

					zones := []struct {
						name string
						zt   cert.ZoneType
					}{
						{"GRID", cert.ZoneTypeGrid},
						{"LOCAL", cert.ZoneTypeLocal},
					}

					for i, z := range zones {
						if _, exists := ct.zoneConnections[z.name]; exists {
							r.debugf("device_zones_full: zone %s already exists, skipping", z.name)
							continue
						}
						r.debugf("device_zones_full: commissioning zone %s (type=%d)", z.name, z.zt)

						if r.pool.Main() != nil && r.pool.Main().isConnected() && r.paseState != nil && r.paseState.completed {
							r.debugf("device_zones_full: sending RemoveZone before disconnect (zone %d)", i)
							r.sendRemoveZone()
						}

						savedPool := r.zoneCAPool
						r.disconnectConnection()
						r.zoneCA = nil
						r.controllerCert = nil
						r.zoneCAPool = savedPool
						r.issuedDeviceCert = nil

						r.commissionZoneType = z.zt
						if err := r.ensureCommissioned(ctx, state); err != nil {
							r.debugf("device_zones_full: commission zone %s FAILED: %v", z.name, err)
							return fmt.Errorf("precondition device_zones_full commission zone %s: %w", z.name, err)
						}

						r.debugf("device_zones_full: zone %s commissioned successfully", z.name)

						zoneConn := r.pool.Main()
						ct.zoneConnections[z.name] = zoneConn
						state.Set(ZoneConnectionStateKey(z.name), zoneConn)

						if r.paseState != nil && r.paseState.sessionKey != nil {
							zID := deriveZoneIDFromSecret(r.paseState.sessionKey)
							r.pool.TrackZone(z.name, zoneConn, zID)

							var stateKey string
							switch z.zt {
							case cert.ZoneTypeGrid:
								stateKey = StateGridZoneID
							case cert.ZoneTypeLocal:
								stateKey = StateLocalZoneID
							}
							if stateKey != "" {
								state.Set(stateKey, zID)
							}
						}

						r.pool.SetMain(&Connection{})

						if i < len(zones)-1 {
							for _, tracked := range ct.zoneConnections {
								if tracked != nil && tracked.isConnected() {
									savedConn := r.pool.Main()
									r.pool.SetMain(tracked)
									r.debugf("device_zones_full: triggering enter commissioning mode on tracked zone connection")
									_, _ = r.sendTrigger(ctx, features.TriggerEnterCommissioningMode, state)
									r.pool.SetMain(savedConn)
									break
								}
							}
							if err := r.waitForCommissioningMode(ctx, 5*time.Second); err != nil {
								r.debugf("device_zones_full: %v (continuing)", err)
							}
						}
					}
					if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
						r.debugf("device_zones_full: post-fill %v (continuing)", err)
					}

					r.commissionZoneType = 0
					r.paseState = nil
				}
			}
		}
	}
	return nil
}

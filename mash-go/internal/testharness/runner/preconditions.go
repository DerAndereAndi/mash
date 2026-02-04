package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
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
	// Environment / capacity simulation.
	PrecondDeviceZonesFull:            true,
	PrecondNoDevicesAdvertising:       true,
	PrecondDeviceSRVPresent:           true,
	PrecondDeviceAAAAMissing:          true,
	PrecondDeviceAddressValid:         true,
	PrecondDevicePortClosed:           true,
	PrecondDeviceWillAppearAfterDelay: true,
	PrecondFiveZonesConnected:         true,
	PrecondTwoZonesConnected:          true,
	PrecondDeviceListening:            true,
	PrecondDeviceInZone:               true,
	PrecondDeviceInTwoZones:           true,
	PrecondZoneCreated:                true,
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
	PrecondDeviceZonesFull:            precondLevelNone,
	PrecondNoDevicesAdvertising:       precondLevelNone,
	PrecondDeviceSRVPresent:           precondLevelNone,
	PrecondDeviceAAAAMissing:          precondLevelNone,
	PrecondDeviceAddressValid:         precondLevelNone,
	PrecondDevicePortClosed:           precondLevelNone,
	PrecondDeviceWillAppearAfterDelay: precondLevelNone,
	PrecondFiveZonesConnected:         precondLevelNone,
	PrecondTwoZonesConnected:          precondLevelNone,
	PrecondDeviceInZone:               precondLevelNone,
	PrecondDeviceInTwoZones:           precondLevelNone,

	PrecondDeviceInCommissioningMode: precondLevelCommissioning,
	PrecondDeviceUncommissioned:      precondLevelCommissioning,
	PrecondCommissioningWindowOpen:   precondLevelCommissioning,
	PrecondDeviceConnected:           precondLevelConnected,
	PrecondTLSConnectionEstablished:  precondLevelConnected,
	PrecondConnectionEstablished:     precondLevelConnected,
	PrecondDeviceCommissioned:        precondLevelCommissioned,
	PrecondSessionEstablished:        precondLevelCommissioned,
}

// preconditionLevelFor determines the highest setup level needed for the given conditions.
func preconditionLevelFor(conditions []loader.Condition) int {
	level := precondLevelNone
	for _, cond := range conditions {
		for key, val := range cond {
			// Only consider conditions set to true.
			if b, ok := val.(bool); !ok || !b {
				continue
			}
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

// SortByPreconditionLevel sorts test cases by their required precondition level
// (lowest first). The sort is stable, preserving file order within the same level.
func SortByPreconditionLevel(cases []*loader.TestCase) {
	sort.SliceStable(cases, func(i, j int) bool {
		return preconditionLevelFor(cases[i].Preconditions) <
			preconditionLevelFor(cases[j].Preconditions)
	})
}

// currentLevel returns the runner's current precondition level based on connection
// and commissioning state.
func (r *Runner) currentLevel() int {
	if r.paseState != nil && r.paseState.completed {
		return precondLevelCommissioned
	}
	if r.conn != nil && r.conn.connected {
		return precondLevelConnected
	}
	return precondLevelNone
}

// setupPreconditions is the callback registered with the engine.
// It inspects tc.Preconditions and ensures the runner is in the right state.
// When transitioning backwards (e.g., from commissioned to commissioning),
// it disconnects to give the device a clean state.
func (r *Runner) setupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error {
	// Populate setup_code so that test steps using ${setup_code} resolve correctly.
	if r.config.SetupCode != "" {
		state.Set(StateSetupCode, r.config.SetupCode)
	}

	// Compute the needed precondition level early so we can clear stale
	// zone state before processing special preconditions or connections.
	needed := r.preconditionLevel(tc.Preconditions)
	current := r.currentLevel()

	r.debugf("setupPreconditions %s: current=%d needed=%d", tc.ID, current, needed)
	r.debugSnapshot("setupPreconditions BEFORE " + tc.ID)

	// Clear stale zone CA state for non-commissioned tests. This prevents
	// a zone CA from a previous commissioned test from causing TLS
	// verification failures on subsequent connection-level or lower tests.
	// Skip clearing when the test needs zone connections (two_zones_connected),
	// since those require the zone CA and controller cert for operational TLS.
	needsZoneConns := hasPrecondition(tc.Preconditions, PrecondTwoZonesConnected)
	if needed < precondLevelCommissioned && !needsZoneConns {
		if r.zoneCA != nil || r.controllerCert != nil || r.zoneCAPool != nil {
			r.debugf("clearing stale zone CA state (needed=%d < commissioned)", needed)
		}
		r.zoneCA = nil
		r.controllerCert = nil
		r.zoneCAPool = nil
	}

	// Close stale zone connections from previous tests so the device marks
	// those zones as disconnected and can accept new connections.
	hadActive := len(r.activeZoneConns) > 0
	if hadActive {
		r.debugf("closing %d stale zone connections", len(r.activeZoneConns))
	}
	r.closeActiveZoneConns()
	// Brief pause to let the device process the disconnections before we
	// attempt to establish new connections.
	if hadActive && r.config.Target != "" {
		r.debugf("sleeping 50ms for device to process zone disconnections")
		time.Sleep(50 * time.Millisecond)
	}

	// Force-close any stale main connection whose socket is still open
	// despite being marked as disconnected. A failed request sets
	// connected=false without closing the socket, leaving the device
	// zone active and preventing commissioning mode.
	if r.conn != nil && !r.conn.connected && (r.conn.tlsConn != nil || r.conn.conn != nil) {
		r.debugf("closing phantom main socket (connected=false but socket open)")
		_ = r.conn.Close()
		if r.config.Target != "" {
			r.lastDeviceConnClose = time.Now()
		}
	}

	// DEC-059: On backward transition from commissioned, send RemoveZone
	// so the device re-enters commissioning mode before we disconnect.
	// This must happen before special precondition handlers (e.g.,
	// PrecondTwoZonesConnected) that need the device in commissioning mode.
	if current >= precondLevelCommissioned && needed <= precondLevelCommissioning {
		r.debugf("backward transition: sending RemoveZone (current=%d -> needed=%d)", current, needed)
		r.sendRemoveZone()
	}

	// Backwards transition: disconnect to give the device a clean state.
	if needed < current && needed <= precondLevelCommissioning {
		r.debugf("backward transition: disconnecting (current=%d -> needed=%d)", current, needed)
		r.ensureDisconnected()
		if r.config.Target != "" {
			r.lastDeviceConnClose = time.Now()
		}
	}

	// Store simulation precondition keys in state so handlers can check them.
	for _, cond := range tc.Preconditions {
		for key, val := range cond {
			if simulationPreconditionKeys[key] {
				if b, ok := val.(bool); ok {
					state.Set(key, b)
				}
			}
		}
	}

	// Handle preconditions that require special setup.
	for _, cond := range tc.Preconditions {
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
			case PrecondControllerCertNearExpiry:
				state.Set(StateCertDaysUntilExpiry, 29)
			case PrecondFiveZonesConnected:
				// Pre-populate connection tracker with 5 dummy zone connections.
				ct := getConnectionTracker(state)
				for _, name := range []string{"GRID", "BUILDING", "HOME", "USER1", "USER2"} {
					if _, exists := ct.zoneConnections[name]; !exists {
						ct.zoneConnections[name] = &Connection{connected: true}
					}
				}
			case PrecondTwoZonesConnected:
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
				// Wait for the device to finish processing zone removals
				// from a previous test. closeActiveZoneConns may have run
				// in an earlier test's precondition setup, so hadActive was
				// false for this test even though the device is still busy
				// with RemoveZone -> EnterCommissioningMode transitions.
				if !r.lastDeviceConnClose.IsZero() {
					elapsed := time.Since(r.lastDeviceConnClose)
					minWait := 1500 * time.Millisecond
					if elapsed < minWait {
						wait := minWait - elapsed
						r.debugf("two_zones_connected: waiting %s for device recovery (elapsed %s)", wait.Round(time.Millisecond), elapsed.Round(time.Millisecond))
						time.Sleep(wait)
					}
				}

				// Commission each zone separately. Each PASE session
				// creates a new zone on the device, and the PASE
				// connection becomes that zone's live framer for
				// subscribe/read/write operations.
				for i, z := range zones {
					if _, exists := ct.zoneConnections[z.name]; exists {
						r.debugf("two_zones_connected: zone %s already exists, skipping", z.name)
						continue
					}
					r.debugf("two_zones_connected: commissioning zone %s (type=%d)", z.name, z.zt)
					r.debugSnapshot("two_zones_connected BEFORE commission " + z.name)

					// Reset connection and PASE state for fresh commission.
					// Save and restore the accumulated zone CA pool so
					// that earlier zones' CAs survive across commissions.
					savedPool := r.zoneCAPool
					r.ensureDisconnected()
					r.zoneCAPool = savedPool

					// Set zone type so performCertExchange generates the
					// correct Zone CA (GRID vs LOCAL).
					r.commissionZoneType = z.zt

					if err := r.ensureCommissioned(ctx, state); err != nil {
						r.debugf("two_zones_connected: PASE FAILED for zone %s: %v", z.name, err)
						r.debugSnapshot("two_zones_connected AFTER PASE FAIL " + z.name)
						return fmt.Errorf("precondition two_zones_connected commission zone %s: %w", z.name, err)
					}

					r.debugf("two_zones_connected: zone %s commissioned successfully", z.name)

					// Move the PASE connection to the zone tracker.
					zoneConn := r.conn
					ct.zoneConnections[z.name] = zoneConn
					r.activeZoneConns[z.name] = zoneConn
					state.Set(ZoneConnectionStateKey(z.name), zoneConn)

					// Store zone ID for explicit RemoveZone on teardown.
					if r.paseState != nil && r.paseState.sessionKey != nil {
						r.activeZoneIDs[z.name] = deriveZoneIDFromSecret(r.paseState.sessionKey)
					}

					// Detach from runner so the next iteration
					// creates a fresh connection.
					r.conn = &Connection{}

					// If not last zone, wait for device to re-enter
					// commissioning mode in test-mode.
					if i < len(zones)-1 {
						r.debugf("two_zones_connected: waiting 600ms for device to re-enter commissioning mode")
						time.Sleep(600 * time.Millisecond)
					}
				}
				// Reset commission zone type to default.
				r.commissionZoneType = 0
			} else {
				// No target available (unit tests): use dummy connections.
				for _, z := range zones {
					if _, exists := ct.zoneConnections[z.name]; exists {
						continue
					}
					dummyConn := &Connection{connected: true}
					ct.zoneConnections[z.name] = dummyConn
					r.activeZoneConns[z.name] = dummyConn
				}
			}
			}
		}
	}

	r.debugSnapshot("setupPreconditions AFTER " + tc.ID)

	switch needed {
	case precondLevelCommissioned:
		r.debugf("ensuring commissioned for %s", tc.ID)
		err := r.ensureCommissioned(ctx, state)
		if err != nil {
			r.debugf("ensureCommissioned FAILED for %s: %v", tc.ID, err)
		}
		return err
	case precondLevelConnected:
		// If currently commissioned but only a connection is needed,
		// disconnect and reconnect for a clean TLS session.
		if current > precondLevelConnected {
			r.debugf("downgrading from commissioned to connected for %s", tc.ID)
			r.ensureDisconnected()
		}
		r.debugf("ensuring connected for %s", tc.ID)
		return r.ensureConnected(ctx, state)
	case precondLevelCommissioning:
		r.debugf("ensuring commissioning mode for %s", tc.ID)
		r.ensureDisconnected()
		return nil
	default:
		return nil
	}
}

// ensureConnected checks if already connected; if not, establishes a commissioning TLS connection.
func (r *Runner) ensureConnected(ctx context.Context, state *engine.ExecutionState) error {
	if r.conn != nil && r.conn.connected {
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

// ensureCommissioned checks if already commissioned; if not, connects and performs PASE.
func (r *Runner) ensureCommissioned(ctx context.Context, state *engine.ExecutionState) error {
	// First ensure we're connected.
	if err := r.ensureConnected(ctx, state); err != nil {
		return err
	}

	// If already commissioned, populate state and return.
	if r.paseState != nil && r.paseState.completed {
		state.Set(KeySessionEstablished, true)
		state.Set(KeyConnectionEstablished, true)
		if r.paseState.sessionKey != nil {
			state.Set(StateSessionKey, r.paseState.sessionKey)
			state.Set(StateSessionKeyLen, len(r.paseState.sessionKey))
		}
		return nil
	}

	// Create a synthetic step to drive handleCommission.
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{},
	}

	// Pass setup_code from config if available.
	if r.config.SetupCode != "" {
		step.Params["setup_code"] = r.config.SetupCode
	}

	_, err := r.handleCommission(ctx, step, state)
	if err != nil {
		// On transient errors (EOF, connection reset, broken pipe), retry
		// up to 2 times after a short delay. The device may still be
		// cycling its commissioning window in test mode, especially after
		// zone removals that trigger mDNS and file I/O operations.
		const maxRetries = 2
		if isTransientError(err) {
			for retry := 1; retry <= maxRetries; retry++ {
				r.ensureDisconnected()
				time.Sleep(1 * time.Second)
				if connErr := r.ensureConnected(ctx, state); connErr != nil {
					return fmt.Errorf("precondition commission retry %d connect failed: %w", retry, connErr)
				}
				_, err = r.handleCommission(ctx, step, state)
				if err == nil {
					return nil
				}
				if !isTransientError(err) {
					break
				}
			}
			return fmt.Errorf("precondition commission failed after %d retries: %w", maxRetries, err)
		}
		return fmt.Errorf("precondition commission failed: %w", err)
	}

	return nil
}

// isTransientError returns true for IO-level errors that may resolve on retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		isNetError(err)
}

// isNetError checks if the error is a network-level error.
func isNetError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

// ensureDisconnected closes any existing connection for a clean start.
// closeActiveZoneConns closes all runner-tracked zone connections from
// previous tests. This ensures the device marks those zones as disconnected
// and can accept new connections.
func (r *Runner) closeActiveZoneConns() {
	closedAny := false
	for id, conn := range r.activeZoneConns {
		r.debugf("closeActiveZoneConns: zone %s (connected=%v tls=%v raw=%v)",
			id, conn.connected, conn.tlsConn != nil, conn.conn != nil)
		// Send explicit RemoveZone before closing so the device can
		// synchronously process the zone removal and re-enter commissioning
		// mode quickly. Without this, the device relies on async TCP
		// disconnect detection which is much slower and each premature
		// PASE attempt further delays recovery.
		if conn.connected && conn.framer != nil {
			if zoneID, ok := r.activeZoneIDs[id]; ok {
				r.debugf("closeActiveZoneConns: sending RemoveZone for zone %s (zoneID=%s)", id, zoneID)
				r.sendRemoveZoneOnConn(conn, zoneID)
			}
		}
		// Always close the underlying socket, even if connected was set
		// to false by a failed request. Without this, the device still
		// sees an active TCP connection and the zone remains "active",
		// preventing it from re-entering commissioning mode.
		if conn.tlsConn != nil || conn.conn != nil {
			closedAny = true
		}
		_ = conn.Close()
		delete(r.activeZoneConns, id)
		delete(r.activeZoneIDs, id)
	}
	if closedAny {
		r.lastDeviceConnClose = time.Now()
	}
}

func (r *Runner) ensureDisconnected() {
	// Always close the socket, not just when connected=true. A failed
	// request sets connected=false without closing the underlying TCP
	// socket, which leaves a phantom zone on the device.
	if r.conn != nil {
		if r.conn.connected || r.conn.tlsConn != nil || r.conn.conn != nil {
			r.debugf("ensureDisconnected: closing (connected=%v tls=%v raw=%v)",
				r.conn.connected, r.conn.tlsConn != nil, r.conn.conn != nil)
		}
		_ = r.conn.Close()
	}
	r.paseState = nil
	r.zoneCA = nil
	r.controllerCert = nil
	r.zoneCAPool = nil
}

// sendRemoveZone sends a RemoveZone invoke to the device so it re-enters
// commissioning mode (DEC-059). Errors are ignored because the device may
// have already closed the connection.
func (r *Runner) sendRemoveZone() {
	if r.conn == nil || !r.conn.connected || r.conn.framer == nil {
		return
	}
	if r.paseState == nil || !r.paseState.completed || r.paseState.sessionKey == nil {
		return
	}

	// Derive zone ID from shared secret (same derivation as device).
	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// Build RemoveZone invoke: endpoint 0, DeviceInfo feature (1), command 0x10.
	req := &wire.Request{
		MessageID:  atomic.AddUint32(&r.messageID, 1),
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

	// Best-effort: send and read response, ignoring errors.
	_, _ = r.sendRequest(data, "remove-zone")
}

// sendRemoveZoneOnConn sends a RemoveZone invoke on a specific zone connection.
// Used by closeActiveZoneConns to explicitly remove zones before closing TCP
// connections, giving the device a synchronous signal instead of relying on
// async disconnect detection. A short read deadline prevents blocking when
// the device enters commissioning mode before responding (e.g., last zone).
func (r *Runner) sendRemoveZoneOnConn(conn *Connection, zoneID string) {
	req := &wire.Request{
		MessageID:  atomic.AddUint32(&r.messageID, 1),
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

// deriveZoneIDFromSecret derives a zone ID from a PASE shared secret
// using the same SHA-256 derivation as the device side.
func deriveZoneIDFromSecret(secret []byte) string {
	hash := sha256.Sum256(secret)
	return hex.EncodeToString(hash[:8])
}

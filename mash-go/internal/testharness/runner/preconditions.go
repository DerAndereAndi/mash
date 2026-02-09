package runner

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
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
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/transport"
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
	PrecondCommissioningWindowAt95s:     true,
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
	PrecondSessionPreviouslyConnected: precondLevelCommissioned,
	PrecondFreshCommission:            precondLevelCommissioned,

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

// teardownTest is called after each test completes (pass or fail).
// It unsubscribes any active subscriptions created during the test and
// closes per-test resources stored in the execution state, such as
// security pool connections opened by handleOpenConnections / handleFillConnections.
// Without this cleanup, subscriptions leak between tests (causing stale
// notifications) and TCP sockets leak until GC (consuming device connection slots).
func (r *Runner) teardownTest(_ context.Context, _ *loader.TestCase, state *engine.ExecutionState) {
	// Unsubscribe all active subscriptions created during this test.
	// Each sendUnsubscribe reads the response and discards any interleaved
	// stale notifications, leaving the wire clean for the next test.
	if len(r.activeSubscriptionIDs) > 0 {
		r.debugf("teardown: unsubscribing %d active subscriptions", len(r.activeSubscriptionIDs))
		for _, subID := range r.activeSubscriptionIDs {
			r.sendUnsubscribe(r.conn, subID)
		}
	}
	r.activeSubscriptionIDs = nil
	r.pendingNotifications = nil
	if r.conn != nil {
		r.conn.pendingNotifications = nil
	}
	for _, zc := range r.activeZoneConns {
		if zc != nil {
			zc.pendingNotifications = nil
		}
	}

	// Close connections in a partial/dirty state (e.g., PASE X/Y exchanged
	// but handshake never completed). Without this, the next test inherits a
	// socket that the device considers mid-handshake, causing EOF/reset errors.
	if r.conn != nil && r.conn.connected && (r.paseState == nil || !r.paseState.completed) {
		r.debugf("teardown: closing connection with incomplete PASE state")
		_ = r.conn.Close()
		r.conn.connected = false
		r.paseState = nil
	}

	if secState, ok := state.Custom["security"].(*securityState); ok && secState.pool != nil {
		secState.pool.mu.Lock()
		for _, conn := range secState.pool.connections {
			if conn.connected {
				_ = conn.Close()
			}
		}
		secState.pool.connections = nil
		secState.pool.mu.Unlock()
	}
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

	// Clear stale notification buffer from previous tests.
	r.pendingNotifications = nil

	// Reset device test state between tests when a previous test modified
	// device state via triggers. Use a short timeout -- this is best-effort
	// and must not consume the test's overall timeout budget.
	if r.deviceStateModified && r.config.Target != "" && r.config.EnableKey != "" {
		r.debugf("resetting device test state")
		resetCtx, resetCancel := context.WithTimeout(ctx, 5*time.Second)
		if err := r.sendTriggerViaZone(resetCtx, features.TriggerResetTestState, state); err != nil {
			r.debugf("reset trigger failed: %v (continuing)", err)
		}
		resetCancel()
		r.deviceStateModified = false
	}

	// Clear stale zone CA state for non-commissioned tests. This prevents
	// a zone CA from a previous commissioned test from causing TLS
	// verification failures on subsequent connection-level or lower tests.
	// Skip clearing when the test needs zone connections (two_zones_connected),
	// since those require the zone CA and controller cert for operational TLS.
	needsZoneConns := hasPrecondition(tc.Preconditions, PrecondTwoZonesConnected) ||
		hasPrecondition(tc.Preconditions, PrecondTwoZonesWithLimits) ||
		hasPreconditionInt(tc.Preconditions, PrecondZoneCountAtLeast, 2) ||
		hasPreconditionString(tc.Preconditions, PrecondSecondZoneConnected)
	if needed < precondLevelCommissioned && !needsZoneConns {
		if r.zoneCA != nil || r.controllerCert != nil || r.zoneCAPool != nil {
			r.debugf("clearing stale zone CA state (needed=%d < commissioned)", needed)
		}
		r.zoneCA = nil
		r.controllerCert = nil
		r.zoneCAPool = nil
		r.issuedDeviceCert = nil
	}

	// Close stale zone connections from previous tests so the device marks
	// those zones as disconnected and can accept new connections.
	// When both the current and needed level are "commissioned" and no
	// incompatible preconditions are present, skip the teardown to reuse
	// the existing operational session (saves ~1.5s per test).
	canReuseSession := current >= precondLevelCommissioned &&
		needed >= precondLevelCommissioned &&
		!needsFreshCommission(tc.Preconditions) &&
		!needsZoneConns &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceZonesFull) &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceHasGridZone) &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceHasLocalZone) &&
		!hasPrecondition(tc.Preconditions, PrecondSessionPreviouslyConnected)

	// Verify session is still healthy before reusing. A previous test may
	// have corrupted the connection (protocol errors, partial reads, etc.).
	// If the health check fails, fall through to closeActiveZoneConns.
	// Only probe real device connections -- stub connections in unit tests
	// don't have framers and would always fail the probe.
	if canReuseSession && r.config.Target != "" {
		if err := r.probeSessionHealth(); err != nil {
			r.debugf("session health check failed for %s: %v, falling back to fresh commission", tc.ID, err)
			canReuseSession = false
		}
	}

	if !canReuseSession {
		hadActive := len(r.activeZoneConns) > 0
		if hadActive {
			r.debugf("closing %d stale zone connections", len(r.activeZoneConns))
		}
		r.closeActiveZoneConns()
	} else {
		r.debugf("reusing session for %s (skipping closeActiveZoneConns)", tc.ID)
	}
	// No explicit sleep needed here: subsequent steps use mDNS polling
	// (waitForCommissioningMode) or protocol probes (waitForOperationalReady)
	// to detect when the device is ready.

	// Force-close any stale main connection whose socket is still open
	// despite being marked as disconnected. A failed request sets
	// connected=false without closing the socket, leaving the device
	// zone active and preventing commissioning mode.
	if r.conn != nil && !r.conn.connected && (r.conn.tlsConn != nil || r.conn.conn != nil) {
		r.debugf("closing phantom main socket (connected=false but socket open)")
		_ = r.conn.Close()
		// Reset PASE state so ensureCommissioned performs a fresh PASE
		// handshake on the new connection instead of assuming the old
		// session is still valid.
		r.paseState = nil
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
				state.Set(key, val)
			}
		}
	}

	// Handle non-boolean preconditions that trigger multi-zone setup.
	// zone_count_at_least: 2 and second_zone_connected: GRID trigger
	// the same two_zones_connected behavior.
	needsMultiZone := false
	for _, cond := range tc.Preconditions {
		if v, ok := cond[PrecondZoneCountAtLeast]; ok {
			if n, ok := v.(int); ok && n >= 2 {
				needsMultiZone = true
			}
		}
		if v, ok := cond[PrecondSecondZoneConnected]; ok {
			if s, ok := v.(string); ok && s != "" {
				needsMultiZone = true
			}
		}
		if v, ok := cond[PrecondTwoZonesWithLimits]; ok {
			if b, ok := v.(bool); ok && b {
				needsMultiZone = true
			}
		}
	}

	// If needsMultiZone was set by zone_count_at_least/second_zone_connected
	// but the test doesn't have an explicit two_zones_connected precondition,
	// inject it so the commissioning flow below picks it up.
	preconds := tc.Preconditions
	if needsMultiZone && !hasPrecondition(tc.Preconditions, PrecondTwoZonesConnected) {
		preconds = append(preconds, map[string]any{PrecondTwoZonesConnected: true})
	}

	// Handle preconditions that require special setup.
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
				// Commission as GRID so the real device gets a GRID zone.
				r.commissionZoneType = cert.ZoneTypeGrid
			case PrecondDeviceHasLocalZone:
				zs := getZoneState(state)
				if !hasZoneOfType(zs, ZoneTypeLocal) {
					step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "LOCAL"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
				r.commissionZoneType = cert.ZoneTypeLocal
			case PrecondDeviceInLocalZone:
				// Ensure zone crypto state exists for LOCAL and set commission
				// zone type so the device gets a LOCAL zone.
				zs := getZoneState(state)
				if !hasZoneOfType(zs, ZoneTypeLocal) {
					step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "LOCAL"}}
					_, _ = r.handleCreateZone(ctx, step, state)
				}
				// Pre-populate zone state so zone_count returns the correct
				// count when the test checks for the existing zone.
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
				// Ensure commissioning state is cleared so the test starts
				// with the window closed. This prevents a previous test's
				// stub-mode commissioning from leaking into this test.
				state.Set(StateCommissioningActive, false)
			case PrecondDeviceInZone:
				// Populate zone state with a LOCAL zone so that handlers like
				// zone_count return the correct count. Without this, commission
				// to a second zone (TC-E2E-003) yields count=1 instead of 2.
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
				// Simulate 95 seconds elapsed of a 120-second window.
				// buildBrowseOutput uses this to compute window_expiry_warning.
				state.Set(StateCommWindowStart, time.Now().Add(-95*time.Second))
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
				needsMultiZone = false // Already handled inline.
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
					// and re-enter commissioning mode (mDNS advertisement).
					if !r.lastDeviceConnClose.IsZero() {
						if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
							r.debugf("two_zones_connected: %v (continuing)", err)
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

						// Send RemoveZone before closing so the device can
						// synchronously process zone removal and re-enter
						// commissioning mode. Without this, the device relies
						// on async TCP disconnect detection which is slower.
						if r.conn != nil && r.conn.connected && r.paseState != nil && r.paseState.completed {
							r.debugf("two_zones_connected: sending RemoveZone before disconnect (zone %d)", i)
							r.sendRemoveZone()
						}

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

						// Store zone ID for explicit RemoveZone on teardown
						// and in state for interpolation ({{ grid_zone_id }}).
						if r.paseState != nil && r.paseState.sessionKey != nil {
							zID := deriveZoneIDFromSecret(r.paseState.sessionKey)
							r.activeZoneIDs[z.name] = zID

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

						// For the last zone, verify the device is processing
						// protocol messages before detaching the connection.
						if i == len(zones)-1 {
							if err := r.waitForOperationalReady(2 * time.Second); err != nil {
								r.debugf("two_zones_connected: %v (continuing)", err)
							}
						}

						// Detach from runner so the next iteration
						// creates a fresh connection.
						r.conn = &Connection{}

						if i < len(zones)-1 {
							// Wait for device to re-enter commissioning mode.
							if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
								r.debugf("two_zones_connected: %v (continuing)", err)
							}
						}
					}
					// Restore the first zone's connection (GRID) to r.conn so
					// that ensureCommissioned (called later) sees an active
					// session and does not attempt a third commission on a
					// device that is already at max capacity. We use the
					// first zone because tests that disconnect LOCAL expect
					// GRID (r.conn) to remain alive.
					firstZone := zones[0]
					if zc, ok := ct.zoneConnections[firstZone.name]; ok && zc.connected {
						r.conn = zc
						r.debugf("two_zones_connected: restored r.conn from zone %s", firstZone.name)
					}
					// Set current_zone_id (GRID, since r.conn is restored to GRID)
					// and other_zone_id (LOCAL) for test interpolation.
					if gridID, ok := state.Get(StateGridZoneID); ok {
						state.Set(StateCurrentZoneID, gridID)
					}
					if localID, ok := state.Get(StateLocalZoneID); ok {
						state.Set(StateOtherZoneID, localID)
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
			case PrecondDeviceZonesFull:
				// Real device: commission zones to fill all slots so the
				// device rejects further commissioning with ErrCodeBusy.
				// Simulation mode relies on the state flag set earlier.
				if r.config.Target != "" {
					r.debugf("device_zones_full: commissioning zones to fill device slots")
					ct := getConnectionTracker(state)

					// Wait for device to process prior zone removals.
					if !r.lastDeviceConnClose.IsZero() {
						if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
							r.debugf("device_zones_full: %v (continuing)", err)
						}
					}

					// In test mode the device allows 3 zones (GRID + LOCAL + TEST,
					// DEC-043/DEC-060), so we must fill all three slots.
					zones := []struct {
						name string
						zt   cert.ZoneType
					}{
						{"GRID", cert.ZoneTypeGrid},
						{"LOCAL", cert.ZoneTypeLocal},
						{"TEST", cert.ZoneTypeTest},
					}

					for i, z := range zones {
						if _, exists := ct.zoneConnections[z.name]; exists {
							r.debugf("device_zones_full: zone %s already exists, skipping", z.name)
							continue
						}
						r.debugf("device_zones_full: commissioning zone %s (type=%d)", z.name, z.zt)

						if r.conn != nil && r.conn.connected && r.paseState != nil && r.paseState.completed {
							r.debugf("device_zones_full: sending RemoveZone before disconnect (zone %d)", i)
							r.sendRemoveZone()
						}

						savedPool := r.zoneCAPool
						r.ensureDisconnected()
						r.zoneCAPool = savedPool

						r.commissionZoneType = z.zt
						if err := r.ensureCommissioned(ctx, state); err != nil {
							r.debugf("device_zones_full: commission zone %s FAILED: %v", z.name, err)
							return fmt.Errorf("precondition device_zones_full commission zone %s: %w", z.name, err)
						}

						r.debugf("device_zones_full: zone %s commissioned successfully", z.name)

						zoneConn := r.conn
						ct.zoneConnections[z.name] = zoneConn
						r.activeZoneConns[z.name] = zoneConn
						state.Set(ZoneConnectionStateKey(z.name), zoneConn)

						if r.paseState != nil && r.paseState.sessionKey != nil {
							zID := deriveZoneIDFromSecret(r.paseState.sessionKey)
							r.activeZoneIDs[z.name] = zID

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

						r.conn = &Connection{}

						if i < len(zones)-1 {
							if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
								r.debugf("device_zones_full: %v (continuing)", err)
							}
						}
					}
					// Wait for device to exit and re-enter commissioning mode
					// after filling all zone slots.
					if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
						r.debugf("device_zones_full: post-fill %v (continuing)", err)
					}

					r.commissionZoneType = 0
					// Clear main connection state so the runner does not
					// appear commissioned on a detached connection.
					r.paseState = nil
				}
			}
		}
	}

	// Handle needsMultiZone: set up two_zones_connected when triggered
	// by zone_count_at_least, second_zone_connected, or two_zones_with_limits.
	if needsMultiZone {
		state.Set(PrecondTwoZonesConnected, true)
		ct := getConnectionTracker(state)
		if len(ct.zoneConnections) < 2 {
			// Synthesize zone connections for simulation mode.
			// Real device mode will be handled by ensureCommissioned + the
			// two_zones_connected precondition flag in state.
			if r.config.Target == "" {
				for _, z := range []string{"GRID", "LOCAL"} {
					if _, exists := ct.zoneConnections[z]; !exists {
						dummyConn := &Connection{connected: true}
						ct.zoneConnections[z] = dummyConn
						r.activeZoneConns[z] = dummyConn
					}
				}
			}
		}
	}

	// If a commissioned session exists but isn't tracked in activeZoneConns,
	// it was established by a test step (not by ensureCommissioned which calls
	// transitionToOperational). Such sessions are unreliable: the device may
	// have closed the idle commissioning connection, and the zone type may not
	// match what this test needs. Force a fresh commission.
	// Only apply to real device connections -- unit tests use stub connections
	// that don't need operational transition.
	if needed >= precondLevelCommissioned &&
		r.paseState != nil && r.paseState.completed &&
		len(r.activeZoneConns) == 0 &&
		r.config.Target != "" {
		r.debugf("resetting untracked commission session (no activeZoneConns)")
		r.ensureDisconnected()
	}

	r.debugSnapshot("setupPreconditions AFTER " + tc.ID)

	var setupErr error
	switch needed {
	case precondLevelCommissioned:
		r.debugf("ensuring commissioned for %s", tc.ID)
		setupErr = r.ensureCommissioned(ctx, state)
		if setupErr != nil {
			r.debugf("ensureCommissioned FAILED for %s: %v", tc.ID, setupErr)
			return setupErr
		}
		// Store the commissioned zone ID for test interpolation
		// (e.g. {{ grid_zone_id }}, {{ local_zone_id }}).
		// Also update the simulated zone state entry with the actual
		// device zone ID so that RemoveZone cleanup can find it.
		//
		// Skip when a multi-zone precondition already set zone IDs
		// (two_zones_connected, device_zones_full, etc.). In that case
		// r.paseState holds the LAST zone's key, not the active r.conn
		// zone, so overwriting StateCurrentZoneID here would be wrong.
		if !needsZoneConns && r.paseState != nil && r.paseState.sessionKey != nil {
			zID := deriveZoneIDFromSecret(r.paseState.sessionKey)
			var stateKey, zoneLabel string
			switch r.commissionZoneType {
			case cert.ZoneTypeGrid:
				stateKey = StateGridZoneID
				zoneLabel = ZoneTypeGrid
			case cert.ZoneTypeLocal, 0:
				stateKey = StateLocalZoneID
				zoneLabel = ZoneTypeLocal
			case cert.ZoneTypeTest:
				stateKey = StateTestZoneID
				zoneLabel = ZoneTypeTest
			}
			if stateKey != "" {
				state.Set(stateKey, zID)
			}
			// Set current_zone_id for test interpolation (e.g. {{ current_zone_id }}).
			// This is the zone ID of the most recently commissioned zone.
			state.Set(StateCurrentZoneID, zID)
			// Patch the simulated zone entry with the real zone ID.
			if zoneLabel != "" {
				zs := getZoneState(state)
				if z, exists := zs.zones[zoneLabel]; exists {
					z.ZoneID = zID
				}
			}
		}
	case precondLevelConnected:
		// If currently commissioned but only a connection is needed,
		// disconnect and reconnect for a clean TLS session.
		if current > precondLevelConnected {
			r.debugf("downgrading from commissioned to connected for %s", tc.ID)
			r.ensureDisconnected()
		}
		r.debugf("ensuring connected for %s", tc.ID)
		setupErr = r.ensureConnected(ctx, state)
		if setupErr != nil {
			return setupErr
		}
	case precondLevelCommissioning:
		r.debugf("ensuring commissioning mode for %s", tc.ID)
		r.ensureDisconnected()
		// Wait for device to re-enter commissioning mode (mDNS advertisement).
		if r.config.Target != "" {
			if err := r.waitForCommissioningMode(ctx, 3*time.Second); err != nil {
				r.debugf("commissioning mode: %v (continuing)", err)
			}
			// Wait for any lingering commissioning cooldown to expire so the
			// first PASE attempt in the test doesn't get a "busy" rejection
			// that would skew the PASE backoff counter.
			time.Sleep(1 * time.Second)
			r.lastDeviceConnClose = time.Now()
		}
		state.Set(StateCommissioningActive, true)
	}

	// Send control/process state triggers on real devices so the device
	// matches the simulated preconditions. At baseline these states leaked
	// from prior tests; with session reuse + fresh zone commissions, the
	// device starts in AUTONOMOUS and needs an explicit trigger.
	if r.config.Target != "" && r.config.EnableKey != "" && needed >= precondLevelCommissioned {
		if csVal := preconditionValue(tc.Preconditions, PrecondControlState); csVal != "" {
			var trigger uint64
			switch csVal {
			case ControlStateControlled:
				trigger = features.TriggerControlStateControlled
			case ControlStateFailsafe:
				trigger = features.TriggerControlStateFailsafe
			case ControlStateAutonomous:
				trigger = features.TriggerControlStateAutonomous
			case ControlStateLimited:
				trigger = features.TriggerControlStateLimited
			case ControlStateOverride:
				trigger = features.TriggerControlStateOverride
			}
			if trigger != 0 {
				trigCtx, trigCancel := context.WithTimeout(ctx, 3*time.Second)
				if err := r.sendTriggerViaZone(trigCtx, trigger, state); err != nil {
					r.debugf("control_state trigger failed: %v (continuing)", err)
				}
				trigCancel()
			}
		}
		if psVal := preconditionValue(tc.Preconditions, PrecondProcessState); psVal != "" {
			var trigger uint64
			switch psVal {
			case ProcessStateRunning:
				trigger = features.TriggerProcessStateRunning
			case ProcessStatePaused:
				trigger = features.TriggerProcessStatePaused
			case ProcessStateAvailable:
				trigger = features.TriggerProcessStateAvailable
			case ProcessStateNone:
				trigger = features.TriggerProcessStateNone
			}
			if trigger != 0 {
				trigCtx, trigCancel := context.WithTimeout(ctx, 3*time.Second)
				if err := r.sendTriggerViaZone(trigCtx, trigger, state); err != nil {
					r.debugf("process_state trigger failed: %v (continuing)", err)
				}
				trigCancel()
			}
		}
	}

	// Clear device-side limits when test requires no_existing_limits.
	if r.config.Target != "" && needed >= precondLevelCommissioned &&
		hasPrecondition(tc.Preconditions, PrecondNoExistingLimits) {
		clearCtx, clearCancel := context.WithTimeout(ctx, 3*time.Second)
		if err := r.sendClearLimitInvoke(clearCtx); err != nil {
			r.debugf("no_existing_limits: ClearLimit failed: %v (continuing)", err)
		}
		clearCancel()
	}

	// Post-setup: session_previously_connected disconnects but preserves
	// zone crypto state so the test's connect step can reconnect with
	// operational TLS using the zone CA pool.
	if hasPrecondition(tc.Preconditions, PrecondSessionPreviouslyConnected) {
		r.debugf("session_previously_connected: disconnecting but preserving zone state")
		savedCA := r.zoneCA
		savedCert := r.controllerCert
		savedPool := r.zoneCAPool
		if r.conn != nil {
			_ = r.conn.Close()
		}
		r.conn = &Connection{}
		r.paseState = nil
		r.zoneCA = savedCA
		r.controllerCert = savedCert
		r.zoneCAPool = savedPool
	}

	return nil
}

// sendClearLimitInvoke sends a ClearLimit invoke (direction=nil, i.e. clear both)
// to endpoint 1 / EnergyControl on the device. Used by the no_existing_limits
// precondition to ensure the device has no stale limits from prior tests.
func (r *Runner) sendClearLimitInvoke(_ context.Context) error {
	if r.conn == nil || !r.conn.connected {
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
	// Track whether we already had a live connection before ensureConnected.
	wasConnected := r.conn != nil && r.conn.connected

	r.debugf("ensureCommissioned: wasConnected=%v paseState=%v paseCompleted=%v",
		wasConnected,
		r.paseState != nil,
		r.paseState != nil && r.paseState.completed)

	// First ensure we're connected.
	if err := r.ensureConnected(ctx, state); err != nil {
		r.debugf("ensureCommissioned: ensureConnected failed: %v", err)
		return err
	}

	// If already commissioned AND the connection was already live (not
	// freshly established), we can reuse the existing session. If the
	// connection was dead and ensureConnected created a new one, the
	// old PASE session is invalid -- the device expects PASE on the
	// new connection, so we must redo commissioning.
	if r.paseState != nil && r.paseState.completed {
		if wasConnected {
			r.debugf("ensureCommissioned: reusing existing PASE session")
			state.Set(KeySessionEstablished, true)
			state.Set(KeyConnectionEstablished, true)
			if r.paseState.sessionKey != nil {
				state.Set(StateSessionKey, r.paseState.sessionKey)
				state.Set(StateSessionKeyLen, len(r.paseState.sessionKey))
			}
			return nil
		}
		// Connection was re-established -- old PASE session is stale.
		r.debugf("ensureCommissioned: connection was re-established, clearing stale PASE state")
		r.paseState = nil
	}

	// Create a synthetic step to drive handleCommission.
	// _from_precondition tells handleCommission to skip creating a tracking
	// connection -- ensureCommissioned calls transitionToOperational after,
	// which handles the operational connection and zone registration.
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{
			ParamFromPrecondition: true,
		},
	}

	// Pass setup_code from config if available.
	if r.config.SetupCode != "" {
		step.Params["setup_code"] = r.config.SetupCode
	}

	_, err := r.handleCommission(ctx, step, state)
	if err != nil {
		// DEC-065: If the device rejects with a cooldown error, wait for the
		// remaining cooldown and retry once. This handles transitions between
		// auto-PICS discovery and test execution where the cooldown from the
		// previous commissioning is still active.
		if wait := cooldownRemaining(err); wait > 0 {
			r.debugf("ensureCommissioned: cooldown active, waiting %s", wait.Round(time.Millisecond))
			time.Sleep(wait)
			r.ensureDisconnected()
			if connErr := r.ensureConnected(ctx, state); connErr != nil {
				return fmt.Errorf("precondition commission cooldown retry connect failed: %w", connErr)
			}
			_, err = r.handleCommission(ctx, step, state)
			if err != nil {
				return fmt.Errorf("precondition commission failed after cooldown wait: %w", err)
			}
			if r.paseState == nil || !r.paseState.completed {
				return fmt.Errorf("precondition commission: PASE did not complete after cooldown retry")
			}
			return r.transitionToOperational(state)
		}

		// Zone slots full or transient errors: retry after disconnect + delay.
		// The device auto-evicts disconnected zones in test mode when slots
		// are full, so a simple retry after reconnecting should succeed.

		// On transient errors (EOF, connection reset, broken pipe, zone slots full), retry
		// up to 2 times after a short delay. The device may still be
		// cycling its commissioning window in test mode, especially after
		// zone removals that trigger mDNS and file I/O operations.
		const maxRetries = 2
		if isTransientError(err) || isZoneSlotsFull(err) {
			for retry := 1; retry <= maxRetries; retry++ {
				r.ensureDisconnected()
				time.Sleep(1 * time.Second)
				if connErr := r.ensureConnected(ctx, state); connErr != nil {
					return fmt.Errorf("precondition commission retry %d connect failed: %w", retry, connErr)
				}
				_, err = r.handleCommission(ctx, step, state)
				if err == nil {
					if r.paseState == nil || !r.paseState.completed {
						return fmt.Errorf("precondition commission: PASE did not complete on retry %d", retry)
					}
					return r.transitionToOperational(state)
				}
				if !isTransientError(err) && !isZoneSlotsFull(err) {
					break
				}
			}
			return fmt.Errorf("precondition commission failed after %d retries: %w", maxRetries, err)
		}
		return fmt.Errorf("precondition commission failed: %w", err)
	}

	// handleCommission may return nil error for PASE protocol failures
	// (device-sent error codes). Check paseState to detect these.
	if r.paseState == nil || !r.paseState.completed {
		return fmt.Errorf("precondition commission: PASE handshake did not complete")
	}

	return r.transitionToOperational(state)
}

// transitionToOperational closes the commissioning connection and establishes
// a new operational TLS connection using the controller certificate received
// during cert exchange. This implements DEC-066: the device closes the
// commissioning connection after sending CertInstallAck, and the controller
// must reconnect with operational certificates.
//
// The new connection is registered in activeZoneConns so that
// closeActiveZoneConns can send RemoveZone and close it between tests.
// Without this registration, the zone leaks on the device and subsequent
// tests fail with "zone slots full".
func (r *Runner) transitionToOperational(state *engine.ExecutionState) error {
	if r.paseState == nil || r.paseState.sessionKey == nil {
		return fmt.Errorf("no PASE session to transition")
	}

	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// DEC-066: Close the commissioning connection.
	// The device has already closed its end after sending CertInstallAck.
	if r.conn != nil {
		r.debugf("transitionToOperational: closing commissioning connection")
		_ = r.conn.Close()
		r.conn = nil
	}

	// DEC-066: Establish new operational TLS connection.
	// Retry the dial briefly in case the device hasn't finished registering
	// the zone as awaiting reconnection.
	r.debugf("transitionToOperational: reconnecting with operational TLS")

	tlsConfig := r.operationalTLSConfig()
	var tlsConn *tls.Conn
	var dialErr error
	for attempt := range 3 {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		tlsConn, dialErr = tls.DialWithDialer(dialer, "tcp", r.config.Target, tlsConfig)
		if dialErr == nil {
			break
		}
		r.debugf("transitionToOperational: dial attempt %d failed: %v", attempt+1, dialErr)
		time.Sleep(50 * time.Millisecond)
	}
	if dialErr != nil {
		return fmt.Errorf("operational reconnection failed: %w", dialErr)
	}

	// Create new connection wrapper
	r.conn = &Connection{
		tlsConn:     tlsConn,
		framer:      transport.NewFramer(tlsConn),
		connected:   true,
		operational: true,
	}
	state.Set(StateConnection, r.conn)
	// Record timestamp for verify_timing (TC-TRANS-004).
	state.Set(StateOperationalConnEstablished, time.Now())

	// Verify the device is processing protocol messages on this connection.
	if err := r.waitForOperationalReady(2 * time.Second); err != nil {
		r.debugf("transitionToOperational: %v (continuing)", err)
	}

	// Register the commissioned zone so closeActiveZoneConns can clean it up.
	connKey := "main-" + zoneID
	r.activeZoneConns[connKey] = r.conn
	r.activeZoneIDs[connKey] = zoneID
	r.debugf("transitionToOperational: reconnected and registered zone %s in activeZoneConns", zoneID)

	return nil
}

// cooldownRemaining extracts the remaining cooldown duration from a PASE
// handshake error. Returns 0 if the error is not a cooldown rejection.
// Error format: "...cooldown active (123.456ms remaining)..."
func cooldownRemaining(err error) time.Duration {
	if err == nil {
		return 0
	}
	msg := err.Error()
	const marker = "cooldown active ("
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return 0
	}
	rest := msg[idx+len(marker):]
	endIdx := strings.Index(rest, " remaining)")
	if endIdx < 0 {
		return 0
	}
	d, parseErr := time.ParseDuration(rest[:endIdx])
	if parseErr != nil {
		return 0
	}
	// Add a small buffer so we don't race with the cooldown expiry.
	return d + 50*time.Millisecond
}

// isZoneSlotsFull returns true if the error indicates the device rejected
// commissioning because all zone slots are occupied.
func isZoneSlotsFull(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "zone slots full")
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
		strings.Contains(msg, "deadline exceeded") ||
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
		// Try to send RemoveZone even on "dead" connections (connected=false).
		// TCP is full-duplex: a read failure doesn't always mean the write
		// side is dead. If the write fails, no harm done.
		if conn.framer != nil {
			if zoneID, ok := r.activeZoneIDs[id]; ok {
				r.debugf("closeActiveZoneConns: sending RemoveZone for zone %s (zoneID=%s, connected=%v)", id, zoneID, conn.connected)
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
		// Send ControlClose before TCP close so the device's message
		// loop exits immediately instead of waiting for TCP timeout.
		if conn.framer != nil {
			closeMsg := &wire.ControlMessage{Type: wire.ControlClose}
			if closeData, err := wire.EncodeControlMessage(closeMsg); err == nil {
				_ = conn.framer.WriteFrame(closeData)
			}
		}
		_ = conn.Close()
		delete(r.activeZoneConns, id)
		delete(r.activeZoneIDs, id)
	}
	// Clear PASE state when any zone connections were closed.
	// The runner must re-commission for the next test. Without this,
	// ensureCommissioned sees paseState.completed=true and skips
	// commissioning, but the connection is already closed.
	if closedAny {
		r.paseState = nil
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
			// Send ControlClose before TCP close so the device's message
			// loop exits immediately instead of waiting for TCP timeout.
			if r.conn.framer != nil {
				closeMsg := &wire.ControlMessage{Type: wire.ControlClose}
				if closeData, err := wire.EncodeControlMessage(closeMsg); err == nil {
					_ = r.conn.framer.WriteFrame(closeData)
				}
			}
		}
		_ = r.conn.Close()
	}
	r.paseState = nil
	r.zoneCA = nil
	r.controllerCert = nil
	r.zoneCAPool = nil
	r.issuedDeviceCert = nil
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

	// Best-effort: send and read response with a short deadline to avoid
	// blocking forever when the device closes the connection (e.g., last
	// zone removed triggers commissioning mode).
	if err := r.conn.framer.WriteFrame(data); err != nil {
		return
	}
	if r.conn.tlsConn != nil {
		r.conn.tlsConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	}
	_, _ = r.conn.framer.ReadFrame()
	// Reset deadline for subsequent operations.
	if r.conn.tlsConn != nil {
		r.conn.tlsConn.SetReadDeadline(time.Time{})
	}
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

// hasZoneOfType checks if a zone of the given type exists in zone state.
func hasZoneOfType(zs *zoneState, zoneType string) bool {
	for _, z := range zs.zones {
		if z.ZoneType == zoneType {
			return true
		}
	}
	return false
}

// deriveZoneIDFromSecret derives a zone ID from a PASE shared secret
// using the same SHA-256 derivation as the device side.
func deriveZoneIDFromSecret(secret []byte) string {
	hash := sha256.Sum256(secret)
	return hex.EncodeToString(hash[:8])
}

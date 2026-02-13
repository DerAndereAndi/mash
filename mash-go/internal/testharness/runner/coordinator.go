package runner

import (
	"context"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
)

// Coordinator manages test lifecycle orchestration: setting up preconditions,
// tearing down tests, and tracking the current precondition level. It
// encapsulates the decision tree that was previously embedded in
// Runner.setupPreconditions, making it independently testable via mocks.
type Coordinator interface {
	// SetupPreconditions inspects tc.Preconditions and ensures the runner
	// is in the correct state (connected, commissioned, etc.) before the
	// test executes.
	SetupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error

	// TeardownTest is called after each test completes (pass or fail).
	// It cleans up subscriptions, security pool connections, and stale
	// PASE state so the next test starts clean.
	TeardownTest(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState)

	// CurrentLevel returns the runner's current precondition level based
	// on connection and commissioning state.
	CurrentLevel() int
}

// CommissioningOps defines the operations the coordinator needs from the
// runner to manage test lifecycle. Runner implements this interface with
// exported wrappers around its private methods.
type CommissioningOps interface {
	// Connection lifecycle
	EnsureConnected(ctx context.Context, state *engine.ExecutionState) error
	EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error
	DisconnectConnection()
	EnsureDisconnected()
	ReconnectToZone(state *engine.ExecutionState) error

	// Session health
	ProbeSessionHealth() error
	WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error

	// Wire operations
	SendRemoveZone()
	SendRemoveZoneOnConn(conn *Connection, zoneID string)
	SendTriggerViaZone(ctx context.Context, trigger uint64, state *engine.ExecutionState) error
	SendClearLimitInvoke(ctx context.Context) error

	// State accessors
	PASEState() *PASEState
	SetPASEState(ps *PASEState)
	DeviceStateModified() bool
	SetDeviceStateModified(modified bool)
	WorkingCrypto() CryptoState
	SetWorkingCrypto(crypto CryptoState)
	ClearWorkingCrypto()
	CommissionZoneType() cert.ZoneType
	SetCommissionZoneType(zt cert.ZoneType)
	DiscoveredDiscriminator() uint16
	LastDeviceConnClose() time.Time
	SetLastDeviceConnClose(t time.Time)
	IsSuiteZoneCommission() bool

	// Diagnostics
	RequestDeviceState(ctx context.Context, state *engine.ExecutionState) DeviceStateSnapshot
	DebugSnapshot(label string)

	// Precondition handler cases (380-line switch stays on Runner)
	HandlePreconditionCases(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState,
		preconds []loader.Condition, needsMultiZone *bool) error
}

// coordinatorImpl is the production implementation of Coordinator.
type coordinatorImpl struct {
	suite  SuiteSession
	pool   ConnPool
	ops    CommissioningOps
	config *Config
	debugf func(string, ...any)
}

// NewCoordinator creates a Coordinator backed by the given components.
func NewCoordinator(suite SuiteSession, pool ConnPool, ops CommissioningOps, config *Config, debugf func(string, ...any)) Coordinator {
	return &coordinatorImpl{
		suite:  suite,
		pool:   pool,
		ops:    ops,
		config: config,
		debugf: debugf,
	}
}

// SetupPreconditions inspects tc.Preconditions and ensures the runner is in
// the right state. When transitioning backwards (e.g., from commissioned to
// commissioning), it disconnects to give the device a clean state.
func (c *coordinatorImpl) SetupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error {
	// Populate setup_code so that test steps using ${setup_code} resolve correctly.
	if c.config.SetupCode != "" {
		state.Set(StateSetupCode, c.config.SetupCode)
	}

	// Populate device_discriminator from auto-PICS discovery.
	if c.ops.DiscoveredDiscriminator() > 0 {
		state.Set(StateDeviceDiscriminator, int(c.ops.DiscoveredDiscriminator()))
	}

	// Compute the needed precondition level early.
	needed := preconditionLevelFor(tc.Preconditions)
	current := c.CurrentLevel()

	c.debugf("setupPreconditions %s: current=%d needed=%d", tc.ID, current, needed)
	c.ops.DebugSnapshot("setupPreconditions BEFORE " + tc.ID)

	// Clear stale notification buffer from previous tests.
	c.pool.ClearNotifications()

	// Reset commission zone type to default (LOCAL) between tests.
	c.ops.SetCommissionZoneType(0)

	// Always reset device test state between tests.
	if c.config.Target != "" && c.config.EnableKey != "" {
		c.debugf("resetting device test state")
		resetCtx, resetCancel := context.WithTimeout(ctx, 5*time.Second)
		err := c.ops.SendTriggerViaZone(resetCtx, features.TriggerResetTestState, state)
		resetCancel()
		if err != nil {
			c.debugf("reset trigger failed: %v, attempting reconnect+retry", err)
			if c.suite.ZoneID() != "" && c.ops.WorkingCrypto().ZoneCAPool != nil {
				if reconErr := c.ops.ReconnectToZone(state); reconErr != nil {
					c.debugf("reconnect for reset retry failed: %v (continuing)", reconErr)
				} else {
					retryCtx, retryCancel := context.WithTimeout(ctx, 5*time.Second)
					err = c.ops.SendTriggerViaZone(retryCtx, features.TriggerResetTestState, state)
					retryCancel()
					if err != nil {
						c.debugf("reset trigger retry failed: %v (continuing)", err)
					}
				}
			}
		}
		c.ops.SetDeviceStateModified(false)

		// Capture device state snapshot AFTER reset, BEFORE preconditions.
		snapCtx, snapCancel := context.WithTimeout(ctx, 3*time.Second)
		if before := c.ops.RequestDeviceState(snapCtx, state); before != nil {
			state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(before)
		}
		snapCancel()
	}

	// Clear stale zone CA state for non-commissioned tests.
	needsZoneConns := hasPrecondition(tc.Preconditions, PrecondTwoZonesConnected) ||
		hasPrecondition(tc.Preconditions, PrecondTwoZonesWithLimits) ||
		hasPreconditionInt(tc.Preconditions, PrecondZoneCountAtLeast, 2) ||
		hasPreconditionString(tc.Preconditions, PrecondSecondZoneConnected)
	if needed < precondLevelCommissioned && !needsZoneConns {
		crypto := c.ops.WorkingCrypto()
		if crypto.ZoneCA != nil || crypto.ControllerCert != nil || crypto.ZoneCAPool != nil {
			c.debugf("clearing stale zone CA state (needed=%d < commissioned)", needed)
		}
		c.ops.ClearWorkingCrypto()
	}

	// Connection tier determines session isolation strategy.
	tier := connectionTierFor(tc)
	c.debugf("preconditions: tier=%s needed=%d current=%d for %s", tier, needed, current, tc.ID)

	// Session reuse decision: only application-tier tests can reuse.
	canReuseSession := tier == TierApplication &&
		current >= precondLevelCommissioned &&
		needed >= precondLevelCommissioned &&
		!needsZoneConns &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceZonesFull) &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceHasGridZone) &&
		!hasPrecondition(tc.Preconditions, PrecondDeviceHasLocalZone) &&
		!hasPrecondition(tc.Preconditions, PrecondSessionPreviouslyConnected)

	// Verify session health before reusing.
	if canReuseSession && c.config.Target != "" {
		if err := c.ops.ProbeSessionHealth(); err != nil {
			c.debugf("session health check failed for %s: %v", tc.ID, err)
			if c.suite.ZoneID() != "" {
				c.debugf("attempting reconnect to suite zone %s", c.suite.ZoneID())
				if reconnErr := c.ops.ReconnectToZone(state); reconnErr != nil {
					c.debugf("reconnect failed: %v, falling back to fresh commission", reconnErr)
					canReuseSession = false
				} else {
					c.debugf("reconnected to suite zone %s", c.suite.ZoneID())
				}
			} else {
				canReuseSession = false
			}
		}
	}

	if !canReuseSession {
		hadActive := c.pool.ZoneCount() > 0
		if hadActive {
			c.debugf("closing %d stale zone connections", c.pool.ZoneCount())
		}
		if needsFreshCommission(tc.Preconditions) && c.suite.ZoneID() != "" {
			c.debugf("fresh_commission: removing suite zone %s", c.suite.ZoneID())
			c.pool.CloseAllZones()
			c.ops.EnsureDisconnected()
		} else {
			c.pool.CloseZonesExcept(c.suite.ConnKey())
		}
	} else {
		// Clean up extra non-suite zones from previous multi-zone tests.
		if !needsZoneConns {
			for _, id := range c.pool.ZoneKeys() {
				conn := c.pool.Zone(id)
				if id == c.suite.ConnKey() || conn == c.pool.Main() {
					continue
				}
				c.debugf("reusing session: cleaning up extra zone %s", id)
				if conn != nil && conn.isConnected() && conn.framer != nil {
					zoneID := c.pool.ZoneID(id)
					if zoneID != "" {
						c.ops.SendRemoveZoneOnConn(conn, zoneID)
					}
				}
				if conn != nil {
					conn.transitionTo(ConnDisconnected)
				}
				c.pool.UntrackZone(id)
			}
		}
		c.debugf("reusing session for %s (skipping closeActiveZoneConns)", tc.ID)
	}

	// DEC-059: backward transition from commissioned.
	if current >= precondLevelCommissioned && needed <= precondLevelCommissioning {
		if c.suite.ZoneID() != "" {
			c.debugf("backward transition: detaching main conn (suite zone %s stays connected)", c.suite.ZoneID())
			c.pool.SetMain(&Connection{})
			c.ops.SetPASEState(nil)
		} else {
			c.debugf("backward transition: sending RemoveZone (current=%d -> needed=%d)", current, needed)
			c.ops.SendRemoveZone()
		}
	}

	// Backwards transition: disconnect to give the device a clean state.
	if needed < current && needed <= precondLevelCommissioning {
		c.debugf("backward transition: disconnecting (current=%d -> needed=%d)", current, needed)
		if c.suite.ZoneID() != "" {
			c.pool.SetMain(&Connection{})
			c.ops.SetPASEState(nil)
		} else {
			c.ops.EnsureDisconnected()
		}
		if c.config.Target != "" {
			c.ops.SetLastDeviceConnClose(time.Now())
		}
	}

	// Store simulation precondition keys in state.
	for _, cond := range tc.Preconditions {
		for key, val := range cond {
			if simulationPreconditionKeys[key] {
				state.Set(key, val)
			}
		}
	}

	// Detect multi-zone setup needs.
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

	// Inject two_zones_connected if needed.
	preconds := tc.Preconditions
	if needsMultiZone && !hasPrecondition(tc.Preconditions, PrecondTwoZonesConnected) {
		preconds = append(preconds, map[string]any{PrecondTwoZonesConnected: true})
	}

	// Save crypto state before precondition handlers may replace it.
	savedCrypto := c.ops.WorkingCrypto()
	savedPASE := c.ops.PASEState()

	// Handle preconditions that require special setup.
	if err := c.ops.HandlePreconditionCases(ctx, tc, state, preconds, &needsMultiZone); err != nil {
		return err
	}

	// Handle needsMultiZone fallthrough.
	if needsMultiZone {
		state.Set(PrecondTwoZonesConnected, true)
		ct := getConnectionTracker(state)
		if len(ct.zoneConnections) < 2 {
			if c.config.Target == "" {
				for _, z := range []string{"GRID", "LOCAL"} {
					if _, exists := ct.zoneConnections[z]; !exists {
						dummyConn := &Connection{state: ConnOperational}
						ct.zoneConnections[z] = dummyConn
						c.pool.TrackZone(z, dummyConn, z)
					}
				}
			}
		}
	}

	// Reset untracked commission sessions.
	if needed >= precondLevelCommissioned &&
		c.ops.PASEState().Completed() &&
		c.pool.ZoneCount() == 0 &&
		c.config.Target != "" {
		c.debugf("resetting untracked commission session (no activeZoneConns)")
		c.ops.EnsureDisconnected()
	}

	c.ops.DebugSnapshot("setupPreconditions AFTER " + tc.ID)

	// Level switch: ensure the right connection/commissioning level.
	var setupErr error
	switch needed {
	case precondLevelCommissioned:
		c.debugf("ensuring commissioned for %s", tc.ID)
		setupErr = c.ops.EnsureCommissioned(ctx, state)
		if setupErr != nil {
			c.debugf("ensureCommissioned FAILED for %s: %v", tc.ID, setupErr)
			return setupErr
		}
		// Store zone IDs for test interpolation.
		if !needsZoneConns && c.ops.PASEState().Completed() && c.ops.PASEState().SessionKey() != nil {
			zID := deriveZoneIDFromSecret(c.ops.PASEState().SessionKey())
			var stateKey, zoneLabel string
			switch c.ops.CommissionZoneType() {
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
			state.Set(StateCurrentZoneID, zID)
			if zoneLabel != "" {
				zs := getZoneState(state)
				if z, exists := zs.zones[zoneLabel]; exists {
					z.ZoneID = zID
				}
			}
		}
	case precondLevelConnected:
		if current > precondLevelConnected {
			c.debugf("downgrading from commissioned to connected for %s", tc.ID)
			if c.suite.ZoneID() != "" {
				c.pool.SetMain(&Connection{})
				c.ops.SetPASEState(nil)
			} else {
				c.ops.EnsureDisconnected()
			}
		}
		c.debugf("ensuring connected for %s", tc.ID)
		setupErr = c.ops.EnsureConnected(ctx, state)
		if setupErr != nil {
			return setupErr
		}
	case precondLevelCommissioning:
		c.debugf("ensuring commissioning mode for %s", tc.ID)
		if c.suite.ZoneID() != "" {
			c.debugf("preserving suite zone %s during commissioning setup", c.suite.ZoneID())
			if c.pool.Main() != nil && c.pool.Main().isConnected() {
				c.pool.SetMain(&Connection{})
			}
			c.ops.SetPASEState(nil)
		} else {
			c.ops.EnsureDisconnected()
		}
		if c.config.Target != "" {
			if err := c.ops.WaitForCommissioningMode(ctx, 3*time.Second); err != nil {
				c.debugf("commissioning mode: %v (continuing)", err)
			}
			c.ops.SetLastDeviceConnClose(time.Now())
		}
		state.Set(StateCommissioningActive, true)
	}

	// Crypto restore: if no fresh PASE occurred and crypto was replaced.
	if c.suite.ZoneID() == "" &&
		c.ops.PASEState() == savedPASE && savedPASE != nil && savedPASE.Completed() &&
		c.ops.WorkingCrypto().ZoneCA != savedCrypto.ZoneCA {
		c.ops.SetWorkingCrypto(savedCrypto)
		c.debugf("restored crypto state after session reuse for %s", tc.ID)
	}

	// Send control/process state triggers on real devices.
	if c.config.Target != "" && c.config.EnableKey != "" && needed >= precondLevelCommissioned {
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
				if err := c.ops.SendTriggerViaZone(trigCtx, trigger, state); err != nil {
					c.debugf("control_state trigger failed: %v (continuing)", err)
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
				if err := c.ops.SendTriggerViaZone(trigCtx, trigger, state); err != nil {
					c.debugf("process_state trigger failed: %v (continuing)", err)
				}
				trigCancel()
			}
		}
	}

	// Clear device-side limits when test requires no_existing_limits.
	if c.config.Target != "" && needed >= precondLevelCommissioned &&
		hasPrecondition(tc.Preconditions, PrecondNoExistingLimits) {
		clearCtx, clearCancel := context.WithTimeout(ctx, 3*time.Second)
		if err := c.ops.SendClearLimitInvoke(clearCtx); err != nil {
			c.debugf("no_existing_limits: ClearLimit failed: %v (continuing)", err)
		}
		clearCancel()
	}

	// Post-setup: session_previously_connected.
	if hasPrecondition(tc.Preconditions, PrecondSessionPreviouslyConnected) {
		c.debugf("session_previously_connected: disconnecting but preserving zone state")
		savedCr := c.ops.WorkingCrypto()
		if c.pool.Main() != nil {
			_ = c.pool.Main().Close()
		}
		c.pool.SetMain(&Connection{})
		c.ops.SetPASEState(nil)
		c.ops.SetWorkingCrypto(savedCr)
	}

	return nil
}

// TeardownTest is called after each test completes (pass or fail).
// It cleans up subscriptions, notifications, stale PASE state, and
// per-test security pool connections.
func (c *coordinatorImpl) TeardownTest(_ context.Context, _ *loader.TestCase, state *engine.ExecutionState) {
	// Capture device state snapshot AFTER the test ran, BEFORE cleanup.
	if c.config.Target != "" && c.config.EnableKey != "" {
		afterCtx, afterCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if after := c.ops.RequestDeviceState(afterCtx, state); after != nil {
			state.Custom[engine.StateKeyDeviceStateAfter] = map[string]any(after)

			if beforeRaw, ok := state.Custom[engine.StateKeyDeviceStateBefore]; ok {
				if before, ok := beforeRaw.(map[string]any); ok {
					diffs := diffSnapshots(DeviceStateSnapshot(before), DeviceStateSnapshot(after))
					if len(diffs) > 0 {
						diffMaps := make([]map[string]any, len(diffs))
						for i, d := range diffs {
							diffMaps[i] = map[string]any{
								"key":    d.Key,
								"before": d.Before,
								"after":  d.After,
							}
						}
						state.Custom[engine.StateKeyDeviceStateDiffs] = diffMaps
						c.debugf("teardown: baseline diverged on %d fields, re-resetting", len(diffs))

						// Re-reset and verify baseline is restored.
						resetCtx, resetCancel := context.WithTimeout(context.Background(), 5*time.Second)
						_ = c.ops.SendTriggerViaZone(resetCtx, features.TriggerResetTestState, state)
						resetCancel()

						verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 3*time.Second)
						if recheck := c.ops.RequestDeviceState(verifyCtx, state); recheck != nil {
							recheckDiffs := diffSnapshots(DeviceStateSnapshot(before), recheck)
							if len(recheckDiffs) > 0 {
								c.debugf("teardown: baseline STILL diverged after re-reset (%d fields)", len(recheckDiffs))
							}
						}
						verifyCancel()
					}
				}
			}
		}
		afterCancel()
	}

	// Unsubscribe all active subscriptions.
	c.pool.UnsubscribeAll(c.pool.Main())
	c.pool.ClearNotifications()
	if c.pool.Main() != nil {
		c.pool.Main().pendingNotifications = nil
	}
	for _, key := range c.pool.ZoneKeys() {
		if zc := c.pool.Zone(key); zc != nil {
			zc.pendingNotifications = nil
		}
	}

	// Close connections with incomplete PASE state.
	if c.pool.Main() != nil && c.pool.Main().isConnected() && !c.ops.PASEState().Completed() {
		c.debugf("teardown: closing connection with incomplete PASE state")
		c.pool.Main().transitionTo(ConnDisconnected)
		c.ops.SetPASEState(nil)
	}

	// Clear stale PASE state from failed handshakes.
	if c.ops.PASEState() != nil && !c.ops.PASEState().Completed() {
		c.debugf("teardown: clearing incomplete PASE state")
		c.ops.SetPASEState(nil)
	}

	// Reset hadConnection for the next test.
	if c.pool.Main() != nil {
		c.pool.Main().hadConnection = false
	}

	// Clean up security pool connections.
	if secState, ok := state.Custom["security"].(*securityState); ok && secState.pool != nil {
		secState.pool.mu.Lock()
		for _, conn := range secState.pool.connections {
			if conn.isConnected() {
				_ = conn.Close()
			}
		}
		secState.pool.connections = nil
		secState.pool.mu.Unlock()
	}
}

// CurrentLevel returns the runner's current precondition level based on
// connection and commissioning state.
func (c *coordinatorImpl) CurrentLevel() int {
	if c.ops.PASEState().Completed() {
		return precondLevelCommissioned
	}
	if c.pool.Main() != nil && c.pool.Main().isConnected() {
		return precondLevelConnected
	}
	return precondLevelNone
}

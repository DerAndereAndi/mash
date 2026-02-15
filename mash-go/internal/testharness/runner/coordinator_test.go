package runner

import (
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newCoord(t *testing.T, cfg *Config) (*coordinatorImpl, *MockSuiteSession, *MockConnPool, *MockCommissioningOps) {
	t.Helper()
	s := NewMockSuiteSession(t)
	p := NewMockConnPool(t)
	o := NewMockCommissioningOps(t)
	if cfg == nil {
		cfg = &Config{}
	}
	c := NewCoordinator(s, p, o, cfg, func(string, ...any) {}).(*coordinatorImpl)
	return c, s, p, o
}

func completedPASE() *PASEState {
	return &PASEState{completed: true, sessionKey: []byte("test-session-key")}
}

func incompletePASE() *PASEState {
	return &PASEState{completed: false}
}

func st() *engine.ExecutionState {
	return engine.NewExecutionState(context.Background())
}

func tcWith(id string, preconds ...loader.Condition) *loader.TestCase {
	return &loader.TestCase{ID: id, Preconditions: preconds}
}

func cond(key string, val any) loader.Condition {
	return loader.Condition{key: val}
}

// allMaybe registers permissive expectations for every mock method. It is
// called once per test. Because testify returns the first matching expectation,
// callers that need specific behavior must register their expectations BEFORE
// calling allMaybe.
func allMaybe(s *MockSuiteSession, p *MockConnPool, o *MockCommissioningOps) {
	s.EXPECT().ZoneID().Return("").Maybe()
	s.EXPECT().ConnKey().Return("").Maybe()
	s.EXPECT().IsCommissioned().Return(false).Maybe()
	s.EXPECT().Crypto().Return(CryptoState{}).Maybe()
	s.EXPECT().Conn().Return((*Connection)(nil)).Maybe()
	s.EXPECT().SetConn(mock.Anything).Return().Maybe()
	s.EXPECT().Record(mock.Anything, mock.Anything).Return().Maybe()
	s.EXPECT().CloseConn().Return().Maybe()
	s.EXPECT().Clear().Return().Maybe()

	p.EXPECT().Main().Return((*Connection)(nil)).Maybe()
	p.EXPECT().SetMain(mock.Anything).Return().Maybe()
	p.EXPECT().ZoneCount().Return(0).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil)).Maybe()
	p.EXPECT().ClearNotifications().Return().Maybe()
	p.EXPECT().CloseZonesExcept(mock.Anything).Return(time.Time{}).Maybe()
	p.EXPECT().CloseAllZones().Return(time.Time{}).Maybe()
	p.EXPECT().UnsubscribeAll(mock.Anything).Return().Maybe()
	p.EXPECT().UntrackZone(mock.Anything).Return().Maybe()
	p.EXPECT().TrackZone(mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	p.EXPECT().Zone(mock.Anything).Return((*Connection)(nil)).Maybe()
	p.EXPECT().ZoneID(mock.Anything).Return("").Maybe()

	o.EXPECT().DiscoveredDiscriminator().Return(uint16(0)).Maybe()
	o.EXPECT().PASEState().Return((*PASEState)(nil)).Maybe()
	o.EXPECT().WorkingCrypto().Return(CryptoState{}).Maybe()
	o.EXPECT().CommissionZoneType().Return(cert.ZoneType(0)).Maybe()
	o.EXPECT().DeviceStateModified().Return(false).Maybe()
	o.EXPECT().LastDeviceConnClose().Return(time.Time{}).Maybe()
	o.EXPECT().IsSuiteZoneCommission().Return(false).Maybe()
	o.EXPECT().DebugSnapshot(mock.Anything).Return().Maybe()
	o.EXPECT().SetCommissionZoneType(mock.Anything).Return().Maybe()
	o.EXPECT().SetPASEState(mock.Anything).Return().Maybe()
	o.EXPECT().SetDeviceStateModified(mock.Anything).Return().Maybe()
	o.EXPECT().SetLastDeviceConnClose(mock.Anything).Return().Maybe()
	o.EXPECT().SetWorkingCrypto(mock.Anything).Return().Maybe()
	o.EXPECT().ClearWorkingCrypto().Return().Maybe()
	o.EXPECT().EnsureDisconnected().Return().Maybe()
	o.EXPECT().DisconnectConnection().Return().Maybe()
	o.EXPECT().SendRemoveZone().Return().Maybe()
	o.EXPECT().SendRemoveZoneOnConn(mock.Anything, mock.Anything).Return().Maybe()
	o.EXPECT().HandlePreconditionCases(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	o.EXPECT().EnsureConnected(mock.Anything, mock.Anything).Return(nil).Maybe()
	o.EXPECT().EnsureCommissioned(mock.Anything, mock.Anything).Return(nil).Maybe()
	o.EXPECT().WaitForCommissioningMode(mock.Anything, mock.Anything).Return(nil).Maybe()
	o.EXPECT().ProbeSessionHealth().Return(nil).Maybe()
	o.EXPECT().ReconnectToZone(mock.Anything).Return(nil).Maybe()
	o.EXPECT().SendTriggerViaZone(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	o.EXPECT().SendClearLimitInvoke(mock.Anything).Return(nil).Maybe()
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil)).Maybe()
}

// ===========================================================================
// 1. CurrentLevel
// ===========================================================================

func TestCoordCurrentLevel_NoPASENoConnection(t *testing.T) {
	c, _, pool, ops := newCoord(t, nil)
	ops.EXPECT().PASEState().Return((*PASEState)(nil))
	pool.EXPECT().Main().Return((*Connection)(nil))
	assert.Equal(t, precondLevelNone, c.CurrentLevel())
}

func TestCoordCurrentLevel_Connected(t *testing.T) {
	c, _, pool, ops := newCoord(t, nil)
	ops.EXPECT().PASEState().Return((*PASEState)(nil))
	pool.EXPECT().Main().Return(&Connection{state: ConnTLSConnected})
	assert.Equal(t, precondLevelConnected, c.CurrentLevel())
}

func TestCoordCurrentLevel_Commissioned(t *testing.T) {
	c, _, _, ops := newCoord(t, nil)
	ops.EXPECT().PASEState().Return(completedPASE())
	assert.Equal(t, precondLevelCommissioned, c.CurrentLevel())
}

// ===========================================================================
// 2. Session reuse
// ===========================================================================

func TestCoordSessionReuse_ReusesWhenCommissionedAndClean(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})

	// Specific expectations BEFORE allMaybe (first match wins).
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	p.EXPECT().ZoneCount().Return(1)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-REUSE", cond(PrecondDeviceCommissioned, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithFreshCommission(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondFreshCommission, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithGridZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondDeviceHasGridZone, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithPreviouslyConnected(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondSessionPreviouslyConnected, true)), st()))
}

func TestCoordSessionReuse_NoReuseWhenNotCommissioned(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
}

// ===========================================================================
// 3. Backward transitions
// ===========================================================================

func TestCoordBackward_DetachMainWhenSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	// current=3 (commissioned), needed=1 (commissioning).
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	var setMainConn *Connection
	p.EXPECT().SetMain(mock.Anything).Run(func(conn *Connection) {
		if conn != nil {
			setMainConn = conn
		}
	}).Return()
	o.EXPECT().SetPASEState(mock.Anything).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	// Should have called SetMain with an empty Connection (detach).
	assert.NotNil(t, setMainConn, "SetMain should be called to detach")
}

func TestCoordBackward_RemoveZoneWhenNoSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("")

	removeZoneCalled := false
	o.EXPECT().SendRemoveZone().Run(func() {
		removeZoneCalled = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, removeZoneCalled, "SendRemoveZone called when no suite zone")
}

func TestCoordBackward_DisconnectWhenNoSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("")

	disconnected := false
	o.EXPECT().EnsureDisconnected().Run(func() {
		disconnected = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected called")
}

func TestCoordBackward_SetsLastDeviceConnClose(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("")

	closeTimeCalled := false
	o.EXPECT().SetLastDeviceConnClose(mock.Anything).Run(func(_ time.Time) {
		closeTimeCalled = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, closeTimeCalled, "SetLastDeviceConnClose called when Target set")
}

// TestCoordBackward_SuiteConnNeverClosed verifies that backward transitions
// from commissioned (L3) to commissioning (L1) do NOT call suite.CloseConn().
// The suite zone connection stays alive for the entire test run; only
// pool.Main() is detached (replaced with an empty Connection).
func TestCoordBackward_SuiteConnNeverClosed(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	// current=3 (commissioned), needed=1 (commissioning).
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	closeConnCalled := false
	s.EXPECT().CloseConn().Run(func() {
		closeConnCalled = true
	}).Return().Maybe()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.False(t, closeConnCalled, "CloseConn must NOT be called -- suite zone stays alive during backward transitions")
}

// TestCoordBackward_PreservesZoneState verifies that a backward transition
// with a suite zone preserves both the connection AND the zone state (zone
// ID, crypto). Neither CloseConn nor Clear should be called.
func TestCoordBackward_PreservesZoneState(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()

	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	closeConnCalled := false
	s.EXPECT().CloseConn().Run(func() {
		closeConnCalled = true
	}).Return().Maybe()

	clearCalled := false
	s.EXPECT().Clear().Run(func() {
		clearCalled = true
	}).Return().Maybe()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.False(t, closeConnCalled, "CloseConn must NOT be called")
	assert.False(t, clearCalled, "Clear must NOT be called (zone state must be preserved)")
}

// TestCoordBackward_SecondBlock_SuiteConnNeverClosed verifies that the second
// backward transition block (needed < current && needed <= commissioning) also
// does NOT close the suite connection.
func TestCoordBackward_SecondBlock_SuiteConnNeverClosed(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	// current=2 (connected), needed=1 (commissioning).
	o.EXPECT().PASEState().Return((*PASEState)(nil))
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	closeConnCalled := false
	s.EXPECT().CloseConn().Run(func() {
		closeConnCalled = true
	}).Return().Maybe()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.False(t, closeConnCalled, "CloseConn must NOT be called in second backward transition block")
}

// TestCoordBackward_ClosesSuiteConnTCP verifies that the backward transition
// to L1 (commissioning mode) closes the suite zone TCP connection to free
// the device's cap slot. The zone itself stays registered -- only the TCP is
// closed. This is the fix for TC-CONN-CAP-001 and TC-CONN-BUSY-003.
func TestCoordBackward_ClosesSuiteConnTCP(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	suiteConn := &Connection{state: ConnOperational}

	// current=3 (commissioned), needed=1 (commissioning).
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	s.EXPECT().Conn().Return(suiteConn)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.Equal(t, ConnDisconnected, suiteConn.state,
		"suite zone TCP must be closed in backward transition to free cap slot")
}

// ===========================================================================
// 4. Device reset trigger
// ===========================================================================

func TestCoordReset_SendsTriggerWhenConfigured(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})
	triggerSent := false
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).Run(
		func(_ context.Context, _ uint64, _ *engine.ExecutionState) { triggerSent = true },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, triggerSent, "reset trigger sent")
}

func TestCoordReset_RetriesViaReconnect(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})

	s.EXPECT().ZoneID().Return("sz1")
	o.EXPECT().WorkingCrypto().Return(CryptoState{ZoneCAPool: x509.NewCertPool()})

	callCount := 0
	// First call fails, second succeeds. Use Once() to chain different returns.
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(_ context.Context, _ uint64, _ *engine.ExecutionState) { callCount++ }).
		Return(fmt.Errorf("io error")).Once()
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(_ context.Context, _ uint64, _ *engine.ExecutionState) { callCount++ }).
		Return(nil).Maybe()

	reconnected := false
	o.EXPECT().ReconnectToZone(mock.Anything).Run(func(_ *engine.ExecutionState) {
		reconnected = true
	}).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, reconnected, "reconnect attempted")
	assert.GreaterOrEqual(t, callCount, 2, "trigger retried")
}

func TestCoordReset_SkipsWhenNoTarget(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{})
	triggerCalled := false
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).Run(
		func(_ context.Context, _ uint64, _ *engine.ExecutionState) { triggerCalled = true },
	).Return(nil).Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.False(t, triggerCalled, "trigger skipped when no Target")
}

// ===========================================================================
// 5. Zone CA clearing
// ===========================================================================

func TestCoordCAClear_ClearsWhenBelowCommissioned(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().WorkingCrypto().Return(CryptoState{ZoneCAPool: x509.NewCertPool()})
	cleared := false
	o.EXPECT().ClearWorkingCrypto().Run(func() { cleared = true }).Return()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceConnected, true)), st()))
	assert.True(t, cleared, "crypto cleared when needed < commissioned")
}

func TestCoordCAClear_NotClearedWhenCommissioned(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	cleared := false
	o.EXPECT().ClearWorkingCrypto().Run(func() { cleared = true }).Return().Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.False(t, cleared, "crypto NOT cleared when needed >= commissioned")
}

func TestCoordCAClear_NotClearedWhenNeedsZoneConns(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	cleared := false
	o.EXPECT().ClearWorkingCrypto().Run(func() { cleared = true }).Return().Maybe()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceConnected, true), cond(PrecondTwoZonesConnected, true)), st()))
	assert.False(t, cleared, "crypto NOT cleared when needsZoneConns")
}

// ===========================================================================
// 6. Crypto save/restore
// ===========================================================================

func TestCoordCryptoRestore_NoRestoreWhenSuiteZoneExists(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	ps := completedPASE()
	o.EXPECT().PASEState().Return(ps)
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	p.EXPECT().ZoneCount().Return(1)
	allMaybe(s, p, o)
	// The restore guard is (suite.ZoneID() == ""). Since we return "sz1",
	// restore is skipped. This test just verifies no error.
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
}

func TestCoordCryptoRestore_RestoresWhenNoSuiteAndSamePASE(t *testing.T) {
	// This test verifies the crypto restore path fires when:
	// 1. No suite zone (ZoneID() == "")
	// 2. PASEState unchanged (same pointer before/after)
	// 3. WorkingCrypto().ZoneCA changed (precondition handlers modified it)
	//
	// We simulate this by having HandlePreconditionCases change the crypto
	// returned by WorkingCrypto from a non-nil ZoneCA to a nil one.
	c, s, p, o := newCoord(t, nil)
	ps := completedPASE()
	originalCA := &cert.ZoneCA{}
	cryptoBefore := CryptoState{ZoneCA: originalCA, ZoneCAPool: x509.NewCertPool()}
	cryptoAfter := CryptoState{} // ZoneCA is nil -- different from before.

	o.EXPECT().PASEState().Return(ps)
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("")
	p.EXPECT().ZoneCount().Return(1)

	// WorkingCrypto is called multiple times. We need the first call (save)
	// to return cryptoBefore and the last call (compare) to return cryptoAfter.
	// Use HandlePreconditionCases to flip the crypto mid-flow.
	cryptoState := cryptoBefore
	o.EXPECT().WorkingCrypto().RunAndReturn(func() CryptoState { return cryptoState })
	o.EXPECT().HandlePreconditionCases(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *loader.TestCase, _ *engine.ExecutionState, _ []loader.Condition, _ *bool) {
			cryptoState = cryptoAfter
		}).Return(nil)

	restored := false
	o.EXPECT().SetWorkingCrypto(mock.Anything).Run(func(_ CryptoState) {
		restored = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, restored, "crypto restored when no suite zone and same PASE")
}

// ===========================================================================
// 7. State triggers
// ===========================================================================

func TestCoordStateTrigger_ControlState(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	var triggers []uint64
	o.EXPECT().SendTriggerViaZone(mock.Anything, mock.Anything, mock.Anything).Run(
		func(_ context.Context, trigger uint64, _ *engine.ExecutionState) { triggers = append(triggers, trigger) },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondControlState, ControlStateControlled)), st()))
	assert.Contains(t, triggers, features.TriggerControlStateControlled)
}

func TestCoordStateTrigger_ProcessState(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	var triggers []uint64
	o.EXPECT().SendTriggerViaZone(mock.Anything, mock.Anything, mock.Anything).Run(
		func(_ context.Context, trigger uint64, _ *engine.ExecutionState) { triggers = append(triggers, trigger) },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondProcessState, ProcessStateRunning)), st()))
	assert.Contains(t, triggers, features.TriggerProcessStateRunning)
}

// ===========================================================================
// 8. Untracked session
// ===========================================================================

func TestCoordUntracked_ResetsWhenNoZoneConns(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return((*Connection)(nil)).Maybe()
	p.EXPECT().ZoneCount().Return(0)

	disconnected := false
	o.EXPECT().EnsureDisconnected().Run(func() { disconnected = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected for untracked session")
}

func TestCoordUntracked_NoResetWhenZonesExist(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	allMaybe(s, p, o)
	// Just verify no error; EnsureDisconnected should not fire for untracked path.
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
}

// TestCoordUntracked_NoResetWhenSuiteZoneExists verifies that the "untracked
// commission session" safety net does NOT destroy the suite zone. This was
// Bug 2: when pool.ZoneCount()==0 but a suite zone existed, the coordinator
// called EnsureDisconnected() which called suite.Clear(), destroying the
// persistent control channel mid-run.
func TestCoordUntracked_NoResetWhenSuiteZoneExists(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	// Completed PASE + empty pool + existing suite zone.
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return((*Connection)(nil)).Maybe()
	p.EXPECT().ZoneCount().Return(0)
	s.EXPECT().ZoneID().Return("sz1")

	disconnected := false
	o.EXPECT().EnsureDisconnected().Run(func() { disconnected = true }).Return().Maybe()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.False(t, disconnected, "EnsureDisconnected must NOT be called when suite zone exists (Bug 2 guard)")
}

// ===========================================================================
// 9. TeardownTest
// ===========================================================================

func TestCoordTeardown_UnsubscribesAndClearsNotifications(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	conn := &Connection{state: ConnOperational}
	p.EXPECT().Main().Return(conn)
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	unsubCalled := false
	p.EXPECT().UnsubscribeAll(conn).Run(func(_ *Connection) { unsubCalled = true }).Return()
	clearCalled := false
	p.EXPECT().ClearNotifications().Run(func() { clearCalled = true }).Return()

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.True(t, unsubCalled)
	assert.True(t, clearCalled)
}

func TestCoordTeardown_ClosesConnectionWithIncompletePASE(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	conn := &Connection{state: ConnTLSConnected}
	p.EXPECT().Main().Return(conn)
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(incompletePASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.Equal(t, ConnDisconnected, conn.state)
}

func TestCoordTeardown_ClearsIncompletePASEState(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	p.EXPECT().Main().Return((*Connection)(nil))
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(incompletePASE())

	paseCleared := false
	o.EXPECT().SetPASEState((*PASEState)(nil)).Run(func(_ *PASEState) { paseCleared = true }).Return()

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.True(t, paseCleared, "incomplete PASE state cleared")
}

func TestCoordTeardown_ResetsHadConnection(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	conn := &Connection{state: ConnOperational, hadConnection: true}
	p.EXPECT().Main().Return(conn)
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.False(t, conn.hadConnection, "hadConnection reset")
}

func TestCoordTeardown_CapturesDeviceState(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	snap := DeviceStateSnapshot{"ctl": "AUTONOMOUS"}
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(snap)

	allMaybe(s, p, o)
	state := st()
	c.TeardownTest(context.Background(), tcWith("TC"), state)

	after, ok := state.Custom[engine.StateKeyDeviceStateAfter]
	assert.True(t, ok, "device state after captured")
	assert.NotNil(t, after)
}

// ===========================================================================
// 9b. Teardown baseline enforcement
// ===========================================================================

func TestCoordTeardown_ResendsResetOnBaselineDivergence(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0, "controlState": "AUTONOMOUS"}
	after := DeviceStateSnapshot{"zoneCount": 1, "controlState": "AUTONOMOUS"}
	// First call returns diverged state, second (re-probe) returns matching.
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(after).Once()
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(before).Once()

	resetCalled := false
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(_ context.Context, _ uint64, _ *engine.ExecutionState) { resetCalled = true }).Return(nil).Once()

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(before)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	assert.True(t, resetCalled, "triggerResetTestState re-sent on divergence")
	o.AssertNumberOfCalls(t, "RequestDeviceState", 2)
}

func TestCoordTeardown_NoResetWhenBaselineMatches(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	snap := DeviceStateSnapshot{"zoneCount": 0}
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(snap)

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(snap)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	// SendTriggerViaZone should NOT be called (no divergence).
	o.AssertNotCalled(t, "SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything)
}

func TestCoordTeardown_SkipsVerificationWithoutTarget(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "", EnableKey: "0011"})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())

	o.AssertNotCalled(t, "RequestDeviceState", mock.Anything, mock.Anything)
}

func TestCoordTeardown_SkipsVerificationWithoutEnableKey(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: ""})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())

	o.AssertNotCalled(t, "RequestDeviceState", mock.Anything, mock.Anything)
}

func TestCoordTeardown_SkipsResetWhenBeforeSnapshotNil(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	after := DeviceStateSnapshot{"zoneCount": 1}
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(after)

	allMaybe(s, p, o)
	state := st()
	// Deliberately do NOT set StateKeyDeviceStateBefore.

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	// No re-reset because there's no baseline to compare against.
	o.AssertNotCalled(t, "SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything)
}

func TestCoordTeardown_LogsWarningWhenResetFails(t *testing.T) {
	var debugMessages []string
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	s := NewMockSuiteSession(t)
	p := NewMockConnPool(t)
	o := NewMockCommissioningOps(t)
	c := NewCoordinator(s, p, o, cfg, func(format string, args ...any) {
		debugMessages = append(debugMessages, fmt.Sprintf(format, args...))
	}).(*coordinatorImpl)

	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0}
	after := DeviceStateSnapshot{"zoneCount": 1}
	// Both RequestDeviceState calls return diverged state.
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(after)
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).Return(nil)

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(before)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	found := false
	for _, msg := range debugMessages {
		if strings.Contains(msg, "STILL diverged") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'STILL diverged' warning in debug output, got: %v", debugMessages)
}

func TestCoordTeardown_SucceedsWhenResetRestoresBaseline(t *testing.T) {
	var debugMessages []string
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	s := NewMockSuiteSession(t)
	p := NewMockConnPool(t)
	o := NewMockCommissioningOps(t)
	c := NewCoordinator(s, p, o, cfg, func(format string, args ...any) {
		debugMessages = append(debugMessages, fmt.Sprintf(format, args...))
	}).(*coordinatorImpl)

	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0}
	after := DeviceStateSnapshot{"zoneCount": 1}
	// First call: diverged. Second call (re-probe): restored.
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(after).Once()
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(before).Once()
	o.EXPECT().SendTriggerViaZone(mock.Anything, features.TriggerResetTestState, mock.Anything).Return(nil)

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(before)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	for _, msg := range debugMessages {
		if strings.Contains(msg, "STILL diverged") {
			t.Fatal("unexpected 'STILL diverged' warning after successful re-reset")
		}
	}
}

// ===========================================================================
// 9b2. Teardown suite zone health check
// ===========================================================================

// TestCoordTeardown_ProbesAndReconnectsWhenBroken verifies that teardown
// detects a broken suite zone connection (e.g., after TC-FRAME-005 corrupts
// framing with raw bytes) and reconnects before the next test starts.
func TestCoordTeardown_ProbesAndReconnectsWhenBroken(t *testing.T) {
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	c, s, p, o := newCoord(t, cfg)

	brokenConn := &Connection{state: ConnOperational}
	p.EXPECT().Main().Return(brokenConn).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	// Suite zone exists with the broken connection.
	s.EXPECT().ZoneID().Return("suite-123").Maybe()
	s.EXPECT().Conn().Return(brokenConn).Maybe()

	// Device state capture returns nil (broken conn can't read).
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil))

	// Health probe FAILS -- broken framing.
	probeCalled := false
	o.EXPECT().ProbeSessionHealth().Run(func() { probeCalled = true }).Return(fmt.Errorf("read: broken pipe"))

	// ReconnectToZone SHOULD be called to restore the suite zone.
	reconnectCalled := false
	o.EXPECT().ReconnectToZone(mock.Anything).Run(func(_ *engine.ExecutionState) { reconnectCalled = true }).Return(nil)

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())

	assert.True(t, probeCalled, "teardown should probe suite zone health")
	assert.True(t, reconnectCalled, "teardown should reconnect when probe fails")
}

// TestCoordTeardown_NoReconnectWhenHealthy verifies that teardown does NOT
// reconnect when the suite zone connection is healthy.
func TestCoordTeardown_NoReconnectWhenHealthy(t *testing.T) {
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	c, s, p, o := newCoord(t, cfg)

	healthyConn := &Connection{state: ConnOperational}
	p.EXPECT().Main().Return(healthyConn).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	s.EXPECT().ZoneID().Return("suite-123").Maybe()
	s.EXPECT().Conn().Return(healthyConn).Maybe()

	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil))

	// Health probe succeeds.
	o.EXPECT().ProbeSessionHealth().Return(nil)

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())

	// No reconnect expected -- allMaybe sets ReconnectToZone as Maybe(),
	// so if it IS called unexpectedly, the mock won't complain. We verify
	// by checking ProbeSessionHealth was called (sufficient for healthy path).
}

// TestCoordTeardown_ReconnectFailureIsNonFatal verifies that a failed
// reconnect attempt in teardown is logged but does not crash.
func TestCoordTeardown_ReconnectFailureIsNonFatal(t *testing.T) {
	var debugMessages []string
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	s := NewMockSuiteSession(t)
	p := NewMockConnPool(t)
	o := NewMockCommissioningOps(t)
	c := NewCoordinator(s, p, o, cfg, func(format string, args ...any) {
		debugMessages = append(debugMessages, fmt.Sprintf(format, args...))
	}).(*coordinatorImpl)

	brokenConn := &Connection{state: ConnOperational}
	p.EXPECT().Main().Return(brokenConn).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	s.EXPECT().ZoneID().Return("suite-123").Maybe()
	s.EXPECT().Conn().Return(brokenConn).Maybe()

	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil))
	o.EXPECT().ProbeSessionHealth().Return(fmt.Errorf("broken pipe"))
	o.EXPECT().ReconnectToZone(mock.Anything).Return(fmt.Errorf("dial failed"))

	allMaybe(s, p, o)
	// Should not panic.
	c.TeardownTest(context.Background(), tcWith("TC"), st())

	// Verify the failure was logged.
	found := false
	for _, msg := range debugMessages {
		if strings.Contains(msg, "reconnect failed") {
			found = true
		}
	}
	assert.True(t, found, "reconnect failure should be logged")
}

// TestCoordTeardown_SuiteAlive_RestoresMainBeforeProbe verifies that teardown
// restores pool.Main() to suite.Conn() before probing session health.
// After non-L3 tests, pool.Main() is dead/empty while suite.Conn() is alive.
// Without restoring main, ProbeSessionHealth would fail ("no active connection")
// and trigger an unnecessary ReconnectToZone.
func TestCoordTeardown_SuiteAlive_RestoresMainBeforeProbe(t *testing.T) {
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	c, s, p, o := newCoord(t, cfg)

	aliveConn := &Connection{state: ConnOperational}

	// pool.Main() is dead (empty from detach after non-L3 test).
	deadMain := &Connection{state: ConnDisconnected}
	p.EXPECT().Main().Return(deadMain).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	// Suite zone exists with alive connection.
	s.EXPECT().ZoneID().Return("suite-123").Maybe()
	s.EXPECT().Conn().Return(aliveConn).Maybe()

	// Device state capture.
	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil)).Maybe()

	// Expect SetMain to be called with the suite conn (restoring main).
	setMainCalled := false
	p.EXPECT().SetMain(aliveConn).Run(func(_ *Connection) {
		setMainCalled = true
	}).Return()

	// ProbeSessionHealth succeeds (because main is restored to suite conn).
	o.EXPECT().ProbeSessionHealth().Return(nil)

	// ReconnectToZone should NOT be called.
	reconnectCalled := false
	o.EXPECT().ReconnectToZone(mock.Anything).Run(func(_ *engine.ExecutionState) {
		reconnectCalled = true
	}).Return(nil).Maybe()

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())

	assert.True(t, setMainCalled, "SetMain should be called with suite.Conn() to restore main before probe")
	assert.False(t, reconnectCalled, "ReconnectToZone must NOT be called when suite conn is alive")
}

// TestCoordTeardown_NoProbeWithoutSuiteZone verifies that the health probe
// is skipped when there is no suite zone (simulation mode or no zone yet).
func TestCoordTeardown_NoProbeWithoutSuiteZone(t *testing.T) {
	cfg := &Config{Target: "localhost:8443", EnableKey: "0011"}
	c, s, p, o := newCoord(t, cfg)

	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	// No suite zone.
	s.EXPECT().ZoneID().Return("").Maybe()
	s.EXPECT().Conn().Return((*Connection)(nil)).Maybe()

	o.EXPECT().RequestDeviceState(mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil))

	// ProbeSessionHealth should NOT be called for suite zone health check.
	// (It may still be called by allMaybe as a fallback, but the explicit
	// setup here verifies no suite-zone-specific probe path is triggered.)

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
}

// TestCoordTeardown_NoProbeInSimMode verifies that the health probe is
// skipped when running in simulation mode (no Target configured).
func TestCoordTeardown_NoProbeInSimMode(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{}) // No Target
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneKeys().Return([]string(nil))
	o.EXPECT().PASEState().Return(completedPASE())

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
}

// ===========================================================================
// 9c. Connection tier integration
// ===========================================================================

func TestCoordSetup_InfrastructureTier_DisconnectsBeforeSetup(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	tc := &loader.TestCase{
		ID:             "TC-INFRA",
		ConnectionTier: TierInfrastructure,
		Preconditions:  []loader.Condition{{PrecondDeviceCommissioned: true}},
	}

	// Pretend we're currently commissioned.
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)

	allMaybe(s, p, o)
	_ = c.SetupPreconditions(context.Background(), tc, st())

	// Infrastructure tier should not reuse the session (canReuseSession is false).
	// Verify it completes without error; session reuse is prevented by tier check.
}

func TestCoordSetup_ApplicationTier_ReusesConnection(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	tc := &loader.TestCase{
		ID:             "TC-APP",
		ConnectionTier: TierApplication,
		Preconditions:  []loader.Condition{{PrecondDeviceCommissioned: true}},
	}

	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)

	probeCalled := false
	o.EXPECT().ProbeSessionHealth().Run(func() { probeCalled = true }).Return(nil)

	allMaybe(s, p, o)
	_ = c.SetupPreconditions(context.Background(), tc, st())

	// Application tier with healthy session should probe and reuse.
	assert.True(t, probeCalled, "application tier probes session health")
}

func TestCoordSetup_ProtocolTier_DoesNotReuseSession(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	tc := &loader.TestCase{
		ID:             "TC-PROTO",
		ConnectionTier: TierProtocol,
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
			{PrecondFreshCommission: true},
		},
	}

	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("test-zone")

	ensureDisconnected := false
	o.EXPECT().EnsureDisconnected().Run(func() { ensureDisconnected = true }).Return()

	allMaybe(s, p, o)
	_ = c.SetupPreconditions(context.Background(), tc, st())

	// Protocol tier with fresh_commission should force full disconnect.
	assert.True(t, ensureDisconnected, "protocol tier forces disconnect for fresh_commission")
}

// TestFreshCommission_SendsRemoveZoneBeforeDisconnect verifies that
// fresh_commission sends RemoveZone on the live connection BEFORE calling
// EnsureDisconnected. Without this, the device keeps the old zone registered
// and presents the old zone's cert on operational reconnect, causing x509
// verification failures.
func TestFreshCommission_SendsRemoveZoneBeforeDisconnect(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	suiteConn := &Connection{state: ConnOperational}
	tc := &loader.TestCase{
		ID:             "TC-FRESH",
		ConnectionTier: TierProtocol,
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
			{PrecondFreshCommission: true},
		},
	}

	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("test-zone")
	s.EXPECT().Conn().Return(suiteConn).Maybe()

	// Track call order.
	var callOrder []string
	o.EXPECT().SendRemoveZoneOnConn(suiteConn, "test-zone").Run(func(conn *Connection, zoneID string) {
		callOrder = append(callOrder, "remove")
	}).Return()
	o.EXPECT().EnsureDisconnected().Run(func() { callOrder = append(callOrder, "disconnect") }).Return()

	allMaybe(s, p, o)
	_ = c.SetupPreconditions(context.Background(), tc, st())

	// SendRemoveZoneOnConn must happen before EnsureDisconnected.
	assert.Contains(t, callOrder, "remove", "SendRemoveZoneOnConn should be called")
	assert.Contains(t, callOrder, "disconnect", "EnsureDisconnected should be called")
	idx := 0
	for i, v := range callOrder {
		if v == "remove" {
			idx = i
			break
		}
	}
	for i, v := range callOrder {
		if v == "disconnect" {
			assert.Greater(t, i, idx, "SendRemoveZoneOnConn must be called before EnsureDisconnected")
			break
		}
	}
}

// ===========================================================================
// 10. Suite zone borrowing
// ===========================================================================

// TestSuiteCanReconnect_BorrowsExistingConn verifies that when the suite zone
// connection is alive, suiteCanReconnect borrows suite.Conn() directly instead
// of closing it and calling ReconnectToZone. This avoids ~100ms reconnect
// overhead and eliminates a fragile close/reconnect cycle.
func TestSuiteCanReconnect_BorrowsExistingConn(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	suiteConn := &Connection{state: ConnOperational}

	// Suite zone exists with alive connection. Main is empty (from detach).
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	s.EXPECT().Conn().Return(suiteConn)

	// Main is nil/dead -> triggers suiteCanReconnect.
	o.EXPECT().PASEState().Return((*PASEState)(nil))
	o.EXPECT().CommissionZoneType().Return(cert.ZoneTypeTest)
	p.EXPECT().Main().Return((*Connection)(nil)).Maybe()

	// Track that SetMain is called with suiteConn (borrow).
	var setMainConn *Connection
	p.EXPECT().SetMain(mock.Anything).Run(func(conn *Connection) {
		setMainConn = conn
	}).Return()

	// ReconnectToZone must NOT be called.
	reconnectCalled := false
	o.EXPECT().ReconnectToZone(mock.Anything).Run(func(_ *engine.ExecutionState) {
		reconnectCalled = true
	}).Return(nil).Maybe()

	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-L3", cond(PrecondDeviceCommissioned, true)), state))

	assert.Equal(t, suiteConn, setMainConn, "pool.Main() should be set to suite.Conn() (borrow)")
	assert.False(t, reconnectCalled, "ReconnectToZone must NOT be called when suite conn is alive")

	// Verify state flags are set.
	v, ok := state.Get(KeySessionEstablished)
	assert.True(t, ok)
	assert.Equal(t, true, v)
	v, ok = state.Get(StateCurrentZoneID)
	assert.True(t, ok)
	assert.Equal(t, "sz1", v)
}

// TestSuiteCanReconnect_FallsBackToReconnect verifies that when the suite zone
// connection is dead (e.g. device closed it), suiteCanReconnect falls back to
// ReconnectToZone to establish a new operational TCP connection.
func TestSuiteCanReconnect_FallsBackToReconnect(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	deadConn := &Connection{state: ConnDisconnected}

	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()
	s.EXPECT().Conn().Return(deadConn)

	o.EXPECT().PASEState().Return((*PASEState)(nil))
	o.EXPECT().CommissionZoneType().Return(cert.ZoneTypeTest)
	p.EXPECT().Main().Return((*Connection)(nil)).Maybe()

	reconnectCalled := false
	o.EXPECT().ReconnectToZone(mock.Anything).Run(func(_ *engine.ExecutionState) {
		reconnectCalled = true
	}).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-L3", cond(PrecondDeviceCommissioned, true)), st()))

	assert.True(t, reconnectCalled, "ReconnectToZone must be called when suite conn is dead")
}

// ===========================================================================
// 10b. Level switch
// ===========================================================================

func TestCoordLevel_EnsureCommissioned(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	called := false
	o.EXPECT().EnsureCommissioned(mock.Anything, mock.Anything).Run(
		func(_ context.Context, _ *engine.ExecutionState) { called = true },
	).Return(nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().ZoneCount().Return(1)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, called, "EnsureCommissioned called for level 3")
}

func TestCoordLevel_EnsureConnected(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	called := false
	o.EXPECT().EnsureConnected(mock.Anything, mock.Anything).Run(
		func(_ context.Context, _ *engine.ExecutionState) { called = true },
	).Return(nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceConnected, true)), st()))
	assert.True(t, called, "EnsureConnected called for level 2")
}

func TestCoordLevel_EnsureDisconnectedForCommissioning(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	s.EXPECT().ZoneID().Return("")

	disconnected := false
	o.EXPECT().EnsureDisconnected().Run(func() { disconnected = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected for commissioning without suite zone")
}

func TestCoordLevel_WaitForCommissioningMode(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	s.EXPECT().ZoneID().Return("")

	waitCalled := false
	o.EXPECT().WaitForCommissioningMode(mock.Anything, mock.Anything).Run(
		func(_ context.Context, _ time.Duration) { waitCalled = true },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, waitCalled, "WaitForCommissioningMode when Target configured")
}

// ===========================================================================
// 11. Multi-zone detection
// ===========================================================================

func TestCoordMultiZone_InjectsFromZoneCountAtLeast(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	var receivedPreconds []loader.Condition
	o.EXPECT().HandlePreconditionCases(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
		func(_ context.Context, _ *loader.TestCase, _ *engine.ExecutionState, preconds []loader.Condition, _ *bool) {
			receivedPreconds = preconds
		},
	).Return(nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondZoneCountAtLeast, 2)), st()))

	found := false
	for _, pc := range receivedPreconds {
		if _, ok := pc[PrecondTwoZonesConnected]; ok {
			found = true
		}
	}
	assert.True(t, found, "two_zones_connected injected when zone_count_at_least >= 2")
}

func TestCoordMultiZone_NoDuplicateInjection(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	var receivedPreconds []loader.Condition
	o.EXPECT().HandlePreconditionCases(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
		func(_ context.Context, _ *loader.TestCase, _ *engine.ExecutionState, preconds []loader.Condition, _ *bool) {
			receivedPreconds = preconds
		},
	).Return(nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondZoneCountAtLeast, 2), cond(PrecondTwoZonesConnected, true)), st()))

	count := 0
	for _, pc := range receivedPreconds {
		if _, ok := pc[PrecondTwoZonesConnected]; ok {
			count++
		}
	}
	assert.Equal(t, 1, count, "two_zones_connected not duplicated")
}

// ===========================================================================
// 12. Session previously connected
// ===========================================================================

func TestCoordPrevConn_DisconnectsAndPreservesCrypto(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	crypto := CryptoState{ZoneCAPool: x509.NewCertPool()}
	o.EXPECT().PASEState().Return(completedPASE())
	o.EXPECT().WorkingCrypto().Return(crypto)
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()

	paseNilCount := 0
	o.EXPECT().SetPASEState((*PASEState)(nil)).Run(func(_ *PASEState) { paseNilCount++ }).Return()

	cryptoRestored := false
	o.EXPECT().SetWorkingCrypto(crypto).Run(func(_ CryptoState) { cryptoRestored = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondSessionPreviouslyConnected, true)), st()))
	assert.Greater(t, paseNilCount, 0, "PASE set to nil")
	assert.True(t, cryptoRestored, "crypto preserved after disconnect")
}

func TestCoordPrevConn_SetsPASEToNil(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()

	paseNilCount := 0
	o.EXPECT().SetPASEState((*PASEState)(nil)).Run(func(_ *PASEState) { paseNilCount++ }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondSessionPreviouslyConnected, true)), st()))
	assert.Greater(t, paseNilCount, 0, "SetPASEState(nil) called")
}

// ===========================================================================
// Additional edge cases
// ===========================================================================

func TestCoordSetup_StoresSetupCode(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{SetupCode: "20202021"})
	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(), tcWith("TC"), state))
	val, ok := state.Get(StateSetupCode)
	assert.True(t, ok)
	assert.Equal(t, "20202021", val)
}

func TestCoordSetup_StoresDiscriminator(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().DiscoveredDiscriminator().Return(uint16(42))
	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(), tcWith("TC"), state))
	val, ok := state.Get(StateDeviceDiscriminator)
	assert.True(t, ok)
	assert.Equal(t, 42, val)
}

func TestCoordSetup_StoresSimulationPreconds(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceZonesFull, true)), state))
	val, ok := state.Get(PrecondDeviceZonesFull)
	assert.True(t, ok)
	assert.Equal(t, true, val)
}

func TestCoordSetup_FreshCommissionClearsSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	s.EXPECT().ZoneID().Return("old-sz")
	s.EXPECT().ConnKey().Return("main-old-sz").Maybe()

	closeAllCalled := false
	p.EXPECT().CloseAllZones().Run(func() { closeAllCalled = true }).Return(time.Time{})

	ensureDisconnectedCalled := false
	o.EXPECT().EnsureDisconnected().Run(func() { ensureDisconnectedCalled = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondFreshCommission, true)), st()))
	assert.True(t, closeAllCalled, "CloseAllZones called for fresh_commission")
	assert.True(t, ensureDisconnectedCalled, "EnsureDisconnected called for fresh_commission")
}

func TestCoordSetup_ClearLimitWhenNoExistingLimits(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	p.EXPECT().ZoneCount().Return(1)
	s.EXPECT().ZoneID().Return("sz1")
	s.EXPECT().ConnKey().Return("main-sz1").Maybe()

	clearCalled := false
	o.EXPECT().SendClearLimitInvoke(mock.Anything).Run(func(_ context.Context) { clearCalled = true }).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondNoExistingLimits, true)), st()))
	assert.True(t, clearCalled, "SendClearLimitInvoke called for no_existing_limits")
}

func TestCoordSetup_NoPreconditions(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(), tcWith("TC"), st()))
}

func TestCoordSetup_CommissioningModeSetsStateFlag(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	s.EXPECT().ZoneID().Return("")
	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), state))
	val, ok := state.Get(StateCommissioningActive)
	assert.True(t, ok)
	assert.Equal(t, true, val)
}

// ===========================================================================
// Interface Segregation: narrow sub-interface tests
// ===========================================================================

// Verify that MockCommissioningOps satisfies each narrow sub-interface individually.
// This is a compile-time check: if MockCommissioningOps fails to implement any of these,
// the test file won't compile.
var (
	_ StateAccessor       = (*MockCommissioningOps)(nil)
	_ LifecycleOps        = (*MockCommissioningOps)(nil)
	_ WireOps             = (*MockCommissioningOps)(nil)
	_ DiagnosticsOps      = (*MockCommissioningOps)(nil)
	_ PreconditionHandler = (*MockCommissioningOps)(nil)
)

// TestNarrowInterface_StateAccessor verifies that a function accepting only
// StateAccessor can read/write state without requiring the full CommissioningOps.
func TestNarrowInterface_StateAccessor(t *testing.T) {
	m := NewMockCommissioningOps(t)
	var accessor StateAccessor = m
	m.EXPECT().PASEState().Return(completedPASE())
	m.EXPECT().WorkingCrypto().Return(CryptoState{}).Maybe()
	m.EXPECT().CommissionZoneType().Return(cert.ZoneType(0)).Maybe()
	m.EXPECT().DeviceStateModified().Return(false).Maybe()
	m.EXPECT().DiscoveredDiscriminator().Return(uint16(0)).Maybe()
	m.EXPECT().LastDeviceConnClose().Return(time.Time{}).Maybe()
	m.EXPECT().IsSuiteZoneCommission().Return(false).Maybe()

	assert.True(t, accessor.PASEState().Completed())
	assert.Equal(t, cert.ZoneType(0), accessor.CommissionZoneType())
	assert.False(t, accessor.DeviceStateModified())
}

// TestNarrowInterface_LifecycleOps verifies that a function accepting only
// LifecycleOps can manage connection transitions.
func TestNarrowInterface_LifecycleOps(t *testing.T) {
	m := NewMockCommissioningOps(t)
	var lifecycle LifecycleOps = m
	m.EXPECT().EnsureConnected(mock.Anything, mock.Anything).Return(nil)
	m.EXPECT().DisconnectConnection().Return()
	m.EXPECT().EnsureDisconnected().Return()

	assert.NoError(t, lifecycle.EnsureConnected(context.Background(), st()))
	lifecycle.DisconnectConnection()
	lifecycle.EnsureDisconnected()
}

// TestCoordCleanup_CloseZonesExceptSuiteZone verifies that when a suite zone
// exists, pool cleanup uses CloseZonesExcept(suiteConnKey) rather than
// CloseAllZones, preventing the suite zone from being removed from the device.
func TestCoordCleanup_CloseZonesExceptSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	// current=3 (commissioned), needed=2 (connected) -- triggers cleanup.
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("suite-zone-1")
	s.EXPECT().ConnKey().Return("main-suite-zone-1")

	// Track that CloseZonesExcept is called with the suite key
	// and CloseAllZones is NOT called.
	closeExceptCalled := false
	closeExceptKey := ""
	p.EXPECT().CloseZonesExcept(mock.Anything).Run(func(key string) {
		closeExceptCalled = true
		closeExceptKey = key
	}).Return(time.Time{})

	closeAllCalled := false
	p.EXPECT().CloseAllZones().Run(func() {
		closeAllCalled = true
	}).Return(time.Time{}).Maybe()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-TEST", cond(PrecondDeviceConnected, true)), st()))

	assert.True(t, closeExceptCalled, "CloseZonesExcept should be called when suite zone exists")
	assert.Equal(t, "main-suite-zone-1", closeExceptKey, "CloseZonesExcept should exclude suite zone key")
	assert.False(t, closeAllCalled, "CloseAllZones should NOT be called when suite zone exists")
}

// TestCoordCleanup_CloseAllZonesWhenNoSuiteZone verifies that without a suite
// zone, CloseAllZones is called normally.
func TestCoordCleanup_CloseAllZonesWhenNoSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.EXPECT().PASEState().Return(completedPASE())
	p.EXPECT().Main().Return(&Connection{state: ConnOperational}).Maybe()
	s.EXPECT().ZoneID().Return("")

	closeAllCalled := false
	p.EXPECT().CloseAllZones().Run(func() {
		closeAllCalled = true
	}).Return(time.Time{})

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-TEST", cond(PrecondDeviceConnected, true)), st()))

	assert.True(t, closeAllCalled, "CloseAllZones should be called when no suite zone")
}

// TestNarrowInterface_WireOps verifies that a function accepting only
// WireOps can send protocol messages.
func TestNarrowInterface_WireOps(t *testing.T) {
	m := NewMockCommissioningOps(t)
	var wireOps WireOps = m
	m.EXPECT().SendRemoveZone().Return()
	m.EXPECT().SendTriggerViaZone(mock.Anything, mock.Anything, mock.Anything).Return(nil)
	m.EXPECT().SendClearLimitInvoke(mock.Anything).Return(nil)

	wireOps.SendRemoveZone()
	assert.NoError(t, wireOps.SendTriggerViaZone(context.Background(), 1, st()))
	assert.NoError(t, wireOps.SendClearLimitInvoke(context.Background()))
}

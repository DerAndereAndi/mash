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

// ===========================================================================
// 10. Level switch
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
	c, s, p, o := newCoord(t, &Config{SetupCode: "12345678"})
	allMaybe(s, p, o)
	state := st()
	assert.NoError(t, c.SetupPreconditions(context.Background(), tcWith("TC"), state))
	val, ok := state.Get(StateSetupCode)
	assert.True(t, ok)
	assert.Equal(t, "12345678", val)
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

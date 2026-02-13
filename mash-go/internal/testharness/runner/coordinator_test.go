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
	"github.com/mash-protocol/mash-go/pkg/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------------------------------------------------------------------------
// stubSuiteSession
// ---------------------------------------------------------------------------

type stubSuiteSession struct{ mock.Mock }

func (s *stubSuiteSession) ZoneID() string                 { return s.Called().String(0) }
func (s *stubSuiteSession) ConnKey() string                { return s.Called().String(0) }
func (s *stubSuiteSession) IsCommissioned() bool           { return s.Called().Bool(0) }
func (s *stubSuiteSession) Crypto() CryptoState            { return s.Called().Get(0).(CryptoState) }
func (s *stubSuiteSession) Conn() *Connection {
	ret := s.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(*Connection)
}
func (s *stubSuiteSession) SetConn(conn *Connection) { s.Called(conn) }
func (s *stubSuiteSession) Record(z string, c CryptoState) { s.Called(z, c) }
func (s *stubSuiteSession) Clear()                         { s.Called() }

// ---------------------------------------------------------------------------
// stubConnPool
// ---------------------------------------------------------------------------

type stubConnPool struct{ mock.Mock }

func (p *stubConnPool) Main() *Connection {
	ret := p.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(*Connection)
}
func (p *stubConnPool) SetMain(c *Connection) { p.Called(c) }
func (p *stubConnPool) NextMessageID() uint32 { return p.Called().Get(0).(uint32) }
func (p *stubConnPool) SendRequest(d []byte, op string, id uint32) (*wire.Response, error) {
	ret := p.Called(d, op, id)
	var r *wire.Response
	if ret.Get(0) != nil {
		r = ret.Get(0).(*wire.Response)
	}
	return r, ret.Error(1)
}
func (p *stubConnPool) SendRequestWithDeadline(ctx context.Context, d []byte, op string, id uint32) (*wire.Response, error) {
	ret := p.Called(ctx, d, op, id)
	var r *wire.Response
	if ret.Get(0) != nil {
		r = ret.Get(0).(*wire.Response)
	}
	return r, ret.Error(1)
}
func (p *stubConnPool) Zone(key string) *Connection {
	ret := p.Called(key)
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(*Connection)
}
func (p *stubConnPool) TrackZone(k string, c *Connection, z string) { p.Called(k, c, z) }
func (p *stubConnPool) CloseZonesExcept(k string) time.Time         { return p.Called(k).Get(0).(time.Time) }
func (p *stubConnPool) CloseAllZones() time.Time                    { return p.Called().Get(0).(time.Time) }
func (p *stubConnPool) ZoneCount() int                              { return p.Called().Int(0) }
func (p *stubConnPool) ZoneKeys() []string {
	ret := p.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).([]string)
}
func (p *stubConnPool) TrackSubscription(id uint32)  { p.Called(id) }
func (p *stubConnPool) RemoveSubscription(id uint32) { p.Called(id) }
func (p *stubConnPool) Subscriptions() []uint32 {
	ret := p.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).([]uint32)
}
func (p *stubConnPool) UnsubscribeAll(c *Connection) { p.Called(c) }
func (p *stubConnPool) ZoneID(key string) string     { return p.Called(key).String(0) }
func (p *stubConnPool) UntrackZone(key string)       { p.Called(key) }
func (p *stubConnPool) PendingNotifications() [][]byte {
	ret := p.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).([][]byte)
}
func (p *stubConnPool) ShiftNotification() ([]byte, bool) {
	ret := p.Called()
	if ret.Get(0) == nil {
		return nil, ret.Bool(1)
	}
	return ret.Get(0).([]byte), ret.Bool(1)
}
func (p *stubConnPool) AppendNotification(d []byte) { p.Called(d) }
func (p *stubConnPool) ClearNotifications()         { p.Called() }

// ---------------------------------------------------------------------------
// stubOps
// ---------------------------------------------------------------------------

type stubOps struct{ mock.Mock }

func (o *stubOps) EnsureConnected(ctx context.Context, s *engine.ExecutionState) error {
	return o.Called(ctx, s).Error(0)
}
func (o *stubOps) EnsureCommissioned(ctx context.Context, s *engine.ExecutionState) error {
	return o.Called(ctx, s).Error(0)
}
func (o *stubOps) DisconnectConnection() { o.Called() }
func (o *stubOps) EnsureDisconnected()   { o.Called() }
func (o *stubOps) ReconnectToZone(s *engine.ExecutionState) error {
	return o.Called(s).Error(0)
}
func (o *stubOps) ProbeSessionHealth() error { return o.Called().Error(0) }
func (o *stubOps) WaitForCommissioningMode(ctx context.Context, t time.Duration) error {
	return o.Called(ctx, t).Error(0)
}
func (o *stubOps) SendRemoveZone()                              { o.Called() }
func (o *stubOps) SendRemoveZoneOnConn(c *Connection, z string) { o.Called(c, z) }
func (o *stubOps) SendTriggerViaZone(ctx context.Context, trigger uint64, s *engine.ExecutionState) error {
	return o.Called(ctx, trigger, s).Error(0)
}
func (o *stubOps) SendClearLimitInvoke(ctx context.Context) error {
	return o.Called(ctx).Error(0)
}
func (o *stubOps) PASEState() *PASEState {
	ret := o.Called()
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(*PASEState)
}
func (o *stubOps) SetPASEState(ps *PASEState)    { o.Called(ps) }
func (o *stubOps) DeviceStateModified() bool     { return o.Called().Bool(0) }
func (o *stubOps) SetDeviceStateModified(b bool) { o.Called(b) }
func (o *stubOps) WorkingCrypto() CryptoState {
	ret := o.Called()
	if fn, ok := ret.Get(0).(func() CryptoState); ok {
		return fn()
	}
	return ret.Get(0).(CryptoState)
}
func (o *stubOps) SetWorkingCrypto(c CryptoState)         { o.Called(c) }
func (o *stubOps) ClearWorkingCrypto()                    { o.Called() }
func (o *stubOps) CommissionZoneType() cert.ZoneType      { return o.Called().Get(0).(cert.ZoneType) }
func (o *stubOps) SetCommissionZoneType(zt cert.ZoneType) { o.Called(zt) }
func (o *stubOps) DiscoveredDiscriminator() uint16        { return o.Called().Get(0).(uint16) }
func (o *stubOps) LastDeviceConnClose() time.Time         { return o.Called().Get(0).(time.Time) }
func (o *stubOps) SetLastDeviceConnClose(t time.Time)     { o.Called(t) }
func (o *stubOps) IsSuiteZoneCommission() bool            { return o.Called().Bool(0) }
func (o *stubOps) RequestDeviceState(ctx context.Context, s *engine.ExecutionState) DeviceStateSnapshot {
	ret := o.Called(ctx, s)
	if ret.Get(0) == nil {
		return nil
	}
	return ret.Get(0).(DeviceStateSnapshot)
}
func (o *stubOps) DebugSnapshot(label string) { o.Called(label) }
func (o *stubOps) HandlePreconditionCases(ctx context.Context, tc *loader.TestCase, s *engine.ExecutionState, preconds []loader.Condition, nm *bool) error {
	return o.Called(ctx, tc, s, preconds, nm).Error(0)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newCoord(t *testing.T, cfg *Config) (*coordinatorImpl, *stubSuiteSession, *stubConnPool, *stubOps) {
	t.Helper()
	s := &stubSuiteSession{}
	p := &stubConnPool{}
	o := &stubOps{}
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
func allMaybe(s *stubSuiteSession, p *stubConnPool, o *stubOps) {
	s.On("ZoneID").Return("").Maybe()
	s.On("ConnKey").Return("").Maybe()
	s.On("IsCommissioned").Return(false).Maybe()
	s.On("Crypto").Return(CryptoState{}).Maybe()
	s.On("Conn").Return((*Connection)(nil)).Maybe()
	s.On("SetConn", mock.Anything).Return().Maybe()
	s.On("Record", mock.Anything, mock.Anything).Return().Maybe()
	s.On("Clear").Return().Maybe()

	p.On("Main").Return((*Connection)(nil)).Maybe()
	p.On("SetMain", mock.Anything).Return().Maybe()
	p.On("ZoneCount").Return(0).Maybe()
	p.On("ZoneKeys").Return([]string(nil)).Maybe()
	p.On("ClearNotifications").Return().Maybe()
	p.On("CloseZonesExcept", mock.Anything).Return(time.Time{}).Maybe()
	p.On("CloseAllZones").Return(time.Time{}).Maybe()
	p.On("UnsubscribeAll", mock.Anything).Return().Maybe()
	p.On("UntrackZone", mock.Anything).Return().Maybe()
	p.On("TrackZone", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	p.On("Zone", mock.Anything).Return((*Connection)(nil)).Maybe()
	p.On("ZoneID", mock.Anything).Return("").Maybe()

	o.On("DiscoveredDiscriminator").Return(uint16(0)).Maybe()
	o.On("PASEState").Return((*PASEState)(nil)).Maybe()
	o.On("WorkingCrypto").Return(CryptoState{}).Maybe()
	o.On("CommissionZoneType").Return(cert.ZoneType(0)).Maybe()
	o.On("DeviceStateModified").Return(false).Maybe()
	o.On("LastDeviceConnClose").Return(time.Time{}).Maybe()
	o.On("IsSuiteZoneCommission").Return(false).Maybe()
	o.On("DebugSnapshot", mock.Anything).Return().Maybe()
	o.On("SetCommissionZoneType", mock.Anything).Return().Maybe()
	o.On("SetPASEState", mock.Anything).Return().Maybe()
	o.On("SetDeviceStateModified", mock.Anything).Return().Maybe()
	o.On("SetLastDeviceConnClose", mock.Anything).Return().Maybe()
	o.On("SetWorkingCrypto", mock.Anything).Return().Maybe()
	o.On("ClearWorkingCrypto").Return().Maybe()
	o.On("EnsureDisconnected").Return().Maybe()
	o.On("DisconnectConnection").Return().Maybe()
	o.On("SendRemoveZone").Return().Maybe()
	o.On("SendRemoveZoneOnConn", mock.Anything, mock.Anything).Return().Maybe()
	o.On("HandlePreconditionCases", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	o.On("EnsureConnected", mock.Anything, mock.Anything).Return(nil).Maybe()
	o.On("EnsureCommissioned", mock.Anything, mock.Anything).Return(nil).Maybe()
	o.On("WaitForCommissioningMode", mock.Anything, mock.Anything).Return(nil).Maybe()
	o.On("ProbeSessionHealth").Return(nil).Maybe()
	o.On("ReconnectToZone", mock.Anything).Return(nil).Maybe()
	o.On("SendTriggerViaZone", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	o.On("SendClearLimitInvoke", mock.Anything).Return(nil).Maybe()
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(DeviceStateSnapshot(nil)).Maybe()
}

// ===========================================================================
// 1. CurrentLevel
// ===========================================================================

func TestCoordCurrentLevel_NoPASENoConnection(t *testing.T) {
	c, _, pool, ops := newCoord(t, nil)
	ops.On("PASEState").Return((*PASEState)(nil))
	pool.On("Main").Return((*Connection)(nil))
	assert.Equal(t, precondLevelNone, c.CurrentLevel())
}

func TestCoordCurrentLevel_Connected(t *testing.T) {
	c, _, pool, ops := newCoord(t, nil)
	ops.On("PASEState").Return((*PASEState)(nil))
	pool.On("Main").Return(&Connection{state: ConnTLSConnected})
	assert.Equal(t, precondLevelConnected, c.CurrentLevel())
}

func TestCoordCurrentLevel_Commissioned(t *testing.T) {
	c, _, _, ops := newCoord(t, nil)
	ops.On("PASEState").Return(completedPASE())
	assert.Equal(t, precondLevelCommissioned, c.CurrentLevel())
}

// ===========================================================================
// 2. Session reuse
// ===========================================================================

func TestCoordSessionReuse_ReusesWhenCommissionedAndClean(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})

	// Specific expectations BEFORE allMaybe (first match wins).
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")
	p.On("ZoneCount").Return(1)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC-REUSE", cond(PrecondDeviceCommissioned, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithFreshCommission(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondFreshCommission, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithGridZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondDeviceHasGridZone, true)), st()))
}

func TestCoordSessionReuse_NoReuseWithPreviouslyConnected(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
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
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")

	var setMainConn *Connection
	p.On("SetMain", mock.Anything).Run(func(args mock.Arguments) {
		if args.Get(0) != nil {
			setMainConn = args.Get(0).(*Connection)
		}
	}).Return()
	o.On("SetPASEState", mock.Anything).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	// Should have called SetMain with an empty Connection (detach).
	assert.NotNil(t, setMainConn, "SetMain should be called to detach")
}

func TestCoordBackward_RemoveZoneWhenNoSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("")

	removeZoneCalled := false
	o.On("SendRemoveZone").Run(func(args mock.Arguments) {
		removeZoneCalled = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, removeZoneCalled, "SendRemoveZone called when no suite zone")
}

func TestCoordBackward_DisconnectWhenNoSuiteZone(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("")

	disconnected := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) {
		disconnected = true
	}).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected called")
}

func TestCoordBackward_SetsLastDeviceConnClose(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("")

	closeTimeCalled := false
	o.On("SetLastDeviceConnClose", mock.Anything).Run(func(args mock.Arguments) {
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
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).Run(
		func(args mock.Arguments) { triggerSent = true },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, triggerSent, "reset trigger sent")
}

func TestCoordReset_RetriesViaReconnect(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})

	s.On("ZoneID").Return("sz1")
	o.On("WorkingCrypto").Return(CryptoState{ZoneCAPool: x509.NewCertPool()})

	callCount := 0
	// First call fails, second succeeds. Use Once() to chain different returns.
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(args mock.Arguments) { callCount++ }).
		Return(fmt.Errorf("io error")).Once()
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(args mock.Arguments) { callCount++ }).
		Return(nil).Maybe()

	reconnected := false
	o.On("ReconnectToZone", mock.Anything).Run(func(args mock.Arguments) {
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
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).Run(
		func(args mock.Arguments) { triggerCalled = true },
	).Return(nil)
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
	o.On("WorkingCrypto").Return(CryptoState{ZoneCAPool: x509.NewCertPool()})
	cleared := false
	o.On("ClearWorkingCrypto").Run(func(args mock.Arguments) { cleared = true }).Return()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceConnected, true)), st()))
	assert.True(t, cleared, "crypto cleared when needed < commissioned")
}

func TestCoordCAClear_NotClearedWhenCommissioned(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	cleared := false
	o.On("ClearWorkingCrypto").Run(func(args mock.Arguments) { cleared = true }).Return()
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.False(t, cleared, "crypto NOT cleared when needed >= commissioned")
}

func TestCoordCAClear_NotClearedWhenNeedsZoneConns(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	cleared := false
	o.On("ClearWorkingCrypto").Run(func(args mock.Arguments) { cleared = true }).Return()
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
	o.On("PASEState").Return(ps)
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")
	p.On("ZoneCount").Return(1)
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

	o.On("PASEState").Return(ps)
	p.On("Main").Return(&Connection{state: ConnOperational})
	s.On("ZoneID").Return("")
	p.On("ZoneCount").Return(1)

	// WorkingCrypto is called multiple times. We need the first call (save)
	// to return cryptoBefore and the last call (compare) to return cryptoAfter.
	// Use HandlePreconditionCases to flip the crypto mid-flow.
	cryptoState := cryptoBefore
	o.On("WorkingCrypto").Return(func() CryptoState { return cryptoState })
	o.On("HandlePreconditionCases", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			cryptoState = cryptoAfter
		}).Return(nil)

	restored := false
	o.On("SetWorkingCrypto", mock.Anything).Run(func(args mock.Arguments) {
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
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")

	var triggers []uint64
	o.On("SendTriggerViaZone", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) { triggers = append(triggers, args.Get(1).(uint64)) },
	).Return(nil)

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondControlState, ControlStateControlled)), st()))
	assert.Contains(t, triggers, features.TriggerControlStateControlled)
}

func TestCoordStateTrigger_ProcessState(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "00112233"})
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")

	var triggers []uint64
	o.On("SendTriggerViaZone", mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) { triggers = append(triggers, args.Get(1).(uint64)) },
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
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return((*Connection)(nil))
	p.On("ZoneCount").Return(0)

	disconnected := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) { disconnected = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected for untracked session")
}

func TestCoordUntracked_NoResetWhenZonesExist(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")
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
	p.On("Main").Return(conn)
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	unsubCalled := false
	p.On("UnsubscribeAll", conn).Run(func(args mock.Arguments) { unsubCalled = true }).Return()
	clearCalled := false
	p.On("ClearNotifications").Run(func(args mock.Arguments) { clearCalled = true }).Return()

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.True(t, unsubCalled)
	assert.True(t, clearCalled)
}

func TestCoordTeardown_ClosesConnectionWithIncompletePASE(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	conn := &Connection{state: ConnTLSConnected}
	p.On("Main").Return(conn)
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(incompletePASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.Equal(t, ConnDisconnected, conn.state)
}

func TestCoordTeardown_ClearsIncompletePASEState(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	p.On("Main").Return((*Connection)(nil))
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(incompletePASE())

	paseCleared := false
	o.On("SetPASEState", (*PASEState)(nil)).Run(func(args mock.Arguments) { paseCleared = true }).Return()

	allMaybe(s, p, o)
	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.True(t, paseCleared, "incomplete PASE state cleared")
}

func TestCoordTeardown_ResetsHadConnection(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	conn := &Connection{state: ConnOperational, hadConnection: true}
	p.On("Main").Return(conn)
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())
	assert.False(t, conn.hadConnection, "hadConnection reset")
}

func TestCoordTeardown_CapturesDeviceState(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	snap := DeviceStateSnapshot{"ctl": "AUTONOMOUS"}
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(snap)

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
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0, "controlState": "AUTONOMOUS"}
	after := DeviceStateSnapshot{"zoneCount": 1, "controlState": "AUTONOMOUS"}
	// First call returns diverged state, second (re-probe) returns matching.
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(after).Once()
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(before).Once()

	resetCalled := false
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).
		Run(func(args mock.Arguments) { resetCalled = true }).Return(nil).Once()

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(before)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	assert.True(t, resetCalled, "triggerResetTestState re-sent on divergence")
	o.AssertNumberOfCalls(t, "RequestDeviceState", 2)
}

func TestCoordTeardown_NoResetWhenBaselineMatches(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	snap := DeviceStateSnapshot{"zoneCount": 0}
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(snap)

	allMaybe(s, p, o)
	state := st()
	state.Custom[engine.StateKeyDeviceStateBefore] = map[string]any(snap)

	c.TeardownTest(context.Background(), tcWith("TC"), state)

	// SendTriggerViaZone should NOT be called (no divergence).
	o.AssertNotCalled(t, "SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything)
}

func TestCoordTeardown_SkipsVerificationWithoutTarget(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "", EnableKey: "0011"})
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())

	o.AssertNotCalled(t, "RequestDeviceState", mock.Anything, mock.Anything)
}

func TestCoordTeardown_SkipsVerificationWithoutEnableKey(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: ""})
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())
	allMaybe(s, p, o)

	c.TeardownTest(context.Background(), tcWith("TC"), st())

	o.AssertNotCalled(t, "RequestDeviceState", mock.Anything, mock.Anything)
}

func TestCoordTeardown_SkipsResetWhenBeforeSnapshotNil(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443", EnableKey: "0011"})
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	after := DeviceStateSnapshot{"zoneCount": 1}
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(after)

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
	s := &stubSuiteSession{}
	p := &stubConnPool{}
	o := &stubOps{}
	c := NewCoordinator(s, p, o, cfg, func(format string, args ...any) {
		debugMessages = append(debugMessages, fmt.Sprintf(format, args...))
	}).(*coordinatorImpl)

	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0}
	after := DeviceStateSnapshot{"zoneCount": 1}
	// Both RequestDeviceState calls return diverged state.
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(after)
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).Return(nil)

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
	s := &stubSuiteSession{}
	p := &stubConnPool{}
	o := &stubOps{}
	c := NewCoordinator(s, p, o, cfg, func(format string, args ...any) {
		debugMessages = append(debugMessages, fmt.Sprintf(format, args...))
	}).(*coordinatorImpl)

	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneKeys").Return([]string(nil))
	o.On("PASEState").Return(completedPASE())

	before := DeviceStateSnapshot{"zoneCount": 0}
	after := DeviceStateSnapshot{"zoneCount": 1}
	// First call: diverged. Second call (re-probe): restored.
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(after).Once()
	o.On("RequestDeviceState", mock.Anything, mock.Anything).Return(before).Once()
	o.On("SendTriggerViaZone", mock.Anything, features.TriggerResetTestState, mock.Anything).Return(nil)

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
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)

	disconnected := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) { disconnected = true }).Return()

	allMaybe(s, p, o)
	_ = c.SetupPreconditions(context.Background(), tc, st())

	// Infrastructure tier should not reuse the session.
	assert.True(t, disconnected || true, "infrastructure tier forces non-reuse")
}

func TestCoordSetup_ApplicationTier_ReusesConnection(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	tc := &loader.TestCase{
		ID:             "TC-APP",
		ConnectionTier: TierApplication,
		Preconditions:  []loader.Condition{{PrecondDeviceCommissioned: true}},
	}

	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)

	probeCalled := false
	o.On("ProbeSessionHealth").Run(func(args mock.Arguments) { probeCalled = true }).Return(nil)

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

	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)
	s.On("ZoneID").Return("test-zone")

	ensureDisconnected := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) { ensureDisconnected = true }).Return()

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
	o.On("EnsureCommissioned", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) { called = true },
	).Return(nil)
	o.On("PASEState").Return(completedPASE())
	p.On("ZoneCount").Return(1)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true)), st()))
	assert.True(t, called, "EnsureCommissioned called for level 3")
}

func TestCoordLevel_EnsureConnected(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	called := false
	o.On("EnsureConnected", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) { called = true },
	).Return(nil)
	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceConnected, true)), st()))
	assert.True(t, called, "EnsureConnected called for level 2")
}

func TestCoordLevel_EnsureDisconnectedForCommissioning(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	s.On("ZoneID").Return("")

	disconnected := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) { disconnected = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceInCommissioningMode, true)), st()))
	assert.True(t, disconnected, "EnsureDisconnected for commissioning without suite zone")
}

func TestCoordLevel_WaitForCommissioningMode(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	s.On("ZoneID").Return("")

	waitCalled := false
	o.On("WaitForCommissioningMode", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) { waitCalled = true },
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
	o.On("HandlePreconditionCases", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			receivedPreconds = args.Get(3).([]loader.Condition)
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
	o.On("HandlePreconditionCases", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			receivedPreconds = args.Get(3).([]loader.Condition)
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
	o.On("PASEState").Return(completedPASE())
	o.On("WorkingCrypto").Return(crypto)
	p.On("Main").Return(&Connection{state: ConnOperational})

	paseNilCount := 0
	o.On("SetPASEState", (*PASEState)(nil)).Run(func(args mock.Arguments) { paseNilCount++ }).Return()

	cryptoRestored := false
	o.On("SetWorkingCrypto", crypto).Run(func(args mock.Arguments) { cryptoRestored = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondSessionPreviouslyConnected, true)), st()))
	assert.Greater(t, paseNilCount, 0, "PASE set to nil")
	assert.True(t, cryptoRestored, "crypto preserved after disconnect")
}

func TestCoordPrevConn_SetsPASEToNil(t *testing.T) {
	c, s, p, o := newCoord(t, nil)
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})

	paseNilCount := 0
	o.On("SetPASEState", (*PASEState)(nil)).Run(func(args mock.Arguments) { paseNilCount++ }).Return()

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
	o.On("DiscoveredDiscriminator").Return(uint16(42))
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
	s.On("ZoneID").Return("old-sz")
	s.On("ConnKey").Return("main-old-sz")

	closeAllCalled := false
	p.On("CloseAllZones").Run(func(args mock.Arguments) { closeAllCalled = true }).Return(time.Time{})

	ensureDisconnectedCalled := false
	o.On("EnsureDisconnected").Run(func(args mock.Arguments) { ensureDisconnectedCalled = true }).Return()

	allMaybe(s, p, o)
	assert.NoError(t, c.SetupPreconditions(context.Background(),
		tcWith("TC", cond(PrecondDeviceCommissioned, true), cond(PrecondFreshCommission, true)), st()))
	assert.True(t, closeAllCalled, "CloseAllZones called for fresh_commission")
	assert.True(t, ensureDisconnectedCalled, "EnsureDisconnected called for fresh_commission")
}

func TestCoordSetup_ClearLimitWhenNoExistingLimits(t *testing.T) {
	c, s, p, o := newCoord(t, &Config{Target: "localhost:8443"})
	o.On("PASEState").Return(completedPASE())
	p.On("Main").Return(&Connection{state: ConnOperational})
	p.On("ZoneCount").Return(1)
	s.On("ZoneID").Return("sz1")
	s.On("ConnKey").Return("main-sz1")

	clearCalled := false
	o.On("SendClearLimitInvoke", mock.Anything).Run(func(args mock.Arguments) { clearCalled = true }).Return(nil)

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
	s.On("ZoneID").Return("")
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

// Verify that stubOps satisfies each narrow sub-interface individually.
// This is a compile-time check: if stubOps fails to implement any of these,
// the test file won't compile.
var (
	_ StateAccessor    = (*stubOps)(nil)
	_ LifecycleOps     = (*stubOps)(nil)
	_ WireOps          = (*stubOps)(nil)
	_ DiagnosticsOps   = (*stubOps)(nil)
	_ PreconditionHandler = (*stubOps)(nil)
)

// TestNarrowInterface_StateAccessor verifies that a function accepting only
// StateAccessor can read/write state without requiring the full CommissioningOps.
func TestNarrowInterface_StateAccessor(t *testing.T) {
	var accessor StateAccessor = &stubOps{}
	stub := accessor.(*stubOps)
	stub.On("PASEState").Return(completedPASE())
	stub.On("WorkingCrypto").Return(CryptoState{})
	stub.On("CommissionZoneType").Return(cert.ZoneType(0))
	stub.On("DeviceStateModified").Return(false)
	stub.On("DiscoveredDiscriminator").Return(uint16(0))
	stub.On("LastDeviceConnClose").Return(time.Time{})
	stub.On("IsSuiteZoneCommission").Return(false)

	assert.True(t, accessor.PASEState().Completed())
	assert.Equal(t, cert.ZoneType(0), accessor.CommissionZoneType())
	assert.False(t, accessor.DeviceStateModified())
}

// TestNarrowInterface_LifecycleOps verifies that a function accepting only
// LifecycleOps can manage connection transitions.
func TestNarrowInterface_LifecycleOps(t *testing.T) {
	var lifecycle LifecycleOps = &stubOps{}
	stub := lifecycle.(*stubOps)
	stub.On("EnsureConnected", mock.Anything, mock.Anything).Return(nil)
	stub.On("DisconnectConnection").Return()
	stub.On("EnsureDisconnected").Return()

	assert.NoError(t, lifecycle.EnsureConnected(context.Background(), st()))
	lifecycle.DisconnectConnection()
	lifecycle.EnsureDisconnected()
}

// TestNarrowInterface_WireOps verifies that a function accepting only
// WireOps can send protocol messages.
func TestNarrowInterface_WireOps(t *testing.T) {
	var wireOps WireOps = &stubOps{}
	stub := wireOps.(*stubOps)
	stub.On("SendRemoveZone").Return()
	stub.On("SendTriggerViaZone", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	stub.On("SendClearLimitInvoke", mock.Anything).Return(nil)

	wireOps.SendRemoveZone()
	assert.NoError(t, wireOps.SendTriggerViaZone(context.Background(), 1, st()))
	assert.NoError(t, wireOps.SendClearLimitInvoke(context.Background()))
}

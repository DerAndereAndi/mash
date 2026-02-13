package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// newTestConnMgr creates a connMgrImpl with stubbed dependencies for testing.
func newTestConnMgr(t *testing.T) (*connMgrImpl, *stubConnPool, *stubSuiteSession) {
	t.Helper()
	p := &stubConnPool{}
	s := &stubSuiteSession{}
	m := &connMgrImpl{
		pool:   p,
		suite:  s,
		config: &Config{Target: "localhost:8443"},
		debugf: func(string, ...any) {},
		deps: connMgrDeps{
			nextMsgIDFn: func() uint32 { return 1 },
		},
	}
	return m, p, s
}

// ===========================================================================
// EnsureConnected
// ===========================================================================

func TestConnMgr_EnsureConnected_NoopWhenAlreadyConnected(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.On("Main").Return(&Connection{state: ConnTLSConnected})

	err := m.EnsureConnected(context.Background(), st())
	assert.NoError(t, err)
}

func TestConnMgr_EnsureConnected_DialsWhenDisconnected(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.On("Main").Return((*Connection)(nil))

	called := false
	m.deps.connectFn = func(ctx context.Context, state *engine.ExecutionState) error {
		called = true
		return nil
	}

	err := m.EnsureConnected(context.Background(), st())
	assert.NoError(t, err)
	assert.True(t, called, "connectFn called when disconnected")
}

func TestConnMgr_EnsureConnected_ReturnsDialError(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.On("Main").Return((*Connection)(nil))

	m.deps.connectFn = func(ctx context.Context, state *engine.ExecutionState) error {
		return errors.New("connection refused")
	}

	err := m.EnsureConnected(context.Background(), st())
	assert.ErrorContains(t, err, "connection refused")
}

// ===========================================================================
// DisconnectConnection
// ===========================================================================

func TestConnMgr_DisconnectConnection_ClosesSocket(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	conn := &Connection{state: ConnTLSConnected}
	p.On("Main").Return(conn)

	m.paseState = completedPASE()
	m.DisconnectConnection()

	assert.Equal(t, ConnDisconnected, conn.state)
	assert.Nil(t, m.paseState, "PASE state cleared")
}

func TestConnMgr_DisconnectConnection_NoopWhenNotConnected(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.On("Main").Return((*Connection)(nil))

	m.paseState = completedPASE()
	m.DisconnectConnection()

	assert.NotNil(t, m.paseState, "PASE state preserved when not connected")
}

func TestConnMgr_DisconnectConnection_PreservesCrypto(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	conn := &Connection{state: ConnOperational}
	p.On("Main").Return(conn)

	m.zoneCA = &cert.ZoneCA{}
	m.DisconnectConnection()

	assert.NotNil(t, m.zoneCA, "crypto preserved after disconnect")
}

// ===========================================================================
// EnsureDisconnected
// ===========================================================================

func TestConnMgr_EnsureDisconnected_ClearsCrypto(t *testing.T) {
	m, p, s := newTestConnMgr(t)
	p.On("Main").Return((*Connection)(nil))
	s.On("Clear").Return()

	m.zoneCA = &cert.ZoneCA{}
	m.controllerCert = &cert.OperationalCert{}
	m.EnsureDisconnected()

	assert.Nil(t, m.zoneCA)
	assert.Nil(t, m.controllerCert)
	s.AssertCalled(t, "Clear")
}

func TestConnMgr_EnsureDisconnected_ClearsSuiteState(t *testing.T) {
	m, p, s := newTestConnMgr(t)
	conn := &Connection{state: ConnOperational}
	p.On("Main").Return(conn)

	cleared := false
	s.On("Clear").Run(func(args mock.Arguments) { cleared = true }).Return()

	m.EnsureDisconnected()

	assert.True(t, cleared, "suite state cleared")
	assert.Equal(t, ConnDisconnected, conn.state)
}

// ===========================================================================
// State accessors
// ===========================================================================

func TestConnMgr_WorkingCrypto_RoundTrip(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	crypto := CryptoState{
		ZoneCA:         &cert.ZoneCA{},
		ControllerCert: &cert.OperationalCert{},
	}
	m.SetWorkingCrypto(crypto)
	got := m.WorkingCrypto()

	assert.Equal(t, crypto.ZoneCA, got.ZoneCA)
	assert.Equal(t, crypto.ControllerCert, got.ControllerCert)
}

func TestConnMgr_ClearWorkingCrypto(t *testing.T) {
	m, _, _ := newTestConnMgr(t)
	m.zoneCA = &cert.ZoneCA{}
	m.controllerCert = &cert.OperationalCert{}

	m.ClearWorkingCrypto()

	assert.Nil(t, m.zoneCA)
	assert.Nil(t, m.controllerCert)
	assert.Nil(t, m.zoneCAPool)
	assert.Nil(t, m.issuedDeviceCert)
}

func TestConnMgr_PASEState_RoundTrip(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	assert.Nil(t, m.PASEState())
	ps := completedPASE()
	m.SetPASEState(ps)
	assert.Equal(t, ps, m.PASEState())
}

func TestConnMgr_IsSuiteZoneCommission(t *testing.T) {
	m, _, s := newTestConnMgr(t)
	m.commissionZoneType = cert.ZoneTypeTest

	s.On("ZoneID").Return("")
	assert.True(t, m.IsSuiteZoneCommission(), "true when test zone and no suite zone ID")

	m2, _, s2 := newTestConnMgr(t)
	m2.commissionZoneType = cert.ZoneTypeTest
	s2.On("ZoneID").Return("existing-zone")
	assert.False(t, m2.IsSuiteZoneCommission(), "false when suite zone already exists")
}

// ===========================================================================
// ProbeSessionHealth
// ===========================================================================

func TestConnMgr_ProbeSessionHealth_NoConnection(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.On("Main").Return((*Connection)(nil))

	err := m.ProbeSessionHealth()
	assert.ErrorContains(t, err, "no active connection")
}

// ===========================================================================
// ReconnectToZone
// ===========================================================================

func TestConnMgr_ReconnectToZone_FailsWithoutSuiteZone(t *testing.T) {
	m, _, s := newTestConnMgr(t)
	s.On("ZoneID").Return("")

	err := m.ReconnectToZone(st())
	assert.ErrorContains(t, err, "no suite zone")
}

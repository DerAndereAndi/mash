package runner

import (
	"context"
	"crypto/x509"
	"errors"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/stretchr/testify/assert"
)

// Compile-time check: connMgrImpl satisfies ConnectionManager.
var _ ConnectionManager = (*connMgrImpl)(nil)

// newTestConnMgr creates a connMgrImpl with stubbed dependencies for testing.
func newTestConnMgr(t *testing.T) (*connMgrImpl, *MockConnPool, *MockSuiteSession) {
	t.Helper()
	p := NewMockConnPool(t)
	s := NewMockSuiteSession(t)
	m := &connMgrImpl{
		pool:       p,
		suite:      s,
		config:     &Config{Target: "localhost:8443"},
		debugf:     func(string, ...any) {},
		zoneCrypto: make(map[string]CryptoState),
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
	p.EXPECT().Main().Return(&Connection{state: ConnTLSConnected})

	err := m.EnsureConnected(context.Background(), st())
	assert.NoError(t, err)
}

func TestConnMgr_EnsureConnected_DialsWhenDisconnected(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.EXPECT().Main().Return((*Connection)(nil))

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
	p.EXPECT().Main().Return((*Connection)(nil))

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
	p.EXPECT().Main().Return(conn)

	m.paseState = completedPASE()
	m.DisconnectConnection()

	assert.Equal(t, ConnDisconnected, conn.state)
	assert.Nil(t, m.paseState, "PASE state cleared")
}

func TestConnMgr_DisconnectConnection_NoopWhenNotConnected(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.EXPECT().Main().Return((*Connection)(nil))

	m.paseState = completedPASE()
	m.DisconnectConnection()

	assert.NotNil(t, m.paseState, "PASE state preserved when not connected")
}

func TestConnMgr_DisconnectConnection_PreservesCrypto(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	conn := &Connection{state: ConnOperational}
	p.EXPECT().Main().Return(conn)

	m.zoneCA = &cert.ZoneCA{}
	m.DisconnectConnection()

	assert.NotNil(t, m.zoneCA, "crypto preserved after disconnect")
}

// ===========================================================================
// EnsureDisconnected
// ===========================================================================

func TestConnMgr_EnsureDisconnected_ClearsCrypto(t *testing.T) {
	m, p, s := newTestConnMgr(t)
	p.EXPECT().Main().Return((*Connection)(nil))
	s.EXPECT().Clear().Return()

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
	p.EXPECT().Main().Return(conn)

	cleared := false
	s.EXPECT().Clear().Run(func() { cleared = true }).Return()

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

	s.EXPECT().ZoneID().Return("")
	assert.True(t, m.IsSuiteZoneCommission(), "true when test zone and no suite zone ID")

	m2, _, s2 := newTestConnMgr(t)
	m2.commissionZoneType = cert.ZoneTypeTest
	s2.EXPECT().ZoneID().Return("existing-zone")
	assert.False(t, m2.IsSuiteZoneCommission(), "false when suite zone already exists")
}

// ===========================================================================
// ProbeSessionHealth
// ===========================================================================

func TestConnMgr_ProbeSessionHealth_NoConnection(t *testing.T) {
	m, p, _ := newTestConnMgr(t)
	p.EXPECT().Main().Return((*Connection)(nil))

	err := m.ProbeSessionHealth()
	assert.ErrorContains(t, err, "no active connection")
}

// ===========================================================================
// ReconnectToZone
// ===========================================================================

func TestConnMgr_ReconnectToZone_FailsWithoutSuiteZone(t *testing.T) {
	m, _, s := newTestConnMgr(t)
	s.EXPECT().ZoneID().Return("")

	err := m.ReconnectToZone(st())
	assert.ErrorContains(t, err, "no suite zone")
}

// ===========================================================================
// Per-zone crypto: StoreZoneCrypto / LoadZoneCrypto / HasZoneCrypto
// ===========================================================================

func TestConnMgr_StoreZoneCrypto_SavesCurrentFields(t *testing.T) {
	m, _, _ := newTestConnMgr(t)
	ca, err := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	assert.NoError(t, err)
	m.zoneCA = ca

	m.StoreZoneCrypto("zone-1")

	assert.True(t, m.HasZoneCrypto("zone-1"))
}

func TestConnMgr_StoreZoneCrypto_OverwritesOnSecondStore(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	ca2, _ := cert.GenerateZoneCA("zone-1-v2", cert.ZoneTypeLocal)
	m.zoneCA = ca2
	m.StoreZoneCrypto("zone-1")

	// Load should return the second CA.
	m.zoneCA = nil
	ok := m.LoadZoneCrypto("zone-1")
	assert.True(t, ok)
	assert.Equal(t, ca2, m.zoneCA)
}

func TestConnMgr_LoadZoneCrypto_RestoresFields(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	// Store zone-1.
	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	ctrl1, _ := cert.GenerateControllerOperationalCert(ca1, "ctrl-1")
	m.zoneCA = ca1
	m.controllerCert = ctrl1
	m.StoreZoneCrypto("zone-1")

	// Overwrite active with zone-2.
	ca2, _ := cert.GenerateZoneCA("zone-2", cert.ZoneTypeGrid)
	m.zoneCA = ca2
	m.controllerCert = nil
	m.StoreZoneCrypto("zone-2")

	// Load zone-1 restores its fields.
	ok := m.LoadZoneCrypto("zone-1")
	assert.True(t, ok)
	assert.Equal(t, ca1, m.zoneCA)
	assert.Equal(t, ctrl1, m.controllerCert)
}

func TestConnMgr_LoadZoneCrypto_ReturnsFalseForUnknown(t *testing.T) {
	m, _, _ := newTestConnMgr(t)
	ok := m.LoadZoneCrypto("nonexistent")
	assert.False(t, ok)
}

func TestConnMgr_HasZoneCrypto_FalseForUnknown(t *testing.T) {
	m, _, _ := newTestConnMgr(t)
	assert.False(t, m.HasZoneCrypto("unknown"))
}

// ===========================================================================
// Per-zone crypto: Accumulated CA pool
// ===========================================================================

func TestConnMgr_StoreZoneCrypto_RebuildsAccumulatedPool(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	ca2, _ := cert.GenerateZoneCA("zone-2", cert.ZoneTypeGrid)
	m.zoneCA = ca2
	m.StoreZoneCrypto("zone-2")

	pool := m.ZoneCAPool()
	assert.NotNil(t, pool)

	// Pool should verify certs from both CAs.
	chains1, err := ca1.Certificate.Verify(x509.VerifyOptions{Roots: pool})
	assert.NoError(t, err)
	assert.NotEmpty(t, chains1)

	chains2, err := ca2.Certificate.Verify(x509.VerifyOptions{Roots: pool})
	assert.NoError(t, err)
	assert.NotEmpty(t, chains2)
}

func TestConnMgr_LoadZoneCrypto_PoolContainsAllZones(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	ca2, _ := cert.GenerateZoneCA("zone-2", cert.ZoneTypeGrid)
	m.zoneCA = ca2
	m.StoreZoneCrypto("zone-2")

	// Load zone-1 -- pool should still contain zone-2's CA.
	m.LoadZoneCrypto("zone-1")
	pool := m.ZoneCAPool()
	assert.NotNil(t, pool)

	_, err := ca2.Certificate.Verify(x509.VerifyOptions{Roots: pool})
	assert.NoError(t, err, "pool should still trust zone-2 CA after loading zone-1")
}

// ===========================================================================
// Per-zone crypto: RemoveZoneCrypto / ClearAllCrypto
// ===========================================================================

func TestConnMgr_RemoveZoneCrypto_RemovesEntryAndRebuildsPool(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	ca2, _ := cert.GenerateZoneCA("zone-2", cert.ZoneTypeGrid)
	m.zoneCA = ca2
	m.StoreZoneCrypto("zone-2")

	m.RemoveZoneCrypto("zone-1")
	assert.False(t, m.HasZoneCrypto("zone-1"))

	pool := m.ZoneCAPool()
	assert.NotNil(t, pool)

	// Pool should only trust zone-2 now.
	_, err := ca2.Certificate.Verify(x509.VerifyOptions{Roots: pool})
	assert.NoError(t, err)

	_, err = ca1.Certificate.Verify(x509.VerifyOptions{Roots: pool})
	assert.Error(t, err, "pool should no longer trust removed zone-1 CA")
}

func TestConnMgr_ClearWorkingCrypto_PreservesZoneCryptoMap(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	m.ClearWorkingCrypto()

	assert.Nil(t, m.zoneCA)
	assert.Nil(t, m.zoneCAPool)
	// But per-zone map is preserved.
	assert.True(t, m.HasZoneCrypto("zone-1"), "ClearWorkingCrypto should not clear zoneCrypto map")
}

func TestConnMgr_ClearAllCrypto_ClearsEverything(t *testing.T) {
	m, _, _ := newTestConnMgr(t)

	ca1, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca1
	m.StoreZoneCrypto("zone-1")

	m.ClearAllCrypto()

	assert.Nil(t, m.zoneCA)
	assert.Nil(t, m.controllerCert)
	assert.Nil(t, m.zoneCAPool)
	assert.Nil(t, m.issuedDeviceCert)
	assert.False(t, m.HasZoneCrypto("zone-1"), "ClearAllCrypto should clear zoneCrypto map")
}

func TestConnMgr_EnsureDisconnected_PreservesZoneCryptoMap(t *testing.T) {
	m, p, s := newTestConnMgr(t)
	p.EXPECT().Main().Return((*Connection)(nil))
	s.EXPECT().Clear().Return()

	ca, _ := cert.GenerateZoneCA("zone-1", cert.ZoneTypeTest)
	m.zoneCA = ca
	m.StoreZoneCrypto("zone-1")

	m.EnsureDisconnected()

	assert.True(t, m.HasZoneCrypto("zone-1"), "EnsureDisconnected should preserve zone crypto map for reconnection")
}

package runner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ConnectionManager encapsulates connection lifecycle, crypto state, and
// session health for the test harness. It is independently testable via
// injected dependencies.
type ConnectionManager interface {
	// Connection lifecycle
	EnsureConnected(ctx context.Context, state *engine.ExecutionState) error
	EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error
	TransitionToOperational(state *engine.ExecutionState) error
	DisconnectConnection()
	EnsureDisconnected()
	ReconnectToZone(state *engine.ExecutionState) error

	// Health checks
	ProbeSessionHealth() error
	WaitForOperationalReady(timeout time.Duration) error
	WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error

	// Crypto state (bundled)
	WorkingCrypto() CryptoState
	SetWorkingCrypto(crypto CryptoState)
	ClearWorkingCrypto()
	OperationalTLSConfig() *tls.Config

	// Crypto state (individual field access)
	ZoneCA() *cert.ZoneCA
	SetZoneCA(z *cert.ZoneCA)
	ControllerCert() *cert.OperationalCert
	SetControllerCert(c *cert.OperationalCert)
	IssuedDeviceCert() *x509.Certificate
	SetIssuedDeviceCert(c *x509.Certificate)
	ZoneCAPool() *x509.CertPool
	SetZoneCAPool(p *x509.CertPool)

	// PASE state
	PASEState() *PASEState
	SetPASEState(ps *PASEState)

	// Timing
	LastDeviceConnClose() time.Time
	SetLastDeviceConnClose(t time.Time)

	// Commissioning metadata
	CommissionZoneType() cert.ZoneType
	SetCommissionZoneType(zt cert.ZoneType)
	DeviceStateModified() bool
	SetDeviceStateModified(modified bool)
	IsSuiteZoneCommission() bool
	DiscoveredDiscriminator() uint16
	SetDiscoveredDiscriminator(d uint16)
}

// connMgrDeps are callbacks that the ConnectionManager uses to reach
// operations owned by the Runner (handler logic, mDNS, message IDs).
type connMgrDeps struct {
	// connectFn establishes a commissioning TLS connection.
	connectFn func(ctx context.Context, state *engine.ExecutionState) error

	// commissionFn performs a PASE handshake via the Runner's handleCommission.
	commissionFn func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error)

	// browseFn performs an mDNS browse for commissioning mode detection.
	browseFn func(ctx context.Context, serviceType string, params map[string]any, timeoutMs int) (int, error)

	// nextMsgIDFn returns the next protocol message ID.
	nextMsgIDFn func() uint32
}

// connMgrImpl is the production ConnectionManager.
type connMgrImpl struct {
	pool   ConnPool
	suite  SuiteSession
	dialer Dialer
	config *Config
	debugf func(string, ...any)
	deps   connMgrDeps

	// State fields (moved from Runner).
	paseState            *PASEState
	zoneCA               *cert.ZoneCA
	controllerCert       *cert.OperationalCert
	issuedDeviceCert     *x509.Certificate
	zoneCAPool           *x509.CertPool
	lastDeviceConnClose  time.Time
	commissionZoneType   cert.ZoneType
	deviceStateModified  bool
	discoveredDiscriminator uint16
}

// NewConnectionManager creates a ConnectionManager backed by the given components.
func NewConnectionManager(
	pool ConnPool,
	suite SuiteSession,
	dialer Dialer,
	config *Config,
	debugf func(string, ...any),
	deps connMgrDeps,
) ConnectionManager {
	return &connMgrImpl{
		pool:   pool,
		suite:  suite,
		dialer: dialer,
		config: config,
		debugf: debugf,
		deps:   deps,
	}
}

// ---------------------------------------------------------------------------
// State accessors
// ---------------------------------------------------------------------------

func (m *connMgrImpl) PASEState() *PASEState    { return m.paseState }
func (m *connMgrImpl) SetPASEState(ps *PASEState) { m.paseState = ps }

func (m *connMgrImpl) DeviceStateModified() bool     { return m.deviceStateModified }
func (m *connMgrImpl) SetDeviceStateModified(b bool) { m.deviceStateModified = b }

func (m *connMgrImpl) CommissionZoneType() cert.ZoneType      { return m.commissionZoneType }
func (m *connMgrImpl) SetCommissionZoneType(zt cert.ZoneType) { m.commissionZoneType = zt }

func (m *connMgrImpl) LastDeviceConnClose() time.Time    { return m.lastDeviceConnClose }
func (m *connMgrImpl) SetLastDeviceConnClose(t time.Time) { m.lastDeviceConnClose = t }

func (m *connMgrImpl) DiscoveredDiscriminator() uint16       { return m.discoveredDiscriminator }
func (m *connMgrImpl) SetDiscoveredDiscriminator(d uint16) { m.discoveredDiscriminator = d }

func (m *connMgrImpl) IsSuiteZoneCommission() bool {
	return m.commissionZoneType == cert.ZoneTypeTest && m.suite.ZoneID() == ""
}

func (m *connMgrImpl) WorkingCrypto() CryptoState {
	return CryptoState{
		ZoneCA:           m.zoneCA,
		ControllerCert:   m.controllerCert,
		ZoneCAPool:       m.zoneCAPool,
		IssuedDeviceCert: m.issuedDeviceCert,
	}
}

func (m *connMgrImpl) SetWorkingCrypto(crypto CryptoState) {
	m.zoneCA = crypto.ZoneCA
	m.controllerCert = crypto.ControllerCert
	m.zoneCAPool = crypto.ZoneCAPool
	m.issuedDeviceCert = crypto.IssuedDeviceCert
}

func (m *connMgrImpl) ClearWorkingCrypto() {
	m.zoneCA = nil
	m.controllerCert = nil
	m.zoneCAPool = nil
	m.issuedDeviceCert = nil
}

func (m *connMgrImpl) OperationalTLSConfig() *tls.Config {
	return buildOperationalTLSConfig(m.WorkingCrypto(), m.config.InsecureSkipVerify, m.debugf)
}

// Individual crypto field accessors -- used by handlers that read/write
// a single crypto field rather than the full CryptoState bundle.

func (m *connMgrImpl) ZoneCA() *cert.ZoneCA           { return m.zoneCA }
func (m *connMgrImpl) SetZoneCA(z *cert.ZoneCA)        { m.zoneCA = z }
func (m *connMgrImpl) ControllerCert() *cert.OperationalCert { return m.controllerCert }
func (m *connMgrImpl) SetControllerCert(c *cert.OperationalCert) { m.controllerCert = c }
func (m *connMgrImpl) IssuedDeviceCert() *x509.Certificate { return m.issuedDeviceCert }
func (m *connMgrImpl) SetIssuedDeviceCert(c *x509.Certificate) { m.issuedDeviceCert = c }
func (m *connMgrImpl) ZoneCAPool() *x509.CertPool     { return m.zoneCAPool }
func (m *connMgrImpl) SetZoneCAPool(p *x509.CertPool) { m.zoneCAPool = p }

// ---------------------------------------------------------------------------
// Connection lifecycle
// ---------------------------------------------------------------------------

// EnsureConnected checks if already connected; if not, delegates to connectFn.
func (m *connMgrImpl) EnsureConnected(ctx context.Context, state *engine.ExecutionState) error {
	if m.pool.Main() != nil && m.pool.Main().isConnected() {
		return nil
	}
	return m.deps.connectFn(ctx, state)
}

// DisconnectConnection closes the TCP socket but preserves crypto material.
// Sends ControlClose before disconnecting. Clears PASE state.
func (m *connMgrImpl) DisconnectConnection() {
	conn := m.pool.Main()
	if conn == nil || !conn.isConnected() {
		return
	}

	// Best-effort ControlClose.
	if conn.framer != nil {
		data, err := wire.EncodeControlMessage(&wire.ControlMessage{Type: wire.ControlClose})
		if err == nil {
			_ = conn.framer.WriteFrame(data)
		}
	}
	_ = conn.Close()
	conn.clearConnectionRefs()
	m.debugf("disconnectConnection: closed TCP (crypto preserved)")
	m.paseState = nil
}

// EnsureDisconnected fully disconnects and clears all crypto + suite state.
func (m *connMgrImpl) EnsureDisconnected() {
	m.DisconnectConnection()
	m.ClearWorkingCrypto()
	m.suite.Clear()
}

// TransitionToOperational closes the commissioning connection and
// establishes a new operational TLS connection using zone crypto.
func (m *connMgrImpl) TransitionToOperational(state *engine.ExecutionState) error {
	if m.paseState == nil || len(m.paseState.sessionKey) == 0 {
		return fmt.Errorf("cannot transition: no PASE session key")
	}

	zoneID := deriveZoneIDFromSecret(m.paseState.sessionKey)
	m.debugf("transitionToOperational: zone ID derived: %s", zoneID)

	// Close commissioning connection.
	if m.pool.Main() != nil && m.pool.Main().isConnected() {
		_ = m.pool.Main().Close()
		m.pool.Main().clearConnectionRefs()
		m.pool.SetMain(nil)
	}

	// Establish new operational TLS connection.
	m.debugf("transitionToOperational: reconnecting with operational TLS")
	target := m.config.Target
	ctx := context.Background()
	crypto := m.WorkingCrypto()
	tlsConn, dialErr := dialWithRetry(ctx, 3, func() (*tls.Conn, error) {
		return m.dialer.DialOperational(ctx, target, crypto)
	})
	if dialErr != nil {
		return fmt.Errorf("operational reconnection failed: %w", dialErr)
	}

	newConn := &Connection{
		tlsConn: tlsConn,
		framer:  transport.NewFramer(tlsConn),
		state:   ConnOperational,
	}
	m.pool.SetMain(newConn)
	state.Set(StateConnection, newConn)

	// Verify device is ready.
	if err := m.WaitForOperationalReady(2 * time.Second); err != nil {
		m.debugf("transitionToOperational: readiness check failed: %v", err)
		return fmt.Errorf("operational readiness failed: %w", err)
	}

	state.Set(StateOperationalConnEstablished, time.Now())
	m.pool.TrackZone(zoneID, m.pool.Main(), zoneID)
	m.debugf("transitionToOperational: operational connection established for zone %s", zoneID)
	return nil
}

// ReconnectToZone re-establishes operational TLS using stored suite zone crypto.
func (m *connMgrImpl) ReconnectToZone(state *engine.ExecutionState) error {
	if m.suite.ZoneID() == "" {
		return fmt.Errorf("no suite zone to reconnect to")
	}

	// Restore crypto from suite if local state is empty.
	if m.zoneCAPool == nil || m.controllerCert == nil {
		saved := m.suite.Crypto()
		m.zoneCA = saved.ZoneCA
		m.controllerCert = saved.ControllerCert
		m.zoneCAPool = saved.ZoneCAPool
		m.issuedDeviceCert = saved.IssuedDeviceCert
	}

	m.debugf("reconnectToZone: reconnecting to zone %s", m.suite.ZoneID())
	target := m.config.Target
	ctx := context.Background()
	crypto := m.WorkingCrypto()
	tlsConn, dialErr := dialWithRetry(ctx, 3, func() (*tls.Conn, error) {
		return m.dialer.DialOperational(ctx, target, crypto)
	})
	if dialErr != nil {
		return fmt.Errorf("reconnectToZone failed: %w", dialErr)
	}

	newConn := &Connection{
		tlsConn: tlsConn,
		framer:  transport.NewFramer(tlsConn),
		state:   ConnOperational,
	}
	m.pool.SetMain(newConn)
	state.Set(StateConnection, newConn)

	if err := m.WaitForOperationalReady(2 * time.Second); err != nil {
		m.debugf("reconnectToZone: readiness check failed: %v", err)
		m.pool.Main().transitionTo(ConnDisconnected)
		return fmt.Errorf("reconnectToZone readiness failed: %w", err)
	}

	m.pool.TrackZone(m.suite.ConnKey(), m.pool.Main(), m.suite.ZoneID())
	m.debugf("reconnectToZone: reconnected to zone %s", m.suite.ZoneID())
	return nil
}

// ---------------------------------------------------------------------------
// Health checks (moved from readiness.go)
// ---------------------------------------------------------------------------

// WaitForCommissioningMode polls mDNS until the device advertises the
// commissionable service, indicating it has re-entered commissioning mode.
func (m *connMgrImpl) WaitForCommissioningMode(ctx context.Context, timeout time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)
	browseMs := 300
	for time.Now().Before(deadline) {
		browseCtx, cancel := context.WithTimeout(ctx, time.Duration(browseMs)*time.Millisecond)
		count, err := m.deps.browseFn(browseCtx, "commissionable", nil, browseMs)
		cancel()
		if err == nil && count > 0 {
			m.debugf("waitForCommissioningMode: device found after %v", time.Since(start))
			return nil
		}
		browseMs = min(browseMs*2, 1000)
	}
	return fmt.Errorf("timeout waiting for commissioning mode after %v", timeout)
}

// ProbeSessionHealth sends a lightweight Read request to verify the connection
// is alive and the device is responding.
func (m *connMgrImpl) ProbeSessionHealth() error {
	if m.pool.Main() == nil || !m.pool.Main().isConnected() || m.pool.Main().framer == nil {
		return fmt.Errorf("no active connection")
	}

	req := &wire.Request{
		MessageID:  m.deps.nextMsgIDFn(),
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  0x01, // FeatureDeviceInfo
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode health probe: %w", err)
	}

	if err := m.pool.Main().framer.WriteFrame(data); err != nil {
		m.pool.Main().transitionTo(ConnDisconnected)
		return fmt.Errorf("send health probe: %w", err)
	}

	if m.pool.Main().tlsConn != nil {
		_ = m.pool.Main().tlsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		defer func() {
			if m.pool.Main().tlsConn != nil {
				_ = m.pool.Main().tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	}

	drained := 0
	for range 20 {
		respData, err := m.pool.Main().framer.ReadFrame()
		if err != nil {
			m.pool.Main().transitionTo(ConnDisconnected)
			return fmt.Errorf("read health probe response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode health probe response: %w", err)
		}
		if resp.MessageID == 0 {
			drained++
			continue
		}
		if resp.MessageID != req.MessageID {
			m.debugf("probeSessionHealth: discarding orphaned response (got msgID=%d, want %d)", resp.MessageID, req.MessageID)
			drained++
			continue
		}
		if drained > 0 {
			m.debugf("probeSessionHealth: discarded %d stale frames", drained)
		}
		m.debugf("probeSessionHealth: device responded (status=%d)", resp.Status)
		return nil
	}
	return fmt.Errorf("health probe: too many interleaved frames (%d discarded)", drained)
}

// WaitForOperationalReady subscribes to DeviceInfo and waits for the
// priming report, confirming the device's operational handler is ready.
func (m *connMgrImpl) WaitForOperationalReady(timeout time.Duration) error {
	if m.pool.Main() == nil || !m.pool.Main().isConnected() {
		return fmt.Errorf("not connected")
	}

	req := &wire.Request{
		MessageID:  m.deps.nextMsgIDFn(),
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0x01, // FeatureDeviceInfo
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode readiness probe: %w", err)
	}

	if err := m.pool.Main().framer.WriteFrame(data); err != nil {
		m.pool.Main().transitionTo(ConnDisconnected)
		return fmt.Errorf("send readiness probe: %w", err)
	}

	if m.pool.Main().tlsConn != nil {
		_ = m.pool.Main().tlsConn.SetReadDeadline(time.Now().Add(timeout))
		defer func() {
			if m.pool.Main().tlsConn != nil {
				_ = m.pool.Main().tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	}

	for range 10 {
		respData, err := m.pool.Main().framer.ReadFrame()
		if err != nil {
			m.pool.Main().transitionTo(ConnDisconnected)
			return fmt.Errorf("read readiness response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode readiness response: %w", err)
		}
		if resp.MessageID == 0 {
			m.pool.AppendNotification(respData)
			continue
		}
		if resp.MessageID != req.MessageID {
			m.debugf("waitForOperationalReady: discarding orphaned response (got msgID=%d, want %d)", resp.MessageID, req.MessageID)
			continue
		}
		m.debugf("waitForOperationalReady: device responded (status=%d)", resp.Status)
		return nil
	}
	return fmt.Errorf("readiness probe: too many interleaved frames")
}

// EnsureCommissioned is a stub that delegates to the Runner's ensureCommissioned.
// The full commissioning orchestration (PASE handshake, retries, cooldowns)
// remains on Runner because it tightly couples to handleCommission.
// This satisfies the ConnectionManager interface so the coordinator can
// call it uniformly.
func (m *connMgrImpl) EnsureCommissioned(ctx context.Context, state *engine.ExecutionState) error {
	// Delegate to Runner via commissionFn callback.
	// The callback wraps the full ensureCommissioned flow.
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{
			ParamTransitionToOperational: true,
			ParamFromPrecondition:        true,
		},
	}
	if m.config.SetupCode != "" {
		step.Params["setup_code"] = m.config.SetupCode
	}

	// Ensure connected first.
	if err := m.EnsureConnected(ctx, state); err != nil {
		return err
	}

	// Delegate to Runner's commission handler.
	_, err := m.deps.commissionFn(ctx, step, state)
	return err
}

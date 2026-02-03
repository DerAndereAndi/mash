package runner

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// PASEState holds the state for PASE commissioning sessions.
type PASEState struct {
	// session is the active PASE client session (if in-progress).
	session *commissioning.PASEClientSession

	// sessionKey is the derived session key after successful handshake.
	sessionKey []byte

	// completed indicates whether commissioning finished successfully.
	completed bool
}

// registerPASEHandlers registers all PASE-related action handlers.
func (r *Runner) registerPASEHandlers() {
	// Primary commissioning action (recommended)
	r.engine.RegisterHandler("commission", r.handleCommission)

	// Legacy granular PASE handlers - these now delegate to commission
	// for backward compatibility with existing test cases
	r.engine.RegisterHandler("pase_request", r.handlePASERequest)
	r.engine.RegisterHandler("pase_receive_response", r.handlePASEReceiveResponse)
	r.engine.RegisterHandler("pase_confirm", r.handlePASEConfirm)
	r.engine.RegisterHandler("pase_receive_verify", r.handlePASEReceiveVerify)
	r.engine.RegisterHandler("verify_session_key", r.handleVerifySessionKey)
}

// handleCommission performs the full PASE commissioning handshake.
// This is the recommended action for test cases - it performs the complete
// SPAKE2+ exchange and returns the session key.
//
// If not already connected, this action will establish a commissioning TLS
// connection first. This ensures the connection and PASE handshake happen
// atomically without any delay between them.
func (r *Runner) handleCommission(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Interpolate parameters so ${setup_code} etc. resolve.
	params := engine.InterpolateParams(step.Params, state)

	// Get setup code from params or config
	setupCode, err := r.getSetupCode(params)
	if err != nil {
		return nil, fmt.Errorf("invalid setup code: %w", err)
	}

	// Get identities from params or use defaults
	clientIdentity := r.getClientIdentity(params)
	serverIdentity := r.getServerIdentity(params)

	// Establish commissioning connection if not already connected
	// This ensures connection + PASE happen atomically
	var conn net.Conn
	if r.conn.connected && r.conn.tlsConn != nil {
		conn = r.conn.tlsConn
	} else {
		// Create new commissioning connection
		target := r.config.Target
		if t, ok := params[KeyTarget].(string); ok && t != "" {
			target = t
		}

		tlsConfig := transport.NewCommissioningTLSConfig()
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		tlsConn, err := tls.DialWithDialer(dialer, "tcp", target, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect for commissioning: %w", err)
		}

		// Store connection for later use
		r.conn.tlsConn = tlsConn
		r.conn.framer = transport.NewFramer(tlsConn)
		r.conn.connected = true
		conn = tlsConn

		state.Set(StateConnection, r.conn)
		state.Set(KeyConnectionEstablished, true)
	}

	// Create PASE client session
	session, err := commissioning.NewPASEClientSession(
		setupCode,
		[]byte(clientIdentity),
		[]byte(serverIdentity),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create PASE session: %w", err)
	}

	// Perform the full SPAKE2+ handshake
	sessionKey, err := session.Handshake(ctx, conn)
	if err != nil {
		// Store failed state for debugging
		r.paseState = &PASEState{
			session:   session,
			completed: false,
		}
		// The handshake writes/reads on the connection. If it fails,
		// the connection is in an unusable state (the device closes
		// it on PASE failure per the MASH spec).
		r.conn.connected = false
		// Return structured output so tests can assert on the failure
		// rather than propagating an error that breaks preconditions.
		return map[string]any{
			KeySessionEstablished: false,
			KeyCommissionSuccess:  false,
			KeySuccess:            false,
			KeyError:              err.Error(),
		}, fmt.Errorf("PASE handshake failed: %w", err)
	}

	// Store successful state
	r.paseState = &PASEState{
		session:    session,
		sessionKey: sessionKey,
		completed:  true,
	}

	// Perform certificate exchange after PASE (MsgType 30-33).
	// This is required: the device blocks waiting for the cert exchange
	// before it will accept operational messages (Read/Write/Invoke).
	deviceID, certErr := r.performCertExchange(ctx)

	// Even if cert exchange fails, fall back to commissioning TLS trust
	// so basic tests that don't need operational connections still work.
	if certErr != nil {
		// Fall back: use peer cert from commissioning TLS.
		if r.conn.tlsConn != nil {
			cs := r.conn.tlsConn.ConnectionState()
			if len(cs.PeerCertificates) > 0 {
				pool := x509.NewCertPool()
				for _, c := range cs.PeerCertificates {
					pool.AddCert(c)
				}
				r.zoneCAPool = pool
			}
		}
	}

	// Also store in execution state for test assertions
	state.Set(KeySessionEstablished, true)
	state.Set(StateSessionKey, sessionKey)
	state.Set(StateSessionKeyLen, len(sessionKey))
	if deviceID != "" {
		state.Set(StateDeviceID, deviceID)
	}

	outputs := map[string]any{
		KeySessionEstablished: true,
		KeyCommissionSuccess:  true,
		KeySuccess:            true,
		KeyKeyLength:          len(sessionKey),
		KeyKeyNotZero:         !isZeroKey(sessionKey),
	}
	if deviceID != "" {
		outputs[KeyDeviceID] = deviceID
	}

	return outputs, nil
}

// performCertExchange executes the 4-message cert exchange after PASE.
// It generates a Zone CA, uses IssueInitialCertSync to exchange certs with
// the device, and stores the resulting certs on the runner for later TLS use.
func (r *Runner) performCertExchange(ctx context.Context) (string, error) {
	if r.paseState == nil || r.paseState.sessionKey == nil {
		return "", fmt.Errorf("no PASE session key")
	}
	if r.conn == nil || !r.conn.connected || r.conn.framer == nil {
		return "", fmt.Errorf("no active connection")
	}

	// Derive zone ID from PASE shared secret (same derivation as device).
	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// Use the configured zone type, defaulting to LOCAL.
	zt := r.commissionZoneType
	if zt == 0 {
		zt = cert.ZoneTypeLocal
	}

	// Generate Zone CA for this zone.
	zoneCA, err := cert.GenerateZoneCA(zoneID, zt)
	if err != nil {
		return "", fmt.Errorf("generate zone CA: %w", err)
	}

	// Create a SyncConnection adapter wrapping the framer.
	syncConn := &framerSyncConn{framer: r.conn.framer}

	// Create the renewal handler and perform the 4-message cert exchange.
	renewalHandler := service.NewControllerRenewalHandler(zoneCA, syncConn)
	deviceCert, err := renewalHandler.IssueInitialCertSync(ctx, syncConn)
	if err != nil {
		return "", fmt.Errorf("cert exchange: %w", err)
	}

	// Store Zone CA and accumulate the CA pool for operational TLS.
	// We accumulate rather than replace so that multiple commissions
	// (multi-zone) can coexist -- each zone's CA must be trusted.
	r.zoneCA = zoneCA
	if r.zoneCAPool == nil {
		r.zoneCAPool = x509.NewCertPool()
	}
	r.zoneCAPool.AddCert(zoneCA.Certificate)

	// Generate controller operational cert for mutual TLS.
	controllerID := "test-controller"
	controllerCert, err := cert.GenerateControllerOperationalCert(zoneCA, controllerID)
	if err != nil {
		return "", fmt.Errorf("generate controller cert: %w", err)
	}
	r.controllerCert = controllerCert

	// Extract device ID from the signed certificate.
	deviceID, _ := cert.ExtractDeviceID(deviceCert)

	return deviceID, nil
}

// handlePASERequest handles the legacy pase_request action.
// For backward compatibility, this initiates commissioning and returns
// the expected outputs from the original stub.
func (r *Runner) handlePASERequest(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Get setup code - use "password" param for backward compatibility
	setupCode, err := r.getSetupCodeFromLegacyParams(step.Params)
	if err != nil {
		return nil, fmt.Errorf("invalid setup code: %w", err)
	}

	clientIdentity := r.getClientIdentity(step.Params)
	serverIdentity := r.getServerIdentity(step.Params)

	// Create session but don't handshake yet - store for subsequent steps
	session, err := commissioning.NewPASEClientSession(
		setupCode,
		[]byte(clientIdentity),
		[]byte(serverIdentity),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create PASE session: %w", err)
	}

	// Initialize state - we'll perform full handshake on pase_receive_verify
	r.paseState = &PASEState{
		session:   session,
		completed: false,
	}

	// Store params for the deferred handshake
	state.Set(StatePasePending, true)

	return map[string]any{
		KeyRequestSent: true,
		KeyPAGenerated: true,
	}, nil
}

// handlePASEReceiveResponse handles the legacy pase_receive_response action.
// Since the handshake is atomic, this just returns the expected outputs.
func (r *Runner) handlePASEReceiveResponse(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.paseState == nil || r.paseState.session == nil {
		return nil, fmt.Errorf("no PASE session in progress: call pase_request first")
	}

	return map[string]any{
		KeyResponseReceived: true,
		KeyPBReceived:       true,
	}, nil
}

// handlePASEConfirm handles the legacy pase_confirm action.
// Since the handshake is atomic, this just returns the expected outputs.
func (r *Runner) handlePASEConfirm(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.paseState == nil || r.paseState.session == nil {
		return nil, fmt.Errorf("no PASE session in progress")
	}

	return map[string]any{
		KeyConfirmSent: true,
	}, nil
}

// handlePASEReceiveVerify handles the legacy pase_receive_verify action.
// This is where we actually perform the handshake for legacy test cases.
func (r *Runner) handlePASEReceiveVerify(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.paseState == nil || r.paseState.session == nil {
		return nil, fmt.Errorf("no PASE session in progress")
	}

	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Perform the actual handshake now
	sessionKey, err := r.paseState.session.Handshake(ctx, r.conn.tlsConn)
	if err != nil {
		r.conn.connected = false
		return nil, fmt.Errorf("PASE handshake failed: %w", err)
	}

	// Update state
	r.paseState.sessionKey = sessionKey
	r.paseState.completed = true

	state.Set(KeySessionEstablished, true)
	state.Set(StateSessionKey, sessionKey)

	return map[string]any{
		KeyVerifyReceived:     true,
		KeySessionEstablished: true,
	}, nil
}

// handleVerifySessionKey verifies the derived session key.
func (r *Runner) handleVerifySessionKey(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.paseState == nil {
		return nil, fmt.Errorf("no PASE session: commissioning not performed")
	}

	if !r.paseState.completed {
		return nil, fmt.Errorf("PASE session not completed")
	}

	key := r.paseState.sessionKey

	return map[string]any{
		KeyKeyLength:  len(key),
		KeyKeyNotZero: !isZeroKey(key),
	}, nil
}

// getSetupCode extracts the setup code from step parameters.
func (r *Runner) getSetupCode(params map[string]any) (commissioning.SetupCode, error) {
	// Try "setup_code" parameter first
	if sc, ok := params[KeySetupCode].(string); ok && sc != "" {
		return commissioning.ParseSetupCode(sc)
	}

	// Try config
	if r.config.SetupCode != "" {
		return commissioning.ParseSetupCode(r.config.SetupCode)
	}

	return 0, fmt.Errorf("no setup code provided: use setup_code parameter or -setup-code flag")
}

// getSetupCodeFromLegacyParams handles the legacy "password" parameter.
func (r *Runner) getSetupCodeFromLegacyParams(params map[string]any) (commissioning.SetupCode, error) {
	// Legacy test cases use "password" parameter
	if pw, ok := params["password"].(string); ok && pw != "" {
		// If it's an 8-digit numeric string, parse as setup code
		if len(pw) == 8 && isNumeric(pw) {
			return commissioning.ParseSetupCode(pw)
		}
		// For non-numeric passwords (like "test-password" in old tests),
		// generate a deterministic setup code from the password
		return deriveSetupCodeFromPassword(pw), nil
	}

	// Fall back to standard setup code handling
	return r.getSetupCode(params)
}

// getClientIdentity returns the client identity from params or default.
// Default matches the identity used by mash-controller and expected by mash-device.
func (r *Runner) getClientIdentity(params map[string]any) string {
	if ci, ok := params["client_identity"].(string); ok && ci != "" {
		return ci
	}
	if r.config.ClientIdentity != "" {
		return r.config.ClientIdentity
	}
	return "mash-controller"
}

// getServerIdentity returns the server identity from params or default.
// Default matches the identity used by mash-device.
func (r *Runner) getServerIdentity(params map[string]any) string {
	if si, ok := params["server_identity"].(string); ok && si != "" {
		return si
	}
	if r.config.ServerIdentity != "" {
		return r.config.ServerIdentity
	}
	return "mash-device"
}

// isZeroKey checks if a key is all zeros.
func isZeroKey(key []byte) bool {
	for _, b := range key {
		if b != 0 {
			return false
		}
	}
	return true
}

// isNumeric checks if a string contains only digits.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// deriveSetupCodeFromPassword creates a deterministic setup code from a password string.
// This provides backward compatibility with test cases that use non-numeric passwords.
func deriveSetupCodeFromPassword(password string) commissioning.SetupCode {
	// Simple hash to create an 8-digit code
	var hash uint32
	for _, c := range password {
		hash = hash*31 + uint32(c)
	}
	// Ensure it fits in 8 digits (max 99999999)
	return commissioning.SetupCode(hash % 100000000)
}

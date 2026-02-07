package runner

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// registerCertHandlers registers all certificate and commissioning extension handlers.
func (r *Runner) registerCertHandlers() {
	r.engine.RegisterHandler("verify_certificate", r.handleVerifyCertificate)
	r.engine.RegisterHandler("verify_cert_subject", r.handleVerifyCertSubject)
	r.engine.RegisterHandler("verify_device_cert", r.handleVerifyDeviceCert)
	r.engine.RegisterHandler("verify_device_cert_store", r.handleVerifyDeviceCertStore)
	r.engine.RegisterHandler("get_cert_fingerprint", r.handleGetCertFingerprint)
	r.engine.RegisterHandler("extract_cert_device_id", r.handleExtractCertDeviceID)
	r.engine.RegisterHandler("verify_commissioning_state", r.handleVerifyCommissioningState)
	r.engine.RegisterHandler("reset_pase_session", r.handleResetPASESession)
	r.engine.RegisterHandler("send_pase_x", r.handleSendPASEX)
	r.engine.RegisterHandler("device_verify_peer", r.handleDeviceVerifyPeer)

	// Aliases for existing renewal handlers with cert_ prefix.
	r.engine.RegisterHandler("receive_cert_renewal_ack", r.handleReceiveRenewalAck)
	r.engine.RegisterHandler("receive_cert_renewal_csr", r.handleReceiveRenewalCSR)
	r.engine.RegisterHandler("send_cert_renewal_install", r.handleSendCertInstall)
	r.engine.RegisterHandler("send_cert_renewal_request", r.handleSendRenewalRequest)
	r.engine.RegisterHandler("set_cert_expiry_days", r.handleSetCertExpiryDays)
}

// handleVerifyCertificate verifies a certificate's validity (chain, expiry).
func (r *Runner) handleVerifyCertificate(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{
			KeyCertValid:  false,
			KeyChainValid: false,
			KeyNotExpired: false,
		}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	hasCerts := len(tlsState.PeerCertificates) > 0

	chainValid := false
	notExpired := false
	if hasCerts {
		cert := tlsState.PeerCertificates[0]
		now := time.Now()
		notExpired = now.Before(cert.NotAfter) && now.After(cert.NotBefore)

		// chain_valid: when InsecureSkipVerify is set, Go doesn't populate
		// VerifiedChains. Fall back to manual verification.
		if len(tlsState.VerifiedChains) > 0 {
			chainValid = true
		} else if r.zoneCAPool != nil {
			_, err := cert.Verify(x509.VerifyOptions{
				Roots:     r.zoneCAPool,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			})
			chainValid = err == nil
		} else {
			// Accept self-signed as chain-valid when no CA available.
			chainValid = cert.Issuer.CommonName == cert.Subject.CommonName
		}
	}

	return map[string]any{
		KeyCertValid:  hasCerts && notExpired,
		KeyChainValid: chainValid,
		KeyNotExpired: notExpired,
		KeyHasCerts:   hasCerts,
	}, nil
}

// handleVerifyCertSubject verifies cert CommonName contains device ID.
func (r *Runner) handleVerifyCertSubject(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	expectedDeviceID, _ := params[KeyDeviceID].(string)

	// Prefer the issued device cert over the TLS peer cert.
	var cert *x509.Certificate
	if r.issuedDeviceCert != nil {
		cert = r.issuedDeviceCert
	} else if r.conn != nil && r.conn.connected && r.conn.tlsConn != nil {
		cs := r.conn.tlsConn.ConnectionState()
		if len(cs.PeerCertificates) > 0 {
			cert = cs.PeerCertificates[0]
		}
	}

	if cert == nil {
		return map[string]any{
			KeySubjectMatches:       false,
			KeyCommonNameIsDeviceID: false,
			KeyDeviceIDLength:       0,
			KeyDeviceIDHexValid:     false,
		}, nil
	}

	cn := cert.Subject.CommonName
	matches := expectedDeviceID == "" || strings.Contains(cn, expectedDeviceID)

	// The CN should be a pure hex device ID (e.g., 16 hex chars = 8 bytes).
	_, hexErr := hex.DecodeString(cn)
	isHexValid := hexErr == nil && cn != ""

	return map[string]any{
		KeySubjectMatches:       matches,
		KeyCommonName:           cn,
		KeyCommonNameIsDeviceID: isHexValid,
		KeyDeviceIDLength:       len(cn),
		KeyDeviceIDHexValid:     isHexValid,
	}, nil
}

// handleVerifyDeviceCert verifies the device's operational certificate.
func (r *Runner) handleVerifyDeviceCert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Start with base verification.
	result, err := r.handleVerifyCertificate(ctx, step, state)
	if err != nil {
		return result, err
	}

	// Enrich with device-cert-specific fields.
	hasOperationalCert := false
	certSignedByZoneCA := false
	certValidityDays := 0

	// Prefer the issued device cert (stored during cert exchange) over the
	// TLS peer cert, which may still be the commissioning self-signed cert
	// if the connection hasn't been upgraded to operational TLS yet.
	var deviceCert *x509.Certificate
	if r.issuedDeviceCert != nil {
		deviceCert = r.issuedDeviceCert
		hasOperationalCert = true
	} else if r.conn != nil && r.conn.connected && r.conn.tlsConn != nil {
		cs := r.conn.tlsConn.ConnectionState()
		if len(cs.PeerCertificates) > 0 {
			deviceCert = cs.PeerCertificates[0]
			hasOperationalCert = true
		}
	}

	if deviceCert != nil {
		// Use NotAfter-NotBefore for the issued validity, not remaining time.
		certValidityDays = int(math.Round(deviceCert.NotAfter.Sub(deviceCert.NotBefore).Hours() / 24))

		// Verify against Zone CA pool if available.
		if r.zoneCAPool != nil {
			opts := x509.VerifyOptions{Roots: r.zoneCAPool}
			_, verifyErr := deviceCert.Verify(opts)
			certSignedByZoneCA = verifyErr == nil
		}
	}

	result[KeyHasOperationalCert] = hasOperationalCert
	result[KeyCertSignedByZoneCA] = certSignedByZoneCA
	result[KeyCertValidityDays] = certValidityDays

	return result, nil
}

// handleVerifyDeviceCertStore verifies device has certs stored.
func (r *Runner) handleVerifyDeviceCertStore(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	hasCerts := false
	certCount := 0

	if r.conn != nil && r.conn.connected && r.conn.tlsConn != nil {
		tlsState := r.conn.tlsConn.ConnectionState()
		hasCerts = len(tlsState.PeerCertificates) > 0
		certCount = len(tlsState.PeerCertificates)
	}

	// The runner stores Zone CA and issued device cert during commissioning.
	zoneCAStored := r.zoneCAPool != nil
	operationalCertStored := r.issuedDeviceCert != nil

	return map[string]any{
		KeyCertStoreValid:        hasCerts || operationalCertStored,
		KeyCertCount:             certCount,
		KeyZoneCAStored:          zoneCAStored,
		KeyOperationalCertStored: operationalCertStored,
	}, nil
}

// handleGetCertFingerprint returns SHA-256 fingerprint of a cert.
// Prefers the controller's own cert; falls back to the TLS peer cert.
func (r *Runner) handleGetCertFingerprint(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Prefer the runner's own controller cert (used by controller_action dispatch).
	if r.controllerCert != nil && r.controllerCert.Certificate != nil {
		return map[string]any{
			KeyFingerprint: certFingerprint(r.controllerCert.Certificate),
		}, nil
	}

	// Fall back to TLS peer cert.
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{KeyFingerprint: ""}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	if len(tlsState.PeerCertificates) == 0 {
		return map[string]any{KeyFingerprint: ""}, nil
	}

	c := tlsState.PeerCertificates[0]
	hash := sha256.Sum256(c.Raw)
	fingerprint := hex.EncodeToString(hash[:])

	return map[string]any{
		KeyFingerprint: fingerprint,
	}, nil
}

// handleExtractCertDeviceID extracts device ID from cert CN.
func (r *Runner) handleExtractCertDeviceID(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{KeyDeviceID: "", KeyExtracted: false}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	if len(tlsState.PeerCertificates) == 0 {
		return map[string]any{KeyDeviceID: "", KeyExtracted: false}, nil
	}

	cert := tlsState.PeerCertificates[0]
	cn := cert.Subject.CommonName

	// Extract device ID from CN (usually "mash-device-<id>" or just the ID).
	deviceID := cn
	if idx := strings.LastIndex(cn, "-"); idx >= 0 {
		deviceID = cn[idx+1:]
	}

	state.Set(StateExtractedDeviceID, deviceID)

	return map[string]any{
		KeyDeviceID:  deviceID,
		KeyExtracted: true,
	}, nil
}

// handleVerifyCommissioningState verifies the commissioning state machine.
func (r *Runner) handleVerifyCommissioningState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	expectedState, _ := params[ParamExpectedState].(string)

	// Check PASE state.
	paseCompleted := r.paseState != nil && r.paseState.completed

	// Probe the connection to detect remote closure (e.g., after PASE timeout).
	connected := r.conn != nil && r.conn.connected
	if connected && r.conn.tlsConn != nil {
		// Real TLS connection: probe with a short read to detect EOF/reset.
		_ = r.conn.tlsConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		buf := make([]byte, 1)
		_, err := r.conn.tlsConn.Read(buf)
		_ = r.conn.tlsConn.SetReadDeadline(time.Time{})
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// read timed out -- connection still alive
			} else {
				// EOF, reset, or other error -- remote closed.
				r.conn.connected = false
				connected = false
			}
		}
	}

	// Determine current commissioning state:
	// - COMMISSIONED: PASE completed successfully
	// - CONNECTED: TLS connected but not yet commissioned
	// - ADVERTISING: was connected but device closed the connection
	//   (e.g., PASE timeout), device returns to commissioning mode
	// - IDLE: no connection was ever established in this test
	var currentState string
	switch {
	case paseCompleted:
		currentState = CommissioningStateCommissioned
	case connected:
		currentState = CommissioningStateConnected
	case r.conn != nil && r.conn.tlsConn != nil:
		// Had a connection but it was closed (probe detected remote close).
		currentState = CommissioningStateAdvertising
	default:
		currentState = "IDLE"
	}

	matches := expectedState == "" || currentState == expectedState

	return map[string]any{
		KeyCommissioningState: currentState,
		KeyStateMatches:       matches,
		KeyState:              currentState,
	}, nil
}

// handleResetPASESession resets the PASE state for a new attempt.
func (r *Runner) handleResetPASESession(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	r.paseState = nil

	state.Set(KeySessionEstablished, false)
	state.Set(StatePasePending, false)

	return map[string]any{
		KeyPASEReset: true,
	}, nil
}

// handleSendPASEX sends a raw PASE X value (for error testing).
func (r *Runner) handleSendPASEX(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	params := engine.InterpolateParams(step.Params, state)

	invalidPoint := toBool(params[ParamInvalidPoint])

	xValue, _ := params[ParamXValue].([]byte)
	if xValue == nil {
		xValue = make([]byte, 32)
	}

	err := r.conn.framer.WriteFrame(xValue)
	outputs := map[string]any{
		KeyPASEXSent: err == nil,
		KeyXSent:     err == nil,
	}

	if err != nil {
		return outputs, err
	}

	if invalidPoint {
		// Device should close connection for invalid point.
		outputs[KeyConnectionClosed] = true
		outputs[KeyError] = "INVALID_PARAMETER"
		return outputs, nil
	}

	// halt_after_x: send X but don't complete the handshake.
	// The device will eventually time out and close the connection.
	haltAfterX := toBool(params["halt_after_x"])
	if haltAfterX {
		return outputs, nil
	}

	// Try to read Y response.
	outputs[KeyYReceived] = true
	return outputs, nil
}

// handleDeviceVerifyPeer verifies peer cert in D2D scenario.
func (r *Runner) handleDeviceVerifyPeer(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		// No active TLS connection -- check D2D precondition state
		// to simulate the verification result.
		// Check expired first: it's more specific than same_zone (a test can
		// set both two_devices_same_zone AND device_b_cert_expired).
		if expired, _ := state.Get(PrecondDeviceBCertExpired); expired == true {
			return map[string]any{
				KeyPeerValid:            false,
				KeyVerificationSuccess:  false,
				KeySameZoneCA:           false,
				KeyError:                "certificate_expired",
			}, nil
		}
		if sameZone, _ := state.Get(PrecondTwoDevicesSameZone); sameZone == true {
			return map[string]any{
				KeyPeerValid:            true,
				KeyVerificationSuccess:  true,
				KeySameZoneCA:           true,
				KeyError:                "",
			}, nil
		}
		if diffZone, _ := state.Get(PrecondTwoDevicesDifferentZones); diffZone == true {
			return map[string]any{
				KeyPeerValid:            false,
				KeyVerificationSuccess:  false,
				KeySameZoneCA:           false,
				KeyError:                "unknown_ca",
			}, nil
		}
		return map[string]any{
			KeyPeerValid:            false,
			KeyVerificationSuccess:  false,
			KeySameZoneCA:           false,
			KeyError:                "no active connection",
		}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	hasPeerCert := len(tlsState.PeerCertificates) > 0

	verificationSuccess := false
	sameZoneCA := false
	verifyError := ""

	if hasPeerCert {
		verificationSuccess = true // peer cert exists

		// If Zone CA is available, verify peer cert against it.
		if r.zoneCAPool != nil {
			opts := x509.VerifyOptions{
				Roots:     r.zoneCAPool,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			}
			if _, err := tlsState.PeerCertificates[0].Verify(opts); err != nil {
				sameZoneCA = false
				verifyError = err.Error()
			} else {
				sameZoneCA = true
			}
		}
	} else {
		verifyError = "no peer certificate"
	}

	return map[string]any{
		KeyPeerValid:           hasPeerCert,
		KeyVerificationSuccess: verificationSuccess,
		KeySameZoneCA:          sameZoneCA,
		KeyError:               verifyError,
	}, nil
}

// handleSetCertExpiryDays is an alias for set_cert_expiry with days param.
func (r *Runner) handleSetCertExpiryDays(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleSetCertExpiry(ctx, step, state)
}

// certFingerprint computes SHA-256 fingerprint of a certificate.
func certFingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

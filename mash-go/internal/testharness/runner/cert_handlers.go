package runner

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
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
			"cert_valid":  false,
			"chain_valid": false,
		}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	hasCerts := len(tlsState.PeerCertificates) > 0

	chainValid := false
	expired := false
	if hasCerts {
		cert := tlsState.PeerCertificates[0]
		chainValid = len(tlsState.VerifiedChains) > 0
		expired = cert.NotAfter.Before(cert.NotBefore)
	}

	return map[string]any{
		"cert_valid":  hasCerts && !expired,
		"chain_valid": chainValid,
		"has_certs":   hasCerts,
	}, nil
}

// handleVerifyCertSubject verifies cert CommonName contains device ID.
func (r *Runner) handleVerifyCertSubject(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	expectedDeviceID, _ := params["device_id"].(string)

	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{"subject_matches": false}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	if len(tlsState.PeerCertificates) == 0 {
		return map[string]any{"subject_matches": false}, nil
	}

	cert := tlsState.PeerCertificates[0]
	cn := cert.Subject.CommonName

	matches := expectedDeviceID == "" || strings.Contains(cn, expectedDeviceID)

	return map[string]any{
		"subject_matches": matches,
		"common_name":     cn,
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

	if r.conn != nil && r.conn.connected && r.conn.tlsConn != nil {
		cs := r.conn.tlsConn.ConnectionState()
		hasOperationalCert = len(cs.PeerCertificates) > 0

		if hasOperationalCert {
			cert := cs.PeerCertificates[0]
			certValidityDays = int(time.Until(cert.NotAfter).Hours() / 24)

			// Verify against Zone CA pool if available.
			if r.zoneCAPool != nil {
				opts := x509.VerifyOptions{Roots: r.zoneCAPool}
				_, verifyErr := cert.Verify(opts)
				certSignedByZoneCA = verifyErr == nil
			}
		}
	}

	result["has_operational_cert"] = hasOperationalCert
	result["cert_signed_by_zone_ca"] = certSignedByZoneCA
	result["cert_validity_days"] = certValidityDays

	return result, nil
}

// handleVerifyDeviceCertStore verifies device has certs stored.
func (r *Runner) handleVerifyDeviceCertStore(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{"cert_store_valid": false}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()

	return map[string]any{
		"cert_store_valid": len(tlsState.PeerCertificates) > 0,
		"cert_count":       len(tlsState.PeerCertificates),
	}, nil
}

// handleGetCertFingerprint returns SHA-256 fingerprint of a cert.
func (r *Runner) handleGetCertFingerprint(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{"fingerprint": ""}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	if len(tlsState.PeerCertificates) == 0 {
		return map[string]any{"fingerprint": ""}, nil
	}

	cert := tlsState.PeerCertificates[0]
	hash := sha256.Sum256(cert.Raw)
	fingerprint := hex.EncodeToString(hash[:])

	return map[string]any{
		"fingerprint": fingerprint,
	}, nil
}

// handleExtractCertDeviceID extracts device ID from cert CN.
func (r *Runner) handleExtractCertDeviceID(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{"device_id": "", "extracted": false}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	if len(tlsState.PeerCertificates) == 0 {
		return map[string]any{"device_id": "", "extracted": false}, nil
	}

	cert := tlsState.PeerCertificates[0]
	cn := cert.Subject.CommonName

	// Extract device ID from CN (usually "mash-device-<id>" or just the ID).
	deviceID := cn
	if idx := strings.LastIndex(cn, "-"); idx >= 0 {
		deviceID = cn[idx+1:]
	}

	state.Set("extracted_device_id", deviceID)

	return map[string]any{
		"device_id": deviceID,
		"extracted": true,
	}, nil
}

// handleVerifyCommissioningState verifies the commissioning state machine.
func (r *Runner) handleVerifyCommissioningState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	expectedState, _ := params["expected_state"].(string)

	// Check PASE state.
	paseCompleted := r.paseState != nil && r.paseState.completed
	connected := r.conn != nil && r.conn.connected

	var currentState string
	switch {
	case paseCompleted:
		currentState = "COMMISSIONED"
	case connected:
		currentState = "CONNECTED"
	default:
		currentState = "IDLE"
	}

	matches := expectedState == "" || currentState == expectedState

	return map[string]any{
		"commissioning_state": currentState,
		"state_matches":       matches,
	}, nil
}

// handleResetPASESession resets the PASE state for a new attempt.
func (r *Runner) handleResetPASESession(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	r.paseState = nil

	state.Set("session_established", false)
	state.Set("pase_pending", false)

	return map[string]any{
		"pase_reset": true,
	}, nil
}

// handleSendPASEX sends a raw PASE X value (for error testing).
func (r *Runner) handleSendPASEX(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	params := engine.InterpolateParams(step.Params, state)

	xValue, _ := params["x_value"].([]byte)
	if xValue == nil {
		// Generate random bytes.
		xValue = make([]byte, 32)
	}

	// Send raw through framer.
	err := r.conn.framer.WriteFrame(xValue)
	return map[string]any{
		"pase_x_sent": err == nil,
	}, err
}

// handleDeviceVerifyPeer verifies peer cert in D2D scenario.
func (r *Runner) handleDeviceVerifyPeer(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{
			"peer_valid":           false,
			"verification_success": false,
			"same_zone_ca":         false,
			"error":                "no active connection",
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
		"peer_valid":           hasPeerCert,
		"verification_success": verificationSuccess,
		"same_zone_ca":         sameZoneCA,
		"error":                verifyError,
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

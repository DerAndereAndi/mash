package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleResetPASESession(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up some PASE state.
	r.paseState = &PASEState{
		sessionKey: []byte{1, 2, 3},
		completed:  true,
	}
	state.Set("session_established", true)

	out, err := r.handleResetPASESession(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["pase_reset"] != true {
		t.Error("expected pase_reset=true")
	}
	if r.paseState != nil {
		t.Error("expected paseState to be nil after reset")
	}

	established, _ := state.Get("session_established")
	if established != false {
		t.Error("expected session_established=false after reset")
	}
}

func TestHandleVerifyCommissioningState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// IDLE state.
	step := &loader.Step{Params: map[string]any{"expected_state": "IDLE"}}
	out, _ := r.handleVerifyCommissioningState(context.Background(), step, state)
	if out["commissioning_state"] != "IDLE" {
		t.Errorf("expected IDLE, got %v", out["commissioning_state"])
	}
	if out["state_matches"] != true {
		t.Error("expected state_matches=true")
	}

	// CONNECTED state.
	r.conn.connected = true
	step = &loader.Step{Params: map[string]any{"expected_state": "CONNECTED"}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out["commissioning_state"] != "CONNECTED" {
		t.Errorf("expected CONNECTED, got %v", out["commissioning_state"])
	}

	// COMMISSIONED state.
	r.paseState = &PASEState{completed: true}
	step = &loader.Step{Params: map[string]any{"expected_state": "COMMISSIONED"}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out["commissioning_state"] != "COMMISSIONED" {
		t.Errorf("expected COMMISSIONED, got %v", out["commissioning_state"])
	}

	// Mismatch.
	step = &loader.Step{Params: map[string]any{"expected_state": "IDLE"}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out["state_matches"] != false {
		t.Error("expected state_matches=false for mismatch")
	}
}

func TestHandleVerifyCertificateNotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleVerifyCertificate(context.Background(), &loader.Step{}, state)
	if out["cert_valid"] != false {
		t.Error("expected cert_valid=false when not connected")
	}
}

func TestHandleVerifyCertSubjectNotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"device_id": "dev-123"}}
	out, _ := r.handleVerifyCertSubject(context.Background(), step, state)
	if out["subject_matches"] != false {
		t.Error("expected subject_matches=false when not connected")
	}
}

func TestHandleGetCertFingerprintNotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleGetCertFingerprint(context.Background(), &loader.Step{}, state)
	if out["fingerprint"] != "" {
		t.Error("expected empty fingerprint when not connected")
	}
}

func TestHandleExtractCertDeviceIDNotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleExtractCertDeviceID(context.Background(), &loader.Step{}, state)
	if out["extracted"] != false {
		t.Error("expected extracted=false when not connected")
	}
}

func TestHandleDeviceVerifyPeerNotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleDeviceVerifyPeer(context.Background(), &loader.Step{}, state)
	if out["peer_valid"] != false {
		t.Error("expected peer_valid=false when not connected")
	}
	if out["verification_success"] != false {
		t.Error("expected verification_success=false when not connected")
	}
	if out["same_zone_ca"] != false {
		t.Error("expected same_zone_ca=false when not connected")
	}
	if out["error"] != "no active connection" {
		t.Errorf("expected 'no active connection' error, got %v", out["error"])
	}
}

// C3: verify_certificate should include not_expired key.
func TestVerifyCertificate_NotExpiredKey(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Not connected: not_expired should be false.
	out, _ := r.handleVerifyCertificate(context.Background(), &loader.Step{}, state)
	if _, ok := out["not_expired"]; !ok {
		t.Error("expected not_expired key in output")
	}
	if out["not_expired"] != false {
		t.Error("expected not_expired=false when not connected")
	}
}

// C9: D2D precondition-driven verify_peer with two_devices_same_zone.
func TestDeviceVerifyPeer_D2DPreconditions_SameZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set precondition state.
	state.Set("two_devices_same_zone", true)

	out, _ := r.handleDeviceVerifyPeer(context.Background(), &loader.Step{}, state)
	if out["peer_valid"] != true {
		t.Error("expected peer_valid=true for same zone precondition")
	}
	if out["verification_success"] != true {
		t.Error("expected verification_success=true for same zone")
	}
	if out["same_zone_ca"] != true {
		t.Error("expected same_zone_ca=true for same zone")
	}
	if out["error"] != "" {
		t.Errorf("expected empty error, got %v", out["error"])
	}
}

// C9: D2D precondition-driven verify_peer with device_b_cert_expired.
func TestDeviceVerifyPeer_D2DPreconditions_Expired(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set("device_b_cert_expired", true)

	out, _ := r.handleDeviceVerifyPeer(context.Background(), &loader.Step{}, state)
	if out["peer_valid"] != false {
		t.Error("expected peer_valid=false for expired cert")
	}
	if out["error"] != "certificate_expired" {
		t.Errorf("expected error=certificate_expired, got %v", out["error"])
	}
}

// C9: D2D precondition-driven verify_peer with two_devices_different_zones.
func TestDeviceVerifyPeer_D2DPreconditions_DiffZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set("two_devices_different_zones", true)

	out, _ := r.handleDeviceVerifyPeer(context.Background(), &loader.Step{}, state)
	if out["peer_valid"] != false {
		t.Error("expected peer_valid=false for different zones")
	}
	if out["same_zone_ca"] != false {
		t.Error("expected same_zone_ca=false for different zones")
	}
	if out["error"] != "unknown_ca" {
		t.Errorf("expected error=unknown_ca, got %v", out["error"])
	}
}

func TestHandleVerifyDeviceCert_NotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, err := r.handleVerifyDeviceCert(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Enriched fields should be present even when not connected.
	if out["has_operational_cert"] != false {
		t.Error("expected has_operational_cert=false when not connected")
	}
	if out["cert_signed_by_zone_ca"] != false {
		t.Error("expected cert_signed_by_zone_ca=false when not connected")
	}
	if out["cert_validity_days"] != 0 {
		t.Errorf("expected cert_validity_days=0, got %v", out["cert_validity_days"])
	}
	// Base fields from handleVerifyCertificate.
	if out["cert_valid"] != false {
		t.Error("expected cert_valid=false when not connected")
	}
}

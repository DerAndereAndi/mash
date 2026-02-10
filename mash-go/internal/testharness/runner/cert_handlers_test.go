package runner

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestHandleResetPASESession(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up some PASE state.
	r.paseState = &PASEState{
		sessionKey: []byte{1, 2, 3},
		completed:  true,
	}
	state.Set(PrecondSessionEstablished, true)

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

	established, _ := state.Get(PrecondSessionEstablished)
	if established != false {
		t.Error("expected session_established=false after reset")
	}
}

func TestHandleVerifyCommissioningState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// IDLE state.
	step := &loader.Step{Params: map[string]any{ParamExpectedState: "IDLE"}}
	out, _ := r.handleVerifyCommissioningState(context.Background(), step, state)
	if out[KeyCommissioningState] != "IDLE" {
		t.Errorf("expected IDLE, got %v", out[KeyCommissioningState])
	}
	if out[KeyStateMatches] != true {
		t.Error("expected state_matches=true")
	}

	// CONNECTED state.
	r.conn.state = ConnTLSConnected
	step = &loader.Step{Params: map[string]any{ParamExpectedState: CommissioningStateConnected}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out[KeyCommissioningState] != CommissioningStateConnected {
		t.Errorf("expected %s, got %v", CommissioningStateConnected, out[KeyCommissioningState])
	}

	// ADVERTISING state: was connected but now disconnected.
	r.conn.state = ConnDisconnected
	r.conn.hadConnection = true
	r.paseState = nil
	step = &loader.Step{Params: map[string]any{ParamExpectedState: CommissioningStateAdvertising}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out[KeyCommissioningState] != CommissioningStateAdvertising {
		t.Errorf("expected %s, got %v", CommissioningStateAdvertising, out[KeyCommissioningState])
	}

	// COMMISSIONED state.
	r.conn.hadConnection = false
	r.paseState = &PASEState{completed: true}
	step = &loader.Step{Params: map[string]any{ParamExpectedState: CommissioningStateCommissioned}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out[KeyCommissioningState] != CommissioningStateCommissioned {
		t.Errorf("expected %s, got %v", CommissioningStateCommissioned, out[KeyCommissioningState])
	}

	// Mismatch.
	step = &loader.Step{Params: map[string]any{ParamExpectedState: "IDLE"}}
	out, _ = r.handleVerifyCommissioningState(context.Background(), step, state)
	if out[KeyStateMatches] != false {
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

	step := &loader.Step{Params: map[string]any{KeyDeviceID: "dev-123"}}
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
	if out[KeyPeerValid] != false {
		t.Error("expected peer_valid=false when not connected")
	}
	if out[KeyVerificationSuccess] != false {
		t.Error("expected verification_success=false when not connected")
	}
	if out[KeySameZoneCA] != false {
		t.Error("expected same_zone_ca=false when not connected")
	}
	if out[KeyError] != "no active connection" {
		t.Errorf("expected 'no active connection' error, got %v", out[KeyError])
	}
}

// C3: verify_certificate should include not_expired key.
func TestVerifyCertificate_NotExpiredKey(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Not connected: not_expired should be false.
	out, _ := r.handleVerifyCertificate(context.Background(), &loader.Step{}, state)
	if _, ok := out[KeyNotExpired]; !ok {
		t.Error("expected not_expired key in output")
	}
	if out[KeyNotExpired] != false {
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
	if out[KeyPeerValid] != true {
		t.Error("expected peer_valid=true for same zone precondition")
	}
	if out[KeyVerificationSuccess] != true {
		t.Error("expected verification_success=true for same zone")
	}
	if out[KeySameZoneCA] != true {
		t.Error("expected same_zone_ca=true for same zone")
	}
	if out[KeyError] != "" {
		t.Errorf("expected empty error, got %v", out[KeyError])
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
	if out[KeyError] != "certificate_expired" {
		t.Errorf("expected error=certificate_expired, got %v", out[KeyError])
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
	if out[KeyError] != "unknown_ca" {
		t.Errorf("expected error=unknown_ca, got %v", out[KeyError])
	}
}

func TestHandleVerifyCertificate_WithZoneCA(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Generate a Zone CA and a device cert signed by it.
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("generate zone CA: %v", err)
	}
	r.zoneCAPool = zoneCA.TLSClientCAs()

	// Generate a device key pair and CSR.
	keyPair, err := cert.GenerateKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}
	csrDER, err := cert.CreateCSR(keyPair, &cert.CSRInfo{
		Identity: cert.DeviceIdentity{DeviceID: "test-device"},
		ZoneID:   "test-zone",
	})
	if err != nil {
		t.Fatalf("create CSR: %v", err)
	}
	deviceCert, err := cert.SignCSR(zoneCA, csrDER)
	if err != nil {
		t.Fatalf("sign CSR: %v", err)
	}

	// Create a local TLS server with the device cert, connect to it.
	serverTLSCert := tls.Certificate{
		Certificate: [][]byte{deviceCert.Raw},
		PrivateKey:  keyPair.PrivateKey,
	}
	serverConfig := &tls.Config{
		Certificates: []tls.Certificate{serverTLSCert},
		MinVersion:   tls.VersionTLS13,
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	// Accept in background.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Keep alive until test finishes.
		buf := make([]byte, 1)
		conn.Read(buf)
	}()

	// Connect the runner to the local TLS server.
	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
	}
	tlsConn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer tlsConn.Close()

	r.conn.tlsConn = tlsConn
	r.conn.state = ConnOperational

	out, err := r.handleVerifyCertificate(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyChainValid] != true {
		t.Error("expected chain_valid=true when zoneCAPool is set and cert is signed by zone CA")
	}
	if out[KeyNotExpired] != true {
		t.Error("expected not_expired=true for fresh cert")
	}
	if out[KeyCertValid] != true {
		t.Error("expected cert_valid=true")
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

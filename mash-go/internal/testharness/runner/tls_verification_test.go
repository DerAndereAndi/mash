package runner

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

// generateSelfSignedCert creates a self-signed cert for testing.
func generateSelfSignedCert(cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	parsed, _ := x509.ParseCertificate(certDER)
	return parsed, key
}

// generateExpiredCert creates an expired cert signed by the given CA.
func generateExpiredCert(ca *cert.ZoneCA) (*x509.Certificate, *ecdsa.PrivateKey) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: "expired-device"},
		NotBefore:    time.Now().Add(-48 * time.Hour),
		NotAfter:     time.Now().Add(-24 * time.Hour), // expired
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, template, ca.Certificate, &key.PublicKey, ca.PrivateKey)
	parsed, _ := x509.ParseCertificate(certDER)
	return parsed, key
}

func TestVerifyPeerCert_AcceptsZoneCASigned(t *testing.T) {
	// Generate Zone CA.
	zoneCA, err := cert.GenerateZoneCA("test-zone-id-1234", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA: %v", err)
	}

	// Generate an operational cert signed by this Zone CA (device-ID CN, no localhost SAN).
	opCert, err := cert.GenerateControllerOperationalCert(zoneCA, "device-abcdef12")
	if err != nil {
		t.Fatalf("GenerateControllerOperationalCert: %v", err)
	}

	r := &Runner{
		config:     &Config{},
		conn:       &Connection{},
		zoneCAPool: zoneCA.TLSClientCAs(),
	}

	// This cert has no localhost SAN -- normal TLS would reject it.
	err = r.verifyPeerCertAgainstZoneCA([][]byte{opCert.Certificate.Raw}, nil)
	if err != nil {
		t.Errorf("expected cert to be accepted, got: %v", err)
	}
}

func TestVerifyPeerCert_RejectsUntrustedCA(t *testing.T) {
	// Generate two different Zone CAs.
	trustedCA, err := cert.GenerateZoneCA("trusted-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA (trusted): %v", err)
	}
	untrustedCA, err := cert.GenerateZoneCA("untrusted-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA (untrusted): %v", err)
	}

	// Generate cert signed by untrusted CA.
	opCert, err := cert.GenerateControllerOperationalCert(untrustedCA, "device-xyz")
	if err != nil {
		t.Fatalf("GenerateControllerOperationalCert: %v", err)
	}

	r := &Runner{
		config:     &Config{},
		conn:       &Connection{},
		zoneCAPool: trustedCA.TLSClientCAs(), // Only trust the first CA
	}

	err = r.verifyPeerCertAgainstZoneCA([][]byte{opCert.Certificate.Raw}, nil)
	if err == nil {
		t.Error("expected cert signed by untrusted CA to be rejected")
	}
}

func TestVerifyPeerCert_RejectsExpiredCert(t *testing.T) {
	zoneCA, err := cert.GenerateZoneCA("expired-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA: %v", err)
	}

	expiredCert, _ := generateExpiredCert(zoneCA)

	r := &Runner{
		config:     &Config{},
		conn:       &Connection{},
		zoneCAPool: zoneCA.TLSClientCAs(),
	}

	err = r.verifyPeerCertAgainstZoneCA([][]byte{expiredCert.Raw}, nil)
	if err == nil {
		t.Error("expected expired cert to be rejected")
	}
}

func TestVerifyPeerCert_RejectsNoCerts(t *testing.T) {
	r := &Runner{
		config:     &Config{},
		conn:       &Connection{},
		zoneCAPool: x509.NewCertPool(),
	}

	err := r.verifyPeerCertAgainstZoneCA(nil, nil)
	if err == nil {
		t.Error("expected error for empty cert list")
	}
}

func TestOperationalTLSConfig_PresentsControllerCert(t *testing.T) {
	zoneCA, err := cert.GenerateZoneCA("tls-test-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA: %v", err)
	}

	controllerCert, err := cert.GenerateControllerOperationalCert(zoneCA, "test-ctrl")
	if err != nil {
		t.Fatalf("GenerateControllerOperationalCert: %v", err)
	}

	r := &Runner{
		config:         &Config{},
		conn:           &Connection{},
		zoneCA:         zoneCA,
		zoneCAPool:     zoneCA.TLSClientCAs(),
		controllerCert: controllerCert,
	}

	tlsConfig := r.operationalTLSConfig()

	// Should have InsecureSkipVerify=true (we use custom verify).
	if !tlsConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true for operational config")
	}

	// Should have a custom VerifyPeerCertificate.
	if tlsConfig.VerifyPeerCertificate == nil {
		t.Error("expected VerifyPeerCertificate to be set")
	}

	// Should present controller cert.
	if len(tlsConfig.Certificates) == 0 {
		t.Error("expected controller cert to be presented")
	}

	// Should use TLS 1.3.
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected TLS 1.3, got %d", tlsConfig.MinVersion)
	}
}

func TestOperationalTLSConfig_NoZoneCA_FallsBack(t *testing.T) {
	r := &Runner{
		config: &Config{InsecureSkipVerify: true},
		conn:   &Connection{},
	}

	tlsConfig := r.operationalTLSConfig()

	// Without zone CA, should use config's insecure setting.
	if !tlsConfig.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true when no zone CA")
	}

	// Should NOT have custom verifier.
	if tlsConfig.VerifyPeerCertificate != nil {
		t.Error("expected no VerifyPeerCertificate when no zone CA")
	}
}

func TestClassifyConnectError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"timeout", fmt.Errorf("dial tcp: i/o timeout"), ErrCodeTimeout},
		{"deadline", fmt.Errorf("context deadline exceeded"), ErrCodeTimeout},
		{"refused", fmt.Errorf("dial tcp 127.0.0.1:8443: connection refused"), ErrCodeConnectionFailed},
		{"tls", fmt.Errorf("tls: bad certificate"), ErrCodeTLSError},
		{"certificate", fmt.Errorf("x509: certificate signed by unknown authority"), ErrCodeTLSError},
		{"generic", fmt.Errorf("some other error"), ErrCodeConnectionError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyConnectError(tt.err)
			if got != tt.want {
				t.Errorf("classifyConnectError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestTLSVersionName(t *testing.T) {
	if v := tlsVersionName(tls.VersionTLS13); v != "1.3" {
		t.Errorf("expected '1.3', got %q", v)
	}
	if v := tlsVersionName(tls.VersionTLS12); v != "1.2" {
		t.Errorf("expected '1.2', got %q", v)
	}
	if v := tlsVersionName(0); v == "1.3" {
		t.Error("expected non-1.3 for unknown version")
	}
}

func TestHandleConnectOperational_ErrorOutputs(t *testing.T) {
	r := &Runner{
		config: &Config{Target: "127.0.0.1:1"},
		conn:   &Connection{},
	}
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		"target":  "127.0.0.1:1",
		KeyZoneID: "zone-test",
	}}
	out, err := r.handleConnectOperational(context.Background(), step, state)
	// Should return output map, not an error.
	if err != nil {
		t.Fatalf("expected nil error (output map), got: %v", err)
	}
	if out[KeyConnectionEstablished] != false {
		t.Error("expected connection_established=false")
	}
	if _, ok := out["error_code"]; !ok {
		t.Error("expected error_code key in output")
	}
	if _, ok := out[KeyError]; !ok {
		t.Error("expected error key in output")
	}
}

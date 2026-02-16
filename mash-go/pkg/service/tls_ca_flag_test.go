package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// TestBuildOperationalTLSConfig_NoCAFlagValidation demonstrates that the
// device's operational TLS config does NOT validate the BasicConstraints
// CA flag on controller certificates.
//
// Root cause of TC-TLS-CTRL-006: Go's standard x509 verification (used by
// tls.Config with ClientAuth=RequireAndVerifyClientCert) does not check whether
// a client certificate has IsCA=true. The device's buildOperationalTLSConfig
// sets up TLS mutual auth but has no VerifyPeerCertificate callback to reject
// controller certs with CA:TRUE.
//
// RED: operationalTLSConfig.VerifyPeerCertificate is nil (no CA flag check).
func TestBuildOperationalTLSConfig_NoCAFlagValidation(t *testing.T) {
	svc := startCappedDevice(t, 2)

	// Simulate a zone being added so buildOperationalTLSConfig has a CA pool.
	svc.HandleZoneConnect("test-zone", 3) // ZoneTypeTest = 3

	// Trigger rebuild of operational TLS config.
	svc.mu.Lock()
	svc.buildOperationalTLSConfig()
	cfg := svc.operationalTLSConfig
	svc.mu.Unlock()

	if cfg == nil {
		t.Fatal("operationalTLSConfig is nil after buildOperationalTLSConfig")
	}

	// The config MUST have a VerifyPeerCertificate callback that rejects
	// certificates with IsCA=true. Currently it has none → RED.
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("operationalTLSConfig.VerifyPeerCertificate is nil — " +
			"device does not validate BasicConstraints CA flag on controller certs")
	}
}

// selfSignedCert creates a self-signed end-entity certificate with the
// given validity period. Used for VerifyPeerCertificate callback tests.
func selfSignedCert(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-controller"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		IsCA:         false,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}

// TestVerifyPeerCertificate_RejectsExpiredWithClockOffset verifies that the
// VerifyPeerCertificate callback rejects certificates that appear expired
// when the device's clock offset is applied.
//
// Root cause of TC-CERT-VAL-005: The device adjusts its clock forward (via
// TriggerAdjustClockBase), making the controller's certificate appear expired.
// The VerifyPeerCertificate callback must use clockOffset when checking cert times.
func TestVerifyPeerCertificate_RejectsExpiredWithClockOffset(t *testing.T) {
	svc := startCappedDevice(t, 2)
	svc.HandleZoneConnect("test-zone", 3)

	svc.mu.Lock()
	svc.buildOperationalTLSConfig()
	cfg := svc.operationalTLSConfig
	svc.mu.Unlock()

	if cfg == nil || cfg.VerifyPeerCertificate == nil {
		t.Fatal("precondition: VerifyPeerCertificate callback must exist")
	}

	// Create a cert valid for the next hour.
	now := time.Now()
	certDER := selfSignedCert(t, now.Add(-1*time.Hour), now.Add(1*time.Hour))

	// Without clock offset: cert should be accepted (valid now).
	if err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil); err != nil {
		t.Fatalf("expected valid cert to pass without clock offset, got: %v", err)
	}

	// Shift device clock forward by 2 hours -- cert is now "expired" from
	// the device's perspective (2h > 1h validity + 300s tolerance).
	svc.mu.Lock()
	svc.clockOffset = 2 * time.Hour
	svc.mu.Unlock()

	// Must rebuild TLS config to pick up the closure's access to clockOffset.
	// Actually, the closure captures `s`, so it reads clockOffset at call time.
	// No rebuild needed.
	if err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil); err == nil {
		t.Fatal("expected expired cert to be rejected with +2h clock offset, but it was accepted")
	}
}

// TestVerifyPeerCertificate_AcceptsWithinClockSkewTolerance verifies that
// certs within the 300s tolerance are accepted even with clock offset.
func TestVerifyPeerCertificate_AcceptsWithinClockSkewTolerance(t *testing.T) {
	svc := startCappedDevice(t, 2)
	svc.HandleZoneConnect("test-zone", 3)

	svc.mu.Lock()
	svc.buildOperationalTLSConfig()
	cfg := svc.operationalTLSConfig
	svc.mu.Unlock()

	if cfg == nil || cfg.VerifyPeerCertificate == nil {
		t.Fatal("precondition: VerifyPeerCertificate callback must exist")
	}

	// Cert valid for the next hour.
	now := time.Now()
	certDER := selfSignedCert(t, now.Add(-1*time.Hour), now.Add(1*time.Hour))

	// Shift clock forward by 1h + 200s (within tolerance of 300s past NotAfter).
	svc.mu.Lock()
	svc.clockOffset = 1*time.Hour + 200*time.Second
	svc.mu.Unlock()

	if err := cfg.VerifyPeerCertificate([][]byte{certDER}, nil); err != nil {
		t.Fatalf("expected cert within 300s tolerance to be accepted, got: %v", err)
	}
}

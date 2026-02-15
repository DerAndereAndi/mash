package service

import (
	"testing"
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

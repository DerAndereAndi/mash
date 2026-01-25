package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// generateTestCertificate creates a self-signed certificate for testing.
func generateTestCertificate(t *testing.T) (tls.Certificate, *x509.Certificate) {
	t.Helper()

	// Generate key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.local",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Parse back for verification
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}, cert
}

func TestNewServerTLSConfig(t *testing.T) {
	cert, _ := generateTestCertificate(t)

	config := &TLSConfig{
		Certificate: cert,
	}

	tlsConfig, err := NewServerTLSConfig(config)
	if err != nil {
		t.Fatalf("NewServerTLSConfig failed: %v", err)
	}

	// Check TLS 1.3 requirement
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %d, want TLS 1.3 (%d)", tlsConfig.MinVersion, tls.VersionTLS13)
	}

	// Check ALPN
	if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != ALPNProtocol {
		t.Errorf("NextProtos = %v, want [%s]", tlsConfig.NextProtos, ALPNProtocol)
	}

	// Check client auth
	if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", tlsConfig.ClientAuth)
	}
}

func TestNewServerTLSConfigNoCert(t *testing.T) {
	config := &TLSConfig{}

	_, err := NewServerTLSConfig(config)
	if err == nil {
		t.Error("expected error for missing certificate")
	}
}

func TestNewClientTLSConfig(t *testing.T) {
	cert, caCert := generateTestCertificate(t)

	// Create CA pool
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	config := &TLSConfig{
		Certificate: cert,
		RootCAs:     caPool,
	}

	tlsConfig, err := NewClientTLSConfig(config)
	if err != nil {
		t.Fatalf("NewClientTLSConfig failed: %v", err)
	}

	// Check TLS 1.3 requirement
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %d, want TLS 1.3 (%d)", tlsConfig.MinVersion, tls.VersionTLS13)
	}

	// Check ALPN
	if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != ALPNProtocol {
		t.Errorf("NextProtos = %v, want [%s]", tlsConfig.NextProtos, ALPNProtocol)
	}

	// Check certificate is set
	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Certificates length = %d, want 1", len(tlsConfig.Certificates))
	}
}

func TestNewClientTLSConfigNoCert(t *testing.T) {
	config := &TLSConfig{}

	_, err := NewClientTLSConfig(config)
	if err == nil {
		t.Error("expected error for missing certificate")
	}
}

func TestNewCommissioningTLSConfig(t *testing.T) {
	tlsConfig := NewCommissioningTLSConfig()

	// Check TLS 1.3 requirement
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %d, want TLS 1.3 (%d)", tlsConfig.MinVersion, tls.VersionTLS13)
	}

	// Check ALPN
	if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != ALPNProtocol {
		t.Errorf("NextProtos = %v, want [%s]", tlsConfig.NextProtos, ALPNProtocol)
	}

	// Commissioning mode skips verification
	if tlsConfig.InsecureSkipVerify != true {
		t.Error("InsecureSkipVerify should be true for commissioning")
	}
}

func TestVerifyConnectionValid(t *testing.T) {
	// Create a mock connection state with valid TLS 1.3
	state := tls.ConnectionState{
		Version:            tls.VersionTLS13,
		NegotiatedProtocol: ALPNProtocol,
	}

	if err := VerifyConnection(state); err != nil {
		t.Errorf("VerifyConnection failed for valid state: %v", err)
	}
}

func TestVerifyConnectionWrongVersion(t *testing.T) {
	state := tls.ConnectionState{
		Version:            tls.VersionTLS12,
		NegotiatedProtocol: ALPNProtocol,
	}

	err := VerifyConnection(state)
	if err == nil {
		t.Error("expected error for TLS 1.2")
	}
}

func TestVerifyConnectionWrongALPN(t *testing.T) {
	state := tls.ConnectionState{
		Version:            tls.VersionTLS13,
		NegotiatedProtocol: "http/1.1",
	}

	err := VerifyConnection(state)
	if err == nil {
		t.Error("expected error for wrong ALPN")
	}
}

func TestVerifyConnectionNoALPN(t *testing.T) {
	state := tls.ConnectionState{
		Version:            tls.VersionTLS13,
		NegotiatedProtocol: "",
	}

	err := VerifyConnection(state)
	if err == nil {
		t.Error("expected error for no ALPN")
	}
}

func TestVerifyConnectionMutualTLS(t *testing.T) {
	cert, _ := generateTestCertificate(t)
	parsedCert, _ := x509.ParseCertificate(cert.Certificate[0])

	state := tls.ConnectionState{
		Version:            tls.VersionTLS13,
		NegotiatedProtocol: ALPNProtocol,
		PeerCertificates:   []*x509.Certificate{parsedCert},
	}

	if err := VerifyConnection(state); err != nil {
		t.Errorf("VerifyConnection failed with peer cert: %v", err)
	}
}

func TestDefaultPort(t *testing.T) {
	if DefaultPort != 8443 {
		t.Errorf("DefaultPort = %d, want 8443", DefaultPort)
	}
}

func TestALPNProtocol(t *testing.T) {
	if ALPNProtocol != "mash/1" {
		t.Errorf("ALPNProtocol = %s, want mash/1", ALPNProtocol)
	}
}

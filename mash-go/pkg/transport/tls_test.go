package transport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/version"
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

	// Check ALPN uses version.SupportedALPNProtocols()
	wantProtos := version.SupportedALPNProtocols()
	if !slices.Equal(tlsConfig.NextProtos, wantProtos) {
		t.Errorf("NextProtos = %v, want %v", tlsConfig.NextProtos, wantProtos)
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

	// Check ALPN uses version.SupportedALPNProtocols()
	wantProtos := version.SupportedALPNProtocols()
	if !slices.Equal(tlsConfig.NextProtos, wantProtos) {
		t.Errorf("NextProtos = %v, want %v", tlsConfig.NextProtos, wantProtos)
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

	// Check ALPN uses the commissioning protocol
	wantProtos := []string{ALPNCommissioningProtocol}
	if !slices.Equal(tlsConfig.NextProtos, wantProtos) {
		t.Errorf("NextProtos = %v, want %v", tlsConfig.NextProtos, wantProtos)
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

func TestSupportedALPNProtocols(t *testing.T) {
	protos := version.SupportedALPNProtocols()
	if len(protos) != 1 || protos[0] != "mash/1" {
		t.Errorf("SupportedALPNProtocols() = %v, want [mash/1]", protos)
	}
}

func TestVerifyALPN_AcceptsCurrentVersion(t *testing.T) {
	state := tls.ConnectionState{NegotiatedProtocol: "mash/1"}
	if err := VerifyALPN(state); err != nil {
		t.Errorf("VerifyALPN should accept mash/1: %v", err)
	}
}

func TestVerifyALPN_RejectsUnknownProtocol(t *testing.T) {
	state := tls.ConnectionState{NegotiatedProtocol: "http/1.1"}
	if err := VerifyALPN(state); err == nil {
		t.Error("VerifyALPN should reject http/1.1")
	}
}

func TestVerifyALPN_RejectsEmptyProtocol(t *testing.T) {
	state := tls.ConnectionState{NegotiatedProtocol: ""}
	if err := VerifyALPN(state); err == nil {
		t.Error("VerifyALPN should reject empty protocol")
	}
}

func TestVerifyALPN_RejectsMalformed(t *testing.T) {
	state := tls.ConnectionState{NegotiatedProtocol: "mash/"}
	if err := VerifyALPN(state); err == nil {
		t.Error("VerifyALPN should reject malformed mash/")
	}
}

// TC-IMPL-TLS-001: Controller TLS Config Uses Operational Cert
func TestNewOperationalClientTLSConfigUsesCert(t *testing.T) {
	cert, _ := generateTestCertificate(t)
	caPool := x509.NewCertPool()

	cfg := &OperationalTLSConfig{
		ControllerCert: cert,
		ZoneCAs:        caPool,
		ServerName:     "device-123",
	}

	tlsConfig, err := NewOperationalClientTLSConfig(cfg)
	if err != nil {
		t.Fatalf("NewOperationalClientTLSConfig() error = %v", err)
	}

	// Verify certificate is set
	if len(tlsConfig.Certificates) != 1 {
		t.Fatalf("Certificates length = %d, want 1", len(tlsConfig.Certificates))
	}

	// Verify it's the controller cert, not some other cert
	if len(tlsConfig.Certificates[0].Certificate) != 1 {
		t.Error("Certificate chain should have 1 certificate")
	}
	if !bytesEqual(tlsConfig.Certificates[0].Certificate[0], cert.Certificate[0]) {
		t.Error("TLS config should contain the controller operational cert")
	}
}

// TC-IMPL-TLS-002: Controller TLS Config Includes Zone CA for Verification
func TestNewOperationalClientTLSConfigIncludesZoneCA(t *testing.T) {
	cert, caCert := generateTestCertificate(t)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	cfg := &OperationalTLSConfig{
		ControllerCert: cert,
		ZoneCAs:        caPool,
		ServerName:     "device-123",
	}

	tlsConfig, err := NewOperationalClientTLSConfig(cfg)
	if err != nil {
		t.Fatalf("NewOperationalClientTLSConfig() error = %v", err)
	}

	// Verify RootCAs is set
	if tlsConfig.RootCAs == nil {
		t.Fatal("RootCAs should be set for device cert verification")
	}

	// The pool should be the same as what we passed in
	if tlsConfig.RootCAs != caPool {
		t.Error("RootCAs should be the Zone CA pool we provided")
	}
}

// TestNewOperationalClientTLSConfigValidation tests input validation
func TestNewOperationalClientTLSConfigValidation(t *testing.T) {
	cert, caCert := generateTestCertificate(t)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	t.Run("NilConfig", func(t *testing.T) {
		_, err := NewOperationalClientTLSConfig(nil)
		if err == nil {
			t.Error("expected error for nil config")
		}
	})

	t.Run("MissingCert", func(t *testing.T) {
		cfg := &OperationalTLSConfig{
			ZoneCAs:    caPool,
			ServerName: "device-123",
		}
		_, err := NewOperationalClientTLSConfig(cfg)
		if err == nil {
			t.Error("expected error for missing controller certificate")
		}
	})

	t.Run("MissingZoneCAs", func(t *testing.T) {
		cfg := &OperationalTLSConfig{
			ControllerCert: cert,
			ServerName:     "device-123",
		}
		_, err := NewOperationalClientTLSConfig(cfg)
		if err == nil {
			t.Error("expected error for missing Zone CA pool")
		}
	})
}

// TC-IMPL-TLS-005: Commissioning Still Uses Insecure TLS
func TestCommissioningTLSConfigNoClientCert(t *testing.T) {
	tlsConfig := NewCommissioningTLSConfig()

	// Commissioning should NOT present a client certificate
	if len(tlsConfig.Certificates) != 0 {
		t.Errorf("Commissioning TLS should not have certificates, got %d", len(tlsConfig.Certificates))
	}

	// Should skip verification (security comes from SPAKE2+)
	if !tlsConfig.InsecureSkipVerify {
		t.Error("Commissioning TLS should have InsecureSkipVerify=true")
	}
}

// TestOperationalClientTLSConfigProperties verifies TLS 1.3 and ALPN
func TestOperationalClientTLSConfigProperties(t *testing.T) {
	cert, caCert := generateTestCertificate(t)
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	cfg := &OperationalTLSConfig{
		ControllerCert: cert,
		ZoneCAs:        caPool,
		ServerName:     "device-123",
	}

	tlsConfig, err := NewOperationalClientTLSConfig(cfg)
	if err != nil {
		t.Fatalf("NewOperationalClientTLSConfig() error = %v", err)
	}

	// Must use TLS 1.3
	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = 0x%x, want TLS 1.3 (0x0304)", tlsConfig.MinVersion)
	}
	if tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Errorf("MaxVersion = 0x%x, want TLS 1.3 (0x0304)", tlsConfig.MaxVersion)
	}

	// Must use MASH ALPN from version package
	wantProtos := version.SupportedALPNProtocols()
	if !slices.Equal(tlsConfig.NextProtos, wantProtos) {
		t.Errorf("NextProtos = %v, want %v", tlsConfig.NextProtos, wantProtos)
	}

	// Operational connections use InsecureSkipVerify=true because we handle
	// verification manually via VerifyPeerCertificate callback. This is needed
	// because MASH uses device IDs (not DNS names) for identification, so Go's
	// built-in hostname verification doesn't apply.
	if !tlsConfig.InsecureSkipVerify {
		t.Error("Operational TLS should have InsecureSkipVerify=true (uses custom verification)")
	}

	// VerifyPeerCertificate should be set to handle chain and device ID verification
	if tlsConfig.VerifyPeerCertificate == nil {
		t.Error("Operational TLS should have VerifyPeerCertificate callback set")
	}

	// ServerName should be set
	if tlsConfig.ServerName != "device-123" {
		t.Errorf("ServerName = %q, want %q", tlsConfig.ServerName, "device-123")
	}
}

// bytesEqual compares two byte slices for equality.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// generateCAAndCert creates a CA and a certificate signed by that CA for testing.
// This simulates the Zone CA -> Operational Cert chain used in MASH.
func generateCAAndCert(t *testing.T, cn string) (caCert *x509.Certificate, caKey *ecdsa.PrivateKey, tlsCert tls.Certificate) {
	t.Helper()

	// Generate CA key pair
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	// Create CA certificate template
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test Zone CA",
			Organization: []string{"MASH Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	// Self-sign CA
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA cert: %v", err)
	}
	caCert, err = x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("failed to parse CA cert: %v", err)
	}

	// Generate end-entity key pair
	eeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate EE key: %v", err)
	}

	// Create end-entity certificate template
	eeTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Sign by CA
	eeCertDER, err := x509.CreateCertificate(rand.Reader, eeTemplate, caCert, &eeKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create EE cert: %v", err)
	}
	eeCert, err := x509.ParseCertificate(eeCertDER)
	if err != nil {
		t.Fatalf("failed to parse EE cert: %v", err)
	}

	tlsCert = tls.Certificate{
		Certificate: [][]byte{eeCertDER},
		PrivateKey:  eeKey,
		Leaf:        eeCert,
	}

	return caCert, caKey, tlsCert
}

// TC-IMPL-TLS-003: Mutual TLS Handshake Succeeds
func TestMutualTLSHandshakeSucceeds(t *testing.T) {
	// Create Zone CA and certificates for both sides
	zoneCA, _, controllerCert := generateCAAndCert(t, "controller-123")
	_, _, deviceCert := generateCAAndCert(t, "device-456")

	// For this test, both certs must be signed by the SAME Zone CA
	// Regenerate device cert signed by the zone CA
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate device key: %v", err)
	}

	// We need the CA key to sign - extract it from the controller cert generation
	// Actually, we need to regenerate with shared CA
	sharedCA, sharedCAKey, sharedControllerCert := generateCAAndCert(t, "controller-123")

	// Create device cert signed by same CA
	deviceTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "device-456",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	deviceCertDER, err := x509.CreateCertificate(rand.Reader, deviceTemplate, sharedCA, &deviceKey.PublicKey, sharedCAKey)
	if err != nil {
		t.Fatalf("failed to create device cert: %v", err)
	}
	deviceCertParsed, _ := x509.ParseCertificate(deviceCertDER)
	sharedDeviceCert := tls.Certificate{
		Certificate: [][]byte{deviceCertDER},
		PrivateKey:  deviceKey,
		Leaf:        deviceCertParsed,
	}

	// Create CA pool with the shared Zone CA
	caPool := x509.NewCertPool()
	caPool.AddCert(sharedCA)

	// Create server (device) TLS config
	serverConfig, err := NewServerTLSConfig(&TLSConfig{
		Certificate: sharedDeviceCert,
		ClientCAs:   caPool,
	})
	if err != nil {
		t.Fatalf("NewServerTLSConfig() error = %v", err)
	}

	// Create client (controller) TLS config
	clientConfig, err := NewOperationalClientTLSConfig(&OperationalTLSConfig{
		ControllerCert: sharedControllerCert,
		ZoneCAs:        caPool,
		ServerName:     "localhost",
	})
	if err != nil {
		t.Fatalf("NewOperationalClientTLSConfig() error = %v", err)
	}

	// Start TLS server
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Server goroutine
	serverDone := make(chan error, 1)
	var serverPeerCerts []*x509.Certificate
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			serverDone <- err
			return
		}

		// Get peer certificates
		serverPeerCerts = tlsConn.ConnectionState().PeerCertificates
		serverDone <- nil
	}()

	// Connect as client
	conn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
	if err != nil {
		t.Fatalf("client TLS dial failed: %v", err)
	}
	defer conn.Close()

	// Verify client got server's certificate
	clientState := conn.ConnectionState()
	if len(clientState.PeerCertificates) == 0 {
		t.Error("client should have received server's certificate")
	}
	if clientState.PeerCertificates[0].Subject.CommonName != "device-456" {
		t.Errorf("client peer cert CN = %q, want %q",
			clientState.PeerCertificates[0].Subject.CommonName, "device-456")
	}

	// Wait for server and check it got client's certificate
	if err := <-serverDone; err != nil {
		t.Fatalf("server handshake failed: %v", err)
	}

	if len(serverPeerCerts) == 0 {
		t.Error("server should have received client's certificate")
	}
	if serverPeerCerts[0].Subject.CommonName != "controller-123" {
		t.Errorf("server peer cert CN = %q, want %q",
			serverPeerCerts[0].Subject.CommonName, "controller-123")
	}

	// Suppress unused variable warnings
	_ = zoneCA
	_ = controllerCert
	_ = deviceCert
}

// TC-IMPL-TLS-004: Mutual TLS Rejects Wrong Zone
func TestMutualTLSRejectsWrongZone(t *testing.T) {
	// Create Zone A CA and controller cert
	zoneACA, _, controllerCert := generateCAAndCert(t, "controller-123")

	// Create Zone B CA and device cert
	zoneBCA, zoneBCAKey, _ := generateCAAndCert(t, "unused")

	// Create device cert signed by Zone B CA
	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	deviceTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "device-456",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}
	deviceCertDER, _ := x509.CreateCertificate(rand.Reader, deviceTemplate, zoneBCA, &deviceKey.PublicKey, zoneBCAKey)
	deviceCertParsed, _ := x509.ParseCertificate(deviceCertDER)
	deviceCert := tls.Certificate{
		Certificate: [][]byte{deviceCertDER},
		PrivateKey:  deviceKey,
		Leaf:        deviceCertParsed,
	}

	// Controller expects Zone A CA
	zoneAPool := x509.NewCertPool()
	zoneAPool.AddCert(zoneACA)

	// Device uses Zone B CA for client verification
	zoneBPool := x509.NewCertPool()
	zoneBPool.AddCert(zoneBCA)

	// Create server (device) with Zone B CA
	serverConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: deviceCert,
		ClientCAs:   zoneBPool,
	})

	// Create client (controller) expecting Zone A CA
	clientConfig, _ := NewOperationalClientTLSConfig(&OperationalTLSConfig{
		ControllerCert: controllerCert,
		ZoneCAs:        zoneAPool, // Wrong CA - device uses Zone B
		ServerName:     "localhost",
	})

	// Start TLS server
	listener, err := tls.Listen("tcp", "127.0.0.1:0", serverConfig)
	if err != nil {
		t.Fatalf("failed to create TLS listener: %v", err)
	}
	defer listener.Close()

	// Server goroutine - expect it to fail
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		_ = tlsConn.Handshake() // Expected to fail
	}()

	// Client connection should fail (device cert not signed by Zone A CA)
	conn, err := tls.Dial("tcp", listener.Addr().String(), clientConfig)
	if err == nil {
		conn.Close()
		t.Error("TLS handshake should fail when certificates are from different Zone CAs")
	}
	// Error is expected - either certificate verification failed or handshake error
}

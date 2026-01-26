package mash_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// TestE2E_Discovery tests that a controller can discover a device via mDNS.
func TestE2E_Discovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup: Device advertises commissionable service
	advertiser, err := discovery.NewMDNSAdvertiser(discovery.AdvertiserConfig{})
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer advertiser.StopAll()

	discriminator := uint16(1234)
	commInfo := &discovery.CommissionableInfo{
		Discriminator: discriminator,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility}, // EV charging
		Serial:        "TEST-001",
		Brand:         "MASHTest",
		Model:         "E2E",
		DeviceName:    "Test EVSE",
		Port:          8443,
	}

	if err := advertiser.AdvertiseCommissionable(ctx, commInfo); err != nil {
		t.Fatalf("Failed to advertise commissionable: %v", err)
	}

	// Give mDNS time to propagate
	time.Sleep(500 * time.Millisecond)

	// Controller browses for devices
	browser, err := discovery.NewMDNSBrowser(discovery.BrowserConfig{})
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	// Find by discriminator
	browseCtx, browseCancel := context.WithTimeout(ctx, 5*time.Second)
	defer browseCancel()

	found, err := browser.FindByDiscriminator(browseCtx, discriminator)
	if err != nil {
		t.Fatalf("Failed to find device: %v", err)
	}

	// Verify discovered info
	if found.Discriminator != discriminator {
		t.Errorf("Discriminator mismatch: expected %d, got %d", discriminator, found.Discriminator)
	}
	if found.Brand != "MASHTest" {
		t.Errorf("Brand mismatch: expected MASHTest, got %s", found.Brand)
	}
	if found.Port != 8443 {
		t.Errorf("Port mismatch: expected 8443, got %d", found.Port)
	}
}

// TestE2E_TLSConnection tests basic TLS connection between client and server.
func TestE2E_TLSConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Generate test certificates
	serverCert, err := generateSelfSignedCert("device.mash.local")
	if err != nil {
		t.Fatalf("Failed to generate server cert: %v", err)
	}

	// Create TLS server
	serverTLSConfig := &transport.TLSConfig{
		Certificate:        serverCert,
		InsecureSkipVerify: true, // For testing
	}

	var receivedMsg []byte
	var msgWg sync.WaitGroup
	msgWg.Add(1)

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: serverTLSConfig,
		Address:   "127.0.0.1:0", // Random port
		OnMessage: func(conn *transport.ServerConn, msg []byte) {
			receivedMsg = msg
			// Echo back
			conn.Send(msg)
			msgWg.Done()
		},
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Get actual server address
	addr := server.Addr().String()

	// Create client in commissioning mode (InsecureSkipVerify)
	client, err := transport.NewClient(transport.ClientConfig{
		CommissioningMode: true,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Connect
	conn, err := client.Connect(ctx, addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send a message
	testMsg := []byte("Hello, MASH!")
	if err := conn.Send(testMsg); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for server to receive and echo
	msgWg.Wait()

	// Verify server received our message
	if string(receivedMsg) != string(testMsg) {
		t.Errorf("Server received wrong message: expected %q, got %q", testMsg, receivedMsg)
	}

	// Receive echo
	response, err := conn.Receive(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	if string(response) != string(testMsg) {
		t.Errorf("Wrong response: expected %q, got %q", testMsg, response)
	}
}

// TestE2E_CommissioningHandshake tests the full PASE handshake over TLS.
// Note: This test uses raw TLS (not the framed transport) because PASE
// has its own length-prefixed framing. In production, PASE messages would
// be wrapped in the transport framing layer.
func TestE2E_CommissioningHandshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup code and identities
	setupCode := commissioning.MustParseSetupCode("12345678")
	clientIdentity := []byte("controller-test")
	serverIdentity := []byte("device-test")

	// Generate verifier (device does this during manufacturing)
	verifier, err := commissioning.GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to generate verifier: %v", err)
	}

	// Generate test certificates
	serverCert, err := generateSelfSignedCert("device.mash.local")
	if err != nil {
		t.Fatalf("Failed to generate server cert: %v", err)
	}

	// Create raw TLS server for PASE (bypassing transport framing)
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{serverCert},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	tlsListener := tls.NewListener(listener, tlsConfig)

	var serverKey []byte
	var serverErr error
	var serverWg sync.WaitGroup
	serverWg.Add(1)

	// Server goroutine - accepts one connection and runs PASE
	go func() {
		defer serverWg.Done()

		conn, err := tlsListener.Accept()
		if err != nil {
			serverErr = fmt.Errorf("accept failed: %w", err)
			return
		}
		defer conn.Close()

		// Create server PASE session
		session, err := commissioning.NewPASEServerSession(verifier, serverIdentity)
		if err != nil {
			serverErr = fmt.Errorf("failed to create server session: %w", err)
			return
		}

		// Run handshake directly over TLS connection
		key, err := session.Handshake(ctx, conn)
		if err != nil {
			serverErr = fmt.Errorf("server handshake failed: %w", err)
			return
		}
		serverKey = key
	}()

	// Client side - connect and run PASE
	clientTLSConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	conn, err := tls.Dial("tcp", listener.Addr().String(), clientTLSConfig)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Create client PASE session
	clientSession, err := commissioning.NewPASEClientSession(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}

	// Run client handshake directly over TLS connection
	clientKey, err := clientSession.Handshake(ctx, conn)
	if err != nil {
		t.Fatalf("Client handshake failed: %v", err)
	}

	// Wait for server handshake to complete
	serverWg.Wait()

	if serverErr != nil {
		t.Fatalf("Server error: %v", serverErr)
	}

	// Verify both sides derived the same key
	if len(clientKey) != commissioning.SharedSecretSize {
		t.Errorf("Client key wrong size: expected %d, got %d", commissioning.SharedSecretSize, len(clientKey))
	}

	if len(serverKey) != commissioning.SharedSecretSize {
		t.Errorf("Server key wrong size: expected %d, got %d", commissioning.SharedSecretSize, len(serverKey))
	}

	// Keys should match
	for i := range clientKey {
		if clientKey[i] != serverKey[i] {
			t.Errorf("Keys don't match at position %d", i)
			break
		}
	}

	t.Logf("PASE handshake successful - both sides derived matching %d-byte session key", len(clientKey))
}

// TestE2E_Failsafe tests that failsafe triggers after connection loss.
func TestE2E_Failsafe(t *testing.T) {
	// This test verifies the failsafe timer mechanism from pkg/service
	// For now, we test the timer directly rather than through the full service
	// since wiring up the full service requires more infrastructure

	t.Skip("TODO: Wire up full service layer for failsafe testing")
}

// TestE2E_MultiZone tests multiple controllers connecting to a device.
func TestE2E_MultiZone(t *testing.T) {
	// This test requires:
	// - Device accepting multiple TLS connections
	// - Zone management in the service layer
	// - Proper certificate handling for different zones

	t.Skip("TODO: Wire up multi-zone support in service layer")
}

// TestE2E_Reconnection tests automatic reconnection after disconnect.
func TestE2E_Reconnection(t *testing.T) {
	// This test requires:
	// - Client-side reconnection logic
	// - Server accepting reconnection
	// - Session continuity

	t.Skip("TODO: Implement reconnection logic in client")
}

// TestE2E_SubscribeNotify tests subscription and notification flow.
func TestE2E_SubscribeNotify(t *testing.T) {
	// This test requires:
	// - Subscription protocol messages
	// - Server-side subscription management
	// - Notification delivery

	t.Skip("TODO: Wire up subscription protocol messages")
}

// TestE2E_ReadWrite tests read and write operations.
func TestE2E_ReadWrite(t *testing.T) {
	// This test requires:
	// - Read/Write protocol messages
	// - Server-side attribute handling
	// - Feature model integration

	t.Skip("TODO: Wire up read/write protocol messages")
}

// Helper functions

// generateSelfSignedCert generates a self-signed certificate for testing.
func generateSelfSignedCert(commonName string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{commonName, "localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Encode to PEM for tls.X509KeyPair
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}

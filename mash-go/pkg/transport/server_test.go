package transport_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/transport"
)

// TestServerTLS13Only verifies the server only accepts TLS 1.3 connections.
func TestServerTLS13Only(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	// Create CA pool
	caPool := x509.NewCertPool()
	caPool.AddCert(parseCert(t, serverCert))
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	// Start server
	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate: serverTLSCert,
			ClientCAs:   clientCAPool,
		},
		Address: "127.0.0.1:0", // Random port
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Try to connect with TLS 1.2 - should fail
	tlsConfig12 := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS12,
		Certificates:       []tls.Certificate{clientTLSCert},
		RootCAs:            caPool,
		InsecureSkipVerify: true, // For testing
	}

	conn12, err := tls.Dial("tcp", addr.String(), tlsConfig12)
	if err == nil {
		conn12.Close()
		t.Error("TLS 1.2 connection should have been rejected")
	}

	// Connect with TLS 1.3 - should succeed
	tlsConfig13 := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		RootCAs:            caPool,
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	conn13, err := tls.Dial("tcp", addr.String(), tlsConfig13)
	if err != nil {
		t.Fatalf("TLS 1.3 connection failed: %v", err)
	}
	defer conn13.Close()

	// Verify TLS 1.3
	state := conn13.ConnectionState()
	if state.Version != tls.VersionTLS13 {
		t.Errorf("Expected TLS 1.3, got version %x", state.Version)
	}
}

// TestServerALPN verifies the server requires ALPN "mash/1".
func TestServerALPN(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	caPool := x509.NewCertPool()
	caPool.AddCert(parseCert(t, serverCert))
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate: serverTLSCert,
			ClientCAs:   clientCAPool,
		},
		Address: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect with correct ALPN
	tlsConfigCorrect := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	connCorrect, err := tls.Dial("tcp", addr.String(), tlsConfigCorrect)
	if err != nil {
		t.Fatalf("Connection with correct ALPN failed: %v", err)
	}

	state := connCorrect.ConnectionState()
	if state.NegotiatedProtocol != transport.ALPNProtocol {
		t.Errorf("Expected ALPN %q, got %q", transport.ALPNProtocol, state.NegotiatedProtocol)
	}
	connCorrect.Close()
}

// TestServerCertificate verifies the server presents a valid certificate.
func TestServerCertificate(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	caPool := x509.NewCertPool()
	caPool.AddCert(parseCert(t, serverCert))
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate: serverTLSCert,
			ClientCAs:   clientCAPool,
		},
		Address: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect and verify server certificate
	var receivedCert *x509.Certificate
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{clientTLSCert},
		RootCAs:      caPool,
		NextProtos:   []string{transport.ALPNProtocol},
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) > 0 {
				cert, err := x509.ParseCertificate(rawCerts[0])
				if err == nil {
					receivedCert = cert
				}
			}
			return nil
		},
		InsecureSkipVerify: true, // We verify manually
	}

	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	if receivedCert == nil {
		t.Fatal("Server did not present a certificate")
	}

	// Verify it's the expected certificate
	expectedCert := parseCert(t, serverCert)
	if !receivedCert.Equal(expectedCert) {
		t.Error("Server certificate doesn't match expected")
	}
}

// TestServerMutualTLS verifies the server requests and verifies client certificates.
func TestServerMutualTLS(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	caPool := x509.NewCertPool()
	caPool.AddCert(parseCert(t, serverCert))
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	// Server configured to require client certs
	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate: serverTLSCert,
			ClientCAs:   clientCAPool,
		},
		Address:           "127.0.0.1:0",
		RequireClientCert: true,
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect with client certificate - should succeed
	tlsConfigWithCert := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		RootCAs:            caPool,
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	connWithCert, err := tls.Dial("tcp", addr.String(), tlsConfigWithCert)
	if err != nil {
		t.Fatalf("Connection with client cert failed: %v", err)
	}
	connWithCert.Close()

	// Connect without client certificate - server should reject
	// Note: With TLS 1.3, the dial may succeed but server closes connection
	// after verifying no client cert was provided
	tlsConfigNoCert := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		RootCAs:            caPool,
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	connNoCert, err := tls.Dial("tcp", addr.String(), tlsConfigNoCert)
	if err != nil {
		// Connection failed at dial - this is acceptable
		return
	}
	defer connNoCert.Close()

	// Connection established, but server should close it
	// Try to send a message - should fail because server closed
	framer := transport.NewFramer(connNoCert)
	testMsg := []byte("test")
	if err := framer.WriteFrame(testMsg); err != nil {
		// Write failed - server closed connection, which is expected
		return
	}

	// If write succeeded, try to read - server should have closed
	connNoCert.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = framer.ReadFrame()
	if err == nil {
		t.Error("Connection without client cert should have been rejected by server")
	}
}

// TestServerFraming verifies the server handles framed messages correctly.
func TestServerFraming(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	var receivedMsg []byte
	var msgMu sync.Mutex
	msgReceived := make(chan struct{})

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		OnMessage: func(conn *transport.ServerConn, msg []byte) {
			msgMu.Lock()
			receivedMsg = msg
			msgMu.Unlock()
			close(msgReceived)
		},
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect client
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	// Send a framed message
	testMsg := []byte("Hello, MASH!")
	framer := transport.NewFramer(conn)
	if err := framer.WriteFrame(testMsg); err != nil {
		t.Fatalf("Failed to send frame: %v", err)
	}

	// Wait for message
	select {
	case <-msgReceived:
		msgMu.Lock()
		if string(receivedMsg) != string(testMsg) {
			t.Errorf("Expected %q, got %q", testMsg, receivedMsg)
		}
		msgMu.Unlock()
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

// TestServerConcurrentConnections verifies the server handles multiple connections.
func TestServerConcurrentConnections(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	var connCount int
	var mu sync.Mutex

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		OnConnect: func(_ *transport.ServerConn) {
			mu.Lock()
			connCount++
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect multiple clients concurrently
	numClients := 5
	var wg sync.WaitGroup
	conns := make([]*tls.Conn, numClients)

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
			if err != nil {
				t.Errorf("Client %d: Connection failed: %v", idx, err)
				return
			}
			conns[idx] = conn
		}(i)
	}

	wg.Wait()

	// Give server time to process connections
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if connCount != numClients {
		t.Errorf("Expected %d connections, got %d", numClients, connCount)
	}
	mu.Unlock()

	// Verify all connections are active
	activeCount := server.ConnectionCount()
	if activeCount != numClients {
		t.Errorf("Expected %d active connections, got %d", numClients, activeCount)
	}

	// Close all connections
	for _, conn := range conns {
		if conn != nil {
			conn.Close()
		}
	}
}

// TestServerKeepAlive verifies the server responds to ping with pong.
func TestServerKeepAlive(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	addr := server.Addr()

	// Connect client
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	framer := transport.NewFramer(conn)

	// Send ping
	pingMsg, _ := transport.EncodePing(42)
	if err := framer.WriteFrame(pingMsg); err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Read pong
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	response, err := framer.ReadFrame()
	if err != nil {
		t.Fatalf("Failed to read pong: %v", err)
	}

	msgType, seq, err := transport.DecodeControlMessage(response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if msgType != transport.ControlPong {
		t.Errorf("Expected pong, got %v", msgType)
	}
	if seq != 42 {
		t.Errorf("Expected sequence 42, got %d", seq)
	}
}

// Helper functions

func generateTestCert(t *testing.T) ([]byte, []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("Failed to marshal key: %v", err)
	}

	return certDER, keyDER
}

func loadCert(t *testing.T, certDER, keyDER []byte) tls.Certificate {
	t.Helper()

	key, err := x509.ParseECPrivateKey(keyDER)
	if err != nil {
		t.Fatalf("Failed to parse key: %v", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
}

func parseCert(t *testing.T, certDER []byte) *x509.Certificate {
	t.Helper()

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	return cert
}

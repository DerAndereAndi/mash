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

	"github.com/mash-protocol/mash-go/pkg/log"
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

// capturingLogger captures log events for testing.
type capturingLogger struct {
	mu     sync.Mutex
	events []log.Event
}

func (l *capturingLogger) Log(event log.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *capturingLogger) Events() []log.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]log.Event(nil), l.events...)
}

// TestServerPassesLoggerToConnection verifies the server passes logger to connections.
func TestServerPassesLoggerToConnection(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		Logger:  logger,
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

	// Send a message to trigger frame logging
	framer := transport.NewFramer(conn)
	framer.WriteFrame([]byte("test"))

	// Give server time to process
	time.Sleep(100 * time.Millisecond)

	conn.Close()

	events := logger.Events()
	if len(events) == 0 {
		t.Fatal("Expected at least one log event, got none")
	}

	// Should have frame events from server receiving the message
	var foundFrameEvent bool
	for _, e := range events {
		if e.Frame != nil {
			foundFrameEvent = true
			if e.ConnectionID == "" {
				t.Error("Frame event has empty ConnectionID")
			}
			break
		}
	}
	if !foundFrameEvent {
		t.Error("Expected at least one frame event")
	}
}

// TestClientPassesLoggerToConnection verifies the client passes logger to connections.
func TestClientPassesLoggerToConnection(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	// Start a simple echo server
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{serverTLSCert},
		NextProtos:   []string{transport.ALPNProtocol},
		ClientAuth:   tls.NoClientCert,
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Echo server
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		tlsConn.Handshake()
		framer := transport.NewFramer(tlsConn)
		for {
			msg, err := framer.ReadFrame()
			if err != nil {
				return
			}
			framer.WriteFrame(msg)
		}
	}()

	logger := &capturingLogger{}

	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Send a message
	conn.Send([]byte("hello"))

	// Read response
	conn.Receive(time.Second)

	conn.Close()

	events := logger.Events()
	if len(events) == 0 {
		t.Fatal("Expected at least one log event, got none")
	}

	// Should have frame events for both send and receive
	var outEvents, inEvents int
	for _, e := range events {
		if e.Frame != nil {
			if e.Direction == log.DirectionOut {
				outEvents++
			} else if e.Direction == log.DirectionIn {
				inEvents++
			}
		}
	}
	if outEvents == 0 {
		t.Error("Expected at least one outgoing frame event")
	}
	if inEvents == 0 {
		t.Error("Expected at least one incoming frame event")
	}
}

// TestConnectionGeneratesUUID verifies each connection gets a unique ID.
func TestConnectionGeneratesUUID(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		Logger:  logger,
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

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	// Create two connections
	conn1, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection 1 failed: %v", err)
	}
	framer1 := transport.NewFramer(conn1)
	framer1.WriteFrame([]byte("conn1"))
	time.Sleep(50 * time.Millisecond)

	conn2, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection 2 failed: %v", err)
	}
	framer2 := transport.NewFramer(conn2)
	framer2.WriteFrame([]byte("conn2"))
	time.Sleep(50 * time.Millisecond)

	conn1.Close()
	conn2.Close()

	events := logger.Events()

	// Collect unique connection IDs
	connIDs := make(map[string]bool)
	for _, e := range events {
		if e.ConnectionID != "" {
			connIDs[e.ConnectionID] = true
		}
	}

	if len(connIDs) < 2 {
		t.Errorf("Expected at least 2 unique connection IDs, got %d: %v", len(connIDs), connIDs)
	}

	// Verify IDs look like UUIDs (basic format check)
	for id := range connIDs {
		if len(id) < 32 {
			t.Errorf("Connection ID %q doesn't look like a UUID (too short)", id)
		}
	}
}

// TestConnectionLogsPing verifies ping control messages are logged.
func TestConnectionLogsPing(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		Logger:  logger,
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
	framer.WriteFrame(pingMsg)

	// Read pong
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	framer.ReadFrame()

	time.Sleep(100 * time.Millisecond)

	events := logger.Events()

	// Should have control message events for ping received and pong sent
	var foundPingIn, foundPongOut bool
	for _, e := range events {
		if e.ControlMsg != nil {
			if e.Direction == log.DirectionIn && e.ControlMsg.Type == log.ControlMsgPing {
				foundPingIn = true
			}
			if e.Direction == log.DirectionOut && e.ControlMsg.Type == log.ControlMsgPong {
				foundPongOut = true
			}
		}
	}

	if !foundPingIn {
		t.Error("Expected incoming PING control message event")
	}
	if !foundPongOut {
		t.Error("Expected outgoing PONG control message event")
	}
}

// TestConnectionLogsPong verifies pong control messages are logged.
func TestConnectionLogsPong(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	// Create a client that logs
	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
		Logger: logger,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Start a server that echoes pong to ping
	tlsConf := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{serverTLSCert},
		ClientCAs:    clientCAPool,
		NextProtos:   []string{transport.ALPNProtocol},
		ClientAuth:   tls.NoClientCert,
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConf)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	// Server that responds to ping with pong
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		tlsConn.Handshake()
		framer := transport.NewFramer(tlsConn)
		for {
			msg, err := framer.ReadFrame()
			if err != nil {
				return
			}
			// Decode and respond to ping
			if msgType, seq, err := transport.DecodeControlMessage(msg); err == nil {
				if msgType == transport.ControlPing {
					pongMsg, _ := transport.EncodePong(seq)
					framer.WriteFrame(pongMsg)
				}
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send ping
	conn.SendPing(123)

	// Read pong
	conn.Receive(time.Second)

	time.Sleep(100 * time.Millisecond)

	events := logger.Events()

	// Should have control message events for ping sent and pong received
	var foundPingOut, foundPongIn bool
	for _, e := range events {
		if e.ControlMsg != nil {
			if e.Direction == log.DirectionOut && e.ControlMsg.Type == log.ControlMsgPing {
				foundPingOut = true
			}
			if e.Direction == log.DirectionIn && e.ControlMsg.Type == log.ControlMsgPong {
				foundPongIn = true
			}
		}
	}

	if !foundPingOut {
		t.Error("Expected outgoing PING control message event")
	}
	if !foundPongIn {
		t.Error("Expected incoming PONG control message event")
	}
}

// TestConnectionLogsClose verifies close control messages are logged.
func TestConnectionLogsClose(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		Logger:  logger,
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

	framer := transport.NewFramer(conn)

	// Send close message
	closeMsg, _ := transport.EncodeClose()
	framer.WriteFrame(closeMsg)

	time.Sleep(200 * time.Millisecond)
	conn.Close()

	events := logger.Events()

	// Should have control message event for close received
	var foundCloseIn bool
	for _, e := range events {
		if e.ControlMsg != nil && e.ControlMsg.Type == log.ControlMsgClose {
			foundCloseIn = true
			break
		}
	}

	if !foundCloseIn {
		t.Error("Expected CLOSE control message event")
	}
}

// TestConnectionLogsStateChanges verifies state changes are logged.
func TestConnectionLogsStateChanges(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	logger := &capturingLogger{}

	server, err := transport.NewServer(transport.ServerConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        serverTLSCert,
			ClientCAs:          clientCAPool,
			InsecureSkipVerify: true,
		},
		Address: "127.0.0.1:0",
		Logger:  logger,
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

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		Certificates:       []tls.Certificate{clientTLSCert},
		InsecureSkipVerify: true,
		NextProtos:         []string{transport.ALPNProtocol},
	}

	// Connect and disconnect
	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Connection failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	conn.Close()
	time.Sleep(100 * time.Millisecond)

	events := logger.Events()

	// Look for state change events
	var foundConnected, foundDisconnected bool
	for _, e := range events {
		if e.StateChange != nil {
			if e.StateChange.NewState == "CONNECTED" {
				foundConnected = true
			}
			if e.StateChange.NewState == "DISCONNECTED" {
				foundDisconnected = true
			}
		}
	}

	if !foundConnected {
		t.Error("Expected CONNECTED state change event")
	}
	if !foundDisconnected {
		t.Error("Expected DISCONNECTED state change event")
	}
}

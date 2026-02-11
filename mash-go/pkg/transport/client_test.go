package transport_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/transport"
)

// TestClientTLS13Only verifies the client only connects using TLS 1.3.
func TestClientTLS13Only(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	// Start a TLS 1.3 server
	listener := startTestServer(t, serverTLSCert, tls.VersionTLS13)
	defer listener.Close()

	// Create client
	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Connect should succeed with TLS 1.3 server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Verify TLS 1.3
	state := conn.TLSState()
	if state.Version != tls.VersionTLS13 {
		t.Errorf("Expected TLS 1.3, got version %x", state.Version)
	}
}

// TestClientALPN verifies the client negotiates ALPN "mash/1".
func TestClientALPN(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	listener := startTestServer(t, serverTLSCert, tls.VersionTLS13)
	defer listener.Close()

	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
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
	defer conn.Close()

	state := conn.TLSState()
	if state.NegotiatedProtocol != transport.ALPNProtocol {
		t.Errorf("Expected ALPN %q, got %q", transport.ALPNProtocol, state.NegotiatedProtocol)
	}
}

// TestClientCertValidation verifies the client validates server certificates.
func TestClientCertValidation(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	listener := startTestServer(t, serverTLSCert, tls.VersionTLS13)
	defer listener.Close()

	// Create CA pool with server cert
	caPool := x509.NewCertPool()
	caPool.AddCert(parseCert(t, serverCert))

	// Client with proper CA - should succeed
	// Note: ServerName must be set or InsecureSkipVerify must be true
	// for Go TLS to work. We use InsecureSkipVerify since our test cert
	// is self-signed and verification happens via RootCAs.
	clientWithCA, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			RootCAs:            caPool,
			InsecureSkipVerify: true, // Still validates against RootCAs
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := clientWithCA.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Connection with proper CA failed: %v", err)
	}
	conn.Close()

	// Client without CA and without InsecureSkipVerify - should fail
	clientNoCA, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate: clientTLSCert,
			// No RootCAs, no InsecureSkipVerify
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, err = clientNoCA.Connect(ctx2, listener.Addr().String())
	if err == nil {
		t.Error("Connection without CA should have failed certificate validation")
	}
}

// TestClientMutualTLS verifies the client presents its certificate to the server.
func TestClientMutualTLS(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	// Server that requires client certs
	clientCAPool := x509.NewCertPool()
	clientCAPool.AddCert(parseCert(t, clientCert))

	listener := startMutualTLSServer(t, serverTLSCert, clientCAPool)
	defer listener.Close()

	var receivedClientCert *x509.Certificate
	var certMu sync.Mutex

	// Accept one connection and check client cert
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			return
		}

		state := tlsConn.ConnectionState()
		certMu.Lock()
		if len(state.PeerCertificates) > 0 {
			receivedClientCert = state.PeerCertificates[0]
		}
		certMu.Unlock()
	}()

	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
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
	conn.Close()

	// Give server time to process
	time.Sleep(100 * time.Millisecond)

	certMu.Lock()
	if receivedClientCert == nil {
		t.Error("Server did not receive client certificate")
	} else {
		expectedCert := parseCert(t, clientCert)
		if !receivedClientCert.Equal(expectedCert) {
			t.Error("Server received different client certificate")
		}
	}
	certMu.Unlock()
}

// TestClientCommissioningMode verifies the client can connect in commissioning mode.
func TestClientCommissioningMode(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)

	listener := startTestServer(t, serverTLSCert, tls.VersionTLS13)
	defer listener.Close()

	// Commissioning mode - no client cert, skip server verification
	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			InsecureSkipVerify: true,
		},
		CommissioningMode: true,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Commissioning mode connection failed: %v", err)
	}
	defer conn.Close()

	// Should still be TLS 1.3
	state := conn.TLSState()
	if state.Version != tls.VersionTLS13 {
		t.Errorf("Expected TLS 1.3, got version %x", state.Version)
	}

	// Should negotiate commissioning ALPN
	if state.NegotiatedProtocol != transport.ALPNCommissioningProtocol {
		t.Errorf("Expected ALPN %q, got %q", transport.ALPNCommissioningProtocol, state.NegotiatedProtocol)
	}
}

// TestClientReconnection verifies the client can reconnect after disconnection.
func TestClientReconnection(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	listener := startTestServer(t, serverTLSCert, tls.VersionTLS13)
	defer listener.Close()

	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First connection
	conn1, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("First connection failed: %v", err)
	}

	// Close connection
	conn1.Close()

	// Second connection - should work
	conn2, err := client.Connect(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Reconnection failed: %v", err)
	}
	defer conn2.Close()

	// Verify it's a new connection
	if conn1 == conn2 {
		t.Error("Expected new connection object")
	}
}

// TestClientSendReceive verifies the client can send and receive messages.
func TestClientSendReceive(t *testing.T) {
	serverCert, serverKey := generateTestCert(t)
	clientCert, clientKey := generateTestCert(t)

	serverTLSCert := loadCert(t, serverCert, serverKey)
	clientTLSCert := loadCert(t, clientCert, clientKey)

	// Create our own listener for this test (not using startTestServer)
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

	// Server goroutine - echo messages
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		tlsConn := conn.(*tls.Conn)
		if err := tlsConn.Handshake(); err != nil {
			return
		}

		framer := transport.NewFramer(tlsConn)
		for {
			msg, err := framer.ReadFrame()
			if err != nil {
				return
			}
			// Echo back
			if err := framer.WriteFrame(msg); err != nil {
				return
			}
		}
	}()

	client, err := transport.NewClient(transport.ClientConfig{
		TLSConfig: &transport.TLSConfig{
			Certificate:        clientTLSCert,
			InsecureSkipVerify: true,
		},
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
	defer conn.Close()

	// Send message
	testMsg := []byte("Hello, MASH!")
	if err := conn.Send(testMsg); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	// Receive echo
	response, err := conn.Receive(2 * time.Second)
	if err != nil {
		t.Fatalf("Failed to receive: %v", err)
	}

	if string(response) != string(testMsg) {
		t.Errorf("Expected %q, got %q", testMsg, response)
	}
}

// Helper functions

func startTestServer(t *testing.T, cert tls.Certificate, tlsVersion uint16) net.Listener {
	t.Helper()

	tlsConfig := &tls.Config{
		MinVersion:   tlsVersion,
		MaxVersion:   tlsVersion,
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{transport.ALPNProtocol, transport.ALPNCommissioningProtocol},
		ClientAuth:   tls.NoClientCert,
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Keep connection open
			go func(c net.Conn) {
				tlsConn := c.(*tls.Conn)
				tlsConn.Handshake()
				// Hold connection open until it's closed
				buf := make([]byte, 1024)
				for {
					_, err := c.Read(buf)
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	return listener
}

func startMutualTLSServer(t *testing.T, cert tls.Certificate, clientCAs *x509.CertPool) net.Listener {
	t.Helper()

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		NextProtos:   []string{transport.ALPNProtocol},
	}

	listener, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	return listener
}

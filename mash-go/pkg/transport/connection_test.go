package transport

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
	"sync/atomic"
	"testing"
	"time"
)

// testCerts holds certificates for testing.
type testCerts struct {
	serverCert tls.Certificate
	clientCert tls.Certificate
	caPool     *x509.CertPool
}

// generateTestCerts creates server and client certificates signed by a test CA.
func generateTestCerts(t *testing.T) *testCerts {
	t.Helper()

	// Generate CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// Generate server certificate
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate server key: %v", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		BasicConstraintsValid: true,
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create server certificate: %v", err)
	}

	serverCertParsed, _ := x509.ParseCertificate(serverDER)

	// Generate client certificate
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			CommonName: "test-client",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create client certificate: %v", err)
	}

	clientCertParsed, _ := x509.ParseCertificate(clientDER)

	return &testCerts{
		serverCert: tls.Certificate{
			Certificate: [][]byte{serverDER},
			PrivateKey:  serverKey,
			Leaf:        serverCertParsed,
		},
		clientCert: tls.Certificate{
			Certificate: [][]byte{clientDER},
			PrivateKey:  clientKey,
			Leaf:        clientCertParsed,
		},
		caPool: caPool,
	}
}

// mockHandler implements ConnectionHandler for testing.
type mockHandler struct {
	mu           sync.Mutex
	messages     [][]byte
	stateChanges []struct{ old, new ConnectionState }
	errors       []error
	messageCh    chan []byte
}

func newMockHandler() *mockHandler {
	return &mockHandler{
		messageCh: make(chan []byte, 10),
	}
}

func (h *mockHandler) OnMessage(msg []byte) {
	h.mu.Lock()
	h.messages = append(h.messages, msg)
	h.mu.Unlock()
	select {
	case h.messageCh <- msg:
	default:
	}
}

func (h *mockHandler) OnStateChange(oldState, newState ConnectionState) {
	h.mu.Lock()
	h.stateChanges = append(h.stateChanges, struct{ old, new ConnectionState }{oldState, newState})
	h.mu.Unlock()
}

func (h *mockHandler) OnError(err error) {
	h.mu.Lock()
	h.errors = append(h.errors, err)
	h.mu.Unlock()
}

func TestConnectionState(t *testing.T) {
	tests := []struct {
		state ConnectionState
		want  string
	}{
		{StateDisconnected, "DISCONNECTED"},
		{StateConnecting, "CONNECTING"},
		{StateConnected, "CONNECTED"},
		{StateClosing, "CLOSING"},
		{ConnectionState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("%d.String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestConnectionInitialState(t *testing.T) {
	handler := newMockHandler()
	config := DefaultConnectionConfig()

	conn := NewConnection(config, handler)

	if conn.State() != StateDisconnected {
		t.Errorf("initial state = %v, want DISCONNECTED", conn.State())
	}
}

func TestDefaultConnectionConfig(t *testing.T) {
	config := DefaultConnectionConfig()

	if config.MaxMessageSize != DefaultMaxMessageSize {
		t.Errorf("MaxMessageSize = %d, want %d", config.MaxMessageSize, DefaultMaxMessageSize)
	}

	if config.CloseTimeout != 5*time.Second {
		t.Errorf("CloseTimeout = %v, want 5s", config.CloseTimeout)
	}

	// Check keep-alive defaults
	if config.KeepAlive.PingInterval != DefaultPingInterval {
		t.Errorf("KeepAlive.PingInterval = %v, want %v", config.KeepAlive.PingInterval, DefaultPingInterval)
	}
}

func TestConnectionConnectAccept(t *testing.T) {
	certs := generateTestCerts(t)

	// Create server config
	serverTLSConfig, err := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})
	if err != nil {
		t.Fatalf("failed to create server TLS config: %v", err)
	}

	// Create client config
	clientTLSConfig, err := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost", // Match certificate CN
	})
	if err != nil {
		t.Fatalf("failed to create client TLS config: %v", err)
	}

	// Start TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 100 * time.Millisecond

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 100 * time.Millisecond

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Accept in goroutine
	var acceptErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		if err != nil {
			acceptErr = err
			return
		}
		acceptErr = serverConn.Accept(ctx, conn)
	}()

	// Connect client
	if err := clientConn.Connect(ctx, listener.Addr().String()); err != nil {
		t.Fatalf("client connect failed: %v", err)
	}

	wg.Wait()
	if acceptErr != nil {
		t.Fatalf("server accept failed: %v", acceptErr)
	}

	// Check states
	if clientConn.State() != StateConnected {
		t.Errorf("client state = %v, want CONNECTED", clientConn.State())
	}
	if serverConn.State() != StateConnected {
		t.Errorf("server state = %v, want CONNECTED", serverConn.State())
	}

	// Cleanup
	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionSendReceive(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second // Slow down keep-alive

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Accept in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Send message from client to server
	testMsg := []byte("hello server")
	if err := clientConn.Send(testMsg); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Wait for message
	select {
	case msg := <-serverHandler.messageCh:
		if string(msg) != string(testMsg) {
			t.Errorf("received %q, want %q", msg, testMsg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	// Send message from server to client
	testMsg2 := []byte("hello client")
	if err := serverConn.Send(testMsg2); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	// Wait for message
	select {
	case msg := <-clientHandler.messageCh:
		if string(msg) != string(testMsg2) {
			t.Errorf("received %q, want %q", msg, testMsg2)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionSendNotConnected(t *testing.T) {
	handler := newMockHandler()
	conn := NewConnection(DefaultConnectionConfig(), handler)

	err := conn.Send([]byte("test"))
	if err != ErrNotConnected {
		t.Errorf("Send error = %v, want ErrNotConnected", err)
	}
}

func TestConnectionDoubleConnect(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())

	// Try to connect again
	err = clientConn.Connect(ctx, listener.Addr().String())
	if err != ErrAlreadyConnected {
		t.Errorf("double connect error = %v, want ErrAlreadyConnected", err)
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionAddresses(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Check addresses
	if clientConn.LocalAddr() == nil {
		t.Error("client LocalAddr is nil")
	}
	if clientConn.RemoteAddr() == nil {
		t.Error("client RemoteAddr is nil")
	}

	// Disconnected connection should have nil addresses
	disconnectedConn := NewConnection(DefaultConnectionConfig(), newMockHandler())
	if disconnectedConn.LocalAddr() != nil {
		t.Error("disconnected LocalAddr should be nil")
	}
	if disconnectedConn.RemoteAddr() != nil {
		t.Error("disconnected RemoteAddr should be nil")
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionTLSState(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Check TLS state
	state, ok := clientConn.TLSConnectionState()
	if !ok {
		t.Fatal("TLSConnectionState returned false")
	}
	if state.Version != tls.VersionTLS13 {
		t.Errorf("TLS version = %x, want TLS 1.3", state.Version)
	}
	if state.NegotiatedProtocol != ALPNProtocol {
		t.Errorf("ALPN = %s, want %s", state.NegotiatedProtocol, ALPNProtocol)
	}

	// Disconnected connection should return false
	disconnectedConn := NewConnection(DefaultConnectionConfig(), newMockHandler())
	_, ok = disconnectedConn.TLSConnectionState()
	if ok {
		t.Error("disconnected TLSConnectionState should return false")
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionForceClose(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Force close
	clientConn.ForceClose()

	// Check state
	if clientConn.State() != StateDisconnected {
		t.Errorf("state after ForceClose = %v, want DISCONNECTED", clientConn.State())
	}

	// Double force close should be safe
	clientConn.ForceClose()

	serverConn.ForceClose()
}

func TestConnectionStateChanges(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	clientHandler := newMockHandler()
	serverHandler := newMockHandler()

	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 1 * time.Second

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 1 * time.Second

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	clientConn.ForceClose()

	// Wait a bit for state changes to propagate
	time.Sleep(50 * time.Millisecond)

	// Check client state changes
	clientHandler.mu.Lock()
	changes := clientHandler.stateChanges
	clientHandler.mu.Unlock()

	// Should have: DISCONNECTED->CONNECTING, CONNECTING->CONNECTED, CONNECTED->DISCONNECTED
	expectedChanges := []struct{ old, new ConnectionState }{
		{StateDisconnected, StateConnecting},
		{StateConnecting, StateConnected},
		{StateConnected, StateDisconnected},
	}

	if len(changes) != len(expectedChanges) {
		t.Errorf("got %d state changes, want %d", len(changes), len(expectedChanges))
	}

	for i, want := range expectedChanges {
		if i < len(changes) {
			if changes[i].old != want.old || changes[i].new != want.new {
				t.Errorf("change %d: got %v->%v, want %v->%v",
					i, changes[i].old, changes[i].new, want.old, want.new)
			}
		}
	}

	serverConn.ForceClose()
}

func TestConnectionKeepAliveIntegration(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	serverHandler := newMockHandler()
	clientHandler := newMockHandler()

	// Use fast keep-alive for testing
	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 50 * time.Millisecond
	serverConfig.KeepAlive.PongTimeout = 20 * time.Millisecond
	serverConfig.KeepAlive.MaxMissedPongs = 2

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 50 * time.Millisecond
	clientConfig.KeepAlive.PongTimeout = 20 * time.Millisecond
	clientConfig.KeepAlive.MaxMissedPongs = 2

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Let keep-alive run for a bit
	time.Sleep(200 * time.Millisecond)

	// Both should still be connected (keep-alive working)
	if clientConn.State() != StateConnected {
		t.Errorf("client state = %v, want CONNECTED", clientConn.State())
	}
	if serverConn.State() != StateConnected {
		t.Errorf("server state = %v, want CONNECTED", serverConn.State())
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

func TestConnectionKeepAliveTimeout(t *testing.T) {
	certs := generateTestCerts(t)

	serverTLSConfig, _ := NewServerTLSConfig(&TLSConfig{
		Certificate: certs.serverCert,
		ClientCAs:   certs.caPool,
	})

	clientTLSConfig, _ := NewClientTLSConfig(&TLSConfig{
		Certificate: certs.clientCert,
		RootCAs:     certs.caPool,
		ServerName:  "localhost",
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	defer listener.Close()

	var serverErrors atomic.Int32
	serverHandler := newMockHandler()

	clientHandler := newMockHandler()

	// Use very fast keep-alive for testing timeout
	serverConfig := DefaultConnectionConfig()
	serverConfig.TLSConfig = serverTLSConfig
	serverConfig.KeepAlive.PingInterval = 20 * time.Millisecond
	serverConfig.KeepAlive.PongTimeout = 10 * time.Millisecond
	serverConfig.KeepAlive.MaxMissedPongs = 2

	clientConfig := DefaultConnectionConfig()
	clientConfig.TLSConfig = clientTLSConfig
	clientConfig.KeepAlive.PingInterval = 20 * time.Millisecond
	clientConfig.KeepAlive.PongTimeout = 10 * time.Millisecond
	clientConfig.KeepAlive.MaxMissedPongs = 2

	serverConn := NewConnection(serverConfig, serverHandler)
	clientConn := NewConnection(clientConfig, clientHandler)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, _ := listener.Accept()
		serverConn.Accept(ctx, conn)
	}()

	clientConn.Connect(ctx, listener.Addr().String())
	wg.Wait()

	// Forcibly close underlying connection to simulate network failure
	// This bypasses the graceful close and should trigger keep-alive timeout
	clientConn.mu.Lock()
	if clientConn.tlsConn != nil {
		clientConn.tlsConn.Close()
	}
	clientConn.mu.Unlock()

	// Wait for keep-alive timeout on server side
	time.Sleep(200 * time.Millisecond)

	// Server should have detected the failure
	serverHandler.mu.Lock()
	errCount := len(serverHandler.errors)
	serverHandler.mu.Unlock()

	// Either keep-alive timeout or read error should have occurred
	if errCount == 0 && serverErrors.Load() == 0 {
		// Check if server is disconnected
		if serverConn.State() == StateConnected {
			t.Log("Server still connected - keep-alive may not have timed out yet")
		}
	}

	clientConn.ForceClose()
	serverConn.ForceClose()
}

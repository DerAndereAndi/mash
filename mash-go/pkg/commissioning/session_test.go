package commissioning_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// mockConn implements a simple in-memory connection for testing.
type mockConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func newMockConnPair() (*mockConn, *mockConn) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()

	return &mockConn{reader: r1, writer: w2},
		&mockConn{reader: r2, writer: w1}
}

func (c *mockConn) Read(p []byte) (int, error)  { return c.reader.Read(p) }
func (c *mockConn) Write(p []byte) (int, error) { return c.writer.Write(p) }
func (c *mockConn) Close() error {
	c.reader.Close()
	c.writer.Close()
	return nil
}
func (c *mockConn) LocalAddr() net.Addr                { return nil }
func (c *mockConn) RemoteAddr() net.Addr               { return nil }
func (c *mockConn) SetDeadline(t time.Time) error      { return nil }
func (c *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestPASESessionSuccess verifies successful PASE handshake between client and server.
func TestPASESessionSuccess(t *testing.T) {
	// Setup
	setupCode := commissioning.MustParseSetupCode("12345678")
	clientIdentity := []byte("controller-001")
	serverIdentity := []byte("device-001")

	// Device generates verifier from setup code (done during manufacturing)
	verifier, err := commissioning.GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to generate verifier: %v", err)
	}

	// Create mock connections
	clientConn, serverConn := newMockConnPair()
	defer clientConn.Close()
	defer serverConn.Close()

	// Create sessions
	clientSession, err := commissioning.NewPASEClientSession(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}

	serverSession, err := commissioning.NewPASEServerSession(verifier, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to create server session: %v", err)
	}

	// Run handshake concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var clientErr, serverErr error
	var clientKey, serverKey []byte

	wg.Add(2)

	// Client side
	go func() {
		defer wg.Done()
		clientKey, clientErr = clientSession.Handshake(ctx, clientConn)
	}()

	// Server side
	go func() {
		defer wg.Done()
		serverKey, serverErr = serverSession.Handshake(ctx, serverConn)
	}()

	wg.Wait()

	// Check results
	if clientErr != nil {
		t.Fatalf("Client handshake failed: %v", clientErr)
	}
	if serverErr != nil {
		t.Fatalf("Server handshake failed: %v", serverErr)
	}

	// Verify both sides derived the same key
	if !bytes.Equal(clientKey, serverKey) {
		t.Error("Client and server derived different session keys")
	}

	if len(clientKey) != commissioning.SharedSecretSize {
		t.Errorf("Session key wrong size: expected %d, got %d", commissioning.SharedSecretSize, len(clientKey))
	}
}

// TestPASESessionWrongPassword verifies PASE fails with wrong setup code.
func TestPASESessionWrongPassword(t *testing.T) {
	// Setup with correct code
	correctCode := commissioning.MustParseSetupCode("12345678")
	wrongCode := commissioning.MustParseSetupCode("87654321")
	clientIdentity := []byte("controller-001")
	serverIdentity := []byte("device-001")

	// Device has verifier for correct code
	verifier, err := commissioning.GenerateVerifier(correctCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to generate verifier: %v", err)
	}

	// Create mock connections
	clientConn, serverConn := newMockConnPair()
	defer clientConn.Close()
	defer serverConn.Close()

	// Client uses wrong code
	clientSession, _ := commissioning.NewPASEClientSession(wrongCode, clientIdentity, serverIdentity)
	serverSession, _ := commissioning.NewPASEServerSession(verifier, serverIdentity)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var clientErr, serverErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		_, clientErr = clientSession.Handshake(ctx, clientConn)
	}()

	go func() {
		defer wg.Done()
		_, serverErr = serverSession.Handshake(ctx, serverConn)
	}()

	wg.Wait()

	// At least one side should fail
	if clientErr == nil && serverErr == nil {
		t.Error("Expected handshake to fail with wrong password")
	}
}

// TestPASESessionTimeout verifies PASE times out properly.
func TestPASESessionTimeout(t *testing.T) {
	setupCode := commissioning.MustParseSetupCode("12345678")
	clientIdentity := []byte("controller-001")
	serverIdentity := []byte("device-001")

	// Create connection pair but only use client side (server won't respond)
	clientConn, serverConn := newMockConnPair()
	defer clientConn.Close()
	defer serverConn.Close()

	clientSession, err := commissioning.NewPASEClientSession(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}

	// Start server session but don't run handshake - just read and discard
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := serverConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = clientSession.Handshake(ctx, clientConn)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

// TestPASEClientIdentity verifies client identity is properly included.
func TestPASEClientIdentity(t *testing.T) {
	setupCode := commissioning.MustParseSetupCode("12345678")
	clientIdentity := []byte("controller-test-identity")
	serverIdentity := []byte("device-test-identity")

	clientSession, err := commissioning.NewPASEClientSession(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to create client session: %v", err)
	}

	if !bytes.Equal(clientSession.ClientIdentity(), clientIdentity) {
		t.Error("Client identity not preserved")
	}
}

// TestVerifierPersistence verifies verifier can be serialized and restored.
func TestVerifierPersistence(t *testing.T) {
	setupCode := commissioning.MustParseSetupCode("12345678")
	clientIdentity := []byte("controller-001")
	serverIdentity := []byte("device-001")

	// Generate verifier
	verifier, err := commissioning.GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("Failed to generate verifier: %v", err)
	}

	// Serialize
	data, err := commissioning.MarshalVerifier(verifier)
	if err != nil {
		t.Fatalf("Failed to marshal verifier: %v", err)
	}

	// Restore
	restored, err := commissioning.UnmarshalVerifier(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal verifier: %v", err)
	}

	// Verify equality
	if !bytes.Equal(verifier.W0, restored.W0) {
		t.Error("W0 mismatch after restore")
	}
	if !bytes.Equal(verifier.L, restored.L) {
		t.Error("L mismatch after restore")
	}
	if !bytes.Equal(verifier.Identity, restored.Identity) {
		t.Error("Identity mismatch after restore")
	}
}

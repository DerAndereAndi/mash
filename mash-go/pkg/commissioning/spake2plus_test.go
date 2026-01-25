package commissioning

import (
	"bytes"
	"testing"
)

func TestSPAKE2PlusBasicExchange(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	// Device: Generate verifier during registration
	verifier, err := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("GenerateVerifier failed: %v", err)
	}

	// Controller: Create client
	client, err := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("NewSPAKE2PlusClient failed: %v", err)
	}

	// Device: Create server
	server, err := NewSPAKE2PlusServer(verifier, serverIdentity)
	if err != nil {
		t.Fatalf("NewSPAKE2PlusServer failed: %v", err)
	}

	// Exchange public values
	pA := client.PublicValue()
	pB := server.PublicValue()

	// Process each other's public values
	if err := server.ProcessClientValue(pA); err != nil {
		t.Fatalf("Server.ProcessClientValue failed: %v", err)
	}
	if err := client.ProcessServerValue(pB); err != nil {
		t.Fatalf("Client.ProcessServerValue failed: %v", err)
	}

	// Verify shared secrets match
	clientSecret := client.SharedSecret()
	serverSecret := server.SharedSecret()

	if !bytes.Equal(clientSecret, serverSecret) {
		t.Errorf("shared secrets don't match:\nclient: %x\nserver: %x", clientSecret, serverSecret)
	}

	// Verify confirmations
	clientConfirm := client.Confirmation()
	serverConfirm := server.Confirmation()

	if err := server.VerifyClientConfirmation(clientConfirm); err != nil {
		t.Errorf("server failed to verify client confirmation: %v", err)
	}
	if err := client.VerifyServerConfirmation(serverConfirm); err != nil {
		t.Errorf("client failed to verify server confirmation: %v", err)
	}
}

func TestSPAKE2PlusWrongPassword(t *testing.T) {
	correctCode := SetupCode(12345678)
	wrongCode := SetupCode(87654321)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	// Device: Generate verifier with correct code
	verifier, err := GenerateVerifier(correctCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("GenerateVerifier failed: %v", err)
	}

	// Controller: Create client with WRONG code
	client, err := NewSPAKE2PlusClient(wrongCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("NewSPAKE2PlusClient failed: %v", err)
	}

	// Device: Create server
	server, err := NewSPAKE2PlusServer(verifier, serverIdentity)
	if err != nil {
		t.Fatalf("NewSPAKE2PlusServer failed: %v", err)
	}

	// Exchange public values
	pA := client.PublicValue()
	pB := server.PublicValue()

	// Process each other's public values (this succeeds)
	if err := server.ProcessClientValue(pA); err != nil {
		t.Fatalf("Server.ProcessClientValue failed: %v", err)
	}
	if err := client.ProcessServerValue(pB); err != nil {
		t.Fatalf("Client.ProcessServerValue failed: %v", err)
	}

	// Shared secrets should NOT match
	clientSecret := client.SharedSecret()
	serverSecret := server.SharedSecret()

	if bytes.Equal(clientSecret, serverSecret) {
		t.Error("shared secrets should not match with wrong password")
	}

	// Confirmations should fail
	clientConfirm := client.Confirmation()
	serverConfirm := server.Confirmation()

	if err := server.VerifyClientConfirmation(clientConfirm); err == nil {
		t.Error("server should reject client confirmation with wrong password")
	}
	if err := client.VerifyServerConfirmation(serverConfirm); err == nil {
		t.Error("client should reject server confirmation with wrong password")
	}
}

func TestSPAKE2PlusReplay(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)

	// First exchange
	client1, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	server1, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

	pA1 := client1.PublicValue()
	pB1 := server1.PublicValue()

	server1.ProcessClientValue(pA1)
	client1.ProcessServerValue(pB1)

	// Second exchange with same verifier but new ephemeral keys
	client2, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	server2, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

	pA2 := client2.PublicValue()
	pB2 := server2.PublicValue()

	server2.ProcessClientValue(pA2)
	client2.ProcessServerValue(pB2)

	// Public values should differ (ephemeral keys are random)
	if bytes.Equal(pA1, pA2) {
		t.Error("client public values should differ between sessions")
	}
	if bytes.Equal(pB1, pB2) {
		t.Error("server public values should differ between sessions")
	}

	// Shared secrets should differ
	if bytes.Equal(client1.SharedSecret(), client2.SharedSecret()) {
		t.Error("shared secrets should differ between sessions")
	}
}

func TestSPAKE2PlusInvalidPublicKey(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	client, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	server, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

	// Get valid public value first
	_ = client.PublicValue()
	_ = server.PublicValue()

	// Try invalid public key (too short)
	err := server.ProcessClientValue([]byte{0x01, 0x02, 0x03})
	if err != ErrInvalidPublicKey {
		t.Errorf("expected ErrInvalidPublicKey, got %v", err)
	}

	err = client.ProcessServerValue([]byte{0x01, 0x02, 0x03})
	if err != ErrInvalidPublicKey {
		t.Errorf("expected ErrInvalidPublicKey, got %v", err)
	}
}

func TestSPAKE2PlusNilVerifier(t *testing.T) {
	_, err := NewSPAKE2PlusServer(nil, []byte("device"))
	if err != ErrInvalidVerifier {
		t.Errorf("expected ErrInvalidVerifier, got %v", err)
	}
}

func TestGenerateVerifier(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, err := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		t.Fatalf("GenerateVerifier failed: %v", err)
	}

	// Check verifier components
	if len(verifier.W0) == 0 {
		t.Error("W0 should not be empty")
	}
	if len(verifier.L) == 0 {
		t.Error("L should not be empty")
	}
	if !bytes.Equal(verifier.Identity, clientIdentity) {
		t.Error("Identity should match clientIdentity")
	}

	// L should be a compressed point (33 bytes for P-256)
	if len(verifier.L) != 33 {
		t.Errorf("L should be 33 bytes (compressed point), got %d", len(verifier.L))
	}

	// Same inputs should produce same verifier
	verifier2, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	if !bytes.Equal(verifier.W0, verifier2.W0) {
		t.Error("same inputs should produce same W0")
	}
	if !bytes.Equal(verifier.L, verifier2.L) {
		t.Error("same inputs should produce same L")
	}

	// Different setup code should produce different verifier
	verifier3, _ := GenerateVerifier(SetupCode(87654321), clientIdentity, serverIdentity)
	if bytes.Equal(verifier.W0, verifier3.W0) {
		t.Error("different setup code should produce different W0")
	}
}

func TestSPAKE2PlusPublicValueIdempotent(t *testing.T) {
	setupCode := SetupCode(12345678)
	client, _ := NewSPAKE2PlusClient(setupCode, []byte("c"), []byte("s"))

	// PublicValue should return the same value on multiple calls
	pA1 := client.PublicValue()
	pA2 := client.PublicValue()

	if !bytes.Equal(pA1, pA2) {
		t.Error("PublicValue should return the same value on multiple calls")
	}
}

func TestSPAKE2PlusSharedSecretSize(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	client, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	server, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

	pA := client.PublicValue()
	pB := server.PublicValue()

	server.ProcessClientValue(pA)
	client.ProcessServerValue(pB)

	if len(client.SharedSecret()) != SharedSecretSize {
		t.Errorf("client shared secret size = %d, want %d", len(client.SharedSecret()), SharedSecretSize)
	}
	if len(server.SharedSecret()) != SharedSecretSize {
		t.Errorf("server shared secret size = %d, want %d", len(server.SharedSecret()), SharedSecretSize)
	}
}

func TestSPAKE2PlusConfirmationSize(t *testing.T) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)
	client, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	server, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

	pA := client.PublicValue()
	pB := server.PublicValue()

	server.ProcessClientValue(pA)
	client.ProcessServerValue(pB)

	if len(client.Confirmation()) != ConfirmationSize {
		t.Errorf("client confirmation size = %d, want %d", len(client.Confirmation()), ConfirmationSize)
	}
	if len(server.Confirmation()) != ConfirmationSize {
		t.Errorf("server confirmation size = %d, want %d", len(server.Confirmation()), ConfirmationSize)
	}
}

func BenchmarkSPAKE2PlusExchange(b *testing.B) {
	setupCode := SetupCode(12345678)
	clientIdentity := []byte("controller")
	serverIdentity := []byte("device")

	verifier, _ := GenerateVerifier(setupCode, clientIdentity, serverIdentity)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client, _ := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
		server, _ := NewSPAKE2PlusServer(verifier, serverIdentity)

		pA := client.PublicValue()
		pB := server.PublicValue()

		server.ProcessClientValue(pA)
		client.ProcessServerValue(pB)

		_ = client.Confirmation()
		_ = server.Confirmation()
	}
}

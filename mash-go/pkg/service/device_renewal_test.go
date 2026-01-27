package service

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// TestDeviceRenewalHandler_HandleRequest verifies CSR generation.
func TestDeviceRenewalHandler_HandleRequest(t *testing.T) {
	// Create a device renewal handler with test identity
	identity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	handler := NewDeviceRenewalHandler(identity)

	// Create renewal request
	nonce := make([]byte, 32)
	for i := range nonce {
		nonce[i] = byte(i)
	}

	req := &commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	}

	// Handle the request
	resp, err := handler.HandleRenewalRequest(req)
	if err != nil {
		t.Fatalf("HandleRenewalRequest failed: %v", err)
	}

	// Verify response type
	if resp.MsgType != commissioning.MsgCertRenewalCSR {
		t.Errorf("Expected MsgType %d, got %d", commissioning.MsgCertRenewalCSR, resp.MsgType)
	}

	// Verify CSR is valid PKCS#10
	csr, err := x509.ParseCertificateRequest(resp.CSR)
	if err != nil {
		t.Fatalf("Failed to parse CSR: %v", err)
	}

	// Verify CSR signature
	if err := csr.CheckSignature(); err != nil {
		t.Errorf("CSR signature invalid: %v", err)
	}

	// Verify CSR contains expected identity
	if csr.Subject.CommonName != identity.DeviceID {
		t.Errorf("Expected CN %s, got %s", identity.DeviceID, csr.Subject.CommonName)
	}

	// Verify handler now has a pending key pair
	if handler.pendingKeyPair == nil {
		t.Error("Expected pendingKeyPair to be set")
	}

	// Verify the CSR public key matches the pending key pair
	csrPub, ok := csr.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("CSR public key is not ECDSA")
	}
	if !publicKeysEqual(csrPub, handler.pendingKeyPair.PublicKey) {
		t.Error("CSR public key does not match pending key pair")
	}
}

// TestDeviceRenewalHandler_HandleInstall verifies certificate installation.
func TestDeviceRenewalHandler_HandleInstall(t *testing.T) {
	identity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	handler := NewDeviceRenewalHandler(identity)

	// First, trigger renewal to generate new key pair
	nonce := make([]byte, 32)
	csrResp, err := handler.HandleRenewalRequest(&commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	})
	if err != nil {
		t.Fatalf("HandleRenewalRequest failed: %v", err)
	}

	// Sign CSR with a test Zone CA
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}

	newCert, err := cert.SignCSR(zoneCA, csrResp.CSR)
	if err != nil {
		t.Fatalf("Failed to sign CSR: %v", err)
	}

	// Install new certificate
	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  newCert.Raw,
		Sequence: 2,
	}

	ack, err := handler.HandleCertInstall(install)
	if err != nil {
		t.Fatalf("HandleCertInstall failed: %v", err)
	}

	// Verify success
	if ack.Status != commissioning.RenewalStatusSuccess {
		t.Errorf("Expected status 0 (success), got %d", ack.Status)
	}
	if ack.ActiveSequence != 2 {
		t.Errorf("Expected sequence 2, got %d", ack.ActiveSequence)
	}

	// Verify pending state is cleared
	if handler.pendingKeyPair != nil {
		t.Error("Expected pendingKeyPair to be cleared after install")
	}

	// Verify new cert is accessible
	activeCert := handler.ActiveCert()
	if activeCert == nil {
		t.Fatal("Expected active cert to be set")
	}
	if !bytes.Equal(activeCert.Raw, newCert.Raw) {
		t.Error("Active cert does not match installed cert")
	}
}

// TestDeviceRenewalHandler_RejectInvalidCert verifies wrong-key rejection.
func TestDeviceRenewalHandler_RejectInvalidCert(t *testing.T) {
	identity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	handler := NewDeviceRenewalHandler(identity)

	// Trigger renewal to generate new key pair
	nonce := make([]byte, 32)
	_, err := handler.HandleRenewalRequest(&commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	})
	if err != nil {
		t.Fatalf("HandleRenewalRequest failed: %v", err)
	}

	// Create a certificate with a DIFFERENT key pair (not matching pending)
	wrongKeyPair, err := cert.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate wrong key pair: %v", err)
	}

	wrongCSR, err := cert.CreateCSR(wrongKeyPair, &cert.CSRInfo{Identity: *identity})
	if err != nil {
		t.Fatalf("Failed to create wrong CSR: %v", err)
	}

	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}

	wrongCert, err := cert.SignCSR(zoneCA, wrongCSR)
	if err != nil {
		t.Fatalf("Failed to sign wrong CSR: %v", err)
	}

	// Try to install cert with wrong public key
	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  wrongCert.Raw,
		Sequence: 2,
	}

	ack, err := handler.HandleCertInstall(install)
	if err != nil {
		t.Fatalf("HandleCertInstall should not return error: %v", err)
	}

	// Should reject with InvalidCert status
	if ack.Status != commissioning.RenewalStatusInvalidCert {
		t.Errorf("Expected status %d (invalid cert), got %d",
			commissioning.RenewalStatusInvalidCert, ack.Status)
	}

	// Pending key pair should still be present (renewal not complete)
	if handler.pendingKeyPair == nil {
		t.Error("Expected pendingKeyPair to remain after rejection")
	}
}

// TestDeviceRenewalHandler_RejectMalformedCert verifies malformed cert handling.
func TestDeviceRenewalHandler_RejectMalformedCert(t *testing.T) {
	identity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	handler := NewDeviceRenewalHandler(identity)

	// Trigger renewal
	nonce := make([]byte, 32)
	_, err := handler.HandleRenewalRequest(&commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	})
	if err != nil {
		t.Fatalf("HandleRenewalRequest failed: %v", err)
	}

	// Try to install garbage data as cert
	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  []byte{0xDE, 0xAD, 0xBE, 0xEF},
		Sequence: 2,
	}

	ack, err := handler.HandleCertInstall(install)
	if err != nil {
		t.Fatalf("HandleCertInstall should not return error: %v", err)
	}

	// Should reject with InvalidCert status
	if ack.Status != commissioning.RenewalStatusInvalidCert {
		t.Errorf("Expected status %d (invalid cert), got %d",
			commissioning.RenewalStatusInvalidCert, ack.Status)
	}
}

// TestDeviceRenewalHandler_NoRenewalInProgress verifies install without request.
func TestDeviceRenewalHandler_NoRenewalInProgress(t *testing.T) {
	identity := &cert.DeviceIdentity{
		DeviceID: "test-device-001",
	}
	handler := NewDeviceRenewalHandler(identity)

	// Try to install without first calling HandleRenewalRequest
	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  []byte{0x30, 0x82, 0x01, 0x00}, // Some data
		Sequence: 1,
	}

	ack, err := handler.HandleCertInstall(install)
	if err != nil {
		t.Fatalf("HandleCertInstall should not return error: %v", err)
	}

	// Should fail - no pending key pair
	if ack.Status == commissioning.RenewalStatusSuccess {
		t.Error("Expected failure when no renewal in progress")
	}
}

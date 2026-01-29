package service

import (
	"crypto/ecdsa"
	"crypto/x509"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// DeviceRenewalHandler handles certificate renewal on the device side.
// It manages the state of an in-progress renewal and performs the
// cryptographic operations (key generation, CSR creation, cert installation).
type DeviceRenewalHandler struct {
	mu sync.Mutex

	// identity is the device's identity for CSR generation.
	identity *cert.DeviceIdentity

	// pendingKeyPair is the new key pair generated during renewal.
	// Set when HandleRenewalRequest is called, cleared on successful install.
	pendingKeyPair *cert.KeyPair

	// renewalNonce is the nonce from the CertRenewalRequest.
	renewalNonce []byte

	// activeCert is the currently active certificate.
	activeCert *x509.Certificate

	// activeKey is the private key for the active certificate.
	activeKey *ecdsa.PrivateKey

	// certSequence is the certificate sequence number.
	certSequence uint32
}

// NewDeviceRenewalHandler creates a new DeviceRenewalHandler.
func NewDeviceRenewalHandler(identity *cert.DeviceIdentity) *DeviceRenewalHandler {
	return &DeviceRenewalHandler{
		identity: identity,
	}
}

// HandleRenewalRequest processes a certificate renewal request.
// It generates a new key pair and returns a CSR.
func (h *DeviceRenewalHandler) HandleRenewalRequest(req *commissioning.CertRenewalRequest) (*commissioning.CertRenewalCSR, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Generate new key pair (kept separate from active keys)
	kp, err := cert.GenerateKeyPair()
	if err != nil {
		return nil, err
	}
	h.pendingKeyPair = kp
	h.renewalNonce = req.Nonce

	// Create CSR with device identity
	csrDER, err := cert.CreateCSR(kp, &cert.CSRInfo{
		Identity: *h.identity,
	})
	if err != nil {
		h.pendingKeyPair = nil
		h.renewalNonce = nil
		return nil, err
	}

	// DEC-047: Compute nonce hash to bind CSR to this renewal request
	nonceHash := commissioning.ComputeNonceHash(req.Nonce)

	return &commissioning.CertRenewalCSR{
		MsgType:   commissioning.MsgCertRenewalCSR,
		CSR:       csrDER,
		NonceHash: nonceHash,
	}, nil
}

// HandleCertInstall processes a new certificate installation.
// It verifies the certificate matches the pending key pair and installs it.
func (h *DeviceRenewalHandler) HandleCertInstall(install *commissioning.CertRenewalInstall) (*commissioning.CertRenewalAck, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if renewal is in progress
	if h.pendingKeyPair == nil {
		return &commissioning.CertRenewalAck{
			MsgType:        commissioning.MsgCertRenewalAck,
			Status:         commissioning.RenewalStatusInstallFailed,
			ActiveSequence: h.certSequence,
		}, nil
	}

	// Parse the new certificate
	newCert, err := x509.ParseCertificate(install.NewCert)
	if err != nil {
		return &commissioning.CertRenewalAck{
			MsgType:        commissioning.MsgCertRenewalAck,
			Status:         commissioning.RenewalStatusInvalidCert,
			ActiveSequence: h.certSequence,
		}, nil
	}

	// Verify certificate's public key matches pending key pair
	certPub, ok := newCert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return &commissioning.CertRenewalAck{
			MsgType:        commissioning.MsgCertRenewalAck,
			Status:         commissioning.RenewalStatusInvalidCert,
			ActiveSequence: h.certSequence,
		}, nil
	}

	if !publicKeysEqual(certPub, h.pendingKeyPair.PublicKey) {
		return &commissioning.CertRenewalAck{
			MsgType:        commissioning.MsgCertRenewalAck,
			Status:         commissioning.RenewalStatusInvalidCert,
			ActiveSequence: h.certSequence,
		}, nil
	}

	// Atomic switch: install new cert and key
	h.activeCert = newCert
	h.activeKey = h.pendingKeyPair.PrivateKey
	h.certSequence = install.Sequence

	// Clear pending state
	h.pendingKeyPair = nil
	h.renewalNonce = nil

	return &commissioning.CertRenewalAck{
		MsgType:        commissioning.MsgCertRenewalAck,
		Status:         commissioning.RenewalStatusSuccess,
		ActiveSequence: install.Sequence,
	}, nil
}

// ActiveCert returns the currently active certificate.
func (h *DeviceRenewalHandler) ActiveCert() *x509.Certificate {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activeCert
}

// ActiveKey returns the private key for the active certificate.
func (h *DeviceRenewalHandler) ActiveKey() *ecdsa.PrivateKey {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.activeKey
}

// CertSequence returns the current certificate sequence number.
func (h *DeviceRenewalHandler) CertSequence() uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.certSequence
}

// RenewalInProgress returns true if a renewal is in progress.
func (h *DeviceRenewalHandler) RenewalInProgress() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pendingKeyPair != nil
}

// SetActiveCert sets the active certificate and key (for initialization).
func (h *DeviceRenewalHandler) SetActiveCert(cert *x509.Certificate, key *ecdsa.PrivateKey, sequence uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeCert = cert
	h.activeKey = key
	h.certSequence = sequence
}

// publicKeysEqual compares two ECDSA public keys for equality.
func publicKeysEqual(a, b *ecdsa.PublicKey) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.X.Cmp(b.X) == 0 && a.Y.Cmp(b.Y) == 0
}

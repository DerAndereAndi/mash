package service

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// ControllerRenewalHandler handles certificate renewal on the controller side.
// It orchestrates the renewal protocol: sending requests, signing CSRs,
// and installing new certificates.
type ControllerRenewalHandler struct {
	mu sync.Mutex

	// zoneCA is the Zone CA used to sign device certificates.
	zoneCA *cert.ZoneCA

	// conn is the connection to the device.
	conn Sendable

	// pendingCSR holds the CSR received from the device during renewal.
	pendingCSR []byte

	// pendingDeviceID is the device currently being renewed.
	pendingDeviceID string

	// certSequence is the next certificate sequence number.
	certSequence uint32

	// responseWait is used to wait for async responses.
	responseWait chan any
}

// NewControllerRenewalHandler creates a new ControllerRenewalHandler.
func NewControllerRenewalHandler(zoneCA *cert.ZoneCA, conn Sendable) *ControllerRenewalHandler {
	return &ControllerRenewalHandler{
		zoneCA:       zoneCA,
		conn:         conn,
		certSequence: 1, // Start at 1
		responseWait: make(chan any, 1),
	}
}

// InitiateRenewal starts the renewal process by sending a CertRenewalRequest.
// It waits for the CSR response from the device.
func (h *ControllerRenewalHandler) InitiateRenewal(ctx context.Context, deviceID string) error {
	h.mu.Lock()
	h.pendingDeviceID = deviceID
	h.pendingCSR = nil
	h.mu.Unlock()

	// Generate 32-byte nonce
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	// Create and send renewal request
	req := &commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	}

	data, err := cbor.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	if err := h.conn.Send(data); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	// Wait for CSR response
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-h.responseWait:
		csrResp, ok := resp.(*commissioning.CertRenewalCSR)
		if !ok {
			return fmt.Errorf("unexpected response type: %T", resp)
		}
		h.mu.Lock()
		h.pendingCSR = csrResp.CSR
		h.mu.Unlock()
		return nil
	}
}

// RenewDevice performs the complete renewal flow for a device.
// It sends a renewal request, signs the CSR, and installs the new certificate.
func (h *ControllerRenewalHandler) RenewDevice(ctx context.Context, deviceID string) (*x509.Certificate, error) {
	// Step 1: Initiate renewal (sends request, waits for CSR)
	if err := h.InitiateRenewal(ctx, deviceID); err != nil {
		return nil, fmt.Errorf("initiate renewal: %w", err)
	}

	// Step 2: Sign the CSR
	h.mu.Lock()
	csrDER := h.pendingCSR
	h.mu.Unlock()

	if csrDER == nil {
		return nil, fmt.Errorf("no CSR received")
	}

	newCert, err := cert.SignCSR(h.zoneCA, csrDER)
	if err != nil {
		return nil, fmt.Errorf("sign CSR: %w", err)
	}

	// Step 3: Send the new certificate
	h.mu.Lock()
	h.certSequence++
	seq := h.certSequence
	h.mu.Unlock()

	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  newCert.Raw,
		Sequence: seq,
	}

	data, err := cbor.Marshal(install)
	if err != nil {
		return nil, fmt.Errorf("encode install: %w", err)
	}

	if err := h.conn.Send(data); err != nil {
		return nil, fmt.Errorf("send install: %w", err)
	}

	// Step 4: Wait for acknowledgment
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-h.responseWait:
		ack, ok := resp.(*commissioning.CertRenewalAck)
		if !ok {
			return nil, fmt.Errorf("unexpected response type: %T", resp)
		}
		if ack.Status != commissioning.RenewalStatusSuccess {
			return nil, fmt.Errorf("renewal failed with status %d", ack.Status)
		}
		return newCert, nil
	}
}

// HandleResponse processes a response message from the device.
// This should be called when a renewal message is received.
func (h *ControllerRenewalHandler) HandleResponse(msg any) {
	select {
	case h.responseWait <- msg:
	default:
		// Channel full, discard
	}
}

// IssueInitialCertSync issues an initial operational certificate during commissioning.
// Unlike RenewDevice, this includes the Zone CA certificate so the device can:
// 1. Verify future connections from this zone's controllers
// 2. Store the Zone CA for reconnection verification
//
// This method uses synchronous reads (blocks until response is received) because
// during commissioning there's no message loop yet to route responses.
//
// Protocol flow:
// 1. Send CertRenewalRequest with ZoneCA
// 2. Read CSR from device (blocking)
// 3. Sign CSR with Zone CA
// 4. Send CertRenewalInstall with new cert
// 5. Read acknowledgment (blocking)
func (h *ControllerRenewalHandler) IssueInitialCertSync(ctx context.Context, syncConn SyncConnection) (*x509.Certificate, error) {
	// Generate 32-byte nonce
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Create renewal request WITH Zone CA certificate (key difference from RenewDevice)
	req := &commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
		ZoneCA:  h.zoneCA.Certificate.Raw, // Include Zone CA for initial commissioning
	}

	data, err := cbor.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	if err := syncConn.Send(data); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Read CSR response (blocking)
	csrData, err := syncConn.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read CSR: %w", err)
	}

	msg, err := commissioning.DecodeRenewalMessage(csrData)
	if err != nil {
		return nil, fmt.Errorf("decode CSR: %w", err)
	}

	csrResp, ok := msg.(*commissioning.CertRenewalCSR)
	if !ok {
		return nil, fmt.Errorf("expected CertRenewalCSR, got %T", msg)
	}

	// Generate device ID from CSR (Matter-style: controller assigns ID)
	deviceID, err := cert.GenerateDeviceID(csrResp.CSR)
	if err != nil {
		return nil, fmt.Errorf("generate device ID: %w", err)
	}

	// Sign the CSR with controller-assigned device ID
	newCert, err := cert.SignCSRWithDeviceID(h.zoneCA, csrResp.CSR, deviceID)
	if err != nil {
		return nil, fmt.Errorf("sign CSR: %w", err)
	}

	// Send the new certificate
	h.mu.Lock()
	h.certSequence++
	seq := h.certSequence
	h.mu.Unlock()

	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  newCert.Raw,
		Sequence: seq,
	}

	data, err = cbor.Marshal(install)
	if err != nil {
		return nil, fmt.Errorf("encode install: %w", err)
	}

	if err := syncConn.Send(data); err != nil {
		return nil, fmt.Errorf("send install: %w", err)
	}

	// Read acknowledgment (blocking)
	ackData, err := syncConn.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read ack: %w", err)
	}

	msg, err = commissioning.DecodeRenewalMessage(ackData)
	if err != nil {
		return nil, fmt.Errorf("decode ack: %w", err)
	}

	ack, ok := msg.(*commissioning.CertRenewalAck)
	if !ok {
		return nil, fmt.Errorf("expected CertRenewalAck, got %T", msg)
	}

	if ack.Status != commissioning.RenewalStatusSuccess {
		return nil, fmt.Errorf("cert installation failed with status %d", ack.Status)
	}

	return newCert, nil
}

// PendingCSR returns the CSR received from the device, if any.
func (h *ControllerRenewalHandler) PendingCSR() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pendingCSR
}

// CertSequence returns the current certificate sequence number.
func (h *ControllerRenewalHandler) CertSequence() uint32 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.certSequence
}

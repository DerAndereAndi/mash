package commissioning

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/fxamacker/cbor/v2"
)

// PASEClientSession manages the client side of a PASE handshake.
type PASEClientSession struct {
	spake          *SPAKE2PlusClient
	clientIdentity []byte
	serverIdentity []byte
}

// NewPASEClientSession creates a new PASE client session.
func NewPASEClientSession(setupCode SetupCode, clientIdentity, serverIdentity []byte) (*PASEClientSession, error) {
	spake, err := NewSPAKE2PlusClient(setupCode, clientIdentity, serverIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPAKE2+ client: %w", err)
	}

	return &PASEClientSession{
		spake:          spake,
		clientIdentity: clientIdentity,
		serverIdentity: serverIdentity,
	}, nil
}

// ClientIdentity returns the client identity.
func (s *PASEClientSession) ClientIdentity() []byte {
	return s.clientIdentity
}

// Handshake performs the PASE handshake and returns the derived session key.
// The connection should be a TLS connection in commissioning mode.
func (s *PASEClientSession) Handshake(ctx context.Context, conn net.Conn) ([]byte, error) {
	// Step 1: Send PASERequest with our public value
	req := &PASERequest{
		MsgType:        MsgPASERequest,
		PublicValue:    s.spake.PublicValue(),
		ClientIdentity: s.clientIdentity,
	}

	if err := writeMessage(conn, req); err != nil {
		return nil, fmt.Errorf("failed to send PASE request: %w", err)
	}

	// Step 2: Read PASEResponse
	msg, err := readMessageWithContext(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read PASE response: %w", err)
	}

	resp, ok := msg.(*PASEResponse)
	if !ok {
		if errMsg, ok := msg.(*CommissioningError); ok {
			return nil, fmt.Errorf("server error: %s (code %d)", errMsg.Message, errMsg.ErrorCode)
		}
		return nil, fmt.Errorf("unexpected message type: expected PASEResponse, got %T", msg)
	}

	// Step 3: Process server's public value
	if err := s.spake.ProcessServerValue(resp.PublicValue); err != nil {
		return nil, fmt.Errorf("failed to process server value: %w", err)
	}

	// Step 4: Send PASEConfirm with our confirmation
	confirm := &PASEConfirm{
		MsgType:      MsgPASEConfirm,
		Confirmation: s.spake.Confirmation(),
	}

	if err := writeMessage(conn, confirm); err != nil {
		return nil, fmt.Errorf("failed to send PASE confirm: %w", err)
	}

	// Step 5: Read PASEComplete
	msg, err = readMessageWithContext(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read PASE complete: %w", err)
	}

	complete, ok := msg.(*PASEComplete)
	if !ok {
		if errMsg, ok := msg.(*CommissioningError); ok {
			return nil, fmt.Errorf("server error: %s (code %d)", errMsg.Message, errMsg.ErrorCode)
		}
		return nil, fmt.Errorf("unexpected message type: expected PASEComplete, got %T", msg)
	}

	// Check for error
	if complete.ErrorCode != ErrCodeSuccess {
		return nil, fmt.Errorf("PASE failed: error code %d", complete.ErrorCode)
	}

	// Step 6: Verify server's confirmation
	if err := s.spake.VerifyServerConfirmation(complete.Confirmation); err != nil {
		return nil, fmt.Errorf("server confirmation failed: %w", err)
	}

	return s.spake.SharedSecret(), nil
}

// PASEServerSession manages the server side of a PASE handshake.
type PASEServerSession struct {
	verifier       *Verifier
	spake          *SPAKE2PlusServer
	serverIdentity []byte
}

// NewPASEServerSession creates a new PASE server session.
func NewPASEServerSession(verifier *Verifier, serverIdentity []byte) (*PASEServerSession, error) {
	spake, err := NewSPAKE2PlusServer(verifier, serverIdentity)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPAKE2+ server: %w", err)
	}

	return &PASEServerSession{
		verifier:       verifier,
		spake:          spake,
		serverIdentity: serverIdentity,
	}, nil
}

// WaitForPASERequest reads the first message from the connection and validates
// that it is a PASERequest. This method does not acquire any lock and is
// intended to be called before the commissioning lock is held (DEC-061).
// The returned *PASERequest should be passed to CompleteHandshake.
func (s *PASEServerSession) WaitForPASERequest(ctx context.Context, conn net.Conn) (*PASERequest, error) {
	msg, err := readMessageWithContext(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read PASE request: %w", err)
	}

	req, ok := msg.(*PASERequest)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: expected PASERequest, got %T", msg)
	}

	return req, nil
}

// CompleteHandshake processes the PASE exchange after the initial PASERequest
// has been received. It performs steps 2-6 of the PASE protocol and returns
// the derived shared secret. This method should be called with the
// commissioning lock held (DEC-061).
func (s *PASEServerSession) CompleteHandshake(ctx context.Context, conn net.Conn, req *PASERequest) ([]byte, error) {
	// Step 2: Process client's public value
	if err := s.spake.ProcessClientValue(req.PublicValue); err != nil {
		// Send error response
		errMsg := &CommissioningError{
			MsgType:   MsgCommissioningError,
			ErrorCode: ErrCodeInvalidPublicKey,
			Message:   "invalid public key",
		}
		writeMessage(conn, errMsg)
		return nil, fmt.Errorf("failed to process client value: %w", err)
	}

	// Step 3: Send PASEResponse with our public value
	resp := &PASEResponse{
		MsgType:     MsgPASEResponse,
		PublicValue: s.spake.PublicValue(),
	}

	if err := writeMessage(conn, resp); err != nil {
		return nil, fmt.Errorf("failed to send PASE response: %w", err)
	}

	// Step 4: Read PASEConfirm
	msg, err := readMessageWithContext(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read PASE confirm: %w", err)
	}

	confirm, ok := msg.(*PASEConfirm)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: expected PASEConfirm, got %T", msg)
	}

	// Step 5: Verify client's confirmation
	var errorCode uint8 = ErrCodeSuccess
	if err := s.spake.VerifyClientConfirmation(confirm.Confirmation); err != nil {
		errorCode = ErrCodeConfirmFailed
	}

	// Step 6: Send PASEComplete with our confirmation
	complete := &PASEComplete{
		MsgType:      MsgPASEComplete,
		Confirmation: s.spake.Confirmation(),
		ErrorCode:    errorCode,
	}

	if err := writeMessage(conn, complete); err != nil {
		return nil, fmt.Errorf("failed to send PASE complete: %w", err)
	}

	if errorCode != ErrCodeSuccess {
		return nil, fmt.Errorf("client confirmation failed: error code %d", errorCode)
	}

	return s.spake.SharedSecret(), nil
}

// Handshake performs the PASE handshake and returns the derived session key.
// This is a convenience wrapper that calls WaitForPASERequest followed by
// CompleteHandshake. For message-gated locking (DEC-061), use the split
// methods directly.
func (s *PASEServerSession) Handshake(ctx context.Context, conn net.Conn) ([]byte, error) {
	req, err := s.WaitForPASERequest(ctx, conn)
	if err != nil {
		return nil, err
	}
	return s.CompleteHandshake(ctx, conn, req)
}

// MarshalVerifier serializes a verifier to CBOR bytes for storage.
func MarshalVerifier(v *Verifier) ([]byte, error) {
	return cbor.Marshal(v)
}

// UnmarshalVerifier deserializes a verifier from CBOR bytes.
func UnmarshalVerifier(data []byte) (*Verifier, error) {
	var v Verifier
	if err := cbor.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// Wire protocol helpers

// writeMessage writes a length-prefixed CBOR message to the connection.
func writeMessage(conn net.Conn, msg interface{}) error {
	data, err := cbor.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Write length prefix (4 bytes, big-endian)
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))

	if _, err := conn.Write(length); err != nil {
		return fmt.Errorf("failed to write length: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// readMessage reads a length-prefixed CBOR message from the connection.
func readMessage(conn net.Conn) (interface{}, error) {
	// Read length prefix
	length := make([]byte, 4)
	if _, err := io.ReadFull(conn, length); err != nil {
		return nil, fmt.Errorf("failed to read length: %w", err)
	}

	msgLen := binary.BigEndian.Uint32(length)
	if msgLen > 65536 { // Sanity check
		return nil, fmt.Errorf("message too large: %d bytes", msgLen)
	}

	// Read message data
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	return DecodePASEMessage(data)
}

// readMessageWithContext reads a message with context cancellation support.
func readMessageWithContext(ctx context.Context, conn net.Conn) (interface{}, error) {
	// Set up a channel to receive the result
	type result struct {
		msg interface{}
		err error
	}
	resultCh := make(chan result, 1)

	go func() {
		msg, err := readMessage(conn)
		resultCh <- result{msg, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-resultCh:
		return r.msg, r.err
	}
}

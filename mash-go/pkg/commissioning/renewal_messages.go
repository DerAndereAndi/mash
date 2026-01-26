package commissioning

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Certificate renewal message types.
const (
	// MsgCertRenewalRequest initiates certificate renewal.
	MsgCertRenewalRequest uint8 = 30

	// MsgCertRenewalCSR contains the device's new CSR.
	MsgCertRenewalCSR uint8 = 31

	// MsgCertRenewalInstall delivers the new certificate.
	MsgCertRenewalInstall uint8 = 32

	// MsgCertRenewalAck confirms certificate installation.
	MsgCertRenewalAck uint8 = 33
)

// Renewal status codes.
const (
	RenewalStatusSuccess       uint8 = 0
	RenewalStatusCSRFailed     uint8 = 1
	RenewalStatusInstallFailed uint8 = 2
	RenewalStatusInvalidCert   uint8 = 3
)

// CertRenewalRequest initiates certificate renewal.
// Sent by controller to device to start renewal process.
// CBOR: { 1: msgType, 2: nonce }
type CertRenewalRequest struct {
	MsgType uint8  `cbor:"1,keyasint"`
	Nonce   []byte `cbor:"2,keyasint"` // 32-byte anti-replay nonce
}

// CertRenewalCSR contains the device's new CSR.
// Sent by device in response to CertRenewalRequest.
// CBOR: { 1: msgType, 2: csr }
type CertRenewalCSR struct {
	MsgType uint8  `cbor:"1,keyasint"`
	CSR     []byte `cbor:"2,keyasint"` // PKCS#10 DER-encoded CSR
}

// CertRenewalInstall delivers the new certificate.
// Sent by controller after signing the device's CSR.
// CBOR: { 1: msgType, 2: newCert, 3: sequence }
type CertRenewalInstall struct {
	MsgType  uint8  `cbor:"1,keyasint"`
	NewCert  []byte `cbor:"2,keyasint"` // X.509 DER-encoded certificate
	Sequence uint32 `cbor:"3,keyasint"` // Certificate sequence number
}

// CertRenewalAck confirms certificate installation.
// Sent by device after installing new certificate.
// CBOR: { 1: msgType, 2: status, 3: activeSequence }
type CertRenewalAck struct {
	MsgType        uint8  `cbor:"1,keyasint"`
	Status         uint8  `cbor:"2,keyasint"` // 0=success, see RenewalStatus* constants
	ActiveSequence uint32 `cbor:"3,keyasint"` // Sequence number of now-active cert
}

// EncodeRenewalMessage encodes a renewal message to CBOR bytes.
func EncodeRenewalMessage(msg any) ([]byte, error) {
	return cbor.Marshal(msg)
}

// DecodeRenewalMessage decodes CBOR bytes to the appropriate renewal message type.
func DecodeRenewalMessage(data []byte) (any, error) {
	// First, decode just to get the message type
	var header struct {
		MsgType uint8 `cbor:"1,keyasint"`
	}
	if err := cbor.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}

	// Decode based on message type
	switch header.MsgType {
	case MsgCertRenewalRequest:
		var msg CertRenewalRequest
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCertRenewalCSR:
		var msg CertRenewalCSR
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCertRenewalInstall:
		var msg CertRenewalInstall
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	case MsgCertRenewalAck:
		var msg CertRenewalAck
		if err := cbor.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("%w: unknown renewal message type %d", ErrInvalidMessage, header.MsgType)
	}
}

// RenewalMessageType returns the message type from a decoded renewal message.
func RenewalMessageType(msg any) uint8 {
	switch m := msg.(type) {
	case *CertRenewalRequest:
		return m.MsgType
	case *CertRenewalCSR:
		return m.MsgType
	case *CertRenewalInstall:
		return m.MsgType
	case *CertRenewalAck:
		return m.MsgType
	default:
		return 0
	}
}

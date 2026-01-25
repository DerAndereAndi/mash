package commissioning

import (
	"errors"
)

// Commissioning message types.
const (
	// MsgPASERequest initiates PASE (SPAKE2+) exchange.
	MsgPASERequest uint8 = 1

	// MsgPASEResponse contains server's public value.
	MsgPASEResponse uint8 = 2

	// MsgPASEConfirm contains client's confirmation.
	MsgPASEConfirm uint8 = 3

	// MsgPASEComplete contains server's confirmation and success status.
	MsgPASEComplete uint8 = 4

	// MsgCSRRequest requests a Certificate Signing Request from the device.
	MsgCSRRequest uint8 = 10

	// MsgCSRResponse contains the device's CSR.
	MsgCSRResponse uint8 = 11

	// MsgCertInstall installs the operational certificate on the device.
	MsgCertInstall uint8 = 12

	// MsgCertInstallResponse confirms certificate installation.
	MsgCertInstallResponse uint8 = 13

	// MsgCommissioningComplete indicates commissioning is finished.
	MsgCommissioningComplete uint8 = 20

	// MsgCommissioningError indicates an error occurred.
	MsgCommissioningError uint8 = 255
)

// Commissioning error codes.
const (
	ErrCodeSuccess           uint8 = 0
	ErrCodeInvalidPublicKey  uint8 = 1
	ErrCodeConfirmFailed     uint8 = 2
	ErrCodeCSRFailed         uint8 = 3
	ErrCodeCertInstallFailed uint8 = 4
	ErrCodeInternalError     uint8 = 255
)

// Message errors.
var (
	ErrInvalidMessage = errors.New("invalid commissioning message")
)

// PASERequest is the initial SPAKE2+ message from controller to device.
// CBOR: { 1: msgType, 2: publicValue, 3: clientIdentity }
type PASERequest struct {
	MsgType        uint8  `cbor:"1,keyasint"`
	PublicValue    []byte `cbor:"2,keyasint"` // pA
	ClientIdentity []byte `cbor:"3,keyasint"`
}

// PASEResponse is the device's response containing its public value.
// CBOR: { 1: msgType, 2: publicValue }
type PASEResponse struct {
	MsgType     uint8  `cbor:"1,keyasint"`
	PublicValue []byte `cbor:"2,keyasint"` // pB
}

// PASEConfirm contains the client's confirmation MAC.
// CBOR: { 1: msgType, 2: confirmation }
type PASEConfirm struct {
	MsgType      uint8  `cbor:"1,keyasint"`
	Confirmation []byte `cbor:"2,keyasint"`
}

// PASEComplete contains the server's confirmation and status.
// CBOR: { 1: msgType, 2: confirmation, 3: errorCode }
type PASEComplete struct {
	MsgType      uint8  `cbor:"1,keyasint"`
	Confirmation []byte `cbor:"2,keyasint"`
	ErrorCode    uint8  `cbor:"3,keyasint"`
}

// CSRRequest requests a Certificate Signing Request from the device.
// CBOR: { 1: msgType, 2: nonce }
type CSRRequest struct {
	MsgType uint8  `cbor:"1,keyasint"`
	Nonce   []byte `cbor:"2,keyasint"` // Random nonce to prevent replay
}

// CSRResponse contains the device's CSR.
// CBOR: { 1: msgType, 2: csr, 3: attestationCert (optional), 4: errorCode }
type CSRResponse struct {
	MsgType         uint8  `cbor:"1,keyasint"`
	CSR             []byte `cbor:"2,keyasint"`           // DER-encoded PKCS#10 CSR
	AttestationCert []byte `cbor:"3,keyasint,omitempty"` // Optional device attestation cert
	ErrorCode       uint8  `cbor:"4,keyasint"`
}

// CertInstall delivers the operational certificate to the device.
// CBOR: { 1: msgType, 2: operationalCert, 3: caCert, 4: zoneType, 5: zonePriority }
type CertInstall struct {
	MsgType         uint8  `cbor:"1,keyasint"`
	OperationalCert []byte `cbor:"2,keyasint"` // DER-encoded X.509 certificate
	CACert          []byte `cbor:"3,keyasint"` // Zone CA certificate
	ZoneType        uint8  `cbor:"4,keyasint"` // ZoneType enum value
	ZonePriority    uint8  `cbor:"5,keyasint"` // Priority within this zone type
}

// CertInstallResponse confirms certificate installation.
// CBOR: { 1: msgType, 2: errorCode }
type CertInstallResponse struct {
	MsgType   uint8 `cbor:"1,keyasint"`
	ErrorCode uint8 `cbor:"2,keyasint"`
}

// CommissioningComplete indicates successful commissioning.
// CBOR: { 1: msgType }
type CommissioningComplete struct {
	MsgType uint8 `cbor:"1,keyasint"`
}

// CommissioningError indicates a commissioning error.
// CBOR: { 1: msgType, 2: errorCode, 3: message }
type CommissioningError struct {
	MsgType   uint8  `cbor:"1,keyasint"`
	ErrorCode uint8  `cbor:"2,keyasint"`
	Message   string `cbor:"3,keyasint,omitempty"`
}

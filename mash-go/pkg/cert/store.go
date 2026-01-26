package cert

import (
	"crypto/ecdsa"
	"crypto/x509"
	"errors"
)

// Store errors.
var (
	ErrCertNotFound   = errors.New("certificate not found")
	ErrMaxZonesExceed = errors.New("maximum zones exceeded")
	ErrZoneExists     = errors.New("zone already exists")
	ErrInvalidCert    = errors.New("invalid certificate")
)

// Store defines the interface for certificate storage.
// Implementations must be safe for concurrent access.
type Store interface {
	// Device attestation certificate (optional, pre-installed at manufacturing)

	// GetDeviceAttestation returns the device attestation certificate and key.
	// Returns ErrCertNotFound if no attestation certificate is stored.
	GetDeviceAttestation() (*x509.Certificate, *ecdsa.PrivateKey, error)

	// SetDeviceAttestation stores the device attestation certificate and key.
	SetDeviceAttestation(cert *x509.Certificate, key *ecdsa.PrivateKey) error

	// Operational certificates (one per zone, max 5 zones)

	// GetOperationalCert returns the operational certificate for a zone.
	// Returns ErrCertNotFound if no certificate exists for the zone.
	GetOperationalCert(zoneID string) (*OperationalCert, error)

	// SetOperationalCert stores an operational certificate for a zone.
	// Returns ErrMaxZonesExceed if the device is already in 5 zones.
	SetOperationalCert(cert *OperationalCert) error

	// RemoveOperationalCert removes the operational certificate for a zone.
	// Returns ErrCertNotFound if no certificate exists for the zone.
	RemoveOperationalCert(zoneID string) error

	// ListZones returns all zone IDs the device belongs to.
	ListZones() []string

	// ZoneCount returns the number of zones the device belongs to.
	ZoneCount() int

	// Zone CA certificates (for verifying peer certificates)

	// GetZoneCACert returns the Zone CA certificate for a zone.
	// Returns ErrCertNotFound if not found.
	GetZoneCACert(zoneID string) (*x509.Certificate, error)

	// SetZoneCACert stores a Zone CA certificate for a zone.
	SetZoneCACert(zoneID string, cert *x509.Certificate) error

	// GetAllZoneCAs returns all stored Zone CA certificates.
	GetAllZoneCAs() []*x509.Certificate

	// Persistence (optional, depends on implementation)

	// Save persists the store to its backing storage.
	// For in-memory stores, this may be a no-op.
	Save() error

	// Load reads the store from its backing storage.
	// For in-memory stores, this may be a no-op.
	Load() error
}

// ControllerStore defines additional storage for zone owners (controllers).
// Controllers need to store their Zone CA private keys.
type ControllerStore interface {
	Store

	// GetZoneCA returns the full Zone CA (including private key).
	// This is only available on the zone owner (controller).
	GetZoneCA() (*ZoneCA, error)

	// SetZoneCA stores the Zone CA (including private key).
	SetZoneCA(ca *ZoneCA) error
}

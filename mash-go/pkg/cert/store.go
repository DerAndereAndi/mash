package cert

import (
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
//
// Note: Devices do NOT persist identity/commissioning certificates.
// The commissioning certificate is generated in-memory at startup and is
// temporary (24h validity). The device ID is derived from the operational
// certificate after commissioning completes.
type Store interface {
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
// Controllers need to store their Zone CA private keys and operational certificate.
type ControllerStore interface {
	Store

	// GetZoneCA returns the full Zone CA (including private key).
	// This is only available on the zone owner (controller).
	GetZoneCA() (*ZoneCA, error)

	// SetZoneCA stores the Zone CA (including private key).
	SetZoneCA(ca *ZoneCA) error

	// GetControllerCert returns the controller's operational certificate.
	// This certificate is used for mutual TLS with devices.
	// Returns ErrCertNotFound if no controller certificate exists.
	GetControllerCert() (*OperationalCert, error)

	// SetControllerCert stores the controller's operational certificate.
	SetControllerCert(cert *OperationalCert) error
}

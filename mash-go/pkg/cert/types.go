package cert

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"time"
)

// Certificate validity periods as defined in security.md.
const (
	// ZoneCAValidity is the validity period for Zone CA certificates.
	// 20 years to match expected device lifetime and avoid mass re-commissioning.
	ZoneCAValidity = 20 * 365 * 24 * time.Hour // 20 years

	// OperationalCertValidity is the validity period for operational certificates.
	OperationalCertValidity = 365 * 24 * time.Hour // 1 year

	// RenewalWindow is how long before expiry to start renewal.
	RenewalWindow = 30 * 24 * time.Hour // 30 days

	// GracePeriod is the optional grace period after expiry.
	GracePeriod = 7 * 24 * time.Hour // 7 days
)

// MaxZones is the maximum number of zones a device can belong to.
const MaxZones = 5

// KeyPair holds an ECDSA P-256 key pair for MASH cryptographic operations.
type KeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// ZoneCA represents a Zone Certificate Authority.
// Zone owners (controllers) generate and hold Zone CAs to issue operational
// certificates to devices during commissioning.
type ZoneCA struct {
	// Certificate is the Zone CA X.509 certificate.
	Certificate *x509.Certificate

	// PrivateKey is the Zone CA private key (only held by zone owner).
	PrivateKey *ecdsa.PrivateKey

	// ZoneID is a unique identifier for this zone.
	ZoneID string

	// ZoneType indicates the type of zone (GRID or LOCAL).
	ZoneType ZoneType
}

// ZoneType represents the type of zone, which determines priority.
type ZoneType uint8

// Zone types with their priorities (lower number = higher priority).
const (
	ZoneTypeGrid  ZoneType = 1 // External/regulatory authority (DSO, SMGW, aggregators)
	ZoneTypeLocal ZoneType = 2 // Local energy management (EMS, residential or commercial)
)

// String returns a human-readable zone type name.
func (zt ZoneType) String() string {
	switch zt {
	case ZoneTypeGrid:
		return "GRID"
	case ZoneTypeLocal:
		return "LOCAL"
	default:
		return "UNKNOWN"
	}
}

// Priority returns the numeric priority (1 = highest).
func (zt ZoneType) Priority() uint8 {
	return uint8(zt)
}

// OperationalCert represents an operational certificate for a device.
// These are issued by Zone CAs during commissioning and prove zone membership.
type OperationalCert struct {
	// Certificate is the X.509 operational certificate.
	Certificate *x509.Certificate

	// PrivateKey is the device's private key for this certificate.
	PrivateKey *ecdsa.PrivateKey

	// ZoneID identifies which zone this certificate is for.
	ZoneID string

	// ZoneType is the type of the zone that issued this certificate.
	ZoneType ZoneType

	// ZoneCACert is the Zone CA certificate (for chain verification).
	ZoneCACert *x509.Certificate
}

// SKI returns the Subject Key Identifier of the operational certificate.
// This serves as the device's unique identifier within MASH.
func (oc *OperationalCert) SKI() []byte {
	if oc.Certificate == nil {
		return nil
	}
	return oc.Certificate.SubjectKeyId
}

// ExpiresAt returns when this certificate expires.
func (oc *OperationalCert) ExpiresAt() time.Time {
	if oc.Certificate == nil {
		return time.Time{}
	}
	return oc.Certificate.NotAfter
}

// NeedsRenewal returns true if the certificate should be renewed.
func (oc *OperationalCert) NeedsRenewal() bool {
	if oc.Certificate == nil {
		return true
	}
	return time.Now().Add(RenewalWindow).After(oc.Certificate.NotAfter)
}

// IsExpired returns true if the certificate has expired.
func (oc *OperationalCert) IsExpired() bool {
	if oc.Certificate == nil {
		return true
	}
	return time.Now().After(oc.Certificate.NotAfter)
}

// IsInGracePeriod returns true if the certificate is expired but within grace period.
func (oc *OperationalCert) IsInGracePeriod() bool {
	if oc.Certificate == nil {
		return false
	}
	now := time.Now()
	return now.After(oc.Certificate.NotAfter) && now.Before(oc.Certificate.NotAfter.Add(GracePeriod))
}

// DeviceIdentity holds the identity information embedded in certificates.
type DeviceIdentity struct {
	// DeviceID is the unique device identifier (typically SKI of operational cert).
	DeviceID string

	// VendorID identifies the device manufacturer.
	VendorID uint32

	// ProductID identifies the device product within the vendor.
	ProductID uint32

	// SerialNumber is the device serial number (optional).
	SerialNumber string
}

// CSRInfo contains information for creating a Certificate Signing Request.
type CSRInfo struct {
	// Identity is the device identity to embed in the CSR.
	Identity DeviceIdentity

	// ZoneID is the zone this CSR is for.
	ZoneID string
}

// TLSCertificate converts the operational certificate to a tls.Certificate
// for use in TLS connections.
func (oc *OperationalCert) TLSCertificate() tls.Certificate {
	if oc == nil || oc.Certificate == nil || oc.PrivateKey == nil {
		return tls.Certificate{}
	}
	return tls.Certificate{
		Certificate: [][]byte{oc.Certificate.Raw},
		PrivateKey:  oc.PrivateKey,
		Leaf:        oc.Certificate,
	}
}

// TLSClientCAs returns an x509.CertPool containing the Zone CA certificate,
// suitable for use as tls.Config.RootCAs to verify peer certificates.
func (ca *ZoneCA) TLSClientCAs() *x509.CertPool {
	if ca == nil || ca.Certificate == nil {
		return nil
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca.Certificate)
	return pool
}

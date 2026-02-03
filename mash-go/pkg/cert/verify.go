package cert

import (
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// Verification errors.
var (
	ErrCertExpired       = errors.New("certificate has expired")
	ErrCertNotYetValid   = errors.New("certificate is not yet valid")
	ErrInvalidChain      = errors.New("invalid certificate chain")
	ErrZoneMismatch      = errors.New("certificate zone mismatch")
	ErrNotOperationalCert = errors.New("not an operational certificate")
)

// VerifyOperationalCert verifies that an operational certificate is valid
// and was issued by the expected Zone CA.
func VerifyOperationalCert(cert *x509.Certificate, zoneCACert *x509.Certificate) error {
	if cert == nil {
		return ErrInvalidCert
	}
	if zoneCACert == nil {
		return fmt.Errorf("%w: Zone CA certificate required", ErrInvalidChain)
	}

	// Check validity period
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return ErrCertNotYetValid
	}
	if now.After(cert.NotAfter) {
		return ErrCertExpired
	}

	// Verify the certificate was signed by the Zone CA
	roots := x509.NewCertPool()
	roots.AddCert(zoneCACert)

	opts := x509.VerifyOptions{
		Roots:       roots,
		CurrentTime: now,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	if _, err := cert.Verify(opts); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidChain, err)
	}

	return nil
}

// VerifyPeerCertificate creates a verification callback for TLS connections.
// It verifies that the peer's certificate was issued by one of the known Zone CAs.
func VerifyPeerCertificate(store Store) func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("no peer certificate")
		}

		// Parse the peer certificate
		peerCert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("parse peer certificate: %w", err)
		}

		// Try to verify against any known Zone CA
		zoneCAs := store.GetAllZoneCAs()
		if len(zoneCAs) == 0 {
			return fmt.Errorf("no Zone CAs configured")
		}

		var lastErr error
		for _, zoneCACert := range zoneCAs {
			if err := VerifyOperationalCert(peerCert, zoneCACert); err == nil {
				return nil // Successfully verified
			} else {
				lastErr = err
			}
		}

		return fmt.Errorf("peer certificate not issued by any known Zone CA: %w", lastErr)
	}
}

// VerifyZoneMembership checks if a certificate belongs to a specific zone.
// This is done by checking if the certificate's Authority Key ID matches
// the Zone CA's Subject Key ID.
func VerifyZoneMembership(cert *x509.Certificate, zoneCACert *x509.Certificate) error {
	if cert == nil || zoneCACert == nil {
		return ErrInvalidCert
	}

	// Check Authority Key ID matches Zone CA's Subject Key ID
	if len(cert.AuthorityKeyId) == 0 || len(zoneCACert.SubjectKeyId) == 0 {
		return fmt.Errorf("%w: missing key identifiers", ErrZoneMismatch)
	}

	if !bytesEqual(cert.AuthorityKeyId, zoneCACert.SubjectKeyId) {
		return ErrZoneMismatch
	}

	return nil
}

// ExtractDeviceID extracts the device ID from a certificate's CommonName.
// This follows the Matter pattern where the commissioner assigns the device ID
// and embeds it in the certificate subject during signing.
func ExtractDeviceID(cert *x509.Certificate) (string, error) {
	if cert == nil {
		return "", ErrInvalidCert
	}
	if cert.Subject.CommonName == "" {
		return "", fmt.Errorf("certificate has no CommonName")
	}
	return cert.Subject.CommonName, nil
}

// CertificateInfo extracts human-readable information from a certificate.
type CertificateInfo struct {
	DeviceID   string
	CommonName string
	Issuer     string
	NotBefore  time.Time
	NotAfter   time.Time
	IsCA       bool
	SKI        []byte
	AKI        []byte
}

// GetCertificateInfo extracts information from a certificate.
func GetCertificateInfo(cert *x509.Certificate) *CertificateInfo {
	if cert == nil {
		return nil
	}

	return &CertificateInfo{
		DeviceID:   cert.Subject.CommonName, // Matter-style: device ID is in CommonName
		CommonName: cert.Subject.CommonName,
		Issuer:     cert.Issuer.CommonName,
		NotBefore:  cert.NotBefore,
		NotAfter:   cert.NotAfter,
		IsCA:       cert.IsCA,
		SKI:        cert.SubjectKeyId,
		AKI:        cert.AuthorityKeyId,
	}
}

// ErrUnknownZoneType is returned when the zone type cannot be determined from a certificate.
var ErrUnknownZoneType = errors.New("unknown zone type in certificate")

// ExtractZoneTypeFromCert extracts the zone type from a Zone CA certificate's
// OrganizationalUnit[0] field. The zone type is embedded during GenerateZoneCA.
func ExtractZoneTypeFromCert(c *x509.Certificate) (ZoneType, error) {
	if c == nil {
		return 0, ErrInvalidCert
	}
	if len(c.Subject.OrganizationalUnit) == 0 {
		return 0, fmt.Errorf("%w: no OrganizationalUnit in certificate", ErrUnknownZoneType)
	}
	switch c.Subject.OrganizationalUnit[0] {
	case "GRID":
		return ZoneTypeGrid, nil
	case "LOCAL":
		return ZoneTypeLocal, nil
	case "TEST":
		return ZoneTypeTest, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrUnknownZoneType, c.Subject.OrganizationalUnit[0])
	}
}

// bytesEqual compares two byte slices for equality.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

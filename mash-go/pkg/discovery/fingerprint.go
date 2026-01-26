package discovery

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
)

// ZoneIDFromCertificate generates a zone ID from a Zone CA certificate.
//
// The zone ID is the first 64 bits (16 hex chars) of SHA-256(certificate DER).
func ZoneIDFromCertificate(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:8])
}

// DeviceIDFromPublicKey generates a device ID from the device's operational certificate public key.
//
// The device ID is the first 64 bits (16 hex chars) of SHA-256(public key DER).
func DeviceIDFromPublicKey(cert *x509.Certificate) (string, error) {
	pubKeyDER, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	hash := sha256.Sum256(pubKeyDER)
	return hex.EncodeToString(hash[:8]), nil
}

// DeviceIDFromPublicKeyBytes generates a device ID from raw public key DER bytes.
func DeviceIDFromPublicKeyBytes(pubKeyDER []byte) string {
	hash := sha256.Sum256(pubKeyDER)
	return hex.EncodeToString(hash[:8])
}

// ZoneIDFromDER generates a zone ID from raw certificate DER bytes.
func ZoneIDFromDER(certDER []byte) string {
	hash := sha256.Sum256(certDER)
	return hex.EncodeToString(hash[:8])
}

// ValidateID checks if an ID string is a valid 64-bit fingerprint (16 hex chars).
func ValidateID(id string) bool {
	if len(id) != IDLength {
		return false
	}
	return isHexString(id)
}

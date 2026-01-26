package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// deriveZoneID derives a zone ID from a PASE shared secret.
// The zone ID is the first 16 hex characters of SHA256(sharedSecret).
func deriveZoneID(sharedSecret []byte) string {
	hash := sha256.Sum256(sharedSecret)
	return hex.EncodeToString(hash[:8]) // 16 hex chars = 8 bytes
}

// deriveDeviceID derives a device ID from a PASE shared secret.
// The device ID is derived similarly to zone ID but uses a different prefix.
func deriveDeviceID(sharedSecret []byte) string {
	// Use different data to derive device ID vs zone ID
	h := sha256.New()
	h.Write([]byte("device:"))
	h.Write(sharedSecret)
	hash := h.Sum(nil)
	return hex.EncodeToString(hash[:8]) // 16 hex chars
}

// deriveServerID creates the SPAKE2+ server identity from a device identifier.
// This is used in the PASE handshake to bind the session to the device.
func deriveServerID(deviceID string) []byte {
	return []byte("mash-device:" + deviceID)
}

// deriveClientID creates the SPAKE2+ client identity for a controller.
func deriveClientID(controllerName string) []byte {
	return []byte("mash-controller:" + controllerName)
}

// generateSelfSignedCert generates a self-signed TLS certificate for commissioning.
// During commissioning, devices use self-signed certificates. The actual security
// comes from the SPAKE2+ (PASE) handshake, not from certificate verification.
func generateSelfSignedCert() (tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "MASH Device (Commissioning)",
			Organization: []string{"MASH"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(24 * time.Hour), // Short validity for commissioning
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}, nil
}

// generateControllerCert generates a self-signed TLS certificate for a controller.
// Used during commissioning as the client certificate.
func generateControllerCert() (tls.Certificate, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "MASH Controller (Commissioning)",
			Organization: []string{"MASH"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privateKey,
	}, nil
}

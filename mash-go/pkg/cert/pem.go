package cert

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
)

// PEM encoding/decoding errors.
var (
	ErrInvalidPEM    = errors.New("invalid PEM data")
	ErrInvalidKey    = errors.New("invalid private key")
	ErrReadFile      = errors.New("failed to read file")
	ErrWriteFile     = errors.New("failed to write file")
	ErrUnsupportedEC = errors.New("unsupported EC key type")
)

// EncodeCertPEM encodes an X.509 certificate to PEM format.
func EncodeCertPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// DecodeCertPEM decodes a PEM-encoded X.509 certificate.
func DecodeCertPEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, ErrInvalidPEM
	}
	return x509.ParseCertificate(block.Bytes)
}

// EncodeKeyPEM encodes an ECDSA private key to PEM format.
func EncodeKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}), nil
}

// DecodeKeyPEM decodes a PEM-encoded ECDSA private key.
func DecodeKeyPEM(data []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, ErrInvalidPEM
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// WriteCertFile writes a certificate to a PEM file.
func WriteCertFile(path string, cert *x509.Certificate) error {
	data := EncodeCertPEM(cert)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return nil
}

// ReadCertFile reads a certificate from a PEM file.
func ReadCertFile(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeCertPEM(data)
}

// WriteKeyFile writes a private key to a PEM file with restricted permissions.
func WriteKeyFile(path string, key *ecdsa.PrivateKey) error {
	data, err := EncodeKeyPEM(key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	return nil
}

// ReadKeyFile reads a private key from a PEM file.
func ReadKeyFile(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeKeyPEM(data)
}

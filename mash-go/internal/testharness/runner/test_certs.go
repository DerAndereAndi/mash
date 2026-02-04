package runner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

// generateTestClientCert creates a test client certificate with specific properties
// for negative TLS testing. Returns a tls.Certificate ready for tlsConfig.Certificates.
//
// Supported cert types:
//   - "controller_not_yet_valid": NotBefore is 24h in the future
//   - "controller_expired": NotAfter is 24h in the past
//   - "controller_wrong_zone": signed by a different CA
//   - "controller_no_client_auth": missing ExtKeyUsageClientAuth
//   - "controller_ca_true": IsCA=true (leaf masquerading as CA)
func generateTestClientCert(certType string, zoneCA *cert.ZoneCA) (tls.Certificate, error) {
	// Generate a fresh key pair for the test certificate.
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()

	// Base template -- modified per cert type below.
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "test-controller",
			Organization: []string{"MASH Test"},
		},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Signing CA defaults to the provided zoneCA.
	signerCert := zoneCA.Certificate
	signerKey := zoneCA.PrivateKey

	switch certType {
	case "controller_not_yet_valid":
		template.NotBefore = now.Add(24 * time.Hour)
		template.NotAfter = now.Add(48 * time.Hour)

	case "controller_expired":
		template.NotBefore = now.Add(-48 * time.Hour)
		template.NotAfter = now.Add(-24 * time.Hour)

	case "controller_wrong_zone":
		// Generate a separate CA and sign with it.
		wrongCA, caErr := cert.GenerateZoneCA("wrong-zone-id", cert.ZoneTypeLocal)
		if caErr != nil {
			return tls.Certificate{}, fmt.Errorf("generate wrong zone CA: %w", caErr)
		}
		signerCert = wrongCA.Certificate
		signerKey = wrongCA.PrivateKey

	case "controller_no_client_auth":
		// Only ServerAuth -- missing ClientAuth.
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	case "controller_ca_true":
		template.IsCA = true
		template.BasicConstraintsValid = true

	default:
		return tls.Certificate{}, fmt.Errorf("unknown test cert type: %q", certType)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, signerCert, &privKey.PublicKey, signerKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	leafCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  privKey,
		Leaf:        leafCert,
	}, nil
}

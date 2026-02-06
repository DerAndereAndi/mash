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

	case "controller_self_signed":
		// Self-signed: use the cert's own key as signer (not the zone CA).
		signerCert = template
		signerKey = privKey

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

// buildCertChain constructs a tls.Certificate from a cert_chain YAML spec.
// The spec is an ordered array describing what the client presents:
//
// Chain composition (leaf + CA certs):
//   - "leaf_cert": Controller's operational certificate (must be first)
//   - "zone_ca_cert": Append the real Zone CA cert to the chain
//   - "wrong_zone_ca": Append a different Zone CA cert to the chain
//
// Deep chain (generates a full 3-level hierarchy):
//   - ["leaf_cert", "intermediate_ca", "root_ca"]
//
// Single invalid certs (for negative TLS alert tests):
//   - "invalid_signature_cert": Cert with corrupted DER signature
//   - "expired_cert": Expired certificate
//   - "wrong_zone_cert": Cert signed by a different Zone CA
func buildCertChain(specs []string, controllerCert *cert.OperationalCert, zoneCA *cert.ZoneCA) (tls.Certificate, error) {
	if len(specs) == 0 {
		return tls.Certificate{}, fmt.Errorf("empty cert_chain spec")
	}

	// Single special cert types (no chain composition needed).
	if len(specs) == 1 {
		switch specs[0] {
		case "invalid_signature_cert":
			return generateInvalidSignatureCert(zoneCA)
		case "expired_cert":
			if zoneCA == nil {
				return tls.Certificate{}, fmt.Errorf("expired_cert requires a zone CA")
			}
			return generateTestClientCert("controller_expired", zoneCA)
		case "wrong_zone_cert":
			if zoneCA == nil {
				return tls.Certificate{}, fmt.Errorf("wrong_zone_cert requires a zone CA")
			}
			return generateTestClientCert("controller_wrong_zone", zoneCA)
		}
	}

	// Deep chain: if intermediate_ca or root_ca appears, generate a 3-level hierarchy.
	for _, s := range specs {
		if s == "intermediate_ca" || s == "root_ca" {
			return generateDeepChain(specs)
		}
	}

	// Standard chain composition: controller leaf + additional CA certs.
	if controllerCert == nil {
		return tls.Certificate{}, fmt.Errorf("cert_chain with leaf_cert requires a controller cert (commission first)")
	}

	tlsCert := controllerCert.TLSCertificate()

	// Append chain certs after the leaf (skip the first "leaf_cert" entry).
	for _, spec := range specs[1:] {
		switch spec {
		case "zone_ca_cert":
			if zoneCA == nil {
				return tls.Certificate{}, fmt.Errorf("zone_ca_cert requires a zone CA")
			}
			tlsCert.Certificate = append(tlsCert.Certificate, zoneCA.Certificate.Raw)
		case "wrong_zone_ca":
			wrongCA, err := cert.GenerateZoneCA("wrong-chain-zone", cert.ZoneTypeLocal)
			if err != nil {
				return tls.Certificate{}, fmt.Errorf("generate wrong zone CA: %w", err)
			}
			tlsCert.Certificate = append(tlsCert.Certificate, wrongCA.Certificate.Raw)
		default:
			return tls.Certificate{}, fmt.Errorf("unknown chain cert spec: %q", spec)
		}
	}

	return tlsCert, nil
}

// generateDeepChain creates a 3-level certificate hierarchy (root -> intermediate -> leaf)
// to test chain depth validation. The device should reject this because MASH limits
// certificate chain depth to 2.
func generateDeepChain(specs []string) (tls.Certificate, error) {
	now := time.Now()

	// 1. Root CA (self-signed).
	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate root key: %w", err)
	}
	rootTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Root CA", Organization: []string{"MASH Test"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create root cert: %w", err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse root cert: %w", err)
	}

	// 2. Intermediate CA (signed by root).
	intermediateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate intermediate key: %w", err)
	}
	intermediateTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Test Intermediate CA", Organization: []string{"MASH Test"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}
	intermediateDER, err := x509.CreateCertificate(rand.Reader, intermediateTemplate, rootCert, &intermediateKey.PublicKey, rootKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create intermediate cert: %w", err)
	}

	// 3. Leaf (signed by intermediate).
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate leaf key: %w", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "test-controller-deep", Organization: []string{"MASH Test"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	intermediateCert, err := x509.ParseCertificate(intermediateDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse intermediate cert: %w", err)
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, intermediateCert, &leafKey.PublicKey, intermediateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create leaf cert: %w", err)
	}
	leafCert, err := x509.ParseCertificate(leafDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse leaf cert: %w", err)
	}

	// Build chain in spec order.
	chain := [][]byte{leafDER}
	for _, spec := range specs[1:] {
		switch spec {
		case "intermediate_ca":
			chain = append(chain, intermediateDER)
		case "root_ca":
			chain = append(chain, rootDER)
		}
	}

	return tls.Certificate{
		Certificate: chain,
		PrivateKey:  leafKey,
		Leaf:        leafCert,
	}, nil
}

// generateInvalidSignatureCert creates a certificate with a corrupted DER signature.
// The cert looks structurally valid but signature verification will fail, triggering
// a bad_certificate TLS alert.
func generateInvalidSignatureCert(zoneCA *cert.ZoneCA) (tls.Certificate, error) {
	if zoneCA == nil {
		return tls.Certificate{}, fmt.Errorf("invalid_signature_cert requires a zone CA")
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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
			CommonName:   "test-controller-invalid",
			Organization: []string{"MASH Test"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, zoneCA.Certificate, &privKey.PublicKey, zoneCA.PrivateKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	// Corrupt the signature by flipping bits in the last 10 bytes of the DER.
	corrupted := make([]byte, len(certDER))
	copy(corrupted, certDER)
	for i := len(corrupted) - 10; i < len(corrupted); i++ {
		corrupted[i] ^= 0xFF
	}

	return tls.Certificate{
		Certificate: [][]byte{corrupted},
		PrivateKey:  privKey,
		// No Leaf -- Go will parse it lazily, but the peer validates the raw DER.
	}, nil
}

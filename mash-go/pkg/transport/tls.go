package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"
)

// TLS constants for MASH protocol.
const (
	// ALPN protocol identifier for MASH.
	ALPNProtocol = "mash/1"

	// DefaultPort is the default MASH port.
	DefaultPort = 8443
)

// TLSConfig holds configuration for MASH TLS connections.
type TLSConfig struct {
	// Certificate is the TLS certificate for this endpoint.
	Certificate tls.Certificate

	// RootCAs is the pool of trusted CA certificates.
	// For devices: Zone CAs that have commissioned this device.
	// For controllers: Zone CA (own) plus any device attestation CAs.
	RootCAs *x509.CertPool

	// ClientCAs is the pool of CA certificates for client authentication.
	// Only used by devices (servers) to verify controller certificates.
	ClientCAs *x509.CertPool

	// ServerName is the expected server name for client connections.
	// Used for certificate verification.
	ServerName string

	// InsecureSkipVerify disables certificate verification.
	// Only for testing - never use in production!
	InsecureSkipVerify bool

	// VerifyPeerCertificate is an optional callback for custom certificate verification.
	VerifyPeerCertificate func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error
}

// NewServerTLSConfig creates a TLS configuration for a MASH device (server).
func NewServerTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("TLSConfig is required")
	}
	if len(cfg.Certificate.Certificate) == 0 {
		return nil, fmt.Errorf("server certificate is required")
	}

	tlsConfig := &tls.Config{
		// TLS 1.3 only - no fallback
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,

		// Require client certificate (mutual TLS)
		ClientAuth: tls.RequireAndVerifyClientCert,

		// Certificate for this device
		Certificates: []tls.Certificate{cfg.Certificate},

		// CA pool for verifying client certificates
		ClientCAs: cfg.ClientCAs,

		// ALPN protocol
		NextProtos: []string{ALPNProtocol},

		// Cipher suites (TLS 1.3 uses different config, but we set preferences)
		// Go's TLS 1.3 implementation automatically uses the right ciphers
		CipherSuites: nil, // TLS 1.3 ignores this, uses built-in secure ciphers

		// Curve preferences for key exchange
		CurvePreferences: []tls.CurveID{
			tls.X25519,    // Recommended
			tls.CurveP256, // Mandatory
		},

		// Session tickets disabled (no resumption)
		SessionTicketsDisabled: true,

		// Custom verification callback
		VerifyPeerCertificate: cfg.VerifyPeerCertificate,
	}

	// For testing only
	if cfg.InsecureSkipVerify {
		tlsConfig.ClientAuth = tls.RequestClientCert
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

// NewClientTLSConfig creates a TLS configuration for a MASH controller (client).
func NewClientTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("TLSConfig is required")
	}
	if len(cfg.Certificate.Certificate) == 0 {
		return nil, fmt.Errorf("client certificate is required")
	}

	tlsConfig := &tls.Config{
		// TLS 1.3 only - no fallback
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,

		// Certificate for this controller
		Certificates: []tls.Certificate{cfg.Certificate},

		// CA pool for verifying server certificates
		RootCAs: cfg.RootCAs,

		// Server name for verification
		ServerName: cfg.ServerName,

		// ALPN protocol
		NextProtos: []string{ALPNProtocol},

		// Curve preferences for key exchange
		CurvePreferences: []tls.CurveID{
			tls.X25519,    // Recommended
			tls.CurveP256, // Mandatory
		},

		// Session tickets disabled (no resumption)
		SessionTicketsDisabled: true,

		// Custom verification callback
		VerifyPeerCertificate: cfg.VerifyPeerCertificate,

		// For testing only
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	return tlsConfig, nil
}

// NewCommissioningTLSConfig creates a TLS configuration for commissioning.
// During commissioning, certificate verification is skipped because the
// device uses a self-signed certificate. Security comes from SPAKE2+.
func NewCommissioningTLSConfig() *tls.Config {
	return &tls.Config{
		// TLS 1.3 only
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,

		// Skip verification during commissioning
		// Security is provided by SPAKE2+ (PASE)
		InsecureSkipVerify: true,

		// ALPN protocol
		NextProtos: []string{ALPNProtocol},

		// Curve preferences
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},

		// Session tickets disabled
		SessionTicketsDisabled: true,
	}
}

// VerifyTLS13 checks that a TLS connection is using TLS 1.3.
func VerifyTLS13(state tls.ConnectionState) error {
	if state.Version != tls.VersionTLS13 {
		return fmt.Errorf("TLS version %x is not TLS 1.3 (0x0304)", state.Version)
	}
	return nil
}

// VerifyALPN checks that the negotiated ALPN protocol is correct.
func VerifyALPN(state tls.ConnectionState) error {
	if state.NegotiatedProtocol != ALPNProtocol {
		return fmt.Errorf("ALPN protocol %q is not %q", state.NegotiatedProtocol, ALPNProtocol)
	}
	return nil
}

// VerifyConnection performs standard MASH connection verification.
func VerifyConnection(state tls.ConnectionState) error {
	if err := VerifyTLS13(state); err != nil {
		return err
	}
	if err := VerifyALPN(state); err != nil {
		return err
	}
	return nil
}

// OperationalTLSConfig holds the certificates needed for operational TLS connections.
// This is a convenience wrapper for building TLS configs for controllers connecting
// to devices after commissioning, when both sides have operational certificates.
type OperationalTLSConfig struct {
	// ControllerCert is the controller's operational certificate as tls.Certificate.
	ControllerCert tls.Certificate

	// ZoneCAs is the pool of Zone CA certificates for verifying device certs.
	ZoneCAs *x509.CertPool

	// ServerName is the expected server name (typically device ID).
	ServerName string
}

// NewOperationalClientTLSConfig creates a TLS configuration for a controller
// connecting to a device for operational (post-commissioning) communication.
// Both sides present operational certificates signed by the same Zone CA.
//
// Unlike typical TLS, MASH uses device IDs (not DNS names) for identification.
// Certificate verification focuses on the chain (Zone CA) rather than hostname.
func NewOperationalClientTLSConfig(cfg *OperationalTLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("OperationalTLSConfig is required")
	}
	if len(cfg.ControllerCert.Certificate) == 0 {
		return nil, fmt.Errorf("controller certificate is required")
	}
	if cfg.ZoneCAs == nil {
		return nil, fmt.Errorf("zone CA pool is required for peer verification")
	}

	// Capture the Zone CAs for use in the verification callback
	zoneCAs := cfg.ZoneCAs
	expectedDeviceID := cfg.ServerName

	// Create custom verification that:
	// 1. Verifies certificate chain via Zone CA (manually, since we skip hostname verification)
	// 2. Verifies device ID in certificate matches expected
	verifyPeer := func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("no certificates presented")
		}

		// Parse the peer certificate
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}

		// Build intermediate pool from remaining certs
		intermediates := x509.NewCertPool()
		for _, rawCert := range rawCerts[1:] {
			intermediateCert, err := x509.ParseCertificate(rawCert)
			if err != nil {
				continue
			}
			intermediates.AddCert(intermediateCert)
		}

		// Verify the certificate chain against Zone CA
		opts := x509.VerifyOptions{
			Roots:         zoneCAs,
			Intermediates: intermediates,
			CurrentTime:   time.Now(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}
		if _, err := cert.Verify(opts); err != nil {
			return fmt.Errorf("certificate chain verification failed: %w", err)
		}

		// Verify device ID matches if expected
		if expectedDeviceID != "" {
			if cert.Subject.CommonName != expectedDeviceID {
				// Also check DNS SANs for newer certificates
				found := false
				for _, dns := range cert.DNSNames {
					if dns == expectedDeviceID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("device ID mismatch: expected %s, got %s",
						expectedDeviceID, cert.Subject.CommonName)
				}
			}
		}

		return nil
	}

	// Build TLS config with custom verification
	// InsecureSkipVerify skips Go's built-in hostname verification
	// Our verifyPeer callback handles chain and device ID verification
	return NewClientTLSConfig(&TLSConfig{
		Certificate:           cfg.ControllerCert,
		RootCAs:               cfg.ZoneCAs,
		ServerName:            cfg.ServerName, // Used for SNI only
		VerifyPeerCertificate: verifyPeer,
		InsecureSkipVerify:    true, // We handle verification in verifyPeer
	})
}

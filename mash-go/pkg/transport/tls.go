package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
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

package runner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

func TestDialer_CommissioningTLSConfig_HasCorrectVersion(t *testing.T) {
	cfg := transport.NewCommissioningTLSConfig()
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected MinVersion TLS 1.3, got %d", cfg.MinVersion)
	}
	if cfg.MaxVersion != tls.VersionTLS13 {
		t.Errorf("expected MaxVersion TLS 1.3, got %d", cfg.MaxVersion)
	}
}

func TestDialer_CommissioningTLSConfig_HasCorrectALPN(t *testing.T) {
	cfg := transport.NewCommissioningTLSConfig()
	want := []string{transport.ALPNCommissioningProtocol}
	if len(cfg.NextProtos) != len(want) || cfg.NextProtos[0] != want[0] {
		t.Errorf("expected ALPN %v, got %v", want, cfg.NextProtos)
	}
}

func TestDialer_CommissioningTLSConfig_HasCorrectCurves(t *testing.T) {
	cfg := transport.NewCommissioningTLSConfig()
	wantCurves := []tls.CurveID{tls.X25519, tls.CurveP256}
	if len(cfg.CurvePreferences) != len(wantCurves) {
		t.Fatalf("expected %d curves, got %d", len(wantCurves), len(cfg.CurvePreferences))
	}
	for i, c := range wantCurves {
		if cfg.CurvePreferences[i] != c {
			t.Errorf("curve[%d]: expected %v, got %v", i, c, cfg.CurvePreferences[i])
		}
	}
}

func TestDialer_OperationalTLSConfig_WithCerts_IncludesCertificates(t *testing.T) {
	opCert := makeTestOperationalCert(t)
	crypto := CryptoState{
		ControllerCert: opCert,
	}

	nop := func(string, ...any) {}
	cfg := buildOperationalTLSConfig(crypto, false, nop)

	if len(cfg.Certificates) != 1 {
		t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
	}
	if len(cfg.NextProtos) != 1 || cfg.NextProtos[0] != transport.ALPNProtocol {
		t.Errorf("expected ALPN [%s], got %v", transport.ALPNProtocol, cfg.NextProtos)
	}
}

func TestDialer_OperationalTLSConfig_WithCAPool_SetsVerifyPeerCertificate(t *testing.T) {
	pool := x509.NewCertPool()
	crypto := CryptoState{
		ZoneCAPool: pool,
	}

	nop := func(string, ...any) {}
	cfg := buildOperationalTLSConfig(crypto, false, nop)

	if !cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true when ZoneCAPool is set")
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Error("expected VerifyPeerCertificate to be set when ZoneCAPool is provided")
	}
}

func TestDialer_OperationalTLSConfig_NoCerts_SkipsVerification(t *testing.T) {
	crypto := CryptoState{} // No certs, no CA pool.

	nop := func(string, ...any) {}

	// Without insecureSkipVerify.
	cfg := buildOperationalTLSConfig(crypto, false, nop)
	if cfg.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=false when no CA pool and not insecure")
	}
	if cfg.VerifyPeerCertificate != nil {
		t.Error("expected no VerifyPeerCertificate when no CA pool")
	}
	if len(cfg.Certificates) != 0 {
		t.Errorf("expected 0 certificates, got %d", len(cfg.Certificates))
	}

	// With insecureSkipVerify.
	cfg2 := buildOperationalTLSConfig(crypto, true, nop)
	if !cfg2.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true when insecure flag is set")
	}
}

// makeTestOperationalCert creates a minimal self-signed cert for testing.
func makeTestOperationalCert(t *testing.T) *cert.OperationalCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leafCert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		t.Fatal(err)
	}

	return &cert.OperationalCert{
		Certificate: leafCert,
		PrivateKey:  key,
	}
}

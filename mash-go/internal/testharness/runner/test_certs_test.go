package runner

import (
	"crypto/x509"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func newTestZoneCA(t *testing.T) *cert.ZoneCA {
	t.Helper()
	ca, err := cert.GenerateZoneCA("test-zone-id", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA: %v", err)
	}
	return ca
}

func TestGenerateTestClientCert_NotYetValid(t *testing.T) {
	ca := newTestZoneCA(t)
	tlsCert, err := generateTestClientCert("controller_not_yet_valid", ca)
	if err != nil {
		t.Fatalf("generateTestClientCert: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("expected non-nil Leaf")
	}
	if !tlsCert.Leaf.NotBefore.After(time.Now()) {
		t.Errorf("NotBefore %v should be in the future", tlsCert.Leaf.NotBefore)
	}
}

func TestGenerateTestClientCert_Expired(t *testing.T) {
	ca := newTestZoneCA(t)
	tlsCert, err := generateTestClientCert("controller_expired", ca)
	if err != nil {
		t.Fatalf("generateTestClientCert: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("expected non-nil Leaf")
	}
	if !tlsCert.Leaf.NotAfter.Before(time.Now()) {
		t.Errorf("NotAfter %v should be in the past", tlsCert.Leaf.NotAfter)
	}
}

func TestGenerateTestClientCert_WrongZone(t *testing.T) {
	ca := newTestZoneCA(t)
	tlsCert, err := generateTestClientCert("controller_wrong_zone", ca)
	if err != nil {
		t.Fatalf("generateTestClientCert: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("expected non-nil Leaf")
	}
	// Verify that the cert is NOT signed by our zone CA.
	pool := x509.NewCertPool()
	pool.AddCert(ca.Certificate)
	_, verifyErr := tlsCert.Leaf.Verify(x509.VerifyOptions{Roots: pool})
	if verifyErr == nil {
		t.Error("expected verification against zone CA to fail for wrong_zone cert")
	}
}

func TestGenerateTestClientCert_NoClientAuth(t *testing.T) {
	ca := newTestZoneCA(t)
	tlsCert, err := generateTestClientCert("controller_no_client_auth", ca)
	if err != nil {
		t.Fatalf("generateTestClientCert: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("expected non-nil Leaf")
	}
	for _, usage := range tlsCert.Leaf.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			t.Error("cert should not have ExtKeyUsageClientAuth")
		}
	}
}

func TestGenerateTestClientCert_CATrue(t *testing.T) {
	ca := newTestZoneCA(t)
	tlsCert, err := generateTestClientCert("controller_ca_true", ca)
	if err != nil {
		t.Fatalf("generateTestClientCert: %v", err)
	}
	if tlsCert.Leaf == nil {
		t.Fatal("expected non-nil Leaf")
	}
	if !tlsCert.Leaf.IsCA {
		t.Error("expected IsCA=true")
	}
}

func TestGenerateTestClientCert_UnknownType(t *testing.T) {
	ca := newTestZoneCA(t)
	_, err := generateTestClientCert("bogus_type", ca)
	if err == nil {
		t.Error("expected error for unknown cert type")
	}
}

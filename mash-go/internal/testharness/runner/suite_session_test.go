package runner

import (
	"crypto/x509"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestSuiteSession_InitiallyNotCommissioned(t *testing.T) {
	s := NewSuiteSession()
	if s.IsCommissioned() {
		t.Fatal("new SuiteSession should not be commissioned")
	}
}

func TestSuiteSession_ZoneID_ReturnsEmpty_WhenNotCommissioned(t *testing.T) {
	s := NewSuiteSession()
	if got := s.ZoneID(); got != "" {
		t.Fatalf("ZoneID() = %q, want empty", got)
	}
}

func TestSuiteSession_ConnKey_ReturnsEmpty_WhenNotCommissioned(t *testing.T) {
	s := NewSuiteSession()
	if got := s.ConnKey(); got != "" {
		t.Fatalf("ConnKey() = %q, want empty", got)
	}
}

func TestSuiteSession_Crypto_ReturnsZeroValues_WhenNotCommissioned(t *testing.T) {
	s := NewSuiteSession()
	cs := s.Crypto()
	if cs.ZoneCA != nil {
		t.Fatal("Crypto().ZoneCA should be nil when not commissioned")
	}
	if cs.ControllerCert != nil {
		t.Fatal("Crypto().ControllerCert should be nil when not commissioned")
	}
	if cs.ZoneCAPool != nil {
		t.Fatal("Crypto().ZoneCAPool should be nil when not commissioned")
	}
	if cs.IssuedDeviceCert != nil {
		t.Fatal("Crypto().IssuedDeviceCert should be nil when not commissioned")
	}
}

func TestSuiteSession_Record_StoresZoneIDAndCrypto(t *testing.T) {
	s := NewSuiteSession()
	crypto := makeCryptoState()

	s.Record("zone-abc", crypto)

	if !s.IsCommissioned() {
		t.Fatal("should be commissioned after Record")
	}
	if got := s.ZoneID(); got != "zone-abc" {
		t.Fatalf("ZoneID() = %q, want %q", got, "zone-abc")
	}
}

func TestSuiteSession_ConnKey_ReturnsMainPrefixedZoneID(t *testing.T) {
	s := NewSuiteSession()
	s.Record("zone-abc", makeCryptoState())

	if got := s.ConnKey(); got != "main-zone-abc" {
		t.Fatalf("ConnKey() = %q, want %q", got, "main-zone-abc")
	}
}

func TestSuiteSession_Crypto_ReturnsStoredMaterial(t *testing.T) {
	s := NewSuiteSession()
	crypto := makeCryptoState()

	s.Record("zone-abc", crypto)

	got := s.Crypto()
	if got.ZoneCA != crypto.ZoneCA {
		t.Fatal("ZoneCA pointer mismatch")
	}
	if got.ControllerCert != crypto.ControllerCert {
		t.Fatal("ControllerCert pointer mismatch")
	}
	if got.ZoneCAPool != crypto.ZoneCAPool {
		t.Fatal("ZoneCAPool pointer mismatch")
	}
	if got.IssuedDeviceCert != crypto.IssuedDeviceCert {
		t.Fatal("IssuedDeviceCert pointer mismatch")
	}
}

func TestSuiteSession_Clear_ResetsEverything(t *testing.T) {
	s := NewSuiteSession()
	s.Record("zone-abc", makeCryptoState())
	s.Clear()

	if s.IsCommissioned() {
		t.Fatal("should not be commissioned after Clear")
	}
	if got := s.ZoneID(); got != "" {
		t.Fatalf("ZoneID() = %q, want empty after Clear", got)
	}
	if got := s.ConnKey(); got != "" {
		t.Fatalf("ConnKey() = %q, want empty after Clear", got)
	}
	cs := s.Crypto()
	if cs.ZoneCA != nil || cs.ControllerCert != nil || cs.ZoneCAPool != nil || cs.IssuedDeviceCert != nil {
		t.Fatal("Crypto() should return zero values after Clear")
	}
}

func TestSuiteSession_Clear_ThenIsCommissioned_ReturnsFalse(t *testing.T) {
	s := NewSuiteSession()
	s.Record("zone-abc", makeCryptoState())

	if !s.IsCommissioned() {
		t.Fatal("should be commissioned before Clear")
	}
	s.Clear()
	if s.IsCommissioned() {
		t.Fatal("should not be commissioned after Clear")
	}
}

func TestSuiteSession_Record_OverwritesPreviousState(t *testing.T) {
	s := NewSuiteSession()
	crypto1 := makeCryptoState()
	s.Record("zone-first", crypto1)

	crypto2 := CryptoState{
		ZoneCA:           &cert.ZoneCA{ZoneID: "second"},
		ControllerCert:   &cert.OperationalCert{ZoneID: "second"},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{SerialNumber: nil},
	}
	s.Record("zone-second", crypto2)

	if got := s.ZoneID(); got != "zone-second" {
		t.Fatalf("ZoneID() = %q, want %q after overwrite", got, "zone-second")
	}
	if got := s.ConnKey(); got != "main-zone-second" {
		t.Fatalf("ConnKey() = %q, want %q after overwrite", got, "main-zone-second")
	}
	if got := s.Crypto(); got.ZoneCA != crypto2.ZoneCA {
		t.Fatal("Crypto().ZoneCA should point to second crypto after overwrite")
	}
}

func TestSuiteSession_CloseConn_ClosesConnectionButPreservesState(t *testing.T) {
	s := NewSuiteSession()
	crypto := makeCryptoState()
	s.Record("zone-abc", crypto)

	conn := &Connection{state: ConnOperational}
	s.SetConn(conn)

	s.CloseConn()

	// Connection should be closed.
	if conn.isConnected() {
		t.Error("expected connection to be closed after CloseConn")
	}

	// Conn() should return nil after CloseConn.
	if s.Conn() != nil {
		t.Error("expected Conn() to return nil after CloseConn")
	}

	// Zone ID, crypto, and commissioned state should be preserved.
	if !s.IsCommissioned() {
		t.Error("expected IsCommissioned() to remain true after CloseConn")
	}
	if got := s.ZoneID(); got != "zone-abc" {
		t.Errorf("ZoneID() = %q, want %q after CloseConn", got, "zone-abc")
	}
	if got := s.ConnKey(); got != "main-zone-abc" {
		t.Errorf("ConnKey() = %q, want %q after CloseConn", got, "main-zone-abc")
	}
	cs := s.Crypto()
	if cs.ZoneCA != crypto.ZoneCA {
		t.Error("expected Crypto().ZoneCA to be preserved after CloseConn")
	}
}

func TestSuiteSession_CloseConn_NoopWhenNoConnection(t *testing.T) {
	s := NewSuiteSession()
	s.Record("zone-abc", makeCryptoState())

	// Should not panic when conn is nil.
	s.CloseConn()

	if !s.IsCommissioned() {
		t.Error("expected IsCommissioned() to remain true")
	}
}

func TestSuiteSession_CloseConn_NoopWhenNotCommissioned(t *testing.T) {
	s := NewSuiteSession()

	// Should not panic when nothing is set.
	s.CloseConn()

	if s.IsCommissioned() {
		t.Error("expected IsCommissioned() to remain false")
	}
}

// makeCryptoState creates a non-zero CryptoState for testing.
func makeCryptoState() CryptoState {
	return CryptoState{
		ZoneCA:           &cert.ZoneCA{ZoneID: "test"},
		ControllerCert:   &cert.OperationalCert{ZoneID: "test"},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{},
	}
}

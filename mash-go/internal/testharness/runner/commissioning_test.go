package runner

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestEnsureCommissioned_AlreadyDone(t *testing.T) {
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.paseState = &PASEState{
		completed:  true,
		sessionKey: []byte{1, 2, 3, 4},
	}
	state := engine.NewExecutionState(context.Background())

	err := r.ensureCommissioned(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error when already commissioned: %v", err)
	}

	// Verify state was populated
	v, ok := state.Get(PrecondSessionEstablished)
	if !ok {
		t.Fatal("expected session_established in state")
	}
	if v != true {
		t.Errorf("expected session_established=true, got %v", v)
	}

	v, ok = state.Get(PrecondConnectionEstablished)
	if !ok {
		t.Fatal("expected connection_established in state")
	}
	if v != true {
		t.Errorf("expected connection_established=true, got %v", v)
	}
}

func TestEnsureCommissioned_RestoresSuiteZoneCrypto(t *testing.T) {
	// When a lower-level test clears zoneCAPool and a subsequent
	// commissioned test reuses the suite zone session, ensureCommissioned
	// must restore the crypto from the saved suite zone state.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	// Simulate suite zone crypto saved during recordSuiteZone.
	r.suite.Record("suite-zone-123", CryptoState{
		ZoneCA:           &cert.ZoneCA{},
		ControllerCert:   &cert.OperationalCert{},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{},
	})

	// Working crypto is nil (cleared by a previous non-commissioned test).
	r.zoneCA = nil
	r.controllerCert = nil
	r.zoneCAPool = nil
	r.issuedDeviceCert = nil

	state := engine.NewExecutionState(context.Background())
	err := r.ensureCommissioned(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have restored from suite zone crypto.
	// The pool is created fresh (since it was nil) and suite CA cert is added
	// if non-nil. With a nil Certificate in suiteZoneCA the pool is empty but non-nil.
	if r.zoneCAPool == nil {
		t.Error("expected zoneCAPool to be created during suite zone crypto restore")
	}
	if r.zoneCA != r.suite.Crypto().ZoneCA {
		t.Error("expected zoneCA to be restored from suite zone crypto")
	}
	if r.controllerCert != r.suite.Crypto().ControllerCert {
		t.Error("expected controllerCert to be restored from suite zone crypto")
	}
}

func TestEnsureCommissioned_NoStaleSuiteRestore_AfterEnsureDisconnected(t *testing.T) {
	// After ensureDisconnected clears suite crypto, ensureCommissioned's
	// session-reuse path must NOT restore stale suite crypto.
	// This tests the fix for TC-IPV6-004/TC-TLS-CTRL-006 shuffle failures.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}

	// Simulate state after ensureDisconnected + fresh commission:
	// Suite session is empty (simulating state after ensureDisconnected + fresh commission).
	// NewSuiteSession() starts empty, so no explicit setup needed.

	// Current crypto from the fresh commission.
	freshCA := &cert.ZoneCA{}
	freshPool := x509.NewCertPool()
	r.zoneCA = freshCA
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = freshPool
	r.issuedDeviceCert = &x509.Certificate{}

	state := engine.NewExecutionState(context.Background())
	err := r.ensureCommissioned(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Current crypto should remain unchanged (not replaced by nil suite crypto).
	if r.zoneCA != freshCA {
		t.Error("expected zoneCA to remain as fresh commission crypto")
	}
	if r.zoneCAPool != freshPool {
		t.Error("expected zoneCAPool to remain as fresh commission crypto")
	}
}

func TestDeriveZoneIDFromSecret(t *testing.T) {
	// Verify deterministic derivation matches device-side logic:
	// hex(SHA-256(secret)[:8])
	secret := []byte("test-shared-secret")
	zoneID := deriveZoneIDFromSecret(secret)

	// Should be 16 hex chars (8 bytes)
	if len(zoneID) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %q", len(zoneID), zoneID)
	}

	// Must be deterministic
	if zoneID != deriveZoneIDFromSecret(secret) {
		t.Error("deriveZoneIDFromSecret is not deterministic")
	}

	// Different secret -> different zone ID
	other := deriveZoneIDFromSecret([]byte("different-secret"))
	if zoneID == other {
		t.Error("different secrets should produce different zone IDs")
	}
}

func TestCooldownRemaining(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool // whether a positive duration is expected
	}{
		{"nil error", nil, false},
		{"unrelated error", fmt.Errorf("connection refused"), false},
		{"cooldown error", fmt.Errorf("PASE handshake failed: server error: cooldown active (460.930083ms remaining) (code 5)"), true},
		{"short cooldown", fmt.Errorf("cooldown active (10ms remaining)"), true},
		{"malformed", fmt.Errorf("cooldown active (badvalue remaining)"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := cooldownRemaining(tt.err)
			if tt.want && d <= 0 {
				t.Errorf("expected positive duration, got %v", d)
			}
			if !tt.want && d != 0 {
				t.Errorf("expected 0, got %v", d)
			}
		})
	}
}

func TestIsSuiteZoneCommission_NoSuiteZone(t *testing.T) {
	r := newTestRunner()
	// suite starts empty (not commissioned)
	if r.isSuiteZoneCommission() {
		t.Error("expected false when no suite zone exists")
	}
}

func TestIsSuiteZoneCommission_SuiteZoneAlive(t *testing.T) {
	// When the suite zone connection is alive, a new commission is for a
	// secondary zone (GRID/LOCAL), not the suite zone itself.
	r := newTestRunner()
	r.suite.Record("suite-123", CryptoState{})
	suiteConn := &Connection{state: ConnOperational}
	r.pool.TrackZone("main-suite-123", suiteConn, "suite-123")

	if r.isSuiteZoneCommission() {
		t.Error("expected false when suite zone connection is alive")
	}
}

func TestIsSuiteZoneCommission_SuiteZoneDead(t *testing.T) {
	// When the suite zone connection is dead, a new commission replaces it.
	r := newTestRunner()
	r.suite.Record("suite-123", CryptoState{})
	suiteConn := &Connection{state: ConnDisconnected}
	r.pool.TrackZone("main-suite-123", suiteConn, "suite-123")

	if !r.isSuiteZoneCommission() {
		t.Error("expected true when suite zone connection is dead")
	}
}

func TestIsSuiteZoneCommission_SuiteZoneMissing(t *testing.T) {
	// When the suite zone connection is not in pool zones (cleaned up),
	// a new commission replaces it.
	r := newTestRunner()
	r.suite.Record("suite-123", CryptoState{})

	if !r.isSuiteZoneCommission() {
		t.Error("expected true when suite zone connection is missing from pool zones")
	}
}

func TestEnsureDisconnected_ClearsSuiteCrypto(t *testing.T) {
	// When ensureDisconnected is called (fresh_commission, suite teardown),
	// it must clear both current AND suite crypto. Without this, a subsequent
	// fresh commission creates new crypto but suiteZoneCAPool still points
	// to the old CA, causing "unknown_ca" TLS failures when ensureCommissioned's
	// session-reuse path restores the stale suite crypto.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	// Simulate suite zone crypto.
	r.suite.Record("suite-zone-123", CryptoState{
		ZoneCA:           &cert.ZoneCA{},
		ControllerCert:   &cert.OperationalCert{},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{},
	})

	// Current crypto.
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()
	r.issuedDeviceCert = &x509.Certificate{}

	r.ensureDisconnected()

	// All current crypto should be cleared.
	if r.zoneCA != nil {
		t.Error("expected zoneCA to be nil after ensureDisconnected")
	}
	if r.controllerCert != nil {
		t.Error("expected controllerCert to be nil after ensureDisconnected")
	}
	if r.zoneCAPool != nil {
		t.Error("expected zoneCAPool to be nil after ensureDisconnected")
	}
	if r.issuedDeviceCert != nil {
		t.Error("expected issuedDeviceCert to be nil after ensureDisconnected")
	}

	// Suite session should be fully cleared.
	if r.suite.IsCommissioned() {
		t.Error("expected suite to not be commissioned after ensureDisconnected")
	}
	if r.suite.ZoneID() != "" {
		t.Error("expected suite ZoneID to be empty after ensureDisconnected")
	}
	if r.suite.ConnKey() != "" {
		t.Error("expected suite ConnKey to be empty after ensureDisconnected")
	}
	crypto := r.suite.Crypto()
	if crypto.ZoneCA != nil || crypto.ControllerCert != nil || crypto.ZoneCAPool != nil || crypto.IssuedDeviceCert != nil {
		t.Error("expected suite crypto to be nil after ensureDisconnected")
	}
}

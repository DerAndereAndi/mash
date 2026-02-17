package runner

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestEnsureCommissioned_AlreadyDone(t *testing.T) {
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.connMgr.SetPASEState(&PASEState{
		completed:  true,
		sessionKey: []byte{1, 2, 3, 4},
	})
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
	r.connMgr.SetPASEState(&PASEState{completed: true, sessionKey: []byte{1, 2, 3}})
	// Simulate suite zone crypto saved during recordSuiteZone.
	r.suite.Record("suite-zone-123", CryptoState{
		ZoneCA:           &cert.ZoneCA{},
		ControllerCert:   &cert.OperationalCert{},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{},
	})

	// Working crypto is nil (cleared by a previous non-commissioned test).
	r.connMgr.SetZoneCA(nil)
	r.connMgr.SetControllerCert(nil)
	r.connMgr.SetZoneCAPool(nil)
	r.connMgr.SetIssuedDeviceCert(nil)

	state := engine.NewExecutionState(context.Background())
	err := r.ensureCommissioned(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have restored from suite zone crypto.
	// The pool is created fresh (since it was nil) and suite CA cert is added
	// if non-nil. With a nil Certificate in suiteZoneCA the pool is empty but non-nil.
	if r.connMgr.ZoneCAPool() == nil {
		t.Error("expected zoneCAPool to be created during suite zone crypto restore")
	}
	if r.connMgr.ZoneCA() != r.suite.Crypto().ZoneCA {
		t.Error("expected zoneCA to be restored from suite zone crypto")
	}
	if r.connMgr.ControllerCert() != r.suite.Crypto().ControllerCert {
		t.Error("expected controllerCert to be restored from suite zone crypto")
	}
}

func TestEnsureCommissioned_NoStaleSuiteRestore_AfterEnsureDisconnected(t *testing.T) {
	// After ensureDisconnected clears suite crypto, ensureCommissioned's
	// session-reuse path must NOT restore stale suite crypto.
	// This tests the fix for TC-IPV6-004/TC-TLS-CTRL-006 shuffle failures.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.connMgr.SetPASEState(&PASEState{completed: true, sessionKey: []byte{1, 2, 3}})

	// Simulate state after ensureDisconnected + fresh commission:
	// Suite session is empty (simulating state after ensureDisconnected + fresh commission).
	// NewSuiteSession() starts empty, so no explicit setup needed.

	// Current crypto from the fresh commission.
	freshCA := &cert.ZoneCA{}
	freshPool := x509.NewCertPool()
	r.connMgr.SetZoneCA(freshCA)
	r.connMgr.SetControllerCert(&cert.OperationalCert{})
	r.connMgr.SetZoneCAPool(freshPool)
	r.connMgr.SetIssuedDeviceCert(&x509.Certificate{})

	state := engine.NewExecutionState(context.Background())
	err := r.ensureCommissioned(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Current crypto should remain unchanged (not replaced by nil suite crypto).
	if r.connMgr.ZoneCA() != freshCA {
		t.Error("expected zoneCA to remain as fresh commission crypto")
	}
	if r.connMgr.ZoneCAPool() != freshPool {
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
	r.suite.SetConn(suiteConn)

	if r.isSuiteZoneCommission() {
		t.Error("expected false when suite zone connection is alive")
	}
}

func TestIsSuiteZoneCommission_SuiteZoneDead(t *testing.T) {
	// When the suite zone connection is dead, a new commission replaces it.
	r := newTestRunner()
	r.suite.Record("suite-123", CryptoState{})
	suiteConn := &Connection{state: ConnDisconnected}
	r.suite.SetConn(suiteConn)

	if !r.isSuiteZoneCommission() {
		t.Error("expected true when suite zone connection is dead")
	}
}

func TestStrictCleanup_LeavesDeviceCommissionable(t *testing.T) {
	zoneCount := 0
	report := evaluateStrictCleanupContract(true, CommissioningStateCommissioned, &zoneCount, nil)
	if !report.IsClean() {
		t.Fatalf("expected clean strict cleanup contract, got issues: %v", report.Issues)
	}
}

func TestStrictCleanup_NoResidualZonesAfterGroupRun(t *testing.T) {
	zoneCount := 2
	report := evaluateStrictCleanupContract(true, CommissioningStateCommissioned, &zoneCount, nil)
	if report.IsClean() {
		t.Fatal("expected residual zone contract failure")
	}
}

func TestStrictCleanup_NoUnknownAuthorityCascadeAfterRecommission(t *testing.T) {
	zoneCount := 0
	report := evaluateStrictCleanupContract(true, CommissioningStateCommissioned, &zoneCount, errors.New("tls: unknown authority"))
	if report.IsClean() {
		t.Fatal("expected reconnect probe failure in strict cleanup contract")
	}
}

func TestStrictCleanup_ZoneCountUnavailableWithHealthyProbeIsNonFatal(t *testing.T) {
	report := evaluateStrictCleanupContract(true, CommissioningStateCommissioned, nil, nil)
	if !report.IsClean() {
		t.Fatalf("expected clean strict cleanup contract when probe is healthy and zone count is unavailable, got issues: %v", report.Issues)
	}
}

func TestIsSuiteZoneCommission_SuiteConnNil(t *testing.T) {
	// When suite.Conn() is nil, a new commission replaces it.
	r := newTestRunner()
	r.suite.Record("suite-123", CryptoState{})

	if !r.isSuiteZoneCommission() {
		t.Error("expected true when suite conn is nil")
	}
}

// TestRecordSuiteZone_DetachesMainFromSuite verifies that after
// recordSuiteZone(), pool.Main() is a fresh empty connection that is NOT
// the same object as suite.Conn(). The suite zone connection lives
// independently so tier transitions on pool.Main() never kill it.
func TestRecordSuiteZone_DetachesMainFromSuite(t *testing.T) {
	r := newTestRunner()
	// Simulate a successful commission: pool.Main() is operational.
	origConn := &Connection{state: ConnOperational}
	r.pool.SetMain(origConn)
	r.connMgr.SetPASEState(&PASEState{
		completed:  true,
		sessionKey: []byte("test-session-key"),
	})

	// Track the zone in the pool (transitionToOperational does this).
	zoneID := deriveZoneIDFromSecret([]byte("test-session-key"))
	connKey := "main-" + zoneID
	r.pool.TrackZone(connKey, origConn, zoneID)

	r.recordSuiteZone()

	// Suite should hold the original connection.
	if r.suite.Conn() != origConn {
		t.Error("expected suite.Conn() to be the original connection")
	}
	if !r.suite.Conn().isConnected() {
		t.Error("expected suite.Conn() to still be connected")
	}

	// pool.Main() must NOT be the same object as suite.Conn().
	if r.pool.Main() == r.suite.Conn() {
		t.Error("pool.Main() must be detached from suite.Conn() -- they should be different objects")
	}

	// pool.Main() should be a fresh, disconnected connection.
	if r.pool.Main().isConnected() {
		t.Error("pool.Main() should not be connected after detach")
	}
}

// TestRecordSuiteZone_UntracksFromPool_UsingTransitionKey verifies that
// recordSuiteZone properly untracks the suite zone from the pool when the
// zone was registered by transitionToOperational. The pool tracking key must
// match suite.ConnKey() ("main-"+zoneID) for the lookup to succeed.
//
// This is the core regression test for the x509 cascade bug: if the tracking
// key doesn't match, the zone stays in the pool and gets its crypto deleted
// by CloseZonesExcept during tier transitions.
func TestRecordSuiteZone_UntracksFromPool_UsingTransitionKey(t *testing.T) {
	r := newTestRunner()
	origConn := &Connection{state: ConnOperational}
	r.pool.SetMain(origConn)

	sessionKey := []byte("test-session-key")
	r.connMgr.SetPASEState(&PASEState{
		completed:  true,
		sessionKey: sessionKey,
	})

	zoneID := deriveZoneIDFromSecret(sessionKey)
	connKey := "main-" + zoneID

	// Simulate what transitionToOperational does after the fix:
	// TrackZone with key="main-"+zoneID to match suite.ConnKey().
	r.pool.TrackZone(connKey, origConn, zoneID)

	r.recordSuiteZone()

	// The zone MUST be untracked from the pool after recordSuiteZone
	// moves it to the suite session.
	if r.pool.Zone(connKey) != nil {
		t.Error("zone still tracked under main-prefixed key -- recordSuiteZone should have untracked it")
	}
	if r.pool.ZoneCount() != 0 {
		t.Errorf("expected 0 zones in pool after recordSuiteZone, got %d", r.pool.ZoneCount())
	}

	// Suite session should hold the connection.
	if r.suite.Conn() != origConn {
		t.Error("expected suite.Conn() to be the original connection")
	}
}

// TestRecordSuiteZone_CloseZonesExcept_PreservesAfterRecord verifies the
// full flow: transitionToOperational tracks → recordSuiteZone untracks →
// CloseZonesExcept does NOT fire onZoneClose for the suite zone.
func TestRecordSuiteZone_CloseZonesExcept_PreservesAfterRecord(t *testing.T) {
	var closedZoneIDs []string
	r := &Runner{
		config: &Config{},
		suite:  NewSuiteSession(),
	}
	r.pool = NewConnPool(func(string, ...any) {}, func(conn *Connection, zoneID string) {
		closedZoneIDs = append(closedZoneIDs, zoneID)
	})
	origConn := &Connection{state: ConnOperational}
	r.pool.SetMain(origConn)
	r.dialer = NewDialer(false, r.debugf)
	r.connMgr = NewConnectionManager(r.pool, r.suite, r.dialer, r.config, r.debugf, connMgrDeps{})

	sessionKey := []byte("test-session-key-2")
	r.connMgr.SetPASEState(&PASEState{
		completed:  true,
		sessionKey: sessionKey,
	})

	zoneID := deriveZoneIDFromSecret(sessionKey)
	connKey := "main-" + zoneID

	// Simulate transitionToOperational (fixed): track with "main-"+zoneID.
	r.pool.TrackZone(connKey, origConn, zoneID)

	r.recordSuiteZone()

	// Now add another zone and close all except suite.
	otherConn := &Connection{state: ConnOperational}
	r.pool.TrackZone("other-zone", otherConn, "other-id")
	r.pool.CloseZonesExcept(r.suite.ConnKey())

	// Only the "other" zone should have been closed.
	// The suite zone was properly untracked by recordSuiteZone,
	// so CloseZonesExcept won't touch it.
	if len(closedZoneIDs) != 1 || closedZoneIDs[0] != "other-id" {
		t.Errorf("expected only other-id to be closed, got %v", closedZoneIDs)
	}
}

// TestHandleCommission_ProhibitedSetupCode verifies that handleCommission
// returns a structured output (not a Go error) when given a prohibited
// low-entropy setup code. This is the fix for TC-PASE-006 / TC-PASE-007.
func TestHandleCommission_ProhibitedSetupCode(t *testing.T) {
	codes := []string{"11111111", "12345678", "87654321"}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			r := newTestRunner()
			step := &loader.Step{
				Action: "commission",
				Params: map[string]any{"setup_code": code},
			}
			state := engine.NewExecutionState(context.Background())

			out, err := r.handleCommission(context.Background(), step, state)
			// Must NOT return a Go error — the engine would treat that as
			// a harness failure, not a structured check result.
			if err != nil {
				t.Fatalf("expected structured output, got Go error: %v", err)
			}
			if out == nil {
				t.Fatal("expected non-nil output map")
			}
			errVal, ok := out[KeyError].(string)
			if !ok || errVal != "invalid_setup_code" {
				t.Errorf("expected error=invalid_setup_code, got %v", out[KeyError])
			}
			if out[KeySessionEstablished] != false {
				t.Errorf("expected session_established=false, got %v", out[KeySessionEstablished])
			}
		})
	}
}

// TestHandleCommission_OutOfRangeCodeNotProhibited verifies that out-of-range
// setup codes (like "00000000") are NOT caught by the prohibited code check.
// These should proceed to the PASE handshake and fail with VERIFICATION_FAILED
// (TC-PASE-002). Only codes on the DEC-068 prohibited list are rejected early.
func TestHandleCommission_OutOfRangeCodeNotProhibited(t *testing.T) {
	r := newTestRunner()
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{"setup_code": "00000000"},
	}
	state := engine.NewExecutionState(context.Background())

	out, err := r.handleCommission(context.Background(), step, state)
	// Should NOT return invalid_setup_code -- "00000000" is wrong but not prohibited.
	// It will proceed to connection and fail there (no real device in unit test).
	if out != nil {
		if errVal, ok := out[KeyError].(string); ok && errVal == "invalid_setup_code" {
			t.Error("out-of-range code 00000000 must NOT be treated as prohibited")
		}
	}
	// err is expected (connection failure in unit test), that's fine.
	_ = err
}

func TestEnsureDisconnected_ClearsSuiteCrypto(t *testing.T) {
	// When ensureDisconnected is called (fresh_commission, suite teardown),
	// it must clear both current AND suite crypto. Without this, a subsequent
	// fresh commission creates new crypto but suiteZoneCAPool still points
	// to the old CA, causing "unknown_ca" TLS failures when ensureCommissioned's
	// session-reuse path restores the stale suite crypto.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.connMgr.SetPASEState(&PASEState{completed: true, sessionKey: []byte{1, 2, 3}})
	// Simulate suite zone crypto.
	r.suite.Record("suite-zone-123", CryptoState{
		ZoneCA:           &cert.ZoneCA{},
		ControllerCert:   &cert.OperationalCert{},
		ZoneCAPool:       x509.NewCertPool(),
		IssuedDeviceCert: &x509.Certificate{},
	})

	// Current crypto.
	r.connMgr.SetZoneCA(&cert.ZoneCA{})
	r.connMgr.SetControllerCert(&cert.OperationalCert{})
	r.connMgr.SetZoneCAPool(x509.NewCertPool())
	r.connMgr.SetIssuedDeviceCert(&x509.Certificate{})

	r.ensureDisconnected()

	// All current crypto should be cleared.
	if r.connMgr.ZoneCA() != nil {
		t.Error("expected zoneCA to be nil after ensureDisconnected")
	}
	if r.connMgr.ControllerCert() != nil {
		t.Error("expected controllerCert to be nil after ensureDisconnected")
	}
	if r.connMgr.ZoneCAPool() != nil {
		t.Error("expected zoneCAPool to be nil after ensureDisconnected")
	}
	if r.connMgr.IssuedDeviceCert() != nil {
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

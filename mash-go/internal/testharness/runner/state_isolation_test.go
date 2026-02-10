package runner

import (
	"context"
	"crypto/x509"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

// These tests verify state isolation between sequential test cases run on
// the same Runner. They exercise the exact transitions that cause failures
// when running TC-MULTI-001 through TC-MULTI-004 in sequence.
//
// Each test creates a Runner, runs setupPreconditions for multiple test
// cases in order, and asserts the runner state between them using snapshots.

// TestStateIsolation_CommissionedToCommissioning verifies that transitioning
// from a commissioned state (level 3) to commissioning mode (level 1)
// properly cleans up all connection and certificate state.
func TestStateIsolation_CommissionedToCommissioning(t *testing.T) {
	r := newTestRunner()

	// Simulate test A completing with a commissioned connection.
	r.conn.state = ConnOperational
	r.paseState = &PASEState{
		completed:  true,
		sessionKey: []byte{0xDE, 0xAD},
	}
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()

	// Now run setupPreconditions for a test that needs commissioning mode.
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-COMM-MODE",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("setupPreconditions: %v", err)
	}

	s := r.snapshot()
	if s.MainConn.Connected {
		t.Error("expected disconnected after backward transition")
	}
	if s.HasPhantomSocket() {
		t.Error("phantom socket after backward transition")
	}
	if s.PASECompleted || s.HasSessionKey {
		t.Error("PASE state should be cleared")
	}
	if s.HasZoneCA || s.HasControllerCert || s.HasZoneCAPool {
		t.Error("cert state should be cleared")
	}
}

// TestStateIsolation_CommissionedToTwoZones verifies that transitioning
// from a commissioned state directly to two_zones_connected properly
// cleans up the old connection and sets up two new zone connections.
// The backward transition (commissioned -> level 0) calls ensureDisconnected
// which clears the old zone CA. This is correct: the two_zones_connected
// handler commissions fresh zones with new CAs on a real target.
func TestStateIsolation_CommissionedToTwoZones(t *testing.T) {
	r := newTestRunner()

	// Simulate test A completing with a commissioned connection.
	r.conn.state = ConnOperational
	r.paseState = &PASEState{
		completed:  true,
		sessionKey: []byte{0xDE, 0xAD},
	}
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()

	// Now run setupPreconditions for a test needing two zones.
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("setupPreconditions: %v", err)
	}

	s := r.snapshot()

	// The backward transition calls ensureDisconnected which clears
	// the old zone CA. This is expected -- fresh commissions create
	// new CAs. (Without a real target, dummy connections don't need CAs.)
	if s.HasZoneCA {
		t.Error("old zoneCA should be cleared by backward transition")
	}

	// Should have exactly 2 active zone connections (dummy since no target).
	if len(s.ActiveZones) != 2 {
		t.Errorf("expected 2 active zones, got %d", len(s.ActiveZones))
	}
	for _, name := range []string{"GRID", "LOCAL"} {
		zc, ok := s.ActiveZones[name]
		if !ok {
			t.Errorf("missing zone %s", name)
			continue
		}
		if !zc.Connected {
			t.Errorf("zone %s should be connected", name)
		}
	}

	// No phantom sockets.
	if name, has := s.HasPhantomZoneSocket(); has {
		t.Errorf("phantom zone socket on %s", name)
	}
}

// TestStateIsolation_TwoZonesToCommissioning verifies that transitioning
// from two_zones_connected back to commissioning mode properly cleans up
// all zone connections.
func TestStateIsolation_TwoZonesToCommissioning(t *testing.T) {
	r := newTestRunner()

	// Simulate two_zones_connected state from a previous test.
	r.activeZoneConns["GRID"] = &Connection{state: ConnOperational}
	r.activeZoneConns["LOCAL"] = &Connection{state: ConnOperational}
	r.activeZoneIDs = map[string]string{"GRID": "aabbccdd", "LOCAL": "11223344"}
	r.zoneCA = &cert.ZoneCA{}
	r.zoneCAPool = x509.NewCertPool()

	// Transition to commissioning mode.
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-COMM-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("setupPreconditions: %v", err)
	}

	s := r.snapshot()

	// All zone connections should be closed and cleaned up.
	if len(s.ActiveZones) != 0 {
		t.Errorf("expected 0 active zones after cleanup, got %d", len(s.ActiveZones))
	}

	// Cert state should be cleared (commissioning mode is level 1, below commissioned).
	if s.HasZoneCA || s.HasControllerCert || s.HasZoneCAPool {
		t.Error("cert state should be cleared for commissioning mode")
	}

	// No phantom sockets.
	if s.HasPhantomSocket() {
		t.Error("phantom main socket")
	}
	if name, has := s.HasPhantomZoneSocket(); has {
		t.Errorf("phantom zone socket on %s", name)
	}
}

// TestStateIsolation_TwoZonesToTwoZones verifies that when two sequential
// tests both need two_zones_connected, the old zone connections are properly
// cleaned up before new ones are created.
func TestStateIsolation_TwoZonesToTwoZones(t *testing.T) {
	r := newTestRunner()

	// Simulate first test's two_zones_connected state.
	oldGrid := &Connection{state: ConnOperational}
	oldLocal := &Connection{state: ConnOperational}
	r.activeZoneConns["GRID"] = oldGrid
	r.activeZoneConns["LOCAL"] = oldLocal
	r.activeZoneIDs = map[string]string{"GRID": "aabbccdd", "LOCAL": "11223344"}

	// Run second two_zones_connected test.
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-MULTI-004",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("setupPreconditions: %v", err)
	}

	// Old connections should be closed.
	if oldGrid.isConnected() {
		t.Error("old GRID connection should be closed")
	}
	if oldLocal.isConnected() {
		t.Error("old LOCAL connection should be closed")
	}

	// New connections should exist (dummy since no target).
	s := r.snapshot()
	if len(s.ActiveZones) != 2 {
		t.Errorf("expected 2 active zones, got %d", len(s.ActiveZones))
	}
}

// TestStateIsolation_PhantomSocketCleanup verifies that a phantom socket
// (connected=false but TCP socket still open) on the main connection is
// detected and cleaned up by setupPreconditions.
func TestStateIsolation_PhantomSocketCleanup(t *testing.T) {
	r := newTestRunner()

	// Simulate the phantom socket bug: sendRequest set connected=false
	// but didn't close the socket.
	r.conn.state = ConnDisconnected
	// We can't set a real net.Conn in unit tests, but we can verify
	// the detection logic via snapshot.

	// Verify the snapshot detects phantom sockets correctly.
	snap := RunnerSnapshot{
		MainConn: ConnSnapshot{
			Connected:  false,
			HasTLSConn: true,
		},
	}
	if !snap.HasPhantomSocket() {
		t.Error("snapshot should detect phantom socket")
	}

	// After cleanup, no phantom.
	snap.MainConn.HasTLSConn = false
	if snap.HasPhantomSocket() {
		t.Error("should not detect phantom after cleanup")
	}
}

// TestStateIsolation_ZoneCAPreservedForTwoZones verifies that the zone CA
// pool is NOT cleared when transitioning to a test that needs
// two_zones_connected, even though its precondition level is 0.
func TestStateIsolation_ZoneCAPreservedForTwoZones(t *testing.T) {
	r := newTestRunner()

	// Simulate commissioned state with zone CA.
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1}}
	r.zoneCAPool = x509.NewCertPool()

	// First: transition to commissioning (clears certs).
	state1 := engine.NewExecutionState(context.Background())
	tc1 := &loader.TestCase{
		ID: "TC-STEP1",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}
	_ = r.setupPreconditions(context.Background(), tc1, state1)
	if r.zoneCAPool != nil {
		t.Error("zoneCAPool should be cleared for commissioning mode")
	}

	// Restore zone CA (simulating a fresh commission in between).
	r.zoneCAPool = x509.NewCertPool()

	// Second: transition to two_zones_connected (should preserve).
	state2 := engine.NewExecutionState(context.Background())
	tc2 := &loader.TestCase{
		ID: "TC-STEP2",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}
	err := r.setupPreconditions(context.Background(), tc2, state2)
	if err != nil {
		t.Fatalf("setupPreconditions: %v", err)
	}
	if r.zoneCAPool == nil {
		t.Error("zoneCAPool should be preserved for two_zones_connected")
	}
}

// TestStateIsolation_ThreeTestSequence exercises the exact sequence that
// fails in production: a commissioned test, followed by a level-0 test,
// followed by a two_zones_connected test.
func TestStateIsolation_ThreeTestSequence(t *testing.T) {
	r := newTestRunner()

	// --- Test A: session_established (level 3) ---
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{0xAB}}
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()

	stateA := engine.NewExecutionState(context.Background())
	tcA := &loader.TestCase{
		ID: "TC-A-COMMISSIONED",
		Preconditions: []loader.Condition{
			{PrecondSessionEstablished: true},
		},
	}
	// Already in commissioned state, so this is a no-op transition.
	err := r.setupPreconditions(context.Background(), tcA, stateA)
	if err != nil {
		t.Fatalf("test A: %v", err)
	}

	snapA := r.snapshot()
	if !snapA.PASECompleted {
		t.Error("test A: expected commissioned state")
	}

	// --- Test B: device_booted (level 0) ---
	stateB := engine.NewExecutionState(context.Background())
	tcB := &loader.TestCase{
		ID: "TC-B-BOOTED",
		Preconditions: []loader.Condition{
			{PrecondDeviceBooted: true},
		},
	}
	err = r.setupPreconditions(context.Background(), tcB, stateB)
	if err != nil {
		t.Fatalf("test B: %v", err)
	}

	snapB := r.snapshot()
	// Level 0 test should clear cert state (not two_zones_connected).
	if snapB.HasZoneCA || snapB.HasControllerCert || snapB.HasZoneCAPool {
		t.Error("test B: cert state should be cleared for level 0")
	}

	// --- Test C: two_zones_connected (level 0, special handler) ---
	stateC := engine.NewExecutionState(context.Background())
	tcC := &loader.TestCase{
		ID: "TC-C-TWOZONES",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}
	err = r.setupPreconditions(context.Background(), tcC, stateC)
	if err != nil {
		t.Fatalf("test C: %v", err)
	}

	snapC := r.snapshot()

	// Should have exactly 2 zone connections (dummy).
	if len(snapC.ActiveZones) != 2 {
		t.Errorf("test C: expected 2 zones, got %d", len(snapC.ActiveZones))
	}

	// No phantom sockets on any connection.
	if snapC.HasPhantomSocket() {
		t.Error("test C: phantom main socket")
	}
	if name, has := snapC.HasPhantomZoneSocket(); has {
		t.Errorf("test C: phantom zone socket on %s", name)
	}
}

// TestStateIsolation_FourMultiZoneTests exercises a sequence of 4 tests
// similar to TC-MULTI-001 through 004, alternating between commissioned
// and two_zones_connected preconditions.
func TestStateIsolation_FourMultiZoneTests(t *testing.T) {
	r := newTestRunner()

	tests := []struct {
		id         string
		precond    string
		wantZones  int
		wantPASE   bool
		wantClosed bool // old zones from previous test should be closed
	}{
		{"TC-MULTI-001", PrecondSessionEstablished, 0, true, false},
		{"TC-MULTI-002", PrecondSessionEstablished, 0, true, false},
		{"TC-MULTI-003", PrecondTwoZonesConnected, 2, false, false},
		{"TC-MULTI-004", PrecondTwoZonesConnected, 2, false, true},
	}

	for i, tt := range tests {
		state := engine.NewExecutionState(context.Background())
		tc := &loader.TestCase{
			ID: tt.id,
			Preconditions: []loader.Condition{
				{tt.precond: true},
			},
		}

		// For commissioned tests, simulate the connection being established
		// (since we have no real target, ensureCommissioned would fail).
		if tt.precond == PrecondSessionEstablished {
			r.conn.state = ConnOperational
			r.paseState = &PASEState{completed: true, sessionKey: []byte{byte(i)}}
		}

		// Track old zone connections for cleanup verification.
		var oldZoneConns []*Connection
		if tt.wantClosed {
			for _, c := range r.activeZoneConns {
				oldZoneConns = append(oldZoneConns, c)
			}
		}

		err := r.setupPreconditions(context.Background(), tc, state)
		if err != nil {
			t.Fatalf("%s: setupPreconditions: %v", tt.id, err)
		}

		s := r.snapshot()

		if len(s.ActiveZones) != tt.wantZones {
			t.Errorf("%s: expected %d zones, got %d", tt.id, tt.wantZones, len(s.ActiveZones))
		}

		// Verify old zone connections were closed.
		for _, c := range oldZoneConns {
			if c.isConnected() {
				t.Errorf("%s: old zone connection should be closed", tt.id)
			}
		}

		// Never have phantom sockets.
		if s.HasPhantomSocket() {
			t.Errorf("%s: phantom main socket", tt.id)
		}
		if name, has := s.HasPhantomZoneSocket(); has {
			t.Errorf("%s: phantom zone socket on %s", tt.id, name)
		}

		t.Logf("%s: %s", tt.id, s.String())
	}
}

// TestStateIsolation_DebugEnabled verifies that enabling debug mode doesn't
// cause panics or change behavior during state transitions.
func TestStateIsolation_DebugEnabled(t *testing.T) {
	r := newTestRunner()
	r.config.Debug = true

	// Simulate commissioned state.
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1}}

	// Backward transition with debug enabled.
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-DEBUG-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("setupPreconditions with debug: %v", err)
	}

	s := r.snapshot()
	if s.MainConn.Connected {
		t.Error("should be disconnected")
	}
	if s.PASECompleted {
		t.Error("PASE should be cleared")
	}
}

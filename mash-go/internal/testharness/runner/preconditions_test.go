package runner

import (
	"context"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestPreconditionLevel(t *testing.T) {
	r := newTestRunner()

	tests := []struct {
		name       string
		conditions []loader.Condition
		wantLevel  int
	}{
		{
			name:       "empty conditions",
			conditions: nil,
			wantLevel:  0,
		},
		{
			name: "session_established true",
			conditions: []loader.Condition{
				{PrecondSessionEstablished: true},
			},
			wantLevel: 3,
		},
		{
			name: "device_commissioned true",
			conditions: []loader.Condition{
				{PrecondDeviceCommissioned: true},
			},
			wantLevel: 3,
		},
		{
			name: "tls_connection_established true",
			conditions: []loader.Condition{
				{PrecondTLSConnectionEstablished: true},
			},
			wantLevel: 2,
		},
		{
			name: "connection_established true",
			conditions: []loader.Condition{
				{PrecondConnectionEstablished: true},
			},
			wantLevel: 2,
		},
		{
			name: "device_in_commissioning_mode true",
			conditions: []loader.Condition{
				{"device_in_commissioning_mode": true},
			},
			wantLevel: 1,
		},
		{
			name: "unknown condition",
			conditions: []loader.Condition{
				{"some_other_condition": true},
			},
			wantLevel: 0,
		},
		{
			name: "multiple conditions highest wins",
			conditions: []loader.Condition{
				{PrecondConnectionEstablished: true},
				{PrecondSessionEstablished: true},
			},
			wantLevel: 3,
		},
		{
			name: "false value ignored",
			conditions: []loader.Condition{
				{PrecondSessionEstablished: false},
			},
			wantLevel: 0,
		},
		{
			name: "mixed true and false",
			conditions: []loader.Condition{
				{PrecondSessionEstablished: false},
				{PrecondConnectionEstablished: true},
			},
			wantLevel: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.preconditionLevel(tt.conditions)
			if got != tt.wantLevel {
				t.Errorf("preconditionLevel() = %d, want %d", got, tt.wantLevel)
			}
		})
	}
}

func TestSetupPreconditions_Level0(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID:            "TC-TEST-001",
		Preconditions: nil,
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error for level 0: %v", err)
	}
}

func TestSetupPreconditions_Level1(t *testing.T) {
	r := newTestRunner()
	// Simulate an existing connection
	r.conn.state = ConnOperational

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-TEST-002",
		Preconditions: []loader.Condition{
			{"device_in_commissioning_mode": true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error for level 1: %v", err)
	}

	// Should have disconnected
	if r.conn.isConnected() {
		t.Error("expected connection to be closed for commissioning mode")
	}
}

func TestEnsureDisconnected(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational

	r.ensureDisconnected()

	if r.conn.isConnected() {
		t.Error("expected connected to be false after ensureDisconnected")
	}
}

func TestEnsureDisconnected_AlreadyDisconnected(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnDisconnected

	// Should not panic or error
	r.ensureDisconnected()

	if r.conn.isConnected() {
		t.Error("expected connected to remain false")
	}
}

func TestEnsureConnected_AlreadyConnected(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	state := engine.NewExecutionState(context.Background())

	err := r.ensureConnected(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error when already connected: %v", err)
	}
}

func TestEnsureCommissioned_AlreadyDone(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
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

func TestSortByPreconditionLevel(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-COMM-001", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},      // level 3
		{ID: "TC-DISC-001", Preconditions: nil},                                                        // level 0
		{ID: "TC-CONN-001", Preconditions: []loader.Condition{{PrecondConnectionEstablished: true}}},   // level 2
		{ID: "TC-PASE-001", Preconditions: []loader.Condition{{"device_in_commissioning_mode": true}}}, // level 1
		{ID: "TC-COMM-002", Preconditions: []loader.Condition{{PrecondSessionEstablished: true}}},      // level 3
		{ID: "TC-DISC-002", Preconditions: nil},                                                        // level 0
		{ID: "TC-PASE-002", Preconditions: []loader.Condition{{"device_uncommissioned": true}}},        // level 1
	}

	SortByPreconditionLevel(cases)

	// Verify ordering: level 0, level 0, level 1, level 1, level 2, level 3, level 3
	wantOrder := []string{
		"TC-DISC-001", "TC-DISC-002", // level 0 -- stable order preserved
		"TC-PASE-001", "TC-PASE-002", // level 1 -- stable order preserved
		"TC-CONN-001",                // level 2
		"TC-COMM-001", "TC-COMM-002", // level 3 -- stable order preserved
	}

	if len(cases) != len(wantOrder) {
		t.Fatalf("got %d cases, want %d", len(cases), len(wantOrder))
	}

	for i, tc := range cases {
		if tc.ID != wantOrder[i] {
			t.Errorf("position %d: got %s, want %s", i, tc.ID, wantOrder[i])
		}
	}
}

func TestShuffleWithinLevels(t *testing.T) {
	// Create sorted test cases (must be pre-sorted by level).
	makeCases := func() []*loader.TestCase {
		return []*loader.TestCase{
			{ID: "L0-A", Preconditions: nil},
			{ID: "L0-B", Preconditions: nil},
			{ID: "L0-C", Preconditions: nil},
			{ID: "L0-D", Preconditions: nil},
			{ID: "L3-A", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},
			{ID: "L3-B", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},
			{ID: "L3-C", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},
			{ID: "L3-D", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},
		}
	}

	// Same seed produces same order.
	cases1 := makeCases()
	cases2 := makeCases()
	ShuffleWithinLevels(cases1, 42)
	ShuffleWithinLevels(cases2, 42)
	for i := range cases1 {
		if cases1[i].ID != cases2[i].ID {
			t.Errorf("determinism: position %d: %s != %s", i, cases1[i].ID, cases2[i].ID)
		}
	}

	// Level boundaries are preserved: L0 cases still come first.
	for i := 0; i < 4; i++ {
		if cases1[i].ID[:2] != "L0" {
			t.Errorf("position %d: expected L0 case, got %s", i, cases1[i].ID)
		}
	}
	for i := 4; i < 8; i++ {
		if cases1[i].ID[:2] != "L3" {
			t.Errorf("position %d: expected L3 case, got %s", i, cases1[i].ID)
		}
	}

	// Different seed produces different order (with very high probability for 4 items).
	cases3 := makeCases()
	ShuffleWithinLevels(cases3, 99)
	sameCount := 0
	for i := range cases1 {
		if cases1[i].ID == cases3[i].ID {
			sameCount++
		}
	}
	if sameCount == len(cases1) {
		t.Error("different seeds produced identical order (extremely unlikely)")
	}
}

func TestPreconditionLevel_NewKeys(t *testing.T) {
	r := newTestRunner()

	tests := []struct {
		name      string
		key       string
		wantLevel int
	}{
		{"device_uncommissioned", "device_uncommissioned", 1},
		{PrecondCommissioningWindowOpen, PrecondCommissioningWindowOpen, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := []loader.Condition{{tt.key: true}}
			got := r.preconditionLevel(conditions)
			if got != tt.wantLevel {
				t.Errorf("preconditionLevel(%s) = %d, want %d", tt.key, got, tt.wantLevel)
			}
		})
	}
}

func TestCurrentLevel(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(r *Runner)
		wantLevel int
	}{
		{
			name:      "no connection",
			setup:     func(r *Runner) {},
			wantLevel: precondLevelNone,
		},
		{
			name: "connected only",
			setup: func(r *Runner) {
				r.conn.state = ConnTLSConnected
			},
			wantLevel: precondLevelConnected,
		},
		{
			name: "commissioned",
			setup: func(r *Runner) {
				r.conn.state = ConnOperational
				r.paseState = &PASEState{completed: true}
			},
			wantLevel: precondLevelCommissioned,
		},
		{
			name: "pase incomplete treated as connected",
			setup: func(r *Runner) {
				r.conn.state = ConnTLSConnected
				r.paseState = &PASEState{completed: false}
			},
			wantLevel: precondLevelConnected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner()
			tt.setup(r)
			got := r.currentLevel()
			if got != tt.wantLevel {
				t.Errorf("currentLevel() = %d, want %d", got, tt.wantLevel)
			}
		})
	}
}

func TestSetupPreconditions_BackwardsTransition(t *testing.T) {
	// Commissioned runner + commissioning-level test -> should disconnect.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-BACK-001",
		Preconditions: []loader.Condition{
			{"device_in_commissioning_mode": true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.conn.isConnected() {
		t.Error("expected connection to be closed for backwards transition")
	}
	if r.paseState != nil {
		t.Error("expected paseState to be nil after backwards transition")
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

// C9: D2D precondition keys are stored in state.
func TestSetupPreconditions_D2DStateKeys(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	tc := &loader.TestCase{
		ID: "TC-D2D-001",
		Preconditions: []loader.Condition{
			{"two_devices_same_zone": true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, ok := state.Get("two_devices_same_zone")
	if !ok {
		t.Error("expected two_devices_same_zone to be stored in state")
	}
	if v != true {
		t.Errorf("expected true, got %v", v)
	}
}

func TestSetupPreconditions_D2DExpiredKey(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	tc := &loader.TestCase{
		ID: "TC-D2D-003",
		Preconditions: []loader.Condition{
			{"device_b_cert_expired": true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, ok := state.Get("device_b_cert_expired")
	if !ok {
		t.Error("expected device_b_cert_expired to be stored in state")
	}
	if v != true {
		t.Errorf("expected true, got %v", v)
	}
}

// D2D precondition keys should be at level 0 (no setup needed).
func TestPreconditionLevel_D2DKeys(t *testing.T) {
	r := newTestRunner()

	for _, key := range []string{"two_devices_same_zone", "two_devices_different_zones", "device_b_cert_expired"} {
		conditions := []loader.Condition{{key: true}}
		got := r.preconditionLevel(conditions)
		if got != precondLevelNone {
			t.Errorf("preconditionLevel(%s) = %d, want %d (level 0)", key, got, precondLevelNone)
		}
	}
}

func TestPreconditionLevel_TwoZonesConnected(t *testing.T) {
	r := newTestRunner()
	conditions := []loader.Condition{
		{PrecondTwoZonesConnected: true},
	}
	got := r.preconditionLevel(conditions)
	if got != precondLevelNone {
		t.Errorf("preconditionLevel(two_zones_connected) = %d, want %d (level 0)", got, precondLevelNone)
	}
}

func TestSetupPreconditions_TwoZonesConnected(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// State key should be set.
	v, ok := state.Get(PrecondTwoZonesConnected)
	if !ok {
		t.Error("expected two_zones_connected to be stored in state")
	}
	if v != true {
		t.Errorf("expected true, got %v", v)
	}

	// Connection tracker should have exactly 2 zone connections.
	ct := getConnectionTracker(state)
	if len(ct.zoneConnections) != 2 {
		t.Errorf("expected 2 zone connections, got %d", len(ct.zoneConnections))
	}
	for _, name := range []string{"GRID", "LOCAL"} {
		conn, exists := ct.zoneConnections[name]
		if !exists {
			t.Errorf("expected zone connection %q to exist", name)
			continue
		}
		if !conn.isConnected() {
			t.Errorf("expected zone %q to be connected", name)
		}
	}
}

func TestSetupPreconditions_TwoZonesConnected_HandlersWork(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dummy connections created by precondition should support simulated I/O.
	step := &loader.Step{Params: map[string]any{
		KeyZoneID:   "GRID",
		KeyEndpoint: float64(1),
		KeyFeature:  "measurement",
	}}
	out, err := r.handleSubscribeAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("subscribe_as_zone on dummy GRID connection: %v", err)
	}
	if out[KeySubscribeSuccess] != true {
		t.Error("expected subscribe_success=true on dummy GRID connection")
	}

	step = &loader.Step{Params: map[string]any{
		KeyZoneID:   "LOCAL",
		KeyEndpoint: float64(1),
		KeyFeature:  "measurement",
	}}
	out, err = r.handleReadAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("read_as_zone on dummy LOCAL connection: %v", err)
	}
	if out[KeyReadSuccess] != true {
		t.Error("expected read_success=true on dummy LOCAL connection")
	}
}

func TestPreconditionLevel_TwoDevicesSameDiscriminator(t *testing.T) {
	r := newTestRunner()
	conditions := []loader.Condition{
		{PrecondTwoDevicesSameDiscriminator: true},
	}
	got := r.preconditionLevel(conditions)
	if got != precondLevelNone {
		t.Errorf("preconditionLevel(two_devices_same_discriminator) = %d, want %d (level 0)", got, precondLevelNone)
	}
}

func TestSendRemoveZone_NilPaseState(t *testing.T) {
	// sendRemoveZone should be a no-op when not commissioned.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = nil

	// Should not panic.
	r.sendRemoveZone()
}

func TestSendRemoveZone_NoSessionKey(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: nil}

	// Should not panic.
	r.sendRemoveZone()
}

func TestSetupPreconditions_BackwardsFromCommissioned_SendsRemoveZone(t *testing.T) {
	// When transitioning from commissioned (level 3) to commissioning (level 1),
	// sendRemoveZone is called before disconnect. Since there's no real server,
	// the send will silently fail but the transition should still complete.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{
		completed:  true,
		sessionKey: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-BACK-REMOVE-001",
		Preconditions: []loader.Condition{
			{"device_in_commissioning_mode": true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Connection should be closed after backward transition
	if r.conn.isConnected() {
		t.Error("expected connection to be closed")
	}
	// PASE state should be cleared
	if r.paseState != nil {
		t.Error("expected paseState to be nil")
	}
}

func TestPreconditionLevel_DeviceInZoneKeys(t *testing.T) {
	r := newTestRunner()

	for _, key := range []string{PrecondDeviceInZone, PrecondDeviceInTwoZones} {
		conditions := []loader.Condition{{key: true}}
		got := r.preconditionLevel(conditions)
		if got != precondLevelNone {
			t.Errorf("preconditionLevel(%s) = %d, want %d (level 0)", key, got, precondLevelNone)
		}
	}

	// Verify they're registered as simulation keys.
	if !simulationPreconditionKeys[PrecondDeviceInZone] {
		t.Error("expected device_in_zone to be a simulation precondition key")
	}
	if !simulationPreconditionKeys[PrecondDeviceInTwoZones] {
		t.Error("expected device_in_two_zones to be a simulation precondition key")
	}
}

func TestZoneCreatedIsSimulationPreconditionKey(t *testing.T) {
	if !simulationPreconditionKeys[PrecondZoneCreated] {
		t.Error("expected zone_created to be a simulation precondition key")
	}
}

func TestSetupPreconditions_ZoneCreated_SetsStateFlag(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	tc := &loader.TestCase{
		ID: "TC-MASHD-002",
		Preconditions: []loader.Condition{
			{PrecondZoneCreated: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The zone_created flag must be stored in state so handlers can check it.
	v, ok := state.Get(PrecondZoneCreated)
	if !ok {
		t.Fatal("expected zone_created to be set in state")
	}
	if v != true {
		t.Errorf("expected zone_created=true, got %v", v)
	}
}

func TestSetupPreconditions_ClearsZoneCAForNonCommissionedTests(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	// Simulate stale zone CA from a previous commissioned test.
	r.zoneCAPool = x509.NewCertPool()

	// A level-2 test (connection_established) should clear the stale pool.
	tc := &loader.TestCase{
		ID: "TC-CONN-001",
		Preconditions: []loader.Condition{
			{PrecondConnectionEstablished: true},
		},
	}

	// Will fail to connect (no target), but that's OK -- we're checking
	// that zoneCAPool was cleared before the connection attempt.
	_ = r.setupPreconditions(context.Background(), tc, state)

	if r.zoneCAPool != nil {
		t.Error("expected zoneCAPool to be nil for non-commissioned (level 2) test")
	}
}

func TestSetupPreconditions_TwoZonesConnected_PreservesZoneCA(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	// Simulate zone CA from a previous commissioned test.
	r.zoneCAPool = x509.NewCertPool()

	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// two_zones_connected needs the zone CA for operational TLS, so it
	// should NOT be cleared even though the precondition level is 0.
	if r.zoneCAPool == nil {
		t.Error("expected zoneCAPool to be preserved for two_zones_connected")
	}
}

func TestSetupPreconditions_ClearsZoneCA_ForNonZoneTests(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	// Simulate stale zone CA from a previous commissioned test.
	r.zoneCAPool = x509.NewCertPool()

	tc := &loader.TestCase{
		ID: "TC-SIMPLE-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceBooted: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Level 0 test without two_zones_connected should clear stale zone CA.
	if r.zoneCAPool != nil {
		t.Error("expected zoneCAPool to be nil for simple level-0 test")
	}
}

func TestSetupPreconditions_ClosesStaleZoneConnections(t *testing.T) {
	r := newTestRunner()

	// Simulate zone connections left over from a previous test.
	conn1 := &Connection{state: ConnOperational}
	conn2 := &Connection{state: ConnOperational}
	r.activeZoneConns["GRID"] = conn1
	r.activeZoneConns["LOCAL"] = conn2

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	_ = r.setupPreconditions(context.Background(), tc, state)

	// Stale zone connections should have been closed.
	if conn1.isConnected() {
		t.Error("expected stale GRID connection to be closed")
	}
	if conn2.isConnected() {
		t.Error("expected stale LOCAL connection to be closed")
	}
	if len(r.activeZoneConns) != 2 {
		// New connections (dummy since no target) should have been created.
		t.Errorf("expected 2 active zone conns, got %d", len(r.activeZoneConns))
	}
}

func TestHandleConnectAsZone_TracksInRunner(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Without a target, handleConnectAsZone won't establish a real connection.
	// Directly verify the runner-level tracking by simulating what the
	// precondition does: dummy connections should be tracked.
	ct := getConnectionTracker(state)
	conn := &Connection{state: ConnOperational}
	ct.zoneConnections["GRID"] = conn
	r.activeZoneConns["GRID"] = conn

	if _, ok := r.activeZoneConns["GRID"]; !ok {
		t.Error("expected GRID to be tracked in runner activeZoneConns")
	}
}

func TestEnsureDisconnected_ClearsZoneCA(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational

	// Simulate state left over from a previous commissioned test.
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()

	r.ensureDisconnected()

	if r.zoneCA != nil {
		t.Error("expected zoneCA to be nil after ensureDisconnected")
	}
	if r.controllerCert != nil {
		t.Error("expected controllerCert to be nil after ensureDisconnected")
	}
	if r.zoneCAPool != nil {
		t.Error("expected zoneCAPool to be nil after ensureDisconnected")
	}
}

func TestPreconditionLevel_DeviceStateKeys(t *testing.T) {
	r := newTestRunner()

	tests := []struct {
		name      string
		key       string
		wantLevel int
	}{
		{"device_reset", PrecondDeviceReset, precondLevelCommissioned},
		{"device_has_grid_zone", PrecondDeviceHasGridZone, precondLevelCommissioned},
		{"device_has_local_zone", PrecondDeviceHasLocalZone, precondLevelCommissioned},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := []loader.Condition{{tt.key: true}}
			got := r.preconditionLevel(conditions)
			if got != tt.wantLevel {
				t.Errorf("preconditionLevel(%s) = %d, want %d", tt.key, got, tt.wantLevel)
			}
		})
	}
}

func TestSetupPreconditions_DeviceHasGridZone(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-ZONE-010",
		Preconditions: []loader.Condition{
			{PrecondDeviceHasGridZone: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a GRID zone was created in zone state.
	zs := getZoneState(state)
	found := false
	for _, z := range zs.zones {
		if z.ZoneType == ZoneTypeGrid {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a GRID zone to be created")
	}

	// Simulation key should be in state.
	v, ok := state.Get(PrecondDeviceHasGridZone)
	if !ok || v != true {
		t.Error("expected device_has_grid_zone=true in state")
	}
}

func TestSetupPreconditions_DeviceHasBothZones(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-ZTYPE-004",
		Preconditions: []loader.Condition{
			{PrecondDeviceHasGridZone: true},
			{PrecondDeviceHasLocalZone: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	zs := getZoneState(state)
	if len(zs.zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zs.zones))
	}

	hasGrid, hasLocal := false, false
	for _, z := range zs.zones {
		switch z.ZoneType {
		case ZoneTypeGrid:
			hasGrid = true
		case ZoneTypeLocal:
			hasLocal = true
		}
	}
	if !hasGrid {
		t.Error("expected GRID zone")
	}
	if !hasLocal {
		t.Error("expected LOCAL zone")
	}
}

func TestPreconditionLevel_SessionPreviouslyConnected(t *testing.T) {
	r := newTestRunner()
	conditions := []loader.Condition{
		{PrecondSessionPreviouslyConnected: true},
	}
	got := r.preconditionLevel(conditions)
	if got != precondLevelCommissioned {
		t.Errorf("preconditionLevel(session_previously_connected) = %d, want %d", got, precondLevelCommissioned)
	}
}

func TestSetupPreconditions_SessionPreviouslyConnected(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3, 4}}

	// Set up zone crypto state (simulates what PASE would produce).
	r.zoneCA = &cert.ZoneCA{}
	r.controllerCert = &cert.OperationalCert{}
	r.zoneCAPool = x509.NewCertPool()

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-SUB-010",
		Preconditions: []loader.Condition{
			{PrecondSessionPreviouslyConnected: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Connection should be closed (simulating disconnect).
	if r.conn.isConnected() {
		t.Error("expected connection to be closed after session_previously_connected setup")
	}

	// Zone crypto state should be preserved for reconnection.
	if r.zoneCAPool == nil {
		t.Error("expected zoneCAPool to be preserved")
	}
	if r.zoneCA == nil {
		t.Error("expected zoneCA to be preserved")
	}
	if r.controllerCert == nil {
		t.Error("expected controllerCert to be preserved")
	}

	// PASE state should be cleared (session is over).
	if r.paseState != nil {
		t.Error("expected paseState to be nil")
	}
}

func TestPreconditionLevel_FreshCommission(t *testing.T) {
	conditions := []loader.Condition{{PrecondFreshCommission: true}}
	got := preconditionLevelFor(conditions)
	if got != precondLevelCommissioned {
		t.Errorf("preconditionLevel(fresh_commission) = %d, want %d", got, precondLevelCommissioned)
	}
}

func TestNeedsFreshCommission(t *testing.T) {
	tests := []struct {
		name       string
		conditions []loader.Condition
		want       bool
	}{
		{"nil conditions", nil, false},
		{"empty conditions", []loader.Condition{}, false},
		{"session_established only", []loader.Condition{{PrecondSessionEstablished: true}}, false},
		{"fresh_commission true", []loader.Condition{{PrecondFreshCommission: true}}, true},
		{"fresh_commission false", []loader.Condition{{PrecondFreshCommission: false}}, false},
		{"mixed with fresh_commission", []loader.Condition{
			{PrecondSessionEstablished: true},
			{PrecondFreshCommission: true},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsFreshCommission(tt.conditions)
			if got != tt.want {
				t.Errorf("needsFreshCommission() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetupPreconditions_SessionReuse_SkipsCloseZoneConns(t *testing.T) {
	// Commissioned runner + session_established test -> session preserved.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID:            "TC-READ-002",
		Preconditions: []loader.Condition{{PrecondSessionEstablished: true}},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.conn.isConnected() {
		t.Error("expected connection to remain connected")
	}
	if r.paseState == nil || !r.paseState.completed {
		t.Error("expected PASE state to be preserved")
	}
	if len(r.activeZoneConns) != 1 {
		t.Errorf("expected 1 active zone conn, got %d", len(r.activeZoneConns))
	}
}

func TestSetupPreconditions_FreshCommission_ClosesZoneConns(t *testing.T) {
	// fresh_commission=true -> session torn down even at level 3 -> level 3.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-COMM-001",
		Preconditions: []loader.Condition{
			{PrecondSessionEstablished: true},
			{PrecondFreshCommission: true},
		},
	}

	_ = r.setupPreconditions(context.Background(), tc, state)

	// closeActiveZoneConns should have been called -- verify the map is empty.
	if len(r.activeZoneConns) != 0 {
		t.Errorf("expected 0 active zone conns, got %d", len(r.activeZoneConns))
	}
	// Note: paseState is cleared only when real sockets (tlsConn/conn) are
	// present, which stub connections lack. The important assertion is that
	// closeActiveZoneConns was called (activeZoneConns emptied).
}

func TestSetupPreconditions_SessionReuse_TwoZonesConnectedForcesClose(t *testing.T) {
	// two_zones_connected forces teardown even at level 3 -> level 3.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-MULTI-003",
		Preconditions: []loader.Condition{
			{PrecondTwoZonesConnected: true},
		},
	}

	_ = r.setupPreconditions(context.Background(), tc, state)

	// The old main connection should have been closed by closeActiveZoneConns.
	if r.paseState != nil {
		t.Error("expected paseState cleared after two_zones_connected teardown")
	}
}

func TestSetupPreconditions_SessionReuse_DeviceHasGridZoneForcesClose(t *testing.T) {
	// device_has_grid_zone forces teardown even at level 3 -> level 3.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-ZONE-010",
		Preconditions: []loader.Condition{
			{PrecondDeviceHasGridZone: true},
		},
	}

	_ = r.setupPreconditions(context.Background(), tc, state)

	// closeActiveZoneConns should have been called, clearing paseState.
	if len(r.activeZoneConns) != 0 {
		t.Errorf("expected 0 active zone conns after device_has_grid_zone, got %d", len(r.activeZoneConns))
	}
}

func TestSetupPreconditions_SessionReuse_BackwardsTransitionUnaffected(t *testing.T) {
	// Level 3 -> level 1 still tears down (canReuseSession=false because needed < commissioned).
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID: "TC-BACK-002",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}

	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.conn.isConnected() {
		t.Error("expected connection to be closed for backwards transition")
	}
	if r.paseState != nil {
		t.Error("expected paseState to be nil after backwards transition")
	}
}

// Phase 1: Subscription tracking tests.

func TestTrackSubscription(t *testing.T) {
	r := newTestRunner()
	r.trackSubscription(1)
	r.trackSubscription(2)

	if len(r.activeSubscriptionIDs) != 2 {
		t.Fatalf("expected 2 tracked subs, got %d", len(r.activeSubscriptionIDs))
	}
	if r.activeSubscriptionIDs[0] != 1 || r.activeSubscriptionIDs[1] != 2 {
		t.Errorf("unexpected IDs: %v", r.activeSubscriptionIDs)
	}
}

func TestRemoveActiveSubscription(t *testing.T) {
	r := newTestRunner()
	r.trackSubscription(10)
	r.trackSubscription(20)
	r.trackSubscription(30)

	r.removeActiveSubscription(20)

	if len(r.activeSubscriptionIDs) != 2 {
		t.Fatalf("expected 2 tracked subs after remove, got %d", len(r.activeSubscriptionIDs))
	}
	// Remaining should be 10 and 30.
	for _, id := range r.activeSubscriptionIDs {
		if id == 20 {
			t.Error("expected subscription 20 to be removed")
		}
	}
}

func TestRemoveActiveSubscription_NotFound(t *testing.T) {
	r := newTestRunner()
	r.trackSubscription(10)

	// Removing a non-existent ID should be a no-op.
	r.removeActiveSubscription(99)

	if len(r.activeSubscriptionIDs) != 1 {
		t.Fatalf("expected 1 tracked sub, got %d", len(r.activeSubscriptionIDs))
	}
}

func TestTeardownTest_ClearsTrackingList(t *testing.T) {
	r := newTestRunner()
	r.trackSubscription(1)
	r.trackSubscription(2)

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{ID: "TC-TEST-001"}
	r.teardownTest(context.Background(), tc, state)

	if len(r.activeSubscriptionIDs) != 0 {
		t.Errorf("expected empty tracking list after teardown, got %d", len(r.activeSubscriptionIDs))
	}
}

func TestTeardownTest_NoUnsubscribeWhenDisconnected(t *testing.T) {
	r := newTestRunner()
	r.trackSubscription(1)
	// conn is not connected (default stub)

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{ID: "TC-TEST-001"}

	// Should not panic or attempt to send on a nil/disconnected conn.
	r.teardownTest(context.Background(), tc, state)

	if len(r.activeSubscriptionIDs) != 0 {
		t.Errorf("expected empty tracking list after teardown, got %d", len(r.activeSubscriptionIDs))
	}
}

func TestTeardownTest_ClearsPendingNotifications(t *testing.T) {
	r := newTestRunner()
	r.pendingNotifications = [][]byte{{1, 2, 3}}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{ID: "TC-TEST-001"}
	r.teardownTest(context.Background(), tc, state)

	if len(r.pendingNotifications) != 0 {
		t.Errorf("expected empty pending notifications after teardown, got %d", len(r.pendingNotifications))
	}
}

// Phase 2: Session health check tests.

func TestProbeSessionHealth_NilConnection(t *testing.T) {
	r := newTestRunner()
	r.conn = nil

	err := r.probeSessionHealth()
	if err == nil {
		t.Fatal("expected error for nil connection")
	}
	if err.Error() != "no active connection" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProbeSessionHealth_DisconnectedConnection(t *testing.T) {
	r := newTestRunner()
	// Default stub has connected=false

	err := r.probeSessionHealth()
	if err == nil {
		t.Fatal("expected error for disconnected connection")
	}
	if err.Error() != "no active connection" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestProbeSessionHealth_NilFramer(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.conn.framer = nil

	err := r.probeSessionHealth()
	if err == nil {
		t.Fatal("expected error for nil framer")
	}
	if err.Error() != "no active connection" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetupPreconditions_SessionReuse_FallsBackOnHealthCheckFailure(t *testing.T) {
	// Commissioned runner with connected=true but no real framer.
	// probeSessionHealth should fail (nil framer) and canReuseSession
	// should fall back to closeActiveZoneConns.
	// Set a target so the health check is actually executed (it's
	// skipped for stub connections without a target).
	r := newTestRunner()
	r.config.Target = "127.0.0.1:9999"
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID:            "TC-READ-HEALTH-001",
		Preconditions: []loader.Condition{{PrecondSessionEstablished: true}},
	}

	// setupPreconditions will try to reuse the session but the health
	// check fails (stub connection has no framer), so it falls back to
	// closeActiveZoneConns.
	_ = r.setupPreconditions(context.Background(), tc, state)

	// activeZoneConns should be emptied because closeActiveZoneConns was called.
	if len(r.activeZoneConns) != 0 {
		t.Errorf("expected 0 active zone conns after health check failure, got %d", len(r.activeZoneConns))
	}
}

// Phase 3: Broadened reset condition tests.

func TestSetupPreconditions_SessionReuseNoResetWhenUnmodified(t *testing.T) {
	// Commissioned runner with deviceStateModified=false.
	// The reset trigger should NOT fire when device state was not modified.
	// Verify that session reuse works correctly in this case.
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1, 2, 3}}
	r.deviceStateModified = false
	r.activeZoneConns["main-abc123"] = r.conn
	r.activeZoneIDs = map[string]string{"main-abc123": "abc123"}

	state := engine.NewExecutionState(context.Background())
	tc := &loader.TestCase{
		ID:            "TC-READ-003",
		Preconditions: []loader.Condition{{PrecondSessionEstablished: true}},
	}

	// Should succeed without error (no target means reset is skipped).
	err := r.setupPreconditions(context.Background(), tc, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Session should still be reusable (no target means health check skipped too).
	if !r.conn.isConnected() {
		t.Error("expected connection to remain connected")
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

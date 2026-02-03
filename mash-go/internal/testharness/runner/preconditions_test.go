package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
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
	r.conn.connected = true

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
	if r.conn.connected {
		t.Error("expected connection to be closed for commissioning mode")
	}
}

func TestEnsureDisconnected(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true

	r.ensureDisconnected()

	if r.conn.connected {
		t.Error("expected connected to be false after ensureDisconnected")
	}
}

func TestEnsureDisconnected_AlreadyDisconnected(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = false

	// Should not panic or error
	r.ensureDisconnected()

	if r.conn.connected {
		t.Error("expected connected to remain false")
	}
}

func TestEnsureConnected_AlreadyConnected(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := engine.NewExecutionState(context.Background())

	err := r.ensureConnected(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error when already connected: %v", err)
	}
}

func TestEnsureCommissioned_AlreadyDone(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
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
		{ID: "TC-COMM-001", Preconditions: []loader.Condition{{PrecondDeviceCommissioned: true}}},          // level 3
		{ID: "TC-DISC-001", Preconditions: nil},                                                        // level 0
		{ID: "TC-CONN-001", Preconditions: []loader.Condition{{PrecondConnectionEstablished: true}}},       // level 2
		{ID: "TC-PASE-001", Preconditions: []loader.Condition{{"device_in_commissioning_mode": true}}}, // level 1
		{ID: "TC-COMM-002", Preconditions: []loader.Condition{{PrecondSessionEstablished: true}}},          // level 3
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
				r.conn.connected = true
			},
			wantLevel: precondLevelConnected,
		},
		{
			name: "commissioned",
			setup: func(r *Runner) {
				r.conn.connected = true
				r.paseState = &PASEState{completed: true}
			},
			wantLevel: precondLevelCommissioned,
		},
		{
			name: "pase incomplete treated as connected",
			setup: func(r *Runner) {
				r.conn.connected = true
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
	r.conn.connected = true
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

	if r.conn.connected {
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

func TestSendRemoveZone_NilPaseState(t *testing.T) {
	// sendRemoveZone should be a no-op when not commissioned.
	r := newTestRunner()
	r.conn.connected = true
	r.paseState = nil

	// Should not panic.
	r.sendRemoveZone()
}

func TestSendRemoveZone_NoSessionKey(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	r.paseState = &PASEState{completed: true, sessionKey: nil}

	// Should not panic.
	r.sendRemoveZone()
}

func TestSetupPreconditions_BackwardsFromCommissioned_SendsRemoveZone(t *testing.T) {
	// When transitioning from commissioned (level 3) to commissioning (level 1),
	// sendRemoveZone is called before disconnect. Since there's no real server,
	// the send will silently fail but the transition should still complete.
	r := newTestRunner()
	r.conn.connected = true
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
	if r.conn.connected {
		t.Error("expected connection to be closed")
	}
	// PASE state should be cleared
	if r.paseState != nil {
		t.Error("expected paseState to be nil")
	}
}

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
				{"session_established": true},
			},
			wantLevel: 3,
		},
		{
			name: "device_commissioned true",
			conditions: []loader.Condition{
				{"device_commissioned": true},
			},
			wantLevel: 3,
		},
		{
			name: "tls_connection_established true",
			conditions: []loader.Condition{
				{"tls_connection_established": true},
			},
			wantLevel: 2,
		},
		{
			name: "connection_established true",
			conditions: []loader.Condition{
				{"connection_established": true},
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
				{"connection_established": true},
				{"session_established": true},
			},
			wantLevel: 3,
		},
		{
			name: "false value ignored",
			conditions: []loader.Condition{
				{"session_established": false},
			},
			wantLevel: 0,
		},
		{
			name: "mixed true and false",
			conditions: []loader.Condition{
				{"session_established": false},
				{"connection_established": true},
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
	v, ok := state.Get("session_established")
	if !ok {
		t.Fatal("expected session_established in state")
	}
	if v != true {
		t.Errorf("expected session_established=true, got %v", v)
	}

	v, ok = state.Get("connection_established")
	if !ok {
		t.Fatal("expected connection_established in state")
	}
	if v != true {
		t.Errorf("expected connection_established=true, got %v", v)
	}
}

func TestSortByPreconditionLevel(t *testing.T) {
	cases := []*loader.TestCase{
		{ID: "TC-COMM-001", Preconditions: []loader.Condition{{"device_commissioned": true}}},          // level 3
		{ID: "TC-DISC-001", Preconditions: nil},                                                         // level 0
		{ID: "TC-CONN-001", Preconditions: []loader.Condition{{"connection_established": true}}},         // level 2
		{ID: "TC-PASE-001", Preconditions: []loader.Condition{{"device_in_commissioning_mode": true}}},   // level 1
		{ID: "TC-COMM-002", Preconditions: []loader.Condition{{"session_established": true}}},            // level 3
		{ID: "TC-DISC-002", Preconditions: nil},                                                         // level 0
		{ID: "TC-PASE-002", Preconditions: []loader.Condition{{"device_uncommissioned": true}}},          // level 1
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
		{"commissioning_window_open", "commissioning_window_open", 1},
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

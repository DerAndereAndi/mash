package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestZoneCreateHasListCount(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Create two zones.
	step := &loader.Step{Params: map[string]any{"zone_type": "GRID_OPERATOR", "zone_id": "grid-01"}}
	out, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["zone_created"] != true {
		t.Error("expected zone_created=true")
	}

	step = &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "home-01"}}
	_, err = r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error creating second zone: %v", err)
	}

	// has_zone.
	step = &loader.Step{Params: map[string]any{"zone_id": "grid-01"}}
	out, err = r.handleHasZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["zone_exists"] != true {
		t.Error("expected zone_exists=true")
	}

	// list_zones.
	out, err = r.handleListZones(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["zone_count"] != 2 {
		t.Errorf("expected zone_count=2, got %v", out["zone_count"])
	}

	// zone_count.
	out, err = r.handleZoneCount(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["count"] != 2 {
		t.Errorf("expected count=2, got %v", out["count"])
	}
}

func TestZoneDuplicateTypeError(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "z1"}}
	_, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	step = &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "z2"}}
	_, err = r.handleCreateZone(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for duplicate zone type")
	}
}

func TestZoneDeleteRemove(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_type": "USER_APP", "zone_id": "u1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// Delete it.
	step = &loader.Step{Params: map[string]any{"zone_id": "u1"}}
	out, err := r.handleDeleteZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["zone_removed"] != true {
		t.Error("expected zone_removed=true")
	}

	// Verify gone.
	step = &loader.Step{Params: map[string]any{"zone_id": "u1"}}
	out, _ = r.handleHasZone(context.Background(), step, state)
	if out["zone_exists"] != false {
		t.Error("expected zone_exists=false after delete")
	}

	// Delete non-existent -> zone_removed false.
	out, _ = r.handleDeleteZone(context.Background(), step, state)
	if out["zone_removed"] != false {
		t.Error("expected zone_removed=false for non-existent zone")
	}
}

func TestHighestPriorityZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Create zones with different priorities.
	step := &loader.Step{Params: map[string]any{"zone_type": "USER_APP", "zone_id": "user"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{"zone_type": "GRID_OPERATOR", "zone_id": "grid"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "home"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// Highest priority should be GRID_OPERATOR.
	out, err := r.handleHighestPriorityZone(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["zone_type"] != "GRID_OPERATOR" {
		t.Errorf("expected GRID_OPERATOR, got %v", out["zone_type"])
	}
	if out["zone_id"] != "grid" {
		t.Errorf("expected zone_id=grid, got %v", out["zone_id"])
	}
}

func TestHighestPriorityConnectedZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_type": "GRID_OPERATOR", "zone_id": "grid"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "home"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// None connected -> empty.
	out, _ := r.handleHighestPriorityConnectedZone(context.Background(), &loader.Step{}, state)
	if out["zone_id"] != "" {
		t.Error("expected empty zone_id when none connected")
	}

	// Connect home.
	zs := getZoneState(state)
	zs.zones["home"].Connected = true

	out, _ = r.handleHighestPriorityConnectedZone(context.Background(), &loader.Step{}, state)
	if out["zone_id"] != "home" {
		t.Errorf("expected home, got %v", out["zone_id"])
	}
}

func TestAddZoneAndVerifyBinding(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "h1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{"zone_id": "h1", "device_id": "dev-001"}}
	out, err := r.handleAddZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["device_added"] != true {
		t.Error("expected device_added=true")
	}

	// Verify binding.
	step = &loader.Step{Params: map[string]any{"zone_id": "h1", "device_id": "dev-001"}}
	out, _ = r.handleVerifyZoneBinding(context.Background(), step, state)
	if out["binding_valid"] != true {
		t.Error("expected binding_valid=true")
	}

	// Non-existent device.
	step = &loader.Step{Params: map[string]any{"zone_id": "h1", "device_id": "dev-999"}}
	out, _ = r.handleVerifyZoneBinding(context.Background(), step, state)
	if out["binding_valid"] != false {
		t.Error("expected binding_valid=false for unknown device")
	}
}

func TestVerifyZoneIDDerivation(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Valid: 16 hex chars.
	step := &loader.Step{Params: map[string]any{"zone_id": "a1b2c3d4e5f6a7b8"}}
	out, _ := r.handleVerifyZoneIDDerivation(context.Background(), step, state)
	if out["derivation_valid"] != true {
		t.Error("expected derivation_valid=true for valid hex ID")
	}

	// Invalid: too short.
	step = &loader.Step{Params: map[string]any{"zone_id": "abc123"}}
	out, _ = r.handleVerifyZoneIDDerivation(context.Background(), step, state)
	if out["derivation_valid"] != false {
		t.Error("expected derivation_valid=false for short ID")
	}
}

func TestDisconnectZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_type": "HOME_MANAGER", "zone_id": "h1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)
	zs := getZoneState(state)
	zs.zones["h1"].Connected = true

	step = &loader.Step{Params: map[string]any{"zone_id": "h1"}}
	out, _ := r.handleDisconnectZone(context.Background(), step, state)
	if out["zone_disconnected"] != true {
		t.Error("expected zone_disconnected=true")
	}

	if zs.zones["h1"].Connected {
		t.Error("expected zone to be disconnected")
	}
}

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"a1b2c3d4e5f6a7b8", true},
		{"AABBCCDD", true},
		{"0123456789", true},
		{"xyz", false},
		{"", false},
		{"a1g2", false},
	}
	for _, tt := range tests {
		if got := isHex(tt.input); got != tt.want {
			t.Errorf("isHex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

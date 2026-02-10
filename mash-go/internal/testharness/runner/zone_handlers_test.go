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
	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeGrid, KeyZoneID: "grid-01"}}
	out, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneCreated] != true {
		t.Error("expected zone_created=true")
	}

	step = &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "home-01"}}
	_, err = r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error creating second zone: %v", err)
	}

	// has_zone.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "grid-01"}}
	out, err = r.handleHasZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneExists] != true {
		t.Error("expected zone_exists=true")
	}

	// list_zones.
	out, err = r.handleListZones(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneCount] != 2 {
		t.Errorf("expected zone_count=2, got %v", out[KeyZoneCount])
	}

	// zone_count.
	out, err = r.handleZoneCount(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyCount] != 2 {
		t.Errorf("expected count=2, got %v", out[KeyCount])
	}
}

func TestZoneDuplicateTypeError(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "z1"}}
	_, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	step = &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "z2"}}
	_, err = r.handleCreateZone(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for duplicate zone type")
	}
}

func TestZoneDeleteRemove(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "u1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// Delete it.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "u1"}}
	out, err := r.handleDeleteZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneRemoved] != true {
		t.Error("expected zone_removed=true")
	}

	// Verify gone.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "u1"}}
	out, _ = r.handleHasZone(context.Background(), step, state)
	if out[KeyZoneExists] != false {
		t.Error("expected zone_exists=false after delete")
	}

	// Delete non-existent -> zone_removed false.
	out, _ = r.handleDeleteZone(context.Background(), step, state)
	if out[KeyZoneRemoved] != false {
		t.Error("expected zone_removed=false for non-existent zone")
	}
}

func TestHighestPriorityZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Create zones with different priorities.
	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "user"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeGrid, KeyZoneID: "grid"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "home"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// Highest priority should be GRID.
	out, err := r.handleHighestPriorityZone(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneType] != ZoneTypeGrid {
		t.Errorf("expected GRID, got %v", out[KeyZoneType])
	}
	if out[KeyZoneID] != "grid" {
		t.Errorf("expected zone_id=grid, got %v", out[KeyZoneID])
	}
}

func TestHighestPriorityConnectedZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeGrid, KeyZoneID: "grid"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "home"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	// None connected -> empty.
	out, _ := r.handleHighestPriorityConnectedZone(context.Background(), &loader.Step{}, state)
	if out[KeyZoneID] != "" {
		t.Error("expected empty zone_id when none connected")
	}

	// Connect home.
	zs := getZoneState(state)
	zs.zones["home"].Connected = true

	out, _ = r.handleHighestPriorityConnectedZone(context.Background(), &loader.Step{}, state)
	if out[KeyZoneID] != "home" {
		t.Errorf("expected home, got %v", out[KeyZoneID])
	}
}

func TestAddZoneAndVerifyBinding(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "h1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{KeyZoneID: "h1", KeyDeviceID: "dev-001"}}
	out, err := r.handleAddZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceAdded] != true {
		t.Error("expected device_added=true")
	}

	// Verify binding.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "h1", KeyDeviceID: "dev-001"}}
	out, _ = r.handleVerifyZoneBinding(context.Background(), step, state)
	if out[KeyBindingValid] != true {
		t.Error("expected binding_valid=true")
	}

	// Non-existent zone.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "no-such-zone"}}
	out, _ = r.handleVerifyZoneBinding(context.Background(), step, state)
	if out[KeyBindingValid] != false {
		t.Error("expected binding_valid=false for unknown zone")
	}
}

func TestVerifyZoneIDDerivation(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Valid: 16 hex chars.
	step := &loader.Step{Params: map[string]any{KeyZoneID: "a1b2c3d4e5f6a7b8"}}
	out, _ := r.handleVerifyZoneIDDerivation(context.Background(), step, state)
	if out[KeyDerivationValid] != true {
		t.Error("expected derivation_valid=true for valid hex ID")
	}

	// Invalid: too short.
	step = &loader.Step{Params: map[string]any{KeyZoneID: "abc123"}}
	out, _ = r.handleVerifyZoneIDDerivation(context.Background(), step, state)
	if out[KeyDerivationValid] != false {
		t.Error("expected derivation_valid=false for short ID")
	}
}

func TestDisconnectZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "h1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)
	zs := getZoneState(state)
	zs.zones["h1"].Connected = true

	step = &loader.Step{Params: map[string]any{KeyZoneID: "h1"}}
	out, _ := r.handleDisconnectZone(context.Background(), step, state)
	if out[KeyZoneDisconnected] != true {
		t.Error("expected zone_disconnected=true")
	}

	if zs.zones["h1"].Connected {
		t.Error("expected zone to be disconnected")
	}
}

func TestHandleCreateZone_SaveZoneID(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "test-zone-123"}}
	out, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeySaveZoneID] != "test-zone-123" {
		t.Errorf("expected save_zone_id=test-zone-123, got %v", out[KeySaveZoneID])
	}
}

func TestHandleVerifyZoneCA_WithRunnerZoneCA(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Create a zone in state (now also generates a real Zone CA).
	step := &loader.Step{Params: map[string]any{KeyZoneType: ZoneTypeLocal, KeyZoneID: "z1"}}
	_, _ = r.handleCreateZone(context.Background(), step, state)

	step = &loader.Step{Params: map[string]any{KeyZoneID: "z1"}}
	out, _ := r.handleVerifyZoneCA(context.Background(), step, state)
	if out[KeyCAValid] != true {
		t.Error("expected ca_valid=true")
	}
	// handleCreateZone now generates a real Zone CA, so cert details should be present.
	if _, exists := out[KeyAlgorithm]; !exists {
		t.Error("expected algorithm field with generated zoneCA")
	}
	if out[KeyBasicConstraintsCA] != true {
		t.Error("expected basic_constraints_ca=true for Zone CA")
	}
}

func TestDisconnectZone_ConnectionOnly(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up connections without zone state entries (mimics two_zones_connected).
	ct := getConnectionTracker(state)
	ct.zoneConnections["GRID"] = &Connection{state: ConnTLSConnected}
	ct.zoneConnections["LOCAL"] = &Connection{state: ConnTLSConnected}

	// Disconnect GRID -- should succeed even without zone state.
	step := &loader.Step{Params: map[string]any{"zone": "GRID"}}
	out, err := r.handleDisconnectZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyZoneDisconnected] != true {
		t.Errorf("expected zone_disconnected=true, got %v", out[KeyZoneDisconnected])
	}

	// GRID should be removed from connections.
	if _, ok := ct.zoneConnections["GRID"]; ok {
		t.Error("expected GRID connection to be removed")
	}

	// LOCAL should still exist.
	if _, ok := ct.zoneConnections["LOCAL"]; !ok {
		t.Error("expected LOCAL connection to still exist")
	}
}

func TestCreateZone_StoresZoneName(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		KeyZoneType: "LOCAL",
		KeyZoneID:   "z1",
		KeyZoneName: "Home Energy",
	}}
	_, err := r.handleCreateZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	zs := getZoneState(state)
	zone := zs.zones["z1"]
	if zone == nil {
		t.Fatal("expected zone z1 to exist")
	}
	if zone.ZoneName != "Home Energy" {
		t.Errorf("expected ZoneName='Home Energy', got %q", zone.ZoneName)
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

func TestCheckSaveZoneID(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Simulate zone creation handler setting save_zone_id in state.
	state.Set(KeySaveZoneID, "a1b2c3d4e5f6a7b8")

	// The checker should save the zone ID under the target key.
	result := r.checkSaveZoneID(KeySaveZoneID, "my_zone_id", state)
	if !result.Passed {
		t.Errorf("expected pass, got: %s", result.Message)
	}

	// Verify the zone ID was saved under the target key.
	saved, exists := state.Get("my_zone_id")
	if !exists {
		t.Fatal("expected my_zone_id to be saved in state")
	}
	if saved != "a1b2c3d4e5f6a7b8" {
		t.Errorf("expected a1b2c3d4e5f6a7b8, got %v", saved)
	}
}

func TestCheckSaveZoneID_NotFound(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Without save_zone_id in state, checker should fail.
	result := r.checkSaveZoneID(KeySaveZoneID, "my_zone_id", state)
	if result.Passed {
		t.Error("expected failure when save_zone_id not in state")
	}
}

func TestHandleVerifyZoneCA_FallbackToMostRecent(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Create a zone first to populate zone state and runner's zoneCA.
	createStep := &loader.Step{
		Params: map[string]any{
			KeyZoneName: "Test Zone",
			KeyZoneType: "LOCAL",
		},
	}
	_, err := r.handleCreateZone(context.Background(), createStep, state)
	if err != nil {
		t.Fatalf("create zone error: %v", err)
	}

	// verify_zone_ca without zone_id param -> should use the most recent zone.
	verifyStep := &loader.Step{Params: map[string]any{}}
	out, err := r.handleVerifyZoneCA(context.Background(), verifyStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyCAValid] != true {
		t.Errorf("expected ca_valid=true, got %v", out[KeyCAValid])
	}
	if out[KeyBasicConstraintsCA] != true {
		t.Errorf("expected basic_constraints_ca=true, got %v", out[KeyBasicConstraintsCA])
	}
}

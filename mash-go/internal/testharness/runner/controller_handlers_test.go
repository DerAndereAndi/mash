package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleControllerAction(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action": "get_controller_id",
		},
	}
	out, err := r.handleControllerAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["controller_id"] != "controller-default" {
		t.Errorf("expected controller-default, got %v", out["controller_id"])
	}

	// Unknown sub_action.
	step = &loader.Step{Params: map[string]any{"sub_action": "nonexistent"}}
	_, err = r.handleControllerAction(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for unknown sub_action")
	}
}

func TestHandleCommissionWithAdmin(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Missing token.
	step := &loader.Step{Params: map[string]any{}}
	_, err := r.handleCommissionWithAdmin(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for missing admin_token")
	}

	// Valid.
	step = &loader.Step{
		Params: map[string]any{
			"admin_token": "token-123",
			KeyDeviceID:   "dev-001",
			KeyZoneID:     "zone-abc",
		},
	}
	out, err := r.handleCommissionWithAdmin(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["commissioned"] != true {
		t.Error("expected commissioned=true")
	}

	// Verify device was added.
	cs := getControllerState(state)
	if cs.devices["dev-001"] != "zone-abc" {
		t.Error("expected device to be tracked")
	}
}

func TestHandleCommissionWithAdmin_TokenValidation(t *testing.T) {
	tests := []struct {
		name              string
		token             string
		expectSuccess     bool
		expectError       string
		expectDeviceAdded bool
	}{
		{
			name:              "valid token succeeds",
			token:             "valid",
			expectSuccess:     true,
			expectDeviceAdded: true,
		},
		{
			name:          "expired token rejected",
			token:         "expired",
			expectSuccess: false,
			expectError:   "INVALID_CERT",
		},
		{
			name:          "invalid_signature token rejected",
			token:         "invalid_signature",
			expectSuccess: false,
			expectError:   "INVALID_CERT",
		},
		{
			name:          "wrong_permissions token rejected",
			token:         "wrong_permissions",
			expectSuccess: false,
			expectError:   "INVALID_CERT",
		},
		{
			name:              "arbitrary token string accepted",
			token:             "some-real-token-abc",
			expectSuccess:     true,
			expectDeviceAdded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRunner()
			state := newTestState()

			step := &loader.Step{
				Params: map[string]any{
					"admin_token": tt.token,
					KeyDeviceID:   "dev-001",
					KeyZoneID:     "zone-abc",
				},
			}
			out, err := r.handleCommissionWithAdmin(context.Background(), step, state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if out[KeyCommissionSuccess] != tt.expectSuccess {
				t.Errorf("commission_success: got %v, want %v", out[KeyCommissionSuccess], tt.expectSuccess)
			}

			if tt.expectError != "" {
				if out[KeyError] != tt.expectError {
					t.Errorf("error: got %v, want %q", out[KeyError], tt.expectError)
				}
			}

			if tt.expectDeviceAdded {
				cs := getControllerState(state)
				if cs.devices["dev-001"] != "zone-abc" {
					t.Error("expected device to be tracked")
				}
			} else {
				cs := getControllerState(state)
				if _, exists := cs.devices["dev-001"]; exists {
					t.Error("device should not be tracked for rejected token")
				}
			}
		})
	}
}

func TestHandleCommissioningWindowDuration(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set.
	step := &loader.Step{Params: map[string]any{"minutes": float64(30)}}
	out, _ := r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["minutes"] != 30.0 {
		t.Errorf("expected 30, got %v", out["minutes"])
	}
	if out["result"] != "ok" {
		t.Errorf("expected result=ok, got %v", out["result"])
	}

	// Get.
	out, _ = r.handleGetCommissioningWindowDuration(context.Background(), &loader.Step{}, state)
	if out["minutes"] != 30.0 {
		t.Errorf("expected 30, got %v", out["minutes"])
	}
}

func TestSetCommissioningWindowDuration_IntParam(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// YAML integers parse as Go int, not float64.
	// duration_seconds as int (600 = 10 minutes).
	step := &loader.Step{Params: map[string]any{"duration_seconds": 600}}
	out, _ := r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["minutes"] != 10.0 {
		t.Errorf("expected 10 minutes from int 600, got %v", out["minutes"])
	}
	if out["result"] != "ok" {
		t.Errorf("expected result=ok, got %v", out["result"])
	}

	// minutes as int.
	step = &loader.Step{Params: map[string]any{"minutes": 30}}
	out, _ = r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["minutes"] != 30.0 {
		t.Errorf("expected 30 minutes from int 30, got %v", out["minutes"])
	}
}

func TestSetCommissioningWindowDuration_DurationSeconds(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set using duration_seconds (600s = 10 minutes).
	step := &loader.Step{Params: map[string]any{"duration_seconds": float64(600)}}
	out, _ := r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["minutes"] != 10.0 {
		t.Errorf("expected 10 minutes from 600s, got %v", out["minutes"])
	}
	if out["duration_seconds"] != 600.0 {
		t.Errorf("expected duration_seconds=600, got %v", out["duration_seconds"])
	}
	if out["result"] != "ok" {
		t.Errorf("expected result=ok, got %v", out["result"])
	}
}

func TestSetCommissioningWindowDuration_Clamped(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Too short (1s, minimum is 3 seconds).
	step := &loader.Step{Params: map[string]any{"duration_seconds": float64(1)}}
	out, _ := r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["result"] != "clamped_or_rejected" {
		t.Errorf("expected clamped_or_rejected for 1s, got %v", out["result"])
	}
	if out["minutes"] != 3.0/60.0 {
		t.Errorf("expected clamped to 0.05 minutes (3s), got %v", out["minutes"])
	}

	// Too long (15000s = 250 minutes, maximum is 180 minutes).
	step = &loader.Step{Params: map[string]any{"duration_seconds": float64(15000)}}
	out, _ = r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["result"] != "clamped_or_rejected" {
		t.Errorf("expected clamped_or_rejected for 15000s, got %v", out["result"])
	}
	if out["minutes"] != 180.0 {
		t.Errorf("expected clamped to 180 minutes, got %v", out["minutes"])
	}
}

func TestGetCommissioningWindowDuration_Seconds(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set to 20 minutes.
	step := &loader.Step{Params: map[string]any{"minutes": float64(20)}}
	r.handleSetCommissioningWindowDuration(context.Background(), step, state)

	// Get and verify seconds fields.
	out, _ := r.handleGetCommissioningWindowDuration(context.Background(), &loader.Step{}, state)
	if out["duration_seconds"] != 1200.0 {
		t.Errorf("expected duration_seconds=1200, got %v", out["duration_seconds"])
	}
	if out["duration_seconds_min"] != 1200.0 {
		t.Errorf("expected duration_seconds_min=1200, got %v", out["duration_seconds_min"])
	}
	if out["duration_seconds_max"] != 1200.0 {
		t.Errorf("expected duration_seconds_max=1200, got %v", out["duration_seconds_max"])
	}
}

func TestHandleRemoveDevice(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	cs := getControllerState(state)
	cs.devices["dev-001"] = "zone-abc"

	step := &loader.Step{Params: map[string]any{KeyDeviceID: "dev-001"}}
	out, _ := r.handleRemoveDevice(context.Background(), step, state)
	if out["device_removed"] != true {
		t.Error("expected device_removed=true")
	}

	// Remove non-existent.
	out, _ = r.handleRemoveDevice(context.Background(), step, state)
	if out["device_removed"] != false {
		t.Error("expected device_removed=false for non-existent device")
	}
}

func TestHandleRemoveDevice_ClearsPreconditionState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Simulate device in two zones -- only set the two-zones precondition,
	// matching the real YAML precondition (device_in_two_zones: true).
	state.Set(PrecondDeviceInTwoZones, true)

	cs := getControllerState(state)
	cs.devices["dev-001"] = "zone-abc"
	cs.devices["dev-001-2"] = "zone-def"

	// Remove from a specific zone (not "all").
	step := &loader.Step{Params: map[string]any{
		KeyDeviceID: "dev-001",
		"zone":      "zone-abc",
	}}
	out, err := r.handleRemoveDevice(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceRemoved] != true {
		t.Error("expected device_removed=true")
	}

	// PrecondDeviceInTwoZones should now be false (removed from one zone).
	v, _ := state.Get(PrecondDeviceInTwoZones)
	if v != false {
		t.Error("expected device_in_two_zones=false after single zone removal")
	}

	// PrecondDeviceInZone should still be true (one device remains).
	v, _ = state.Get(PrecondDeviceInZone)
	if v != true {
		t.Error("expected device_in_zone=true (one device still present)")
	}
}

func TestHandleRemoveDevice_ClearsAllZonePreconditions(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Simulate device in zone.
	state.Set(PrecondDeviceInZone, true)
	state.Set(PrecondDeviceInTwoZones, true)

	cs := getControllerState(state)
	cs.devices["dev-001"] = "zone-abc"

	// Remove with zone="all".
	step := &loader.Step{Params: map[string]any{
		KeyDeviceID: "dev-001",
		"zone":      "all",
	}}
	out, err := r.handleRemoveDevice(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceRemoved] != true {
		t.Error("expected device_removed=true")
	}

	// Both preconditions should be cleared.
	v, _ := state.Get(PrecondDeviceInZone)
	if v != false {
		t.Error("expected device_in_zone=false after remove all")
	}
	v, _ = state.Get(PrecondDeviceInTwoZones)
	if v != false {
		t.Error("expected device_in_two_zones=false after remove all")
	}
}

func TestHandleRemoveDevice_ZoneOnlyParam_SingleZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Matches TC-MASHO-006: device_in_zone precondition, no device_id in params.
	state.Set(PrecondDeviceInZone, true)

	step := &loader.Step{Params: map[string]any{
		"zone": "all",
	}}
	out, err := r.handleRemoveDevice(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceRemoved] != true {
		t.Error("expected device_removed=true")
	}

	v, _ := state.Get(PrecondDeviceInZone)
	if v != false {
		t.Error("expected device_in_zone=false after remove all (zone-only param)")
	}
	v, _ = state.Get(StateDeviceWasRemoved)
	if v != true {
		t.Error("expected device_was_removed=true after remove all")
	}
}

func TestHandleRemoveDevice_ZoneOnlyParam_TwoZones(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Matches TC-MASHO-005: device_in_two_zones precondition, no device_id in params.
	state.Set(PrecondDeviceInTwoZones, true)

	step := &loader.Step{Params: map[string]any{
		"zone": "zone_1",
	}}
	out, err := r.handleRemoveDevice(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceRemoved] != true {
		t.Error("expected device_removed=true")
	}

	v, _ := state.Get(PrecondDeviceInTwoZones)
	if v != false {
		t.Error("expected device_in_two_zones=false after removing one zone")
	}
	v, _ = state.Get(PrecondDeviceInZone)
	if v != true {
		t.Error("expected device_in_zone=true (one zone remains)")
	}
}

func TestHandleCheckRenewal(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set("renewal_complete", true)
	state.Set("status", 0)

	out, _ := r.handleCheckRenewal(context.Background(), &loader.Step{}, state)
	if out["renewal_complete"] != true {
		t.Error("expected renewal_complete=true")
	}
}

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
			"device_id":   "dev-001",
			"zone_id":     "zone-abc",
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

	// Too short (60s = 1 minute, minimum is 3 minutes).
	step := &loader.Step{Params: map[string]any{"duration_seconds": float64(60)}}
	out, _ := r.handleSetCommissioningWindowDuration(context.Background(), step, state)
	if out["result"] != "clamped_or_rejected" {
		t.Errorf("expected clamped_or_rejected for 60s, got %v", out["result"])
	}
	if out["minutes"] != 3.0 {
		t.Errorf("expected clamped to 3 minutes, got %v", out["minutes"])
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

	step := &loader.Step{Params: map[string]any{"device_id": "dev-001"}}
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

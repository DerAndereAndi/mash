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
			"sub_action":  "get_controller_id",
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

	// Get.
	out, _ = r.handleGetCommissioningWindowDuration(context.Background(), &loader.Step{}, state)
	if out["minutes"] != 30.0 {
		t.Errorf("expected 30, got %v", out["minutes"])
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

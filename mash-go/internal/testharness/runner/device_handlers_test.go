package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleChangeState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"operating_state": "RUNNING",
			"control_state":   "CONTROLLED",
			"process_state":   "SCHEDULED",
		},
	}
	out, err := r.handleChangeState(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["state_changed"] != true {
		t.Error("expected state_changed=true")
	}
	if out["operating_state"] != "RUNNING" {
		t.Errorf("expected RUNNING, got %v", out["operating_state"])
	}
	if out["control_state"] != "CONTROLLED" {
		t.Errorf("expected CONTROLLED, got %v", out["control_state"])
	}
}

func TestHandleTriggerAndClearFault(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Trigger fault.
	step := &loader.Step{
		Params: map[string]any{
			"fault_code":    float64(42),
			"fault_message": "overcurrent",
		},
	}
	out, err := r.handleTriggerFault(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["fault_triggered"] != true {
		t.Error("expected fault_triggered=true")
	}
	if out["active_fault_count"] != 1 {
		t.Errorf("expected 1 fault, got %v", out["active_fault_count"])
	}

	ds := getDeviceState(state)
	if ds.operatingState != "FAULT" {
		t.Errorf("expected FAULT state, got %s", ds.operatingState)
	}

	// Clear fault.
	step = &loader.Step{
		Params: map[string]any{"fault_code": float64(42)},
	}
	out, err = r.handleClearFault(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["fault_cleared"] != true {
		t.Error("expected fault_cleared=true")
	}
	if out["active_fault_count"] != 0 {
		t.Errorf("expected 0 faults, got %v", out["active_fault_count"])
	}
	if ds.operatingState != "STANDBY" {
		t.Errorf("expected STANDBY after all faults cleared, got %s", ds.operatingState)
	}
}

func TestHandleDeviceLocalAction(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action":      "change_state",
			"operating_state": "RUNNING",
		},
	}
	out, err := r.handleDeviceLocalAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["operating_state"] != "RUNNING" {
		t.Errorf("expected RUNNING, got %v", out["operating_state"])
	}

	// Unknown sub_action.
	step = &loader.Step{
		Params: map[string]any{"sub_action": "nonexistent"},
	}
	_, err = r.handleDeviceLocalAction(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for unknown sub_action")
	}
}

func TestHandleEVLifecycle(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Plug in cable.
	out, _ := r.handlePlugInCable(context.Background(), &loader.Step{}, state)
	if out["cable_plugged_in"] != true {
		t.Error("expected cable_plugged_in=true")
	}

	// Connect EV.
	out, _ = r.handleEVConnect(context.Background(), &loader.Step{}, state)
	if out["ev_connected"] != true {
		t.Error("expected ev_connected=true")
	}

	// Request charge.
	out, _ = r.handleEVRequestsCharge(context.Background(), &loader.Step{}, state)
	if out["charge_requested"] != true {
		t.Error("expected charge_requested=true")
	}

	// Disconnect EV.
	out, _ = r.handleEVDisconnect(context.Background(), &loader.Step{}, state)
	if out["ev_disconnected"] != true {
		t.Error("expected ev_disconnected=true")
	}

	ds := getDeviceState(state)
	if ds.evConnected {
		t.Error("expected evConnected=false after disconnect")
	}
}

func TestHandleVerifyDeviceState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Default state.
	step := &loader.Step{
		Params: map[string]any{
			"operating_state": "STANDBY",
			"control_state":   "AUTONOMOUS",
		},
	}
	out, _ := r.handleVerifyDeviceState(context.Background(), step, state)
	if out["state_matches"] != true {
		t.Error("expected state_matches=true for default state")
	}

	// Mismatched.
	step = &loader.Step{
		Params: map[string]any{"operating_state": "RUNNING"},
	}
	out, _ = r.handleVerifyDeviceState(context.Background(), step, state)
	if out["state_matches"] != false {
		t.Error("expected state_matches=false for mismatch")
	}
}

func TestHandleFactoryReset(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Change some state.
	ds := getDeviceState(state)
	ds.operatingState = "FAULT"
	ds.controlState = "FAILSAFE"

	out, _ := r.handleFactoryReset(context.Background(), &loader.Step{}, state)
	if out["factory_reset"] != true {
		t.Error("expected factory_reset=true")
	}

	ds = getDeviceState(state)
	if ds.operatingState != "STANDBY" {
		t.Errorf("expected STANDBY after reset, got %s", ds.operatingState)
	}
	if ds.controlState != "AUTONOMOUS" {
		t.Errorf("expected AUTONOMOUS after reset, got %s", ds.controlState)
	}
}

func TestHandleUserOverride(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleUserOverride(context.Background(), &loader.Step{}, state)
	if out["control_state"] != "OVERRIDE" {
		t.Errorf("expected OVERRIDE, got %v", out["control_state"])
	}
}

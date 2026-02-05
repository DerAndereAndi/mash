package runner

import (
	"context"
	"net"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
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
	if out[StateOperatingState] != OperatingStateRunning {
		t.Errorf("expected RUNNING, got %v", out[StateOperatingState])
	}
	if out[StateControlState] != ControlStateControlled {
		t.Errorf("expected CONTROLLED, got %v", out[StateControlState])
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
	if ds.operatingState != OperatingStateFault {
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
	if ds.operatingState != OperatingStateStandby {
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
	if out[StateOperatingState] != OperatingStateRunning {
		t.Errorf("expected RUNNING, got %v", out[StateOperatingState])
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
	ds.operatingState = OperatingStateFault
	ds.controlState = ControlStateFailsafe

	out, _ := r.handleFactoryReset(context.Background(), &loader.Step{}, state)
	if out["factory_reset"] != true {
		t.Error("expected factory_reset=true")
	}

	ds = getDeviceState(state)
	if ds.operatingState != OperatingStateStandby {
		t.Errorf("expected STANDBY after reset, got %s", ds.operatingState)
	}
	if ds.controlState != ControlStateAutonomous {
		t.Errorf("expected AUTONOMOUS after reset, got %s", ds.controlState)
	}
}

func TestHandleUserOverride(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleUserOverride(context.Background(), &loader.Step{}, state)
	if out[StateControlState] != ControlStateOverride {
		t.Errorf("expected OVERRIDE, got %v", out[StateControlState])
	}
}

func TestHandleDeviceLocalAction_CheckDisplay(t *testing.T) {
	r := newTestRunner()
	r.config.SetupCode = "12345678"
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action":    "check_display",
			"discriminator": float64(1234),
			"setup_code":    "12345678",
		},
	}
	out, err := r.handleDeviceLocalAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["qr_present"] != true {
		t.Error("expected qr_present=true")
	}
	if out["format_valid"] != true {
		t.Error("expected format_valid=true")
	}
	if _, ok := out["discriminator_length"]; !ok {
		t.Error("expected discriminator_length field")
	}
	if out["action_triggered"] != true {
		t.Error("expected action_triggered=true")
	}
}

func TestHandleDeviceLocalAction_GetQR(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action":    "get_qr",
			"discriminator": float64(567),
			"setup_code":    "87654321",
		},
	}
	out, err := r.handleDeviceLocalAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["qr_present"] != true {
		t.Error("expected qr_present=true")
	}
	if out["format_valid"] != true {
		t.Error("expected format_valid=true")
	}
	if _, ok := out["setup_code_length"]; !ok {
		t.Error("expected setup_code_length field")
	}
}

func TestHandleDeviceLocalAction_CheckDisplayAutoGenerated(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No explicit discriminator/setup_code -> auto-generates QR payload.
	step := &loader.Step{
		Params: map[string]any{
			"sub_action": "check_display",
		},
	}
	out, err := r.handleDeviceLocalAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["qr_present"] != true {
		t.Error("expected qr_present=true (auto-generated)")
	}
}

func TestHandleDeviceLocalAction_CheckDisplayQRDisplayed(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action": "check_display",
		},
	}
	out, err := r.handleDeviceLocalAction(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyQRDisplayed] != true {
		t.Errorf("expected qr_displayed=true, got %v", out[KeyQRDisplayed])
	}
}

func TestHandleConfigureDevice(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"endpoints": []any{"ep1"},
			"features":  []any{"EnergyControl"},
		},
	}
	out, err := r.handleConfigureDevice(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyDeviceConfigured] != true {
		t.Error("expected device_configured=true")
	}
	if out[KeyConfigurationSuccess] != true {
		t.Errorf("expected configuration_success=true, got %v", out[KeyConfigurationSuccess])
	}
}

// newTriggerTestRunner creates a Runner with a net.Pipe-backed connection
// and config.Target set, for testing handlers that call sendTriggerViaZone.
func newTriggerTestRunner() (*Runner, *engine.ExecutionState, net.Conn) {
	r, server := newPipedRunner()
	r.config.Target = "localhost:8443"
	r.config.EnableKey = "0000000000000000"
	state := newTestState()
	return r, state, server
}

func TestHandleTriggerFault_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"fault_code":    42,
		"fault_message": "test fault",
	}}
	out, err := r.handleTriggerFault(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["fault_triggered"] != true {
		t.Error("expected fault_triggered=true")
	}
}

func TestHandleClearFault_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()

	// First add a fault to clear.
	ds := getDeviceState(state)
	ds.faults = append(ds.faults, faultEntry{Code: 42, Message: "test"})
	ds.operatingState = OperatingStateFault

	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"fault_code": 42,
	}}
	out, err := r.handleClearFault(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["fault_cleared"] != true {
		t.Error("expected fault_cleared=true")
	}
}

func TestHandleEVConnect_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleEVConnect(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyEVConnected] != true {
		t.Error("expected ev_connected=true")
	}
}

func TestHandleEVDisconnect_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleEVDisconnect(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyEVDisconnected] != true {
		t.Error("expected ev_disconnected=true")
	}
}

func TestHandleEVRequestsCharge_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleEVRequestsCharge(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["charge_requested"] != true {
		t.Error("expected charge_requested=true")
	}
}

func TestHandlePlugInCable_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handlePlugInCable(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["cable_plugged_in"] != true {
		t.Error("expected cable_plugged_in=true")
	}
}

func TestHandleUnplugCable_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleUnplugCable(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["cable_unplugged"] != true {
		t.Error("expected cable_unplugged=true")
	}
}

func TestHandleStartOperation_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleStartOperation(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["operation_started"] != true {
		t.Error("expected operation_started=true")
	}
}

func TestHandleFactoryReset_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleFactoryReset(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["factory_reset"] != true {
		t.Error("expected factory_reset=true")
	}
}

func TestHandleMakeProcessAvailable_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleMakeProcessAvailable(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[StateProcessState] != ProcessStateAvailable {
		t.Errorf("expected process_state=AVAILABLE, got %v", out[StateProcessState])
	}
}

func TestHandleUserOverride_SendsTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleUserOverride(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[StateControlState] != ControlStateOverride {
		t.Errorf("expected control_state=OVERRIDE, got %v", out[StateControlState])
	}
}

func TestDeviceStateModified_SetByTrigger(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()
	go serverEchoResponse(server)

	if r.deviceStateModified {
		t.Fatal("expected deviceStateModified=false initially")
	}

	step := &loader.Step{Params: map[string]any{}}
	_, err := r.handleMakeProcessAvailable(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.deviceStateModified {
		t.Error("expected deviceStateModified=true after trigger send")
	}
}

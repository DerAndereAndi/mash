package runner

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
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
	r.config.SetupCode = "20202021"
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"sub_action":    "check_display",
			"discriminator": float64(1234),
			"setup_code":    "20202021",
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

	if r.connMgr.DeviceStateModified() {
		t.Fatal("expected deviceStateModified=false initially")
	}

	step := &loader.Step{Params: map[string]any{}}
	_, err := r.handleMakeProcessAvailable(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !r.connMgr.DeviceStateModified() {
		t.Error("expected deviceStateModified=true after trigger send")
	}
}

// Regression for G15 subscription failures:
// device_set_value must route TriggerTestEvent over the suite TEST control
// channel, not require pool.Main().
func TestHandleDeviceSetValue_UsesSuiteControlChannel(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:8443"
	r.config.EnableKey = "00112233445566778899aabbccddeeff"

	// Main is disconnected; suite control channel is available.
	r.pool.Main().state = ConnDisconnected
	suiteConn, suiteServer := newPipeConnection()
	r.suite.SetConn(suiteConn)
	defer suiteServer.Close()

	state := newTestState()
	step := &loader.Step{Params: map[string]any{
		"endpoint":  float64(1),
		"feature":   "Measurement",
		"attribute": "acActivePower",
		"value":     float64(4000000),
	}}

	go echoSuccessResponse(suiteServer)

	out, err := r.handleDeviceSetValue(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyValueSet] != true {
		t.Fatalf("expected value_set=true, got out=%v", out)
	}
	if out[KeyTriggerSent] != true {
		t.Fatalf("expected trigger_sent=true, got out=%v", out)
	}
	if got, _ := state.Get("acActivePower"); got != float64(4000000) {
		t.Fatalf("expected state acActivePower=4000000, got %v", got)
	}
}

// Ensure device_set_value does not regress to main-path usage when both are
// present; suite must be preferred to avoid non-TEST NOT_AUTHORIZED behavior.
func TestHandleDeviceSetValue_PrefersSuiteOverMain(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:8443"
	r.config.EnableKey = "00112233445566778899aabbccddeeff"

	// Main is live but has no responder; if used, request would block/fail.
	mainConn, mainServer := newPipeConnection()
	r.pool.SetMain(mainConn)
	defer mainServer.Close()

	suiteConn, suiteServer := newPipeConnection()
	r.suite.SetConn(suiteConn)
	defer suiteServer.Close()

	state := newTestState()
	step := &loader.Step{Params: map[string]any{
		"endpoint":  float64(1),
		"feature":   "Measurement",
		"attribute": "acActivePower",
		"value":     float64(8888000),
	}}

	go echoSuccessResponse(suiteServer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := r.handleDeviceSetValue(ctx, step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyValueSet] != true || out[KeyTriggerSent] != true {
		t.Fatalf("expected suite trigger success, got out=%v", out)
	}
}

// Multi-zone preconditions intentionally remove suite TEST control to free
// device slots. In strict lifecycle mode, trigger delivery must still work
// via active main connection when no suite zone exists.
func TestHandleChangeState_StrictLifecycle_AllowsMainWhenSuiteAbsent(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()

	r.config.StrictLifecycle = true
	r.suite.Clear() // No suite zone commissioned/connected in this scenario.

	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"operating_state": "RUNNING",
	}}
	out, err := r.handleChangeState(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyStateChanged] != true {
		t.Fatalf("expected state_changed=true, got out=%v", out)
	}
	if out[StateOperatingState] != "RUNNING" {
		t.Fatalf("expected operating_state=RUNNING, got out=%v", out)
	}
}

func TestHandleChangeState_StrictLifecycle_MainFallbackReturnsTriggerStatusError(t *testing.T) {
	r, state, server := newTriggerTestRunner()
	defer server.Close()

	r.config.StrictLifecycle = true
	r.suite.Clear() // No suite zone commissioned/connected in this scenario.

	go echoErrorStatusResponse(server, wire.StatusNotAuthorized)

	step := &loader.Step{Params: map[string]any{
		"operating_state": "RUNNING",
	}}
	_, err := r.handleChangeState(context.Background(), step, state)
	if err == nil {
		t.Fatal("expected trigger status error when main fallback receives NOT_AUTHORIZED")
	}
	if !strings.Contains(err.Error(), "status 8") {
		t.Fatalf("expected trigger status code in error, got: %v", err)
	}
}

func echoErrorStatusResponse(server net.Conn, status wire.Status) {
	fr := transport.NewFramer(server)
	reqData, err := fr.ReadFrame()
	if err != nil {
		return
	}
	req, err := wire.DecodeRequest(reqData)
	if err != nil {
		return
	}
	resp := &wire.Response{
		MessageID: req.MessageID,
		Status:    status,
	}
	respData, err := wire.EncodeResponse(resp)
	if err != nil {
		return
	}
	_ = fr.WriteFrame(respData)
}

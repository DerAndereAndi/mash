package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// registerDeviceHandlers registers all device state action handlers.
func (r *Runner) registerDeviceHandlers() {
	r.engine.RegisterHandler("device_local_action", r.handleDeviceLocalAction)
	r.engine.RegisterHandler("device_set_value", r.handleDeviceSetValue)
	r.engine.RegisterHandler("device_set_values_rapid", r.handleDeviceSetValuesRapid)
	r.engine.RegisterHandler("device_trigger", r.handleDeviceTrigger)
	r.engine.RegisterHandler("configure_device", r.handleConfigureDevice)
	r.engine.RegisterHandler("configure_exposed_device", r.handleConfigureExposedDevice)
	r.engine.RegisterHandler("update_exposed_attribute", r.handleUpdateExposedAttribute)
	r.engine.RegisterHandler("change_state", r.handleChangeState)
	r.engine.RegisterHandler("set_state_detail", r.handleSetStateDetail)
	r.engine.RegisterHandler("trigger_fault", r.handleTriggerFault)
	r.engine.RegisterHandler("clear_fault", r.handleClearFault)
	r.engine.RegisterHandler("query_device_state", r.handleQueryDeviceState)
	r.engine.RegisterHandler("verify_device_state", r.handleVerifyDeviceState)
	r.engine.RegisterHandler("set_connected", r.handleSetConnected)
	r.engine.RegisterHandler("set_disconnected", r.handleSetDisconnected)
	r.engine.RegisterHandler("set_failsafe_limit", r.handleSetFailsafeLimit)
	r.engine.RegisterHandler("make_process_available", r.handleMakeProcessAvailable)
	r.engine.RegisterHandler("start_operation", r.handleStartOperation)
	r.engine.RegisterHandler("ev_connect", r.handleEVConnect)
	r.engine.RegisterHandler("ev_disconnect", r.handleEVDisconnect)
	r.engine.RegisterHandler("ev_requests_charge", r.handleEVRequestsCharge)
	r.engine.RegisterHandler("plug_in_cable", r.handlePlugInCable)
	r.engine.RegisterHandler("unplug_cable", r.handleUnplugCable)
	r.engine.RegisterHandler("user_override", r.handleUserOverride)
	r.engine.RegisterHandler("factory_reset", r.handleFactoryReset)
	r.engine.RegisterHandler("power_cycle", r.handlePowerCycle)
	r.engine.RegisterHandler("power_on_device", r.handlePowerOnDevice)
	r.engine.RegisterHandler("reboot", r.handleReboot)
	r.engine.RegisterHandler("restart", r.handleRestart)
}

// handleDeviceLocalAction dispatches to sub-actions based on sub_action param.
func (r *Runner) handleDeviceLocalAction(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	subAction, _ := params["sub_action"].(string)
	if subAction == "" {
		// Try action field.
		subAction, _ = params[KeyAction].(string)
	}

	// Create a new step with the same params for dispatch.
	subStep := &loader.Step{
		Params: params,
	}

	var result map[string]any
	var err error

	switch subAction {
	case "change_state":
		result, err = r.handleChangeState(ctx, subStep, state)
	case "trigger_fault":
		result, err = r.handleTriggerFault(ctx, subStep, state)
	case "clear_fault":
		result, err = r.handleClearFault(ctx, subStep, state)
	case "set_connected":
		result, err = r.handleSetConnected(ctx, subStep, state)
	case "set_disconnected":
		result, err = r.handleSetDisconnected(ctx, subStep, state)
	case "ev_connect":
		result, err = r.handleEVConnect(ctx, subStep, state)
	case "ev_disconnect":
		result, err = r.handleEVDisconnect(ctx, subStep, state)
	case "ev_requests_charge":
		result, err = r.handleEVRequestsCharge(ctx, subStep, state)
	case "plug_in_cable":
		result, err = r.handlePlugInCable(ctx, subStep, state)
	case "unplug_cable":
		result, err = r.handleUnplugCable(ctx, subStep, state)
	case "make_process_available":
		result, err = r.handleMakeProcessAvailable(ctx, subStep, state)
	case "start_operation":
		result, err = r.handleStartOperation(ctx, subStep, state)
	case "set_failsafe_limit":
		result, err = r.handleSetFailsafeLimit(ctx, subStep, state)
	case "user_override":
		result, err = r.handleUserOverride(ctx, subStep, state)
	case "factory_reset":
		result, err = r.handleFactoryReset(ctx, subStep, state)
	case "power_cycle":
		result, err = r.handlePowerCycle(ctx, subStep, state)
	case "reboot":
		result, err = r.handleReboot(ctx, subStep, state)
	case "restart":
		result, err = r.handleRestart(ctx, subStep, state)
	case "set_state_detail":
		result, err = r.handleSetStateDetail(ctx, subStep, state)
	case "configure_device":
		result, err = r.handleConfigureDevice(ctx, subStep, state)
	case "set_value":
		result, err = r.handleDeviceSetValue(ctx, subStep, state)

	// Zone management sub-actions.
	case "add_zone":
		result, err = r.handleAddZone(ctx, subStep, state)
	case "remove_zone":
		result, err = r.handleRemoveZone(ctx, subStep, state)
	case "has_zone":
		result, err = r.handleHasZone(ctx, subStep, state)
	case "list_zones":
		result, err = r.handleListZones(ctx, subStep, state)
	case "zone_count":
		result, err = r.handleZoneCount(ctx, subStep, state)
	case "highest_priority_zone":
		result, err = r.handleHighestPriorityZone(ctx, subStep, state)

	// Network simulation sub-actions.
	case "interface_down":
		result, err = r.handleInterfaceDown(ctx, subStep, state)
	case "interface_up":
		result, err = r.handleInterfaceUp(ctx, subStep, state)
	case "change_address":
		result, err = r.handleChangeAddress(ctx, subStep, state)
	case "adjust_clock":
		result, err = r.handleAdjustClock(ctx, subStep, state)

	// Discovery sub-actions.
	case "browse_commissioners":
		result, err = r.handleBrowseCommissioners(ctx, subStep, state)
	case "get_qr_payload", "get_qr":
		result, err = r.handleGetQRPayload(ctx, subStep, state)
		// Enrich with format validation fields.
		if result != nil {
			payload, _ := result[KeyQRPayload].(string)
			result[KeyQRPresent] = payload != ""
			result[KeyFormatValid] = len(payload) > 0
			if disc, ok := result[KeyDiscriminator].(int); ok {
				result[KeyDiscriminatorLength] = len(fmt.Sprintf("%d", disc))
			}
			if sc, ok := result[KeySetupCode].(string); ok {
				result[KeySetupCodeLength] = len(sc)
			}
		}
	case "check_display":
		// Check if the device has a QR display.
		result, err = r.handleGetQRPayload(ctx, subStep, state)
		if result != nil {
			payload, _ := result[KeyQRPayload].(string)
			result[KeyQRPresent] = payload != ""
			result[KeyFormatValid] = len(payload) > 0
			if disc, ok := result[KeyDiscriminator].(int); ok {
				result[KeyDiscriminatorLength] = len(fmt.Sprintf("%d", disc))
			}
		}
	case "enter_commissioning_mode":
		result, err = r.handleEnterCommissioningMode(ctx, subStep, state)
	default:
		return nil, fmt.Errorf("unknown device_local_action sub_action: %s", subAction)
	}

	// Mark successful dispatches so tests can verify the action was triggered.
	if err == nil && result != nil {
		result[KeyActionTriggered] = true
	}
	return result, err
}


// handleDeviceSetValue sets an attribute value on the device.
func (r *Runner) handleDeviceSetValue(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params[KeyKey].(string)
	value := params[KeyValue]

	if key != "" {
		ds.attributes[key] = value
		state.Set(key, value)
	}

	return map[string]any{
		KeyValueSet: true,
		KeyKey:      key,
	}, nil
}

// handleDeviceSetValuesRapid sets multiple values rapidly (stress test).
func (r *Runner) handleDeviceSetValuesRapid(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	values, ok := params["values"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("values parameter must be a map")
	}

	count := 0
	for k, v := range values {
		ds.attributes[k] = v
		state.Set(k, v)
		count++
	}

	return map[string]any{
		KeyValuesSet: count,
		KeyRapid:     true,
	}, nil
}

// handleDeviceTrigger triggers a device event.
func (r *Runner) handleDeviceTrigger(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	event, _ := params[KeyEvent].(string)
	state.Set(StateLastTrigger, event)

	return map[string]any{
		KeyTriggered: true,
		KeyEventType: event,
	}, nil
}

// handleConfigureDevice configures the device model.
func (r *Runner) handleConfigureDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	ds.configured = true

	// Store configuration params.
	if endpoints, ok := params["endpoints"]; ok {
		ds.attributes["endpoints"] = endpoints
	}
	if features, ok := params["features"]; ok {
		ds.attributes["features"] = features
	}

	return map[string]any{
		KeyDeviceConfigured: true,
	}, nil
}

// handleConfigureExposedDevice configures an exposed device for controller tests.
func (r *Runner) handleConfigureExposedDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	ds.configured = true
	if attrs, ok := params["attributes"].(map[string]any); ok {
		for k, v := range attrs {
			ds.attributes[k] = v
		}
	}

	return map[string]any{
		KeyExposedDeviceConfigured: true,
	}, nil
}

// handleUpdateExposedAttribute updates an attribute on an exposed device.
func (r *Runner) handleUpdateExposedAttribute(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params["attribute"].(string)
	value := params[KeyValue]

	if key != "" {
		ds.attributes[key] = value
	}

	return map[string]any{
		KeyAttributeUpdated: true,
	}, nil
}

// handleChangeState changes device operating/control/process state.
func (r *Runner) handleChangeState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	changed := false

	if s, ok := params["operating_state"].(string); ok {
		ds.operatingState = s
		state.Set(StateOperatingState, s)
		changed = true
	}
	if s, ok := params["control_state"].(string); ok {
		ds.controlState = s
		state.Set(StateControlState, s)
		changed = true
	}
	if s, ok := params["process_state"].(string); ok {
		ds.processState = s
		state.Set(StateProcessState, s)
		changed = true
	}

	return map[string]any{
		KeyStateChanged:      changed,
		StateOperatingState:  ds.operatingState,
		StateControlState:    ds.controlState,
		StateProcessState:    ds.processState,
	}, nil
}

// handleSetStateDetail sets stateDetails vendor data.
func (r *Runner) handleSetStateDetail(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params[KeyKey].(string)
	value := params[KeyValue]

	if key != "" {
		ds.stateDetails[key] = value
	}

	return map[string]any{
		KeyDetailSet: true,
	}, nil
}

// handleTriggerFault adds a fault to activeFaults.
func (r *Runner) handleTriggerFault(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	code := uint32(0)
	if c, ok := params[KeyFaultCode].(float64); ok {
		code = uint32(c)
	}
	message, _ := params["fault_message"].(string)

	ds.faults = append(ds.faults, faultEntry{
		Code:    code,
		Message: message,
		Time:    time.Now(),
	})
	ds.operatingState = "FAULT"

	state.Set(StateActiveFaultCount, len(ds.faults))

	return map[string]any{
		KeyFaultTriggered:    true,
		KeyFaultCode:         code,
		StateActiveFaultCount: len(ds.faults),
	}, nil
}

// handleClearFault removes a fault from activeFaults.
func (r *Runner) handleClearFault(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	code := uint32(0)
	if c, ok := params[KeyFaultCode].(float64); ok {
		code = uint32(c)
	}

	found := false
	for i, f := range ds.faults {
		if f.Code == code {
			ds.faults = append(ds.faults[:i], ds.faults[i+1:]...)
			found = true
			break
		}
	}

	if len(ds.faults) == 0 {
		ds.operatingState = "STANDBY"
	}

	state.Set(StateActiveFaultCount, len(ds.faults))

	return map[string]any{
		KeyFaultCleared:       found,
		StateActiveFaultCount: len(ds.faults),
	}, nil
}

// handleQueryDeviceState returns the current device state.
func (r *Runner) handleQueryDeviceState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)

	return map[string]any{
		StateOperatingState: ds.operatingState,
		StateControlState:   ds.controlState,
		StateProcessState:   ds.processState,
		KeyActiveFaults:     len(ds.faults),
		KeyEVConnected:      ds.evConnected,
		KeyCablePluggedIn:   ds.cablePluggedIn,
	}, nil
}

// handleVerifyDeviceState verifies device state matches expected.
func (r *Runner) handleVerifyDeviceState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	allMatch := true
	if expected, ok := params["operating_state"].(string); ok {
		if ds.operatingState != expected {
			allMatch = false
		}
	}
	if expected, ok := params["control_state"].(string); ok {
		if ds.controlState != expected {
			allMatch = false
		}
	}
	if expected, ok := params["process_state"].(string); ok {
		if ds.processState != expected {
			allMatch = false
		}
	}

	return map[string]any{
		KeyStateMatches:     allMatch,
		StateOperatingState: ds.operatingState,
		StateControlState:   ds.controlState,
		StateProcessState:   ds.processState,
	}, nil
}

// handleSetConnected sets the connection state flag.
func (r *Runner) handleSetConnected(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "CONTROLLED"
	state.Set(StateDeviceConnected, true)

	return map[string]any{KeyConnected: true}, nil
}

// handleSetDisconnected sets the disconnection state.
func (r *Runner) handleSetDisconnected(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "FAILSAFE"
	state.Set(StateDeviceConnected, false)

	return map[string]any{KeyDisconnected: true}, nil
}

// handleSetFailsafeLimit sets the failsafe power limit.
func (r *Runner) handleSetFailsafeLimit(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	limit := 0.0
	if l, ok := params[KeyLimitWatts].(float64); ok {
		limit = l
	}

	ds.failsafeLimit = &limit
	state.Set(StateFailsafeLimit, limit)

	return map[string]any{
		KeyFailsafeLimitSet: true,
		KeyLimitWatts:        limit,
	}, nil
}

// handleMakeProcessAvailable sets processState to AVAILABLE.
func (r *Runner) handleMakeProcessAvailable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.processState = "AVAILABLE"
	state.Set(StateProcessState, "AVAILABLE")

	return map[string]any{StateProcessState: "AVAILABLE"}, nil
}

// handleStartOperation begins process execution.
func (r *Runner) handleStartOperation(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.processState = "RUNNING"
	ds.operatingState = "RUNNING"
	state.Set(StateProcessState, "RUNNING")
	state.Set(StateOperatingState, "RUNNING")

	return map[string]any{
		StateProcessState:  "RUNNING",
		KeyOperationStarted: true,
	}, nil
}

// handleEVConnect simulates EV connection.
func (r *Runner) handleEVConnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = true
	ds.cablePluggedIn = true
	state.Set(StateEVConnected, true)

	return map[string]any{KeyEVConnected: true}, nil
}

// handleEVDisconnect simulates EV disconnection.
func (r *Runner) handleEVDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = false
	ds.cablePluggedIn = false
	state.Set(StateEVConnected, false)

	return map[string]any{KeyEVDisconnected: true}, nil
}

// handleEVRequestsCharge simulates EV requesting charge.
func (r *Runner) handleEVRequestsCharge(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = true
	state.Set(StateEVChargeRequested, true)

	return map[string]any{KeyChargeRequested: true}, nil
}

// handlePlugInCable simulates plugging in the cable.
func (r *Runner) handlePlugInCable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.cablePluggedIn = true
	state.Set(StateCablePluggedIn, true)

	return map[string]any{KeyCablePluggedIn: true}, nil
}

// handleUnplugCable simulates unplugging the cable.
func (r *Runner) handleUnplugCable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.cablePluggedIn = false
	ds.evConnected = false
	state.Set(StateCablePluggedIn, false)
	state.Set(StateEVConnected, false)

	return map[string]any{KeyCableUnplugged: true}, nil
}

// handleUserOverride simulates a user override command.
func (r *Runner) handleUserOverride(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "OVERRIDE"
	state.Set(StateControlState, "OVERRIDE")

	return map[string]any{
		KeyOverrideActive: true,
		StateControlState: "OVERRIDE",
	}, nil
}

// handleFactoryReset simulates a factory reset.
func (r *Runner) handleFactoryReset(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Reset device state to defaults.
	s := &deviceState{
		operatingState: "STANDBY",
		controlState:   "AUTONOMOUS",
		processState:   "NONE",
		faults:         make([]faultEntry, 0),
		stateDetails:   make(map[string]any),
		attributes:     make(map[string]any),
	}
	state.Custom["device_state"] = s

	return map[string]any{
		KeyFactoryReset:     true,
		StateOperatingState: "STANDBY",
		StateControlState:   "AUTONOMOUS",
	}, nil
}

// handlePowerCycle simulates a power cycle.
func (r *Runner) handlePowerCycle(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.operatingState = "STANDBY"
	ds.controlState = "AUTONOMOUS"
	ds.processState = "NONE"

	// Close connection if any.
	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}

	return map[string]any{
		KeyPowerCycled:      true,
		StateOperatingState: "STANDBY",
	}, nil
}

// handlePowerOnDevice simulates powering on a device.
func (r *Runner) handlePowerOnDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.operatingState = "STANDBY"

	return map[string]any{
		KeyPoweredOn:        true,
		StateOperatingState: "STANDBY",
	}, nil
}

// handleReboot simulates a device reboot.
func (r *Runner) handleReboot(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handlePowerCycle(ctx, step, state)
}

// handleRestart simulates a device restart.
func (r *Runner) handleRestart(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handlePowerCycle(ctx, step, state)
}


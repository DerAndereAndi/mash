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
		subAction, _ = params["action"].(string)
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
			payload, _ := result["qr_payload"].(string)
			result["qr_present"] = payload != ""
			result["format_valid"] = len(payload) > 0
			if disc, ok := result["discriminator"].(int); ok {
				result["discriminator_length"] = len(fmt.Sprintf("%d", disc))
			}
			if sc, ok := result["setup_code"].(string); ok {
				result["setup_code_length"] = len(sc)
			}
		}
	case "check_display":
		// Check if the device has a QR display.
		result, err = r.handleGetQRPayload(ctx, subStep, state)
		if result != nil {
			payload, _ := result["qr_payload"].(string)
			result["qr_present"] = payload != ""
			result["format_valid"] = len(payload) > 0
			if disc, ok := result["discriminator"].(int); ok {
				result["discriminator_length"] = len(fmt.Sprintf("%d", disc))
			}
		}
	case "enter_commissioning_mode":
		result, err = r.handleEnterCommissioningMode(ctx, subStep, state)
	default:
		return nil, fmt.Errorf("unknown device_local_action sub_action: %s", subAction)
	}

	// Mark successful dispatches so tests can verify the action was triggered.
	if err == nil && result != nil {
		result["action_triggered"] = true
	}
	return result, err
}


// handleDeviceSetValue sets an attribute value on the device.
func (r *Runner) handleDeviceSetValue(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params["key"].(string)
	value := params["value"]

	if key != "" {
		ds.attributes[key] = value
		state.Set(key, value)
	}

	return map[string]any{
		"value_set": true,
		"key":       key,
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
		"values_set": count,
		"rapid":      true,
	}, nil
}

// handleDeviceTrigger triggers a device event.
func (r *Runner) handleDeviceTrigger(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	event, _ := params["event"].(string)
	state.Set("last_trigger", event)

	return map[string]any{
		"triggered":  true,
		"event_type": event,
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
		"device_configured": true,
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
		"exposed_device_configured": true,
	}, nil
}

// handleUpdateExposedAttribute updates an attribute on an exposed device.
func (r *Runner) handleUpdateExposedAttribute(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params["attribute"].(string)
	value := params["value"]

	if key != "" {
		ds.attributes[key] = value
	}

	return map[string]any{
		"attribute_updated": true,
	}, nil
}

// handleChangeState changes device operating/control/process state.
func (r *Runner) handleChangeState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	changed := false

	if s, ok := params["operating_state"].(string); ok {
		ds.operatingState = s
		state.Set("operating_state", s)
		changed = true
	}
	if s, ok := params["control_state"].(string); ok {
		ds.controlState = s
		state.Set("control_state", s)
		changed = true
	}
	if s, ok := params["process_state"].(string); ok {
		ds.processState = s
		state.Set("process_state", s)
		changed = true
	}

	return map[string]any{
		"state_changed":   changed,
		"operating_state": ds.operatingState,
		"control_state":   ds.controlState,
		"process_state":   ds.processState,
	}, nil
}

// handleSetStateDetail sets stateDetails vendor data.
func (r *Runner) handleSetStateDetail(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	key, _ := params["key"].(string)
	value := params["value"]

	if key != "" {
		ds.stateDetails[key] = value
	}

	return map[string]any{
		"detail_set": true,
	}, nil
}

// handleTriggerFault adds a fault to activeFaults.
func (r *Runner) handleTriggerFault(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	code := uint32(0)
	if c, ok := params["fault_code"].(float64); ok {
		code = uint32(c)
	}
	message, _ := params["fault_message"].(string)

	ds.faults = append(ds.faults, faultEntry{
		Code:    code,
		Message: message,
		Time:    time.Now(),
	})
	ds.operatingState = "FAULT"

	state.Set("active_fault_count", len(ds.faults))

	return map[string]any{
		"fault_triggered":    true,
		"fault_code":         code,
		"active_fault_count": len(ds.faults),
	}, nil
}

// handleClearFault removes a fault from activeFaults.
func (r *Runner) handleClearFault(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	code := uint32(0)
	if c, ok := params["fault_code"].(float64); ok {
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

	state.Set("active_fault_count", len(ds.faults))

	return map[string]any{
		"fault_cleared":      found,
		"active_fault_count": len(ds.faults),
	}, nil
}

// handleQueryDeviceState returns the current device state.
func (r *Runner) handleQueryDeviceState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)

	return map[string]any{
		"operating_state":  ds.operatingState,
		"control_state":    ds.controlState,
		"process_state":    ds.processState,
		"active_faults":    len(ds.faults),
		"ev_connected":     ds.evConnected,
		"cable_plugged_in": ds.cablePluggedIn,
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
		"state_matches":   allMatch,
		"operating_state": ds.operatingState,
		"control_state":   ds.controlState,
		"process_state":   ds.processState,
	}, nil
}

// handleSetConnected sets the connection state flag.
func (r *Runner) handleSetConnected(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "CONTROLLED"
	state.Set("device_connected", true)

	return map[string]any{"connected": true}, nil
}

// handleSetDisconnected sets the disconnection state.
func (r *Runner) handleSetDisconnected(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "FAILSAFE"
	state.Set("device_connected", false)

	return map[string]any{"disconnected": true}, nil
}

// handleSetFailsafeLimit sets the failsafe power limit.
func (r *Runner) handleSetFailsafeLimit(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDeviceState(state)

	limit := 0.0
	if l, ok := params["limit_watts"].(float64); ok {
		limit = l
	}

	ds.failsafeLimit = &limit
	state.Set("failsafe_limit", limit)

	return map[string]any{
		"failsafe_limit_set": true,
		"limit_watts":        limit,
	}, nil
}

// handleMakeProcessAvailable sets processState to AVAILABLE.
func (r *Runner) handleMakeProcessAvailable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.processState = "AVAILABLE"
	state.Set("process_state", "AVAILABLE")

	return map[string]any{"process_state": "AVAILABLE"}, nil
}

// handleStartOperation begins process execution.
func (r *Runner) handleStartOperation(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.processState = "RUNNING"
	ds.operatingState = "RUNNING"
	state.Set("process_state", "RUNNING")
	state.Set("operating_state", "RUNNING")

	return map[string]any{
		"process_state":  "RUNNING",
		"operation_started": true,
	}, nil
}

// handleEVConnect simulates EV connection.
func (r *Runner) handleEVConnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = true
	ds.cablePluggedIn = true
	state.Set("ev_connected", true)

	return map[string]any{"ev_connected": true}, nil
}

// handleEVDisconnect simulates EV disconnection.
func (r *Runner) handleEVDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = false
	ds.cablePluggedIn = false
	state.Set("ev_connected", false)

	return map[string]any{"ev_disconnected": true}, nil
}

// handleEVRequestsCharge simulates EV requesting charge.
func (r *Runner) handleEVRequestsCharge(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.evConnected = true
	state.Set("ev_charge_requested", true)

	return map[string]any{"charge_requested": true}, nil
}

// handlePlugInCable simulates plugging in the cable.
func (r *Runner) handlePlugInCable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.cablePluggedIn = true
	state.Set("cable_plugged_in", true)

	return map[string]any{"cable_plugged_in": true}, nil
}

// handleUnplugCable simulates unplugging the cable.
func (r *Runner) handleUnplugCable(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.cablePluggedIn = false
	ds.evConnected = false
	state.Set("cable_plugged_in", false)
	state.Set("ev_connected", false)

	return map[string]any{"cable_unplugged": true}, nil
}

// handleUserOverride simulates a user override command.
func (r *Runner) handleUserOverride(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.controlState = "OVERRIDE"
	state.Set("control_state", "OVERRIDE")

	return map[string]any{
		"override_active": true,
		"control_state":   "OVERRIDE",
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
		"factory_reset":   true,
		"operating_state": "STANDBY",
		"control_state":   "AUTONOMOUS",
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
		"power_cycled":    true,
		"operating_state": "STANDBY",
	}, nil
}

// handlePowerOnDevice simulates powering on a device.
func (r *Runner) handlePowerOnDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDeviceState(state)
	ds.operatingState = "STANDBY"

	return map[string]any{
		"powered_on":      true,
		"operating_state": "STANDBY",
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


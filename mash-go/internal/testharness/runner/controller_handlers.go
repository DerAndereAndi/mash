package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// registerControllerHandlers registers all controller action handlers.
func (r *Runner) registerControllerHandlers() {
	r.engine.RegisterHandler("controller_action", r.handleControllerAction)
	r.engine.RegisterHandler("commission_with_admin", r.handleCommissionWithAdmin)
	r.engine.RegisterHandler("get_controller_id", r.handleGetControllerID)
	r.engine.RegisterHandler("verify_controller_cert", r.handleVerifyControllerCert)
	r.engine.RegisterHandler("verify_controller_state", r.handleVerifyControllerState)
	r.engine.RegisterHandler("set_commissioning_window_duration", r.handleSetCommissioningWindowDuration)
	r.engine.RegisterHandler("get_commissioning_window_duration", r.handleGetCommissioningWindowDuration)
	r.engine.RegisterHandler("remove_device", r.handleRemoveDevice)
	r.engine.RegisterHandler("renew_cert", r.handleRenewCert)
	r.engine.RegisterHandler("check_renewal", r.handleCheckRenewal)
}

// handleControllerAction dispatches to controller sub-actions.
func (r *Runner) handleControllerAction(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	subAction, _ := params["sub_action"].(string)
	if subAction == "" {
		subAction, _ = params["action"].(string)
	}

	subStep := &loader.Step{Params: params}

	switch subAction {
	case "commission_with_admin":
		return r.handleCommissionWithAdmin(ctx, subStep, state)
	case "get_controller_id":
		return r.handleGetControllerID(ctx, subStep, state)
	case "verify_controller_cert":
		return r.handleVerifyControllerCert(ctx, subStep, state)
	case "verify_controller_state":
		return r.handleVerifyControllerState(ctx, subStep, state)
	case "set_commissioning_window_duration":
		return r.handleSetCommissioningWindowDuration(ctx, subStep, state)
	case "get_commissioning_window_duration":
		return r.handleGetCommissioningWindowDuration(ctx, subStep, state)
	case "remove_device":
		return r.handleRemoveDevice(ctx, subStep, state)
	case "renew_cert":
		return r.handleRenewCert(ctx, subStep, state)
	case "check_renewal":
		return r.handleCheckRenewal(ctx, subStep, state)

	// Zone management sub-actions.
	case "create_zone":
		return r.handleCreateZone(ctx, subStep, state)
	case "get_zone_ca_fingerprint":
		return r.handleGetZoneCAFingerprint(ctx, subStep, state)

	// Cert sub-actions.
	case "get_cert_fingerprint":
		return r.handleGetCertFingerprint(ctx, subStep, state)
	case "set_cert_expiry_days":
		return r.handleSetCertExpiryDays(ctx, subStep, state)
	default:
		return nil, fmt.Errorf("unknown controller_action sub_action: %s", subAction)
	}
}

// handleCommissionWithAdmin commissions using an admin token.
func (r *Runner) handleCommissionWithAdmin(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	token, _ := params["admin_token"].(string)
	deviceID, _ := params["device_id"].(string)
	zoneID, _ := params["zone_id"].(string)

	if token == "" {
		return nil, fmt.Errorf("admin_token required")
	}

	cs.devices[deviceID] = zoneID

	return map[string]any{
		"commissioned":  true,
		"device_id":     deviceID,
		"zone_id":       zoneID,
	}, nil
}

// handleGetControllerID returns the controller's ID.
func (r *Runner) handleGetControllerID(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	cs := getControllerState(state)

	id := cs.controllerID
	if id == "" {
		id = "controller-default"
	}

	return map[string]any{
		"controller_id": id,
	}, nil
}

// handleVerifyControllerCert verifies the controller cert validity.
func (r *Runner) handleVerifyControllerCert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check if TLS connection has peer certificates.
	if r.conn != nil && r.conn.connected && r.conn.tlsConn != nil {
		tlsState := r.conn.tlsConn.ConnectionState()
		hasCerts := len(tlsState.PeerCertificates) > 0

		return map[string]any{
			"cert_valid": hasCerts,
			"cert_count": len(tlsState.PeerCertificates),
		}, nil
	}

	return map[string]any{
		"cert_valid": false,
	}, nil
}

// handleVerifyControllerState verifies controller state.
func (r *Runner) handleVerifyControllerState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	allMatch := true
	if expected, ok := params["device_count"].(float64); ok {
		if len(cs.devices) != int(expected) {
			allMatch = false
		}
	}

	return map[string]any{
		"state_valid":  allMatch,
		"device_count": len(cs.devices),
	}, nil
}

// handleSetCommissioningWindowDuration sets the commissioning window duration.
func (r *Runner) handleSetCommissioningWindowDuration(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	minutes := 15.0
	if m, ok := params["minutes"].(float64); ok {
		minutes = m
	}

	cs.commissioningWindowDuration = time.Duration(minutes) * time.Minute

	return map[string]any{
		"duration_set": true,
		"minutes":      minutes,
	}, nil
}

// handleGetCommissioningWindowDuration returns the commissioning window duration.
func (r *Runner) handleGetCommissioningWindowDuration(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	cs := getControllerState(state)

	return map[string]any{
		"minutes": cs.commissioningWindowDuration.Minutes(),
	}, nil
}

// handleRemoveDevice removes a device from a zone.
func (r *Runner) handleRemoveDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	deviceID, _ := params["device_id"].(string)

	_, existed := cs.devices[deviceID]
	delete(cs.devices, deviceID)

	return map[string]any{
		"device_removed": existed,
	}, nil
}

// handleRenewCert triggers certificate renewal.
func (r *Runner) handleRenewCert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Delegate to existing full_renewal_flow handler.
	return r.handleFullRenewalFlow(ctx, step, state)
}

// handleCheckRenewal checks renewal status.
func (r *Runner) handleCheckRenewal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check if renewal was completed.
	renewalComplete, _ := state.Get("renewal_complete")
	status, _ := state.Get("status")

	return map[string]any{
		"renewal_complete": renewalComplete,
		"status":           status,
	}, nil
}

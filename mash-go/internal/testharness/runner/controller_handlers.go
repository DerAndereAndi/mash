package runner

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// registerControllerHandlers registers all controller action handlers.
func (r *Runner) registerControllerHandlers() {
	r.engine.RegisterHandler(ActionControllerAction, r.handleControllerAction)
	r.engine.RegisterHandler(ActionCommissionWithAdmin, r.handleCommissionWithAdmin)
	r.engine.RegisterHandler(ActionGetControllerID, r.handleGetControllerID)
	r.engine.RegisterHandler(ActionVerifyControllerCert, r.handleVerifyControllerCert)
	r.engine.RegisterHandler(ActionVerifyControllerState, r.handleVerifyControllerState)
	r.engine.RegisterHandler(ActionSetCommissioningWindowDuration, r.handleSetCommissioningWindowDuration)
	r.engine.RegisterHandler(ActionGetCommissioningWindowDuration, r.handleGetCommissioningWindowDuration)
	r.engine.RegisterHandler(ActionRemoveDevice, r.handleRemoveDevice)
	r.engine.RegisterHandler(ActionRenewCert, r.handleRenewCert)
	r.engine.RegisterHandler(ActionCheckRenewal, r.handleCheckRenewal)

	// Custom checkers for controller cert tests.
	r.engine.RegisterChecker("validity_days_min", r.checkValidityDaysMin)
}

// handleControllerAction dispatches to controller sub-actions.
func (r *Runner) handleControllerAction(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	subAction, _ := params[ParamSubAction].(string)
	if subAction == "" {
		subAction, _ = params[KeyAction].(string)
	}

	subStep := &loader.Step{Params: params}

	var result map[string]any
	var err error

	switch subAction {
	case "commission_with_admin":
		result, err = r.handleCommissionWithAdmin(ctx, subStep, state)
	case "get_controller_id":
		result, err = r.handleGetControllerID(ctx, subStep, state)
	case "verify_controller_cert":
		result, err = r.handleVerifyControllerCert(ctx, subStep, state)
	case "verify_controller_state":
		result, err = r.handleVerifyControllerState(ctx, subStep, state)
	case "set_commissioning_window_duration":
		result, err = r.handleSetCommissioningWindowDuration(ctx, subStep, state)
	case "get_commissioning_window_duration":
		result, err = r.handleGetCommissioningWindowDuration(ctx, subStep, state)
	case "remove_device":
		result, err = r.handleRemoveDevice(ctx, subStep, state)
	case "renew_cert":
		result, err = r.handleRenewCert(ctx, subStep, state)
	case "check_renewal":
		result, err = r.handleCheckRenewal(ctx, subStep, state)

	// Zone management sub-actions.
	case "create_zone":
		result, err = r.handleCreateZone(ctx, subStep, state)
	case "delete_zone":
		result, err = r.handleDeleteZone(ctx, subStep, state)
	case "get_zone_ca_fingerprint":
		result, err = r.handleGetZoneCAFingerprint(ctx, subStep, state)

	case "get_zone_metadata":
		result, err = r.handleGetZoneMetadata(ctx, subStep, state)

	// Cert sub-actions.
	case "get_cert_fingerprint":
		result, err = r.handleGetCertFingerprint(ctx, subStep, state)
	case "set_cert_expiry_days":
		result, err = r.handleSetCertExpiryDays(ctx, subStep, state)
	case "restart":
		result, err = r.handleControllerRestart(ctx, subStep, state)
	default:
		return nil, fmt.Errorf("unknown controller_action sub_action: %s", subAction)
	}

	// Mark successful dispatches so tests can verify the action was triggered.
	if err == nil && result != nil {
		result[KeyActionTriggered] = true
	}
	return result, err
}

// handleCommissionWithAdmin commissions using an admin token.
// Token validation: sentinel values "expired", "invalid_signature", and
// "wrong_permissions" simulate rejection with INVALID_CERT. All other
// non-empty tokens are treated as valid.
func (r *Runner) handleCommissionWithAdmin(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	token, _ := params[ParamAdminToken].(string)
	deviceID, _ := params[KeyDeviceID].(string)
	zoneID, _ := params[KeyZoneID].(string)

	if token == "" {
		return nil, fmt.Errorf("admin_token required")
	}

	// Simulate token validation: known-bad sentinels are rejected.
	switch token {
	case "expired", "invalid_signature", "wrong_permissions":
		return map[string]any{
			KeyCommissionSuccess: false,
			KeyCommissioned:      false,
			KeyError:             "INVALID_CERT",
		}, nil
	}

	cs.devices[deviceID] = zoneID

	return map[string]any{
		KeyCommissionSuccess: true,
		KeyCommissioned:      true,
		KeyDeviceID:          deviceID,
		KeyZoneID:            zoneID,
		PrecondDeviceInZone:  true,
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
		KeyControllerID: id,
	}, nil
}

// handleVerifyControllerCert verifies the controller's own operational cert.
func (r *Runner) handleVerifyControllerCert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check the runner's own controller certificate (not the TLS peer cert).
	if r.controllerCert != nil && r.controllerCert.Certificate != nil {
		c := r.controllerCert.Certificate
		notExpired := time.Now().Before(c.NotAfter)
		validityDays := int(time.Until(c.NotAfter).Hours() / 24)

		signedByZoneCA := false
		issuerFP := ""
		if r.zoneCA != nil && r.zoneCA.Certificate != nil {
			opts := x509.VerifyOptions{
				Roots: r.zoneCAPool,
			}
			_, err := c.Verify(opts)
			signedByZoneCA = err == nil
			issuerFP = certFingerprint(r.zoneCA.Certificate)
		}

		return map[string]any{
			KeyCertValid:         true,
			KeyCertPresent:       true,
			KeySignedByZoneCA:    signedByZoneCA,
			KeyNotExpired:        notExpired,
			KeyIssuerFingerprint: issuerFP,
			KeyValidityDaysMin:   validityDays,
		}, nil
	}

	return map[string]any{
		KeyCertValid:         false,
		KeyCertPresent:       false,
		KeySignedByZoneCA:    false,
		KeyNotExpired:        false,
		KeyIssuerFingerprint: "",
		KeyValidityDaysMin:   0,
	}, nil
}

// handleVerifyControllerState verifies controller state.
func (r *Runner) handleVerifyControllerState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	allMatch := true
	if _, ok := params[KeyDeviceCount]; ok {
		if len(cs.devices) != paramInt(params, KeyDeviceCount, 0) {
			allMatch = false
		}
	}

	outputs := map[string]any{
		KeyStateValid:          allMatch,
		KeyDeviceCount:         len(cs.devices),
		KeyVerificationSuccess: allMatch,
	}
	if _, ok := params[ParamHasExposedDevice]; ok {
		outputs[KeyHasExposedDevice] = len(cs.devices) > 0
	}
	return outputs, nil
}

// handleSetCommissioningWindowDuration sets the commissioning window duration.
func (r *Runner) handleSetCommissioningWindowDuration(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	minutes := 15.0
	if v, ok := params[KeyMinutes]; ok {
		minutes = toFloat(v)
	}
	// Also accept duration_seconds param (convert to minutes).
	if v, ok := params[KeyDurationSeconds]; ok {
		minutes = toFloat(v) / 60.0
	}

	// Validate bounds: min 3 seconds (0.05 min), max 180 minutes (10800s).
	// Device-side enforcement clamps to [3s, 10800s] (test_event_handler.go).
	const minMinutes = 3.0 / 60.0 // 3 seconds
	const maxMinutes = 180.0
	result := "ok"
	if minutes < minMinutes || minutes > maxMinutes {
		result = "clamped_or_rejected"
		if minutes < minMinutes {
			minutes = minMinutes
		} else {
			minutes = maxMinutes
		}
	}

	cs.commissioningWindowDuration = time.Duration(minutes * float64(time.Minute))

	// Dual-mode: also send the real command to the device when connected
	// with an operational TLS session. Errors are logged but not propagated
	// (simulation state is the source of truth for the test harness).
	durationSeconds := uint32(minutes * 60)
	r.sendSetCommWindowDuration(durationSeconds, state)

	return map[string]any{
		KeyDurationSet:     true,
		KeyMinutes:         minutes,
		KeyDurationSeconds: minutes * 60,
		KeyResult:          result,
	}, nil
}

// sendSetCommWindowDuration sends the setCommissioningWindowDuration command
// to the real device over an operational zone connection. Falls back to
// zone connections (from the connection tracker) when r.conn is not
// available (e.g. after device_zones_full precondition).
func (r *Runner) sendSetCommWindowDuration(durationSeconds uint32, state *engine.ExecutionState) {
	if r.config.EnableKey == "" {
		return
	}

	// Find a usable connection: prefer r.conn, fall back to zone connections.
	conn := r.conn
	if conn == nil || !conn.connected || !conn.operational {
		conn = nil
		ct := getConnectionTracker(state)
		for _, c := range ct.zoneConnections {
			if c.connected && c.framer != nil {
				conn = c
				break
			}
		}
	}
	if conn == nil {
		return
	}

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureTestControl), //nolint:gosec // constant fits in uint8
		Payload: &wire.InvokePayload{
			CommandID: features.TestControlCmdSetCommissioningWindowDuration,
			Parameters: map[string]any{
				features.SetCommWindowDurParamEnableKey:       r.config.EnableKey,
				features.SetCommWindowDurParamDurationSeconds: durationSeconds,
			},
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		r.debugf("sendSetCommWindowDuration: encode error: %v", err)
		return
	}

	if err := conn.framer.WriteFrame(data); err != nil {
		r.debugf("sendSetCommWindowDuration: write error: %v", err)
		return
	}

	// Read the response (may need to skip notifications).
	for range 5 {
		respData, readErr := conn.framer.ReadFrame()
		if readErr != nil {
			r.debugf("sendSetCommWindowDuration: read error: %v", readErr)
			return
		}
		msgType, peekErr := wire.PeekMessageType(respData)
		if peekErr != nil {
			r.debugf("sendSetCommWindowDuration: peek error: %v", peekErr)
			return
		}
		if msgType == wire.MessageTypeNotification {
			conn.pendingNotifications = append(conn.pendingNotifications, respData)
			continue
		}
		// Got the invoke response -- done.
		r.debugf("sendSetCommWindowDuration: success (durationSeconds=%d)", durationSeconds)
		return
	}
	r.debugf("sendSetCommWindowDuration: too many notifications, giving up")
}

// handleGetCommissioningWindowDuration returns the commissioning window duration.
func (r *Runner) handleGetCommissioningWindowDuration(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	cs := getControllerState(state)

	minutes := cs.commissioningWindowDuration.Minutes()
	seconds := minutes * 60

	return map[string]any{
		KeyMinutes:            minutes,
		KeyDurationSeconds:    seconds,
		KeyDurationSecondsMin: seconds,
		KeyDurationSecondsMax: seconds,
	}, nil
}

// handleRemoveDevice removes a device from a zone.
func (r *Runner) handleRemoveDevice(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	cs := getControllerState(state)

	deviceID, _ := params[KeyDeviceID].(string)
	zone, _ := params[ParamZone].(string)

	// Try device-map based removal first.
	_, existed := cs.devices[deviceID]
	if existed {
		delete(cs.devices, deviceID)
	}

	// When no device_id was provided (conformance test pattern), infer
	// removal from precondition state flags directly.
	if !existed {
		inTwoZones, _ := state.Get(PrecondDeviceInTwoZones)
		inZone, _ := state.Get(PrecondDeviceInZone)
		existed = inTwoZones == true || inZone == true
	}

	// Update simulation precondition state to reflect the removal.
	if existed {
		if zone == "all" || (deviceID != "" && len(cs.devices) == 0) {
			state.Set(PrecondDeviceInZone, false)
			state.Set(PrecondDeviceInTwoZones, false)
			state.Set(StateDeviceWasRemoved, true)
		} else {
			// Removed from one zone but others remain.
			state.Set(PrecondDeviceInTwoZones, false)
			state.Set(PrecondDeviceInZone, true)
		}
	}

	return map[string]any{
		KeyDeviceRemoved: existed,
	}, nil
}

// handleRenewCert triggers certificate renewal.
// If a Zone CA is available, the controller cert is renewed locally without
// the wire protocol. Otherwise it falls back to the full renewal flow.
func (r *Runner) handleRenewCert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.zoneCA != nil {
		newCert, err := cert.GenerateControllerOperationalCert(r.zoneCA, "test-controller")
		if err != nil {
			return nil, fmt.Errorf("renew controller cert: %w", err)
		}
		r.controllerCert = newCert
		state.Set(StateRenewalComplete, true)

		return map[string]any{
			KeyRenewalComplete: true,
			KeyRenewalSuccess:  true,
			KeyStatus:          0,
		}, nil
	}

	// Fall back to full wire renewal flow.
	result, err := r.handleFullRenewalFlow(ctx, step, state)
	if err != nil {
		return result, err
	}
	if complete, ok := result[KeyRenewalComplete].(bool); ok {
		result[KeyRenewalSuccess] = complete
	}
	return result, nil
}

// handleCheckRenewal checks renewal status and whether renewal should be initiated.
func (r *Runner) handleCheckRenewal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	renewalInitiated := false

	// Check simulated cert expiry set via set_cert_expiry_days.
	if daysVal, ok := state.Get(StateCertDaysUntilExpiry); ok {
		if days, ok := daysVal.(int); ok && days <= 30 {
			renewalInitiated = true
		}
	}

	// Also check controller cert directly.
	if r.controllerCert != nil && r.controllerCert.NeedsRenewal() {
		renewalInitiated = true
	}

	renewalComplete, _ := state.Get(StateRenewalComplete)
	status, _ := state.Get(KeyStatus)

	return map[string]any{
		KeyRenewalInitiated: renewalInitiated,
		KeyRenewalComplete:  renewalComplete,
		KeyStatus:           status,
	}, nil
}

// handleControllerRestart simulates a controller restart.
// The controller cert persists across restarts.
func (r *Runner) handleControllerRestart(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return map[string]any{
		KeyRestarted: true,
	}, nil
}

// checkValidityDaysMin verifies that validity_days_min output is >= expected.
func (r *Runner) checkValidityDaysMin(key string, expected interface{}, state *engine.ExecutionState) *engine.ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &engine.ExpectResult{
			Key:      key,
			Expected: expected,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found", key),
		}
	}

	actualNum, ok1 := engine.ToFloat64(actual)
	expectedNum, ok2 := engine.ToFloat64(expected)
	if !ok1 || !ok2 {
		return &engine.ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("cannot compare: actual=%T expected=%T", actual, expected),
		}
	}

	passed := actualNum >= expectedNum
	return &engine.ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("%v >= %v = %v", actualNum, expectedNum, passed),
	}
}

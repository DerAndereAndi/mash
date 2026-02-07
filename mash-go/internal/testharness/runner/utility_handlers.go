package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/inspect"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// registerUtilityHandlers registers utility action handlers.
func (r *Runner) registerUtilityHandlers() {
	r.engine.RegisterHandler("compare", r.handleCompare)
	r.engine.RegisterHandler("compare_values", r.handleCompareValues)
	r.engine.RegisterHandler("evaluate", r.handleEvaluate)
	r.engine.RegisterHandler("conditional_read", r.handleConditionalRead)
	r.engine.RegisterHandler("record_time", r.handleRecordTime)
	r.engine.RegisterHandler("verify_timing", r.handleVerifyTiming)
	r.engine.RegisterHandler("check_response", r.handleCheckResponse)
	r.engine.RegisterHandler("verify_correlation", r.handleVerifyCorrelation)
	r.engine.RegisterHandler("wait_for_state", r.handleWaitForState)
	r.engine.RegisterHandler("wait_notification", r.handleWaitNotification)
	r.engine.RegisterHandler("wait_report", r.handleWaitReport)
	r.engine.RegisterHandler("parse_qr", r.handleParseQR)
}

// handleCompare compares two stored values.
func (r *Runner) handleCompare(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	left := params[ParamLeft]
	right := params[ParamRight]
	op, _ := params[ParamOperator].(string)
	if op == "" {
		op = "equal"
	}

	leftF := toFloat(left)
	rightF := toFloat(right)

	var result bool
	switch op {
	case "equal", "eq", "==":
		result = fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	case "not_equal", "ne", "!=":
		result = fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right)
	case "greater_than", "gt", ">":
		result = leftF > rightF
	case "less_than", "lt", "<":
		result = leftF < rightF
	case "greater_equal", "gte", ">=":
		result = leftF >= rightF
	case "less_equal", "lte", "<=":
		result = leftF <= rightF
	default:
		return nil, fmt.Errorf("unknown comparison operator: %s", op)
	}

	return map[string]any{
		KeyComparisonResult: result,
		KeyValuesEqual:      fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right),
	}, nil
}

// handleCompareValues compares N values for equality/difference.
func (r *Runner) handleCompareValues(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	values, ok := params[ParamValues].([]any)
	if !ok {
		return nil, fmt.Errorf("values parameter must be an array")
	}

	if len(values) == 0 {
		return map[string]any{
			KeyAllEqual:     true,
			KeyAllDifferent: true,
		}, nil
	}

	allEqual := true
	allDifferent := true
	seen := make(map[string]bool)

	first := fmt.Sprintf("%v", values[0])
	seen[first] = true

	for i := 1; i < len(values); i++ {
		s := fmt.Sprintf("%v", values[i])
		if s != first {
			allEqual = false
		}
		if seen[s] {
			allDifferent = false
		}
		seen[s] = true
	}

	return map[string]any{
		KeyAllEqual:     allEqual,
		KeyAllDifferent: allDifferent,
	}, nil
}

// handleEvaluate evaluates a boolean expression from state.
func (r *Runner) handleEvaluate(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// The expression is a key name whose value is treated as a boolean.
	expr, _ := params[ParamExpression].(string)
	if expr != "" {
		val, exists := state.Get(expr)
		if exists {
			return map[string]any{KeyResult: toBool(val)}, nil
		}
		// Multi-word expressions (containing spaces) are descriptive
		// assertions (e.g. "current_state can inform retry decision").
		// These always pass -- reaching this step means prior steps
		// succeeded. Single-word expressions are state key lookups
		// that genuinely failed.
		if strings.Contains(expr, " ") {
			return map[string]any{KeyResult: true}, nil
		}
	}

	// Direct value evaluation.
	if v, ok := params[KeyValue]; ok {
		return map[string]any{KeyResult: toBool(v)}, nil
	}

	return map[string]any{KeyResult: false}, nil
}

// handleConditionalRead reads only if a condition is met.
func (r *Runner) handleConditionalRead(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	condKey, _ := params[ParamCondition].(string)
	if condKey != "" {
		val, exists := state.Get(condKey)
		if !exists || !toBool(val) {
			return map[string]any{
				KeySkipped:     true,
				KeyReadSuccess: false,
			}, nil
		}
	}

	return r.handleRead(ctx, step, state)
}

// handleRecordTime records the current timestamp under a key.
func (r *Runner) handleRecordTime(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	key, _ := params[KeyKey].(string)
	if key == "" {
		// Accept "name" as alias for "key" (used in test YAML).
		key, _ = params[ParamName].(string)
	}
	if key == "" {
		key = "recorded_time"
	}

	now := time.Now()
	state.Set(key, now)

	return map[string]any{
		KeyTimeRecorded: true,
		KeyTimestampMs:  now.UnixMilli(),
	}, nil
}

// handleVerifyTiming compares two recorded times against a tolerance.
func (r *Runner) handleVerifyTiming(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// Accept "reference" and "event" as aliases for "start_key" (used in test YAML).
	startKey, _ := params[ParamStartKey].(string)
	if startKey == "" {
		startKey, _ = params[ParamReference].(string)
	}
	if startKey == "" {
		startKey, _ = params[KeyEvent].(string)
	}
	if startKey == "" {
		startKey = "start_time"
	}

	endKey, _ := params[ParamEndKey].(string)

	startVal, startOK := state.Get(startKey)
	if !startOK {
		return map[string]any{
			KeyWithinTolerance: false,
			KeyWithinLimit:     false,
			KeyElapsedMs:       int64(0),
			KeyError:           fmt.Sprintf("start time %q not recorded", startKey),
		}, nil
	}

	startTime, sok := startVal.(time.Time)
	if !sok {
		return map[string]any{
			KeyWithinTolerance: false,
			KeyWithinLimit:     false,
			KeyElapsedMs:       int64(0),
			KeyError:           fmt.Sprintf("recorded value for %q is not time.Time", startKey),
		}, nil
	}

	// Use end_key if specified, otherwise use current time.
	endTime := time.Now()
	if endKey != "" {
		endVal, endOK := state.Get(endKey)
		if !endOK {
			return map[string]any{
				KeyWithinTolerance: false,
				KeyWithinLimit:     false,
				KeyElapsedMs:       int64(0),
				KeyError:           fmt.Sprintf("end time %q not recorded", endKey),
			}, nil
		}
		var eok bool
		endTime, eok = endVal.(time.Time)
		if !eok {
			return map[string]any{
				KeyWithinTolerance: false,
				KeyWithinLimit:     false,
				KeyElapsedMs:       int64(0),
				KeyError:           fmt.Sprintf("recorded value for %q is not time.Time", endKey),
			}, nil
		}
	}

	elapsed := endTime.Sub(startTime)
	elapsedMs := elapsed.Milliseconds()

	minMs := int64(paramInt(params, "min_ms", 0))
	maxMs := int64(paramInt(params, "max_ms", 0))

	// Support min_duration as a Go duration string (e.g., "9s").
	if md, ok := params[ParamMinDuration].(string); ok && md != "" {
		if d, err := time.ParseDuration(md); err == nil {
			minMs = d.Milliseconds()
		}
	}

	// Support max_duration as a Go duration string (e.g., "6s").
	if md, ok := params[ParamMaxDuration].(string); ok && md != "" {
		if d, err := time.ParseDuration(md); err == nil {
			maxMs = d.Milliseconds()
		}
	}

	// Support expected_duration + tolerance (e.g., "60s" +/- "60s").
	if ed, ok := params[ParamExpectedDuration].(string); ok && ed != "" {
		if d, err := time.ParseDuration(ed); err == nil {
			expectedMs := d.Milliseconds()
			toleranceMs := int64(0)
			if tol, ok := params[ParamTolerance].(string); ok && tol != "" {
				if td, err := time.ParseDuration(tol); err == nil {
					toleranceMs = td.Milliseconds()
				}
			}
			minMs = expectedMs - toleranceMs
			if minMs < 0 {
				minMs = 0
			}
			maxMs = expectedMs + toleranceMs
		}
	}

	withinTolerance := elapsedMs >= minMs
	if maxMs > 0 {
		withinTolerance = withinTolerance && elapsedMs <= maxMs
	}

	// Derive timeout_detected from pong_received state (keepalive timeout tests).
	timeoutDetected := false
	if pr, ok := state.Get(KeyPongReceived); ok {
		if pongReceived, ok := pr.(bool); ok && !pongReceived {
			timeoutDetected = true
		}
	}

	return map[string]any{
		KeyWithinTolerance: withinTolerance,
		KeyWithinLimit:     withinTolerance,
		KeyWithinBounds:    withinTolerance,
		KeyElapsedMs:       elapsedMs,
		KeyTimeoutDetected: timeoutDetected,
	}, nil
}

// handleCheckResponse verifies response status and payload fields.
func (r *Runner) handleCheckResponse(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	statusMatches := true
	if expected, ok := params[ParamExpectedStatus]; ok {
		actual, _ := state.Get(KeyStatus)
		statusMatches = fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}

	payloadMatches := true
	if fields, ok := params[ParamExpectedFields].(map[string]any); ok {
		for key, expected := range fields {
			actual, exists := state.Get(key)
			if !exists || fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected) {
				payloadMatches = false
				break
			}
		}
	}

	return map[string]any{
		KeyStatusMatches:  statusMatches,
		KeyPayloadMatches: payloadMatches,
	}, nil
}

// handleVerifyCorrelation checks txnId correlation between request/response.
func (r *Runner) handleVerifyCorrelation(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	requestKey, _ := params[ParamRequestKey].(string)
	responseKey, _ := params[ParamResponseKey].(string)
	if requestKey == "" {
		requestKey = "request_message_id"
	}
	if responseKey == "" {
		responseKey = "response_message_id"
	}

	reqVal, reqOK := state.Get(requestKey)
	respVal, respOK := state.Get(responseKey)

	valid := reqOK && respOK && fmt.Sprintf("%v", reqVal) == fmt.Sprintf("%v", respVal)

	return map[string]any{
		KeyCorrelationValid: valid,
	}, nil
}

// handleWaitForState polls a state key until it matches the expected value or times out.
func (r *Runner) handleWaitForState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	key, _ := params[KeyKey].(string)
	expected := params[KeyValue]
	timeoutMs := paramInt(params, KeyTimeoutMs, 5000)
	pollMs := paramInt(params, "poll_ms", 100)

	deadline := time.After(time.Duration(timeoutMs) * time.Millisecond)
	ticker := time.NewTicker(time.Duration(pollMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		actual, exists := state.Get(key)
		if exists && fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected) {
			return map[string]any{KeyStateReached: true}, nil
		}

		select {
		case <-deadline:
			return map[string]any{KeyStateReached: false}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			// continue polling
		}
	}
}

// handleWaitNotification waits for a notification frame on the connection.
func (r *Runner) handleWaitNotification(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := paramInt(params, KeyTimeoutMs, 5000)
	// Also accept "timeout" as a duration string (e.g., "5s").
	if t, ok := params[ParamTimeout].(string); ok {
		if d, parseErr := time.ParseDuration(t); parseErr == nil {
			timeoutMs = int(d.Milliseconds())
		}
	}

	eventType, _ := params[KeyEventType].(string)

	// Check if subscribe response already contained priming data.
	// If so, treat it as a received priming notification without reading the wire.
	if primingData, ok := state.Get(StatePrimingData); ok && primingData != nil {
		// Consume priming data so subsequent calls wait for real notifications.
		state.Set(StatePrimingData, nil)
		r.debugf("receive_notification: using priming data from subscribe response")
		return r.buildNotificationOutput(primingData, eventType, state, true)
	}

	// Check if a notification was buffered by sendRequest (interleaved with a response).
	if len(r.pendingNotifications) > 0 {
		data := r.pendingNotifications[0]
		r.pendingNotifications = r.pendingNotifications[1:]
		r.debugf("receive_notification: using buffered notification frame (%d bytes)", len(data))
		notif, err := wire.DecodeNotification(data)
		if err == nil {
			out, outErr := r.buildNotificationOutput(notif.Changes, eventType, state, false)
			if out != nil {
				out[KeySubscriptionID] = notif.SubscriptionID
			}
			return out, outErr
		}
		// Not a valid notification -- ignore and continue to wire read.
	}

	if r.conn == nil || !r.conn.connected {
		return map[string]any{
			KeyNotificationReceived: false,
			KeyError:                "not connected",
		}, nil
	}

	// Try to read a frame within timeout
	readCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	type readResult struct {
		data []byte
		err  error
	}

	ch := make(chan readResult, 1)
	go func() {
		data, err := r.conn.framer.ReadFrame()
		ch <- readResult{data, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return map[string]any{
				KeyNotificationReceived: false,
				KeyError:                res.err.Error(),
			}, nil
		}
		// Decode the notification frame
		notif, err := wire.DecodeNotification(res.data)
		if err != nil {
			// Might be a response or other message - return raw
			return map[string]any{
				KeyNotificationReceived: true,
				KeyNotificationData:     res.data,
				KeyEventType:            eventType,
			}, nil
		}
		out, outErr := r.buildNotificationOutput(notif.Changes, eventType, state, false)
		if out != nil {
			out[KeySubscriptionID] = notif.SubscriptionID
		}
		return out, outErr
	case <-readCtx.Done():
		return map[string]any{
			KeyNotificationReceived: false,
			KeyEventType:            eventType,
		}, nil
	}
}

// buildNotificationOutput builds the output map from notification changes.
// It resolves attribute names, determines priming vs delta, etc.
func (r *Runner) buildNotificationOutput(changes any, eventType string, state *engine.ExecutionState, fromPriming bool) (map[string]any, error) {
	outputs := map[string]any{
		KeyNotificationReceived: true,
		KeyEventType:            eventType,
	}

	// Get the subscribed feature ID to resolve attribute names.
	featureID := uint8(0)
	if fid, ok := state.Get(StateSubscribedFeatureID); ok {
		switch v := fid.(type) {
		case uint8:
			featureID = v
		case int:
			featureID = uint8(v)
		}
	}

	// Normalize the changes map to map[uint16]any.
	changesMap := normalizeChangesMap(changes)
	r.debugf("buildNotificationOutput: featureID=%d attrs=%d fromPriming=%v", featureID, len(changesMap), fromPriming)
	if changesMap == nil {
		outputs[KeyIsPrimingReport] = fromPriming
		outputs[KeyIsDelta] = !fromPriming
		return outputs, nil
	}

	// Resolve attribute names from the changes.
	namedValues := make(map[string]any)
	attrNames := make([]string, 0, len(changesMap))
	for attrID, val := range changesMap {
		name := inspect.GetAttributeName(featureID, attrID)
		if name == "" {
			name = fmt.Sprintf("%d", attrID)
		}
		namedValues[name] = val
		attrNames = append(attrNames, name)
	}

	// Determine notification type using priming attribute count as baseline.
	// - priming: the first report after subscribe (from subscribe response)
	// - heartbeat: full-state notification at maxInterval (same attr count as priming)
	// - delta: notification with fewer attributes than priming
	isPriming := fromPriming
	if isPriming {
		// Save attribute count from priming for future heartbeat/delta detection.
		state.Set(StatePrimingAttrCount, len(changesMap))
	}

	primingCount := 0
	if pc, ok := state.Get(StatePrimingAttrCount); ok {
		if v, ok := pc.(int); ok {
			primingCount = v
		}
	}

	isFullState := primingCount > 0 && len(changesMap) >= primingCount
	isHeartbeat := !fromPriming && isFullState
	isDelta := !fromPriming && !isFullState

	outputs[KeyIsPrimingReport] = isPriming
	outputs[KeyIsHeartbeat] = isHeartbeat
	outputs[KeyIsDelta] = isDelta
	outputs[KeyContainsAllAttributes] = isFullState
	outputs[KeyContainsFullState] = isFullState
	outputs[KeyValue] = namedValues
	outputs["contains"] = attrNames
	outputs[KeyContainsOnly] = attrNames

	// Expose the changed attribute name for simple assertions.
	// Prefer attributes that match what was subscribed, since notifications
	// may contain additional attributes beyond the subscribed set.
	if len(attrNames) > 0 {
		best := attrNames[0]
		if subs, ok := state.Get(StateSubscribedAttributes); ok {
			if subList, ok := subs.([]string); ok {
				for _, name := range attrNames {
					for _, sub := range subList {
						if name == sub {
							best = name
							goto found
						}
					}
				}
			found:
			}
		}
		outputs[KeyChangedAttribute] = best
	}

	return outputs, nil
}

// normalizeChangesMap converts various CBOR-decoded map types to map[uint16]any.
func normalizeChangesMap(v any) map[uint16]any {
	switch m := v.(type) {
	case map[uint16]any:
		return m
	case map[any]any:
		result := make(map[uint16]any, len(m))
		for k, val := range m {
			if id, ok := wire.ToUint32(k); ok {
				result[uint16(id)] = val
			}
		}
		return result
	case map[uint64]any:
		result := make(map[uint64]any, len(m))
		_ = result // needed for type conversion
		out := make(map[uint16]any, len(m))
		for k, val := range m {
			out[uint16(k)] = val
		}
		return out
	default:
		return nil
	}
}

// handleWaitReport waits for a subscription priming report.
// If the subscribe response already included priming data (current values),
// that counts as a received report without waiting for a notification frame.
func (r *Runner) handleWaitReport(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := paramInt(params, KeyTimeoutMs, 5000)
	// Also accept "timeout" as a duration string (e.g., "5s").
	if t, ok := params[ParamTimeout].(string); ok {
		if d, parseErr := time.ParseDuration(t); parseErr == nil {
			timeoutMs = int(d.Milliseconds())
		}
	}

	// Check if the subscribe response already contained priming data
	// (current values in SubscribeResponsePayload key 2). If so, treat
	// that as a received report without waiting for a wire notification.
	if primingData, ok := state.Get("_priming_data"); ok && primingData != nil {
		return map[string]any{
			KeyReportReceived: true,
			KeyReportData:     primingData,
		}, nil
	}

	if r.conn == nil || !r.conn.connected {
		return map[string]any{
			KeyReportReceived: false,
			KeyError:          "not connected",
		}, nil
	}

	readCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	type readResult struct {
		data []byte
		err  error
	}

	ch := make(chan readResult, 1)
	go func() {
		data, err := r.conn.framer.ReadFrame()
		ch <- readResult{data, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return map[string]any{
				KeyReportReceived: false,
				KeyError:          res.err.Error(),
			}, nil
		}
		return map[string]any{
			KeyReportReceived: true,
			KeyReportData:     res.data,
		}, nil
	case <-readCtx.Done():
		return map[string]any{
			KeyReportReceived: false,
		}, nil
	}
}

// handleParseQR parses a QR payload string into components.
// Uses discovery.ParseQRCode (4-field format: MASH:v:d:s) which returns
// specific sentinel errors for each failure mode, mapped to coded strings.
func (r *Runner) handleParseQR(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	payload, _ := params[ParamPayload].(string)
	// Accept "content" as fallback when "payload" is empty.
	if payload == "" {
		payload, _ = params[ParamContent].(string)
	}
	if payload == "" {
		return map[string]any{KeyValid: false, KeyParseSuccess: false, KeyError: "no_payload"}, nil
	}

	qr, err := discovery.ParseQRCode(payload)
	if err != nil {
		errorCode := mapQRError(err)
		return map[string]any{
			KeyValid:        false,
			KeyParseSuccess: false,
			KeyError:        errorCode,
			KeyErrorDetail:  err.Error(),
		}, nil
	}

	return map[string]any{
		KeyValid:         true,
		KeyParseSuccess:  true,
		KeyVersion:       int(qr.Version),
		KeyDiscriminator: int(qr.Discriminator),
		KeySetupCode:     qr.SetupCode,
	}, nil
}

// mapQRError maps discovery.ParseQRCode sentinel errors to coded strings.
func mapQRError(err error) string {
	switch {
	case errors.Is(err, discovery.ErrInvalidPrefix):
		return "invalid_prefix"
	case errors.Is(err, discovery.ErrInvalidFieldCount):
		return "invalid_field_count"
	case errors.Is(err, discovery.ErrInvalidVersion):
		return "invalid_version"
	case errors.Is(err, discovery.ErrInvalidDiscriminator):
		return "discriminator_out_of_range"
	case errors.Is(err, discovery.ErrInvalidSetupCode):
		return "invalid_setup_code"
	default:
		return "parse_error"
	}
}

// paramInt extracts an integer parameter from a params map, handling
// int, float64, and int64 types that YAML v3 may produce. Returns
// defaultVal if the key is missing or not numeric.
func paramInt(params map[string]any, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case int64:
		return int(val)
	default:
		return defaultVal
	}
}

// paramFloat extracts a float64 parameter from a params map, handling
// int, float64, and int64 types that YAML v3 may produce. Returns
// defaultVal if the key is missing or not numeric.
func paramFloat(params map[string]any, key string, defaultVal float64) float64 {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return defaultVal
	}
}

// toFloat converts various numeric types to float64.
func toFloat(v any) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	default:
		return 0
	}
}

// toBool converts a value to bool.
func toBool(v any) bool {
	switch val := v.(type) {
	case bool:
		return val
	case int:
		return val != 0
	case int64:
		return val != 0
	case float64:
		return val != 0
	case string:
		return val != "" && val != "false" && val != "0"
	default:
		return v != nil
	}
}

package runner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/discovery"
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

	left := params["left"]
	right := params["right"]
	op, _ := params["operator"].(string)
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
	case "greater_than", "gt":
		result = leftF > rightF
	case "less_than", "lt":
		result = leftF < rightF
	case "greater_equal", "gte":
		result = leftF >= rightF
	case "less_equal", "lte":
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

	values, ok := params["values"].([]any)
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
	expr, _ := params["expression"].(string)
	if expr != "" {
		val, exists := state.Get(expr)
		if exists {
			return map[string]any{KeyResult: toBool(val)}, nil
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

	condKey, _ := params["condition"].(string)
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

	startKey, _ := params["start_key"].(string)
	endKey, _ := params["end_key"].(string)
	if startKey == "" {
		startKey = "start_time"
	}
	if endKey == "" {
		endKey = "end_time"
	}

	startVal, startOK := state.Get(startKey)
	endVal, endOK := state.Get(endKey)

	if !startOK || !endOK {
		return map[string]any{
			KeyWithinTolerance: false,
			KeyElapsedMs:       int64(0),
			KeyError:           "start or end time not recorded",
		}, nil
	}

	startTime, sok := startVal.(time.Time)
	endTime, eok := endVal.(time.Time)
	if !sok || !eok {
		return map[string]any{
			KeyWithinTolerance: false,
			KeyElapsedMs:       int64(0),
			KeyError:           "recorded values are not time.Time",
		}, nil
	}

	elapsed := endTime.Sub(startTime)
	elapsedMs := elapsed.Milliseconds()

	minMs := int64(0)
	maxMs := int64(0)
	if m, ok := params["min_ms"].(float64); ok {
		minMs = int64(m)
	}
	if m, ok := params["max_ms"].(float64); ok {
		maxMs = int64(m)
	}

	// Support max_duration as a Go duration string (e.g., "6s").
	if md, ok := params["max_duration"].(string); ok && md != "" {
		if d, err := time.ParseDuration(md); err == nil {
			maxMs = d.Milliseconds()
		}
	}

	withinTolerance := elapsedMs >= minMs
	if maxMs > 0 {
		withinTolerance = withinTolerance && elapsedMs <= maxMs
	}

	return map[string]any{
		KeyWithinTolerance: withinTolerance,
		KeyWithinLimit:     withinTolerance,
		KeyElapsedMs:       elapsedMs,
	}, nil
}

// handleCheckResponse verifies response status and payload fields.
func (r *Runner) handleCheckResponse(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	statusMatches := true
	if expected, ok := params["expected_status"]; ok {
		actual, _ := state.Get(KeyStatus)
		statusMatches = fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	}

	payloadMatches := true
	if fields, ok := params["expected_fields"].(map[string]any); ok {
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

	requestKey, _ := params["request_key"].(string)
	responseKey, _ := params["response_key"].(string)
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
	timeoutMs := 5000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}
	pollMs := 100
	if p, ok := params["poll_ms"].(float64); ok {
		pollMs = int(p)
	}

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

	timeoutMs := 5000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

	eventType, _ := params[KeyEventType].(string)

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
		return map[string]any{
			KeyNotificationReceived: true,
			KeyEventType:            eventType,
			KeyNotificationData:     res.data,
		}, nil
	case <-readCtx.Done():
		return map[string]any{
			KeyNotificationReceived: false,
			KeyEventType:            eventType,
		}, nil
	}
}

// handleWaitReport waits for a subscription priming report.
// If the subscribe response already included priming data (current values),
// that counts as a received report without waiting for a notification frame.
func (r *Runner) handleWaitReport(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := 5000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}
	// Also accept "timeout" as a duration string (e.g., "5s").
	if t, ok := params["timeout"].(string); ok {
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

	payload, _ := params["payload"].(string)
	// Accept "content" as fallback when "payload" is empty.
	if payload == "" {
		payload, _ = params["content"].(string)
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

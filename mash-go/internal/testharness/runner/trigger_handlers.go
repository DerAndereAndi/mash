package runner

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// registerTriggerHandlers registers test event trigger action handlers.
func (r *Runner) registerTriggerHandlers() {
	r.engine.RegisterHandler("trigger_test_event", r.handleTriggerTestEvent)
}

// handleTriggerTestEvent sends a triggerTestEvent invoke to the device's
// TestControl feature on endpoint 0.
func (r *Runner) handleTriggerTestEvent(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	params := engine.InterpolateParams(step.Params, state)

	// Get enable key: from step params, or fall back to runner config.
	enableKey := r.config.EnableKey
	if k, ok := params["enable_key"].(string); ok && k != "" {
		enableKey = k
	}
	if enableKey == "" {
		return nil, fmt.Errorf("no enable key configured (set --enable-key or provide enable_key param)")
	}

	// Get event trigger: accepts hex string (0x...) or numeric value.
	trigger, err := parseEventTrigger(params["event_trigger"])
	if err != nil {
		return nil, fmt.Errorf("invalid event_trigger: %w", err)
	}

	// Build invoke request: endpoint 0, TestControl (0x0A), command triggerTestEvent (0x01).
	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  uint8(model.FeatureTestControl),
		Payload: &wire.InvokePayload{
			CommandID: features.TestControlCmdTriggerTestEvent,
			Parameters: map[string]any{
				features.TriggerTestEventParamEnableKey:    enableKey,
				features.TriggerTestEventParamEventTrigger: trigger,
			},
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode trigger request: %w", err)
	}

	resp, err := r.sendRequest(data, "trigger_test_event")
	if err != nil {
		return nil, err
	}

	// Extract success from response payload.
	success := resp.IsSuccess()
	if resp.Payload != nil {
		if m, ok := resp.Payload.(map[any]any); ok {
			if v, ok := m["success"].(bool); ok {
				success = v
			}
		} else if m, ok := resp.Payload.(map[string]any); ok {
			if v, ok := m["success"].(bool); ok {
				success = v
			}
		}
	}

	return map[string]any{
		"trigger_sent":  true,
		"event_trigger": trigger,
		"success":       success,
		"status":        resp.Status,
	}, nil
}

// sendTrigger is a helper that sends a trigger via triggerTestEvent.
// Used by the convenience wrappers (enter/exit commissioning mode).
func (r *Runner) sendTrigger(ctx context.Context, trigger uint64, state *engine.ExecutionState) (map[string]any, error) {
	syntheticStep := &loader.Step{
		Action: "trigger_test_event",
		Params: map[string]any{
			"event_trigger": trigger,
		},
	}
	return r.handleTriggerTestEvent(ctx, syntheticStep, state)
}

// parseEventTrigger parses an event trigger value from YAML.
// Accepts: hex string "0x0001000000000001", float64 (from YAML numeric), uint64.
func parseEventTrigger(v any) (uint64, error) {
	switch val := v.(type) {
	case uint64:
		return val, nil
	case int64:
		return uint64(val), nil
	case int:
		return uint64(val), nil
	case float64:
		// YAML numeric values decode as float64.
		if val < 0 || val > math.MaxUint64 || val != math.Trunc(val) {
			return 0, fmt.Errorf("float64 value out of uint64 range: %v", val)
		}
		return uint64(val), nil
	case string:
		s := strings.TrimSpace(val)
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			return strconv.ParseUint(s[2:], 16, 64)
		}
		return strconv.ParseUint(s, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

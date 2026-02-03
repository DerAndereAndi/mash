package runner

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func newTestRunner() *Runner {
	return &Runner{
		config:          &Config{},
		conn:            &Connection{},
		activeZoneConns: make(map[string]*Connection),
	}
}

func TestHandleCompare(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	tests := []struct {
		name     string
		left     any
		right    any
		op       string
		wantComp bool
	}{
		{"equal ints", float64(42), float64(42), "equal", true},
		{"not equal ints", float64(42), float64(43), "equal", false},
		{"gt true", float64(10), float64(5), "gt", true},
		{"gt false", float64(5), float64(10), "gt", false},
		{"lt true", float64(5), float64(10), "lt", true},
		{"lt false", float64(10), float64(5), "lt", false},
		{"gte equal", float64(5), float64(5), "gte", true},
		{"lte equal", float64(5), float64(5), "lte", true},
		{"ne true", float64(1), float64(2), "ne", true},
		{"equal strings", "hello", "hello", "equal", true},
		{"default op is equal", float64(7), float64(7), "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &loader.Step{
				Params: map[string]any{
					"left":     tt.left,
					"right":    tt.right,
					"operator": tt.op,
				},
			}
			out, err := r.handleCompare(context.Background(), step, state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out["comparison_result"] != tt.wantComp {
				t.Errorf("comparison_result = %v, want %v", out["comparison_result"], tt.wantComp)
			}
		})
	}
}

func TestHandleCompareValues(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	tests := []struct {
		name          string
		values        []any
		wantEqual     bool
		wantDifferent bool
	}{
		{"all equal", []any{"a", "a", "a"}, true, false},
		{"all different", []any{"a", "b", "c"}, false, true},
		{"mixed", []any{"a", "b", "a"}, false, false},
		{"single", []any{"a"}, true, true},
		{"empty", []any{}, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &loader.Step{
				Params: map[string]any{"values": tt.values},
			}
			out, err := r.handleCompareValues(context.Background(), step, state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out["all_equal"] != tt.wantEqual {
				t.Errorf("all_equal = %v, want %v", out["all_equal"], tt.wantEqual)
			}
			if out["all_different"] != tt.wantDifferent {
				t.Errorf("all_different = %v, want %v", out["all_different"], tt.wantDifferent)
			}
		})
	}
}

func TestHandleEvaluate(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Evaluate from state key.
	state.Set("flag", true)
	step := &loader.Step{
		Params: map[string]any{"expression": "flag"},
	}
	out, err := r.handleEvaluate(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["result"] != true {
		t.Error("expected result=true for true state key")
	}

	// Evaluate direct value.
	step = &loader.Step{
		Params: map[string]any{KeyValue: false},
	}
	out, err = r.handleEvaluate(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["result"] != false {
		t.Error("expected result=false for false value")
	}

	// Missing expression returns false.
	step = &loader.Step{
		Params: map[string]any{"expression": "nonexistent"},
	}
	out, err = r.handleEvaluate(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["result"] != false {
		t.Error("expected result=false for missing key")
	}
}

func TestHandleRecordTimeAndVerifyTiming(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Record start time.
	step := &loader.Step{
		Params: map[string]any{"key": "t_start"},
	}
	out, err := r.handleRecordTime(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["time_recorded"] != true {
		t.Error("expected time_recorded=true")
	}
	if out["timestamp_ms"].(int64) <= 0 {
		t.Error("expected positive timestamp")
	}

	// Small delay.
	time.Sleep(10 * time.Millisecond)

	// Record end time.
	step = &loader.Step{
		Params: map[string]any{"key": "t_end"},
	}
	_, err = r.handleRecordTime(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify timing within tolerance.
	step = &loader.Step{
		Params: map[string]any{
			"start_key": "t_start",
			"end_key":   "t_end",
			"min_ms":    float64(5),
			"max_ms":    float64(5000),
		},
	}
	out, err = r.handleVerifyTiming(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["within_tolerance"] != true {
		t.Errorf("expected within_tolerance=true, elapsed=%v", out["elapsed_ms"])
	}
	elapsed := out["elapsed_ms"].(int64)
	if elapsed < 5 {
		t.Errorf("expected elapsed >= 5ms, got %d", elapsed)
	}
}

func TestHandleCheckResponse(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set("status", "SUCCESS")
	state.Set("device_id", "dev-123")

	step := &loader.Step{
		Params: map[string]any{
			"expected_status": "SUCCESS",
			"expected_fields": map[string]any{
				"device_id": "dev-123",
			},
		},
	}

	out, err := r.handleCheckResponse(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status_matches"] != true {
		t.Error("expected status_matches=true")
	}
	if out["payload_matches"] != true {
		t.Error("expected payload_matches=true")
	}

	// Mismatching status.
	step.Params["expected_status"] = "FAILURE"
	out, err = r.handleCheckResponse(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["status_matches"] != false {
		t.Error("expected status_matches=false for mismatched status")
	}
}

func TestHandleVerifyCorrelation(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set("request_message_id", uint32(42))
	state.Set("response_message_id", uint32(42))

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleVerifyCorrelation(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["correlation_valid"] != true {
		t.Error("expected correlation_valid=true")
	}

	// Mismatch.
	state.Set("response_message_id", uint32(99))
	out, err = r.handleVerifyCorrelation(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["correlation_valid"] != false {
		t.Error("expected correlation_valid=false")
	}
}

func TestHandleWaitForState(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// State already set -> immediate return.
	state.Set("ready", true)
	step := &loader.Step{
		Params: map[string]any{
			"key":        "ready",
			KeyValue:     true,
			"timeout_ms": float64(100),
			"poll_ms":    float64(10),
		},
	}
	out, err := r.handleWaitForState(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["state_reached"] != true {
		t.Error("expected state_reached=true")
	}

	// State never set -> timeout.
	step = &loader.Step{
		Params: map[string]any{
			"key":        "never_set",
			KeyValue:     "done",
			"timeout_ms": float64(50),
			"poll_ms":    float64(10),
		},
	}
	out, err = r.handleWaitForState(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["state_reached"] != false {
		t.Error("expected state_reached=false on timeout")
	}
}

func TestHandleParseQR(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Valid QR payload (4-field discovery format).
	step := &loader.Step{
		Params: map[string]any{
			"payload": "MASH:1:1234:12345678",
		},
	}
	out, err := r.handleParseQR(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Errorf("expected valid=true, got error=%v", out[KeyError])
	}
	if out["version"] != 1 {
		t.Errorf("expected version=1, got %v", out["version"])
	}
	if out["discriminator"] != 1234 {
		t.Errorf("expected discriminator=1234, got %v", out["discriminator"])
	}
	if out["setup_code"] != "12345678" {
		t.Errorf("expected setup_code=12345678, got %v", out["setup_code"])
	}

	// Invalid QR payload.
	step = &loader.Step{
		Params: map[string]any{"payload": "invalid"},
	}
	out, err = r.handleParseQR(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != false {
		t.Error("expected valid=false for invalid payload")
	}

	// Empty payload.
	step = &loader.Step{
		Params: map[string]any{},
	}
	out, err = r.handleParseQR(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != false {
		t.Error("expected valid=false for empty payload")
	}
}

func TestHandleParseQR_ErrorCodes(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	tests := []struct {
		name     string
		payload  string
		wantCode string
	}{
		{
			name:     "InvalidPrefix",
			payload:  "EEBUS:1:1234:12345678",
			wantCode: "invalid_prefix",
		},
		{
			name:     "InvalidFieldCount",
			payload:  "MASH:1:1234",
			wantCode: "invalid_field_count",
		},
		{
			name:     "InvalidVersion",
			payload:  "MASH:0:1234:12345678",
			wantCode: "invalid_version",
		},
		{
			name:     "DiscriminatorOutOfRange",
			payload:  "MASH:1:9999:12345678",
			wantCode: "discriminator_out_of_range",
		},
		{
			name:     "InvalidSetupCode",
			payload:  "MASH:1:1234:123",
			wantCode: "invalid_setup_code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &loader.Step{
				Params: map[string]any{"payload": tt.payload},
			}
			out, err := r.handleParseQR(context.Background(), step, state)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out["valid"] != false {
				t.Error("expected valid=false")
			}
			if out[KeyError] != tt.wantCode {
				t.Errorf("expected error=%q, got %q", tt.wantCode, out[KeyError])
			}
			// error_detail should contain the full error message.
			if out["error_detail"] == nil || out["error_detail"] == "" {
				t.Error("expected error_detail to be set")
			}
		})
	}
}

func TestHandleParseQR_LeadingZerosPreserved(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{"payload": "MASH:1:1234:00000001"},
	}
	out, err := r.handleParseQR(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Errorf("expected valid=true, got error=%v", out[KeyError])
	}
	if out["setup_code"] != "00000001" {
		t.Errorf("expected setup_code=00000001, got %v", out["setup_code"])
	}
}

func TestToBool(t *testing.T) {
	tests := []struct {
		input any
		want  bool
	}{
		{true, true},
		{false, false},
		{1, true},
		{0, false},
		{1.0, true},
		{0.0, false},
		{"hello", true},
		{"", false},
		{"false", false},
		{"0", false},
		{nil, false},
	}
	for _, tt := range tests {
		got := toBool(tt.input)
		if got != tt.want {
			t.Errorf("toBool(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		input any
		want  float64
	}{
		{42, 42.0},
		{int64(100), 100.0},
		{3.14, 3.14},
		{float32(2.5), 2.5},
		{"nope", 0},
		{nil, 0},
	}
	for _, tt := range tests {
		got := toFloat(tt.input)
		if got != tt.want {
			t.Errorf("toFloat(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHandleVerifyTiming_WithinLimitAlias(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up start and end times with a known gap.
	now := time.Now()
	state.Set("start_time", now.Add(-50*time.Millisecond))
	state.Set("end_time", now)

	step := &loader.Step{
		Params: map[string]any{
			"max_ms": float64(5000),
		},
	}
	out, err := r.handleVerifyTiming(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both within_tolerance and within_limit should be present with the same value.
	wt, wtOK := out[KeyWithinTolerance]
	wl, wlOK := out[KeyWithinLimit]
	if !wtOK {
		t.Error("expected within_tolerance key in output")
	}
	if !wlOK {
		t.Error("expected within_limit key in output")
	}
	if wt != wl {
		t.Errorf("within_tolerance (%v) != within_limit (%v)", wt, wl)
	}
}

func TestHandleVerifyTiming_MaxDuration(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up timing with a 50ms gap.
	now := time.Now()
	state.Set("start_time", now.Add(-50*time.Millisecond))
	state.Set("end_time", now)

	// Use max_duration as a Go duration string ("6s" = 6000ms).
	step := &loader.Step{
		Params: map[string]any{
			"max_duration": "6s",
		},
	}
	out, err := r.handleVerifyTiming(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyWithinTolerance] != true {
		t.Errorf("expected within_tolerance=true for 50ms < 6s, got %v", out[KeyWithinTolerance])
	}

	// Now test with a very short max_duration that should fail.
	step = &loader.Step{
		Params: map[string]any{
			"max_duration": "1ms",
		},
	}
	out, err = r.handleVerifyTiming(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyWithinTolerance] != false {
		t.Errorf("expected within_tolerance=false for 50ms > 1ms, got %v", out[KeyWithinTolerance])
	}
}

func TestHandleConnect_RecordsStartEndTime(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:1" // Connection refused -- fast failure
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{},
	}

	// Connect will fail (port closed) but should still record timing.
	_, _ = r.handleConnect(context.Background(), step, state)

	startVal, startOK := state.Get("start_time")
	endVal, endOK := state.Get("end_time")

	if !startOK {
		t.Error("expected start_time to be set in state after connect")
	}
	if !endOK {
		t.Error("expected end_time to be set in state after connect")
	}

	if startOK {
		if _, ok := startVal.(time.Time); !ok {
			t.Errorf("expected start_time to be time.Time, got %T", startVal)
		}
	}
	if endOK {
		if _, ok := endVal.(time.Time); !ok {
			t.Errorf("expected end_time to be time.Time, got %T", endVal)
		}
	}
}

func TestHandleCompareUnknownOperator(t *testing.T) {
	r := newTestRunner()
	state := engine.NewExecutionState(context.Background())

	step := &loader.Step{
		Params: map[string]any{
			"left":     float64(1),
			"right":    float64(2),
			"operator": "unknown_op",
		},
	}
	_, err := r.handleCompare(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for unknown operator")
	}
}

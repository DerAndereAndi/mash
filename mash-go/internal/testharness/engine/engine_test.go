package engine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestEngineBasic tests basic engine functionality.
func TestEngineBasic(t *testing.T) {
	e := engine.New()

	// Register a simple handler
	e.RegisterHandler("test_action", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return map[string]interface{}{
			"result": "success",
		}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-001",
		Name: "Basic Test",
		Steps: []loader.Step{
			{
				Action: "test_action",
				Expect: map[string]interface{}{
					"result": "success",
				},
			},
		},
	}

	result := e.Run(context.Background(), tc)

	if !result.Passed {
		t.Errorf("Test should pass, error: %v", result.Error)
	}
	if len(result.StepResults) != 1 {
		t.Errorf("Expected 1 step result, got %d", len(result.StepResults))
	}
}

// TestEngineSteps tests sequential step execution.
func TestEngineSteps(t *testing.T) {
	e := engine.New()

	// Track step execution order
	var executionOrder []int

	e.RegisterHandler("step_one", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		executionOrder = append(executionOrder, 1)
		return map[string]interface{}{"step_one_done": true}, nil
	})

	e.RegisterHandler("step_two", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		executionOrder = append(executionOrder, 2)
		// Access output from step one
		if _, ok := state.Get("step_one_done"); !ok {
			return nil, errors.New("step_one_done not found")
		}
		return map[string]interface{}{"step_two_done": true}, nil
	})

	e.RegisterHandler("step_three", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		executionOrder = append(executionOrder, 3)
		return map[string]interface{}{"step_three_done": true}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-STEPS",
		Name: "Steps Test",
		Steps: []loader.Step{
			{Action: "step_one", Expect: map[string]interface{}{"step_one_done": true}},
			{Action: "step_two", Expect: map[string]interface{}{"step_two_done": true}},
			{Action: "step_three", Expect: map[string]interface{}{"step_three_done": true}},
		},
	}

	result := e.Run(context.Background(), tc)

	if !result.Passed {
		t.Errorf("Test should pass, error: %v", result.Error)
	}
	if len(executionOrder) != 3 {
		t.Fatalf("Expected 3 steps executed, got %d", len(executionOrder))
	}
	for i, v := range executionOrder {
		if v != i+1 {
			t.Errorf("Step %d executed out of order: expected %d, got %d", i, i+1, v)
		}
	}
}

// TestEngineFilter tests PICS filtering.
func TestEngineFilter(t *testing.T) {
	config := engine.DefaultConfig()
	config.PICS = &loader.PICSFile{
		Items: map[string]interface{}{
			"D.COMM.SC":     true,
			"D.ELEC.PHASES": 3,
		},
	}

	e := engine.NewWithConfig(config)

	e.RegisterHandler("test", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return nil, nil
	})

	// Test case that should run
	tc1 := &loader.TestCase{
		ID:               "TC-MATCH",
		Name:             "Matching PICS",
		PICSRequirements: []string{"D.COMM.SC"},
		Steps:            []loader.Step{{Action: "test"}},
	}

	// Test case that should be skipped
	tc2 := &loader.TestCase{
		ID:               "TC-SKIP",
		Name:             "Missing PICS",
		PICSRequirements: []string{"D.NONEXISTENT"},
		Steps:            []loader.Step{{Action: "test"}},
	}

	result1 := e.Run(context.Background(), tc1)
	if result1.Skipped {
		t.Error("TC-MATCH should not be skipped")
	}

	result2 := e.Run(context.Background(), tc2)
	if !result2.Skipped {
		t.Error("TC-SKIP should be skipped")
	}
	if result2.SkipReason == "" {
		t.Error("SkipReason should be set")
	}
}

// TestDefaultChecker_PresentValue tests that "present" means "key exists".
func TestDefaultChecker_PresentValue(t *testing.T) {
	e := engine.New()

	e.RegisterHandler("emit_field", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return map[string]interface{}{
			"txt_field_ZN": "Home Energy",
			"txt_field_ZI": "abc123",
		}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-PRESENT",
		Name: "Present Checker",
		Steps: []loader.Step{
			{
				Action: "emit_field",
				Expect: map[string]interface{}{
					"txt_field_ZN": "present",
					"txt_field_ZI": "present",
				},
			},
		},
	}

	result := e.Run(context.Background(), tc)
	if !result.Passed {
		for _, sr := range result.StepResults {
			for _, er := range sr.ExpectResults {
				if !er.Passed {
					t.Errorf("expectation %s failed: %s", er.Key, er.Message)
				}
			}
		}
	}
}

// TestDefaultChecker_PresentValue_MissingKey tests that "present" fails for missing keys.
func TestDefaultChecker_PresentValue_MissingKey(t *testing.T) {
	e := engine.New()

	e.RegisterHandler("emit_nothing", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-PRESENT-MISSING",
		Name: "Present Missing Key",
		Steps: []loader.Step{
			{
				Action: "emit_nothing",
				Expect: map[string]interface{}{
					"txt_field_ZN": "present",
				},
			},
		},
	}

	result := e.Run(context.Background(), tc)
	if result.Passed {
		t.Error("expected test to fail when key is missing")
	}
}

// TestEngineTimeout tests timeout handling.
func TestEngineTimeout(t *testing.T) {
	config := engine.DefaultConfig()
	config.StepTimeout = 100 * time.Millisecond

	e := engine.NewWithConfig(config)

	e.RegisterHandler("slow_action", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			return map[string]interface{}{"done": true}, nil
		}
	})

	tc := &loader.TestCase{
		ID:    "TC-TIMEOUT",
		Name:  "Timeout Test",
		Steps: []loader.Step{{Action: "slow_action"}},
	}

	result := e.Run(context.Background(), tc)

	if result.Passed {
		t.Error("Test should fail due to timeout")
	}
	if result.Error == nil {
		t.Error("Error should be set")
	}
}

// TestEngineResults tests result collection.
func TestEngineResults(t *testing.T) {
	e := engine.New()

	e.RegisterHandler("pass", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return map[string]interface{}{"pass": true}, nil
	})

	e.RegisterHandler("fail", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return nil, errors.New("intentional failure")
	})

	cases := []*loader.TestCase{
		{ID: "TC-PASS-1", Name: "Pass 1", Steps: []loader.Step{{Action: "pass", Expect: map[string]interface{}{"pass": true}}}},
		{ID: "TC-PASS-2", Name: "Pass 2", Steps: []loader.Step{{Action: "pass", Expect: map[string]interface{}{"pass": true}}}},
		{ID: "TC-FAIL", Name: "Fail", Steps: []loader.Step{{Action: "fail"}}},
	}

	result := e.RunSuite(context.Background(), cases)

	if result.PassCount != 2 {
		t.Errorf("Expected 2 passed, got %d", result.PassCount)
	}
	if result.FailCount != 1 {
		t.Errorf("Expected 1 failed, got %d", result.FailCount)
	}
	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}
}

// TestEngineStopOnFirstFailure tests stop-on-failure mode.
func TestEngineStopOnFirstFailure(t *testing.T) {
	config := engine.DefaultConfig()
	config.StopOnFirstFailure = true

	e := engine.NewWithConfig(config)

	executed := make(map[string]bool)

	e.RegisterHandler("pass", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		executed[step.Params["id"].(string)] = true
		return nil, nil
	})

	e.RegisterHandler("fail", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		executed[step.Params["id"].(string)] = true
		return nil, errors.New("fail")
	})

	cases := []*loader.TestCase{
		{ID: "TC-1", Steps: []loader.Step{{Action: "pass", Params: map[string]interface{}{"id": "1"}}}},
		{ID: "TC-2", Steps: []loader.Step{{Action: "fail", Params: map[string]interface{}{"id": "2"}}}},
		{ID: "TC-3", Steps: []loader.Step{{Action: "pass", Params: map[string]interface{}{"id": "3"}}}},
	}

	result := e.RunSuite(context.Background(), cases)

	if executed["3"] {
		t.Error("TC-3 should not have executed after TC-2 failed")
	}
	if result.FailCount != 1 {
		t.Errorf("Expected 1 failure, got %d", result.FailCount)
	}
	if len(result.Results) != 2 {
		t.Errorf("Expected 2 results (stopped after failure), got %d", len(result.Results))
	}
}

// TestEnginePreconditions tests precondition support.
func TestEnginePreconditions(t *testing.T) {
	e := engine.New()

	// Handler that checks precondition was met
	e.RegisterHandler("check_precond", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		// In a real implementation, preconditions would be checked before running
		// For now, we just verify the state is accessible
		return map[string]interface{}{"checked": true}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-PRECOND",
		Name: "Preconditions Test",
		Preconditions: []loader.Condition{
			{"device_in_commissioning_mode": true},
		},
		Steps: []loader.Step{
			{Action: "check_precond", Expect: map[string]interface{}{"checked": true}},
		},
	}

	result := e.Run(context.Background(), tc)

	if !result.Passed {
		t.Errorf("Test should pass, error: %v", result.Error)
	}
}

// TestEngineExpectations tests expectation checking.
func TestEngineExpectations(t *testing.T) {
	e := engine.New()

	e.RegisterHandler("produce", func(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]interface{}, error) {
		return map[string]interface{}{
			"string_value": "hello",
			"int_value":    42,
			"bool_value":   true,
		}, nil
	})

	tc := &loader.TestCase{
		ID:   "TC-EXPECT",
		Name: "Expectations Test",
		Steps: []loader.Step{
			{
				Action: "produce",
				Expect: map[string]interface{}{
					"string_value": "hello",
					"int_value":    42,
					"bool_value":   true,
				},
			},
		},
	}

	result := e.Run(context.Background(), tc)

	if !result.Passed {
		t.Errorf("Test should pass, error: %v", result.Error)
	}

	// Check step result details
	if len(result.StepResults) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(result.StepResults))
	}

	sr := result.StepResults[0]
	if len(sr.ExpectResults) != 3 {
		t.Errorf("Expected 3 expect results, got %d", len(sr.ExpectResults))
	}

	for key, er := range sr.ExpectResults {
		if !er.Passed {
			t.Errorf("Expectation %s should pass: %s", key, er.Message)
		}
	}
}

// TestEngineUnknownAction tests handling of unknown actions.
func TestEngineUnknownAction(t *testing.T) {
	e := engine.New()

	tc := &loader.TestCase{
		ID:    "TC-UNKNOWN",
		Name:  "Unknown Action Test",
		Steps: []loader.Step{{Action: "nonexistent_action"}},
	}

	result := e.Run(context.Background(), tc)

	if result.Passed {
		t.Error("Test should fail for unknown action")
	}
	if result.Error == nil {
		t.Error("Error should be set")
	}
}

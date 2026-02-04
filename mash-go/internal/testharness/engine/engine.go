package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// Engine executes test cases.
type Engine struct {
	config   *EngineConfig
	handlers map[string]ActionHandler
	checkers map[string]ExpectChecker
	mu       sync.RWMutex
}

// New creates a new test engine with default configuration.
func New() *Engine {
	return NewWithConfig(DefaultConfig())
}

// NewWithConfig creates a new test engine with the given configuration.
func NewWithConfig(config *EngineConfig) *Engine {
	if config == nil {
		config = DefaultConfig()
	}

	e := &Engine{
		config:   config,
		handlers: make(map[string]ActionHandler),
		checkers: make(map[string]ExpectChecker),
	}

	// Register default checkers
	e.RegisterChecker(CheckerNameDefault, defaultChecker)

	return e
}

// RegisterHandler registers an action handler.
func (e *Engine) RegisterHandler(action string, handler ActionHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[action] = handler
}

// RegisterChecker registers an expectation checker.
func (e *Engine) RegisterChecker(key string, checker ExpectChecker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checkers[key] = checker
}

// Run executes a single test case.
func (e *Engine) Run(ctx context.Context, tc *loader.TestCase) *TestResult {
	result := &TestResult{
		TestCase:  tc,
		StartTime: time.Now(),
	}

	// Check explicit skip flag from YAML.
	if tc.Skip {
		result.Skipped = true
		result.SkipReason = tc.SkipReason
		if result.SkipReason == "" {
			result.SkipReason = "skipped by test definition"
		}
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Check PICS requirements
	if e.config.PICS != nil {
		if !loader.CheckPICSRequirements(e.config.PICS, tc.PICSRequirements) {
			result.Skipped = true
			result.SkipReason = "PICS requirements not met"
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}
	}

	// Parse test timeout
	timeout := e.config.DefaultTimeout
	if tc.Timeout != "" {
		if d, err := time.ParseDuration(tc.Timeout); err == nil {
			timeout = d
		}
	}

	// Create context with timeout
	testCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create execution state
	state := NewExecutionState(testCtx)

	// Fulfill preconditions
	if e.config.SetupPreconditions != nil {
		if err := e.config.SetupPreconditions(testCtx, tc, state); err != nil {
			result.Passed = false
			result.Error = fmt.Errorf("precondition setup failed: %w", err)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result
		}
	}

	// Execute steps
	for i := range tc.Steps {
		step := &tc.Steps[i]
		stepResult := e.executeStep(testCtx, step, i, state)
		result.StepResults = append(result.StepResults, stepResult)

		if !stepResult.Passed {
			result.Passed = false
			result.Error = stepResult.Error
			break
		}
	}

	// If all steps passed, mark as passed
	if result.Error == nil && !result.Skipped {
		result.Passed = true
		for _, sr := range result.StepResults {
			if !sr.Passed {
				result.Passed = false
				break
			}
		}
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// executeStep executes a single step.
func (e *Engine) executeStep(ctx context.Context, step *loader.Step, index int, state *ExecutionState) *StepResult {
	result := &StepResult{
		Step:          step,
		StepIndex:     index,
		ExpectResults: make(map[string]*ExpectResult),
		Output:        make(map[string]interface{}),
	}

	startTime := time.Now()

	// Parse step timeout
	timeout := e.config.StepTimeout
	if step.Timeout != "" {
		if d, err := time.ParseDuration(step.Timeout); err == nil {
			timeout = d
		}
	}

	// Create context with step timeout
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get handler
	e.mu.RLock()
	handler, exists := e.handlers[step.Action]
	e.mu.RUnlock()

	if !exists {
		result.Passed = false
		result.Error = fmt.Errorf("unknown action: %s", step.Action)
		result.Duration = time.Since(startTime)
		return result
	}

	// Execute handler
	outputs, err := handler(stepCtx, step, state)
	if err != nil {
		result.Passed = false
		result.Error = err
		result.Duration = time.Since(startTime)
		return result
	}

	// Store outputs
	for k, v := range outputs {
		state.Set(k, v)
		result.Output[k] = v
	}

	// Store the complete step output for save_as/value_equals checkers.
	outputCopy := make(map[string]interface{}, len(result.Output))
	for k, v := range result.Output {
		outputCopy[k] = v
	}
	state.Set(InternalStepOutput, outputCopy)

	// Check expectations with PICS-aware interpolation
	result.Passed = true
	interpolatedExpect := InterpolateParamsWithPICS(step.Expect, state, e.config.PICS)
	for key, expected := range interpolatedExpect {
		expectResult := e.checkExpectation(key, expected, state)
		result.ExpectResults[key] = expectResult
		if !expectResult.Passed {
			result.Passed = false
			result.Error = fmt.Errorf("expectation failed: %s - %s", key, expectResult.Message)
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// checkExpectation checks a single expectation.
func (e *Engine) checkExpectation(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	e.mu.RLock()
	checker, exists := e.checkers[key]
	if !exists {
		checker = e.checkers[CheckerNameDefault]
	}
	e.mu.RUnlock()

	return checker(key, expected, state)
}

// defaultChecker is the default expectation checker.
func defaultChecker(key string, expected interface{}, state *ExecutionState) *ExpectResult {
	actual, exists := state.Get(key)
	if !exists {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found in outputs", key),
		}
	}

	// "present" means the key exists with any non-nil value.
	if expStr, ok := expected.(string); ok && expStr == "present" {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   true,
			Message:  fmt.Sprintf("%s = %v", key, actual),
		}
	}

	// Detect unresolved PICS references (e.g., "${MASH.S.ZONE.MAX + 1}").
	// This happens when no PICS file is loaded but the expectation uses
	// a PICS value. Report a clear error instead of a confusing string
	// comparison between the raw expression and the actual value.
	if expStr, ok := expected.(string); ok && picsPattern.MatchString(expStr) {
		return &ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   actual,
			Passed:   false,
			Message:  fmt.Sprintf("unresolved PICS reference %s (no -pics flag provided?)", expStr),
		}
	}

	// Simple equality check
	passed := fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
	result := &ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
	}

	if passed {
		result.Message = fmt.Sprintf("%s = %v", key, expected)
	} else {
		result.Message = fmt.Sprintf("expected %v, got %v", expected, actual)
	}

	return result
}

// RunSuite executes all test cases in a suite.
func (e *Engine) RunSuite(ctx context.Context, cases []*loader.TestCase) *SuiteResult {
	result := &SuiteResult{
		SuiteName: "Test Suite",
	}

	startTime := time.Now()

	for _, tc := range cases {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return result
		default:
		}

		testResult := e.Run(ctx, tc)
		result.Results = append(result.Results, testResult)

		if testResult.Skipped {
			result.SkipCount++
		} else if testResult.Passed {
			result.PassCount++
		} else {
			result.FailCount++
		}

		if e.config.OnTestComplete != nil {
			e.config.OnTestComplete(testResult)
		}

		if !testResult.Passed && !testResult.Skipped && e.config.StopOnFirstFailure {
			break
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// FilterAndRun filters test cases by PICS and runs matching tests.
func (e *Engine) FilterAndRun(ctx context.Context, cases []*loader.TestCase, pics *loader.PICSFile) *SuiteResult {
	filtered := loader.FilterTestCases(cases, pics)
	return e.RunSuite(ctx, filtered)
}

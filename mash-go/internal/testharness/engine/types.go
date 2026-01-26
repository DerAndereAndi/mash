// Package engine provides test execution orchestration for the MASH test harness.
package engine

import (
	"context"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestResult represents the outcome of a single test case.
type TestResult struct {
	// TestCase is the test case that was executed.
	TestCase *loader.TestCase

	// Passed indicates if all steps passed.
	Passed bool

	// Error is the error that caused failure, if any.
	Error error

	// StepResults contains results for each step.
	StepResults []*StepResult

	// Duration is how long the test took.
	Duration time.Duration

	// StartTime when the test started.
	StartTime time.Time

	// EndTime when the test finished.
	EndTime time.Time

	// Skipped indicates if the test was skipped (e.g., PICS mismatch).
	Skipped bool

	// SkipReason explains why the test was skipped.
	SkipReason string
}

// StepResult represents the outcome of a single step.
type StepResult struct {
	// Step is the step that was executed.
	Step *loader.Step

	// StepIndex is the index of this step (0-based).
	StepIndex int

	// Passed indicates if the step passed.
	Passed bool

	// Error is the error that caused failure, if any.
	Error error

	// ExpectResults maps expectation keys to their assertion results.
	ExpectResults map[string]*ExpectResult

	// Duration is how long the step took.
	Duration time.Duration

	// Output contains any captured output from the step.
	Output map[string]interface{}
}

// ExpectResult represents the result of checking an expectation.
type ExpectResult struct {
	// Key is the expectation key (e.g., "device_found").
	Key string

	// Expected is the expected value.
	Expected interface{}

	// Actual is the actual value.
	Actual interface{}

	// Passed indicates if the expectation was met.
	Passed bool

	// Message describes the result.
	Message string
}

// SuiteResult represents the outcome of running a test suite.
type SuiteResult struct {
	// SuiteName identifies the test suite.
	SuiteName string

	// Results contains results for each test case.
	Results []*TestResult

	// PassCount is the number of passed tests.
	PassCount int

	// FailCount is the number of failed tests.
	FailCount int

	// SkipCount is the number of skipped tests.
	SkipCount int

	// Duration is the total time for all tests.
	Duration time.Duration
}

// ActionHandler processes a test step action.
// Returns outputs to make available for subsequent steps, and an error if the action failed.
type ActionHandler func(ctx context.Context, step *loader.Step, state *ExecutionState) (map[string]interface{}, error)

// ExpectChecker checks an expectation against actual results.
type ExpectChecker func(key string, expected interface{}, state *ExecutionState) *ExpectResult

// ExecutionState holds state during test execution.
type ExecutionState struct {
	// Outputs accumulated from previous steps.
	Outputs map[string]interface{}

	// Device is the mock device under test (if any).
	Device interface{}

	// Controller is the mock controller (if any).
	Controller interface{}

	// Context for cancellation.
	Context context.Context

	// Custom state that handlers can use.
	Custom map[string]interface{}
}

// NewExecutionState creates a new execution state.
func NewExecutionState(ctx context.Context) *ExecutionState {
	return &ExecutionState{
		Outputs: make(map[string]interface{}),
		Custom:  make(map[string]interface{}),
		Context: ctx,
	}
}

// Get retrieves a value from outputs, supporting template syntax.
func (s *ExecutionState) Get(key string) (interface{}, bool) {
	// Check for template reference
	if len(key) > 4 && key[:2] == "{{" && key[len(key)-2:] == "}}" {
		refKey := key[2 : len(key)-2]
		refKey = trimSpaces(refKey)
		v, ok := s.Outputs[refKey]
		return v, ok
	}
	v, ok := s.Outputs[key]
	return v, ok
}

// Set stores a value in outputs.
func (s *ExecutionState) Set(key string, value interface{}) {
	s.Outputs[key] = value
}

func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// EngineConfig configures the test engine.
type EngineConfig struct {
	// DefaultTimeout is the default timeout for test cases.
	DefaultTimeout time.Duration

	// StepTimeout is the default timeout for individual steps.
	StepTimeout time.Duration

	// ParallelTests is the number of tests to run in parallel.
	// 0 or 1 means sequential execution.
	ParallelTests int

	// StopOnFirstFailure stops execution after the first test failure.
	StopOnFirstFailure bool

	// PICS is the PICS configuration to filter tests.
	PICS *loader.PICSFile
}

// DefaultConfig returns the default engine configuration.
func DefaultConfig() *EngineConfig {
	return &EngineConfig{
		DefaultTimeout: 30 * time.Second,
		StepTimeout:    10 * time.Second,
		ParallelTests:  1,
	}
}

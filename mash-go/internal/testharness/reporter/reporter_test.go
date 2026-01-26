package reporter_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/internal/testharness/reporter"
)

func createTestResult(id, name string, passed, skipped bool, err error) *engine.TestResult {
	return &engine.TestResult{
		TestCase: &loader.TestCase{
			ID:   id,
			Name: name,
		},
		Passed:     passed,
		Skipped:    skipped,
		Error:      err,
		SkipReason: "PICS not met",
		Duration:   100 * time.Millisecond,
		StepResults: []*engine.StepResult{
			{
				Step:      &loader.Step{Action: "test_action"},
				StepIndex: 0,
				Passed:    passed,
				Duration:  50 * time.Millisecond,
				ExpectResults: map[string]*engine.ExpectResult{
					"result": {
						Key:      "result",
						Expected: "success",
						Actual:   "success",
						Passed:   passed,
						Message:  "result = success",
					},
				},
				Output: map[string]any{"result": "success"},
			},
		},
	}
}

func createSuiteResult() *engine.SuiteResult {
	return &engine.SuiteResult{
		SuiteName: "Test Suite",
		Results: []*engine.TestResult{
			createTestResult("TC-001", "Test 1", true, false, nil),
			createTestResult("TC-002", "Test 2", false, false, &testError{msg: "failed"}),
			createTestResult("TC-003", "Test 3", false, true, nil),
		},
		PassCount: 1,
		FailCount: 1,
		SkipCount: 1,
		Duration:  500 * time.Millisecond,
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestTextReporter(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewTextReporter(&buf, false)

	suite := createSuiteResult()
	r.ReportSuite(suite)

	output := buf.String()

	// Check header
	if !strings.Contains(output, "=== Suite: Test Suite ===") {
		t.Error("Missing suite header")
	}

	// Check test statuses
	if !strings.Contains(output, "[PASS] TC-001") {
		t.Error("Missing passed test")
	}
	if !strings.Contains(output, "[FAIL] TC-002") {
		t.Error("Missing failed test")
	}
	if !strings.Contains(output, "[SKIP] TC-003") {
		t.Error("Missing skipped test")
	}

	// Check summary
	if !strings.Contains(output, "Total:   3") {
		t.Error("Missing total count")
	}
	if !strings.Contains(output, "Passed:  1") {
		t.Error("Missing passed count")
	}
	if !strings.Contains(output, "Failed:  1") {
		t.Error("Missing failed count")
	}
	if !strings.Contains(output, "Pass Rate: 50.0%") {
		t.Error("Missing pass rate")
	}
}

func TestTextReporterVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewTextReporter(&buf, true)

	result := createTestResult("TC-001", "Test 1", true, false, nil)
	r.ReportTest(result)

	output := buf.String()

	// Check step details are included
	if !strings.Contains(output, "Step 1:") {
		t.Error("Missing step details in verbose mode")
	}
	if !strings.Contains(output, "test_action") {
		t.Error("Missing action name in verbose mode")
	}
	if !strings.Contains(output, "[OK] result") {
		t.Error("Missing expectation result in verbose mode")
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewJSONReporter(&buf, true)

	suite := createSuiteResult()
	r.ReportSuite(suite)

	// Parse JSON output
	var result reporter.JSONSuiteResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify structure
	if result.SuiteName != "Test Suite" {
		t.Errorf("Expected suite name 'Test Suite', got %s", result.SuiteName)
	}
	if result.Total != 3 {
		t.Errorf("Expected 3 total tests, got %d", result.Total)
	}
	if result.Passed != 1 {
		t.Errorf("Expected 1 passed, got %d", result.Passed)
	}
	if result.Failed != 1 {
		t.Errorf("Expected 1 failed, got %d", result.Failed)
	}
	if result.Skipped != 1 {
		t.Errorf("Expected 1 skipped, got %d", result.Skipped)
	}
	if result.PassRate != 50.0 {
		t.Errorf("Expected 50%% pass rate, got %.1f%%", result.PassRate)
	}

	// Verify tests array
	if len(result.Tests) != 3 {
		t.Fatalf("Expected 3 tests, got %d", len(result.Tests))
	}

	// Check test statuses
	if result.Tests[0].Status != "passed" {
		t.Errorf("Test 1 should be passed, got %s", result.Tests[0].Status)
	}
	if result.Tests[1].Status != "failed" {
		t.Errorf("Test 2 should be failed, got %s", result.Tests[1].Status)
	}
	if result.Tests[2].Status != "skipped" {
		t.Errorf("Test 3 should be skipped, got %s", result.Tests[2].Status)
	}
}

func TestJSONReporterSingleTest(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewJSONReporter(&buf, false)

	result := createTestResult("TC-001", "Test 1", true, false, nil)
	r.ReportTest(result)

	var jr reporter.JSONTestResult
	if err := json.Unmarshal(buf.Bytes(), &jr); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if jr.ID != "TC-001" {
		t.Errorf("Expected ID TC-001, got %s", jr.ID)
	}
	if jr.Status != "passed" {
		t.Errorf("Expected status passed, got %s", jr.Status)
	}
}

func TestJUnitReporter(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewJUnitReporter(&buf)

	suite := createSuiteResult()
	r.ReportSuite(suite)

	output := buf.String()

	// Check XML header
	if !strings.HasPrefix(output, `<?xml version="1.0"`) {
		t.Error("Missing XML header")
	}

	// Check testsuite element
	if !strings.Contains(output, `<testsuite name="Test Suite"`) {
		t.Error("Missing testsuite element")
	}
	if !strings.Contains(output, `tests="3"`) {
		t.Error("Missing tests count")
	}
	if !strings.Contains(output, `failures="1"`) {
		t.Error("Missing failures count")
	}
	if !strings.Contains(output, `skipped="1"`) {
		t.Error("Missing skipped count")
	}

	// Check testcase elements
	if !strings.Contains(output, `<testcase name="Test 1"`) {
		t.Error("Missing test case 1")
	}
	if !strings.Contains(output, `<testcase name="Test 2"`) {
		t.Error("Missing test case 2")
	}
	if !strings.Contains(output, `<testcase name="Test 3"`) {
		t.Error("Missing test case 3")
	}

	// Check failure element
	if !strings.Contains(output, `<failure message="failed">`) {
		t.Error("Missing failure element")
	}

	// Check skipped element
	if !strings.Contains(output, `<skipped message="`) {
		t.Error("Missing skipped element")
	}

	// Check closing tag
	if !strings.Contains(output, `</testsuite>`) {
		t.Error("Missing closing testsuite tag")
	}
}

func TestJUnitReporterSingleTest(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewJUnitReporter(&buf)

	result := createTestResult("TC-001", "Test 1", true, false, nil)
	r.ReportTest(result)

	output := buf.String()

	if !strings.Contains(output, `<testsuite name="Single Test"`) {
		t.Error("Single test should be wrapped in suite")
	}
	if !strings.Contains(output, `tests="1"`) {
		t.Error("Should have 1 test")
	}
}

func TestXMLEscaping(t *testing.T) {
	var buf bytes.Buffer
	r := reporter.NewJUnitReporter(&buf)

	result := &engine.TestResult{
		TestCase: &loader.TestCase{
			ID:   "TC-<>&'\"",
			Name: "Test with <special> & 'chars'",
		},
		Passed:      true,
		Duration:    100 * time.Millisecond,
		StepResults: []*engine.StepResult{},
	}

	r.ReportTest(result)
	output := buf.String()

	// Verify XML escaping
	if strings.Contains(output, `<special>`) {
		t.Error("Special characters not escaped")
	}
	if !strings.Contains(output, "&lt;special&gt;") {
		t.Error("< and > should be escaped")
	}
	if !strings.Contains(output, "&amp;") {
		t.Error("& should be escaped")
	}
}

// Package reporter provides test result formatting and output.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
)

// Reporter formats and outputs test results.
type Reporter interface {
	// ReportSuite reports results for a test suite.
	ReportSuite(result *engine.SuiteResult)

	// ReportTest reports results for a single test.
	ReportTest(result *engine.TestResult)
}

// TextReporter outputs human-readable text reports.
type TextReporter struct {
	writer  io.Writer
	verbose bool
}

// NewTextReporter creates a new text reporter.
func NewTextReporter(w io.Writer, verbose bool) *TextReporter {
	return &TextReporter{
		writer:  w,
		verbose: verbose,
	}
}

// ReportSuite reports suite results in text format.
func (r *TextReporter) ReportSuite(result *engine.SuiteResult) {
	fmt.Fprintf(r.writer, "\n=== Suite: %s ===\n", result.SuiteName)
	fmt.Fprintf(r.writer, "Duration: %s\n", result.Duration.Round(time.Millisecond))
	fmt.Fprintf(r.writer, "\n")

	for _, tr := range result.Results {
		r.ReportTest(tr)
	}

	// Summary
	fmt.Fprintf(r.writer, "\n--- Summary ---\n")
	fmt.Fprintf(r.writer, "Total:   %d\n", len(result.Results))
	fmt.Fprintf(r.writer, "Passed:  %d\n", result.PassCount)
	fmt.Fprintf(r.writer, "Failed:  %d\n", result.FailCount)
	fmt.Fprintf(r.writer, "Skipped: %d\n", result.SkipCount)

	// Pass rate
	total := result.PassCount + result.FailCount
	if total > 0 {
		rate := float64(result.PassCount) / float64(total) * 100
		fmt.Fprintf(r.writer, "Pass Rate: %.1f%%\n", rate)
	}
}

// ReportTest reports a single test result in text format.
func (r *TextReporter) ReportTest(result *engine.TestResult) {
	tc := result.TestCase

	// Status indicator
	var status string
	switch {
	case result.Skipped:
		status = "SKIP"
	case result.Passed:
		status = "PASS"
	default:
		status = "FAIL"
	}

	fmt.Fprintf(r.writer, "[%s] %s - %s (%s)\n",
		status, tc.ID, tc.Name, result.Duration.Round(time.Millisecond))

	if result.Skipped && result.SkipReason != "" {
		fmt.Fprintf(r.writer, "       Skip reason: %s\n", result.SkipReason)
	}

	if !result.Passed && result.Error != nil {
		fmt.Fprintf(r.writer, "       Error: %v\n", result.Error)
	}

	// Verbose: show step details
	if r.verbose {
		for _, sr := range result.StepResults {
			stepStatus := "PASS"
			if !sr.Passed {
				stepStatus = "FAIL"
			}
			fmt.Fprintf(r.writer, "    [%s] Step %d: %s (%s)\n",
				stepStatus, sr.StepIndex+1, sr.Step.Action, sr.Duration.Round(time.Millisecond))

			if !sr.Passed && sr.Error != nil {
				fmt.Fprintf(r.writer, "           Error: %v\n", sr.Error)
			}

			// Show expectation results
			for key, er := range sr.ExpectResults {
				expStatus := "OK"
				if !er.Passed {
					expStatus = "FAILED"
				}
				fmt.Fprintf(r.writer, "           [%s] %s: %s\n", expStatus, key, er.Message)
			}
		}
	}
}

// JSONReporter outputs JSON-formatted reports.
type JSONReporter struct {
	writer io.Writer
	pretty bool
}

// NewJSONReporter creates a new JSON reporter.
func NewJSONReporter(w io.Writer, pretty bool) *JSONReporter {
	return &JSONReporter{
		writer: w,
		pretty: pretty,
	}
}

// JSONSuiteResult is the JSON representation of suite results.
type JSONSuiteResult struct {
	SuiteName string           `json:"suite_name"`
	Duration  string           `json:"duration"`
	Total     int              `json:"total"`
	Passed    int              `json:"passed"`
	Failed    int              `json:"failed"`
	Skipped   int              `json:"skipped"`
	PassRate  float64          `json:"pass_rate"`
	Tests     []JSONTestResult `json:"tests"`
}

// JSONTestResult is the JSON representation of a test result.
type JSONTestResult struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	Duration   string           `json:"duration"`
	Error      string           `json:"error,omitempty"`
	SkipReason string           `json:"skip_reason,omitempty"`
	Steps      []JSONStepResult `json:"steps,omitempty"`
}

// JSONStepResult is the JSON representation of a step result.
type JSONStepResult struct {
	Index    int                     `json:"index"`
	Action   string                  `json:"action"`
	Status   string                  `json:"status"`
	Duration string                  `json:"duration"`
	Error    string                  `json:"error,omitempty"`
	Expects  map[string]JSONExpect   `json:"expects,omitempty"`
	Outputs  map[string]any          `json:"outputs,omitempty"`
}

// JSONExpect is the JSON representation of an expectation result.
type JSONExpect struct {
	Passed   bool   `json:"passed"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Message  string `json:"message"`
}

// ReportSuite reports suite results in JSON format.
func (r *JSONReporter) ReportSuite(result *engine.SuiteResult) {
	total := result.PassCount + result.FailCount
	var passRate float64
	if total > 0 {
		passRate = float64(result.PassCount) / float64(total) * 100
	}

	jr := JSONSuiteResult{
		SuiteName: result.SuiteName,
		Duration:  result.Duration.Round(time.Millisecond).String(),
		Total:     len(result.Results),
		Passed:    result.PassCount,
		Failed:    result.FailCount,
		Skipped:   result.SkipCount,
		PassRate:  passRate,
		Tests:     make([]JSONTestResult, 0, len(result.Results)),
	}

	for _, tr := range result.Results {
		jr.Tests = append(jr.Tests, r.testToJSON(tr))
	}

	r.writeJSON(jr)
}

// ReportTest reports a single test result in JSON format.
func (r *JSONReporter) ReportTest(result *engine.TestResult) {
	jr := r.testToJSON(result)
	r.writeJSON(jr)
}

func (r *JSONReporter) testToJSON(result *engine.TestResult) JSONTestResult {
	tc := result.TestCase

	var status string
	switch {
	case result.Skipped:
		status = "skipped"
	case result.Passed:
		status = "passed"
	default:
		status = "failed"
	}

	jr := JSONTestResult{
		ID:       tc.ID,
		Name:     tc.Name,
		Status:   status,
		Duration: result.Duration.Round(time.Millisecond).String(),
	}

	if result.Error != nil {
		jr.Error = result.Error.Error()
	}
	if result.SkipReason != "" {
		jr.SkipReason = result.SkipReason
	}

	// Add step results
	for _, sr := range result.StepResults {
		stepStatus := "passed"
		if !sr.Passed {
			stepStatus = "failed"
		}

		jsr := JSONStepResult{
			Index:    sr.StepIndex,
			Action:   sr.Step.Action,
			Status:   stepStatus,
			Duration: sr.Duration.Round(time.Millisecond).String(),
			Expects:  make(map[string]JSONExpect),
			Outputs:  sr.Output,
		}

		if sr.Error != nil {
			jsr.Error = sr.Error.Error()
		}

		for key, er := range sr.ExpectResults {
			jsr.Expects[key] = JSONExpect{
				Passed:   er.Passed,
				Expected: er.Expected,
				Actual:   er.Actual,
				Message:  er.Message,
			}
		}

		jr.Steps = append(jr.Steps, jsr)
	}

	return jr
}

func (r *JSONReporter) writeJSON(v any) {
	var data []byte
	var err error

	if r.pretty {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		fmt.Fprintf(r.writer, `{"error": "failed to marshal: %s"}`, err)
		return
	}

	fmt.Fprintln(r.writer, string(data))
}

// JUnitReporter outputs JUnit XML format for CI integration.
type JUnitReporter struct {
	writer io.Writer
}

// NewJUnitReporter creates a new JUnit reporter.
func NewJUnitReporter(w io.Writer) *JUnitReporter {
	return &JUnitReporter{writer: w}
}

// ReportSuite reports suite results in JUnit XML format.
func (r *JUnitReporter) ReportSuite(result *engine.SuiteResult) {
	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString("\n")

	// testsuite element
	fmt.Fprintf(&b, `<testsuite name="%s" tests="%d" failures="%d" skipped="%d" time="%.3f">`,
		escapeXML(result.SuiteName),
		len(result.Results),
		result.FailCount,
		result.SkipCount,
		result.Duration.Seconds())
	b.WriteString("\n")

	// testcase elements
	for _, tr := range result.Results {
		tc := tr.TestCase
		fmt.Fprintf(&b, `  <testcase name="%s" classname="%s" time="%.3f">`,
			escapeXML(tc.Name),
			escapeXML(tc.ID),
			tr.Duration.Seconds())
		b.WriteString("\n")

		if tr.Skipped {
			fmt.Fprintf(&b, `    <skipped message="%s"/>`, escapeXML(tr.SkipReason))
			b.WriteString("\n")
		} else if !tr.Passed && tr.Error != nil {
			fmt.Fprintf(&b, `    <failure message="%s">`, escapeXML(tr.Error.Error()))
			b.WriteString("\n")

			// Add step details in CDATA
			b.WriteString("      <![CDATA[")
			for _, sr := range tr.StepResults {
				if !sr.Passed {
					fmt.Fprintf(&b, "Step %d (%s): %v\n", sr.StepIndex+1, sr.Step.Action, sr.Error)
				}
			}
			b.WriteString("]]>\n")
			b.WriteString("    </failure>\n")
		}

		b.WriteString("  </testcase>\n")
	}

	b.WriteString("</testsuite>\n")

	fmt.Fprint(r.writer, b.String())
}

// ReportTest reports a single test in JUnit format (wraps in minimal testsuite).
func (r *JUnitReporter) ReportTest(result *engine.TestResult) {
	// Wrap single test in suite
	suite := &engine.SuiteResult{
		SuiteName: "Single Test",
		Results:   []*engine.TestResult{result},
		Duration:  result.Duration,
	}
	if result.Passed {
		suite.PassCount = 1
	} else if result.Skipped {
		suite.SkipCount = 1
	} else {
		suite.FailCount = 1
	}
	r.ReportSuite(suite)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

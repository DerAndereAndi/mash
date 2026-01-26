// Command mash-test is a test runner for MASH protocol conformance testing.
//
// This command runs protocol conformance tests against MASH devices or
// controllers, validating that they correctly implement the specification.
//
// Usage:
//
//	mash-test [flags] [test-pattern]
//
// Flags:
//
//	-target string      Target address (host:port) of device/controller under test
//	-mode string        Test mode: device, controller (default "device")
//	-pics string        Path to PICS file for the target
//	-tests string       Path to test cases directory
//	-timeout duration   Test timeout (default 30s)
//	-verbose            Enable verbose output
//	-json               Output results as JSON
//	-report string      Write HTML report to file
//
// Examples:
//
//	# Test a device at localhost:8443
//	mash-test -target localhost:8443 -mode device
//
//	# Test specific patterns with PICS file
//	mash-test -target 192.168.1.100:8443 -pics device.pics -tests ./testdata/cases
//
//	# Run specific test pattern with verbose output
//	mash-test -target localhost:8443 -verbose "EnergyControl.*"
//
// Test Pattern:
//
//	The optional test-pattern argument filters which tests to run.
//	It supports glob-style patterns:
//	  - "EnergyControl.*" - Run all EnergyControl tests
//	  - "*.SetLimit" - Run all SetLimit tests
//	  - "*" or "" - Run all tests
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds test runner configuration.
type Config struct {
	Target  string
	Mode    string
	PICS    string
	Tests   string
	Timeout time.Duration
	Verbose bool
	JSON    bool
	Report  string
	Pattern string
}

// TestResult represents the result of a single test.
type TestResult struct {
	Name     string        `json:"name"`
	Passed   bool          `json:"passed"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
	Skipped  bool          `json:"skipped,omitempty"`
	Reason   string        `json:"reason,omitempty"`
}

// TestReport represents the complete test run report.
type TestReport struct {
	Target    string        `json:"target"`
	Mode      string        `json:"mode"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
	Total     int           `json:"total"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
	Skipped   int           `json:"skipped"`
	Results   []TestResult  `json:"results"`
}

var config Config

func init() {
	flag.StringVar(&config.Target, "target", "", "Target address (host:port) of device/controller under test")
	flag.StringVar(&config.Mode, "mode", "device", "Test mode: device, controller")
	flag.StringVar(&config.PICS, "pics", "", "Path to PICS file for the target")
	flag.StringVar(&config.Tests, "tests", "./testdata/cases", "Path to test cases directory")
	flag.DurationVar(&config.Timeout, "timeout", 30*time.Second, "Test timeout")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&config.JSON, "json", false, "Output results as JSON")
	flag.StringVar(&config.Report, "report", "", "Write HTML report to file")
}

func main() {
	flag.Parse()

	// Get optional test pattern
	if flag.NArg() > 0 {
		config.Pattern = flag.Arg(0)
	}

	// Validate configuration
	if err := validateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Setup logging
	if !config.JSON {
		log.SetFlags(log.Ltime)
		if config.Verbose {
			log.SetFlags(log.Ltime | log.Lmicroseconds)
		}
	} else {
		log.SetOutput(os.Stderr)
	}

	if !config.JSON {
		printBanner()
		log.Printf("Target: %s", config.Target)
		log.Printf("Mode: %s", config.Mode)
		if config.PICS != "" {
			log.Printf("PICS: %s", config.PICS)
		}
		if config.Pattern != "" {
			log.Printf("Pattern: %s", config.Pattern)
		}
		log.Println()
	}

	// Run tests
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	report := runTests(ctx)

	// Output results
	if config.JSON {
		outputJSON(report)
	} else {
		outputText(report)
	}

	// Write HTML report if requested
	if config.Report != "" {
		if err := writeHTMLReport(report, config.Report); err != nil {
			log.Printf("Warning: Failed to write report: %v", err)
		} else {
			log.Printf("Report written to: %s", config.Report)
		}
	}

	// Exit with appropriate code
	if report.Failed > 0 {
		os.Exit(1)
	}
}

func validateConfig() error {
	if config.Target == "" {
		return fmt.Errorf("target address is required (-target)")
	}
	if config.Mode != "device" && config.Mode != "controller" {
		return fmt.Errorf("mode must be 'device' or 'controller', got '%s'", config.Mode)
	}
	return nil
}

func printBanner() {
	fmt.Print(`
 __  __    _    ____  _   _   _____         _
|  \/  |  / \  / ___|| | | | |_   _|__  ___| |_
| |\/| | / _ \ \___ \| |_| |   | |/ _ \/ __| __|
| |  | |/ ___ \ ___) |  _  |   | |  __/\__ \ |_
|_|  |_/_/   \_\____/|_| |_|   |_|\___||___/\__|

Protocol Conformance Test Runner
`)
}

func runTests(ctx context.Context) *TestReport {
	start := time.Now()

	report := &TestReport{
		Target:    config.Target,
		Mode:      config.Mode,
		Timestamp: start,
		Results:   []TestResult{},
	}

	// Load PICS if provided
	var pics map[string]bool
	if config.PICS != "" {
		var err error
		pics, err = loadPICS(config.PICS)
		if err != nil {
			log.Printf("Warning: Failed to load PICS: %v", err)
		} else if config.Verbose {
			log.Printf("Loaded %d PICS entries", len(pics))
		}
	}

	// Load test cases
	tests, err := loadTests(config.Tests, config.Pattern)
	if err != nil {
		log.Printf("Warning: Failed to load tests: %v", err)
		// Create some built-in tests
		tests = getBuiltinTests()
	}

	if !config.JSON {
		log.Printf("Running %d tests...\n", len(tests))
	}

	// Run each test
	for _, test := range tests {
		result := runTest(ctx, test, pics)
		report.Results = append(report.Results, result)
		report.Total++

		if result.Skipped {
			report.Skipped++
		} else if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}

		// Print progress
		if !config.JSON {
			printTestResult(result)
		}
	}

	report.Duration = time.Since(start)
	return report
}

func loadPICS(path string) (map[string]bool, error) {
	// TODO: Implement PICS file parsing
	// For now, return empty map
	_ = path
	return make(map[string]bool), nil
}

func loadTests(dir string, pattern string) ([]TestCase, error) {
	var tests []TestCase

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("test directory does not exist: %s", dir)
	}

	// Walk directory for YAML test files
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
			return nil
		}

		// TODO: Parse YAML test case
		// For now, create a placeholder test from filename
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

		// Apply pattern filter
		if pattern != "" && !matchPattern(name, pattern) {
			return nil
		}

		tests = append(tests, TestCase{
			Name: name,
			Path: path,
		})
		return nil
	})

	return tests, err
}

func matchPattern(name, pattern string) bool {
	// Simple glob matching
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		return strings.Contains(name, pattern[1:len(pattern)-1])
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(name, pattern[1:])
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}
	return name == pattern
}

func getBuiltinTests() []TestCase {
	return []TestCase{
		{Name: "Connection.TLS", Description: "Verify TLS connection can be established"},
		{Name: "Connection.Keepalive", Description: "Verify keepalive/ping-pong works"},
		{Name: "DeviceInfo.Read", Description: "Read DeviceInfo attributes"},
		{Name: "Electrical.Read", Description: "Read Electrical configuration"},
		{Name: "Measurement.Read", Description: "Read Measurement values"},
		{Name: "EnergyControl.Read", Description: "Read EnergyControl state"},
		{Name: "EnergyControl.SetLimit", Description: "Set power limit"},
		{Name: "EnergyControl.ClearLimit", Description: "Clear power limit"},
		{Name: "Subscription.Create", Description: "Create subscription"},
		{Name: "Subscription.Notification", Description: "Receive notifications"},
	}
}

// TestCase represents a test case to execute.
type TestCase struct {
	Name        string
	Description string
	Path        string
	PICSRequire []string
}

func runTest(ctx context.Context, test TestCase, pics map[string]bool) TestResult {
	start := time.Now()

	result := TestResult{
		Name: test.Name,
	}

	// Check PICS requirements
	for _, req := range test.PICSRequire {
		if pics != nil {
			if supported, exists := pics[req]; exists && !supported {
				result.Skipped = true
				result.Reason = fmt.Sprintf("PICS %s not supported", req)
				return result
			}
		}
	}

	// Create test context with timeout
	testCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Execute test
	err := executeTest(testCtx, test)
	result.Duration = time.Since(start)

	if err != nil {
		result.Passed = false
		result.Error = err.Error()
	} else {
		result.Passed = true
	}

	return result
}

func executeTest(ctx context.Context, test TestCase) error {
	// TODO: Implement actual test execution
	// This would:
	// 1. Connect to target
	// 2. Execute test steps (read, write, invoke, subscribe)
	// 3. Verify responses match expectations

	// For now, simulate test execution
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		// Simulate test running
	}

	// Placeholder: All tests pass for now
	if config.Verbose {
		log.Printf("  Executing: %s", test.Name)
	}

	return nil
}

func printTestResult(result TestResult) {
	var status string
	if result.Skipped {
		status = "SKIP"
	} else if result.Passed {
		status = "PASS"
	} else {
		status = "FAIL"
	}

	fmt.Printf("  [%s] %s (%v)\n", status, result.Name, result.Duration.Round(time.Millisecond))

	if result.Error != "" && config.Verbose {
		fmt.Printf("         Error: %s\n", result.Error)
	}
	if result.Reason != "" && config.Verbose {
		fmt.Printf("         Reason: %s\n", result.Reason)
	}
}

func outputText(report *TestReport) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("               SUMMARY                  ")
	fmt.Println("========================================")
	fmt.Printf("  Total:   %d\n", report.Total)
	fmt.Printf("  Passed:  %d\n", report.Passed)
	fmt.Printf("  Failed:  %d\n", report.Failed)
	fmt.Printf("  Skipped: %d\n", report.Skipped)
	fmt.Printf("  Duration: %v\n", report.Duration.Round(time.Millisecond))
	fmt.Println("========================================")

	if report.Failed > 0 {
		fmt.Println("\nFailed tests:")
		for _, r := range report.Results {
			if !r.Passed && !r.Skipped {
				fmt.Printf("  - %s: %s\n", r.Name, r.Error)
			}
		}
	}
}

func outputJSON(report *TestReport) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

func writeHTMLReport(report *TestReport, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>MASH Test Report</title>
    <style>
        body { font-family: -apple-system, sans-serif; margin: 40px; }
        h1 { color: #333; }
        .summary { background: #f5f5f5; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .passed { color: #28a745; }
        .failed { color: #dc3545; }
        .skipped { color: #6c757d; }
        table { border-collapse: collapse; width: 100%%; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background: #f0f0f0; }
        tr:hover { background: #f9f9f9; }
        .status-pass { background: #d4edda; }
        .status-fail { background: #f8d7da; }
        .status-skip { background: #e9ecef; }
    </style>
</head>
<body>
    <h1>MASH Protocol Conformance Test Report</h1>

    <div class="summary">
        <p><strong>Target:</strong> %s</p>
        <p><strong>Mode:</strong> %s</p>
        <p><strong>Timestamp:</strong> %s</p>
        <p><strong>Duration:</strong> %v</p>
    </div>

    <h2>Results</h2>
    <p>
        <span class="passed">Passed: %d</span> |
        <span class="failed">Failed: %d</span> |
        <span class="skipped">Skipped: %d</span> |
        Total: %d
    </p>

    <table>
        <tr>
            <th>Test</th>
            <th>Status</th>
            <th>Duration</th>
            <th>Details</th>
        </tr>
`, report.Target, report.Mode, report.Timestamp.Format(time.RFC3339),
		report.Duration.Round(time.Millisecond),
		report.Passed, report.Failed, report.Skipped, report.Total)

	for _, r := range report.Results {
		var status, class, details string
		if r.Skipped {
			status = "SKIP"
			class = "status-skip"
			details = r.Reason
		} else if r.Passed {
			status = "PASS"
			class = "status-pass"
		} else {
			status = "FAIL"
			class = "status-fail"
			details = r.Error
		}
		html += fmt.Sprintf(`        <tr class="%s">
            <td>%s</td>
            <td>%s</td>
            <td>%v</td>
            <td>%s</td>
        </tr>
`, class, r.Name, status, r.Duration.Round(time.Millisecond), details)
	}

	html += `    </table>
</body>
</html>`

	_, err = f.WriteString(html)
	return err
}

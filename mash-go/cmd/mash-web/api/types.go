// Package api provides HTTP API handlers for the MASH web testing frontend.
package api

import "time"

// TestCase represents a test case in API responses.
type TestCase struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	PICSRequirements []string `json:"pics_requirements,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	StepCount        int      `json:"step_count"`
	Timeout          string   `json:"timeout,omitempty"`
	Skip             bool     `json:"skip,omitempty"`
	SkipReason       string   `json:"skip_reason,omitempty"`
	SetID            string   `json:"set_id,omitempty"`
}

// TestSet represents a group of related test cases from a single file.
type TestSet struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	FileName    string     `json:"file_name"`
	TestCount   int        `json:"test_count"`
	Tags        []string   `json:"tags,omitempty"`
	Tests       []TestCase `json:"tests"`
}

// TestSetsResponse is the response for GET /api/v1/tests with grouping.
type TestSetsResponse struct {
	Sets  []TestSet `json:"sets"`
	Total int       `json:"total"`
}

// TestListResponse is the response for GET /api/v1/tests.
type TestListResponse struct {
	Tests []TestCase `json:"tests"`
	Total int        `json:"total"`
}

// RunRequest is the request body for POST /api/v1/runs.
type RunRequest struct {
	Target    string `json:"target"`
	Pattern   string `json:"pattern,omitempty"`
	SetupCode string `json:"setup_code,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Timeout   string `json:"timeout,omitempty"`
}

// Run represents a test run in API responses.
type Run struct {
	ID          string     `json:"id"`
	Target      string     `json:"target"`
	Pattern     string     `json:"pattern,omitempty"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	PassCount   int        `json:"pass_count"`
	FailCount   int        `json:"fail_count"`
	SkipCount   int        `json:"skip_count"`
	TotalCount  int        `json:"total_count"`
	Duration    string     `json:"duration,omitempty"`
}

// RunListResponse is the response for GET /api/v1/runs.
type RunListResponse struct {
	Runs  []Run `json:"runs"`
	Total int   `json:"total"`
}

// RunDetailResponse is the response for GET /api/v1/runs/:id.
type RunDetailResponse struct {
	Run
	Results []TestResult `json:"results,omitempty"`
}

// TestResult represents a single test result within a run.
type TestResult struct {
	TestID     string       `json:"test_id"`
	TestName   string       `json:"test_name"`
	Status     string       `json:"status"`
	Duration   string       `json:"duration,omitempty"`
	Error      string       `json:"error,omitempty"`
	SkipReason string       `json:"skip_reason,omitempty"`
	Steps      []StepResult `json:"steps,omitempty"`
}

// StepResult represents a single step result.
type StepResult struct {
	Index    int               `json:"index"`
	Action   string            `json:"action"`
	Status   string            `json:"status"`
	Duration string            `json:"duration,omitempty"`
	Error    string            `json:"error,omitempty"`
	Expects  map[string]Expect `json:"expects,omitempty"`
}

// Expect represents an expectation result.
type Expect struct {
	Passed   bool   `json:"passed"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
	Message  string `json:"message"`
}

// Device represents a discovered device in API responses.
type Device struct {
	InstanceName  string   `json:"instance_name"`
	Host          string   `json:"host"`
	Port          uint16   `json:"port"`
	Addresses     []string `json:"addresses"`
	Discriminator uint16   `json:"discriminator"`
	Brand         string   `json:"brand,omitempty"`
	Model         string   `json:"model,omitempty"`
	DeviceName    string   `json:"device_name,omitempty"`
	Serial        string   `json:"serial,omitempty"`
}

// DeviceListResponse is the response for GET /api/v1/devices.
type DeviceListResponse struct {
	Devices      []Device  `json:"devices"`
	DiscoveredAt time.Time `json:"discovered_at"`
	Timeout      string    `json:"timeout"`
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// RunStatus constants.
const (
	RunStatusPending   = "pending"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

// TestStatus constants.
const (
	TestStatusPassed  = "passed"
	TestStatusFailed  = "failed"
	TestStatusSkipped = "skipped"
)

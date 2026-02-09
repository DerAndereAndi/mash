// Package loader provides YAML test case loading for the MASH test harness.
package loader

import "fmt"

// TestCase represents a single test case loaded from YAML.
type TestCase struct {
	// ID is the unique test case identifier (e.g., "TC-COMM-001").
	ID string `yaml:"id"`

	// Name is a human-readable name for the test.
	Name string `yaml:"name"`

	// Description explains what the test validates.
	Description string `yaml:"description"`

	// PICSRequirements lists PICS items that must be supported to run this test.
	// Entries can be strings ("MASH.S.COMM") or maps ("MASH.S.COMM.X: 900").
	PICSRequirements PICSRequirementList `yaml:"pics_requirements"`

	// Preconditions are conditions that must be true before the test runs.
	Preconditions []Condition `yaml:"preconditions"`

	// Steps are the actions to execute in order.
	Steps []Step `yaml:"steps"`

	// Postconditions are conditions to verify after the test completes.
	Postconditions []Condition `yaml:"postconditions"`

	// Timeout is the maximum duration for the test (e.g., "30s").
	Timeout string `yaml:"timeout,omitempty"`

	// Tags for categorizing tests.
	Tags []string `yaml:"tags,omitempty"`

	// Skip indicates this test should be skipped.
	Skip bool `yaml:"skip,omitempty"`

	// SkipReason explains why the test is skipped.
	SkipReason string `yaml:"skip_reason,omitempty"`
}

// PICSRequirementList is a list of PICS requirement strings that supports
// mixed YAML entries: both plain strings ("MASH.S.COMM") and key-value maps
// ("MASH.S.COMM.WINDOW_DEFAULT: 900") are normalized to strings.
type PICSRequirementList []string

// UnmarshalYAML implements custom unmarshaling to handle mixed list entries.
func (p *PICSRequirementList) UnmarshalYAML(unmarshal func(any) error) error {
	var raw []any
	if err := unmarshal(&raw); err != nil {
		return err
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		switch v := item.(type) {
		case string:
			result = append(result, v)
		case map[string]any:
			for k, val := range v {
				result = append(result, fmt.Sprintf("%s: %v", k, val))
			}
		default:
			result = append(result, fmt.Sprintf("%v", v))
		}
	}
	*p = result
	return nil
}

// Condition represents a precondition or postcondition.
type Condition map[string]interface{}

// Step represents a single action in a test case.
type Step struct {
	// Action is the action to perform (e.g., "controller_discover", "device_write").
	Action string `yaml:"action"`

	// Params are parameters for the action.
	Params map[string]interface{} `yaml:"params,omitempty"`

	// Expect defines expected outcomes after the action.
	Expect map[string]interface{} `yaml:"expect,omitempty"`

	// Timeout overrides the test-level timeout for this step.
	Timeout string `yaml:"timeout,omitempty"`

	// Description explains what this step does.
	Description string `yaml:"description,omitempty"`

	// StoreResult stores a handler's primary output value (e.g., device_id)
	// under the given name for later interpolation via {{ name }}.
	StoreResult string `yaml:"store_result,omitempty"`
}

// TestSuite represents a collection of test cases.
type TestSuite struct {
	// Name of the test suite.
	Name string `yaml:"name"`

	// Description of what this suite tests.
	Description string `yaml:"description"`

	// Cases are the test cases in this suite.
	Cases []*TestCase `yaml:"cases"`

	// CommonPICS are PICS requirements for all tests in the suite.
	CommonPICS []string `yaml:"common_pics,omitempty"`
}

// PICSDevice contains device identification metadata from YAML PICS files.
type PICSDevice struct {
	Vendor  string `yaml:"vendor"`
	Product string `yaml:"product"`
	Model   string `yaml:"model"`
	Version string `yaml:"version"`
}

// PICSFile represents a Protocol Implementation Conformance Statement.
type PICSFile struct {
	// Name identifies this PICS configuration.
	Name string `yaml:"-"`

	// Device contains optional device metadata (YAML format only).
	Device PICSDevice `yaml:"device"`

	// Items maps PICS identifiers to their values.
	// Boolean items: MASH.S.TRANS.SC=true
	// Numeric items: MASH.S.ELEC.MAX_CURRENT=32000
	Items map[string]interface{} `yaml:"items"`
}

// ValidationLevel indicates the severity of a validation issue.
type ValidationLevel string

const (
	// ValidationLevelError indicates a critical issue that must be fixed.
	ValidationLevelError ValidationLevel = "error"
	// ValidationLevelWarning indicates an issue that should be reviewed.
	ValidationLevelWarning ValidationLevel = "warning"
	// ValidationLevelInfo indicates informational feedback.
	ValidationLevelInfo ValidationLevel = "info"
)

// ValidationError represents a PICS validation issue.
type ValidationError struct {
	// Field is the PICS item key that has an issue.
	Field string
	// Message describes the validation issue.
	Message string
	// Level indicates the severity of the issue.
	Level ValidationLevel
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// LoadError provides details about a test case loading error.
type LoadError struct {
	// File is the path to the file that failed to load.
	File string

	// Line is the line number where the error occurred (0 if unknown).
	Line int

	// Message describes the error.
	Message string

	// Cause is the underlying error, if any.
	Cause error
}

func (e *LoadError) Error() string {
	if e.Line > 0 {
		return e.File + ":" + string(rune(e.Line)) + ": " + e.Message
	}
	return e.File + ": " + e.Message
}

func (e *LoadError) Unwrap() error {
	return e.Cause
}

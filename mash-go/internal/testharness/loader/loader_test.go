package loader_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestLoaderParseBasic tests basic YAML test case parsing.
func TestLoaderParseBasic(t *testing.T) {
	yaml := `
id: TC-TEST-001
name: Basic Test
description: A simple test case
steps:
  - action: test_action
    params:
      key: value
`
	tc, err := loader.ParseTestCase([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse test case: %v", err)
	}

	if tc.ID != "TC-TEST-001" {
		t.Errorf("ID mismatch: expected TC-TEST-001, got %s", tc.ID)
	}
	if tc.Name != "Basic Test" {
		t.Errorf("Name mismatch: expected 'Basic Test', got %s", tc.Name)
	}
	if tc.Description != "A simple test case" {
		t.Errorf("Description mismatch")
	}
	if len(tc.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(tc.Steps))
	}
	if tc.Steps[0].Action != "test_action" {
		t.Errorf("Step action mismatch: expected test_action, got %s", tc.Steps[0].Action)
	}
}

// TestLoaderPICSRequirements tests PICS requirement parsing.
func TestLoaderPICSRequirements(t *testing.T) {
	yaml := `
id: TC-PICS-001
name: PICS Test
description: Test with PICS requirements
pics_requirements:
  - MASH.S.TRANS.SC
  - MASH.C.TRANS.SC
  - MASH.S.ELEC.PHASES
steps:
  - action: test
`
	tc, err := loader.ParseTestCase([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse test case: %v", err)
	}

	if len(tc.PICSRequirements) != 3 {
		t.Fatalf("Expected 3 PICS requirements, got %d", len(tc.PICSRequirements))
	}

	expected := []string{"MASH.S.TRANS.SC", "MASH.C.TRANS.SC", "MASH.S.ELEC.PHASES"}
	for i, req := range tc.PICSRequirements {
		if req != expected[i] {
			t.Errorf("PICS requirement %d mismatch: expected %s, got %s", i, expected[i], req)
		}
	}
}

// TestLoaderSteps tests step parsing with various configurations.
func TestLoaderSteps(t *testing.T) {
	yaml := `
id: TC-STEPS-001
name: Steps Test
description: Test step parsing
steps:
  - action: controller_discover
    params:
      discriminator: 1234
    expect:
      device_found: true
    timeout: 5s
    description: Find device by discriminator

  - action: controller_connect
    params:
      target: "{{ discovered_device }}"
      insecure: true
    expect:
      connection_established: true

  - action: controller_pase
    params:
      setup_code: "12345678"
    expect:
      pase_success: true
      session_key_derived: true
`
	tc, err := loader.ParseTestCase([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse test case: %v", err)
	}

	if len(tc.Steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(tc.Steps))
	}

	// Check first step
	step1 := tc.Steps[0]
	if step1.Action != "controller_discover" {
		t.Errorf("Step 1 action mismatch")
	}
	if step1.Timeout != "5s" {
		t.Errorf("Step 1 timeout mismatch: expected 5s, got %s", step1.Timeout)
	}
	if step1.Params["discriminator"] != 1234 {
		t.Errorf("Step 1 discriminator param mismatch")
	}
	if step1.Expect["device_found"] != true {
		t.Errorf("Step 1 expect mismatch")
	}

	// Check second step has template param
	step2 := tc.Steps[1]
	if step2.Params["target"] != "{{ discovered_device }}" {
		t.Errorf("Step 2 target param mismatch")
	}

	// Check third step has multiple expects
	step3 := tc.Steps[2]
	if len(step3.Expect) != 2 {
		t.Errorf("Step 3 should have 2 expectations, got %d", len(step3.Expect))
	}
}

// TestLoaderPreconditions tests precondition parsing.
func TestLoaderPreconditions(t *testing.T) {
	yaml := `
id: TC-PRECOND-001
name: Preconditions Test
description: Test precondition parsing
preconditions:
  - device_in_commissioning_mode: true
  - controller_has_zone_cert: true
  - device_connection_count: 0
postconditions:
  - device_has_zone_cert: true
  - device_state: CONTROLLED
steps:
  - action: test
`
	tc, err := loader.ParseTestCase([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse test case: %v", err)
	}

	if len(tc.Preconditions) != 3 {
		t.Fatalf("Expected 3 preconditions, got %d", len(tc.Preconditions))
	}
	if len(tc.Postconditions) != 2 {
		t.Fatalf("Expected 2 postconditions, got %d", len(tc.Postconditions))
	}

	// Check first precondition
	if tc.Preconditions[0]["device_in_commissioning_mode"] != true {
		t.Error("Precondition 1 value mismatch")
	}

	// Check postcondition with string value
	if tc.Postconditions[1]["device_state"] != "CONTROLLED" {
		t.Error("Postcondition 2 value mismatch")
	}
}

// TestLoaderErrors tests error handling for invalid YAML.
func TestLoaderErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "invalid yaml syntax",
			yaml: `
id: TC-ERR-001
name: Bad YAML
  invalid indentation here
`,
		},
		{
			name: "missing required id",
			yaml: `
name: No ID Test
steps:
  - action: test
`,
		},
		{
			name: "empty steps",
			yaml: `
id: TC-ERR-002
name: Empty Steps
steps: []
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loader.ParseTestCase([]byte(tt.yaml))
			if err == nil {
				t.Error("Expected error but got nil")
			}
		})
	}
}

// TestLoaderLoadFile tests loading a test case from a file.
func TestLoaderLoadFile(t *testing.T) {
	// Create temp file
	dir := t.TempDir()
	file := filepath.Join(dir, "test-case.yaml")

	yaml := `
id: TC-FILE-001
name: File Test
description: Test loaded from file
steps:
  - action: test_action
`
	if err := os.WriteFile(file, []byte(yaml), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	tc, err := loader.LoadTestCase(file)
	if err != nil {
		t.Fatalf("Failed to load test case: %v", err)
	}

	if tc.ID != "TC-FILE-001" {
		t.Errorf("ID mismatch: expected TC-FILE-001, got %s", tc.ID)
	}
}

// TestLoaderLoadDirectory tests loading all test cases from a directory.
func TestLoaderLoadDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create multiple test files
	files := map[string]string{
		"tc-001.yaml": `
id: TC-001
name: Test 1
steps:
  - action: test
`,
		"tc-002.yaml": `
id: TC-002
name: Test 2
steps:
  - action: test
`,
		"readme.md": "# Not a test file",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	cases, err := loader.LoadDirectory(dir)
	if err != nil {
		t.Fatalf("Failed to load directory: %v", err)
	}

	if len(cases) != 2 {
		t.Errorf("Expected 2 test cases, got %d", len(cases))
	}
}

// TestLoadDirectoryWithFilter tests file-path scoping.
func TestLoadDirectoryWithFilter(t *testing.T) {
	dir := t.TempDir()

	// Create test files with different names.
	files := map[string]string{
		"protocol-behavior-tests.yaml": `
id: TC-PROTO-001
name: Protocol Test
steps:
  - action: test
`,
		"connection-basic-tests.yaml": `
id: TC-CONN-001
name: Connection Test
steps:
  - action: test
`,
		"connection-reaper-tests.yaml": `
id: TC-CONN-REAP-001
name: Reaper Test
steps:
  - action: test
`,
		"energy-control-tests.yaml": `
id: TC-EC-001
name: Energy Test
steps:
  - action: test
`,
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	tests := []struct {
		name     string
		filter   string
		wantIDs  []string
	}{
		{
			name:    "exact stem",
			filter:  "protocol-behavior-tests",
			wantIDs: []string{"TC-PROTO-001"},
		},
		{
			name:    "glob wildcard",
			filter:  "connection-*",
			wantIDs: []string{"TC-CONN-001", "TC-CONN-REAP-001"},
		},
		{
			name:    "comma-separated patterns",
			filter:  "protocol-*,energy-*",
			wantIDs: []string{"TC-PROTO-001", "TC-EC-001"},
		},
		{
			name:    "empty filter loads all",
			filter:  "",
			wantIDs: []string{"TC-PROTO-001", "TC-CONN-001", "TC-CONN-REAP-001", "TC-EC-001"},
		},
		{
			name:    "no matches",
			filter:  "nonexistent-*",
			wantIDs: []string{},
		},
		{
			name:    "whitespace in patterns trimmed",
			filter:  " protocol-* , energy-* ",
			wantIDs: []string{"TC-PROTO-001", "TC-EC-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cases, err := loader.LoadDirectoryWithFilter(dir, tt.filter)
			if err != nil {
				t.Fatalf("LoadDirectoryWithFilter(%q) error: %v", tt.filter, err)
			}
			gotIDs := make(map[string]bool)
			for _, tc := range cases {
				gotIDs[tc.ID] = true
			}
			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("expected %s in results, got %v", wantID, gotIDs)
				}
			}
			if len(cases) != len(tt.wantIDs) {
				t.Errorf("expected %d cases, got %d", len(tt.wantIDs), len(cases))
			}
		})
	}
}

// TestLoaderParsePICS tests PICS file parsing.
func TestLoaderParsePICS(t *testing.T) {
	pics := `
# EVSE PICS file
MASH.S.INFO=1
MASH.S.ELEC=1
MASH.S.ELEC.PHASES=3
MASH.S.ELEC.MAX_CURRENT=32000
MASH.S.TRANS.SC=1
MASH.S.TRANS.WINDOW_TIMEOUT=120
MASH.S.CTRL.LIMIT=1
MASH.S.CTRL.FAILSAFE=14400
`
	pf, err := loader.ParsePICS([]byte(pics))
	if err != nil {
		t.Fatalf("Failed to parse PICS: %v", err)
	}

	// Check boolean item (1 is parsed as bool true)
	if v, ok := pf.Items["MASH.S.INFO"]; !ok || v != true {
		t.Errorf("MASH.S.INFO should be true, got %v", v)
	}

	// Check numeric item
	if v, ok := pf.Items["MASH.S.ELEC.PHASES"]; !ok {
		t.Error("MASH.S.ELEC.PHASES should exist")
	} else if v != 3 {
		t.Errorf("MASH.S.ELEC.PHASES should be 3, got %v", v)
	}

	// Check larger numeric item
	if v, ok := pf.Items["MASH.S.ELEC.MAX_CURRENT"]; !ok {
		t.Error("MASH.S.ELEC.MAX_CURRENT should exist")
	} else if v != 32000 {
		t.Errorf("MASH.S.ELEC.MAX_CURRENT should be 32000, got %v", v)
	}
}

// TestLoaderCheckPICS tests PICS requirement checking.
func TestLoaderCheckPICS(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"MASH.S.TRANS.SC":          true,
			"MASH.S.ELEC.PHASES":       3,
			"MASH.S.ELEC.ENABLED":      false,
			"MASH.S.COMM.WINDOW_MIN":   180,
			"MASH.S.COMM.WINDOW_MAX":   10800,
			"MASH.S.DISC.BROWSE_TIMEOUT": 10,
			"MASH.S.TLS.VERSION":       "1.3",
			"MASH.S.TLS.ALPN":          "mash/1",
		},
	}

	tests := []struct {
		name         string
		requirements []string
		shouldMatch  bool
	}{
		{
			name:         "all present and true",
			requirements: []string{"MASH.S.TRANS.SC"},
			shouldMatch:  true,
		},
		{
			name:         "numeric requirement exists",
			requirements: []string{"MASH.S.ELEC.PHASES"},
			shouldMatch:  true,
		},
		{
			name:         "missing requirement",
			requirements: []string{"MASH.S.NONEXISTENT"},
			shouldMatch:  false,
		},
		{
			name:         "false boolean",
			requirements: []string{"MASH.S.ELEC.ENABLED"},
			shouldMatch:  false,
		},
		{
			name:         "mixed - one fails",
			requirements: []string{"MASH.S.TRANS.SC", "MASH.S.NONEXISTENT"},
			shouldMatch:  false,
		},
		{
			name:         "empty requirements",
			requirements: []string{},
			shouldMatch:  true,
		},
		// Key-value format: "KEY: VALUE" (from YAML map entries)
		{
			name:         "colon format - int match",
			requirements: []string{"MASH.S.COMM.WINDOW_MIN: 180"},
			shouldMatch:  true,
		},
		{
			name:         "colon format - int mismatch",
			requirements: []string{"MASH.S.COMM.WINDOW_MIN: 999"},
			shouldMatch:  false,
		},
		{
			name:         "colon format - string match",
			requirements: []string{"MASH.S.TLS.ALPN: mash/1"},
			shouldMatch:  true,
		},
		{
			name:         "colon format - string mismatch",
			requirements: []string{"MASH.S.TLS.ALPN: wrong"},
			shouldMatch:  false,
		},
		{
			name:         "colon format - missing key",
			requirements: []string{"MASH.S.NONEXISTENT: 42"},
			shouldMatch:  false,
		},
		// Key-value format: "KEY=VALUE" (inline format)
		{
			name:         "equals format - int match",
			requirements: []string{"MASH.S.DISC.BROWSE_TIMEOUT=10"},
			shouldMatch:  true,
		},
		{
			name:         "equals format - int mismatch",
			requirements: []string{"MASH.S.DISC.BROWSE_TIMEOUT=99"},
			shouldMatch:  false,
		},
		{
			name:         "equals format - string match",
			requirements: []string{"MASH.S.TLS.VERSION=1.3"},
			shouldMatch:  true,
		},
		{
			name:         "equals format - missing key",
			requirements: []string{"MASH.S.NONEXISTENT=1"},
			shouldMatch:  false,
		},
		// Mixed requirements
		{
			name:         "mixed bool and key-value - all pass",
			requirements: []string{"MASH.S.TRANS.SC", "MASH.S.COMM.WINDOW_MIN: 180", "MASH.S.DISC.BROWSE_TIMEOUT=10"},
			shouldMatch:  true,
		},
		{
			name:         "mixed bool and key-value - value fails",
			requirements: []string{"MASH.S.TRANS.SC", "MASH.S.COMM.WINDOW_MIN: 999"},
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.CheckPICSRequirements(pf, tt.requirements)
			if result != tt.shouldMatch {
				t.Errorf("CheckPICSRequirements returned %v, expected %v", result, tt.shouldMatch)
			}
		})
	}
}

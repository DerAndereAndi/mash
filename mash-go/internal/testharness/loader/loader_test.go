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
  - D.COMM.SC
  - C.COMM.SC
  - D.ELEC.PHASES
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

	expected := []string{"D.COMM.SC", "C.COMM.SC", "D.ELEC.PHASES"}
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

// TestLoaderParsePICS tests PICS file parsing.
func TestLoaderParsePICS(t *testing.T) {
	pics := `
# EVSE PICS file
D.DI.TYPE.EVSE=true
D.DI.SOFT.VERSION=true
D.ELEC.PHASES=3
D.ELEC.MAX_CURRENT=32000
D.COMM.SC=true
D.COMM.WINDOW_TIMEOUT=120
D.EC.LIMIT.CONSUMPTION=true
D.EC.FAILSAFE.DURATION=14400
`
	pf, err := loader.ParsePICS([]byte(pics))
	if err != nil {
		t.Fatalf("Failed to parse PICS: %v", err)
	}

	// Check boolean item
	if v, ok := pf.Items["D.DI.TYPE.EVSE"]; !ok || v != true {
		t.Error("D.DI.TYPE.EVSE should be true")
	}

	// Check numeric item
	if v, ok := pf.Items["D.ELEC.PHASES"]; !ok {
		t.Error("D.ELEC.PHASES should exist")
	} else if v != 3 {
		t.Errorf("D.ELEC.PHASES should be 3, got %v", v)
	}

	// Check larger numeric item
	if v, ok := pf.Items["D.ELEC.MAX_CURRENT"]; !ok {
		t.Error("D.ELEC.MAX_CURRENT should exist")
	} else if v != 32000 {
		t.Errorf("D.ELEC.MAX_CURRENT should be 32000, got %v", v)
	}
}

// TestLoaderCheckPICS tests PICS requirement checking.
func TestLoaderCheckPICS(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"D.COMM.SC":      true,
			"D.ELEC.PHASES":  3,
			"D.ELEC.ENABLED": false,
		},
	}

	tests := []struct {
		name         string
		requirements []string
		shouldMatch  bool
	}{
		{
			name:         "all present and true",
			requirements: []string{"D.COMM.SC"},
			shouldMatch:  true,
		},
		{
			name:         "numeric requirement exists",
			requirements: []string{"D.ELEC.PHASES"},
			shouldMatch:  true,
		},
		{
			name:         "missing requirement",
			requirements: []string{"D.NONEXISTENT"},
			shouldMatch:  false,
		},
		{
			name:         "false boolean",
			requirements: []string{"D.ELEC.ENABLED"},
			shouldMatch:  false,
		},
		{
			name:         "mixed - one fails",
			requirements: []string{"D.COMM.SC", "D.NONEXISTENT"},
			shouldMatch:  false,
		},
		{
			name:         "empty requirements",
			requirements: []string{},
			shouldMatch:  true,
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

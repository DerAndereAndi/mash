package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerator_ProducesValidGo(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal valid YAML with scenarios
	yamlContent := `
name: TEST
id: 0xFF
fullName: Test Use Case
specVersion: "1.0"
major: 1
minor: 0
description: A test use case.
endpointTypes:
  - EV_CHARGER
scenarios:
  - bit: 0
    name: BASE
    description: Base scenario.
    features:
      - feature: EnergyControl
        required: true
        attributes:
          - name: acceptsLimits
            requiredValue: true
        commands:
          - setLimit
        subscribe: all
  - bit: 1
    name: MEASUREMENT
    description: Measurement scenario.
    features:
      - feature: Measurement
        required: true
commands:
  - test-cmd
`
	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "test.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(tmpDir, "definitions_gen.go")

	// Run generator
	err := runGenerator(inputDir, outputFile, "1.0")
	if err != nil {
		t.Fatalf("generator failed: %v", err)
	}

	// Verify output exists
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}

	content := string(data)

	// Check structure
	if !strings.Contains(content, "package usecase") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(content, "Registry") {
		t.Error("missing Registry variable")
	}
	if !strings.Contains(content, `"TEST"`) {
		t.Error("missing TEST use case")
	}
	if !strings.Contains(content, "ID:") {
		t.Error("missing ID field in generated output")
	}
	if !strings.Contains(content, "0xFF") || !strings.Contains(content, "0xff") {
		// Accept either case for hex
		if !strings.Contains(content, "0xFF") && !strings.Contains(content, "0xff") {
			t.Error("missing hex ID value in generated output")
		}
	}
	if !strings.Contains(content, "Scenarios:") {
		t.Error("missing Scenarios field in generated output")
	}
	if !strings.Contains(content, "ScenarioDef") {
		t.Error("missing ScenarioDef type in generated output")
	}
	if !strings.Contains(content, "NameToID") {
		t.Error("missing NameToID map")
	}
	if !strings.Contains(content, "IDToName") {
		t.Error("missing IDToName map")
	}
	if !strings.Contains(content, "Major:") {
		t.Error("missing Major field in generated output")
	}
	if !strings.Contains(content, "Minor:") {
		t.Error("missing Minor field in generated output")
	}

	// Just verify syntax with gofmt
	cmd := exec.Command("gofmt", "-e", outputFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("generated file has syntax errors: %v\n%s", err, out)
	}
}

func TestGenerator_InvalidYAML_Fails(t *testing.T) {
	tmpDir := t.TempDir()

	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(inputDir, "bad.yaml"), []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(tmpDir, "out.go")
	err := runGenerator(inputDir, outputFile, "1.0")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestGenerator_UnknownFeature_Fails(t *testing.T) {
	tmpDir := t.TempDir()

	inputDir := filepath.Join(tmpDir, "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	yamlContent := `
name: BAD
id: 0xFE
fullName: Bad Use Case
specVersion: "1.0"
scenarios:
  - bit: 0
    name: BASE
    features:
      - feature: NoSuchFeature
        required: true
commands: []
`
	if err := os.WriteFile(filepath.Join(inputDir, "bad.yaml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(tmpDir, "out.go")
	err := runGenerator(inputDir, outputFile, "1.0")
	if err == nil {
		t.Fatal("expected error for unknown feature")
	}
	if !strings.Contains(err.Error(), "NoSuchFeature") {
		t.Errorf("error should mention feature name, got: %v", err)
	}
}

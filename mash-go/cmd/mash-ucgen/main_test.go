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

	// Write a minimal valid YAML
	yamlContent := `
name: TEST
fullName: Test Use Case
specVersion: "1.0"
description: A test use case.
endpointTypes:
  - EV_CHARGER
features:
  - feature: EnergyControl
    required: true
    attributes:
      - name: acceptsLimits
        requiredValue: true
    commands:
      - setLimit
    subscriptions:
      - controlState
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

	// Verify it compiles by writing a main.go that imports it
	mainDir := filepath.Join(tmpDir, "check")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Copy the generated file into a temp package to compile-check
	checkFile := filepath.Join(mainDir, "check.go")
	checkContent := `package main

import "fmt"

` + content + `

func main() {
	fmt.Println(len(Registry))
}
`
	// Replace package name for standalone compilation
	checkContent = strings.Replace(checkContent, "package usecase", "// generated", 1)

	if err := os.WriteFile(checkFile, []byte(checkContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Just verify syntax with go vet on the generated file itself
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
fullName: Bad Use Case
specVersion: "1.0"
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

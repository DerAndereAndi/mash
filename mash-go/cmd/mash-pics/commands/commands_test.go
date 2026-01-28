package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunValidate_ValidFile(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunValidate([]string{"../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected OK in output, got: %s", stdout.String())
	}
}

func TestRunValidate_InvalidFile(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunValidate([]string{"nonexistent.pics"}, stdout, stderr)

	// Parse errors result in validation failure (exitValidation)
	if exitCode != exitValidation {
		t.Errorf("expected exit code %d (validation failed), got %d", exitValidation, exitCode)
	}
}

func TestRunValidate_NoFile(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunValidate([]string{}, stdout, stderr)

	if exitCode != exitCommandError {
		t.Errorf("expected exit code %d, got %d", exitCommandError, exitCode)
	}

	if !strings.Contains(stderr.String(), "no files specified") {
		t.Errorf("expected 'no files specified' in stderr, got: %s", stderr.String())
	}
}

func TestRunValidate_JSONOutput(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunValidate([]string{"--json", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	// JSON output should contain valid field
	if !strings.Contains(stdout.String(), `"valid"`) {
		t.Errorf("expected JSON output with 'valid' field, got: %s", stdout.String())
	}
}

func TestRunValidate_MultipleFiles(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunValidate([]string{
		"../../../testdata/pics/minimal-device-pairing.pics",
		"../../../testdata/pics/minimal-device-pairing.pics",
	}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	// Should have two results
	if strings.Count(stdout.String(), "OK") != 2 {
		t.Errorf("expected two OK results, got: %s", stdout.String())
	}
}

func TestRunLint_Clean(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunLint([]string{"../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	// Clean means no errors or warnings (suggestions are OK)
	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}
}

func TestRunLint_NoFile(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunLint([]string{}, stdout, stderr)

	if exitCode != exitCommandError {
		t.Errorf("expected exit code %d, got %d", exitCommandError, exitCode)
	}
}

func TestRunLint_JSONOutput(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunLint([]string{"--json", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	if !strings.Contains(stdout.String(), `"file"`) {
		t.Errorf("expected JSON output with 'file' field, got: %s", stdout.String())
	}
}

func TestRunLint_Verbose(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunLint([]string{"--verbose", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	// Verbose should show suggestions
	if !strings.Contains(stdout.String(), "SUGGESTION") {
		t.Errorf("expected SUGGESTION in verbose output, got: %s", stdout.String())
	}
}

func TestRunShow_TextFormat(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunShow([]string{"../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "File:") {
		t.Errorf("expected 'File:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Format:") {
		t.Errorf("expected 'Format:' in output, got: %s", output)
	}
	if !strings.Contains(output, "Entries:") {
		t.Errorf("expected 'Entries:' in output, got: %s", output)
	}
}

func TestRunShow_JSONFormat(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunShow([]string{"--format", "json", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	if !strings.Contains(stdout.String(), `"file"`) {
		t.Errorf("expected JSON with 'file' field, got: %s", stdout.String())
	}
}

func TestRunShow_YAMLFormat(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunShow([]string{"--format", "yaml", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	// YAML output should contain file: key
	if !strings.Contains(stdout.String(), "file:") {
		t.Errorf("expected YAML with 'file:' field, got: %s", stdout.String())
	}
}

func TestRunShow_NoFile(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunShow([]string{}, stdout, stderr)

	if exitCode != exitCommandError {
		t.Errorf("expected exit code %d, got %d", exitCommandError, exitCode)
	}
}

func TestRunShow_GroupByFeature(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunShow([]string{"--group-by", "feature", "../../../testdata/pics/minimal-device-pairing.pics"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	// Grouped output should have bracket notation
	if !strings.Contains(stdout.String(), "[") {
		t.Errorf("expected grouped output with brackets, got: %s", stdout.String())
	}
}

func TestRunConvert_ToStdout(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunConvert([]string{"../../../testdata/pics/ev-charger.yaml"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "# MASH PICS File") {
		t.Errorf("expected header comment in output, got: %s", output)
	}
	if !strings.Contains(output, "MASH.S.") {
		t.Errorf("expected MASH.S. codes in output, got: %s", output)
	}
}

func TestRunConvert_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.pics")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunConvert([]string{"-o", outputFile, "../../../testdata/pics/ev-charger.yaml"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
		t.Logf("stderr: %s", stderr.String())
	}

	// Check file was created
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "# MASH PICS File") {
		t.Errorf("expected header in output file, got: %s", string(content))
	}
}

func TestRunConvert_NoInput(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunConvert([]string{}, stdout, stderr)

	if exitCode != exitCommandError {
		t.Errorf("expected exit code %d, got %d", exitCommandError, exitCode)
	}
}

func TestRunConvert_ClientSide(t *testing.T) {
	// The --side flag only affects legacy D.* codes during parsing.
	// Since ev-charger.yaml now has explicit MASH.S.* codes, the --side flag
	// doesn't change the output. This test verifies the flag is accepted.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := RunConvert([]string{"--side", "C", "../../../testdata/pics/ev-charger.yaml"}, stdout, stderr)

	if exitCode != exitSuccess {
		t.Errorf("expected exit code %d, got %d", exitSuccess, exitCode)
	}

	output := stdout.String()
	// File has explicit MASH.S.* codes, so output still contains MASH.S
	if !strings.Contains(output, "MASH.S.") {
		t.Errorf("expected MASH.S. codes in output, got: %s", output)
	}
}

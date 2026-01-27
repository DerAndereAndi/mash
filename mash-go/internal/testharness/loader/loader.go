package loader

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseTestCase parses a test case from YAML bytes.
func ParseTestCase(data []byte) (*TestCase, error) {
	var tc TestCase
	if err := yaml.Unmarshal(data, &tc); err != nil {
		return nil, &LoadError{
			Message: "failed to parse YAML",
			Cause:   err,
		}
	}

	// Validate required fields
	if tc.ID == "" {
		return nil, &LoadError{
			Message: "test case ID is required",
		}
	}

	if len(tc.Steps) == 0 {
		return nil, &LoadError{
			Message: "test case must have at least one step",
		}
	}

	return &tc, nil
}

// LoadTestCase loads a test case from a file.
func LoadTestCase(path string) (*TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &LoadError{
			File:    path,
			Message: "failed to read file",
			Cause:   err,
		}
	}

	tc, err := ParseTestCase(data)
	if err != nil {
		if le, ok := err.(*LoadError); ok {
			le.File = path
			return nil, le
		}
		return nil, &LoadError{
			File:    path,
			Message: err.Error(),
		}
	}

	return tc, nil
}

// LoadDirectory loads all test cases from a directory.
// Only files with .yaml or .yml extensions are loaded.
func LoadDirectory(dir string) ([]*TestCase, error) {
	var cases []*TestCase

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &LoadError{
			File:    dir,
			Message: "failed to read directory",
			Cause:   err,
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		tc, err := LoadTestCase(path)
		if err != nil {
			return nil, err
		}

		cases = append(cases, tc)
	}

	return cases, nil
}

// LoadDirectoryRecursive loads all test cases from a directory and subdirectories.
func LoadDirectoryRecursive(dir string) ([]*TestCase, error) {
	var cases []*TestCase

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		tc, err := LoadTestCase(path)
		if err != nil {
			return err
		}

		cases = append(cases, tc)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return cases, nil
}

// ParsePICS parses a PICS file from bytes.
// Auto-detects format: YAML (with device/items sections) or legacy key=value format.
func ParsePICS(data []byte) (*PICSFile, error) {
	// Auto-detect format: if it looks like YAML (has "items:" or "device:"), parse as YAML
	if isYAMLFormat(data) {
		return parsePICSYAML(data)
	}
	return parsePICSKeyValue(data)
}

// isYAMLFormat detects if the data appears to be YAML format.
// Returns true if the content contains YAML structure markers like "items:" or "device:".
func isYAMLFormat(data []byte) bool {
	content := string(data)
	// Look for YAML section markers (ignoring comments)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Check for YAML section markers at root level (no indentation after trim)
		if strings.HasPrefix(trimmed, "items:") || strings.HasPrefix(trimmed, "device:") {
			return true
		}
		// If first non-comment line contains "=" it's likely key=value format
		if strings.Contains(trimmed, "=") && !strings.Contains(trimmed, ":") {
			return false
		}
	}
	return false
}

// parsePICSYAML parses PICS data in YAML format.
func parsePICSYAML(data []byte) (*PICSFile, error) {
	var yamlFile picsYAMLFile
	if err := yaml.Unmarshal(data, &yamlFile); err != nil {
		return nil, &LoadError{
			Message: "failed to parse YAML PICS",
			Cause:   err,
		}
	}

	pf := &PICSFile{
		Device: yamlFile.Device,
		Items:  yamlFile.Items,
	}

	// Ensure Items map is initialized
	if pf.Items == nil {
		pf.Items = make(map[string]interface{})
	}

	// Normalize integer types (YAML unmarshals numbers as int, but we want consistency)
	for key, value := range pf.Items {
		// YAML unmarshals integers directly as int, keep them as-is
		// Float64 values stay as float64
		// Booleans and strings stay as-is
		_ = key
		_ = value
	}

	return pf, nil
}

// parsePICSKeyValue parses PICS data in legacy key=value format.
func parsePICSKeyValue(data []byte) (*PICSFile, error) {
	pf := &PICSFile{
		Items: make(map[string]interface{}),
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, &LoadError{
				Line:    lineNum,
				Message: fmt.Sprintf("invalid PICS line: %s", line),
			}
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Try to parse as boolean
		if value == "true" {
			pf.Items[key] = true
			continue
		}
		if value == "false" {
			pf.Items[key] = false
			continue
		}

		// Try to parse as integer
		if i, err := strconv.Atoi(value); err == nil {
			pf.Items[key] = i
			continue
		}

		// Try to parse as float
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			pf.Items[key] = f
			continue
		}

		// Store as string
		pf.Items[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, &LoadError{
			Message: "failed to read PICS data",
			Cause:   err,
		}
	}

	return pf, nil
}

// LoadPICS loads a PICS file from disk.
func LoadPICS(path string) (*PICSFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &LoadError{
			File:    path,
			Message: "failed to read PICS file",
			Cause:   err,
		}
	}

	pf, err := ParsePICS(data)
	if err != nil {
		if le, ok := err.(*LoadError); ok {
			le.File = path
			return nil, le
		}
		return nil, err
	}

	// Use filename as name if not set
	if pf.Name == "" {
		pf.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return pf, nil
}

// CheckPICSRequirements checks if a PICS file satisfies the given requirements.
// Returns true if all requirements are met.
func CheckPICSRequirements(pf *PICSFile, requirements []string) bool {
	for _, req := range requirements {
		value, exists := pf.Items[req]
		if !exists {
			return false
		}

		// For boolean items, must be true
		if b, ok := value.(bool); ok && !b {
			return false
		}

		// For non-boolean items, existence is sufficient
	}
	return true
}

// FilterTestCases returns test cases that match the given PICS file.
func FilterTestCases(cases []*TestCase, pics *PICSFile) []*TestCase {
	var result []*TestCase
	for _, tc := range cases {
		if CheckPICSRequirements(pics, tc.PICSRequirements) {
			result = append(result, tc)
		}
	}
	return result
}

// ValidatePICS validates a PICS file for conformance rules.
// Returns a list of validation errors (which may be empty if valid).
func ValidatePICS(pf *PICSFile) []*ValidationError {
	if pf == nil {
		return nil
	}

	var errors []*ValidationError

	// Rule: V2X requires EMOB base support
	if getBool(pf.Items, "D.EMOB.V2X") && !getBool(pf.Items, "D.EMOB.BASE") {
		errors = append(errors, &ValidationError{
			Field:   "D.EMOB.V2X",
			Message: "V2X support requires D.EMOB.BASE to be true",
			Level:   ValidationLevelError,
		})
	}

	// Rule: ChargingSession requires EMOB base support
	if getBool(pf.Items, "D.CHARGE.SESSION") && !getBool(pf.Items, "D.EMOB.BASE") {
		errors = append(errors, &ValidationError{
			Field:   "D.CHARGE.SESSION",
			Message: "Charging session support requires D.EMOB.BASE to be true",
			Level:   ValidationLevelError,
		})
	}

	// Rule: Electrical feature requires phase count
	if getBool(pf.Items, "D.ELEC.PRESENT") && !hasKey(pf.Items, "D.ELEC.PHASES") {
		errors = append(errors, &ValidationError{
			Field:   "D.ELEC.PHASES",
			Message: "Electrical feature (D.ELEC.PRESENT) requires D.ELEC.PHASES",
			Level:   ValidationLevelError,
		})
	}

	// Rule: Phase count must be 1-3
	if phases, ok := getInt(pf.Items, "D.ELEC.PHASES"); ok {
		if phases < 1 || phases > 3 {
			errors = append(errors, &ValidationError{
				Field:   "D.ELEC.PHASES",
				Message: fmt.Sprintf("D.ELEC.PHASES must be 1-3, got %d", phases),
				Level:   ValidationLevelError,
			})
		}
	}

	// Rule: Max zones must be 1-5
	if zones, ok := getInt(pf.Items, "D.ZONE.MAX"); ok {
		if zones < 1 || zones > 5 {
			errors = append(errors, &ValidationError{
				Field:   "D.ZONE.MAX",
				Message: fmt.Sprintf("D.ZONE.MAX must be 1-5, got %d", zones),
				Level:   ValidationLevelError,
			})
		}
	}

	return errors
}

// getBool retrieves a boolean value from the items map.
func getBool(items map[string]interface{}, key string) bool {
	if v, ok := items[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// hasKey checks if a key exists in the items map.
func hasKey(items map[string]interface{}, key string) bool {
	_, ok := items[key]
	return ok
}

// getInt retrieves an integer value from the items map.
// Handles both int and float64 (from YAML parsing).
func getInt(items map[string]interface{}, key string) (int, bool) {
	v, ok := items[key]
	if !ok {
		return 0, false
	}

	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

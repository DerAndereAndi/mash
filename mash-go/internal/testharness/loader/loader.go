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
// PICS files use a simple key=value format with # comments.
func ParsePICS(data []byte) (*PICSFile, error) {
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

package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
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
// Uses pkg/pics for parsing.
func ParsePICS(data []byte) (*PICSFile, error) {
	p, err := pics.ParseBytes(data)
	if err != nil {
		return nil, &LoadError{
			Message: "failed to parse PICS",
			Cause:   err,
		}
	}
	return picsToFile(p), nil
}

// LoadPICS loads a PICS file from disk.
func LoadPICS(path string) (*PICSFile, error) {
	p, err := pics.ParseFile(path)
	if err != nil {
		return nil, &LoadError{
			File:    path,
			Message: "failed to read PICS file",
			Cause:   err,
		}
	}

	pf := picsToFile(p)

	// Use filename as name if not set
	if pf.Name == "" {
		pf.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	return pf, nil
}

// picsToFile converts a pics.PICS to a PICSFile for backward compatibility.
func picsToFile(p *pics.PICS) *PICSFile {
	pf := &PICSFile{
		Items: make(map[string]interface{}),
	}

	// Copy device metadata
	if p.Device != nil {
		pf.Device = PICSDevice{
			Vendor:  p.Device.Vendor,
			Product: p.Device.Product,
			Model:   p.Device.Model,
			Version: p.Device.Version,
		}
	}

	// Copy entries to Items map
	for _, entry := range p.Entries {
		code := entry.Code.String()
		// Convert Value to appropriate type
		if entry.Value.IsBool() {
			pf.Items[code] = entry.Value.Bool
		} else if entry.Value.Int != 0 || entry.Value.Raw == "0" {
			pf.Items[code] = int(entry.Value.Int)
		} else {
			pf.Items[code] = entry.Value.String
		}
	}

	return pf
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
func FilterTestCases(cases []*TestCase, picsFile *PICSFile) []*TestCase {
	var result []*TestCase
	for _, tc := range cases {
		if CheckPICSRequirements(picsFile, tc.PICSRequirements) {
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
	if getBool(pf.Items, "MASH.S.CTRL.F0A") && !getBool(pf.Items, "MASH.S.CTRL.F03") {
		errors = append(errors, &ValidationError{
			Field:   "MASH.S.CTRL.F0A",
			Message: "V2X support (F0A) requires EMOB (F03) to be enabled",
			Level:   ValidationLevelError,
		})
	}

	// Rule: ChargingSession requires CHRG feature
	if getBool(pf.Items, "MASH.S.CHRG.SESSION") && !getBool(pf.Items, "MASH.S.CHRG") {
		errors = append(errors, &ValidationError{
			Field:   "MASH.S.CHRG.SESSION",
			Message: "Charging session support requires MASH.S.CHRG feature",
			Level:   ValidationLevelError,
		})
	}

	// Rule: Electrical feature requires phase count
	if getBool(pf.Items, "MASH.S.ELEC") && !hasKey(pf.Items, "MASH.S.ELEC.A01") {
		errors = append(errors, &ValidationError{
			Field:   "MASH.S.ELEC.A01",
			Message: "Electrical feature requires phaseCount attribute (A01)",
			Level:   ValidationLevelError,
		})
	}

	// Rule: Phase count must be 1-3
	if phases, ok := getInt(pf.Items, "MASH.S.ELEC.A01"); ok {
		if phases < 1 || phases > 3 {
			errors = append(errors, &ValidationError{
				Field:   "MASH.S.ELEC.A01",
				Message: fmt.Sprintf("phaseCount must be 1-3, got %d", phases),
				Level:   ValidationLevelError,
			})
		}
	}

	// Rule: Max zones must be 1-5
	if zones, ok := getInt(pf.Items, "MASH.S.ZONE.MAX"); ok {
		if zones < 1 || zones > 5 {
			errors = append(errors, &ValidationError{
				Field:   "MASH.S.ZONE.MAX",
				Message: fmt.Sprintf("max zones must be 1-5, got %d", zones),
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
		// Also check for integer 1
		if i, ok := v.(int); ok {
			return i != 0
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

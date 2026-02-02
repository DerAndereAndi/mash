// Command mash-specaudit cross-references TC-* IDs from behavior specs
// against YAML test cases and reports coverage gaps.
//
// Usage:
//
//	mash-specaudit [flags]
//
// Flags:
//
//	-root string    Path to repository root (default: auto-detect)
//	-json           Output as JSON instead of text
//
// The tool scans:
//   - docs/testing/behavior/*.md for behavior spec TC-* IDs
//   - mash-go/testdata/cases/*.yaml for implemented YAML test case IDs
//   - docs/testing/test-matrix.md for test matrix TC-* IDs (cross-validation)
//
// Exit code is 0 if all behavior spec TCs have YAML tests, 1 if gaps exist.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// tcPattern matches TC-* IDs in markdown (table rows and section headers).
// It excludes wildcard group headers like "TC-TLS-VERSION-*".
var tcPattern = regexp.MustCompile(`TC-[A-Z][A-Z0-9]*(?:-[A-Z][A-Z0-9]*)*-(\d+)`)

// yamlIDPattern matches the id: field in YAML test case files.
var yamlIDPattern = regexp.MustCompile(`^\s*id:\s*(TC-\S+)`)

// specResult holds TC IDs parsed from a single behavior spec file.
type specResult struct {
	File string   `json:"file"`
	IDs  []string `json:"ids"`
}

// report is the JSON output structure.
type report struct {
	BehaviorSpecs []specFileReport `json:"behavior_specs"`
	Summary       summary          `json:"summary"`
	YAMLOnly      []string         `json:"yaml_only"`
}

type specFileReport struct {
	File        string   `json:"file"`
	TotalTCs    int      `json:"total_tcs"`
	Implemented int      `json:"implemented"`
	Missing     []string `json:"missing,omitempty"`
}

type summary struct {
	TotalSpecTCs  int `json:"total_spec_tcs"`
	TotalYAMLTCs  int `json:"total_yaml_tcs"`
	Covered       int `json:"covered"`
	MissingYAML   int `json:"missing_yaml"`
	YAMLOnlyCount int `json:"yaml_only_count"`
}

func main() {
	var rootDir string
	var jsonOutput bool
	flag.StringVar(&rootDir, "root", "", "Path to repository root (default: auto-detect)")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.Parse()

	if rootDir == "" {
		var err error
		rootDir, err = detectRoot()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
	}

	behaviorDir := filepath.Join(rootDir, "docs", "testing", "behavior")
	yamlDir := filepath.Join(rootDir, "mash-go", "testdata", "cases")
	matrixFile := filepath.Join(rootDir, "docs", "testing", "test-matrix.md")

	// Parse all sources.
	specResults, err := parseBehaviorSpecs(behaviorDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing behavior specs: %v\n", err)
		os.Exit(2)
	}

	yamlIDs, err := parseYAMLTestCases(yamlDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing YAML test cases: %v\n", err)
		os.Exit(2)
	}

	matrixIDs, err := parseMatrixFile(matrixFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not parse test matrix: %v\n", err)
		// Non-fatal: matrix is for cross-validation only.
	}

	// Normalize all IDs for matching.
	yamlNorm := normalizeSet(yamlIDs)
	_ = normalizeSet(matrixIDs) // available for future cross-validation

	// Build report.
	allSpecNorm := make(map[string]bool)
	var specReports []specFileReport
	totalCovered := 0

	for _, sr := range specResults {
		var missing []string
		implemented := 0
		for _, id := range sr.IDs {
			norm := normalizeID(id)
			allSpecNorm[norm] = true
			if yamlNorm[norm] {
				implemented++
			} else {
				missing = append(missing, id)
			}
		}
		totalCovered += implemented
		specReports = append(specReports, specFileReport{
			File:        relPath(rootDir, sr.File),
			TotalTCs:    len(sr.IDs),
			Implemented: implemented,
			Missing:     missing,
		})
	}

	// Sort spec reports: most missing first, then alphabetical.
	sort.Slice(specReports, func(i, j int) bool {
		mi, mj := len(specReports[i].Missing), len(specReports[j].Missing)
		if mi != mj {
			return mi > mj
		}
		return specReports[i].File < specReports[j].File
	})

	// Find YAML-only IDs (implemented but no spec).
	var yamlOnly []string
	for id := range yamlNorm {
		if !allSpecNorm[id] {
			yamlOnly = append(yamlOnly, id)
		}
	}
	sort.Strings(yamlOnly)

	totalSpecTCs := len(allSpecNorm)
	totalYAMLTCs := len(yamlNorm)

	rpt := report{
		BehaviorSpecs: specReports,
		Summary: summary{
			TotalSpecTCs:  totalSpecTCs,
			TotalYAMLTCs:  totalYAMLTCs,
			Covered:       totalCovered,
			MissingYAML:   totalSpecTCs - totalCovered,
			YAMLOnlyCount: len(yamlOnly),
		},
		YAMLOnly: yamlOnly,
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rpt); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
			os.Exit(2)
		}
	} else {
		printTextReport(rpt)
	}

	if rpt.Summary.MissingYAML > 0 {
		os.Exit(1)
	}
}

// detectRoot walks up from the current directory looking for the repo root.
func detectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		// Check for docs/testing/behavior/ as the distinguishing marker.
		if _, err := os.Stat(filepath.Join(dir, "docs", "testing", "behavior")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root (no docs/testing/behavior/ found)")
		}
		dir = parent
	}
}

// parseBehaviorSpecs reads all .md files from the behavior directory and
// extracts unique TC-* IDs from each.
func parseBehaviorSpecs(dir string) ([]specResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}

	var results []specResult
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		ids := extractTCIDs(string(data))
		if len(ids) > 0 {
			results = append(results, specResult{File: path, IDs: ids})
		}
	}
	return results, nil
}

// extractTCIDs extracts unique TC-* IDs from markdown content,
// excluding wildcard group headers.
func extractTCIDs(content string) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, match := range tcPattern.FindAllString(content, -1) {
		// Skip wildcard group headers like "TC-TLS-VERSION-*".
		if strings.Contains(match, "*") {
			continue
		}
		if !seen[match] {
			seen[match] = true
			ids = append(ids, match)
		}
	}
	sort.Strings(ids)
	return ids
}

// parseYAMLTestCases reads all .yaml files (recursively) from the test cases
// directory and extracts TC-* IDs from id: fields.
func parseYAMLTestCases(dir string) ([]string, error) {
	seen := make(map[string]bool)
	var ids []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if m := yamlIDPattern.FindStringSubmatch(line); m != nil {
				id := strings.TrimSpace(m[1])
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return ids, nil
}

// parseMatrixFile extracts TC-* IDs from the test matrix markdown file.
func parseMatrixFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return extractTCIDs(string(data)), nil
}

// normalizeID normalizes a TC ID by zero-padding the trailing numeric suffix
// to 3 digits and uppercasing. This allows TC-PASE-1 to match TC-PASE-001.
func normalizeID(id string) string {
	id = strings.ToUpper(strings.TrimSpace(id))
	loc := tcPattern.FindStringSubmatchIndex(id)
	if loc == nil {
		return id
	}
	// loc[2]:loc[3] is the capture group (the numeric suffix).
	numStr := id[loc[2]:loc[3]]
	prefix := id[:loc[2]]
	// Zero-pad to at least 3 digits.
	for len(numStr) < 3 {
		numStr = "0" + numStr
	}
	return prefix + numStr
}

// normalizeSet builds a set of normalized IDs.
func normalizeSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[normalizeID(id)] = true
	}
	return m
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}

func printTextReport(rpt report) {
	fmt.Println("=== MASH Spec-Test Gap Report ===")
	fmt.Println()

	// Specs with gaps.
	fmt.Println("--- Behavior specs with missing YAML test cases ---")
	fmt.Println()
	anyMissing := false
	for _, sr := range rpt.BehaviorSpecs {
		if len(sr.Missing) == 0 {
			continue
		}
		anyMissing = true
		fmt.Printf("%s (%d TCs, %d implemented):\n", sr.File, sr.TotalTCs, sr.Implemented)
		fmt.Printf("  Missing: %s\n", strings.Join(sr.Missing, ", "))
		fmt.Println()
	}
	if !anyMissing {
		fmt.Println("  (none -- all behavior spec TCs have YAML tests)")
		fmt.Println()
	}

	// Fully covered specs.
	fmt.Println("--- Fully covered behavior specs ---")
	fmt.Println()
	anyCovered := false
	for _, sr := range rpt.BehaviorSpecs {
		if len(sr.Missing) > 0 {
			continue
		}
		anyCovered = true
		fmt.Printf("  %s (%d TCs)\n", sr.File, sr.TotalTCs)
	}
	if !anyCovered {
		fmt.Println("  (none)")
	}
	fmt.Println()

	// Summary.
	fmt.Println("--- Summary ---")
	pct := 0
	if rpt.Summary.TotalSpecTCs > 0 {
		pct = rpt.Summary.Covered * 100 / rpt.Summary.TotalSpecTCs
	}
	fmt.Printf("Total behavior spec TCs: %d\n", rpt.Summary.TotalSpecTCs)
	fmt.Printf("Total YAML test cases:   %d\n", rpt.Summary.TotalYAMLTCs)
	fmt.Printf("Covered:                 %d (%d%%)\n", rpt.Summary.Covered, pct)
	fmt.Printf("Missing YAML tests:      %d\n", rpt.Summary.MissingYAML)
	fmt.Printf("YAML-only (no spec):     %d\n", rpt.Summary.YAMLOnlyCount)
	fmt.Println()

	// YAML-only.
	if len(rpt.YAMLOnly) > 0 {
		fmt.Println("--- YAML tests with no behavior spec ---")
		fmt.Printf("  %s\n", strings.Join(rpt.YAMLOnly, ", "))
		fmt.Println()
	}
}

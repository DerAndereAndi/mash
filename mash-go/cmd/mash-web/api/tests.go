package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestsAPI handles test case listing endpoints.
type TestsAPI struct {
	testDir string

	// Cache for loaded test cases
	mu         sync.RWMutex
	sets       []TestSet
	cases      []*loader.TestCase
	caseByID   map[string]*loader.TestCase
	caseFileID map[string]string // maps test ID to set ID (filename without extension)
}

// NewTestsAPI creates a new tests API handler.
func NewTestsAPI(testDir string) *TestsAPI {
	return &TestsAPI{
		testDir:    testDir,
		caseByID:   make(map[string]*loader.TestCase),
		caseFileID: make(map[string]string),
	}
}

// loadCases loads and caches test cases from the test directory, grouped by file.
func (t *TestsAPI) loadCases() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Return cached if already loaded
	if t.cases != nil {
		return nil
	}

	entries, err := os.ReadDir(t.testDir)
	if err != nil {
		return err
	}

	var allCases []*loader.TestCase
	var sets []TestSet

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(t.testDir, name)
		fileCases, err := loader.LoadTestCases(path)
		if err != nil {
			continue // Skip files with errors
		}

		if len(fileCases) == 0 {
			continue
		}

		// Create a test set from this file
		setID := strings.TrimSuffix(name, ext)
		set := TestSet{
			ID:        setID,
			Name:      formatSetName(setID),
			FileName:  name,
			TestCount: len(fileCases),
			Tests:     make([]TestCase, 0, len(fileCases)),
		}

		// Collect tags and build description from first test
		tagSet := make(map[string]bool)
		if len(fileCases) > 0 && fileCases[0].Description != "" {
			// Use first line of first test's description as set description
			desc := fileCases[0].Description
			if idx := strings.Index(desc, "\n"); idx > 0 {
				desc = desc[:idx]
			}
			set.Description = strings.TrimSpace(desc)
		}

		for _, tc := range fileCases {
			apiTC := toAPITestCase(tc)
			apiTC.SetID = setID
			set.Tests = append(set.Tests, apiTC)

			for _, tag := range tc.Tags {
				tagSet[tag] = true
			}
		}

		// Convert tag set to slice
		for tag := range tagSet {
			set.Tags = append(set.Tags, tag)
		}
		sort.Strings(set.Tags)

		sets = append(sets, set)
		allCases = append(allCases, fileCases...)
	}

	// Sort sets by name
	sort.Slice(sets, func(i, j int) bool {
		return sets[i].Name < sets[j].Name
	})

	t.sets = sets
	t.cases = allCases
	t.caseByID = make(map[string]*loader.TestCase, len(allCases))
	t.caseFileID = make(map[string]string, len(allCases))
	for _, tc := range allCases {
		t.caseByID[tc.ID] = tc
	}
	// Build caseFileID map from sets
	for _, set := range sets {
		for _, tc := range set.Tests {
			t.caseFileID[tc.ID] = set.ID
		}
	}

	return nil
}

// formatSetName converts a file name like "commissioning-pase" to "Commissioning PASE"
func formatSetName(id string) string {
	// Replace hyphens with spaces
	name := strings.ReplaceAll(id, "-", " ")
	// Remove common suffixes
	name = strings.TrimSuffix(name, " tests")
	name = strings.TrimSuffix(name, " test")
	// Title case each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			// Handle acronyms (all caps words 2-4 chars)
			upper := strings.ToUpper(word)
			if len(word) <= 4 && isAcronym(word) {
				words[i] = upper
			} else {
				words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
			}
		}
	}
	return strings.Join(words, " ")
}

// isAcronym checks if a word looks like an acronym
func isAcronym(word string) bool {
	acronyms := map[string]bool{
		"tls": true, "pase": true, "cert": true, "qr": true,
		"mdns": true, "ipv6": true, "cbor": true, "e2e": true,
		"evc": true, "gpl": true, "mpd": true, "cob": true,
		"ohpcf": true, "floa": true, "itpcm": true, "podf": true,
		"poen": true, "tout": true, "d2d": true, "api": true,
	}
	return acronyms[strings.ToLower(word)]
}

// ReloadCases forces a reload of test cases from disk.
func (t *TestsAPI) ReloadCases() error {
	t.mu.Lock()
	t.cases = nil
	t.sets = nil
	t.caseByID = make(map[string]*loader.TestCase)
	t.mu.Unlock()

	return t.loadCases()
}

// Count returns the number of test cases.
func (t *TestsAPI) Count() (int, error) {
	if err := t.loadCases(); err != nil {
		return 0, err
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.cases), nil
}

// Reload forces a reload of test cases from disk.
func (t *TestsAPI) Reload() error {
	t.mu.Lock()
	t.cases = nil
	t.sets = nil
	t.caseByID = make(map[string]*loader.TestCase)
	t.caseFileID = make(map[string]string)
	t.mu.Unlock()

	return t.loadCases()
}

// HandleReload handles POST /api/v1/tests/reload.
func (t *TestsAPI) HandleReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := t.Reload(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to reload test cases", err.Error())
		return
	}

	t.mu.RLock()
	count := len(t.cases)
	setCount := len(t.sets)
	t.mu.RUnlock()

	resp := map[string]any{
		"status":    "reloaded",
		"tests":     count,
		"test_sets": setCount,
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

// HandleList handles GET /api/v1/tests.
func (t *TestsAPI) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := t.loadCases(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load test cases", err.Error())
		return
	}

	pattern := r.URL.Query().Get("pattern")
	grouped := r.URL.Query().Get("grouped") == "true"

	t.mu.RLock()
	defer t.mu.RUnlock()

	if grouped {
		// Return grouped by test set
		var filteredSets []TestSet
		totalTests := 0

		for _, set := range t.sets {
			var filteredTests []TestCase
			for _, tc := range set.Tests {
				if pattern != "" && !matchPattern(tc.ID, pattern) && !matchPattern(tc.Name, pattern) {
					continue
				}
				filteredTests = append(filteredTests, tc)
			}

			if len(filteredTests) > 0 {
				filteredSet := set
				filteredSet.Tests = filteredTests
				filteredSet.TestCount = len(filteredTests)
				filteredSets = append(filteredSets, filteredSet)
				totalTests += len(filteredTests)
			}
		}

		resp := TestSetsResponse{
			Sets:  filteredSets,
			Total: totalTests,
		}
		writeJSONResponse(w, http.StatusOK, resp)
		return
	}

	// Return flat list (original behavior)
	var tests []TestCase
	for _, tc := range t.cases {
		if pattern != "" && !matchPattern(tc.ID, pattern) && !matchPattern(tc.Name, pattern) {
			continue
		}
		tests = append(tests, toAPITestCase(tc))
	}

	resp := TestListResponse{
		Tests: tests,
		Total: len(tests),
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// HandleGet handles GET /api/v1/tests/:id.
func (t *TestsAPI) HandleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/v1/tests/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tests/")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Test ID required", "")
		return
	}

	if err := t.loadCases(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load test cases", err.Error())
		return
	}

	t.mu.RLock()
	tc, ok := t.caseByID[id]
	t.mu.RUnlock()

	if !ok {
		writeJSONError(w, http.StatusNotFound, "Test case not found", id)
		return
	}

	writeJSONResponse(w, http.StatusOK, toAPITestCase(tc))
}

// HandleGetYAML handles GET /api/v1/tests/:id/yaml - returns raw YAML source.
func (t *TestsAPI) HandleGetYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := t.loadCases(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load test cases", err.Error())
		return
	}

	// Extract test ID from URL (format: /api/v1/tests/:id/yaml)
	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/v1/tests/")
	path = strings.TrimSuffix(path, "/yaml")
	id := path

	t.mu.RLock()
	_, ok := t.caseByID[id]
	setID := t.caseFileID[id]
	t.mu.RUnlock()

	if !ok || setID == "" {
		writeJSONError(w, http.StatusNotFound, "Test case not found", id)
		return
	}

	// Find the file for this test
	var filename string
	t.mu.RLock()
	for _, set := range t.sets {
		if set.ID == setID {
			filename = set.FileName
			break
		}
	}
	t.mu.RUnlock()

	if filename == "" {
		writeJSONError(w, http.StatusNotFound, "Test file not found", setID)
		return
	}

	// Read and extract YAML for this specific test
	filePath := filepath.Join(t.testDir, filename)
	yaml, err := extractTestYAML(filePath, id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to read test YAML", err.Error())
		return
	}

	resp := map[string]string{
		"test_id":  id,
		"filename": filename,
		"yaml":     yaml,
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

// extractTestYAML reads a YAML file and extracts the document for a specific test ID.
func extractTestYAML(filePath, testID string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// Split by YAML document separator
	docs := strings.Split(string(content), "\n---")

	for i, doc := range docs {
		// Add back the separator for all but the first document
		if i > 0 {
			doc = "---" + doc
		}
		doc = strings.TrimSpace(doc)

		// Check if this document contains our test ID
		// Look for "id: TEST_ID" at the start of a line
		lines := strings.Split(doc, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "id:") {
				idValue := strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
				// Remove quotes if present
				idValue = strings.Trim(idValue, "\"'")
				if idValue == testID {
					return doc, nil
				}
				break // Found id line, but not our test, move to next doc
			}
		}
	}

	return "", fmt.Errorf("test ID %s not found in file", testID)
}

// HandleSets handles GET /api/v1/testsets.
func (t *TestsAPI) HandleSets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := t.loadCases(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load test cases", err.Error())
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	// Return sets without individual tests (summary only)
	sets := make([]TestSet, len(t.sets))
	for i, s := range t.sets {
		sets[i] = TestSet{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			FileName:    s.FileName,
			TestCount:   s.TestCount,
			Tags:        s.Tags,
			// Tests omitted for summary
		}
	}

	total := 0
	for _, s := range sets {
		total += s.TestCount
	}

	resp := TestSetsResponse{
		Sets:  sets,
		Total: total,
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// HandleSetByID handles GET /api/v1/testsets/:id.
func (t *TestsAPI) HandleSetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/v1/testsets/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/testsets/")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "Test set ID required", "")
		return
	}

	if err := t.loadCases(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to load test cases", err.Error())
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, set := range t.sets {
		if set.ID == id {
			writeJSONResponse(w, http.StatusOK, set)
			return
		}
	}

	writeJSONError(w, http.StatusNotFound, "Test set not found", id)
}

// toAPITestCase converts a loader.TestCase to an API TestCase.
func toAPITestCase(tc *loader.TestCase) TestCase {
	return TestCase{
		ID:               tc.ID,
		Name:             tc.Name,
		Description:      tc.Description,
		PICSRequirements: tc.PICSRequirements,
		Tags:             tc.Tags,
		StepCount:        len(tc.Steps),
		Timeout:          tc.Timeout,
		Skip:             tc.Skip,
		SkipReason:       tc.SkipReason,
	}
}

// matchPattern performs simple glob matching for test filtering.
func matchPattern(name, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}

	hasPrefix := len(pattern) > 0 && pattern[0] == '*'
	hasSuffix := len(pattern) > 0 && pattern[len(pattern)-1] == '*'

	if hasPrefix && hasSuffix && len(pattern) > 2 {
		// *foo* - contains
		return strings.Contains(name, pattern[1:len(pattern)-1])
	}
	if hasPrefix {
		// *foo - suffix match
		return strings.HasSuffix(name, pattern[1:])
	}
	if hasSuffix {
		// foo* - prefix match
		return strings.HasPrefix(name, pattern[:len(pattern)-1])
	}

	return name == pattern
}

// writeJSONResponse writes a JSON response.
func writeJSONResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message, details string) {
	resp := ErrorResponse{
		Error:   message,
		Details: details,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

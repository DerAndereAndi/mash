package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create test files
	testCases := []string{
		`
id: TC-READ-001
name: Basic read test
description: Tests basic read functionality
tags:
  - read
  - basic
steps:
  - action: connect
  - action: read
    params:
      endpoint: DEVICE_ROOT
      feature: DeviceInfo
`,
		`
id: TC-WRITE-001
name: Basic write test
description: Tests basic write functionality
tags:
  - write
steps:
  - action: connect
  - action: write
    params:
      endpoint: EV_CHARGER
      feature: EnergyControl
`,
	}

	for i, tc := range testCases {
		filename := filepath.Join(tmpDir, "test_"+string(rune('a'+i))+".yaml")
		if err := os.WriteFile(filename, []byte(tc), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return tmpDir
}

func TestTestsAPIHandleList(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests", nil)
	w := httptest.NewRecorder()

	api.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp TestListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 tests, got %d", resp.Total)
	}

	if len(resp.Tests) != 2 {
		t.Errorf("Expected 2 test items, got %d", len(resp.Tests))
	}
}

func TestTestsAPIHandleListWithPattern(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests?pattern=TC-READ-*", nil)
	w := httptest.NewRecorder()

	api.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp TestListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("Expected 1 test, got %d", resp.Total)
	}

	if len(resp.Tests) != 1 || resp.Tests[0].ID != "TC-READ-001" {
		t.Errorf("Expected TC-READ-001, got %v", resp.Tests)
	}
}

func TestTestsAPIHandleGet(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests/TC-READ-001", nil)
	w := httptest.NewRecorder()

	api.HandleGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp TestCase
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.ID != "TC-READ-001" {
		t.Errorf("Expected ID 'TC-READ-001', got %q", resp.ID)
	}

	if resp.Name != "Basic read test" {
		t.Errorf("Expected name 'Basic read test', got %q", resp.Name)
	}

	if resp.StepCount != 2 {
		t.Errorf("Expected 2 steps, got %d", resp.StepCount)
	}

	if len(resp.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(resp.Tags))
	}
}

func TestTestsAPIHandleGetNotFound(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests/TC-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	api.HandleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestTestsAPIHandleListMethodNotAllowed(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tests", nil)
	w := httptest.NewRecorder()

	api.HandleList(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestTestsAPICount(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	count, err := api.Count()
	if err != nil {
		t.Fatalf("Failed to get count: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2, got %d", count)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{"TC-READ-001", "TC-READ-*", true},
		{"TC-READ-001", "TC-WRITE-*", false},
		{"TC-READ-001", "*-001", true},
		{"TC-READ-001", "*READ*", true},
		{"TC-READ-001", "TC-READ-001", true},
		{"TC-READ-001", "TC-READ-002", false},
		{"TC-READ-001", "*", true},
		{"TC-READ-001", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.pattern, func(t *testing.T) {
			got := matchPattern(tt.name, tt.pattern)
			if got != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.name, tt.pattern, got, tt.expected)
			}
		})
	}
}

func TestTestsAPIHandleListGrouped(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests?grouped=true", nil)
	w := httptest.NewRecorder()

	api.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// When grouped=true, response is TestSetsResponse
	var resp TestSetsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("Expected 2 tests total, got %d", resp.Total)
	}

	if len(resp.Sets) == 0 {
		t.Error("Expected sets to be populated when grouped=true")
	}

	// Each test file should become a set
	if len(resp.Sets) != 2 {
		t.Errorf("Expected 2 sets (one per file), got %d", len(resp.Sets))
	}
}

func TestTestsAPIHandleSets(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/testsets", nil)
	w := httptest.NewRecorder()

	api.HandleSets(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp TestSetsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Sets) != 2 {
		t.Errorf("Expected 2 sets, got %d", len(resp.Sets))
	}

	// HandleSets returns summary only (no tests), just test count
	for _, set := range resp.Sets {
		if set.TestCount == 0 {
			t.Errorf("Set %q has 0 tests", set.ID)
		}
		// Tests array is empty in summary view
		if len(set.Tests) != 0 {
			t.Errorf("Set %q: expected empty Tests array in summary, got %d", set.ID, len(set.Tests))
		}
	}
}

func TestTestsAPIHandleSetByID(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	// First get the list of sets to find a valid ID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/testsets", nil)
	w := httptest.NewRecorder()
	api.HandleSets(w, req)

	var setsResp TestSetsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &setsResp); err != nil {
		t.Fatalf("Failed to parse sets response: %v", err)
	}

	if len(setsResp.Sets) == 0 {
		t.Fatal("No sets found")
	}

	setID := setsResp.Sets[0].ID

	// Now get the specific set
	req = httptest.NewRequest(http.MethodGet, "/api/v1/testsets/"+setID, nil)
	w = httptest.NewRecorder()

	api.HandleSetByID(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var set TestSet
	if err := json.Unmarshal(w.Body.Bytes(), &set); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if set.ID != setID {
		t.Errorf("Expected set ID %q, got %q", setID, set.ID)
	}

	if set.TestCount == 0 {
		t.Error("Expected set to have tests")
	}
}

func TestTestsAPIHandleSetByIDNotFound(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/testsets/nonexistent-set", nil)
	w := httptest.NewRecorder()

	api.HandleSetByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestTestsAPIHandleSetsMethodNotAllowed(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/testsets", nil)
	w := httptest.NewRecorder()

	api.HandleSets(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestTestsAPIHandleGetYAML(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	// First load tests
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests", nil)
	w := httptest.NewRecorder()
	api.HandleList(w, req)

	// Now get YAML for a specific test
	req = httptest.NewRequest(http.MethodGet, "/api/v1/tests/TC-READ-001/yaml", nil)
	w = httptest.NewRecorder()

	api.HandleGetYAML(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["test_id"] != "TC-READ-001" {
		t.Errorf("Expected test_id 'TC-READ-001', got %q", resp["test_id"])
	}

	if resp["yaml"] == "" {
		t.Error("Expected yaml content, got empty string")
	}

	// Verify YAML contains expected content
	if !strings.Contains(resp["yaml"], "id: TC-READ-001") {
		t.Error("YAML should contain 'id: TC-READ-001'")
	}

	if !strings.Contains(resp["yaml"], "Basic read test") {
		t.Error("YAML should contain test name")
	}
}

func TestTestsAPIHandleGetYAMLNotFound(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests/TC-NONEXISTENT/yaml", nil)
	w := httptest.NewRecorder()

	api.HandleGetYAML(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestTestsAPIHandleReload(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	// First load to populate cache
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests", nil)
	w := httptest.NewRecorder()
	api.HandleList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Initial load failed: %d", w.Code)
	}

	// Now reload
	req = httptest.NewRequest(http.MethodPost, "/api/v1/tests/reload", nil)
	w = httptest.NewRecorder()

	api.HandleReload(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "reloaded" {
		t.Errorf("Expected status 'reloaded', got %v", resp["status"])
	}

	if resp["tests"].(float64) != 2 {
		t.Errorf("Expected 2 tests, got %v", resp["tests"])
	}
}

func TestTestsAPIHandleReloadMethodNotAllowed(t *testing.T) {
	testDir := setupTestDir(t)
	api := NewTestsAPI(testDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tests/reload", nil)
	w := httptest.NewRecorder()

	api.HandleReload(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestFormatSetName(t *testing.T) {
	// Note: formatSetName takes the set ID (filename without extension),
	// replaces hyphens with spaces and title-cases words
	// Only specific known acronyms (tls, pase, api, etc.) are kept uppercase
	tests := []struct {
		input    string
		expected string
	}{
		{"basic-tests", "Basic"},               // hyphen replaced, "tests" suffix trimmed
		{"read-operations", "Read Operations"}, // hyphens replaced
		{"tc-sec-backoff", "Tc Sec Backoff"},   // title case (not known acronyms)
		{"simple", "Simple"},                   // title case
		{"test-file-name", "Test File Name"},   // hyphens replaced
		{"tls-test", "TLS"},                    // "tls" is a known acronym, "test" suffix trimmed
		{"pase-auth", "PASE Auth"},             // "pase" is a known acronym
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := formatSetName(tt.input)
			if got != tt.expected {
				t.Errorf("formatSetName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

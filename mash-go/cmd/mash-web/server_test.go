package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a temporary test directory with a test case
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(`
id: TC-TEST-001
name: Test case
steps:
  - action: connect
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{
		Port:    0,
		TestDir: tmpDir,
		DBPath:  ":memory:",
		Version: "1.0.0-test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Create a request to the health endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %q", resp["status"])
	}

	if resp["version"] != "1.0.0-test" {
		t.Errorf("Expected version '1.0.0-test', got %q", resp["version"])
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(`
id: TC-TEST-001
name: Test case
steps:
  - action: connect
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{
		Port:    0,
		TestDir: tmpDir,
		DBPath:  ":memory:",
		Version: "test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestInfoEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(`
id: TC-TEST-001
name: Test case 1
steps:
  - action: connect
---
id: TC-TEST-002
name: Test case 2
steps:
  - action: read
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{
		Port:    0,
		TestDir: tmpDir,
		DBPath:  ":memory:",
		Version: "test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["test_count"] != 2 {
		t.Errorf("Expected test_count 2, got %d", resp["test_count"])
	}

	if resp["run_count"] != 0 {
		t.Errorf("Expected run_count 0, got %d", resp["run_count"])
	}
}

func TestStaticFileRouting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(`
id: TC-TEST-001
name: Test case
steps:
  - action: connect
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{
		Port:    0,
		TestDir: tmpDir,
		DBPath:  ":memory:",
		Version: "test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	tests := []struct {
		path        string
		wantStatus  int
		wantContain string
	}{
		{"/", http.StatusOK, "<!DOCTYPE html>"},
		{"/style.css", http.StatusOK, ""},
		{"/app.js", http.StatusOK, ""},
		{"/run", http.StatusOK, "<!DOCTYPE html>"},
		{"/run?id=test-123", http.StatusOK, "<!DOCTYPE html>"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}

			if tt.wantContain != "" && !strings.Contains(w.Body.String(), tt.wantContain) {
				t.Errorf("Response body should contain %q", tt.wantContain)
			}
		})
	}
}

func TestRunPageRouting(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(`
id: TC-TEST-001
name: Test case
steps:
  - action: connect
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := ServerConfig{
		Port:    0,
		TestDir: tmpDir,
		DBPath:  ":memory:",
		Version: "test",
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	// Test /run route returns run.html content
	req := httptest.NewRequest(http.MethodGet, "/run?id=test-run-123", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Run") {
		t.Error("Expected run.html content with 'Test Run' title")
	}

	if !strings.Contains(body, "log-container") {
		t.Error("Expected run.html content with log container")
	}
}

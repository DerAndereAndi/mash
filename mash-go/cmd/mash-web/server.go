package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/mash-protocol/mash-go/cmd/mash-web/api"
)

//go:embed static/*
var staticFiles embed.FS

// ServerConfig holds configuration for the HTTP server.
type ServerConfig struct {
	Port    int
	TestDir string
	DBPath  string
	Version string
}

// Server is the HTTP server for the MASH testing frontend.
type Server struct {
	config  ServerConfig
	mux     *http.ServeMux
	server  *http.Server
	store   *api.Store
	testAPI *api.TestsAPI
	runsAPI *api.RunsAPI
}

// NewServer creates a new server with the given configuration.
func NewServer(cfg ServerConfig) (*Server, error) {
	// Initialize SQLite store
	store, err := api.NewStore(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	// Initialize APIs
	testAPI := api.NewTestsAPI(cfg.TestDir)
	runsAPI := api.NewRunsAPI(store, cfg.TestDir)

	s := &Server{
		config:  cfg,
		mux:     http.NewServeMux(),
		store:   store,
		testAPI: testAPI,
		runsAPI: runsAPI,
	}

	s.registerRoutes()

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: s.mux,
	}

	return s, nil
}

// registerRoutes sets up all HTTP routes.
func (s *Server) registerRoutes() {
	// API routes
	s.mux.HandleFunc("/api/v1/health", s.handleHealth)
	s.mux.HandleFunc("/api/v1/info", s.handleInfo)

	// Test routes
	s.mux.HandleFunc("/api/v1/tests", s.testAPI.HandleList)
	s.mux.HandleFunc("/api/v1/tests/reload", s.testAPI.HandleReload)
	s.mux.HandleFunc("/api/v1/tests/", s.handleTestRoutes)
	s.mux.HandleFunc("/api/v1/testsets", s.testAPI.HandleSets)
	s.mux.HandleFunc("/api/v1/testsets/", s.testAPI.HandleSetByID)

	// Run routes
	s.mux.HandleFunc("/api/v1/runs", s.runsAPI.HandleRuns)
	s.mux.HandleFunc("/api/v1/runs/", s.runsAPI.HandleRunByID)

	// Device discovery
	s.mux.HandleFunc("/api/v1/devices", s.handleDevices)

	// Static files and SPA
	s.mux.HandleFunc("/", s.handleStatic)
}

// handleTestRoutes routes /api/v1/tests/:id and /api/v1/tests/:id/yaml requests.
func (s *Server) handleTestRoutes(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/yaml") {
		s.testAPI.HandleGetYAML(w, r)
	} else {
		s.testAPI.HandleGet(w, r)
	}
}

// handleHealth returns the server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	version := s.config.Version
	if version == "" {
		version = "dev"
	}

	resp := map[string]string{
		"status":  "ok",
		"version": version,
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleInfo returns server information.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	testCount, _ := s.testAPI.Count()
	runCount, _ := s.store.CountRuns()

	resp := map[string]int{
		"test_count": testCount,
		"run_count":  runCount,
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleDevices handles device discovery requests.
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get timeout from query parameter (default 10s)
	timeout := r.URL.Query().Get("timeout")
	if timeout == "" {
		timeout = "10s"
	}

	devices, err := api.DiscoverDevices(r.Context(), timeout)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

// handleStatic serves static files and the SPA.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Get the file path
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Route /run to run.html
	if path == "/run" || strings.HasPrefix(path, "/run?") {
		path = "/run.html"
	}

	// Try to serve from embedded files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove leading slash for fs.FS
	filePath := strings.TrimPrefix(path, "/")

	// Check if file exists
	file, err := staticFS.Open(filePath)
	if err != nil {
		// Fall back to index.html for SPA routing
		filePath = "index.html"
	} else {
		file.Close()
	}

	// Set content type based on extension
	switch {
	case strings.HasSuffix(filePath, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(filePath, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(filePath, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}

	http.ServeFileFS(w, r, staticFS, filePath)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// Close shuts down the server and closes the store.
func (s *Server) Close() error {
	if s.store != nil {
		s.store.Close()
	}
	return nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

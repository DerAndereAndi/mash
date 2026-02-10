package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/runner"
)

// RunsAPI handles test run endpoints.
type RunsAPI struct {
	store   *Store
	testDir string

	// Track active runs for SSE streaming
	mu          sync.RWMutex
	activeRuns  map[string]*activeRun
	sseChannels map[string][]chan *TestResult
}

// activeRun tracks state for an in-progress test run.
type activeRun struct {
	runID  string
	cancel context.CancelFunc
}

// NewRunsAPI creates a new runs API handler.
func NewRunsAPI(store *Store, testDir string) *RunsAPI {
	return &RunsAPI{
		store:       store,
		testDir:     testDir,
		activeRuns:  make(map[string]*activeRun),
		sseChannels: make(map[string][]chan *TestResult),
	}
}

// HandleRuns handles GET and POST /api/v1/runs.
func (r *RunsAPI) HandleRuns(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.handleListRuns(w, req)
	case http.MethodPost:
		r.handleCreateRun(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleRunByID handles GET /api/v1/runs/:id and GET /api/v1/runs/:id/stream.
func (r *RunsAPI) HandleRunByID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /api/v1/runs/{id} or /api/v1/runs/{id}/stream
	path := strings.TrimPrefix(req.URL.Path, "/api/v1/runs/")

	if strings.HasSuffix(path, "/stream") {
		id := strings.TrimSuffix(path, "/stream")
		r.handleStream(w, req, id)
		return
	}

	r.handleGetRun(w, req, path)
}

// handleListRuns handles GET /api/v1/runs.
func (r *RunsAPI) handleListRuns(w http.ResponseWriter, req *http.Request) {
	runs, err := r.store.ListRuns(100, 0)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list runs", err.Error())
		return
	}

	resp := RunListResponse{
		Runs:  runs,
		Total: len(runs),
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// handleCreateRun handles POST /api/v1/runs.
func (r *RunsAPI) handleCreateRun(w http.ResponseWriter, req *http.Request) {
	var runReq RunRequest
	if err := json.NewDecoder(req.Body).Decode(&runReq); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	if runReq.Target == "" {
		writeJSONError(w, http.StatusBadRequest, "Target is required", "")
		return
	}

	// Create the run
	runID := uuid.New().String()
	now := time.Now()

	run := &Run{
		ID:        runID,
		Target:    runReq.Target,
		Pattern:   runReq.Pattern,
		Status:    RunStatusPending,
		StartedAt: &now,
	}

	if err := r.store.CreateRun(run); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create run", err.Error())
		return
	}

	// Start the test run in a goroutine
	ctx, cancel := context.WithCancel(context.Background())

	r.mu.Lock()
	r.activeRuns[runID] = &activeRun{
		runID:  runID,
		cancel: cancel,
	}
	r.mu.Unlock()

	go r.executeRun(ctx, runID, runReq)

	// Return immediately with the run ID
	run.Status = RunStatusRunning
	writeJSONResponse(w, http.StatusAccepted, run)
}

// executeRun runs tests and updates the store with results.
func (r *RunsAPI) executeRun(ctx context.Context, runID string, req RunRequest) {
	defer func() {
		r.mu.Lock()
		delete(r.activeRuns, runID)
		// Close all SSE channels for this run
		for _, ch := range r.sseChannels[runID] {
			close(ch)
		}
		delete(r.sseChannels, runID)
		r.mu.Unlock()
	}()

	// Update status to running
	r.store.UpdateRunStatus(runID, RunStatusRunning)

	// Parse timeout
	timeout := 30 * time.Second
	if req.Timeout != "" {
		if d, err := time.ParseDuration(req.Timeout); err == nil {
			timeout = d
		}
	}

	// Determine mode
	mode := req.Mode
	if mode == "" {
		mode = "device"
	}

	// Create runner configuration
	config := &runner.Config{
		Target:    req.Target,
		Mode:      mode,
		Pattern:   req.Pattern,
		TestDir:   r.testDir,
		Timeout:   timeout,
		SetupCode: req.SetupCode,
		Output:    io.Discard, // We capture results via callback
	}

	// Create and run the test runner
	testRunner := runner.New(config)
	defer testRunner.Close()

	// We need to intercept test results for streaming and storage
	// Unfortunately, the runner's OnTestComplete callback is internal,
	// so we'll run the tests and process results after completion

	result, err := testRunner.Run(ctx)
	if err != nil {
		r.store.CompleteRun(runID, 0, 0, 0, 0, err.Error())
		return
	}

	// Store individual test results
	for _, tr := range result.Results {
		testResult := engineResultToAPI(tr)
		r.store.AddTestResult(runID, testResult)

		// Broadcast to SSE listeners
		r.broadcastResult(runID, testResult)
	}

	// Complete the run
	r.store.CompleteRun(runID, result.PassCount, result.FailCount, result.SkipCount, len(result.Results), "")
}

// broadcastResult sends a test result to all SSE listeners for a run.
func (r *RunsAPI) broadcastResult(runID string, result *TestResult) {
	r.mu.RLock()
	channels := r.sseChannels[runID]
	r.mu.RUnlock()

	for _, ch := range channels {
		select {
		case ch <- result:
		default:
			// Channel full, skip
		}
	}
}

// handleGetRun handles GET /api/v1/runs/:id.
func (r *RunsAPI) handleGetRun(w http.ResponseWriter, req *http.Request, id string) {
	run, err := r.store.GetRun(id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get run", err.Error())
		return
	}

	if run == nil {
		writeJSONError(w, http.StatusNotFound, "Run not found", id)
		return
	}

	// Get test results
	results, err := r.store.GetRunResults(id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get run results", err.Error())
		return
	}

	resp := RunDetailResponse{
		Run:     *run,
		Results: results,
	}

	writeJSONResponse(w, http.StatusOK, resp)
}

// handleStream handles GET /api/v1/runs/:id/stream (Server-Sent Events).
func (r *RunsAPI) handleStream(w http.ResponseWriter, req *http.Request, id string) {
	// Check if the run exists
	run, err := r.store.GetRun(id)
	if err != nil || run == nil {
		writeJSONError(w, http.StatusNotFound, "Run not found", id)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channel for this client
	ch := make(chan *TestResult, 100)

	r.mu.Lock()
	r.sseChannels[id] = append(r.sseChannels[id], ch)
	r.mu.Unlock()

	// Clean up on disconnect
	defer func() {
		r.mu.Lock()
		channels := r.sseChannels[id]
		for i, c := range channels {
			if c == ch {
				r.sseChannels[id] = append(channels[:i], channels[i+1:]...)
				break
			}
		}
		r.mu.Unlock()
		close(ch)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send existing results first
	existingResults, _ := r.store.GetRunResults(id)
	for _, result := range existingResults {
		data, _ := json.Marshal(result)
		fmt.Fprintf(w, "event: result\ndata: %s\n\n", data)
		flusher.Flush()
	}

	// Stream new results
	for {
		select {
		case result, ok := <-ch:
			if !ok {
				// Channel closed, run completed
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			}
			data, _ := json.Marshal(result)
			fmt.Fprintf(w, "event: result\ndata: %s\n\n", data)
			flusher.Flush()

		case <-req.Context().Done():
			return
		}
	}
}

// engineResultToAPI converts an engine.TestResult to an API TestResult.
func engineResultToAPI(tr *engine.TestResult) *TestResult {
	status := TestStatusPassed
	if tr.Skipped {
		status = TestStatusSkipped
	} else if !tr.Passed {
		status = TestStatusFailed
	}

	result := &TestResult{
		TestID:   tr.TestCase.ID,
		TestName: tr.TestCase.Name,
		Status:   status,
		Duration: tr.Duration.Round(time.Millisecond).String(),
	}

	if tr.Error != nil {
		result.Error = tr.Error.Error()
	}
	if tr.SkipReason != "" {
		result.SkipReason = tr.SkipReason
	}

	// Convert step results
	for _, sr := range tr.StepResults {
		stepStatus := TestStatusPassed
		if !sr.Passed {
			stepStatus = TestStatusFailed
		}

		step := StepResult{
			Index:    sr.StepIndex,
			Action:   sr.Step.Action,
			Status:   stepStatus,
			Duration: sr.Duration.Round(time.Millisecond).String(),
			Expects:  make(map[string]Expect),
		}

		if sr.Error != nil {
			step.Error = sr.Error.Error()
		}

		for key, er := range sr.ExpectResults {
			step.Expects[key] = Expect{
				Passed:   er.Passed,
				Expected: er.Expected,
				Actual:   er.Actual,
				Message:  er.Message,
			}
		}

		result.Steps = append(result.Steps, step)
	}

	return result
}

// CancelRun cancels an active test run.
func (r *RunsAPI) CancelRun(runID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if active, ok := r.activeRuns[runID]; ok {
		active.cancel()
		return true
	}
	return false
}

// LogTestResult logs a test result (for debugging).
func LogTestResult(result *TestResult) {
	log.Printf("[%s] %s - %s (%s)", result.Status, result.TestID, result.TestName, result.Duration)
}

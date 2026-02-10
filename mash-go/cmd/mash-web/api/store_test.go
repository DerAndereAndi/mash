package api

import (
	"testing"
	"time"
)

func TestStoreCreateAndGetRun(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run-1",
		Target:    "localhost:8443",
		Pattern:   "TC-*",
		Status:    RunStatusPending,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	got, err := store.GetRun("test-run-1")
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	if got == nil {
		t.Fatal("Expected run, got nil")
	}

	if got.ID != "test-run-1" {
		t.Errorf("Expected ID 'test-run-1', got %q", got.ID)
	}

	if got.Target != "localhost:8443" {
		t.Errorf("Expected target 'localhost:8443', got %q", got.Target)
	}

	if got.Pattern != "TC-*" {
		t.Errorf("Expected pattern 'TC-*', got %q", got.Pattern)
	}

	if got.Status != RunStatusPending {
		t.Errorf("Expected status 'pending', got %q", got.Status)
	}
}

func TestStoreGetRunNotFound(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	got, err := store.GetRun("nonexistent")
	if err != nil {
		t.Fatalf("Expected nil error, got %v", err)
	}

	if got != nil {
		t.Errorf("Expected nil run, got %+v", got)
	}
}

func TestStoreListRuns(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	for i := 0; i < 5; i++ {
		startedAt := now.Add(time.Duration(i) * time.Minute)
		run := &Run{
			ID:        "run-" + string(rune('a'+i)),
			Target:    "localhost:8443",
			Status:    RunStatusCompleted,
			StartedAt: &startedAt,
		}
		if err := store.CreateRun(run); err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
	}

	runs, err := store.ListRuns(10, 0)
	if err != nil {
		t.Fatalf("Failed to list runs: %v", err)
	}

	if len(runs) != 5 {
		t.Errorf("Expected 5 runs, got %d", len(runs))
	}

	// Should be ordered by most recent first
	if runs[0].ID != "run-e" {
		t.Errorf("Expected first run to be 'run-e', got %q", runs[0].ID)
	}
}

func TestStoreUpdateRunStatus(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run",
		Target:    "localhost:8443",
		Status:    RunStatusPending,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	if err := store.UpdateRunStatus("test-run", RunStatusRunning); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	got, err := store.GetRun("test-run")
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	if got.Status != RunStatusRunning {
		t.Errorf("Expected status 'running', got %q", got.Status)
	}
}

func TestStoreCompleteRun(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run",
		Target:    "localhost:8443",
		Status:    RunStatusRunning,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	if err := store.CompleteRun("test-run", 10, 2, 3, 15, ""); err != nil {
		t.Fatalf("Failed to complete run: %v", err)
	}

	got, err := store.GetRun("test-run")
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	if got.Status != RunStatusCompleted {
		t.Errorf("Expected status 'completed', got %q", got.Status)
	}

	if got.PassCount != 10 {
		t.Errorf("Expected pass_count 10, got %d", got.PassCount)
	}

	if got.FailCount != 2 {
		t.Errorf("Expected fail_count 2, got %d", got.FailCount)
	}

	if got.SkipCount != 3 {
		t.Errorf("Expected skip_count 3, got %d", got.SkipCount)
	}

	if got.TotalCount != 15 {
		t.Errorf("Expected total_count 15, got %d", got.TotalCount)
	}

	if got.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

func TestStoreCompleteRunWithError(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run",
		Target:    "localhost:8443",
		Status:    RunStatusRunning,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	if err := store.CompleteRun("test-run", 0, 0, 0, 0, "connection failed"); err != nil {
		t.Fatalf("Failed to complete run: %v", err)
	}

	got, err := store.GetRun("test-run")
	if err != nil {
		t.Fatalf("Failed to get run: %v", err)
	}

	if got.Status != RunStatusFailed {
		t.Errorf("Expected status 'failed', got %q", got.Status)
	}
}

func TestStoreAddAndGetTestResults(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run",
		Target:    "localhost:8443",
		Status:    RunStatusRunning,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	results := []*TestResult{
		{
			TestID:   "TC-001",
			TestName: "Test 1",
			Status:   TestStatusPassed,
			Duration: "100ms",
		},
		{
			TestID:   "TC-002",
			TestName: "Test 2",
			Status:   TestStatusFailed,
			Duration: "50ms",
			Error:    "assertion failed",
		},
		{
			TestID:     "TC-003",
			TestName:   "Test 3",
			Status:     TestStatusSkipped,
			SkipReason: "PICS mismatch",
		},
	}

	for _, r := range results {
		if err := store.AddTestResult("test-run", r); err != nil {
			t.Fatalf("Failed to add test result: %v", err)
		}
	}

	got, err := store.GetRunResults("test-run")
	if err != nil {
		t.Fatalf("Failed to get results: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(got))
	}

	if got[0].TestID != "TC-001" {
		t.Errorf("Expected first result TestID 'TC-001', got %q", got[0].TestID)
	}

	if got[1].Error != "assertion failed" {
		t.Errorf("Expected second result error 'assertion failed', got %q", got[1].Error)
	}

	if got[2].SkipReason != "PICS mismatch" {
		t.Errorf("Expected third result skip_reason 'PICS mismatch', got %q", got[2].SkipReason)
	}
}

func TestStoreCountRuns(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	count, err := store.CountRuns()
	if err != nil {
		t.Fatalf("Failed to count runs: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 runs, got %d", count)
	}

	now := time.Now()
	for i := 0; i < 3; i++ {
		run := &Run{
			ID:        "run-" + string(rune('a'+i)),
			Target:    "localhost:8443",
			Status:    RunStatusCompleted,
			StartedAt: &now,
		}
		if err := store.CreateRun(run); err != nil {
			t.Fatalf("Failed to create run: %v", err)
		}
	}

	count, err = store.CountRuns()
	if err != nil {
		t.Fatalf("Failed to count runs: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 runs, got %d", count)
	}
}

func TestStoreDeleteRun(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	run := &Run{
		ID:        "test-run",
		Target:    "localhost:8443",
		Status:    RunStatusCompleted,
		StartedAt: &now,
	}

	if err := store.CreateRun(run); err != nil {
		t.Fatalf("Failed to create run: %v", err)
	}

	// Add a result
	result := &TestResult{
		TestID:   "TC-001",
		TestName: "Test 1",
		Status:   TestStatusPassed,
	}
	if err := store.AddTestResult("test-run", result); err != nil {
		t.Fatalf("Failed to add result: %v", err)
	}

	// Delete the run
	if err := store.DeleteRun("test-run"); err != nil {
		t.Fatalf("Failed to delete run: %v", err)
	}

	// Verify run is gone
	got, err := store.GetRun("test-run")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got != nil {
		t.Error("Expected run to be deleted")
	}

	// Verify results are also gone (cascade delete)
	results, err := store.GetRunResults("test-run")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results after delete, got %d", len(results))
	}
}

package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Store provides SQLite persistence for test runs and results.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore creates a new store with the given database path.
// Use ":memory:" for an in-memory database.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys and WAL mode for better performance
	_, err = db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	s := &Store{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return s, nil
}

// migrate creates the database schema.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		target TEXT NOT NULL,
		pattern TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		started_at DATETIME,
		completed_at DATETIME,
		pass_count INTEGER DEFAULT 0,
		fail_count INTEGER DEFAULT 0,
		skip_count INTEGER DEFAULT 0,
		total_count INTEGER DEFAULT 0,
		error_message TEXT
	);

	CREATE TABLE IF NOT EXISTS run_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
		test_id TEXT NOT NULL,
		test_name TEXT,
		status TEXT NOT NULL,
		duration_ms INTEGER,
		error TEXT,
		skip_reason TEXT,
		result_json TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_run_results_run_id ON run_results(run_id);
	CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
	CREATE INDEX IF NOT EXISTS idx_runs_started_at ON runs(started_at);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateRun creates a new test run.
func (s *Store) CreateRun(run *Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO runs (id, target, pattern, status, started_at)
		VALUES (?, ?, ?, ?, ?)
	`, run.ID, run.Target, run.Pattern, run.Status, run.StartedAt)

	return err
}

// GetRun retrieves a run by ID.
func (s *Store) GetRun(id string) (*Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var run Run
	var startedAt, completedAt sql.NullTime
	var pattern sql.NullString

	err := s.db.QueryRow(`
		SELECT id, target, pattern, status, started_at, completed_at,
		       pass_count, fail_count, skip_count, total_count
		FROM runs WHERE id = ?
	`, id).Scan(
		&run.ID, &run.Target, &pattern, &run.Status,
		&startedAt, &completedAt,
		&run.PassCount, &run.FailCount, &run.SkipCount, &run.TotalCount,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if pattern.Valid {
		run.Pattern = pattern.String
	}
	if startedAt.Valid {
		run.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}

	// Calculate duration if completed
	if run.StartedAt != nil && run.CompletedAt != nil {
		run.Duration = run.CompletedAt.Sub(*run.StartedAt).Round(time.Millisecond).String()
	}

	return &run, nil
}

// ListRuns retrieves all runs, ordered by most recent first.
func (s *Store) ListRuns(limit, offset int) ([]Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(`
		SELECT id, target, pattern, status, started_at, completed_at,
		       pass_count, fail_count, skip_count, total_count
		FROM runs
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var run Run
		var startedAt, completedAt sql.NullTime
		var pattern sql.NullString

		if err := rows.Scan(
			&run.ID, &run.Target, &pattern, &run.Status,
			&startedAt, &completedAt,
			&run.PassCount, &run.FailCount, &run.SkipCount, &run.TotalCount,
		); err != nil {
			return nil, err
		}

		if pattern.Valid {
			run.Pattern = pattern.String
		}
		if startedAt.Valid {
			run.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			run.CompletedAt = &completedAt.Time
		}

		if run.StartedAt != nil && run.CompletedAt != nil {
			run.Duration = run.CompletedAt.Sub(*run.StartedAt).Round(time.Millisecond).String()
		}

		runs = append(runs, run)
	}

	return runs, rows.Err()
}

// CountRuns returns the total number of runs.
func (s *Store) CountRuns() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM runs").Scan(&count)
	return count, err
}

// UpdateRunStatus updates the status of a run.
func (s *Store) UpdateRunStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE runs SET status = ? WHERE id = ?
	`, status, id)
	return err
}

// CompleteRun marks a run as completed with final statistics.
func (s *Store) CompleteRun(id string, passCount, failCount, skipCount, totalCount int, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := RunStatusCompleted
	if errMsg != "" {
		status = RunStatusFailed
	}

	_, err := s.db.Exec(`
		UPDATE runs
		SET status = ?, completed_at = ?, pass_count = ?, fail_count = ?,
		    skip_count = ?, total_count = ?, error_message = ?
		WHERE id = ?
	`, status, time.Now(), passCount, failCount, skipCount, totalCount, errMsg, id)
	return err
}

// AddTestResult adds a test result to a run.
func (s *Store) AddTestResult(runID string, result *TestResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Serialize the full result as JSON for detailed retrieval
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Parse duration to milliseconds
	var durationMs int64
	if result.Duration != "" {
		if d, err := time.ParseDuration(result.Duration); err == nil {
			durationMs = d.Milliseconds()
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO run_results (run_id, test_id, test_name, status, duration_ms, error, skip_reason, result_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, result.TestID, result.TestName, result.Status, durationMs, result.Error, result.SkipReason, string(resultJSON))

	return err
}

// GetRunResults retrieves all test results for a run.
func (s *Store) GetRunResults(runID string) ([]TestResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT result_json FROM run_results WHERE run_id = ? ORDER BY id
	`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []TestResult
	for rows.Next() {
		var resultJSON string
		if err := rows.Scan(&resultJSON); err != nil {
			return nil, err
		}

		var result TestResult
		if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %w", err)
		}

		results = append(results, result)
	}

	return results, rows.Err()
}

// DeleteRun deletes a run and its results.
func (s *Store) DeleteRun(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM runs WHERE id = ?", id)
	return err
}

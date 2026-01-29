package service

import (
	"testing"
	"time"
)

func TestPASEAttemptTracker_GetDelay(t *testing.T) {
	tiers := [4]time.Duration{
		0,                        // Tier 1: no delay
		1000 * time.Millisecond,  // Tier 2: 1s
		3000 * time.Millisecond,  // Tier 3: 3s
		10000 * time.Millisecond, // Tier 4: 10s
	}

	tracker := NewPASEAttemptTracker(tiers)

	tests := []struct {
		name          string
		failedBefore  int
		expectedDelay time.Duration
	}{
		{"attempt 1 (no prior failures)", 0, 0},
		{"attempt 2 (1 prior failure)", 1, 0},
		{"attempt 3 (2 prior failures)", 2, 0},
		{"attempt 4 (3 prior failures)", 3, 1000 * time.Millisecond},
		{"attempt 5 (4 prior failures)", 4, 1000 * time.Millisecond},
		{"attempt 6 (5 prior failures)", 5, 1000 * time.Millisecond},
		{"attempt 7 (6 prior failures)", 6, 3000 * time.Millisecond},
		{"attempt 8 (7 prior failures)", 7, 3000 * time.Millisecond},
		{"attempt 10 (9 prior failures)", 9, 3000 * time.Millisecond},
		{"attempt 11 (10 prior failures)", 10, 10000 * time.Millisecond},
		{"attempt 20 (19 prior failures)", 19, 10000 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracker and record the specified number of failures
			tracker.Reset()
			for i := 0; i < tt.failedBefore; i++ {
				tracker.RecordFailure()
			}

			delay := tracker.GetDelay()
			if delay != tt.expectedDelay {
				t.Errorf("GetDelay() = %v, want %v", delay, tt.expectedDelay)
			}
		})
	}
}

func TestPASEAttemptTracker_RecordFailure(t *testing.T) {
	tracker := NewPASEAttemptTracker([4]time.Duration{0, 1, 2, 3})

	if tracker.AttemptCount() != 0 {
		t.Errorf("initial count = %d, want 0", tracker.AttemptCount())
	}

	tracker.RecordFailure()
	if tracker.AttemptCount() != 1 {
		t.Errorf("after 1 failure: count = %d, want 1", tracker.AttemptCount())
	}

	tracker.RecordFailure()
	tracker.RecordFailure()
	if tracker.AttemptCount() != 3 {
		t.Errorf("after 3 failures: count = %d, want 3", tracker.AttemptCount())
	}
}

func TestPASEAttemptTracker_Reset(t *testing.T) {
	tracker := NewPASEAttemptTracker([4]time.Duration{0, 1, 2, 3})

	// Accumulate some failures
	for i := 0; i < 10; i++ {
		tracker.RecordFailure()
	}

	if tracker.AttemptCount() != 10 {
		t.Errorf("before reset: count = %d, want 10", tracker.AttemptCount())
	}

	tracker.Reset()

	if tracker.AttemptCount() != 0 {
		t.Errorf("after reset: count = %d, want 0", tracker.AttemptCount())
	}

	// Verify delay is back to tier 1
	delay := tracker.GetDelay()
	if delay != 0 {
		t.Errorf("after reset: delay = %v, want 0", delay)
	}
}

func TestPASEAttemptTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewPASEAttemptTracker([4]time.Duration{0, 1, 2, 3})

	// Run concurrent operations to verify thread safety
	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			tracker.RecordFailure()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = tracker.GetDelay()
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 10; i++ {
			tracker.Reset()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Test passes if no race conditions occurred
	// (run with -race flag to detect races)
}

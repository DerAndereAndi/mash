package service

import (
	"sync"
	"time"
)

// PASEAttemptTracker tracks failed PASE attempts and implements exponential backoff.
// It is used during commissioning to prevent brute force attacks on the setup code
// while maintaining usability for legitimate users who may mistype the code.
//
// Backoff behavior (DEC-047):
// - Attempts 1-3: No delay (normal user mistakes)
// - Attempts 4-6: Tier 2 delay (possible attack)
// - Attempts 7-10: Tier 3 delay (likely attack)
// - Attempts 11+: Tier 4 delay (definite attack)
//
// The counter resets when:
// - Commissioning window closes
// - Commissioning succeeds
type PASEAttemptTracker struct {
	mu             sync.Mutex
	failedAttempts int
	backoffTiers   [4]time.Duration
}

// NewPASEAttemptTracker creates a new tracker with the specified backoff tiers.
// The tiers array contains delays for: [1-3 attempts, 4-6, 7-10, 11+].
func NewPASEAttemptTracker(tiers [4]time.Duration) *PASEAttemptTracker {
	return &PASEAttemptTracker{
		backoffTiers: tiers,
	}
}

// GetDelay returns the delay that should be applied before responding to the
// current attempt. Call this before processing the PASE request.
func (t *PASEAttemptTracker) GetDelay() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Determine which tier we're in based on current attempt count
	// Note: failedAttempts is the count of PAST failures,
	// so attempt number is failedAttempts + 1
	attemptNumber := t.failedAttempts + 1

	switch {
	case attemptNumber <= 3:
		return t.backoffTiers[0] // Tier 1: no delay for first 3 attempts
	case attemptNumber <= 6:
		return t.backoffTiers[1] // Tier 2: attempts 4-6
	case attemptNumber <= 10:
		return t.backoffTiers[2] // Tier 3: attempts 7-10
	default:
		return t.backoffTiers[3] // Tier 4: attempts 11+
	}
}

// RecordFailure increments the failed attempt counter.
// Call this after a PASE verification failure.
func (t *PASEAttemptTracker) RecordFailure() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failedAttempts++
}

// Reset clears the failed attempt counter.
// Call this when commissioning window closes or commissioning succeeds.
func (t *PASEAttemptTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failedAttempts = 0
}

// AttemptCount returns the current number of failed attempts.
// This is primarily for testing and diagnostics.
func (t *PASEAttemptTracker) AttemptCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failedAttempts
}

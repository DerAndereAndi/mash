package service

import (
	"sync"
	"testing"
	"time"
)

// TestCommissioningLockBasic verifies the basic accept/release cycle.
func TestCommissioningLockBasic(t *testing.T) {
	svc := &DeviceService{
		config: DeviceConfig{MaxZones: 2},
	}

	ok, _, _, gen := svc.acceptCommissioningConnection()
	if !ok {
		t.Fatal("first accept should succeed")
	}
	if gen == 0 {
		t.Fatal("generation should be non-zero after accept")
	}

	// Second accept should fail (already in progress).
	ok2, reason, _, _ := svc.acceptCommissioningConnection()
	if ok2 {
		t.Fatal("second accept should fail")
	}
	if reason != rejectAlreadyInProgress {
		t.Fatalf("expected rejectAlreadyInProgress, got %v", reason)
	}

	// Release with correct generation.
	svc.releaseCommissioningConnection(gen)

	// Should be able to accept again after release.
	ok3, _, _, gen3 := svc.acceptCommissioningConnection()
	if !ok3 {
		t.Fatal("accept after release should succeed")
	}
	if gen3 <= gen {
		t.Fatalf("generation should increase: got %d, previous %d", gen3, gen)
	}
	svc.releaseCommissioningConnection(gen3)
}

// TestCommissioningLockStaleRelease demonstrates the race condition that caused
// TC-CONN-BUSY-003 failures: a stale goroutine from a previous test holding an
// old generation cannot release the lock acquired by a newer goroutine.
func TestCommissioningLockStaleRelease(t *testing.T) {
	svc := &DeviceService{
		config: DeviceConfig{MaxZones: 2},
	}

	// Simulate test N: accept and capture generation.
	ok, _, _, oldGen := svc.acceptCommissioningConnection()
	if !ok {
		t.Fatal("initial accept should succeed")
	}

	// Simulate TriggerResetTestState between tests: force-clear + bump generation.
	svc.connectionMu.Lock()
	svc.commissioningConnActive = false
	svc.commissioningGeneration++
	svc.connectionMu.Unlock()

	// Simulate test N+1: accept acquires lock with new generation.
	ok2, _, _, newGen := svc.acceptCommissioningConnection()
	if !ok2 {
		t.Fatal("accept after reset should succeed")
	}
	if newGen <= oldGen {
		t.Fatalf("new generation (%d) should be greater than old (%d)", newGen, oldGen)
	}

	// Stale goroutine from test N calls release with old generation.
	// This must NOT clear the lock owned by test N+1.
	svc.releaseCommissioningConnection(oldGen)

	// Verify the lock is still held by test N+1.
	svc.connectionMu.Lock()
	active := svc.commissioningConnActive
	svc.connectionMu.Unlock()
	if !active {
		t.Fatal("stale release cleared the lock -- generation check failed")
	}

	// A third connection should be rejected (lock still held by test N+1).
	ok3, reason, _, _ := svc.acceptCommissioningConnection()
	if ok3 {
		t.Fatal("accept should fail while lock is held")
	}
	if reason != rejectAlreadyInProgress {
		t.Fatalf("expected rejectAlreadyInProgress, got %v", reason)
	}

	// Correct release with newGen works.
	svc.releaseCommissioningConnection(newGen)

	svc.connectionMu.Lock()
	active = svc.commissioningConnActive
	svc.connectionMu.Unlock()
	if active {
		t.Fatal("correct release should clear the lock")
	}
}

// TestCommissioningLockConcurrentStaleRelease simulates the exact race from
// TC-CONN-BUSY-003: goroutine A (old test) releases while goroutine B (new
// test) holds the lock, then goroutine C tries to acquire.
func TestCommissioningLockConcurrentStaleRelease(t *testing.T) {
	svc := &DeviceService{
		config: DeviceConfig{MaxZones: 2},
	}

	// Step 1: goroutine A acquires lock (simulating TC-CONN-BUSY-002).
	_, _, _, genA := svc.acceptCommissioningConnection()

	// Step 2: TriggerResetTestState between tests.
	svc.connectionMu.Lock()
	svc.commissioningConnActive = false
	svc.commissioningGeneration++
	svc.connectionMu.Unlock()

	// Step 3: goroutine B acquires lock (simulating TC-CONN-BUSY-003 step 1).
	ok, _, _, genB := svc.acceptCommissioningConnection()
	if !ok {
		t.Fatal("goroutine B should acquire lock")
	}

	// Step 4: concurrently, goroutine A's defer fires (stale release).
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.releaseCommissioningConnection(genA)
	}()
	wg.Wait()

	// Step 5: goroutine C tries to acquire (simulating TC-CONN-BUSY-003 step 2).
	// With the generation fix, this should be rejected because B still holds.
	ok3, reason, _, _ := svc.acceptCommissioningConnection()
	if ok3 {
		t.Fatal("goroutine C should be rejected -- B still holds the lock")
	}
	if reason != rejectAlreadyInProgress {
		t.Fatalf("expected rejectAlreadyInProgress, got %v", reason)
	}

	svc.releaseCommissioningConnection(genB)
}

// TestCommissioningLockCooldownAfterRelease verifies that release sets the
// cooldown timestamp only when the generation matches.
func TestCommissioningLockCooldownAfterRelease(t *testing.T) {
	svc := &DeviceService{
		config: DeviceConfig{
			MaxZones:           2,
			ConnectionCooldown: 500 * time.Millisecond,
		},
	}

	_, _, _, gen := svc.acceptCommissioningConnection()
	svc.releaseCommissioningConnection(gen)

	// Cooldown should be active.
	ok, reason, _, _ := svc.acceptCommissioningConnection()
	if ok {
		t.Fatal("accept during cooldown should fail")
	}
	if reason != rejectCooldown {
		t.Fatalf("expected rejectCooldown, got %v", reason)
	}

	// Stale release should NOT update cooldown timestamp.
	// Wait for cooldown to expire, then do a stale release -- it should not
	// re-arm the cooldown.
	time.Sleep(600 * time.Millisecond)

	// Accept again (cooldown expired).
	ok2, _, _, gen2 := svc.acceptCommissioningConnection()
	if !ok2 {
		t.Fatal("accept after cooldown expiry should succeed")
	}

	// Stale release with old gen should not set lastCommissioningAttempt.
	svc.connectionMu.Lock()
	svc.lastCommissioningAttempt = time.Time{} // clear for test
	svc.connectionMu.Unlock()

	svc.releaseCommissioningConnection(gen) // stale gen

	svc.connectionMu.Lock()
	ts := svc.lastCommissioningAttempt
	svc.connectionMu.Unlock()
	if !ts.IsZero() {
		t.Fatal("stale release should not set cooldown timestamp")
	}

	svc.releaseCommissioningConnection(gen2)
}

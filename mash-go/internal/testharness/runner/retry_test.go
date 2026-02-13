package runner

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoff_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   50 * time.Millisecond,
	}, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryWithBackoff_SucceedsOnThirdAttempt(t *testing.T) {
	calls := 0
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
	}, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_ExhaustsAttempts(t *testing.T) {
	sentinel := errors.New("persistent failure")
	calls := 0
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   10 * time.Millisecond,
	}, func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryWithBackoff_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	start := time.Now()
	err := retryWithBackoff(ctx, RetryConfig{
		MaxAttempts: 10,
		BaseDelay:   500 * time.Millisecond,
	}, func() error {
		calls++
		if calls == 1 {
			cancel() // cancel during first backoff wait
		}
		return errors.New("fail")
	})
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// Should not have waited the full 500ms backoff.
	if elapsed > 200*time.Millisecond {
		t.Fatalf("took too long (%v), context cancellation not respected", elapsed)
	}
}

func TestRetryWithBackoff_ExponentialDelays(t *testing.T) {
	var timestamps []time.Time
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 4,
		BaseDelay:   50 * time.Millisecond,
		MaxDelay:    1 * time.Second,
	}, func() error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) < 4 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(timestamps) != 4 {
		t.Fatalf("expected 4 timestamps, got %d", len(timestamps))
	}

	// Delay between call 1 and 2: ~50ms (base * 2^0)
	d1 := timestamps[1].Sub(timestamps[0])
	// Delay between call 2 and 3: ~100ms (base * 2^1)
	d2 := timestamps[2].Sub(timestamps[1])
	// Delay between call 3 and 4: ~200ms (base * 2^2)
	d3 := timestamps[3].Sub(timestamps[2])

	assertInRange(t, "delay1", d1, 30*time.Millisecond, 120*time.Millisecond)
	assertInRange(t, "delay2", d2, 70*time.Millisecond, 220*time.Millisecond)
	assertInRange(t, "delay3", d3, 150*time.Millisecond, 400*time.Millisecond)

	// Verify delays are roughly doubling.
	ratio := float64(d2) / float64(d1)
	if ratio < 1.3 || ratio > 3.5 {
		t.Errorf("delay2/delay1 ratio %.2f not in expected range [1.3, 3.5]", ratio)
	}
}

func TestRetryWithBackoff_StopsOnDeviceError(t *testing.T) {
	calls := 0
	deviceErr := &ClassifiedError{Category: ErrCatDevice, Err: errors.New("zone type exists")}
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
	}, func() error {
		calls++
		return deviceErr
	})
	if calls != 1 {
		t.Fatalf("expected 1 call (device error stops immediately), got %d", calls)
	}
	if !errors.Is(err, deviceErr) {
		t.Fatalf("expected device error, got %v", err)
	}
}

func TestRetryWithBackoff_CapsAtMaxDelay(t *testing.T) {
	var timestamps []time.Time
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    150 * time.Millisecond,
	}, func() error {
		timestamps = append(timestamps, time.Now())
		if len(timestamps) < 5 {
			return errors.New("fail")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// The 3rd+ delays should be capped at ~150ms, not 400ms+.
	for i := 2; i < len(timestamps)-1; i++ {
		d := timestamps[i+1].Sub(timestamps[i])
		if d > 300*time.Millisecond {
			t.Errorf("delay %d = %v exceeds maxDelay cap (expected <=300ms with jitter)", i, d)
		}
	}
}

func TestRetryWithBackoff_ZeroAttemptsReturnsError(t *testing.T) {
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 0,
		BaseDelay:   10 * time.Millisecond,
	}, func() error {
		t.Fatal("fn should not be called with 0 attempts")
		return nil
	})
	if err == nil {
		t.Fatal("expected error for zero attempts")
	}
	if !errors.Is(err, errNoAttempts) {
		t.Fatalf("expected errNoAttempts, got %v", err)
	}
}

func TestRetryWithBackoff_StopsOnProtocolError(t *testing.T) {
	calls := 0
	protoErr := &ClassifiedError{Category: ErrCatProtocol, Err: errors.New("malformed message")}
	err := retryWithBackoff(context.Background(), RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   10 * time.Millisecond,
	}, func() error {
		calls++
		return protoErr
	})
	if calls != 1 {
		t.Fatalf("expected 1 call (protocol error stops immediately), got %d", calls)
	}
	if !errors.Is(err, protoErr) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestDialWithRetry_SucceedsOnSecondAttempt(t *testing.T) {
	calls := 0
	conn, err := dialWithRetry(context.Background(), 3, func() (*tls.Conn, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("connection refused")
		}
		return &tls.Conn{}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestDialWithRetry_ExhaustsAttempts(t *testing.T) {
	calls := 0
	conn, err := dialWithRetry(context.Background(), 3, func() (*tls.Conn, error) {
		calls++
		return nil, errors.New("connection refused")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if conn != nil {
		t.Fatal("expected nil connection")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestDialWithRetry_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := dialWithRetry(ctx, 10, func() (*tls.Conn, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return nil, errors.New("connection refused")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestContextSleep_RespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	start := time.Now()
	err := contextSleep(ctx, 5*time.Second)
	elapsed := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("contextSleep should have returned immediately, took %v", elapsed)
	}
}

func TestContextSleep_WaitsFullDuration(t *testing.T) {
	start := time.Now()
	err := contextSleep(context.Background(), 50*time.Millisecond)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if elapsed < 30*time.Millisecond {
		t.Fatalf("contextSleep returned too early: %v", elapsed)
	}
}

func assertInRange(t *testing.T, name string, d, lo, hi time.Duration) {
	t.Helper()
	if d < lo || d > hi {
		t.Errorf("%s = %v, want [%v, %v]", name, d, lo, hi)
	}
}

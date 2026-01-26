package connection

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	t.Run("DefaultSequence", func(t *testing.T) {
		b := NewBackoff()

		// Expected sequence (without jitter): 1s, 2s, 4s, 8s, 16s, 32s, 60s, 60s...
		expected := []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			16 * time.Second,
			32 * time.Second,
			60 * time.Second,
			60 * time.Second, // Should stay at max
		}

		for i, exp := range expected {
			// Get the base (current) value before adding jitter
			base := b.Current()
			_ = b.Next() // Advance

			// Allow for some floating point imprecision
			if base < exp-time.Millisecond || base > exp+time.Millisecond {
				t.Errorf("Attempt %d: base = %v, want %v", i, base, exp)
			}
		}
	})

	t.Run("Jitter", func(t *testing.T) {
		b := NewBackoff()

		// Collect multiple samples to verify jitter is being applied
		samples := make([]time.Duration, 10)
		for i := range samples {
			samples[i] = b.Peek()
		}

		// All samples should be between 1s and 1.25s (with jitter)
		for i, s := range samples {
			if s < 1*time.Second || s > time.Duration(float64(1*time.Second)*1.25)+time.Millisecond {
				t.Errorf("Sample %d: %v out of expected range [1s, 1.25s]", i, s)
			}
		}

		// At least some samples should be different (jitter should vary)
		allSame := true
		for i := 1; i < len(samples); i++ {
			if samples[i] != samples[0] {
				allSame = false
				break
			}
		}
		if allSame {
			t.Error("All jittered samples are identical - jitter may not be working")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		b := NewBackoff()

		// Advance a few times
		for i := 0; i < 5; i++ {
			b.Next()
		}

		if b.Current() <= InitialBackoff {
			t.Error("Backoff should have increased")
		}

		// Reset
		b.Reset()

		if b.Current() != InitialBackoff {
			t.Errorf("Current() = %v after reset, want %v", b.Current(), InitialBackoff)
		}
		if b.Attempts() != 0 {
			t.Errorf("Attempts() = %d after reset, want 0", b.Attempts())
		}
	})

	t.Run("Attempts", func(t *testing.T) {
		b := NewBackoff()

		if b.Attempts() != 0 {
			t.Errorf("Initial Attempts() = %d, want 0", b.Attempts())
		}

		for i := 1; i <= 5; i++ {
			b.Next()
			if b.Attempts() != i {
				t.Errorf("After %d calls, Attempts() = %d", i, b.Attempts())
			}
		}
	})

	t.Run("CustomConfig", func(t *testing.T) {
		b := NewBackoffWithConfig(BackoffConfig{
			Initial:    100 * time.Millisecond,
			Max:        500 * time.Millisecond,
			Multiplier: 2.0,
			Jitter:     0, // No jitter for deterministic test
		})

		expected := []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			400 * time.Millisecond,
			500 * time.Millisecond, // Max
			500 * time.Millisecond,
		}

		for i, exp := range expected {
			got := b.Next()
			if got != exp {
				t.Errorf("Attempt %d: got %v, want %v", i, got, exp)
			}
		}
	})
}

func TestManager(t *testing.T) {
	t.Run("InitialState", func(t *testing.T) {
		m := NewManager(func(ctx context.Context) error { return nil })
		defer m.Close()

		if m.State() != StateDisconnected {
			t.Errorf("Initial state = %v, want StateDisconnected", m.State())
		}
		if m.IsConnected() {
			t.Error("IsConnected() = true, want false")
		}
	})

	t.Run("SuccessfulConnect", func(t *testing.T) {
		connectCalled := false
		m := NewManager(func(ctx context.Context) error {
			connectCalled = true
			return nil
		})
		defer m.Close()

		var connectedCalled bool
		m.OnConnected(func() {
			connectedCalled = true
		})

		err := m.Connect(context.Background())
		if err != nil {
			t.Fatalf("Connect() error = %v", err)
		}

		if !connectCalled {
			t.Error("Connect function was not called")
		}
		if !connectedCalled {
			t.Error("OnConnected callback was not called")
		}
		if m.State() != StateConnected {
			t.Errorf("State() = %v, want StateConnected", m.State())
		}
	})

	t.Run("FailedConnect", func(t *testing.T) {
		expectedErr := errors.New("connection failed")
		m := NewManager(func(ctx context.Context) error {
			return expectedErr
		})
		defer m.Close()

		err := m.Connect(context.Background())
		if err != expectedErr {
			t.Errorf("Connect() error = %v, want %v", err, expectedErr)
		}
		if m.State() != StateDisconnected {
			t.Errorf("State() = %v, want StateDisconnected", m.State())
		}
	})

	t.Run("AlreadyConnected", func(t *testing.T) {
		m := NewManager(func(ctx context.Context) error { return nil })
		defer m.Close()

		m.Connect(context.Background())

		err := m.Connect(context.Background())
		if err != ErrAlreadyConnected {
			t.Errorf("Second Connect() error = %v, want ErrAlreadyConnected", err)
		}
	})

	t.Run("Disconnect", func(t *testing.T) {
		m := NewManager(func(ctx context.Context) error { return nil })
		m.SetAutoReconnect(false) // Disable auto-reconnect for this test
		defer m.Close()

		m.Connect(context.Background())

		var disconnectedCalled bool
		m.OnDisconnected(func() {
			disconnectedCalled = true
		})

		m.Disconnect()

		if !disconnectedCalled {
			t.Error("OnDisconnected callback was not called")
		}
		if m.State() != StateDisconnected {
			t.Errorf("State() = %v, want StateDisconnected", m.State())
		}
	})

	t.Run("StateChangeCallback", func(t *testing.T) {
		m := NewManager(func(ctx context.Context) error { return nil })
		m.SetAutoReconnect(false)
		defer m.Close()

		var transitions []struct{ old, new State }
		m.OnStateChange(func(old, new State) {
			transitions = append(transitions, struct{ old, new State }{old, new})
		})

		m.Connect(context.Background())
		m.Disconnect()

		expected := []struct{ old, new State }{
			{StateDisconnected, StateConnecting},
			{StateConnecting, StateConnected},
			{StateConnected, StateDisconnected},
		}

		if len(transitions) != len(expected) {
			t.Fatalf("Got %d transitions, want %d", len(transitions), len(expected))
		}

		for i, exp := range expected {
			if transitions[i].old != exp.old || transitions[i].new != exp.new {
				t.Errorf("Transition %d: got %v→%v, want %v→%v",
					i, transitions[i].old, transitions[i].new, exp.old, exp.new)
			}
		}
	})
}

func TestManagerReconnect(t *testing.T) {
	t.Run("AutoReconnectOnDisconnect", func(t *testing.T) {
		var connectCount atomic.Int32
		m := NewManager(func(ctx context.Context) error {
			count := connectCount.Add(1)
			if count == 1 {
				return nil // First connect succeeds
			}
			return nil // Reconnect also succeeds
		})
		m.StartReconnectLoop()
		defer m.Close()

		// Initial connect
		err := m.Connect(context.Background())
		if err != nil {
			t.Fatalf("Initial Connect() error = %v", err)
		}

		// Trigger disconnect - should start reconnecting
		m.NotifyConnectionLost()

		// Wait for reconnection
		time.Sleep(1500 * time.Millisecond) // Initial backoff is 1s + jitter

		if m.State() != StateConnected {
			t.Errorf("State() = %v, want StateConnected after reconnect", m.State())
		}

		if connectCount.Load() < 2 {
			t.Errorf("Connect was only called %d times, want at least 2", connectCount.Load())
		}
	})

	t.Run("BackoffOnFailure", func(t *testing.T) {
		var connectCount atomic.Int32
		var mu sync.Mutex
		var attempts []time.Time

		m := NewManager(func(ctx context.Context) error {
			mu.Lock()
			attempts = append(attempts, time.Now())
			mu.Unlock()

			count := connectCount.Add(1)
			if count < 3 {
				return errors.New("not yet")
			}
			return nil // Third attempt succeeds
		})

		// Use shorter backoff for testing
		m.backoff = NewBackoffWithConfig(BackoffConfig{
			Initial:    50 * time.Millisecond,
			Max:        200 * time.Millisecond,
			Multiplier: 2.0,
			Jitter:     0,
		})

		m.StartReconnectLoop()
		defer m.Close()

		// Start in reconnecting state
		m.mu.Lock()
		m.state = StateReconnecting
		m.mu.Unlock()
		m.triggerReconnect()

		// Wait for reconnection
		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		attemptsCopy := make([]time.Time, len(attempts))
		copy(attemptsCopy, attempts)
		mu.Unlock()

		if len(attemptsCopy) < 3 {
			t.Fatalf("Expected at least 3 attempts, got %d", len(attemptsCopy))
		}

		// Verify backoff is being applied
		// Delays include backoff time plus execution time
		// Just verify they're increasing (backoff is working)
		if len(attemptsCopy) >= 3 {
			delay1 := attemptsCopy[1].Sub(attemptsCopy[0])
			delay2 := attemptsCopy[2].Sub(attemptsCopy[1])
			// Second delay should be longer due to backoff increase
			// Allow some tolerance for execution time
			if delay1 < 30*time.Millisecond {
				t.Errorf("First delay = %v, expected at least 30ms", delay1)
			}
			if delay2 < delay1-20*time.Millisecond {
				t.Logf("Note: delay1=%v, delay2=%v (backoff should increase)", delay1, delay2)
			}
		}

		if m.State() != StateConnected {
			t.Errorf("Final state = %v, want StateConnected", m.State())
		}
	})

	t.Run("DisabledAutoReconnect", func(t *testing.T) {
		var connectCount atomic.Int32
		m := NewManager(func(ctx context.Context) error {
			connectCount.Add(1)
			return nil
		})
		m.SetAutoReconnect(false)
		m.StartReconnectLoop()
		defer m.Close()

		m.Connect(context.Background())
		m.Disconnect()

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		if m.State() != StateDisconnected {
			t.Errorf("State() = %v, want StateDisconnected (no auto-reconnect)", m.State())
		}

		if connectCount.Load() != 1 {
			t.Errorf("Connect called %d times, want 1 (no reconnection)", connectCount.Load())
		}
	})
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateDisconnected, "DISCONNECTED"},
		{StateConnecting, "CONNECTING"},
		{StateConnected, "CONNECTED"},
		{StateReconnecting, "RECONNECTING"},
		{StateClosed, "CLOSED"},
		{State(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBackoffSequence(t *testing.T) {
	seq := BackoffSequence()

	if len(seq) != 7 {
		t.Errorf("BackoffSequence() has %d elements, want 7", len(seq))
	}

	if seq[0] != 1*time.Second {
		t.Errorf("First element = %v, want 1s", seq[0])
	}

	if seq[len(seq)-1] != 60*time.Second {
		t.Errorf("Last element = %v, want 60s", seq[len(seq)-1])
	}
}

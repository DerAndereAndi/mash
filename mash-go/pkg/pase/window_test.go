package pase

import (
	"sync"
	"testing"
	"time"
)

func TestWindowInitialState(t *testing.T) {
	w := NewWindow()

	if w.State() != WindowClosed {
		t.Errorf("State() = %v, want WindowClosed", w.State())
	}
	if !w.IsClosed() {
		t.Error("IsClosed() = false, want true")
	}
	if w.IsOpen() {
		t.Error("IsOpen() = true, want false")
	}
	if w.IsBusy() {
		t.Error("IsBusy() = true, want false")
	}
	if w.Timeout() != DefaultWindowTimeout {
		t.Errorf("Timeout() = %v, want %v", w.Timeout(), DefaultWindowTimeout)
	}
}

func TestWindowOpen(t *testing.T) {
	w := NewWindow()

	err := w.Open(TriggerButton)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if w.State() != WindowOpen {
		t.Errorf("State() = %v, want WindowOpen", w.State())
	}
	if !w.IsOpen() {
		t.Error("IsOpen() = false, want true")
	}
	if w.IsClosed() {
		t.Error("IsClosed() = true, want false")
	}

	// Remaining time should be close to timeout
	remaining := w.RemainingTime()
	if remaining < DefaultWindowTimeout-time.Second || remaining > DefaultWindowTimeout {
		t.Errorf("RemainingTime() = %v, want ~%v", remaining, DefaultWindowTimeout)
	}
}

func TestWindowClose(t *testing.T) {
	w := NewWindow()
	w.Open(TriggerButton)

	w.Close()

	if w.State() != WindowClosed {
		t.Errorf("State() = %v, want WindowClosed", w.State())
	}
	if w.RemainingTime() != 0 {
		t.Errorf("RemainingTime() = %v, want 0", w.RemainingTime())
	}
}

func TestWindowBeginPASE(t *testing.T) {
	t.Run("FromOpen", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)

		sessionID, err := w.BeginPASE()
		if err != nil {
			t.Fatalf("BeginPASE() error = %v", err)
		}
		if sessionID == "" {
			t.Error("BeginPASE() returned empty session ID")
		}

		if w.State() != WindowPASEInProgress {
			t.Errorf("State() = %v, want WindowPASEInProgress", w.State())
		}
		if !w.IsBusy() {
			t.Error("IsBusy() = false, want true")
		}
		if w.SessionID() != sessionID {
			t.Errorf("SessionID() = %q, want %q", w.SessionID(), sessionID)
		}
	})

	t.Run("FromClosed", func(t *testing.T) {
		w := NewWindow()

		_, err := w.BeginPASE()
		if err != ErrWindowClosed {
			t.Errorf("BeginPASE() error = %v, want ErrWindowClosed", err)
		}
	})

	t.Run("AlreadyInPASE", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)
		w.BeginPASE()

		_, err := w.BeginPASE()
		if err != ErrWindowBusy {
			t.Errorf("BeginPASE() error = %v, want ErrWindowBusy", err)
		}
	})
}

func TestWindowEndPASE(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)
		sessionID, _ := w.BeginPASE()

		err := w.EndPASE(sessionID, true)
		if err != nil {
			t.Fatalf("EndPASE() error = %v", err)
		}

		// Success should close the window
		if w.State() != WindowClosed {
			t.Errorf("State() = %v, want WindowClosed", w.State())
		}
		if w.SessionID() != "" {
			t.Errorf("SessionID() = %q, want empty", w.SessionID())
		}
	})

	t.Run("Failure", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)
		sessionID, _ := w.BeginPASE()

		err := w.EndPASE(sessionID, false)
		if err != nil {
			t.Fatalf("EndPASE() error = %v", err)
		}

		// Failure should return to OPEN (timeout not expired)
		if w.State() != WindowOpen {
			t.Errorf("State() = %v, want WindowOpen", w.State())
		}
	})

	t.Run("WrongSessionID", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)
		w.BeginPASE()

		err := w.EndPASE("wrong-session", true)
		if err == nil {
			t.Error("EndPASE(wrong-session) should return error")
		}
	})

	t.Run("NotInPASE", func(t *testing.T) {
		w := NewWindow()
		w.Open(TriggerButton)

		err := w.EndPASE("some-session", true)
		if err != ErrWindowNotInPASE {
			t.Errorf("EndPASE() error = %v, want ErrWindowNotInPASE", err)
		}
	})
}

func TestWindowTimeout(t *testing.T) {
	// Create window and directly manipulate the timeout for testing
	// by using a custom timer approach
	w := &Window{
		state:   WindowClosed,
		timeout: 50 * time.Millisecond, // Below minimum, but set directly for testing
	}

	var timeoutCalled bool
	var mu sync.Mutex
	w.OnTimeout(func() {
		mu.Lock()
		timeoutCalled = true
		mu.Unlock()
	})

	// Manually set up the window state to simulate Open()
	w.mu.Lock()
	w.state = WindowOpen
	w.openedAt = time.Now()
	w.timer = time.AfterFunc(w.timeout, func() {
		w.handleTimeout()
	})
	w.mu.Unlock()

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	called := timeoutCalled
	mu.Unlock()

	if !called {
		t.Error("OnTimeout callback was not called")
	}

	if w.State() != WindowClosed {
		t.Errorf("State() = %v, want WindowClosed after timeout", w.State())
	}
}

func TestWindowSetTimeoutValidation(t *testing.T) {
	w := NewWindow()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{"TooShort", 10 * time.Second, true},
		{"MinValid", MinWindowTimeout, false},
		{"Normal", 60 * time.Second, false},
		{"MaxValid", MaxWindowTimeout, false},
		{"TooLong", 600 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.SetTimeout(tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetTimeout(%v) error = %v, wantErr %v", tt.timeout, err, tt.wantErr)
			}
		})
	}
}

func TestWindowStateChangeCallback(t *testing.T) {
	w := NewWindow()

	var transitions []struct {
		old, new WindowState
	}
	w.OnStateChange(func(old, new WindowState) {
		transitions = append(transitions, struct{ old, new WindowState }{old, new})
	})

	// Open
	w.Open(TriggerButton)

	// Begin PASE
	sessionID, _ := w.BeginPASE()

	// End PASE successfully
	w.EndPASE(sessionID, true)

	// Verify transitions
	expected := []struct {
		old, new WindowState
	}{
		{WindowClosed, WindowOpen},
		{WindowOpen, WindowPASEInProgress},
		{WindowPASEInProgress, WindowClosed},
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
}

func TestWindowStateString(t *testing.T) {
	tests := []struct {
		state WindowState
		want  string
	}{
		{WindowClosed, "CLOSED"},
		{WindowOpen, "OPEN"},
		{WindowPASEInProgress, "PASE_IN_PROGRESS"},
		{WindowState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenTriggerString(t *testing.T) {
	tests := []struct {
		trigger OpenTrigger
		want    string
	}{
		{TriggerButton, "BUTTON"},
		{TriggerCommand, "COMMAND"},
		{TriggerFactoryReset, "FACTORY_RESET"},
		{OpenTrigger(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.trigger.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWindowOpenExtendsTimeout(t *testing.T) {
	// Create window with short timeout directly for testing
	w := &Window{
		state:   WindowClosed,
		timeout: 100 * time.Millisecond,
	}

	// Open the window
	w.mu.Lock()
	w.state = WindowOpen
	w.openedAt = time.Now()
	w.timer = time.AfterFunc(w.timeout, func() {
		w.handleTimeout()
	})
	w.mu.Unlock()

	// Wait a bit
	time.Sleep(50 * time.Millisecond)

	// Re-open should extend the timeout (call Open which extends if already open)
	w.Open(TriggerButton) // This will call resetTimer

	// Remaining time should be close to full timeout again
	remaining := w.RemainingTime()
	if remaining < 80*time.Millisecond {
		t.Errorf("RemainingTime() = %v, should be close to full timeout after re-open", remaining)
	}

	// Clean up timer
	w.Close()
}

func TestConcurrentAccess(t *testing.T) {
	w := NewWindow()
	w.Open(TriggerButton)

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Multiple goroutines trying to begin PASE
	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := w.BeginPASE()
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Only one should succeed
	if successCount != 1 {
		t.Errorf("Got %d successful BeginPASE calls, want exactly 1", successCount)
	}
}

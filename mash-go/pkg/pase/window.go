package pase

import (
	"errors"
	"sync"
	"time"
)

// Window state constants.
const (
	// DefaultWindowTimeout is the default commissioning window timeout.
	DefaultWindowTimeout = 120 * time.Second

	// MinWindowTimeout is the minimum allowed window timeout.
	MinWindowTimeout = 30 * time.Second

	// MaxWindowTimeout is the maximum allowed window timeout.
	MaxWindowTimeout = 300 * time.Second
)

// WindowState represents the state of the commissioning window.
type WindowState uint8

const (
	// WindowClosed indicates the device is not accepting commissioning.
	WindowClosed WindowState = iota

	// WindowOpen indicates the device is accepting commissioning attempts.
	WindowOpen

	// WindowPASEInProgress indicates a SPAKE2+ exchange is in progress.
	WindowPASEInProgress
)

// String returns a human-readable state name.
func (s WindowState) String() string {
	switch s {
	case WindowClosed:
		return "CLOSED"
	case WindowOpen:
		return "OPEN"
	case WindowPASEInProgress:
		return "PASE_IN_PROGRESS"
	default:
		return "UNKNOWN"
	}
}

// Window errors.
var (
	ErrWindowClosed      = errors.New("commissioning window is closed")
	ErrWindowBusy        = errors.New("commissioning already in progress")
	ErrWindowNotInPASE   = errors.New("not in PASE state")
	ErrInvalidTimeout    = errors.New("invalid timeout value")
)

// OpenTrigger indicates how the window was opened.
type OpenTrigger uint8

const (
	// TriggerButton indicates physical button press.
	TriggerButton OpenTrigger = iota

	// TriggerCommand indicates remote command (from existing zone).
	TriggerCommand

	// TriggerFactoryReset indicates factory reset.
	TriggerFactoryReset
)

// String returns a human-readable trigger name.
func (t OpenTrigger) String() string {
	switch t {
	case TriggerButton:
		return "BUTTON"
	case TriggerCommand:
		return "COMMAND"
	case TriggerFactoryReset:
		return "FACTORY_RESET"
	default:
		return "UNKNOWN"
	}
}

// Window manages the commissioning window state machine.
type Window struct {
	mu sync.RWMutex

	// Current state
	state WindowState

	// Timeout for the open window
	timeout time.Duration

	// Timer for auto-close
	timer *time.Timer

	// When the window was opened
	openedAt time.Time

	// How the window was opened
	openTrigger OpenTrigger

	// Session ID for current PASE session (if any)
	sessionID string

	// Callbacks
	onStateChange func(oldState, newState WindowState)
	onTimeout     func()
}

// NewWindow creates a new commissioning window with default settings.
func NewWindow() *Window {
	return &Window{
		state:   WindowClosed,
		timeout: DefaultWindowTimeout,
	}
}

// State returns the current window state.
func (w *Window) State() WindowState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

// IsOpen returns true if the window is open (accepting commissioning).
func (w *Window) IsOpen() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state == WindowOpen
}

// IsBusy returns true if a PASE session is in progress.
func (w *Window) IsBusy() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state == WindowPASEInProgress
}

// IsClosed returns true if the window is closed.
func (w *Window) IsClosed() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state == WindowClosed
}

// RemainingTime returns the time remaining until the window closes.
// Returns 0 if the window is closed.
func (w *Window) RemainingTime() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.state == WindowClosed {
		return 0
	}

	elapsed := time.Since(w.openedAt)
	remaining := w.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// SetTimeout sets the window timeout.
// Must be called before opening the window.
func (w *Window) SetTimeout(d time.Duration) error {
	if d < MinWindowTimeout || d > MaxWindowTimeout {
		return ErrInvalidTimeout
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.timeout = d
	return nil
}

// Timeout returns the current timeout setting.
func (w *Window) Timeout() time.Duration {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.timeout
}

// Open opens the commissioning window.
// Returns an error if already open or in PASE.
func (w *Window) Open(trigger OpenTrigger) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != WindowClosed {
		// Already open - extend the timeout
		w.resetTimer()
		return nil
	}

	oldState := w.state
	w.state = WindowOpen
	w.openedAt = time.Now()
	w.openTrigger = trigger
	w.sessionID = ""

	// Start timeout timer
	w.timer = time.AfterFunc(w.timeout, func() {
		w.handleTimeout()
	})

	if w.onStateChange != nil {
		w.onStateChange(oldState, w.state)
	}

	return nil
}

// Close closes the commissioning window.
func (w *Window) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == WindowClosed {
		return
	}

	oldState := w.state
	w.state = WindowClosed
	w.sessionID = ""

	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}

	if w.onStateChange != nil {
		w.onStateChange(oldState, w.state)
	}
}

// BeginPASE transitions to PASE_IN_PROGRESS state.
// Returns a session ID on success.
// Returns ErrWindowClosed if window is closed.
// Returns ErrWindowBusy if another PASE session is in progress.
func (w *Window) BeginPASE() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch w.state {
	case WindowClosed:
		return "", ErrWindowClosed
	case WindowPASEInProgress:
		return "", ErrWindowBusy
	case WindowOpen:
		// OK to proceed
	}

	oldState := w.state
	w.state = WindowPASEInProgress
	w.sessionID = generateSessionID()

	if w.onStateChange != nil {
		w.onStateChange(oldState, w.state)
	}

	return w.sessionID, nil
}

// EndPASE ends a PASE session.
// If success is true, the window closes (commissioning complete).
// If success is false, the window returns to OPEN (if not timed out).
func (w *Window) EndPASE(sessionID string, success bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != WindowPASEInProgress {
		return ErrWindowNotInPASE
	}

	if w.sessionID != sessionID {
		return errors.New("invalid session ID")
	}

	oldState := w.state

	if success {
		// Commissioning complete - close window
		w.state = WindowClosed
		w.sessionID = ""
		if w.timer != nil {
			w.timer.Stop()
			w.timer = nil
		}
	} else {
		// Failed - return to OPEN if not timed out
		if w.RemainingTimeUnlocked() > 0 {
			w.state = WindowOpen
		} else {
			w.state = WindowClosed
			if w.timer != nil {
				w.timer.Stop()
				w.timer = nil
			}
		}
		w.sessionID = ""
	}

	if w.onStateChange != nil {
		w.onStateChange(oldState, w.state)
	}

	return nil
}

// SessionID returns the current PASE session ID.
// Returns empty string if no session is active.
func (w *Window) SessionID() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessionID
}

// OnStateChange sets a callback for state changes.
func (w *Window) OnStateChange(fn func(oldState, newState WindowState)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onStateChange = fn
}

// OnTimeout sets a callback for when the window times out.
func (w *Window) OnTimeout(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onTimeout = fn
}

// handleTimeout is called when the window timer expires.
func (w *Window) handleTimeout() {
	w.mu.Lock()

	if w.state == WindowClosed {
		w.mu.Unlock()
		return
	}

	oldState := w.state
	w.state = WindowClosed
	w.sessionID = ""
	w.timer = nil

	// Capture callbacks before releasing lock
	stateChangeFn := w.onStateChange
	timeoutFn := w.onTimeout

	w.mu.Unlock()

	// Call callbacks outside lock to prevent deadlock
	if stateChangeFn != nil {
		stateChangeFn(oldState, WindowClosed)
	}
	if timeoutFn != nil {
		timeoutFn()
	}
}

// resetTimer resets the timeout timer.
func (w *Window) resetTimer() {
	if w.timer != nil {
		w.timer.Stop()
	}
	w.openedAt = time.Now()
	w.timer = time.AfterFunc(w.timeout, func() {
		w.handleTimeout()
	})
}

// RemainingTimeUnlocked returns remaining time without taking the lock.
// Only call when lock is already held.
func (w *Window) RemainingTimeUnlocked() time.Duration {
	if w.state == WindowClosed {
		return 0
	}

	elapsed := time.Since(w.openedAt)
	remaining := w.timeout - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return time.Now().Format("20060102150405.000000000")
}

package failsafe

import (
	"errors"
	"sync"
	"time"
)

// Failsafe timer constants.
const (
	// MinDuration is the minimum failsafe duration.
	MinDuration = 2 * time.Hour

	// MaxDuration is the maximum failsafe duration.
	MaxDuration = 24 * time.Hour

	// DefaultDuration is the default failsafe duration.
	DefaultDuration = 4 * time.Hour

	// DefaultGracePeriod is the default grace period after reconnection.
	DefaultGracePeriod = 5 * time.Minute

	// AccuracyPercent is the timer accuracy as a percentage.
	AccuracyPercent = 1

	// AccuracyAbsolute is the minimum timer accuracy in seconds.
	AccuracyAbsolute = 60 * time.Second
)

// Timer errors.
var (
	ErrInvalidDuration = errors.New("invalid failsafe duration")
	ErrTimerNotRunning = errors.New("failsafe timer not running")
)

// State represents the failsafe state.
type State uint8

const (
	// StateNormal indicates normal operation (not in failsafe).
	StateNormal State = iota

	// StateTimerRunning indicates the failsafe timer is running.
	StateTimerRunning

	// StateFailsafe indicates failsafe mode is active.
	StateFailsafe

	// StateGracePeriod indicates the grace period after reconnection.
	StateGracePeriod
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateNormal:
		return "NORMAL"
	case StateTimerRunning:
		return "TIMER_RUNNING"
	case StateFailsafe:
		return "FAILSAFE"
	case StateGracePeriod:
		return "GRACE_PERIOD"
	default:
		return "UNKNOWN"
	}
}

// Limits holds the failsafe power limits.
type Limits struct {
	// ConsumptionLimit is the maximum consumption in watts (positive).
	// Zero means no limit (or not applicable).
	ConsumptionLimit int64

	// ProductionLimit is the maximum production in watts (negative, closer to zero is more restrictive).
	// Zero means no limit (or not applicable).
	ProductionLimit int64

	// HasConsumptionLimit indicates if ConsumptionLimit is set.
	HasConsumptionLimit bool

	// HasProductionLimit indicates if ProductionLimit is set.
	HasProductionLimit bool
}

// Timer manages the failsafe timer for a MASH device.
type Timer struct {
	mu sync.RWMutex

	// Current state
	state State

	// Failsafe duration
	duration time.Duration

	// Grace period duration
	gracePeriod time.Duration

	// Failsafe limits to apply
	limits Limits

	// Timer instances
	failsafeTimer   *time.Timer
	gracePeriodTimer *time.Timer

	// When the timer was started
	startedAt time.Time

	// Persistence support
	persistEnabled bool

	// Callbacks
	onStateChange   func(oldState, newState State)
	onFailsafeEnter func(limits Limits)
	onFailsafeExit  func()
}

// NewTimer creates a new failsafe timer with default settings.
func NewTimer() *Timer {
	return &Timer{
		state:       StateNormal,
		duration:    DefaultDuration,
		gracePeriod: DefaultGracePeriod,
	}
}

// Config holds failsafe timer configuration.
type Config struct {
	Duration         time.Duration
	GracePeriod      time.Duration
	Limits           Limits
	PersistEnabled   bool
}

// NewTimerWithConfig creates a failsafe timer with custom configuration.
func NewTimerWithConfig(cfg Config) (*Timer, error) {
	if cfg.Duration != 0 && (cfg.Duration < MinDuration || cfg.Duration > MaxDuration) {
		return nil, ErrInvalidDuration
	}

	t := &Timer{
		state:          StateNormal,
		duration:       cfg.Duration,
		gracePeriod:    cfg.GracePeriod,
		limits:         cfg.Limits,
		persistEnabled: cfg.PersistEnabled,
	}

	if t.duration == 0 {
		t.duration = DefaultDuration
	}
	if t.gracePeriod == 0 {
		t.gracePeriod = DefaultGracePeriod
	}

	return t, nil
}

// State returns the current failsafe state.
func (t *Timer) State() State {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// IsFailsafe returns true if in failsafe mode.
func (t *Timer) IsFailsafe() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state == StateFailsafe
}

// IsTimerRunning returns true if the failsafe timer is running.
func (t *Timer) IsTimerRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state == StateTimerRunning
}

// Duration returns the configured failsafe duration.
func (t *Timer) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.duration
}

// SetDuration sets the failsafe duration.
func (t *Timer) SetDuration(d time.Duration) error {
	if d < MinDuration || d > MaxDuration {
		return ErrInvalidDuration
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.duration = d
	return nil
}

// SetLimits sets the failsafe limits.
func (t *Timer) SetLimits(limits Limits) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.limits = limits
}

// Limits returns the current failsafe limits.
func (t *Timer) Limits() Limits {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.limits
}

// Start starts the failsafe timer.
// Called when connection to all zones is lost.
func (t *Timer) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == StateTimerRunning || t.state == StateFailsafe {
		return // Already running or in failsafe
	}

	oldState := t.state
	t.state = StateTimerRunning
	t.startedAt = time.Now()

	// Start the timer
	t.failsafeTimer = time.AfterFunc(t.duration, func() {
		t.enterFailsafe()
	})

	if t.onStateChange != nil {
		t.onStateChange(oldState, t.state)
	}
}

// Stop stops the failsafe timer.
// Called when any zone reconnects.
func (t *Timer) Stop() {
	t.mu.Lock()

	if t.state == StateNormal {
		t.mu.Unlock()
		return
	}

	oldState := t.state
	wasFailsafe := t.state == StateFailsafe

	// Stop any running timers
	if t.failsafeTimer != nil {
		t.failsafeTimer.Stop()
		t.failsafeTimer = nil
	}

	// If we were in failsafe, enter grace period (if configured)
	if wasFailsafe && t.gracePeriod > 0 {
		t.state = StateGracePeriod
		t.gracePeriodTimer = time.AfterFunc(t.gracePeriod, func() {
			t.exitGracePeriod()
		})
	} else {
		t.state = StateNormal
	}

	stateChangeFn := t.onStateChange
	failsafeExitFn := t.onFailsafeExit
	newState := t.state

	t.mu.Unlock()

	if stateChangeFn != nil {
		stateChangeFn(oldState, newState)
	}
	if wasFailsafe && failsafeExitFn != nil {
		failsafeExitFn()
	}
}

// Reset fully resets the timer to normal state.
func (t *Timer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	oldState := t.state

	if t.failsafeTimer != nil {
		t.failsafeTimer.Stop()
		t.failsafeTimer = nil
	}
	if t.gracePeriodTimer != nil {
		t.gracePeriodTimer.Stop()
		t.gracePeriodTimer = nil
	}

	t.state = StateNormal
	t.startedAt = time.Time{}

	if t.onStateChange != nil && oldState != StateNormal {
		t.onStateChange(oldState, StateNormal)
	}
}

// RemainingTime returns the time remaining until failsafe triggers.
// Returns 0 if timer is not running.
func (t *Timer) RemainingTime() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.state != StateTimerRunning {
		return 0
	}

	elapsed := time.Since(t.startedAt)
	remaining := t.duration - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// enterFailsafe is called when the timer expires.
func (t *Timer) enterFailsafe() {
	t.mu.Lock()

	if t.state != StateTimerRunning {
		t.mu.Unlock()
		return
	}

	oldState := t.state
	t.state = StateFailsafe
	t.failsafeTimer = nil

	stateChangeFn := t.onStateChange
	failsafeEnterFn := t.onFailsafeEnter
	limits := t.limits

	t.mu.Unlock()

	if stateChangeFn != nil {
		stateChangeFn(oldState, StateFailsafe)
	}
	if failsafeEnterFn != nil {
		failsafeEnterFn(limits)
	}
}

// exitGracePeriod is called when the grace period expires.
func (t *Timer) exitGracePeriod() {
	t.mu.Lock()

	if t.state != StateGracePeriod {
		t.mu.Unlock()
		return
	}

	oldState := t.state
	t.state = StateNormal
	t.gracePeriodTimer = nil

	stateChangeFn := t.onStateChange

	t.mu.Unlock()

	if stateChangeFn != nil {
		stateChangeFn(oldState, StateNormal)
	}
}

// OnStateChange sets a callback for state changes.
func (t *Timer) OnStateChange(fn func(oldState, newState State)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onStateChange = fn
}

// OnFailsafeEnter sets a callback for entering failsafe mode.
func (t *Timer) OnFailsafeEnter(fn func(limits Limits)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onFailsafeEnter = fn
}

// OnFailsafeExit sets a callback for exiting failsafe mode.
func (t *Timer) OnFailsafeExit(fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onFailsafeExit = fn
}

// CalculateAccuracy returns the timer accuracy for a given duration.
// Accuracy is +/- 1% or 60 seconds, whichever is greater.
func CalculateAccuracy(d time.Duration) time.Duration {
	percentAccuracy := time.Duration(float64(d) * float64(AccuracyPercent) / 100)
	if percentAccuracy > AccuracyAbsolute {
		return percentAccuracy
	}
	return AccuracyAbsolute
}

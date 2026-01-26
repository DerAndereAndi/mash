package duration

import (
	"errors"
	"sync"
	"time"
)

// Duration timer errors.
var (
	ErrTimerNotFound  = errors.New("timer not found")
	ErrInvalidDuration = errors.New("invalid duration")
)

// Duration limits.
const (
	// MinDuration is the minimum allowed duration (1 second).
	MinDuration = 1 * time.Second

	// MaxDuration is the maximum allowed duration (24 hours).
	MaxDuration = 24 * time.Hour

	// AccuracyPercent is the timer accuracy as a percentage.
	AccuracyPercent = 1

	// AccuracyAbsolute is the minimum timer accuracy.
	AccuracyAbsolute = 1 * time.Second
)

// CommandType identifies the type of command with duration.
type CommandType uint8

const (
	// CmdLimitConsumption is SetLimit with consumptionLimit.
	CmdLimitConsumption CommandType = iota + 1

	// CmdLimitProduction is SetLimit with productionLimit.
	CmdLimitProduction

	// CmdCurrentLimitConsumption is SetCurrentLimits with consumptionLimit.
	CmdCurrentLimitConsumption

	// CmdCurrentLimitProduction is SetCurrentLimits with productionLimit.
	CmdCurrentLimitProduction

	// CmdSetpointConsumption is SetSetpoint with consumptionSetpoint.
	CmdSetpointConsumption

	// CmdSetpointProduction is SetSetpoint with productionSetpoint.
	CmdSetpointProduction

	// CmdCurrentSetpointConsumption is SetCurrentSetpoints with consumptionSetpoint.
	CmdCurrentSetpointConsumption

	// CmdCurrentSetpointProduction is SetCurrentSetpoints with productionSetpoint.
	CmdCurrentSetpointProduction

	// CmdPause is the Pause command.
	CmdPause
)

// String returns a human-readable command type name.
func (c CommandType) String() string {
	switch c {
	case CmdLimitConsumption:
		return "LIMIT_CONSUMPTION"
	case CmdLimitProduction:
		return "LIMIT_PRODUCTION"
	case CmdCurrentLimitConsumption:
		return "CURRENT_LIMIT_CONSUMPTION"
	case CmdCurrentLimitProduction:
		return "CURRENT_LIMIT_PRODUCTION"
	case CmdSetpointConsumption:
		return "SETPOINT_CONSUMPTION"
	case CmdSetpointProduction:
		return "SETPOINT_PRODUCTION"
	case CmdCurrentSetpointConsumption:
		return "CURRENT_SETPOINT_CONSUMPTION"
	case CmdCurrentSetpointProduction:
		return "CURRENT_SETPOINT_PRODUCTION"
	case CmdPause:
		return "PAUSE"
	default:
		return "UNKNOWN"
	}
}

// timerKey uniquely identifies a timer.
type timerKey struct {
	zoneID  uint8
	cmdType CommandType
}

// Timer represents an active duration timer.
type Timer struct {
	// Key identifies this timer
	Key timerKey

	// StartTime is when the timer started (monotonic-like)
	StartTime time.Time

	// Duration is the timer duration
	Duration time.Duration

	// Value is the command value that will be cleared on expiry
	Value any

	// timer is the Go timer for automatic expiry
	timer *time.Timer
}

// ExpiresAt returns when the timer will expire.
func (t *Timer) ExpiresAt() time.Time {
	return t.StartTime.Add(t.Duration)
}

// RemainingTime returns time until expiry.
func (t *Timer) RemainingTime() time.Duration {
	remaining := t.Duration - time.Since(t.StartTime)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired returns true if the timer has expired.
func (t *Timer) IsExpired() bool {
	return time.Since(t.StartTime) >= t.Duration
}

// Manager manages duration timers for commands.
type Manager struct {
	mu sync.RWMutex

	// Active timers by (zoneID, commandType)
	timers map[timerKey]*Timer

	// Callback when timer expires
	onExpiry func(zoneID uint8, cmdType CommandType, value any)
}

// NewManager creates a new duration timer manager.
func NewManager() *Manager {
	return &Manager{
		timers: make(map[timerKey]*Timer),
	}
}

// SetTimer creates or replaces a duration timer.
// The timer starts immediately (on receipt of this call).
// Returns an error if duration is invalid.
func (m *Manager) SetTimer(zoneID uint8, cmdType CommandType, duration time.Duration, value any) error {
	if duration < MinDuration {
		return ErrInvalidDuration
	}
	if duration > MaxDuration {
		return ErrInvalidDuration
	}

	key := timerKey{zoneID: zoneID, cmdType: cmdType}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel existing timer if any
	if existing, exists := m.timers[key]; exists {
		if existing.timer != nil {
			existing.timer.Stop()
		}
	}

	// Create new timer
	timer := &Timer{
		Key:       key,
		StartTime: time.Now(),
		Duration:  duration,
		Value:     value,
	}

	// Set up automatic expiry
	timer.timer = time.AfterFunc(duration, func() {
		m.expireTimer(key)
	})

	m.timers[key] = timer
	return nil
}

// CancelTimer cancels a timer without triggering expiry callback.
func (m *Manager) CancelTimer(zoneID uint8, cmdType CommandType) error {
	key := timerKey{zoneID: zoneID, cmdType: cmdType}

	m.mu.Lock()
	defer m.mu.Unlock()

	timer, exists := m.timers[key]
	if !exists {
		return ErrTimerNotFound
	}

	if timer.timer != nil {
		timer.timer.Stop()
	}
	delete(m.timers, key)
	return nil
}

// CancelZoneTimers cancels all timers for a zone (e.g., on connection loss).
func (m *Manager) CancelZoneTimers(zoneID uint8) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for key, timer := range m.timers {
		if key.zoneID == zoneID {
			if timer.timer != nil {
				timer.timer.Stop()
			}
			delete(m.timers, key)
		}
	}
}

// GetTimer returns timer info for a specific command, or nil if not set.
func (m *Manager) GetTimer(zoneID uint8, cmdType CommandType) *Timer {
	key := timerKey{zoneID: zoneID, cmdType: cmdType}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if timer, exists := m.timers[key]; exists {
		// Return a copy to avoid race conditions
		return &Timer{
			Key:       timer.Key,
			StartTime: timer.StartTime,
			Duration:  timer.Duration,
			Value:     timer.Value,
		}
	}
	return nil
}

// GetZoneTimers returns all active timers for a zone.
func (m *Manager) GetZoneTimers(zoneID uint8) []*Timer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Timer
	for key, timer := range m.timers {
		if key.zoneID == zoneID {
			result = append(result, &Timer{
				Key:       timer.Key,
				StartTime: timer.StartTime,
				Duration:  timer.Duration,
				Value:     timer.Value,
			})
		}
	}
	return result
}

// Count returns the total number of active timers.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.timers)
}

// OnExpiry sets the callback for timer expiry.
// The callback receives the zone ID, command type, and the value that was set.
func (m *Manager) OnExpiry(fn func(zoneID uint8, cmdType CommandType, value any)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExpiry = fn
}

// expireTimer handles timer expiry.
func (m *Manager) expireTimer(key timerKey) {
	m.mu.Lock()

	timer, exists := m.timers[key]
	if !exists {
		m.mu.Unlock()
		return
	}

	value := timer.Value
	delete(m.timers, key)

	callback := m.onExpiry

	m.mu.Unlock()

	// Call callback outside lock
	if callback != nil {
		callback(key.zoneID, key.cmdType, value)
	}
}

// CalculateAccuracy returns the timer accuracy for a given duration.
// Accuracy is +/- 1% or +/- 1 second, whichever is greater.
func CalculateAccuracy(d time.Duration) time.Duration {
	percentAccuracy := time.Duration(float64(d) * float64(AccuracyPercent) / 100)
	if percentAccuracy > AccuracyAbsolute {
		return percentAccuracy
	}
	return AccuracyAbsolute
}

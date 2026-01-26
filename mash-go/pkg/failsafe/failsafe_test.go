package failsafe

import (
	"sync"
	"testing"
	"time"
)

func TestTimerInitialState(t *testing.T) {
	timer := NewTimer()

	if timer.State() != StateNormal {
		t.Errorf("State() = %v, want StateNormal", timer.State())
	}
	if timer.IsFailsafe() {
		t.Error("IsFailsafe() = true, want false")
	}
	if timer.IsTimerRunning() {
		t.Error("IsTimerRunning() = true, want false")
	}
	if timer.Duration() != DefaultDuration {
		t.Errorf("Duration() = %v, want %v", timer.Duration(), DefaultDuration)
	}
}

func TestTimerSetDuration(t *testing.T) {
	timer := NewTimer()

	tests := []struct {
		name    string
		dur     time.Duration
		wantErr bool
	}{
		{"TooShort", 1 * time.Hour, true},
		{"MinValid", MinDuration, false},
		{"Normal", 6 * time.Hour, false},
		{"MaxValid", MaxDuration, false},
		{"TooLong", 25 * time.Hour, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := timer.SetDuration(tt.dur)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetDuration(%v) error = %v, wantErr %v", tt.dur, err, tt.wantErr)
			}
		})
	}
}

func TestTimerStartStop(t *testing.T) {
	timer := NewTimer()

	// Start timer
	timer.Start()

	if timer.State() != StateTimerRunning {
		t.Errorf("State() = %v, want StateTimerRunning", timer.State())
	}
	if !timer.IsTimerRunning() {
		t.Error("IsTimerRunning() = false, want true")
	}

	// Remaining time should be close to duration
	remaining := timer.RemainingTime()
	if remaining < timer.Duration()-time.Second || remaining > timer.Duration() {
		t.Errorf("RemainingTime() = %v, expected ~%v", remaining, timer.Duration())
	}

	// Stop timer
	timer.Stop()

	if timer.State() != StateNormal {
		t.Errorf("State() = %v, want StateNormal", timer.State())
	}
	if timer.RemainingTime() != 0 {
		t.Errorf("RemainingTime() = %v, want 0", timer.RemainingTime())
	}
}

func TestTimerFailsafeTriggered(t *testing.T) {
	// Create timer with very short duration for testing
	timer := &Timer{
		state:       StateNormal,
		duration:    50 * time.Millisecond,
		gracePeriod: 0, // No grace period
		limits: Limits{
			ConsumptionLimit:    3000,
			HasConsumptionLimit: true,
		},
	}

	var failsafeEntered bool
	var enteredLimits Limits
	var mu sync.Mutex

	timer.OnFailsafeEnter(func(limits Limits) {
		mu.Lock()
		failsafeEntered = true
		enteredLimits = limits
		mu.Unlock()
	})

	timer.Start()

	// Wait for failsafe to trigger
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	entered := failsafeEntered
	limits := enteredLimits
	mu.Unlock()

	if !entered {
		t.Error("OnFailsafeEnter callback was not called")
	}
	if timer.State() != StateFailsafe {
		t.Errorf("State() = %v, want StateFailsafe", timer.State())
	}
	if !timer.IsFailsafe() {
		t.Error("IsFailsafe() = false, want true")
	}
	if limits.ConsumptionLimit != 3000 {
		t.Errorf("Limits.ConsumptionLimit = %d, want 3000", limits.ConsumptionLimit)
	}
}

func TestTimerGracePeriod(t *testing.T) {
	// Create timer with short durations for testing
	timer := &Timer{
		state:       StateNormal,
		duration:    30 * time.Millisecond,
		gracePeriod: 50 * time.Millisecond,
	}

	timer.Start()

	// Wait for failsafe
	time.Sleep(50 * time.Millisecond)

	if timer.State() != StateFailsafe {
		t.Fatalf("State() = %v, want StateFailsafe", timer.State())
	}

	// Stop (reconnect) - should enter grace period
	timer.Stop()

	if timer.State() != StateGracePeriod {
		t.Errorf("State() = %v, want StateGracePeriod", timer.State())
	}

	// Wait for grace period to expire
	time.Sleep(80 * time.Millisecond)

	if timer.State() != StateNormal {
		t.Errorf("State() = %v, want StateNormal after grace period", timer.State())
	}
}

func TestTimerReset(t *testing.T) {
	timer := NewTimer()
	timer.Start()

	// Reset while timer is running
	timer.Reset()

	if timer.State() != StateNormal {
		t.Errorf("State() = %v, want StateNormal after reset", timer.State())
	}
	if timer.RemainingTime() != 0 {
		t.Errorf("RemainingTime() = %v, want 0 after reset", timer.RemainingTime())
	}
}

func TestTimerStateChangeCallback(t *testing.T) {
	timer := &Timer{
		state:       StateNormal,
		duration:    30 * time.Millisecond,
		gracePeriod: 0,
	}

	var transitions []struct{ old, new State }
	var mu sync.Mutex

	timer.OnStateChange(func(old, new State) {
		mu.Lock()
		transitions = append(transitions, struct{ old, new State }{old, new})
		mu.Unlock()
	})

	timer.Start()
	time.Sleep(50 * time.Millisecond) // Wait for failsafe
	timer.Stop()

	mu.Lock()
	defer mu.Unlock()

	expected := []struct{ old, new State }{
		{StateNormal, StateTimerRunning},
		{StateTimerRunning, StateFailsafe},
		{StateFailsafe, StateNormal}, // No grace period
	}

	if len(transitions) != len(expected) {
		t.Fatalf("Got %d transitions, want %d: %v", len(transitions), len(expected), transitions)
	}

	for i, exp := range expected {
		if transitions[i].old != exp.old || transitions[i].new != exp.new {
			t.Errorf("Transition %d: got %v→%v, want %v→%v",
				i, transitions[i].old, transitions[i].new, exp.old, exp.new)
		}
	}
}

func TestTimerLimits(t *testing.T) {
	timer := NewTimer()

	limits := Limits{
		ConsumptionLimit:    5000,
		HasConsumptionLimit: true,
		ProductionLimit:     -3000,
		HasProductionLimit:  true,
	}

	timer.SetLimits(limits)

	got := timer.Limits()
	if got.ConsumptionLimit != 5000 {
		t.Errorf("ConsumptionLimit = %d, want 5000", got.ConsumptionLimit)
	}
	if got.ProductionLimit != -3000 {
		t.Errorf("ProductionLimit = %d, want -3000", got.ProductionLimit)
	}
	if !got.HasConsumptionLimit {
		t.Error("HasConsumptionLimit = false, want true")
	}
	if !got.HasProductionLimit {
		t.Error("HasProductionLimit = false, want true")
	}
}

func TestTimerWithConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := Config{
			Duration:    8 * time.Hour,
			GracePeriod: 10 * time.Minute,
			Limits: Limits{
				ConsumptionLimit:    4000,
				HasConsumptionLimit: true,
			},
		}

		timer, err := NewTimerWithConfig(cfg)
		if err != nil {
			t.Fatalf("NewTimerWithConfig() error = %v", err)
		}

		if timer.Duration() != 8*time.Hour {
			t.Errorf("Duration() = %v, want 8h", timer.Duration())
		}
		if timer.Limits().ConsumptionLimit != 4000 {
			t.Errorf("Limits().ConsumptionLimit = %d, want 4000", timer.Limits().ConsumptionLimit)
		}
	})

	t.Run("InvalidDuration", func(t *testing.T) {
		cfg := Config{
			Duration: 1 * time.Hour, // Too short
		}

		_, err := NewTimerWithConfig(cfg)
		if err != ErrInvalidDuration {
			t.Errorf("NewTimerWithConfig() error = %v, want ErrInvalidDuration", err)
		}
	})
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNormal, "NORMAL"},
		{StateTimerRunning, "TIMER_RUNNING"},
		{StateFailsafe, "FAILSAFE"},
		{StateGracePeriod, "GRACE_PERIOD"},
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

func TestCalculateAccuracy(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     time.Duration
	}{
		{"ShortDuration", 30 * time.Minute, AccuracyAbsolute}, // 1% of 30min = 18s < 60s, use 60s
		{"MediumDuration", 2 * time.Hour, 72 * time.Second},   // 1% of 2h = 72s > 60s, use 72s
		{"LongDuration", 10 * time.Hour, 6 * time.Minute},     // 1% of 10h = 6min
		{"MaxDuration", 24 * time.Hour, 864 * time.Second},    // 1% of 24h = 14.4min
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateAccuracy(tt.duration)
			// Allow for some floating point tolerance
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Errorf("CalculateAccuracy(%v) = %v, want ~%v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestTimerIdempotentStart(t *testing.T) {
	timer := NewTimer()

	timer.Start()
	state1 := timer.State()

	// Starting again should be idempotent
	timer.Start()
	state2 := timer.State()

	if state1 != state2 {
		t.Errorf("State changed on second Start(): %v → %v", state1, state2)
	}
}

func TestTimerIdempotentStop(t *testing.T) {
	timer := NewTimer()

	// Stopping when already stopped should be idempotent
	timer.Stop()

	if timer.State() != StateNormal {
		t.Errorf("State() = %v after Stop on already stopped, want StateNormal", timer.State())
	}
}

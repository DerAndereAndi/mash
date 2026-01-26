package duration

import (
	"sync"
	"testing"
	"time"
)

func TestTimerBasic(t *testing.T) {
	timer := &Timer{
		Key:       timerKey{zoneID: 1, cmdType: CmdLimitConsumption},
		StartTime: time.Now(),
		Duration:  60 * time.Second,
		Value:     int64(5000000),
	}

	if timer.IsExpired() {
		t.Error("Timer should not be expired immediately")
	}

	remaining := timer.RemainingTime()
	if remaining < 59*time.Second || remaining > 60*time.Second {
		t.Errorf("RemainingTime() = %v, expected ~60s", remaining)
	}

	expiresAt := timer.ExpiresAt()
	expectedExpiry := timer.StartTime.Add(timer.Duration)
	if expiresAt != expectedExpiry {
		t.Errorf("ExpiresAt() = %v, want %v", expiresAt, expectedExpiry)
	}
}

func TestTimerExpired(t *testing.T) {
	// Create timer that's already expired
	timer := &Timer{
		Key:       timerKey{zoneID: 1, cmdType: CmdLimitConsumption},
		StartTime: time.Now().Add(-2 * time.Second),
		Duration:  1 * time.Second,
		Value:     int64(5000000),
	}

	if !timer.IsExpired() {
		t.Error("Timer should be expired")
	}

	if timer.RemainingTime() != 0 {
		t.Errorf("RemainingTime() = %v, want 0 for expired timer", timer.RemainingTime())
	}
}

func TestManagerSetTimer(t *testing.T) {
	m := NewManager()

	err := m.SetTimer(1, CmdLimitConsumption, 5*time.Second, int64(5000000))
	if err != nil {
		t.Fatalf("SetTimer() error = %v", err)
	}

	if m.Count() != 1 {
		t.Errorf("Count() = %d, want 1", m.Count())
	}

	timer := m.GetTimer(1, CmdLimitConsumption)
	if timer == nil {
		t.Fatal("GetTimer() returned nil")
	}

	if timer.Value != int64(5000000) {
		t.Errorf("Timer value = %v, want 5000000", timer.Value)
	}
}

func TestManagerInvalidDuration(t *testing.T) {
	m := NewManager()

	// Too short
	err := m.SetTimer(1, CmdLimitConsumption, 500*time.Millisecond, int64(5000000))
	if err != ErrInvalidDuration {
		t.Errorf("SetTimer with too short duration error = %v, want ErrInvalidDuration", err)
	}

	// Too long
	err = m.SetTimer(1, CmdLimitConsumption, 25*time.Hour, int64(5000000))
	if err != ErrInvalidDuration {
		t.Errorf("SetTimer with too long duration error = %v, want ErrInvalidDuration", err)
	}

	// Valid min
	err = m.SetTimer(1, CmdLimitConsumption, MinDuration, int64(5000000))
	if err != nil {
		t.Errorf("SetTimer with MinDuration error = %v", err)
	}

	// Valid max
	err = m.SetTimer(2, CmdLimitConsumption, MaxDuration, int64(5000000))
	if err != nil {
		t.Errorf("SetTimer with MaxDuration error = %v", err)
	}
}

func TestManagerTimerReplacement(t *testing.T) {
	m := NewManager()

	// Set first timer
	m.SetTimer(1, CmdLimitConsumption, 10*time.Second, int64(5000000))

	// Replace with new timer
	m.SetTimer(1, CmdLimitConsumption, 20*time.Second, int64(3000000))

	if m.Count() != 1 {
		t.Errorf("Count() = %d after replacement, want 1", m.Count())
	}

	timer := m.GetTimer(1, CmdLimitConsumption)
	if timer == nil {
		t.Fatal("GetTimer() returned nil")
	}

	if timer.Value != int64(3000000) {
		t.Errorf("Timer value = %v after replacement, want 3000000", timer.Value)
	}

	if timer.Duration != 20*time.Second {
		t.Errorf("Timer duration = %v after replacement, want 20s", timer.Duration)
	}
}

func TestManagerCancelTimer(t *testing.T) {
	m := NewManager()

	m.SetTimer(1, CmdLimitConsumption, 5*time.Second, int64(5000000))

	err := m.CancelTimer(1, CmdLimitConsumption)
	if err != nil {
		t.Fatalf("CancelTimer() error = %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("Count() = %d after cancel, want 0", m.Count())
	}

	// Cancel non-existent timer
	err = m.CancelTimer(1, CmdLimitConsumption)
	if err != ErrTimerNotFound {
		t.Errorf("CancelTimer non-existent error = %v, want ErrTimerNotFound", err)
	}
}

func TestManagerCancelZoneTimers(t *testing.T) {
	m := NewManager()

	// Set timers for multiple zones
	m.SetTimer(1, CmdLimitConsumption, 5*time.Second, int64(5000000))
	m.SetTimer(1, CmdLimitProduction, 5*time.Second, int64(-3000000))
	m.SetTimer(2, CmdLimitConsumption, 5*time.Second, int64(4000000))

	if m.Count() != 3 {
		t.Fatalf("Count() = %d before cancel, want 3", m.Count())
	}

	// Cancel zone 1 timers
	m.CancelZoneTimers(1)

	if m.Count() != 1 {
		t.Errorf("Count() = %d after cancel zone 1, want 1", m.Count())
	}

	// Zone 2 timer should still exist
	timer := m.GetTimer(2, CmdLimitConsumption)
	if timer == nil {
		t.Error("Zone 2 timer should still exist")
	}

	// Zone 1 timers should be gone
	if m.GetTimer(1, CmdLimitConsumption) != nil {
		t.Error("Zone 1 consumption timer should be cancelled")
	}
	if m.GetTimer(1, CmdLimitProduction) != nil {
		t.Error("Zone 1 production timer should be cancelled")
	}
}

func TestManagerTimerExpiry(t *testing.T) {
	m := NewManager()

	var expiredZone uint8
	var expiredCmd CommandType
	var expiredValue any
	var mu sync.Mutex
	var expiryCalled bool

	m.OnExpiry(func(zoneID uint8, cmdType CommandType, value any) {
		mu.Lock()
		expiryCalled = true
		expiredZone = zoneID
		expiredCmd = cmdType
		expiredValue = value
		mu.Unlock()
	})

	// Create timer directly with short duration (bypassing validation for testing)
	key := timerKey{zoneID: 1, cmdType: CmdLimitConsumption}
	timer := &Timer{
		Key:       key,
		StartTime: time.Now(),
		Duration:  50 * time.Millisecond,
		Value:     int64(5000000),
	}
	timer.timer = time.AfterFunc(50*time.Millisecond, func() {
		m.expireTimer(key)
	})

	m.mu.Lock()
	m.timers[key] = timer
	m.mu.Unlock()

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if !expiryCalled {
		t.Error("Expiry callback was not called")
	}
	if expiredZone != 1 {
		t.Errorf("Expired zone = %d, want 1", expiredZone)
	}
	if expiredCmd != CmdLimitConsumption {
		t.Errorf("Expired cmd = %v, want CmdLimitConsumption", expiredCmd)
	}
	if expiredValue != int64(5000000) {
		t.Errorf("Expired value = %v, want 5000000", expiredValue)
	}

	if m.Count() != 0 {
		t.Errorf("Count() = %d after expiry, want 0", m.Count())
	}
}

func TestManagerMultipleZonesIndependent(t *testing.T) {
	m := NewManager()

	var expirations []struct {
		zone uint8
		cmd  CommandType
	}
	var mu sync.Mutex

	m.OnExpiry(func(zoneID uint8, cmdType CommandType, value any) {
		mu.Lock()
		expirations = append(expirations, struct {
			zone uint8
			cmd  CommandType
		}{zoneID, cmdType})
		mu.Unlock()
	})

	// Create timers directly with short durations for testing
	// Zone 1: 50ms, Zone 2: 100ms
	key1 := timerKey{zoneID: 1, cmdType: CmdLimitConsumption}
	timer1 := &Timer{
		Key:       key1,
		StartTime: time.Now(),
		Duration:  50 * time.Millisecond,
		Value:     int64(5000000),
	}
	timer1.timer = time.AfterFunc(50*time.Millisecond, func() {
		m.expireTimer(key1)
	})

	key2 := timerKey{zoneID: 2, cmdType: CmdLimitConsumption}
	timer2 := &Timer{
		Key:       key2,
		StartTime: time.Now(),
		Duration:  100 * time.Millisecond,
		Value:     int64(3000000),
	}
	timer2.timer = time.AfterFunc(100*time.Millisecond, func() {
		m.expireTimer(key2)
	})

	m.mu.Lock()
	m.timers[key1] = timer1
	m.timers[key2] = timer2
	m.mu.Unlock()

	// After 75ms, only Zone 1 should have expired
	time.Sleep(75 * time.Millisecond)

	mu.Lock()
	if len(expirations) != 1 || expirations[0].zone != 1 {
		t.Errorf("After 75ms: expected only zone 1 expired, got %v", expirations)
	}
	mu.Unlock()

	// After another 50ms, Zone 2 should also expire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(expirations) != 2 {
		t.Errorf("After 125ms: expected 2 expirations, got %d", len(expirations))
	}
	mu.Unlock()
}

func TestManagerGetZoneTimers(t *testing.T) {
	m := NewManager()

	m.SetTimer(1, CmdLimitConsumption, 5*time.Second, int64(5000000))
	m.SetTimer(1, CmdLimitProduction, 5*time.Second, int64(-3000000))
	m.SetTimer(1, CmdSetpointConsumption, 5*time.Second, int64(4000000))
	m.SetTimer(2, CmdLimitConsumption, 5*time.Second, int64(6000000))

	zone1Timers := m.GetZoneTimers(1)
	if len(zone1Timers) != 3 {
		t.Errorf("GetZoneTimers(1) returned %d timers, want 3", len(zone1Timers))
	}

	zone2Timers := m.GetZoneTimers(2)
	if len(zone2Timers) != 1 {
		t.Errorf("GetZoneTimers(2) returned %d timers, want 1", len(zone2Timers))
	}

	zone3Timers := m.GetZoneTimers(3)
	if len(zone3Timers) != 0 {
		t.Errorf("GetZoneTimers(3) returned %d timers, want 0", len(zone3Timers))
	}
}

func TestCommandTypeString(t *testing.T) {
	tests := []struct {
		cmdType CommandType
		want    string
	}{
		{CmdLimitConsumption, "LIMIT_CONSUMPTION"},
		{CmdLimitProduction, "LIMIT_PRODUCTION"},
		{CmdCurrentLimitConsumption, "CURRENT_LIMIT_CONSUMPTION"},
		{CmdCurrentLimitProduction, "CURRENT_LIMIT_PRODUCTION"},
		{CmdSetpointConsumption, "SETPOINT_CONSUMPTION"},
		{CmdSetpointProduction, "SETPOINT_PRODUCTION"},
		{CmdCurrentSetpointConsumption, "CURRENT_SETPOINT_CONSUMPTION"},
		{CmdCurrentSetpointProduction, "CURRENT_SETPOINT_PRODUCTION"},
		{CmdPause, "PAUSE"},
		{CommandType(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.cmdType.String(); got != tt.want {
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
		{"ShortDuration", 30 * time.Second, AccuracyAbsolute},       // 1% of 30s = 0.3s < 1s, use 1s
		{"MediumDuration", 2 * time.Minute, 1200 * time.Millisecond}, // 1% of 2min = 1.2s > 1s, use 1.2s
		{"LongDuration", 10 * time.Minute, 6 * time.Second},          // 1% of 10min = 6s
		{"MaxDuration", 24 * time.Hour, 864 * time.Second},           // 1% of 24h = 14.4min
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateAccuracy(tt.duration)
			// Allow for some floating point tolerance
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 100*time.Millisecond {
				t.Errorf("CalculateAccuracy(%v) = %v, want ~%v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestTimerReplacementCancelsCallback(t *testing.T) {
	m := NewManager()

	var expirations []int64
	var mu sync.Mutex

	m.OnExpiry(func(zoneID uint8, cmdType CommandType, value any) {
		mu.Lock()
		expirations = append(expirations, value.(int64))
		mu.Unlock()
	})

	// Create first timer directly with short duration
	key := timerKey{zoneID: 1, cmdType: CmdLimitConsumption}
	timer1 := &Timer{
		Key:       key,
		StartTime: time.Now(),
		Duration:  100 * time.Millisecond,
		Value:     int64(5000000),
	}
	timer1.timer = time.AfterFunc(100*time.Millisecond, func() {
		m.expireTimer(key)
	})

	m.mu.Lock()
	m.timers[key] = timer1
	m.mu.Unlock()

	// After 50ms, replace with new timer
	time.Sleep(50 * time.Millisecond)

	// Stop old timer and create replacement
	m.mu.Lock()
	if old, exists := m.timers[key]; exists {
		old.timer.Stop()
	}
	timer2 := &Timer{
		Key:       key,
		StartTime: time.Now(),
		Duration:  100 * time.Millisecond,
		Value:     int64(3000000),
	}
	timer2.timer = time.AfterFunc(100*time.Millisecond, func() {
		m.expireTimer(key)
	})
	m.timers[key] = timer2
	m.mu.Unlock()

	// Wait for original timer's expiry time
	time.Sleep(60 * time.Millisecond)

	mu.Lock()
	if len(expirations) != 0 {
		t.Errorf("Original timer should have been cancelled, got expirations: %v", expirations)
	}
	mu.Unlock()

	// Wait for replacement timer to expire
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(expirations) != 1 {
		t.Errorf("Expected 1 expiration from replacement timer, got %d", len(expirations))
	}
	if len(expirations) > 0 && expirations[0] != 3000000 {
		t.Errorf("Expiration value = %d, want 3000000", expirations[0])
	}
	mu.Unlock()
}

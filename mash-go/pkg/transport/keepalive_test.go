package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeepAliveConfig(t *testing.T) {
	config := DefaultKeepAliveConfig()

	if config.PingInterval != DefaultPingInterval {
		t.Errorf("PingInterval = %v, want %v", config.PingInterval, DefaultPingInterval)
	}
	if config.PongTimeout != DefaultPongTimeout {
		t.Errorf("PongTimeout = %v, want %v", config.PongTimeout, DefaultPongTimeout)
	}
	if config.MaxMissedPongs != DefaultMaxMissedPongs {
		t.Errorf("MaxMissedPongs = %d, want %d", config.MaxMissedPongs, DefaultMaxMissedPongs)
	}

	// Verify detection delay calculation
	delay := config.DetectionDelay()
	expected := 30*time.Second*3 + 5*time.Second // 95 seconds
	if delay != expected {
		t.Errorf("DetectionDelay = %v, want %v", delay, expected)
	}
}

func TestKeepAliveBasic(t *testing.T) {
	var pingCount atomic.Int32
	var lastSeq atomic.Uint32

	config := KeepAliveConfig{
		PingInterval:   50 * time.Millisecond,
		PongTimeout:    20 * time.Millisecond,
		MaxMissedPongs: 3,
	}

	ka := NewKeepAlive(config,
		func(seq uint32) error {
			pingCount.Add(1)
			lastSeq.Store(seq)
			return nil
		},
		func() {
			t.Log("Timeout called")
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ka.Start(ctx)

	// Wait for at least 2 pings
	time.Sleep(120 * time.Millisecond)

	// Respond to pings
	ka.PongReceived(lastSeq.Load())

	time.Sleep(60 * time.Millisecond)
	ka.PongReceived(lastSeq.Load())

	ka.Stop()

	if pingCount.Load() < 2 {
		t.Errorf("expected at least 2 pings, got %d", pingCount.Load())
	}
}

func TestKeepAliveTimeout(t *testing.T) {
	var timeoutCalled atomic.Bool

	config := KeepAliveConfig{
		PingInterval:   20 * time.Millisecond,
		PongTimeout:    10 * time.Millisecond,
		MaxMissedPongs: 2,
	}

	ka := NewKeepAlive(config,
		func(seq uint32) error {
			return nil
		},
		func() {
			timeoutCalled.Store(true)
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ka.Start(ctx)

	// Don't respond to pings - should timeout
	// Timeout should occur after: 2 * 20ms + 10ms = 50ms (approximately)
	time.Sleep(100 * time.Millisecond)

	if !timeoutCalled.Load() {
		t.Error("expected timeout to be called")
	}
}

func TestKeepAlivePongResetsCounter(t *testing.T) {
	var mu sync.Mutex
	var pongReceived bool

	config := KeepAliveConfig{
		PingInterval:   30 * time.Millisecond,
		PongTimeout:    10 * time.Millisecond,
		MaxMissedPongs: 3,
	}

	ka := NewKeepAlive(config,
		func(seq uint32) error {
			return nil
		},
		func() {
			t.Error("timeout should not be called")
		},
	)

	ka.SetPongReceivedCallback(func(seq uint32, latency time.Duration) {
		mu.Lock()
		pongReceived = true
		mu.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ka.Start(ctx)

	// Miss one ping
	time.Sleep(50 * time.Millisecond)

	// Now respond
	stats := ka.Stats()
	ka.PongReceived(stats.CurrentSeq)

	// Check that missed count was reset
	time.Sleep(20 * time.Millisecond)
	stats = ka.Stats()
	if stats.MissedPongs != 0 {
		t.Errorf("MissedPongs should be 0 after pong, got %d", stats.MissedPongs)
	}

	mu.Lock()
	if !pongReceived {
		t.Error("pong callback should have been called")
	}
	mu.Unlock()

	ka.Stop()
}

func TestKeepAliveStats(t *testing.T) {
	config := KeepAliveConfig{
		PingInterval:   50 * time.Millisecond,
		PongTimeout:    20 * time.Millisecond,
		MaxMissedPongs: 3,
	}

	ka := NewKeepAlive(config,
		func(seq uint32) error { return nil },
		func() {},
	)

	// Initial stats
	stats := ka.Stats()
	if stats.CurrentSeq != 0 {
		t.Errorf("initial CurrentSeq = %d, want 0", stats.CurrentSeq)
	}
	if stats.MissedPongs != 0 {
		t.Errorf("initial MissedPongs = %d, want 0", stats.MissedPongs)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ka.Start(ctx)
	time.Sleep(60 * time.Millisecond)

	stats = ka.Stats()
	if stats.CurrentSeq == 0 {
		t.Error("CurrentSeq should be > 0 after ping")
	}
	if stats.LastPingTime.IsZero() {
		t.Error("LastPingTime should be set")
	}

	ka.Stop()
}

func TestKeepAliveStartStop(t *testing.T) {
	ka := NewKeepAlive(DefaultKeepAliveConfig(),
		func(seq uint32) error { return nil },
		func() {},
	)

	if ka.IsRunning() {
		t.Error("should not be running initially")
	}

	ctx := context.Background()
	ka.Start(ctx)

	if !ka.IsRunning() {
		t.Error("should be running after Start")
	}

	// Start again should be no-op
	ka.Start(ctx)
	if !ka.IsRunning() {
		t.Error("should still be running")
	}

	ka.Stop()

	// Give it a moment to stop
	time.Sleep(10 * time.Millisecond)

	if ka.IsRunning() {
		t.Error("should not be running after Stop")
	}

	// Stop again should be no-op
	ka.Stop()
}

func TestKeepAliveContextCancel(t *testing.T) {
	var pingCount atomic.Int32

	config := KeepAliveConfig{
		PingInterval:   20 * time.Millisecond,
		PongTimeout:    10 * time.Millisecond,
		MaxMissedPongs: 3,
	}

	ka := NewKeepAlive(config,
		func(seq uint32) error {
			pingCount.Add(1)
			return nil
		},
		func() {},
	)

	ctx, cancel := context.WithCancel(context.Background())
	ka.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	countBefore := pingCount.Load()

	// Cancel context
	cancel()
	time.Sleep(50 * time.Millisecond)

	countAfter := pingCount.Load()

	// Should not have sent more pings after cancel
	if countAfter > countBefore+1 {
		t.Errorf("pings continued after cancel: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestCalculateDetectionDelay(t *testing.T) {
	tests := []struct {
		pingInterval   time.Duration
		pongTimeout    time.Duration
		maxMissedPongs int
		expected       time.Duration
	}{
		{30 * time.Second, 5 * time.Second, 3, 95 * time.Second},
		{10 * time.Second, 2 * time.Second, 5, 52 * time.Second},
		{1 * time.Second, 1 * time.Second, 1, 2 * time.Second},
	}

	for _, tt := range tests {
		got := CalculateDetectionDelay(tt.pingInterval, tt.pongTimeout, tt.maxMissedPongs)
		if got != tt.expected {
			t.Errorf("CalculateDetectionDelay(%v, %v, %d) = %v, want %v",
				tt.pingInterval, tt.pongTimeout, tt.maxMissedPongs, got, tt.expected)
		}
	}
}

func TestKeepAliveLatencyCallback(t *testing.T) {
	var receivedLatency time.Duration
	var mu sync.Mutex
	done := make(chan struct{})

	config := KeepAliveConfig{
		PingInterval:   50 * time.Millisecond,
		PongTimeout:    100 * time.Millisecond,
		MaxMissedPongs: 3,
	}

	var lastSeq atomic.Uint32

	ka := NewKeepAlive(config,
		func(seq uint32) error {
			lastSeq.Store(seq)
			return nil
		},
		func() {},
	)

	ka.SetPongReceivedCallback(func(seq uint32, latency time.Duration) {
		mu.Lock()
		receivedLatency = latency
		mu.Unlock()
		close(done)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ka.Start(ctx)

	// Wait for ping to be sent
	time.Sleep(10 * time.Millisecond)

	// Simulate some latency
	time.Sleep(25 * time.Millisecond)

	// Send pong
	ka.PongReceived(lastSeq.Load())

	// Wait for callback
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("latency callback not called")
	}

	mu.Lock()
	if receivedLatency < 20*time.Millisecond {
		t.Errorf("latency too low: %v", receivedLatency)
	}
	mu.Unlock()

	ka.Stop()
}

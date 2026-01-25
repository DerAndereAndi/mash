package transport

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Keep-alive constants.
const (
	// DefaultPingInterval is the default interval between pings.
	DefaultPingInterval = 30 * time.Second

	// DefaultPongTimeout is the default timeout waiting for a pong response.
	DefaultPongTimeout = 5 * time.Second

	// DefaultMaxMissedPongs is the default number of missed pongs before disconnect.
	DefaultMaxMissedPongs = 3

	// MaxDetectionDelay is the maximum time to detect connection loss.
	// Calculated as: PingInterval * MaxMissedPongs + PongTimeout
	// Default: 30 * 3 + 5 = 95 seconds
	MaxDetectionDelay = 95 * time.Second
)

// KeepAliveConfig configures keep-alive behavior.
type KeepAliveConfig struct {
	// PingInterval is the interval between pings.
	PingInterval time.Duration

	// PongTimeout is the timeout waiting for a pong response.
	PongTimeout time.Duration

	// MaxMissedPongs is the number of missed pongs before disconnect.
	MaxMissedPongs int
}

// DefaultKeepAliveConfig returns the default keep-alive configuration.
func DefaultKeepAliveConfig() KeepAliveConfig {
	return KeepAliveConfig{
		PingInterval:   DefaultPingInterval,
		PongTimeout:    DefaultPongTimeout,
		MaxMissedPongs: DefaultMaxMissedPongs,
	}
}

// DetectionDelay calculates the maximum detection delay for this configuration.
func (c KeepAliveConfig) DetectionDelay() time.Duration {
	return c.PingInterval*time.Duration(c.MaxMissedPongs) + c.PongTimeout
}

// KeepAlive manages connection liveness monitoring.
type KeepAlive struct {
	config KeepAliveConfig

	// Callbacks
	sendPing       func(seq uint32) error
	onTimeout      func()
	onPongReceived func(seq uint32, latency time.Duration)

	// State
	sequence     atomic.Uint32
	missedPongs  int
	lastPingTime time.Time
	lastPongTime time.Time
	pendingPing  uint32
	hasPending   bool

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	pongCh  chan uint32
}

// NewKeepAlive creates a new keep-alive manager.
func NewKeepAlive(config KeepAliveConfig, sendPing func(seq uint32) error, onTimeout func()) *KeepAlive {
	if config.PingInterval == 0 {
		config.PingInterval = DefaultPingInterval
	}
	if config.PongTimeout == 0 {
		config.PongTimeout = DefaultPongTimeout
	}
	if config.MaxMissedPongs == 0 {
		config.MaxMissedPongs = DefaultMaxMissedPongs
	}

	return &KeepAlive{
		config:    config,
		sendPing:  sendPing,
		onTimeout: onTimeout,
		stopCh:    make(chan struct{}),
		pongCh:    make(chan uint32, 1),
	}
}

// SetPongReceivedCallback sets a callback for when pongs are received.
func (ka *KeepAlive) SetPongReceivedCallback(cb func(seq uint32, latency time.Duration)) {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	ka.onPongReceived = cb
}

// Start begins the keep-alive monitoring loop.
func (ka *KeepAlive) Start(ctx context.Context) {
	ka.mu.Lock()
	if ka.running {
		ka.mu.Unlock()
		return
	}
	ka.running = true
	ka.stopCh = make(chan struct{})
	ka.mu.Unlock()

	go ka.loop(ctx)
}

// Stop stops the keep-alive monitoring.
func (ka *KeepAlive) Stop() {
	ka.mu.Lock()
	defer ka.mu.Unlock()

	if !ka.running {
		return
	}

	ka.running = false
	close(ka.stopCh)
}

// PongReceived should be called when a pong message is received.
func (ka *KeepAlive) PongReceived(seq uint32) {
	select {
	case ka.pongCh <- seq:
	default:
		// Channel full, drop (shouldn't happen in practice)
	}
}

// IsRunning returns true if keep-alive monitoring is active.
func (ka *KeepAlive) IsRunning() bool {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	return ka.running
}

// Stats returns current keep-alive statistics.
func (ka *KeepAlive) Stats() KeepAliveStats {
	ka.mu.Lock()
	defer ka.mu.Unlock()
	return KeepAliveStats{
		LastPingTime: ka.lastPingTime,
		LastPongTime: ka.lastPongTime,
		MissedPongs:  ka.missedPongs,
		CurrentSeq:   ka.sequence.Load(),
	}
}

// KeepAliveStats contains keep-alive statistics.
type KeepAliveStats struct {
	LastPingTime time.Time
	LastPongTime time.Time
	MissedPongs  int
	CurrentSeq   uint32
}

// loop is the main keep-alive monitoring loop.
func (ka *KeepAlive) loop(ctx context.Context) {
	ticker := time.NewTicker(ka.config.PingInterval)
	defer ticker.Stop()

	// Send initial ping
	ka.sendPingMessage()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ka.stopCh:
			return
		case <-ticker.C:
			ka.handleTick()
		case seq := <-ka.pongCh:
			ka.handlePong(seq)
		}
	}
}

// sendPingMessage sends a ping and records the time.
func (ka *KeepAlive) sendPingMessage() {
	seq := ka.sequence.Add(1)

	ka.mu.Lock()
	ka.lastPingTime = time.Now()
	ka.pendingPing = seq
	ka.hasPending = true
	ka.mu.Unlock()

	if err := ka.sendPing(seq); err != nil {
		// Send failed - connection is likely dead
		ka.mu.Lock()
		ka.hasPending = false
		ka.mu.Unlock()
		// Let the pong timeout handle it
	}
}

// handleTick handles the ping interval tick.
func (ka *KeepAlive) handleTick() {
	ka.mu.Lock()

	// Check if we have a pending ping that timed out
	if ka.hasPending {
		elapsed := time.Since(ka.lastPingTime)
		if elapsed >= ka.config.PongTimeout {
			// Pong not received in time
			ka.missedPongs++
			ka.hasPending = false

			if ka.missedPongs >= ka.config.MaxMissedPongs {
				// Connection considered dead
				ka.mu.Unlock()
				if ka.onTimeout != nil {
					ka.onTimeout()
				}
				return
			}
		}
	}

	ka.mu.Unlock()

	// Send next ping
	ka.sendPingMessage()
}

// handlePong handles a received pong.
func (ka *KeepAlive) handlePong(seq uint32) {
	ka.mu.Lock()
	defer ka.mu.Unlock()

	now := time.Now()
	ka.lastPongTime = now

	// Check if this pong matches our pending ping
	if ka.hasPending && seq == ka.pendingPing {
		latency := now.Sub(ka.lastPingTime)
		ka.hasPending = false
		ka.missedPongs = 0 // Reset on successful pong

		// Notify callback if set
		if ka.onPongReceived != nil {
			go ka.onPongReceived(seq, latency)
		}
	}
	// Ignore pongs with wrong sequence (could be delayed from previous ping)
}

// CalculateDetectionDelay calculates the maximum detection delay for given parameters.
func CalculateDetectionDelay(pingInterval, pongTimeout time.Duration, maxMissedPongs int) time.Duration {
	return pingInterval*time.Duration(maxMissedPongs) + pongTimeout
}

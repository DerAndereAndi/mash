package connection

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Connection errors.
var (
	ErrConnectionClosed  = errors.New("connection closed")
	ErrReconnectDisabled = errors.New("reconnection disabled")
	ErrConnectTimeout    = errors.New("connection timeout")
	ErrAlreadyConnected  = errors.New("already connected")
	ErrNotConnected      = errors.New("not connected")
)

// State represents the connection state.
type State uint8

const (
	// StateDisconnected indicates no active connection.
	StateDisconnected State = iota

	// StateConnecting indicates a connection attempt is in progress.
	StateConnecting

	// StateConnected indicates an active connection.
	StateConnected

	// StateReconnecting indicates automatic reconnection is in progress.
	StateReconnecting

	// StateClosed indicates the connection manager has been closed.
	StateClosed
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "DISCONNECTED"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateReconnecting:
		return "RECONNECTING"
	case StateClosed:
		return "CLOSED"
	default:
		return "UNKNOWN"
	}
}

// ConnectFunc is called to establish a connection.
// It should return nil on success or an error on failure.
type ConnectFunc func(ctx context.Context) error

// Manager manages connection lifecycle with automatic reconnection.
type Manager struct {
	mu sync.RWMutex

	// Current state
	state State

	// Backoff calculator
	backoff *Backoff

	// Connection function
	connectFn ConnectFunc

	// Auto-reconnect enabled
	autoReconnect bool

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for reconnection goroutine
	wg sync.WaitGroup

	// Channel to signal reconnection should start
	reconnectCh chan struct{}

	// Callbacks
	onStateChange func(oldState, newState State)
	onConnected   func()
	onDisconnected func()
	onReconnecting func(attempt int, delay time.Duration)
}

// NewManager creates a new connection manager.
func NewManager(connectFn ConnectFunc) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		state:         StateDisconnected,
		backoff:       NewBackoff(),
		connectFn:     connectFn,
		autoReconnect: true,
		ctx:           ctx,
		cancel:        cancel,
		reconnectCh:   make(chan struct{}, 1),
	}
}

// State returns the current connection state.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// IsConnected returns true if currently connected.
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateConnected
}

// SetAutoReconnect enables or disables automatic reconnection.
func (m *Manager) SetAutoReconnect(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoReconnect = enabled
}

// Connect initiates a connection.
// Returns immediately if already connected.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.state == StateConnected {
		m.mu.Unlock()
		return ErrAlreadyConnected
	}
	if m.state == StateClosed {
		m.mu.Unlock()
		return ErrConnectionClosed
	}

	oldState := m.state
	m.state = StateConnecting
	m.mu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(oldState, StateConnecting)
	}

	// Attempt connection
	err := m.connectFn(ctx)

	m.mu.Lock()
	if err != nil {
		m.state = StateDisconnected
		m.mu.Unlock()
		if m.onStateChange != nil {
			m.onStateChange(StateConnecting, StateDisconnected)
		}
		return err
	}

	m.state = StateConnected
	m.backoff.Reset()
	m.mu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(StateConnecting, StateConnected)
	}
	if m.onConnected != nil {
		m.onConnected()
	}

	return nil
}

// Disconnect closes the connection.
// If autoReconnect is enabled, reconnection will be attempted.
func (m *Manager) Disconnect() {
	m.mu.Lock()
	if m.state != StateConnected {
		m.mu.Unlock()
		return
	}

	oldState := m.state
	autoReconnect := m.autoReconnect

	if autoReconnect {
		m.state = StateReconnecting
	} else {
		m.state = StateDisconnected
	}
	m.mu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(oldState, m.state)
	}
	if m.onDisconnected != nil {
		m.onDisconnected()
	}

	if autoReconnect {
		m.triggerReconnect()
	}
}

// NotifyConnectionLost should be called when a connection loss is detected.
// This triggers automatic reconnection if enabled.
func (m *Manager) NotifyConnectionLost() {
	m.mu.Lock()
	if m.state != StateConnected {
		m.mu.Unlock()
		return
	}

	oldState := m.state
	autoReconnect := m.autoReconnect

	if autoReconnect {
		m.state = StateReconnecting
	} else {
		m.state = StateDisconnected
	}
	m.mu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(oldState, m.state)
	}
	if m.onDisconnected != nil {
		m.onDisconnected()
	}

	if autoReconnect {
		m.triggerReconnect()
	}
}

// StartReconnectLoop starts the background reconnection loop.
// Must be called once before reconnection will work.
func (m *Manager) StartReconnectLoop() {
	m.wg.Add(1)
	go m.reconnectLoop()
}

// Close shuts down the connection manager.
func (m *Manager) Close() {
	m.mu.Lock()
	if m.state == StateClosed {
		m.mu.Unlock()
		return
	}

	oldState := m.state
	m.state = StateClosed
	m.mu.Unlock()

	if m.onStateChange != nil {
		m.onStateChange(oldState, StateClosed)
	}

	m.cancel()
	m.wg.Wait()
}

// triggerReconnect signals that reconnection should be attempted.
func (m *Manager) triggerReconnect() {
	select {
	case m.reconnectCh <- struct{}{}:
	default:
		// Already pending
	}
}

// reconnectLoop runs in a goroutine and handles reconnection attempts.
func (m *Manager) reconnectLoop() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.reconnectCh:
			m.attemptReconnect()
		}
	}
}

// attemptReconnect performs reconnection with backoff.
func (m *Manager) attemptReconnect() {
	for {
		m.mu.RLock()
		state := m.state
		m.mu.RUnlock()

		if state == StateClosed {
			return
		}
		if state == StateConnected {
			return
		}

		// Get next backoff delay
		delay := m.backoff.Next()
		attempts := m.backoff.Attempts()

		if m.onReconnecting != nil {
			m.onReconnecting(attempts, delay)
		}

		// Wait for backoff delay
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(delay):
		}

		// Attempt connection
		m.mu.Lock()
		if m.state == StateClosed || m.state == StateConnected {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
		err := m.connectFn(ctx)
		cancel()

		if err == nil {
			// Success!
			m.mu.Lock()
			oldState := m.state
			m.state = StateConnected
			m.backoff.Reset()
			m.mu.Unlock()

			if m.onStateChange != nil {
				m.onStateChange(oldState, StateConnected)
			}
			if m.onConnected != nil {
				m.onConnected()
			}
			return
		}

		// Failed - continue looping with next backoff
	}
}

// OnStateChange sets a callback for state changes.
func (m *Manager) OnStateChange(fn func(oldState, newState State)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStateChange = fn
}

// OnConnected sets a callback for successful connection.
func (m *Manager) OnConnected(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onConnected = fn
}

// OnDisconnected sets a callback for disconnection.
func (m *Manager) OnDisconnected(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onDisconnected = fn
}

// OnReconnecting sets a callback for reconnection attempts.
func (m *Manager) OnReconnecting(fn func(attempt int, delay time.Duration)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReconnecting = fn
}

// BackoffAttempts returns the current number of reconnection attempts.
func (m *Manager) BackoffAttempts() int {
	return m.backoff.Attempts()
}

package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Connection states.
type ConnectionState int

const (
	// StateDisconnected indicates no connection.
	StateDisconnected ConnectionState = iota

	// StateConnecting indicates connection in progress.
	StateConnecting

	// StateConnected indicates an active connection.
	StateConnected

	// StateClosing indicates graceful close in progress.
	StateClosing
)

// String returns the connection state name.
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "DISCONNECTED"
	case StateConnecting:
		return "CONNECTING"
	case StateConnected:
		return "CONNECTED"
	case StateClosing:
		return "CLOSING"
	default:
		return "UNKNOWN"
	}
}

// Connection errors.
var (
	ErrNotConnected     = errors.New("not connected")
	ErrAlreadyConnected = errors.New("already connected")
	ErrConnectionClosed = errors.New("connection closed")
	ErrCloseTimeout     = errors.New("close timeout")
)

// ConnectionConfig configures a MASH connection.
type ConnectionConfig struct {
	// TLS configuration
	TLSConfig *tls.Config

	// MaxMessageSize is the maximum message size (default: 64KB)
	MaxMessageSize uint32

	// KeepAlive configuration
	KeepAlive KeepAliveConfig

	// CloseTimeout is the timeout for graceful close (default: 5s)
	CloseTimeout time.Duration

	// ReadTimeout is the timeout for read operations (0 = no timeout)
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for write operations (0 = no timeout)
	WriteTimeout time.Duration
}

// DefaultConnectionConfig returns the default connection configuration.
func DefaultConnectionConfig() ConnectionConfig {
	return ConnectionConfig{
		MaxMessageSize: DefaultMaxMessageSize,
		KeepAlive:      DefaultKeepAliveConfig(),
		CloseTimeout:   5 * time.Second,
	}
}

// ConnectionHandler handles connection events.
type ConnectionHandler interface {
	// OnMessage is called when a message is received.
	OnMessage(msg []byte)

	// OnStateChange is called when the connection state changes.
	OnStateChange(oldState, newState ConnectionState)

	// OnError is called when an error occurs.
	OnError(err error)
}

// Connection represents a MASH connection.
type Connection struct {
	config  ConnectionConfig
	handler ConnectionHandler

	// Network connection
	conn    net.Conn
	tlsConn *tls.Conn
	framer  *Framer

	// Keep-alive
	keepAlive *KeepAlive

	// State
	state     atomic.Int32
	closeOnce sync.Once
	closeCh   chan struct{}
	closeDone chan struct{}

	// Message handling
	messageSeq atomic.Uint32

	// Synchronization
	mu      sync.RWMutex
	writeMu sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewConnection creates a new connection (not yet connected).
func NewConnection(config ConnectionConfig, handler ConnectionHandler) *Connection {
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = DefaultMaxMessageSize
	}
	if config.CloseTimeout == 0 {
		config.CloseTimeout = 5 * time.Second
	}

	c := &Connection{
		config:    config,
		handler:   handler,
		closeCh:   make(chan struct{}),
		closeDone: make(chan struct{}),
	}
	c.state.Store(int32(StateDisconnected))

	return c
}

// State returns the current connection state.
func (c *Connection) State() ConnectionState {
	return ConnectionState(c.state.Load())
}

// Connect establishes a connection to the specified address.
func (c *Connection) Connect(ctx context.Context, address string) error {
	if !c.state.CompareAndSwap(int32(StateDisconnected), int32(StateConnecting)) {
		return ErrAlreadyConnected
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.closeCh = make(chan struct{})
	c.closeDone = make(chan struct{})

	// Notify state change
	c.notifyStateChange(StateDisconnected, StateConnecting)

	// Dial with timeout from context
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateConnecting, StateDisconnected)
		return fmt.Errorf("dial failed: %w", err)
	}

	// TLS handshake
	tlsConn := tls.Client(conn, c.config.TLSConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateConnecting, StateDisconnected)
		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Verify connection
	if err := VerifyConnection(tlsConn.ConnectionState()); err != nil {
		tlsConn.Close()
		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateConnecting, StateDisconnected)
		return fmt.Errorf("connection verification failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.tlsConn = tlsConn
	c.framer = NewFramerWithMaxSize(tlsConn, c.config.MaxMessageSize)
	c.mu.Unlock()

	// Start keep-alive
	c.startKeepAlive()

	// Start read loop
	go c.readLoop()

	c.state.Store(int32(StateConnected))
	c.notifyStateChange(StateConnecting, StateConnected)

	return nil
}

// Accept accepts an incoming connection on a listener.
func (c *Connection) Accept(ctx context.Context, conn net.Conn) error {
	if !c.state.CompareAndSwap(int32(StateDisconnected), int32(StateConnecting)) {
		return ErrAlreadyConnected
	}

	c.ctx, c.cancel = context.WithCancel(ctx)
	c.closeCh = make(chan struct{})
	c.closeDone = make(chan struct{})

	c.notifyStateChange(StateDisconnected, StateConnecting)

	// TLS handshake (server side)
	tlsConn := tls.Server(conn, c.config.TLSConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateConnecting, StateDisconnected)
		return fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Verify connection
	if err := VerifyConnection(tlsConn.ConnectionState()); err != nil {
		tlsConn.Close()
		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateConnecting, StateDisconnected)
		return fmt.Errorf("connection verification failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.tlsConn = tlsConn
	c.framer = NewFramerWithMaxSize(tlsConn, c.config.MaxMessageSize)
	c.mu.Unlock()

	// Start keep-alive
	c.startKeepAlive()

	// Start read loop
	go c.readLoop()

	c.state.Store(int32(StateConnected))
	c.notifyStateChange(StateConnecting, StateConnected)

	return nil
}

// Send sends a message over the connection.
func (c *Connection) Send(data []byte) error {
	if c.State() != StateConnected {
		return ErrNotConnected
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.mu.RLock()
	framer := c.framer
	tlsConn := c.tlsConn
	c.mu.RUnlock()

	if framer == nil {
		return ErrNotConnected
	}

	// Set write deadline if configured
	if c.config.WriteTimeout > 0 {
		tlsConn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
		defer tlsConn.SetWriteDeadline(time.Time{})
	}

	return framer.WriteFrame(data)
}

// SendControlMessage sends a control message (ping/pong/close).
func (c *Connection) SendControlMessage(msgType wire.ControlMessageType, seq uint32) error {
	msg := wire.ControlMessage{
		Type:     msgType,
		Sequence: seq,
	}

	data, err := wire.EncodeControlMessage(&msg)
	if err != nil {
		return fmt.Errorf("failed to encode control message: %w", err)
	}

	return c.Send(data)
}

// Close gracefully closes the connection.
func (c *Connection) Close() error {
	return c.CloseWithTimeout(c.config.CloseTimeout)
}

// CloseWithTimeout gracefully closes with a specific timeout.
func (c *Connection) CloseWithTimeout(timeout time.Duration) error {
	var closeErr error

	c.closeOnce.Do(func() {
		// Only proceed if connected or connecting
		currentState := c.State()
		if currentState == StateDisconnected {
			return
		}

		c.state.Store(int32(StateClosing))
		c.notifyStateChange(currentState, StateClosing)

		// Send close message
		c.SendControlMessage(wire.ControlClose, 0)

		// Wait for close acknowledgment or timeout
		select {
		case <-c.closeDone:
			// Clean close
		case <-time.After(timeout):
			closeErr = ErrCloseTimeout
		}

		// Stop keep-alive
		if c.keepAlive != nil {
			c.keepAlive.Stop()
		}

		// Cancel context
		if c.cancel != nil {
			c.cancel()
		}

		// Close connection
		c.mu.Lock()
		if c.tlsConn != nil {
			c.tlsConn.Close()
			c.tlsConn = nil
		}
		c.conn = nil
		c.framer = nil
		c.mu.Unlock()

		c.state.Store(int32(StateDisconnected))
		c.notifyStateChange(StateClosing, StateDisconnected)
	})

	return closeErr
}

// ForceClose immediately closes the connection without graceful handshake.
func (c *Connection) ForceClose() {
	c.closeOnce.Do(func() {
		currentState := c.State()

		// Stop keep-alive
		if c.keepAlive != nil {
			c.keepAlive.Stop()
		}

		// Cancel context
		if c.cancel != nil {
			c.cancel()
		}

		// Close connection
		c.mu.Lock()
		if c.tlsConn != nil {
			c.tlsConn.Close()
			c.tlsConn = nil
		}
		c.conn = nil
		c.framer = nil
		c.mu.Unlock()

		c.state.Store(int32(StateDisconnected))
		if currentState != StateDisconnected {
			c.notifyStateChange(currentState, StateDisconnected)
		}
	})
}

// LocalAddr returns the local network address.
func (c *Connection) LocalAddr() net.Addr {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn != nil {
		return c.conn.LocalAddr()
	}
	return nil
}

// RemoteAddr returns the remote network address.
func (c *Connection) RemoteAddr() net.Addr {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}
	return nil
}

// TLSConnectionState returns the TLS connection state.
func (c *Connection) TLSConnectionState() (tls.ConnectionState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.tlsConn != nil {
		return c.tlsConn.ConnectionState(), true
	}
	return tls.ConnectionState{}, false
}

// startKeepAlive initializes and starts keep-alive monitoring.
func (c *Connection) startKeepAlive() {
	c.keepAlive = NewKeepAlive(
		c.config.KeepAlive,
		func(seq uint32) error {
			return c.SendControlMessage(wire.ControlPing, seq)
		},
		func() {
			// Keep-alive timeout - connection dead
			c.handler.OnError(fmt.Errorf("keep-alive timeout"))
			c.ForceClose()
		},
	)
	c.keepAlive.Start(c.ctx)
}

// readLoop reads messages from the connection.
func (c *Connection) readLoop() {
	defer func() {
		close(c.closeDone)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.closeCh:
			return
		default:
		}

		c.mu.RLock()
		framer := c.framer
		tlsConn := c.tlsConn
		c.mu.RUnlock()

		if framer == nil {
			return
		}

		// Set read deadline if configured
		if c.config.ReadTimeout > 0 {
			tlsConn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
		}

		data, err := framer.ReadFrame()
		if err != nil {
			if c.State() == StateClosing || c.ctx.Err() != nil {
				return // Expected during close
			}
			c.handler.OnError(fmt.Errorf("read error: %w", err))
			c.ForceClose()
			return
		}

		// Try to decode as control message
		if ctrlMsg, err := wire.DecodeControlMessage(data); err == nil {
			c.handleControlMessage(ctrlMsg)
			continue
		}

		// Regular message - pass to handler
		c.handler.OnMessage(data)
	}
}

// handleControlMessage processes control messages.
func (c *Connection) handleControlMessage(msg *wire.ControlMessage) {
	switch msg.Type {
	case wire.ControlPing:
		// Respond with pong
		c.SendControlMessage(wire.ControlPong, msg.Sequence)

	case wire.ControlPong:
		// Notify keep-alive
		if c.keepAlive != nil {
			c.keepAlive.PongReceived(msg.Sequence)
		}

	case wire.ControlClose:
		// Peer initiated close
		c.SendControlMessage(wire.ControlClose, 0) // Acknowledge
		c.ForceClose()
	}
}

// notifyStateChange notifies the handler of state changes.
func (c *Connection) notifyStateChange(oldState, newState ConnectionState) {
	if c.handler != nil {
		c.handler.OnStateChange(oldState, newState)
	}
}

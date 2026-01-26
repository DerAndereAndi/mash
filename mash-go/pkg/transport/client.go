package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"
)

// ClientConfig configures a MASH client.
type ClientConfig struct {
	// TLSConfig contains TLS settings.
	TLSConfig *TLSConfig

	// CommissioningMode enables commissioning mode where server cert
	// verification is skipped (security comes from SPAKE2+).
	CommissioningMode bool

	// MaxMessageSize is the maximum message size (default: 64KB).
	MaxMessageSize uint32

	// ConnectTimeout is the connection timeout (default: 30s).
	ConnectTimeout time.Duration

	// KeepAlive configuration.
	KeepAlive KeepAliveConfig
}

// Client is a MASH TLS client that connects to devices.
type Client struct {
	config  ClientConfig
	tlsConf *tls.Config
}

// NewClient creates a new MASH client.
func NewClient(config ClientConfig) (*Client, error) {
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = DefaultMaxMessageSize
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 30 * time.Second
	}

	var tlsConf *tls.Config
	var err error

	if config.CommissioningMode {
		// Commissioning mode - use minimal TLS config
		tlsConf = NewCommissioningTLSConfig()
	} else if config.TLSConfig != nil {
		// Normal mode - use provided config
		tlsConf, err = NewClientTLSConfig(config.TLSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
	} else {
		return nil, fmt.Errorf("TLSConfig is required when not in commissioning mode")
	}

	return &Client{
		config:  config,
		tlsConf: tlsConf,
	}, nil
}

// Connect establishes a connection to the specified address.
func (c *Client) Connect(ctx context.Context, address string) (*ClientConn, error) {
	// Apply timeout from config if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.ConnectTimeout)
		defer cancel()
	}

	// Dial TCP connection
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	// TLS handshake
	tlsConn := tls.Client(conn, c.tlsConf)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Verify TLS version and ALPN
	state := tlsConn.ConnectionState()
	if err := VerifyConnection(state); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("connection verification failed: %w", err)
	}

	// Create client connection wrapper
	clientConn := &ClientConn{
		conn:     tlsConn,
		framer:   NewFramerWithMaxSize(tlsConn, c.config.MaxMessageSize),
		tlsState: state,
		client:   c,
		closeCh:  make(chan struct{}),
	}

	return clientConn, nil
}

// ClientConn represents a connection from client to server.
type ClientConn struct {
	conn     *tls.Conn
	framer   *Framer
	tlsState tls.ConnectionState
	client   *Client
	closeCh  chan struct{}

	closeOnce sync.Once
	writeMu   sync.Mutex
	readMu    sync.Mutex
}

// TLSState returns the TLS connection state.
func (c *ClientConn) TLSState() tls.ConnectionState {
	return c.tlsState
}

// LocalAddr returns the local network address.
func (c *ClientConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *ClientConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Send sends a message to the server.
func (c *ClientConn) Send(data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	select {
	case <-c.closeCh:
		return ErrConnectionClosed
	default:
	}

	return c.framer.WriteFrame(data)
}

// Receive receives a message from the server with timeout.
func (c *ClientConn) Receive(timeout time.Duration) ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()

	select {
	case <-c.closeCh:
		return nil, ErrConnectionClosed
	default:
	}

	if timeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(timeout))
		defer c.conn.SetReadDeadline(time.Time{})
	}

	return c.framer.ReadFrame()
}

// Close closes the connection.
func (c *ClientConn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closeCh)
		err = c.conn.Close()
	})
	return err
}

// SendPing sends a ping control message.
func (c *ClientConn) SendPing(seq uint32) error {
	msg, err := EncodePing(seq)
	if err != nil {
		return err
	}
	return c.Send(msg)
}

// SendClose sends a close control message.
func (c *ClientConn) SendClose() error {
	msg, err := EncodeClose()
	if err != nil {
		return err
	}
	return c.Send(msg)
}

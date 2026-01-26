package service

import (
	"io"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/transport"
)

// framedConnection wraps an io.ReadWriteCloser with length-prefixed framing.
// It implements the Sendable interface for use with sessions.
type framedConnection struct {
	mu     sync.Mutex
	conn   io.ReadWriteCloser
	framer *transport.Framer
	closed bool
}

// newFramedConnection creates a new framed connection wrapper.
func newFramedConnection(conn io.ReadWriteCloser) *framedConnection {
	return &framedConnection{
		conn:   conn,
		framer: transport.NewFramer(conn),
	}
}

// Send sends data with length-prefixed framing.
// Implements the Sendable interface.
func (c *framedConnection) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrSessionClosed
	}

	return c.framer.WriteFrame(data)
}

// ReadFrame reads a length-prefixed frame from the connection.
func (c *framedConnection) ReadFrame() ([]byte, error) {
	return c.framer.ReadFrame()
}

// Close closes the underlying connection.
func (c *framedConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	return c.conn.Close()
}

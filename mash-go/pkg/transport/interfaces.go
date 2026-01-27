package transport

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// ServerConnection represents a server-side connection to a client.
// Implemented by ServerConn.
type ServerConnection interface {
	// RemoteAddr returns the remote network address of the client.
	RemoteAddr() net.Addr

	// TLSState returns the TLS connection state.
	TLSState() tls.ConnectionState

	// Send sends a message to the client.
	Send(data []byte) error

	// Close closes the connection.
	Close() error
}

// ClientConnection represents a client-side connection to a server.
// Implemented by ClientConn.
type ClientConnection interface {
	// TLSState returns the TLS connection state.
	TLSState() tls.ConnectionState

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr

	// Send sends a message to the server.
	Send(data []byte) error

	// Receive receives a message with the specified timeout.
	Receive(timeout time.Duration) ([]byte, error)

	// Close closes the connection.
	Close() error

	// SendPing sends a ping control message with the given sequence number.
	SendPing(seq uint32) error

	// SendClose sends a close control message.
	SendClose() error
}

// TransportServer represents a MASH TLS server.
// Implemented by Server.
type TransportServer interface {
	// Start begins accepting connections.
	Start(ctx context.Context) error

	// Stop gracefully stops the server.
	Stop() error

	// Addr returns the server's listen address.
	Addr() net.Addr

	// ConnectionCount returns the number of active connections.
	ConnectionCount() int
}

// FrameReadWriter provides length-prefixed frame I/O.
// Implemented by Framer.
type FrameReadWriter interface {
	// ReadFrame reads a length-prefixed frame.
	ReadFrame() ([]byte, error)

	// WriteFrame writes a length-prefixed frame.
	WriteFrame(data []byte) error
}

// Compile-time interface satisfaction checks.
var (
	_ ServerConnection = (*ServerConn)(nil)
	_ ClientConnection = (*ClientConn)(nil)
	_ TransportServer  = (*Server)(nil)
	_ FrameReadWriter  = (*Framer)(nil)
)

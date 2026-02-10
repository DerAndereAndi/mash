package runner

import (
	"net"
	"testing"
)

// fakeConn is a minimal net.Conn for testing socket close behavior.
type fakeConn struct {
	net.Conn
	closed bool
}

func (f *fakeConn) Close() error {
	f.closed = true
	return nil
}

func TestTransitionToDisconnected_ClosesSocket(t *testing.T) {
	fc := &fakeConn{}
	c := &Connection{
		conn:  fc,
		state: ConnTLSConnected,
	}
	c.transitionTo(ConnDisconnected)
	if !fc.closed {
		t.Fatal("expected socket to be closed")
	}
	// transitionTo closes but does NOT nil pointers -- goroutines may still
	// hold references to framer and need to get IO errors, not nil panics.
	if c.conn == nil {
		t.Fatal("transitionTo should not nil conn (close-but-not-nil)")
	}
	if c.state != ConnDisconnected {
		t.Fatalf("expected ConnDisconnected, got %d", c.state)
	}
}

func TestClearConnectionRefs_NilsPointers(t *testing.T) {
	fc := &fakeConn{}
	c := &Connection{
		conn:  fc,
		state: ConnDisconnected,
	}
	c.clearConnectionRefs()
	if c.conn != nil {
		t.Fatal("expected conn to be nil after clearConnectionRefs")
	}
	if c.tlsConn != nil {
		t.Fatal("expected tlsConn to be nil after clearConnectionRefs")
	}
	if c.framer != nil {
		t.Fatal("expected framer to be nil after clearConnectionRefs")
	}
}

func TestTransitionToDisconnected_Idempotent(t *testing.T) {
	c := &Connection{state: ConnDisconnected}
	// Should not panic even with nil socket.
	c.transitionTo(ConnDisconnected)
	if c.state != ConnDisconnected {
		t.Fatalf("expected ConnDisconnected, got %d", c.state)
	}
}

func TestTransitionToOperational_KeepsSocket(t *testing.T) {
	fc := &fakeConn{}
	c := &Connection{
		conn:  fc,
		state: ConnTLSConnected,
	}
	c.transitionTo(ConnOperational)
	if fc.closed {
		t.Fatal("socket should not be closed for non-disconnect transition")
	}
	if c.state != ConnOperational {
		t.Fatalf("expected ConnOperational, got %d", c.state)
	}
}

func TestIsConnected(t *testing.T) {
	c := &Connection{state: ConnDisconnected}
	if c.isConnected() {
		t.Fatal("disconnected should not be connected")
	}
	c.state = ConnTLSConnected
	if !c.isConnected() {
		t.Fatal("TLS connected should be connected")
	}
	c.state = ConnOperational
	if !c.isConnected() {
		t.Fatal("operational should be connected")
	}
}

func TestIsOperational(t *testing.T) {
	c := &Connection{state: ConnDisconnected}
	if c.isOperational() {
		t.Fatal("disconnected should not be operational")
	}
	c.state = ConnTLSConnected
	if c.isOperational() {
		t.Fatal("TLS connected should not be operational")
	}
	c.state = ConnOperational
	if !c.isOperational() {
		t.Fatal("operational should be operational")
	}
}

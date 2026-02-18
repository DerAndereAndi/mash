package runner

import (
	"net"
	"time"
)

const postHandshakeRejectionProbeTimeout = 500 * time.Millisecond

// detectPostHandshakeClose checks whether the peer closes the connection within
// the provided window. Returns (closed=true, err=closeErr) on close/reject,
// (closed=false, err=nil) on timeout/no close.
func detectPostHandshakeClose(conn net.Conn, timeout time.Duration) (bool, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	defer conn.SetReadDeadline(time.Time{})

	probe := make([]byte, 1)
	_, err := conn.Read(probe)
	if err == nil {
		// Unexpected data means connection stayed open for probe purposes.
		return false, nil
	}
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return false, nil
	}
	return true, err
}

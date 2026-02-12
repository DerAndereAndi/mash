package service

import (
	"net"
	"sync"
	"time"
)

// connTracker tracks pre-operational connections and their creation times.
// It is used by the stale connection reaper (DEC-064) to force-close
// connections that have been idle too long without completing commissioning.
type connTracker struct {
	mu    sync.Mutex
	conns map[net.Conn]time.Time
}

// newConnTracker creates a new connection tracker.
func newConnTracker() *connTracker {
	return &connTracker{
		conns: make(map[net.Conn]time.Time),
	}
}

// Add registers a connection with the current time.
func (ct *connTracker) Add(conn net.Conn) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.conns[conn] = time.Now()
}

// Remove deregisters a connection. Safe to call on absent connections.
func (ct *connTracker) Remove(conn net.Conn) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	delete(ct.conns, conn)
}

// CloseStale closes and removes all connections older than maxAge.
// Returns the number of connections closed.
func (ct *connTracker) CloseStale(maxAge time.Duration) int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	closed := 0
	for conn, added := range ct.conns {
		if added.Before(cutoff) {
			_ = conn.Close()
			delete(ct.conns, conn)
			closed++
		}
	}
	return closed
}

// CloseAll closes and removes all tracked connections.
func (ct *connTracker) CloseAll() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	closed := 0
	for conn := range ct.conns {
		_ = conn.Close()
		delete(ct.conns, conn)
		closed++
	}
	return closed
}

// Len returns the number of tracked connections.
func (ct *connTracker) Len() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return len(ct.conns)
}

package service

import (
	"net"
	"sync"
	"testing"
	"time"
)

// mockTrackerConn implements net.Conn for tracker tests.
// Only Close() is meaningful; other methods are stubs.
type mockTrackerConn struct {
	closed bool
	mu     sync.Mutex
}

func (c *mockTrackerConn) Read([]byte) (int, error)         { return 0, nil }
func (c *mockTrackerConn) Write([]byte) (int, error)        { return 0, nil }
func (c *mockTrackerConn) LocalAddr() net.Addr               { return nil }
func (c *mockTrackerConn) RemoteAddr() net.Addr              { return nil }
func (c *mockTrackerConn) SetDeadline(time.Time) error       { return nil }
func (c *mockTrackerConn) SetReadDeadline(time.Time) error   { return nil }
func (c *mockTrackerConn) SetWriteDeadline(time.Time) error  { return nil }

func (c *mockTrackerConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *mockTrackerConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func TestConnTracker_AddAndRemove(t *testing.T) {
	ct := newConnTracker()
	conn := &mockTrackerConn{}

	ct.Add(conn)
	if ct.Len() != 1 {
		t.Errorf("Len after Add: expected 1, got %d", ct.Len())
	}

	ct.Remove(conn)
	if ct.Len() != 0 {
		t.Errorf("Len after Remove: expected 0, got %d", ct.Len())
	}
}

func TestConnTracker_CloseStale_ClosesOldConnections(t *testing.T) {
	ct := newConnTracker()

	stale := &mockTrackerConn{}
	fresh := &mockTrackerConn{}

	// Add stale conn with an old timestamp
	ct.Add(stale)
	ct.mu.Lock()
	ct.conns[stale] = time.Now().Add(-2 * time.Minute)
	ct.mu.Unlock()

	// Add fresh conn (just now)
	ct.Add(fresh)

	closed := ct.CloseStale(1 * time.Minute)

	if closed != 1 {
		t.Errorf("CloseStale: expected 1 closed, got %d", closed)
	}
	if !stale.isClosed() {
		t.Error("stale conn should be closed")
	}
	if fresh.isClosed() {
		t.Error("fresh conn should NOT be closed")
	}
	if ct.Len() != 1 {
		t.Errorf("Len after CloseStale: expected 1, got %d", ct.Len())
	}
}

func TestConnTracker_CloseStale_NoneStale(t *testing.T) {
	ct := newConnTracker()

	conn1 := &mockTrackerConn{}
	conn2 := &mockTrackerConn{}
	ct.Add(conn1)
	ct.Add(conn2)

	closed := ct.CloseStale(1 * time.Minute)

	if closed != 0 {
		t.Errorf("CloseStale: expected 0 closed, got %d", closed)
	}
	if ct.Len() != 2 {
		t.Errorf("Len: expected 2, got %d", ct.Len())
	}
}

func TestConnTracker_CloseStale_AllStale(t *testing.T) {
	ct := newConnTracker()

	conns := make([]*mockTrackerConn, 3)
	for i := range conns {
		conns[i] = &mockTrackerConn{}
		ct.Add(conns[i])
	}

	// Make all stale
	ct.mu.Lock()
	for conn := range ct.conns {
		ct.conns[conn] = time.Now().Add(-5 * time.Minute)
	}
	ct.mu.Unlock()

	closed := ct.CloseStale(1 * time.Minute)

	if closed != 3 {
		t.Errorf("CloseStale: expected 3 closed, got %d", closed)
	}
	if ct.Len() != 0 {
		t.Errorf("Len: expected 0, got %d", ct.Len())
	}
	for i, c := range conns {
		if !c.isClosed() {
			t.Errorf("conn[%d] should be closed", i)
		}
	}
}

func TestConnTracker_RemoveIdempotent(t *testing.T) {
	ct := newConnTracker()
	conn := &mockTrackerConn{}

	// Remove a conn that was never added -- should not panic.
	ct.Remove(conn)

	ct.Add(conn)
	ct.Remove(conn)
	ct.Remove(conn) // second remove -- should not panic

	if ct.Len() != 0 {
		t.Errorf("Len: expected 0, got %d", ct.Len())
	}
}

func TestConnTracker_ConcurrentAccess(t *testing.T) {
	ct := newConnTracker()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			conn := &mockTrackerConn{}
			ct.Add(conn)
			ct.CloseStale(1 * time.Hour) // Nothing should be stale
			ct.Remove(conn)
		}()
	}

	wg.Wait()

	if ct.Len() != 0 {
		t.Errorf("Len after concurrent access: expected 0, got %d", ct.Len())
	}
}

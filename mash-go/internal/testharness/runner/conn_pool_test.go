package runner

import (
	"context"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// helper: create a ConnPool with a piped main connection for I/O tests.
func newPipedPool() (ConnPool, net.Conn) {
	client, server := net.Pipe()
	conn := &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}
	pool := NewConnPool(nil, nil)
	pool.SetMain(conn)
	return pool, server
}

// --- Message ID ---

func TestConnPool_NextMessageID_StartsAtOne(t *testing.T) {
	pool := NewConnPool(nil, nil)
	got := pool.NextMessageID()
	if got != 1 {
		t.Errorf("expected first message ID to be 1, got %d", got)
	}
}

func TestConnPool_NextMessageID_Atomic_Concurrent(t *testing.T) {
	pool := NewConnPool(nil, nil)
	const goroutines = 100
	var wg sync.WaitGroup
	ids := make([]uint32, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = pool.NextMessageID()
		}(i)
	}
	wg.Wait()
	seen := make(map[uint32]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate message ID: %d", id)
		}
		seen[id] = true
	}
}

// --- Main connection ---

func TestConnPool_Main_ReturnsNil_Initially(t *testing.T) {
	pool := NewConnPool(nil, nil)
	if pool.Main() != nil {
		t.Error("expected Main() to return nil for new pool")
	}
}

func TestConnPool_SetMain_And_Main_RoundTrip(t *testing.T) {
	pool := NewConnPool(nil, nil)
	conn := &Connection{state: ConnOperational}
	pool.SetMain(conn)
	if pool.Main() != conn {
		t.Error("expected Main() to return the connection that was set")
	}
}

// --- SendRequest ---

func TestConnPool_SendRequest_Success(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	msgID := pool.NextMessageID()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // drain request
		resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead, EndpointID: 0, FeatureID: 1}
	data, _ := wire.EncodeRequest(req)

	resp, err := pool.SendRequest(data, "read", msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != msgID {
		t.Errorf("expected msgID=%d, got %d", msgID, resp.MessageID)
	}
	if resp.Status != wire.StatusSuccess {
		t.Errorf("expected success status, got %d", resp.Status)
	}
}

func TestConnPool_SendRequest_WriteError_TransitionsToDisconnected(t *testing.T) {
	pool, server := newPipedPool()
	server.Close() // close server side so writes fail

	_, err := pool.SendRequest([]byte{0x01}, "read", 1)
	if err == nil {
		t.Fatal("expected error from closed connection")
	}
	if pool.Main().state != ConnDisconnected {
		t.Error("expected connection to be disconnected after write error")
	}
}

func TestConnPool_SendRequest_ReadError_TransitionsToDisconnected(t *testing.T) {
	pool, server := newPipedPool()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // read the request
		server.Close()            // close before sending response
	}()

	msgID := pool.NextMessageID()
	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead}
	data, _ := wire.EncodeRequest(req)

	_, err := pool.SendRequest(data, "read", msgID)
	if err == nil {
		t.Fatal("expected error from closed connection")
	}
	if pool.Main().state != ConnDisconnected {
		t.Error("expected connection to be disconnected after read error")
	}
}

func TestConnPool_SendRequest_SkipsNotifications_BuffersThemAsPending(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	msgID := pool.NextMessageID()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // drain request

		// Send a notification first (messageID=0).
		notif := &wire.Response{MessageID: 0, Status: wire.StatusSuccess}
		notifData, _ := wire.EncodeResponse(notif)
		_ = framer.WriteFrame(notifData)

		// Then the real response.
		resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead}
	data, _ := wire.EncodeRequest(req)

	resp, err := pool.SendRequest(data, "read", msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != msgID {
		t.Errorf("expected msgID=%d, got %d", msgID, resp.MessageID)
	}

	// Notification should be buffered as pending.
	pending := pool.PendingNotifications()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending notification, got %d", len(pending))
	}
}

func TestConnPool_SendRequest_DiscardsOrphanedResponses(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	msgID := pool.NextMessageID()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // drain request

		// Send an orphaned response (wrong msgID).
		orphan := &wire.Response{MessageID: 999, Status: wire.StatusSuccess}
		orphanData, _ := wire.EncodeResponse(orphan)
		_ = framer.WriteFrame(orphanData)

		// Then the real response.
		resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead}
	data, _ := wire.EncodeRequest(req)

	resp, err := pool.SendRequest(data, "read", msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != msgID {
		t.Errorf("expected msgID=%d, got %d", msgID, resp.MessageID)
	}
}

func TestConnPool_SendRequest_TooManyInterleavedFrames_ReturnsError(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	msgID := pool.NextMessageID()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // drain request

		// Send 10 notifications (fills the 10-frame limit).
		for range 10 {
			notif := &wire.Response{MessageID: 0, Status: wire.StatusSuccess}
			notifData, _ := wire.EncodeResponse(notif)
			_ = framer.WriteFrame(notifData)
		}
	}()

	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead}
	data, _ := wire.EncodeRequest(req)

	_, err := pool.SendRequest(data, "read", msgID)
	if err == nil {
		t.Fatal("expected error from too many interleaved frames")
	}
}

func TestConnPool_SendRequestWithDeadline_RespectsContextDeadline(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	// Server reads but never responds, so the deadline should fire.
	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		// Intentionally don't respond.
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	msgID := pool.NextMessageID()
	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead}
	data, _ := wire.EncodeRequest(req)

	start := time.Now()
	_, err := pool.SendRequestWithDeadline(ctx, data, "read", msgID)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from deadline")
	}
	if elapsed > 2*time.Second {
		t.Errorf("expected deadline to fire within 2s, took %v", elapsed)
	}
}

// --- Zone tracking ---

func TestConnPool_TrackZone_And_Zone_RoundTrip(t *testing.T) {
	pool := NewConnPool(nil, nil)
	conn := &Connection{state: ConnOperational}
	pool.TrackZone("main-abc123", conn, "abc123")
	if pool.Zone("main-abc123") != conn {
		t.Error("expected Zone() to return the tracked connection")
	}
}

func TestConnPool_Zone_ReturnsNil_ForUnknown(t *testing.T) {
	pool := NewConnPool(nil, nil)
	if pool.Zone("nonexistent") != nil {
		t.Error("expected nil for unknown zone key")
	}
}

func TestConnPool_ZoneCount_ReflectsTrackedZones(t *testing.T) {
	pool := NewConnPool(nil, nil)
	if pool.ZoneCount() != 0 {
		t.Error("expected 0 zones initially")
	}
	pool.TrackZone("z1", &Connection{}, "id1")
	pool.TrackZone("z2", &Connection{}, "id2")
	if pool.ZoneCount() != 2 {
		t.Errorf("expected 2 zones, got %d", pool.ZoneCount())
	}
}

func TestConnPool_ZoneKeys_ReturnsAllKeys(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.TrackZone("z1", &Connection{}, "id1")
	pool.TrackZone("z2", &Connection{}, "id2")
	keys := pool.ZoneKeys()
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "z1" || keys[1] != "z2" {
		t.Errorf("expected [z1, z2], got %v", keys)
	}
}

// --- Zone cleanup ---

func TestConnPool_CloseZonesExcept_ClosesNonExcepted(t *testing.T) {
	var closedZoneIDs []string
	pool := NewConnPool(nil, func(conn *Connection, zoneID string) {
		closedZoneIDs = append(closedZoneIDs, zoneID)
	})
	conn1 := &Connection{state: ConnOperational}
	conn2 := &Connection{state: ConnOperational}
	pool.TrackZone("z1", conn1, "id1")
	pool.TrackZone("z2", conn2, "id2")

	pool.CloseZonesExcept("z1")

	if pool.Zone("z2") != nil {
		t.Error("expected z2 to be removed")
	}
	if pool.Zone("z1") == nil {
		t.Error("expected z1 to be preserved")
	}
	if len(closedZoneIDs) != 1 || closedZoneIDs[0] != "id2" {
		t.Errorf("expected callback for id2, got %v", closedZoneIDs)
	}
}

func TestConnPool_CloseZonesExcept_SkipsExceptedKey(t *testing.T) {
	callbackCalled := false
	pool := NewConnPool(nil, func(conn *Connection, zoneID string) {
		callbackCalled = true
	})
	conn := &Connection{state: ConnOperational}
	pool.TrackZone("suite", conn, "sid")

	pool.CloseZonesExcept("suite")

	if callbackCalled {
		t.Error("expected callback to NOT be called for excepted key")
	}
	if pool.Zone("suite") == nil {
		t.Error("expected suite zone to be preserved")
	}
}

func TestConnPool_CloseZonesExcept_ReturnsCloseTime(t *testing.T) {
	pool := NewConnPool(nil, nil)
	client, server := net.Pipe()
	defer server.Close()
	conn := &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}
	pool.TrackZone("z1", conn, "id1")

	before := time.Now()
	closeTime := pool.CloseZonesExcept("")
	after := time.Now()

	if closeTime.Before(before) || closeTime.After(after) {
		t.Errorf("expected close time between %v and %v, got %v", before, after, closeTime)
	}
}

func TestConnPool_CloseZonesExcept_NoZones_ReturnsZeroTime(t *testing.T) {
	pool := NewConnPool(nil, nil)
	closeTime := pool.CloseZonesExcept("")
	if !closeTime.IsZero() {
		t.Error("expected zero time when no zones to close")
	}
}

func TestConnPool_CloseAllZones_ClosesEverything(t *testing.T) {
	closeCount := 0
	pool := NewConnPool(nil, func(conn *Connection, zoneID string) {
		closeCount++
	})
	pool.TrackZone("z1", &Connection{state: ConnOperational}, "id1")
	pool.TrackZone("z2", &Connection{state: ConnOperational}, "id2")

	pool.CloseAllZones()

	if pool.ZoneCount() != 0 {
		t.Errorf("expected 0 zones after CloseAllZones, got %d", pool.ZoneCount())
	}
}

// --- Subscriptions ---

func TestConnPool_TrackSubscription_And_UnsubscribeAll(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	pool.TrackSubscription(42)
	pool.TrackSubscription(43)

	// Server: respond to 2 sequential unsubscribe requests.
	// Message IDs are 1 and 2 since the pool is fresh (NextMessageID starts at 0).
	go func() {
		framer := transport.NewFramer(server)
		for msgID := uint32(1); msgID <= 2; msgID++ {
			_, err := framer.ReadFrame()
			if err != nil {
				return
			}
			resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
			respData, _ := wire.EncodeResponse(resp)
			_ = framer.WriteFrame(respData)
		}
	}()

	pool.UnsubscribeAll(pool.Main())
	// Success: no hang, no panic. Subscription list is cleared internally.
}

func TestConnPool_RemoveSubscription(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.TrackSubscription(42)
	pool.TrackSubscription(43)
	pool.RemoveSubscription(42)
	// Verify no panic. The internal list should contain only [43].
}

// --- Notifications ---

func TestConnPool_AppendNotification_And_PendingNotifications(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.AppendNotification([]byte{1, 2, 3})
	pool.AppendNotification([]byte{4, 5, 6})

	pending := pool.PendingNotifications()
	if len(pending) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(pending))
	}
}

func TestConnPool_PendingNotifications_DoesNotClear(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.AppendNotification([]byte{1})

	_ = pool.PendingNotifications()
	pending := pool.PendingNotifications()
	if len(pending) != 1 {
		t.Errorf("expected 1 after second call (no auto-clear), got %d", len(pending))
	}
}

func TestConnPool_ShiftNotification_FIFO(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.AppendNotification([]byte{1})
	pool.AppendNotification([]byte{2})

	data, ok := pool.ShiftNotification()
	if !ok || data[0] != 1 {
		t.Errorf("expected first notification [1], got %v ok=%v", data, ok)
	}
	data, ok = pool.ShiftNotification()
	if !ok || data[0] != 2 {
		t.Errorf("expected second notification [2], got %v ok=%v", data, ok)
	}
	_, ok = pool.ShiftNotification()
	if ok {
		t.Error("expected false when buffer is empty")
	}
}

func TestConnPool_ZoneID_ReturnsTrackedID(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.TrackZone("z1", &Connection{}, "abc123")
	if pool.ZoneID("z1") != "abc123" {
		t.Errorf("expected abc123, got %s", pool.ZoneID("z1"))
	}
	if pool.ZoneID("nonexistent") != "" {
		t.Error("expected empty for nonexistent")
	}
}

func TestConnPool_UntrackZone_RemovesBothMaps(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.TrackZone("z1", &Connection{}, "id1")
	pool.UntrackZone("z1")
	if pool.Zone("z1") != nil {
		t.Error("expected nil after untrack")
	}
	if pool.ZoneID("z1") != "" {
		t.Error("expected empty zone ID after untrack")
	}
	if pool.ZoneCount() != 0 {
		t.Error("expected 0 zones after untrack")
	}
}

func TestConnPool_ClearNotifications(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.AppendNotification([]byte{1})
	pool.ClearNotifications()

	pending := pool.PendingNotifications()
	if len(pending) != 0 {
		t.Errorf("expected empty after ClearNotifications, got %d", len(pending))
	}
}

// --- Narrow sub-interface compile-time checks ---

// Verify connPoolImpl satisfies each narrow sub-interface.
var (
	_ ConnReader         = (*connPoolImpl)(nil)
	_ ConnWriter         = (*connPoolImpl)(nil)
	_ ConnLifecycle      = (*connPoolImpl)(nil)
	_ RequestSender      = (*connPoolImpl)(nil)
	_ NotificationBuffer = (*connPoolImpl)(nil)
)

// TestNarrowConnReader verifies that a function accepting only ConnReader
// can read pool state without access to mutating operations.
func TestNarrowConnReader(t *testing.T) {
	pool := NewConnPool(nil, nil)
	pool.TrackZone("z1", &Connection{state: ConnOperational}, "id1")

	// Use ConnReader to read state.
	var reader ConnReader = pool.(ConnReader)
	if reader.ZoneCount() != 1 {
		t.Errorf("expected 1 zone, got %d", reader.ZoneCount())
	}
	if reader.ZoneID("z1") != "id1" {
		t.Errorf("expected id1, got %s", reader.ZoneID("z1"))
	}
}

// TestNarrowRequestSender verifies that a function accepting only
// RequestSender can send protocol requests.
func TestNarrowRequestSender(t *testing.T) {
	pool, server := newPipedPool()
	defer server.Close()

	msgID := pool.NextMessageID()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	// Use RequestSender to send and receive.
	var sender RequestSender = pool.(RequestSender)
	req := &wire.Request{MessageID: msgID, Operation: wire.OpRead, EndpointID: 0, FeatureID: 1}
	data, _ := wire.EncodeRequest(req)

	resp, err := sender.SendRequest(data, "read", msgID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MessageID != msgID {
		t.Errorf("expected msgID=%d, got %d", msgID, resp.MessageID)
	}
}

// TestNarrowNotificationBuffer verifies that a function accepting only
// NotificationBuffer can manage the notification queue.
func TestNarrowNotificationBuffer(t *testing.T) {
	pool := NewConnPool(nil, nil)

	var buf NotificationBuffer = pool.(NotificationBuffer)
	buf.AppendNotification([]byte{42})
	data, ok := buf.ShiftNotification()
	if !ok || data[0] != 42 {
		t.Errorf("expected [42], got %v ok=%v", data, ok)
	}
}

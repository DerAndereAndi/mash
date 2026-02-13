package runner

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ConnReader provides read-only access to pool state.
// Used by handlers that need to inspect connections and zone state.
type ConnReader interface {
	Main() *Connection
	Zone(key string) *Connection
	ZoneID(key string) string
	ZoneCount() int
	ZoneKeys() []string
	NextMessageID() uint32
	Subscriptions() []uint32
}

// ConnWriter provides mutating pool operations.
// Used by handlers that manage zone lifecycle and subscription tracking.
type ConnWriter interface {
	SetMain(conn *Connection)
	TrackZone(key string, conn *Connection, zoneID string)
	UntrackZone(key string)
	TrackSubscription(subID uint32)
	RemoveSubscription(subID uint32)
}

// ConnLifecycle manages connection close and cleanup operations.
// Used by Coordinator teardown and preconditions.
type ConnLifecycle interface {
	CloseZonesExcept(exceptKey string) time.Time
	CloseAllZones() time.Time
	UnsubscribeAll(conn *Connection)
}

// RequestSender sends wire-level requests and reads responses.
// Used by handlers for protocol operations.
type RequestSender interface {
	SendRequest(data []byte, op string, expectedMsgID uint32) (*wire.Response, error)
	SendRequestWithDeadline(ctx context.Context, data []byte, op string, expectedMsgID uint32) (*wire.Response, error)
}

// NotificationBuffer manages the notification queue.
// Used by utility handlers for notification inspection.
type NotificationBuffer interface {
	PendingNotifications() [][]byte
	ShiftNotification() ([]byte, bool)
	AppendNotification(data []byte)
	ClearNotifications()
}

// ConnPool manages connections to the device under test.
// It composes the focused sub-interfaces above for backward compatibility.
type ConnPool interface {
	ConnReader
	ConnWriter
	ConnLifecycle
	RequestSender
	NotificationBuffer
}

type connPoolImpl struct {
	main          *Connection
	zones         map[string]*Connection
	zoneIDs       map[string]string
	messageID     uint32 // atomic
	subscriptions []uint32
	notifications [][]byte
	debugFn       func(string, ...any)
	onZoneClose   func(conn *Connection, zoneID string)
}

// NewConnPool creates a ConnPool. debugFn may be nil (debug logging is skipped).
// onZoneClose is called before closing each zone connection in CloseZonesExcept;
// it typically sends RemoveZone + ControlClose. May be nil for tests.
func NewConnPool(debugFn func(string, ...any), onZoneClose func(conn *Connection, zoneID string)) ConnPool {
	return &connPoolImpl{
		zones:       make(map[string]*Connection),
		zoneIDs:     make(map[string]string),
		debugFn:     debugFn,
		onZoneClose: onZoneClose,
	}
}

func (p *connPoolImpl) debugf(format string, args ...any) {
	if p.debugFn != nil {
		p.debugFn(format, args...)
	}
}

// --- Main connection ---

func (p *connPoolImpl) Main() *Connection     { return p.main }
func (p *connPoolImpl) SetMain(conn *Connection) { p.main = conn }

// --- Message ID ---

func (p *connPoolImpl) NextMessageID() uint32 {
	return atomic.AddUint32(&p.messageID, 1)
}

// --- SendRequest ---

func (p *connPoolImpl) SendRequest(data []byte, op string, expectedMsgID uint32) (*wire.Response, error) {
	return p.SendRequestWithDeadline(context.Background(), data, op, expectedMsgID)
}

func (p *connPoolImpl) SendRequestWithDeadline(ctx context.Context, data []byte, op string, expectedMsgID uint32) (*wire.Response, error) {
	if err := p.main.framer.WriteFrame(data); err != nil {
		p.main.transitionTo(ConnDisconnected)
		return nil, fmt.Errorf("failed to send %s request: %w", op, err)
	}

	// Set a read deadline so we don't block forever if the device never
	// responds. Use context deadline if available, otherwise 30s.
	deadline := time.Now().Add(30 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if p.main.tlsConn != nil {
		_ = p.main.tlsConn.SetReadDeadline(deadline)
		defer func() {
			if p.main.tlsConn != nil {
				_ = p.main.tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	} else if p.main.conn != nil {
		_ = p.main.conn.SetReadDeadline(deadline)
		defer func() {
			if p.main.conn != nil {
				_ = p.main.conn.SetReadDeadline(time.Time{})
			}
		}()
	}

	// Read frames until we get a matching response. Skip notifications
	// (messageId=0) and discard orphaned responses from previous operations.
	for range 10 {
		respData, err := p.main.framer.ReadFrame()
		if err != nil {
			p.main.transitionTo(ConnDisconnected)
			return nil, fmt.Errorf("failed to read %s response: %w", op, err)
		}

		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode %s response: %w", op, err)
		}

		// Notifications have messageId=0. Queue them for later consumption.
		if resp.MessageID == 0 {
			p.debugf("sendRequest(%s): skipping notification frame (buffered)", op)
			p.notifications = append(p.notifications, respData)
			continue
		}

		// Discard orphaned responses from previous operations.
		if resp.MessageID != expectedMsgID {
			p.debugf("sendRequest(%s): discarding orphaned response (got msgID=%d, want %d)", op, resp.MessageID, expectedMsgID)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("failed to read %s response: too many interleaved frames", op)
}

// --- Zone tracking ---

func (p *connPoolImpl) Zone(key string) *Connection { return p.zones[key] }

func (p *connPoolImpl) TrackZone(key string, conn *Connection, zoneID string) {
	p.zones[key] = conn
	p.zoneIDs[key] = zoneID
}

func (p *connPoolImpl) ZoneID(key string) string { return p.zoneIDs[key] }

func (p *connPoolImpl) UntrackZone(key string) {
	delete(p.zones, key)
	delete(p.zoneIDs, key)
}

func (p *connPoolImpl) ZoneCount() int { return len(p.zones) }

func (p *connPoolImpl) ZoneKeys() []string {
	keys := make([]string, 0, len(p.zones))
	for k := range p.zones {
		keys = append(keys, k)
	}
	return keys
}

// --- Zone cleanup ---

func (p *connPoolImpl) CloseZonesExcept(exceptKey string) time.Time {
	closedAny := false
	for key, conn := range p.zones {
		if exceptKey != "" && key == exceptKey {
			p.debugf("closeZones: keeping zone %s", key)
			continue
		}

		// Call the protocol-level close callback (RemoveZone + ControlClose).
		if p.onZoneClose != nil {
			if zoneID, ok := p.zoneIDs[key]; ok {
				p.onZoneClose(conn, zoneID)
			}
		}

		if conn.tlsConn != nil || conn.conn != nil {
			closedAny = true
		}
		_ = conn.Close()
		conn.clearConnectionRefs()
		delete(p.zones, key)
		delete(p.zoneIDs, key)
	}
	if closedAny {
		return time.Now()
	}
	return time.Time{}
}

func (p *connPoolImpl) CloseAllZones() time.Time {
	return p.CloseZonesExcept("")
}

// --- Subscriptions ---

func (p *connPoolImpl) TrackSubscription(subID uint32) {
	p.subscriptions = append(p.subscriptions, subID)
}

func (p *connPoolImpl) Subscriptions() []uint32 { return p.subscriptions }

func (p *connPoolImpl) RemoveSubscription(subID uint32) {
	for i, id := range p.subscriptions {
		if id == subID {
			p.subscriptions = append(p.subscriptions[:i], p.subscriptions[i+1:]...)
			return
		}
	}
}

func (p *connPoolImpl) UnsubscribeAll(conn *Connection) {
	if conn == nil || !conn.isConnected() || conn.framer == nil {
		p.subscriptions = nil
		return
	}
	for _, subID := range p.subscriptions {
		p.sendUnsubscribe(conn, subID)
	}
	p.subscriptions = nil
}

func (p *connPoolImpl) sendUnsubscribe(conn *Connection, subID uint32) {
	if conn == nil || !conn.isConnected() || conn.framer == nil {
		return
	}
	req := &wire.Request{
		MessageID:  p.NextMessageID(),
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0,
		Payload:    &wire.UnsubscribePayload{SubscriptionID: subID},
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return
	}
	if err := conn.framer.WriteFrame(data); err != nil {
		return
	}

	// Read frames until the unsubscribe response arrives, discarding any
	// interleaved notifications. Short deadline to avoid blocking.
	if conn.tlsConn != nil {
		_ = conn.tlsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		defer func() {
			if conn.tlsConn != nil {
				_ = conn.tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	} else if conn.conn != nil {
		_ = conn.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		defer func() {
			if conn.conn != nil {
				_ = conn.conn.SetReadDeadline(time.Time{})
			}
		}()
	}
	drained := 0
	for range 20 {
		respData, err := conn.framer.ReadFrame()
		if err != nil {
			break
		}
		resp, decErr := wire.DecodeResponse(respData)
		if decErr != nil {
			break
		}
		if resp.MessageID == 0 {
			drained++
			continue
		}
		if resp.MessageID != req.MessageID {
			p.debugf("sendUnsubscribe(%d): discarding orphaned response (got msgID=%d, want %d)", subID, resp.MessageID, req.MessageID)
			drained++
			continue
		}
		break
	}
	if drained > 0 {
		p.debugf("sendUnsubscribe(%d): discarded %d stale frames", subID, drained)
	}
}

// --- Notifications ---

func (p *connPoolImpl) PendingNotifications() [][]byte {
	return p.notifications
}

func (p *connPoolImpl) ShiftNotification() ([]byte, bool) {
	if len(p.notifications) == 0 {
		return nil, false
	}
	data := p.notifications[0]
	p.notifications = p.notifications[1:]
	return data, true
}

func (p *connPoolImpl) AppendNotification(data []byte) {
	p.notifications = append(p.notifications, data)
}

func (p *connPoolImpl) ClearNotifications() {
	p.notifications = nil
}

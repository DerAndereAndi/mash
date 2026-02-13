package runner

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ConnPool manages connections to the device under test.
type ConnPool interface {
	// Main returns the current main connection, or nil if not connected.
	Main() *Connection

	// SetMain replaces the main connection.
	SetMain(conn *Connection)

	// NextMessageID returns the next atomic message ID.
	NextMessageID() uint32

	// SendRequest sends a request on the main connection and reads the response.
	// Handles notification interleaving and orphaned response filtering.
	// On IO error, transitions the main connection to ConnDisconnected.
	SendRequest(data []byte, op string, expectedMsgID uint32) (*wire.Response, error)

	// SendRequestWithDeadline is like SendRequest but respects the context deadline.
	SendRequestWithDeadline(ctx context.Context, data []byte, op string, expectedMsgID uint32) (*wire.Response, error)

	// Zone returns the connection for the given zone key, or nil if not tracked.
	Zone(key string) *Connection

	// TrackZone registers a connection under the given key with its zone ID.
	TrackZone(key string, conn *Connection, zoneID string)

	// CloseZonesExcept closes and removes all tracked zone connections except
	// the one matching exceptKey. Calls onZoneClose before closing each socket.
	// Returns the time of the last real connection close, or zero if none.
	CloseZonesExcept(exceptKey string) time.Time

	// CloseAllZones closes all tracked zone connections including the suite zone.
	CloseAllZones() time.Time

	// ZoneCount returns the number of tracked zone connections.
	ZoneCount() int

	// ZoneKeys returns all tracked zone connection keys.
	ZoneKeys() []string

	// TrackSubscription records an active subscription ID for later cleanup.
	TrackSubscription(subID uint32)

	// RemoveSubscription removes a subscription ID from tracking.
	RemoveSubscription(subID uint32)

	// Subscriptions returns the current list of tracked subscription IDs.
	Subscriptions() []uint32

	// UnsubscribeAll sends Unsubscribe for all tracked subscription IDs
	// on the given connection, then clears the tracking list.
	UnsubscribeAll(conn *Connection)

	// ZoneID returns the zone ID for a tracked zone connection key.
	ZoneID(key string) string

	// UntrackZone removes a zone from tracking without closing the connection.
	UntrackZone(key string)

	// PendingNotifications returns all buffered notification frames without clearing.
	PendingNotifications() [][]byte

	// ShiftNotification pops and returns the first buffered notification.
	// Returns nil, false if no notifications are buffered.
	ShiftNotification() ([]byte, bool)

	// AppendNotification buffers a notification frame.
	AppendNotification(data []byte)

	// ClearNotifications discards all buffered notification frames.
	ClearNotifications()
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

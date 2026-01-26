package service

import (
	"sync"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ZoneSession manages a device-side session with a connected controller zone.
// It handles incoming requests, routes them to the ProtocolHandler,
// and sends responses back over the connection.
type ZoneSession struct {
	mu sync.RWMutex

	zoneID  string
	conn    Sendable
	handler *ProtocolHandler
	closed  bool
}

// NewZoneSession creates a new zone session.
func NewZoneSession(zoneID string, conn Sendable, device *model.Device) *ZoneSession {
	handler := NewProtocolHandler(device)
	handler.SetZoneID(zoneID)

	return &ZoneSession{
		zoneID:  zoneID,
		conn:    conn,
		handler: handler,
	}
}

// ZoneID returns the session's zone ID.
func (s *ZoneSession) ZoneID() string {
	return s.zoneID
}

// OnMessage handles an incoming message from the connection.
// This is called by the transport layer when data is received.
func (s *ZoneSession) OnMessage(data []byte) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	// Determine message type
	msgType, err := wire.PeekMessageType(data)
	if err != nil {
		// Invalid message - ignore
		return
	}

	switch msgType {
	case wire.MessageTypeRequest:
		s.handleRequest(data)
	case wire.MessageTypeNotification:
		// Devices don't typically receive notifications
		// (they send them), but ignore gracefully
	default:
		// Unknown message type - ignore
	}
}

// handleRequest processes an incoming request and sends a response.
func (s *ZoneSession) handleRequest(data []byte) {
	// Decode request
	req, err := wire.DecodeRequest(data)
	if err != nil {
		// Send error response with messageID 0 (unknown)
		s.sendErrorResponse(0, wire.StatusInvalidParameter, "failed to decode request")
		return
	}

	// Process request through handler
	resp := s.handler.HandleRequest(req)

	// Send response
	respData, err := wire.EncodeResponse(resp)
	if err != nil {
		// Can't encode response - send error with simpler payload
		s.sendErrorResponse(req.MessageID, wire.StatusBusy, "failed to encode response")
		return
	}

	s.conn.Send(respData)
}

// sendErrorResponse sends an error response.
func (s *ZoneSession) sendErrorResponse(messageID uint32, status wire.Status, message string) {
	resp := &wire.Response{
		MessageID: messageID,
		Status:    status,
		Payload: &wire.ErrorPayload{
			Message: message,
		},
	}

	if respData, err := wire.EncodeResponse(resp); err == nil {
		s.conn.Send(respData)
	}
}

// SendNotification sends a subscription notification to the zone.
func (s *ZoneSession) SendNotification(notif *wire.Notification) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSessionClosed
	}
	s.mu.RUnlock()

	data, err := wire.EncodeNotification(notif)
	if err != nil {
		return err
	}

	return s.conn.Send(data)
}

// SubscriptionCount returns the number of active subscriptions.
func (s *ZoneSession) SubscriptionCount() int {
	return s.handler.SubscriptionCount()
}

// Close closes the session and cleans up resources.
func (s *ZoneSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true

	// Clear all subscriptions for this zone
	// Note: In a full implementation, we'd also notify the subscription
	// manager to stop sending notifications to this zone
	s.clearSubscriptions()
}

// clearSubscriptions removes all subscriptions from the handler.
// This is called when the session is closed.
func (s *ZoneSession) clearSubscriptions() {
	// The ProtocolHandler doesn't have a ClearAll method,
	// so we need to track subscription IDs and unsubscribe each
	// For now, we'll recreate the handler to clear subscriptions
	device := s.handler.Device()
	s.handler = NewProtocolHandler(device)
	s.handler.SetZoneID(s.zoneID)
}

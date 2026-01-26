package service

import (
	"log/slog"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
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
	logger  *slog.Logger

	// Renewal handling
	renewalHandler *DeviceRenewalHandler
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
	renewalHandler := s.renewalHandler
	s.mu.RUnlock()

	// Check for renewal messages first (MsgType 30-33 at key 1)
	if isRenewalMessage(data) {
		if renewalHandler != nil {
			s.handleRenewalMessage(data, renewalHandler)
		}
		return
	}

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
		if s.logger != nil {
			s.logger.Debug("SendNotification: session closed",
				"zoneID", s.zoneID,
				"subscriptionID", notif.SubscriptionID)
		}
		return ErrSessionClosed
	}
	logger := s.logger
	s.mu.RUnlock()

	if logger != nil {
		logger.Debug("SendNotification: encoding notification",
			"zoneID", s.zoneID,
			"subscriptionID", notif.SubscriptionID,
			"endpointID", notif.EndpointID,
			"featureID", notif.FeatureID,
			"changesCount", len(notif.Changes))
	}

	data, err := wire.EncodeNotification(notif)
	if err != nil {
		if logger != nil {
			logger.Debug("SendNotification: encode failed",
				"zoneID", s.zoneID,
				"error", err)
		}
		return err
	}

	if logger != nil {
		logger.Debug("SendNotification: sending",
			"zoneID", s.zoneID,
			"dataLen", len(data))
	}

	return s.conn.Send(data)
}

// SetLogger sets the logger for this session.
func (s *ZoneSession) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
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

// SetRenewalHandler sets the handler for certificate renewal messages.
func (s *ZoneSession) SetRenewalHandler(handler *DeviceRenewalHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewalHandler = handler
}

// handleRenewalMessage processes certificate renewal messages.
func (s *ZoneSession) handleRenewalMessage(data []byte, handler *DeviceRenewalHandler) {
	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return
	}

	var resp any

	switch m := msg.(type) {
	case *commissioning.CertRenewalRequest:
		resp, err = handler.HandleRenewalRequest(m)
	case *commissioning.CertRenewalInstall:
		resp, err = handler.HandleCertInstall(m)
	default:
		// Unknown renewal message type
		return
	}

	if err != nil {
		return
	}

	// Send response
	respData, err := commissioning.EncodeRenewalMessage(resp)
	if err != nil {
		return
	}

	s.conn.Send(respData)
}

// isRenewalMessage checks if data is a renewal message (MsgType 30-33).
func isRenewalMessage(data []byte) bool {
	// Quick check: renewal messages have MsgType at CBOR key 1 with value 30-33.
	// We need to peek at the first integer after key 1 in the CBOR map.
	// For simplicity, try to decode as a renewal message header.
	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return false
	}
	// If it decoded successfully, it's a renewal message
	msgType := commissioning.RenewalMessageType(msg)
	return msgType >= commissioning.MsgCertRenewalRequest && msgType <= commissioning.MsgCertRenewalAck
}

// InitializeRenewalHandler creates and sets a renewal handler with the given identity.
func (s *ZoneSession) InitializeRenewalHandler(identity *cert.DeviceIdentity) {
	handler := NewDeviceRenewalHandler(identity)
	s.SetRenewalHandler(handler)
}

// RenewalHandler returns the session's renewal handler.
func (s *ZoneSession) RenewalHandler() *DeviceRenewalHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.renewalHandler
}

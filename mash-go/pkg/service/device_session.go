package service

import (
	"context"
	"crypto/tls"
	"log/slog"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// DeviceSession manages a controller-side session with a connected device.
// It wraps an interaction.Client to provide Read/Write/Subscribe/Invoke
// operations to applications.
type DeviceSession struct {
	mu sync.RWMutex

	deviceID string
	conn     Sendable
	client   *interaction.Client
	sender   *TransportRequestSender
	closed   bool
	logger   *slog.Logger

	// Renewal handling
	renewalHandler *ControllerRenewalHandler
}

// NewDeviceSession creates a new device session.
func NewDeviceSession(deviceID string, conn Sendable) *DeviceSession {
	sender := NewTransportRequestSender(conn)
	client := interaction.NewClient(sender)

	return &DeviceSession{
		deviceID: deviceID,
		conn:     conn,
		client:   client,
		sender:   sender,
	}
}

// DeviceID returns the session's device ID.
func (s *DeviceSession) DeviceID() string {
	return s.deviceID
}

// OnMessage handles an incoming message from the device.
// This is called by the transport layer when data is received.
func (s *DeviceSession) OnMessage(data []byte) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	client := s.client
	logger := s.logger
	renewalHandler := s.renewalHandler
	s.mu.RUnlock()

	// Check for renewal response messages first (MsgType 31 or 33)
	if isRenewalMessage(data) {
		if renewalHandler != nil {
			s.handleRenewalResponse(data, renewalHandler)
		}
		return
	}

	// Determine message type
	msgType, err := wire.PeekMessageType(data)
	if err != nil {
		if logger != nil {
			logger.Debug("OnMessage: failed to peek message type",
				"deviceID", s.deviceID,
				"error", err)
		}
		return
	}

	switch msgType {
	case wire.MessageTypeResponse:
		// Decode and deliver to client
		resp, err := wire.DecodeResponse(data)
		if err != nil {
			if logger != nil {
				logger.Debug("OnMessage: failed to decode response",
					"deviceID", s.deviceID,
					"error", err)
			}
			return
		}
		client.HandleResponse(resp)

	case wire.MessageTypeNotification:
		// Decode and deliver to client
		notif, err := wire.DecodeNotification(data)
		if err != nil {
			if logger != nil {
				logger.Debug("OnMessage: failed to decode notification",
					"deviceID", s.deviceID,
					"error", err)
			}
			return
		}
		if logger != nil {
			logger.Debug("OnMessage: received notification",
				"deviceID", s.deviceID,
				"subscriptionID", notif.SubscriptionID,
				"endpointID", notif.EndpointID,
				"featureID", notif.FeatureID,
				"changesCount", len(notif.Changes))
			for attrID, val := range notif.Changes {
				logger.Debug("OnMessage: notification change",
					"attrID", attrID,
					"valueType", slog.AnyValue(val).Kind().String(),
					"value", val)
			}
		}
		client.HandleNotification(notif)
	}
}

// SetLogger sets the logger for this session.
func (s *DeviceSession) SetLogger(logger *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = logger
}

// Read reads attributes from a feature on the device.
// If attrIDs is nil, all attributes are read.
func (s *DeviceSession) Read(ctx context.Context, endpointID uint8, featureID uint8, attrIDs []uint16) (map[uint16]any, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSessionClosed
	}
	client := s.client
	s.mu.RUnlock()

	return client.Read(ctx, endpointID, featureID, attrIDs)
}

// Write writes attributes to a feature on the device.
func (s *DeviceSession) Write(ctx context.Context, endpointID uint8, featureID uint8, attrs map[uint16]any) (map[uint16]any, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSessionClosed
	}
	client := s.client
	s.mu.RUnlock()

	return client.Write(ctx, endpointID, featureID, attrs)
}

// Subscribe subscribes to attribute changes on a feature.
// Returns the subscription ID and initial attribute values (priming report).
func (s *DeviceSession) Subscribe(ctx context.Context, endpointID uint8, featureID uint8, opts *interaction.SubscribeOptions) (uint32, map[uint16]any, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, nil, ErrSessionClosed
	}
	client := s.client
	s.mu.RUnlock()

	return client.Subscribe(ctx, endpointID, featureID, opts)
}

// Unsubscribe cancels a subscription.
func (s *DeviceSession) Unsubscribe(ctx context.Context, subscriptionID uint32) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSessionClosed
	}
	client := s.client
	s.mu.RUnlock()

	return client.Unsubscribe(ctx, subscriptionID)
}

// Invoke executes a command on a feature.
func (s *DeviceSession) Invoke(ctx context.Context, endpointID uint8, featureID uint8, commandID uint8, params map[string]any) (any, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSessionClosed
	}
	client := s.client
	s.mu.RUnlock()

	return client.Invoke(ctx, endpointID, featureID, commandID, params)
}

// SetNotificationHandler sets the handler for incoming notifications.
func (s *DeviceSession) SetNotificationHandler(handler func(*wire.Notification)) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return
	}
	client := s.client
	s.mu.RUnlock()

	client.SetNotificationHandler(handler)
}

// Close closes the session and cleans up resources.
func (s *DeviceSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	return s.client.Close()
}

// SetRenewalHandler sets the handler for certificate renewal.
func (s *DeviceSession) SetRenewalHandler(handler *ControllerRenewalHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewalHandler = handler
}

// RenewalHandler returns the session's renewal handler.
func (s *DeviceSession) RenewalHandler() *ControllerRenewalHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.renewalHandler
}

// handleRenewalResponse processes a renewal response from the device.
func (s *DeviceSession) handleRenewalResponse(data []byte, handler *ControllerRenewalHandler) {
	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return
	}

	// Route to the handler's response channel
	handler.HandleResponse(msg)
}

// Conn returns the underlying connection (for renewal handler initialization).
func (s *DeviceSession) Conn() Sendable {
	return s.conn
}

// TLSConnectionState returns the TLS connection state for this session.
// Returns nil if the session is closed or connection is not TLS.
func (s *DeviceSession) TLSConnectionState() *tls.ConnectionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil
	}

	// Try to get TLS state from framed connection
	if fc, ok := s.conn.(*framedConnection); ok {
		return fc.TLSConnectionState()
	}
	return nil
}

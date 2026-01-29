package service

import (
	"context"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// NotificationSender is a function that sends a notification to the remote peer.
type NotificationSender func(notification *wire.Notification) error

// WriteCallback is called after a successful write operation.
// It receives the endpoint ID, feature ID, and the map of attribute IDs to values that were written.
type WriteCallback func(endpointID uint8, featureID uint8, attrs map[uint16]any)

// InvokeCallback is called after a successful invoke operation.
// It receives the endpoint ID, feature ID, command ID, parameters, and result.
type InvokeCallback func(endpointID uint8, featureID uint8, commandID uint8, params map[string]any, result any)

// ProtocolHandler handles MASH protocol messages for a device or controller.
// It routes Read/Write/Subscribe/Invoke requests to the appropriate
// features and generates responses. The handler is bidirectional and can be
// used on both device and controller sides.
type ProtocolHandler struct {
	mu sync.RWMutex

	device       *model.Device
	peerID       string         // The remote peer's identifier (generic, works for both device and controller)
	peerZoneType cert.ZoneType  // The remote peer's zone type (GRID, LOCAL, etc.)

	// Subscription management using SubscriptionManager
	subscriptions *SubscriptionManager

	// Send function for notifications
	sendNotification NotificationSender

	// Write callback (optional) - called after successful writes
	onWrite WriteCallback

	// Invoke callback (optional) - called after successful invokes
	onInvoke InvokeCallback

	// Protocol logging (optional)
	logger log.Logger
	connID string
}

// NewProtocolHandler creates a new protocol handler for a device.
func NewProtocolHandler(device *model.Device) *ProtocolHandler {
	return &ProtocolHandler{
		device:        device,
		subscriptions: NewSubscriptionManager(),
	}
}

// NewProtocolHandlerWithSend creates a new protocol handler with a custom notification sender.
func NewProtocolHandlerWithSend(device *model.Device, send NotificationSender) *ProtocolHandler {
	return &ProtocolHandler{
		device:           device,
		subscriptions:    NewSubscriptionManager(),
		sendNotification: send,
	}
}

// Device returns the underlying device model.
func (h *ProtocolHandler) Device() *model.Device {
	return h.device
}

// SubscriptionManager returns the subscription manager for this handler.
func (h *ProtocolHandler) SubscriptionManager() *SubscriptionManager {
	return h.subscriptions
}

// SetSendNotification sets the notification sender function.
func (h *ProtocolHandler) SetSendNotification(send NotificationSender) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sendNotification = send
}

// SetOnWrite sets the callback for write operations.
func (h *ProtocolHandler) SetOnWrite(cb WriteCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onWrite = cb
}

// SetOnInvoke sets the callback for invoke operations.
func (h *ProtocolHandler) SetOnInvoke(cb InvokeCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onInvoke = cb
}

// PeerID returns the remote peer's identifier.
func (h *ProtocolHandler) PeerID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peerID
}

// SetPeerID sets the remote peer's identifier.
func (h *ProtocolHandler) SetPeerID(peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peerID = peerID
}

// ZoneID returns the current zone context.
// Deprecated: Use PeerID instead for generic peer identification.
func (h *ProtocolHandler) ZoneID() string {
	return h.PeerID()
}

// SetZoneID sets the zone context for authorization.
// Deprecated: Use SetPeerID instead for generic peer identification.
func (h *ProtocolHandler) SetZoneID(zoneID string) {
	h.SetPeerID(zoneID)
}

// PeerZoneType returns the remote peer's zone type.
func (h *ProtocolHandler) PeerZoneType() cert.ZoneType {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.peerZoneType
}

// SetPeerZoneType sets the remote peer's zone type.
func (h *ProtocolHandler) SetPeerZoneType(zt cert.ZoneType) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.peerZoneType = zt
}

// SetLogger sets the protocol logger and connection ID.
// Events logged will include the connectionID for correlation.
func (h *ProtocolHandler) SetLogger(logger log.Logger, connectionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logger = logger
	h.connID = connectionID
}

// HandleRequest processes a protocol request and returns a response.
func (h *ProtocolHandler) HandleRequest(req *wire.Request) *wire.Response {
	startTime := time.Now()

	// Log the incoming request
	h.logRequest(req)

	var resp *wire.Response

	// Validate the operation
	if !req.Operation.IsValid() {
		resp = &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusUnsupported,
			Payload: &wire.ErrorPayload{
				Message: "unsupported operation",
			},
		}
	} else {
		// Route to appropriate handler
		switch req.Operation {
		case wire.OpRead:
			resp = h.handleRead(req)
		case wire.OpWrite:
			resp = h.handleWrite(req)
		case wire.OpSubscribe:
			resp = h.handleSubscribe(req)
		case wire.OpInvoke:
			resp = h.handleInvoke(req)
		default:
			resp = &wire.Response{
				MessageID: req.MessageID,
				Status:    wire.StatusUnsupported,
				Payload: &wire.ErrorPayload{
					Message: "unsupported operation",
				},
			}
		}
	}

	// Log the outgoing response with processing time
	processingTime := time.Since(startTime)
	h.logResponse(resp, processingTime)

	return resp
}

// logRequest logs an incoming request event.
func (h *ProtocolHandler) logRequest(req *wire.Request) {
	h.mu.RLock()
	logger := h.logger
	connID := h.connID
	h.mu.RUnlock()

	if logger == nil {
		return
	}

	op := req.Operation
	endpointID := req.EndpointID
	featureID := req.FeatureID

	logger.Log(log.Event{
		Timestamp:    time.Now(),
		ConnectionID: connID,
		Direction:    log.DirectionIn,
		Layer:        log.LayerWire,
		Category:     log.CategoryMessage,
		Message: &log.MessageEvent{
			Type:       log.MessageTypeRequest,
			MessageID:  req.MessageID,
			Operation:  &op,
			EndpointID: &endpointID,
			FeatureID:  &featureID,
			Payload:    req.Payload,
		},
	})
}

// logResponse logs an outgoing response event.
func (h *ProtocolHandler) logResponse(resp *wire.Response, processingTime time.Duration) {
	h.mu.RLock()
	logger := h.logger
	connID := h.connID
	h.mu.RUnlock()

	if logger == nil {
		return
	}

	status := resp.Status

	logger.Log(log.Event{
		Timestamp:    time.Now(),
		ConnectionID: connID,
		Direction:    log.DirectionOut,
		Layer:        log.LayerWire,
		Category:     log.CategoryMessage,
		Message: &log.MessageEvent{
			Type:           log.MessageTypeResponse,
			MessageID:      resp.MessageID,
			Status:         &status,
			Payload:        resp.Payload,
			ProcessingTime: &processingTime,
		},
	})
}

// handleRead processes a Read request.
func (h *ProtocolHandler) handleRead(req *wire.Request) *wire.Response {
	// Get the endpoint
	endpoint, err := h.device.GetEndpoint(req.EndpointID)
	if err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
			Payload: &wire.ErrorPayload{
				Message: "endpoint not found",
			},
		}
	}

	// Get the feature
	feature, ferr := endpoint.GetFeatureByID(req.FeatureID)
	if ferr != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidFeature,
			Payload: &wire.ErrorPayload{
				Message: "feature not found",
			},
		}
	}

	// Determine which attributes to read
	var attributeIDs []uint16
	if req.Payload != nil {
		if readPayload, ok := req.Payload.(*wire.ReadPayload); ok && len(readPayload.AttributeIDs) > 0 {
			attributeIDs = readPayload.AttributeIDs
		}
	}

	// Read attributes
	var result map[uint16]any
	if len(attributeIDs) == 0 {
		// Read all attributes
		result = feature.ReadAllAttributes()
	} else {
		// Read specific attributes
		result = make(map[uint16]any)
		for _, attrID := range attributeIDs {
			value, err := feature.ReadAttribute(attrID)
			if err == nil {
				result[attrID] = value
			}
			// Silently skip attributes that don't exist or aren't readable
		}
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
		Payload:   result,
	}
}

// handleWrite processes a Write request.
func (h *ProtocolHandler) handleWrite(req *wire.Request) *wire.Response {
	// Get the endpoint
	endpoint, err := h.device.GetEndpoint(req.EndpointID)
	if err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
			Payload: &wire.ErrorPayload{
				Message: "endpoint not found",
			},
		}
	}

	// Get the feature
	feature, ferr := endpoint.GetFeatureByID(req.FeatureID)
	if ferr != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidFeature,
			Payload: &wire.ErrorPayload{
				Message: "feature not found",
			},
		}
	}

	// Parse write payload
	writePayload, ok := req.Payload.(wire.WritePayload)
	if !ok {
		// Try map[uint16]any as well
		if mapPayload, ok := req.Payload.(map[uint16]any); ok {
			writePayload = wire.WritePayload(mapPayload)
		} else {
			return &wire.Response{
				MessageID: req.MessageID,
				Status:    wire.StatusInvalidParameter,
				Payload: &wire.ErrorPayload{
					Message: "invalid write payload",
				},
			}
		}
	}

	// Write attributes
	result := make(map[uint16]any)
	var firstError error
	var firstErrorStatus wire.Status

	for attrID, value := range writePayload {
		err := feature.WriteAttribute(attrID, value)
		if err != nil {
			if firstError == nil {
				firstError = err
				// Determine error type
				switch err {
				case model.ErrAttributeNotFound:
					firstErrorStatus = wire.StatusInvalidAttribute
				case model.ErrFeatureReadOnly:
					firstErrorStatus = wire.StatusReadOnly
				default:
					firstErrorStatus = wire.StatusConstraintError
				}
			}
		} else {
			// Read back the actual value (may have been constrained)
			if actual, err := feature.ReadAttribute(attrID); err == nil {
				result[attrID] = actual
			}
		}
	}

	if firstError != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    firstErrorStatus,
			Payload: &wire.ErrorPayload{
				Message: firstError.Error(),
			},
		}
	}

	// Call write callback if set and we have successful writes
	if len(result) > 0 {
		h.mu.RLock()
		onWrite := h.onWrite
		h.mu.RUnlock()
		if onWrite != nil {
			onWrite(req.EndpointID, req.FeatureID, result)
		}
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
		Payload:   result,
	}
}

// handleSubscribe processes a Subscribe or Unsubscribe request.
func (h *ProtocolHandler) handleSubscribe(req *wire.Request) *wire.Response {
	// Check for unsubscribe (featureID = 0)
	if req.FeatureID == 0 {
		return h.handleUnsubscribe(req)
	}

	// Get the endpoint
	endpoint, err := h.device.GetEndpoint(req.EndpointID)
	if err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
			Payload: &wire.ErrorPayload{
				Message: "endpoint not found",
			},
		}
	}

	// Get the feature
	feature, ferr := endpoint.GetFeatureByID(req.FeatureID)
	if ferr != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidFeature,
			Payload: &wire.ErrorPayload{
				Message: "feature not found",
			},
		}
	}

	// Parse subscribe payload
	var subPayload *wire.SubscribePayload
	if req.Payload != nil {
		if sp, ok := req.Payload.(*wire.SubscribePayload); ok {
			subPayload = sp
		}
	}

	// Extract attribute IDs from payload
	var attributeIDs []uint16
	if subPayload != nil {
		attributeIDs = subPayload.AttributeIDs
	}

	// Add subscription using SubscriptionManager (inbound = from remote peer to our features)
	subID := h.subscriptions.AddInbound(req.EndpointID, req.FeatureID, attributeIDs)

	// Read current values for priming report
	var currentValues map[uint16]any
	if len(attributeIDs) == 0 {
		currentValues = feature.ReadAllAttributes()
	} else {
		currentValues = make(map[uint16]any)
		for _, attrID := range attributeIDs {
			if value, err := feature.ReadAttribute(attrID); err == nil {
				currentValues[attrID] = value
			}
		}
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
		Payload: &wire.SubscribeResponsePayload{
			SubscriptionID: subID,
			CurrentValues:  currentValues,
		},
	}
}

// handleUnsubscribe processes an unsubscribe request.
func (h *ProtocolHandler) handleUnsubscribe(req *wire.Request) *wire.Response {
	// Parse unsubscribe payload
	var unsubPayload *wire.UnsubscribePayload
	if req.Payload != nil {
		if up, ok := req.Payload.(*wire.UnsubscribePayload); ok {
			unsubPayload = up
		}
	}

	if unsubPayload == nil || unsubPayload.SubscriptionID == 0 {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "subscription ID required",
			},
		}
	}

	// Remove subscription using SubscriptionManager
	exists := h.subscriptions.RemoveInbound(unsubPayload.SubscriptionID)

	if !exists {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "subscription not found",
			},
		}
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
	}
}

// handleInvoke processes an Invoke request.
func (h *ProtocolHandler) handleInvoke(req *wire.Request) *wire.Response {
	// Get the endpoint
	endpoint, err := h.device.GetEndpoint(req.EndpointID)
	if err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
			Payload: &wire.ErrorPayload{
				Message: "endpoint not found",
			},
		}
	}

	// Get the feature
	feature, ferr := endpoint.GetFeatureByID(req.FeatureID)
	if ferr != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidFeature,
			Payload: &wire.ErrorPayload{
				Message: "feature not found",
			},
		}
	}

	// Parse invoke payload - CBOR may decode as different types
	var commandID uint8
	var params map[string]any

	switch p := req.Payload.(type) {
	case *wire.InvokePayload:
		commandID = p.CommandID
		if p.Parameters != nil {
			switch pm := p.Parameters.(type) {
			case map[string]any:
				params = pm
			case map[any]any:
				params = convertMapAnyAnyToStringAny(pm)
			}
		}
	case wire.InvokePayload:
		commandID = p.CommandID
		if p.Parameters != nil {
			switch pm := p.Parameters.(type) {
			case map[string]any:
				params = pm
			case map[any]any:
				params = convertMapAnyAnyToStringAny(pm)
			}
		}
	case map[any]any:
		// CBOR decoded payload as generic map with integer keys
		// InvokePayload: {1: commandId, 2: parameters}
		if v, ok := p[uint64(1)].(uint64); ok {
			commandID = uint8(v)
		}
		if v, ok := p[uint64(2)].(map[any]any); ok {
			params = convertMapAnyAnyToStringAny(v)
		} else if v, ok := p[uint64(2)].(map[string]any); ok {
			params = v
		}
	default:
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "command payload required",
			},
		}
	}

	// Invoke the command with caller zone ID and type in context
	ctx := context.Background()
	if h.peerID != "" {
		ctx = ContextWithCallerZoneID(ctx, h.peerID)
	}
	if h.peerZoneType != 0 {
		ctx = ContextWithCallerZoneType(ctx, h.peerZoneType)
	}
	result, err := feature.InvokeCommand(ctx, commandID, params)
	if err != nil {
		status := wire.StatusInvalidCommand
		if err == model.ErrCommandNotFound {
			status = wire.StatusInvalidCommand
		}
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    status,
			Payload: &wire.ErrorPayload{
				Message: err.Error(),
			},
		}
	}

	// Call invoke callback if set
	h.mu.RLock()
	onInvoke := h.onInvoke
	h.mu.RUnlock()
	if onInvoke != nil {
		onInvoke(req.EndpointID, req.FeatureID, commandID, params, result)
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
		Payload:   result,
	}
}

// GetSubscription returns an inbound subscription by ID.
func (h *ProtocolHandler) GetSubscription(id uint32) (*Subscription, bool) {
	sub := h.subscriptions.GetInbound(id)
	return sub, sub != nil
}

// SubscriptionCount returns the number of active inbound subscriptions.
func (h *ProtocolHandler) SubscriptionCount() int {
	return h.subscriptions.InboundCount()
}

// GetMatchingSubscriptions returns subscription IDs that match the given endpoint, feature, and attribute.
// If attrID is 0, it matches subscriptions to any attribute of the feature.
func (h *ProtocolHandler) GetMatchingSubscriptions(endpointID uint8, featureID uint8, attrID uint16) []uint32 {
	matches := h.subscriptions.GetMatchingInbound(endpointID, featureID, attrID)
	result := make([]uint32, len(matches))
	for i, sub := range matches {
		result[i] = sub.ID
	}
	return result
}

// NotifyAttributeChange sends notifications to all inbound subscriptions that
// match the given endpoint, feature, and attribute.
func (h *ProtocolHandler) NotifyAttributeChange(endpointID, featureID uint8, attributeID uint16, value any) error {
	h.mu.RLock()
	sendFunc := h.sendNotification
	h.mu.RUnlock()

	// If no send function is configured, silently succeed
	if sendFunc == nil {
		return nil
	}

	// Find matching subscriptions
	matches := h.subscriptions.GetMatchingInbound(endpointID, featureID, attributeID)

	// Send notification to each matching subscription
	for _, sub := range matches {
		notification := &wire.Notification{
			SubscriptionID: sub.ID,
			EndpointID:     endpointID,
			FeatureID:      featureID,
			Changes:        map[uint16]any{attributeID: value},
		}
		if err := sendFunc(notification); err != nil {
			return err
		}
	}

	return nil
}

// convertMapAnyAnyToStringAny converts a map[any]any to map[string]any,
// keeping only entries with string keys. CBOR decoding sometimes produces
// map[any]any instead of map[string]any.
func convertMapAnyAnyToStringAny(m map[any]any) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		if key, ok := k.(string); ok {
			result[key] = v
		}
	}
	return result
}

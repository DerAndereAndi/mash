package service

import (
	"context"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// NotificationSender is a function that sends a notification to the remote peer.
type NotificationSender func(notification *wire.Notification) error

// ProtocolHandler handles MASH protocol messages for a device or controller.
// It routes Read/Write/Subscribe/Invoke requests to the appropriate
// features and generates responses. The handler is bidirectional and can be
// used on both device and controller sides.
type ProtocolHandler struct {
	mu sync.RWMutex

	device *model.Device
	peerID string // The remote peer's identifier (generic, works for both device and controller)

	// Subscription management using SubscriptionManager
	subscriptions *SubscriptionManager

	// Send function for notifications
	sendNotification NotificationSender
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

// HandleRequest processes a protocol request and returns a response.
func (h *ProtocolHandler) HandleRequest(req *wire.Request) *wire.Response {
	// Validate the operation
	if !req.Operation.IsValid() {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusUnsupported,
			Payload: &wire.ErrorPayload{
				Message: "unsupported operation",
			},
		}
	}

	// Route to appropriate handler
	switch req.Operation {
	case wire.OpRead:
		return h.handleRead(req)
	case wire.OpWrite:
		return h.handleWrite(req)
	case wire.OpSubscribe:
		return h.handleSubscribe(req)
	case wire.OpInvoke:
		return h.handleInvoke(req)
	default:
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusUnsupported,
			Payload: &wire.ErrorPayload{
				Message: "unsupported operation",
			},
		}
	}
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

	// Invoke the command with caller zone ID in context
	ctx := context.Background()
	if h.peerID != "" {
		ctx = ContextWithCallerZoneID(ctx, h.peerID)
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

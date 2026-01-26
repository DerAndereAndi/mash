package service

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ProtocolHandler handles MASH protocol messages for a device.
// It routes Read/Write/Subscribe/Invoke requests to the appropriate
// features and generates responses.
type ProtocolHandler struct {
	mu sync.RWMutex

	device *model.Device
	zoneID string

	// Subscription management
	nextSubscriptionID uint32
	subscriptions      map[uint32]*activeSubscription
}

// activeSubscription tracks an active subscription.
type activeSubscription struct {
	ID           uint32
	EndpointID   uint8
	FeatureID    uint8
	AttributeIDs []uint16 // Empty means all
	MinInterval  uint32   // Milliseconds
	MaxInterval  uint32   // Milliseconds
}

// NewProtocolHandler creates a new protocol handler for a device.
func NewProtocolHandler(device *model.Device) *ProtocolHandler {
	return &ProtocolHandler{
		device:        device,
		subscriptions: make(map[uint32]*activeSubscription),
	}
}

// Device returns the underlying device model.
func (h *ProtocolHandler) Device() *model.Device {
	return h.device
}

// ZoneID returns the current zone context.
func (h *ProtocolHandler) ZoneID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.zoneID
}

// SetZoneID sets the zone context for authorization.
func (h *ProtocolHandler) SetZoneID(zoneID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.zoneID = zoneID
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

	// Generate subscription ID
	subID := atomic.AddUint32(&h.nextSubscriptionID, 1)

	// Create subscription record
	sub := &activeSubscription{
		ID:         subID,
		EndpointID: req.EndpointID,
		FeatureID:  req.FeatureID,
	}
	if subPayload != nil {
		sub.AttributeIDs = subPayload.AttributeIDs
		sub.MinInterval = subPayload.MinInterval
		sub.MaxInterval = subPayload.MaxInterval
	}

	h.mu.Lock()
	h.subscriptions[subID] = sub
	h.mu.Unlock()

	// Read current values for priming report
	var currentValues map[uint16]any
	if len(sub.AttributeIDs) == 0 {
		currentValues = feature.ReadAllAttributes()
	} else {
		currentValues = make(map[uint16]any)
		for _, attrID := range sub.AttributeIDs {
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

	h.mu.Lock()
	_, exists := h.subscriptions[unsubPayload.SubscriptionID]
	if exists {
		delete(h.subscriptions, unsubPayload.SubscriptionID)
	}
	h.mu.Unlock()

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

	// Parse invoke payload
	var invokePayload *wire.InvokePayload
	if req.Payload != nil {
		if ip, ok := req.Payload.(*wire.InvokePayload); ok {
			invokePayload = ip
		}
	}

	if invokePayload == nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "command payload required",
			},
		}
	}

	// Convert parameters
	var params map[string]any
	if invokePayload.Parameters != nil {
		if p, ok := invokePayload.Parameters.(map[string]any); ok {
			params = p
		}
	}

	// Invoke the command
	ctx := context.Background()
	result, err := feature.InvokeCommand(ctx, invokePayload.CommandID, params)
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

// GetSubscription returns a subscription by ID.
func (h *ProtocolHandler) GetSubscription(id uint32) (*activeSubscription, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	sub, exists := h.subscriptions[id]
	return sub, exists
}

// SubscriptionCount returns the number of active subscriptions.
func (h *ProtocolHandler) SubscriptionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscriptions)
}

// GetMatchingSubscriptions returns subscription IDs that match the given endpoint, feature, and attribute.
// If attrID is 0, it matches subscriptions to any attribute of the feature.
func (h *ProtocolHandler) GetMatchingSubscriptions(endpointID uint8, featureID uint8, attrID uint16) []uint32 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var matches []uint32
	for id, sub := range h.subscriptions {
		if sub.EndpointID != endpointID || sub.FeatureID != featureID {
			continue
		}

		// Empty AttributeIDs means subscribed to all attributes
		if len(sub.AttributeIDs) == 0 {
			matches = append(matches, id)
			continue
		}

		// Check if specific attribute is in the subscription
		for _, subAttrID := range sub.AttributeIDs {
			if subAttrID == attrID {
				matches = append(matches, id)
				break
			}
		}
	}
	return matches
}

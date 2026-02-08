package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/subscription"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// ConnectionSender is a function that sends data to a connection.
type ConnectionSender func(data []byte) error

// NotificationDispatcher manages subscription notifications across multiple connections.
// It bridges the ProtocolHandler's subscription handling with the subscription.Manager's
// notification processing, routing notifications to the correct client connections.
type NotificationDispatcher struct {
	mu sync.RWMutex

	handler *ProtocolHandler
	manager *subscription.Manager

	// Connection tracking
	connections     map[uint64]*connectionInfo
	nextConnID      uint64
	subscriptionMap map[uint32]uint64 // subscriptionID -> connectionID

	// Background processing
	ctx       context.Context
	cancel    context.CancelFunc
	processWg sync.WaitGroup
	running   atomic.Bool
	interval  time.Duration

	// Protocol logging (optional)
	logger log.Logger
	connID string
}

// connectionInfo tracks a connection and its subscriptions.
type connectionInfo struct {
	id              uint64
	sender          ConnectionSender
	subscriptionIDs []uint32
}

// NewNotificationDispatcher creates a new notification dispatcher.
func NewNotificationDispatcher(handler *ProtocolHandler) *NotificationDispatcher {
	d := &NotificationDispatcher{
		handler:         handler,
		manager:         subscription.NewManager(),
		connections:     make(map[uint64]*connectionInfo),
		subscriptionMap: make(map[uint32]uint64),
		interval:        100 * time.Millisecond, // Default processing interval
	}

	// Set up notification callback
	d.manager.OnNotification(d.handleNotification)

	return d
}

// SetProcessingInterval sets the interval for background notification processing.
// Must be called before Start().
func (d *NotificationDispatcher) SetProcessingInterval(interval time.Duration) {
	d.interval = interval
}

// SetLogger sets the protocol logger and connection ID.
// Events logged will include the connectionID for correlation.
func (d *NotificationDispatcher) SetLogger(logger log.Logger, connectionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger = logger
	d.connID = connectionID
}

// Start begins background notification processing.
func (d *NotificationDispatcher) Start() {
	if d.running.Swap(true) {
		return // Already running
	}

	d.ctx, d.cancel = context.WithCancel(context.Background())
	d.processWg.Add(1)
	go d.processLoop()
}

// Stop stops background notification processing.
func (d *NotificationDispatcher) Stop() {
	if !d.running.Swap(false) {
		return // Not running
	}

	if d.cancel != nil {
		d.cancel()
	}
	d.processWg.Wait()
}

// RegisterConnection registers a new connection and returns its ID.
func (d *NotificationDispatcher) RegisterConnection(sender ConnectionSender) uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextConnID++
	connID := d.nextConnID

	d.connections[connID] = &connectionInfo{
		id:              connID,
		sender:          sender,
		subscriptionIDs: make([]uint32, 0),
	}

	return connID
}

// UnregisterConnection removes a connection and all its subscriptions.
func (d *NotificationDispatcher) UnregisterConnection(connID uint64) {
	d.mu.Lock()

	conn, exists := d.connections[connID]
	if !exists {
		d.mu.Unlock()
		return
	}

	// Copy subscription IDs to unsubscribe (avoiding modification during iteration)
	subIDs := make([]uint32, len(conn.subscriptionIDs))
	copy(subIDs, conn.subscriptionIDs)

	delete(d.connections, connID)

	// Remove subscription mappings
	for _, subID := range subIDs {
		delete(d.subscriptionMap, subID)
	}

	d.mu.Unlock()

	// Unsubscribe from manager (outside lock to avoid deadlock)
	for _, subID := range subIDs {
		d.manager.Unsubscribe(subID)
	}
}

// HandleSubscribe processes a subscribe request for a connection.
func (d *NotificationDispatcher) HandleSubscribe(connID uint64, req *wire.Request) *wire.Response {
	d.mu.Lock()
	_, exists := d.connections[connID]
	if !exists {
		d.mu.Unlock()
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "connection not registered",
			},
		}
	}
	d.mu.Unlock()

	// Get the endpoint and feature for reading current values
	endpoint, err := d.handler.Device().GetEndpoint(req.EndpointID)
	if err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
			Payload: &wire.ErrorPayload{
				Message: "endpoint not found",
			},
		}
	}

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

	// Parse subscribe payload (handles both typed and CBOR-decoded raw maps)
	subPayload := wire.ExtractSubscribePayload(req.Payload)

	// Extract intervals (default to sensible values)
	minInterval := time.Duration(1000) * time.Millisecond
	maxInterval := time.Duration(60000) * time.Millisecond
	var attributeIDs []uint16

	if subPayload != nil {
		if subPayload.MinInterval > 0 {
			minInterval = time.Duration(subPayload.MinInterval) * time.Millisecond
		}
		if subPayload.MaxInterval > 0 {
			maxInterval = time.Duration(subPayload.MaxInterval) * time.Millisecond
		}
		attributeIDs = subPayload.AttributeIDs
	}

	// Read current values for priming
	// TODO(task-4): Use ReadAllAttributesWithContext / ReadAttributeWithContext here once
	// the dispatcher tracks per-connection zone identity. Currently the dispatcher
	// only knows connections by numeric ID and does not have access to the peer's
	// zone ID or zone type needed to build the context.
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

	// Create subscription in manager
	subID, subErr := d.manager.Subscribe(
		uint16(req.EndpointID),
		uint16(req.FeatureID),
		attributeIDs,
		minInterval,
		maxInterval,
		currentValues,
	)
	if subErr != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusResourceExhausted,
			Payload: &wire.ErrorPayload{
				Message: subErr.Error(),
			},
		}
	}

	// Track subscription -> connection mapping
	d.mu.Lock()
	d.subscriptionMap[subID] = connID
	if conn, ok := d.connections[connID]; ok {
		conn.subscriptionIDs = append(conn.subscriptionIDs, subID)
	}
	d.mu.Unlock()

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
		Payload: &wire.SubscribeResponsePayload{
			SubscriptionID: subID,
			CurrentValues:  currentValues,
		},
	}
}

// HandleUnsubscribe processes an unsubscribe request for a connection.
func (d *NotificationDispatcher) HandleUnsubscribe(connID uint64, req *wire.Request) *wire.Response {
	// Parse unsubscribe payload. After CBOR roundtrip, the payload may be
	// a typed *UnsubscribePayload (from Go tests) or a map[any]any (from wire).
	var subID uint32
	if req.Payload != nil {
		switch p := req.Payload.(type) {
		case *wire.UnsubscribePayload:
			subID = p.SubscriptionID
		case map[any]any:
			if id, ok := wire.ToUint32(p[uint64(1)]); ok {
				subID = id
			}
		}
	}

	if subID == 0 {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "subscription ID required",
			},
		}
	}

	// Verify subscription belongs to this connection
	d.mu.Lock()
	ownerConnID, exists := d.subscriptionMap[subID]
	if !exists || ownerConnID != connID {
		d.mu.Unlock()
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: "subscription not found or not owned by connection",
			},
		}
	}

	// Remove from tracking
	delete(d.subscriptionMap, subID)
	if conn, ok := d.connections[connID]; ok {
		for i, id := range conn.subscriptionIDs {
			if id == subID {
				conn.subscriptionIDs = append(conn.subscriptionIDs[:i], conn.subscriptionIDs[i+1:]...)
				break
			}
		}
	}
	d.mu.Unlock()

	// Unsubscribe from manager
	if err := d.manager.Unsubscribe(subID); err != nil {
		return &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidParameter,
			Payload: &wire.ErrorPayload{
				Message: err.Error(),
			},
		}
	}

	return &wire.Response{
		MessageID: req.MessageID,
		Status:    wire.StatusSuccess,
	}
}

// NotifyChange records an attribute change that may trigger notifications.
func (d *NotificationDispatcher) NotifyChange(endpointID uint8, featureID, attrID uint16, value any) {
	d.manager.NotifyChange(uint16(endpointID), featureID, attrID, value)
}

// NotifyChanges records multiple attribute changes at once.
func (d *NotificationDispatcher) NotifyChanges(endpointID uint8, featureID uint16, changes map[uint16]any) {
	d.manager.NotifyChanges(uint16(endpointID), featureID, changes)
}

// SubscriptionCount returns the total number of active subscriptions.
func (d *NotificationDispatcher) SubscriptionCount() int {
	return d.manager.Count()
}

// handleNotification is called by the subscription manager when a notification is ready.
func (d *NotificationDispatcher) handleNotification(notif subscription.Notification) {
	d.mu.RLock()
	mappedConnID, exists := d.subscriptionMap[notif.SubscriptionID]
	if !exists {
		d.mu.RUnlock()
		return
	}

	conn, ok := d.connections[mappedConnID]
	if !ok {
		d.mu.RUnlock()
		return
	}

	sender := conn.sender
	logger := d.logger
	logConnID := d.connID
	d.mu.RUnlock()

	// Convert to wire notification
	wireNotif := &wire.Notification{
		SubscriptionID: notif.SubscriptionID,
		EndpointID:     uint8(notif.EndpointID),
		FeatureID:      uint8(notif.FeatureID),
		Changes:        notif.Attributes,
	}

	// Log the outgoing notification
	d.logNotification(logger, logConnID, wireNotif)

	// Encode and send
	data, err := wire.EncodeNotification(wireNotif)
	if err != nil {
		return // Log error in production
	}

	sender(data)
}

// logNotification logs an outgoing notification event.
func (d *NotificationDispatcher) logNotification(logger log.Logger, connectionID string, notif *wire.Notification) {
	if logger == nil {
		return
	}

	subscriptionID := notif.SubscriptionID
	endpointID := notif.EndpointID
	featureID := notif.FeatureID

	logger.Log(log.Event{
		Timestamp:    time.Now(),
		ConnectionID: connectionID,
		Direction:    log.DirectionOut,
		Layer:        log.LayerWire,
		Category:     log.CategoryMessage,
		Message: &log.MessageEvent{
			Type:           log.MessageTypeNotification,
			SubscriptionID: &subscriptionID,
			EndpointID:     &endpointID,
			FeatureID:      &featureID,
			Payload:        notif.Changes,
		},
	})
}

// processLoop runs the background notification processing.
func (d *NotificationDispatcher) processLoop() {
	defer d.processWg.Done()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.manager.ProcessNotifications()
		}
	}
}

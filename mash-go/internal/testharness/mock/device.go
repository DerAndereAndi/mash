// Package mock provides mock device and controller implementations for testing.
package mock

import (
	"context"
	"sync"
)

// Device represents a mock MASH device for testing.
type Device struct {
	// ID is the device identifier.
	ID string

	// Endpoints holds the device's endpoints.
	Endpoints map[uint8]*Endpoint

	// State tracks device state changes.
	State map[string]any

	// ReceivedMessages tracks messages received by the device.
	ReceivedMessages []Message

	// SendQueue holds messages to be sent.
	SendQueue []Message

	// Handlers are callbacks for specific operations.
	Handlers DeviceHandlers

	mu sync.RWMutex
}

// Endpoint represents a mock endpoint.
type Endpoint struct {
	// ID is the endpoint ID.
	ID uint8

	// Type is the endpoint type (e.g., "DEVICE_ROOT", "EV_CHARGER").
	Type string

	// Features holds the endpoint's features.
	Features map[string]*Feature
}

// Feature represents a mock feature.
type Feature struct {
	// Name is the feature name.
	Name string

	// Attributes holds the feature's attributes.
	Attributes map[string]any

	// FeatureMap is the feature capability bitmap.
	FeatureMap uint32
}

// Message represents a mock protocol message.
type Message struct {
	// Type is the message type (read, write, subscribe, invoke, response).
	Type string

	// Path is the target path (endpoint/feature/attribute).
	Path string

	// Payload is the message payload.
	Payload any

	// Error is an error code if applicable.
	Error uint8
}

// DeviceHandlers holds callbacks for device operations.
type DeviceHandlers struct {
	// OnRead is called when a read operation is received.
	OnRead func(path string) (any, error)

	// OnWrite is called when a write operation is received.
	OnWrite func(path string, value any) error

	// OnSubscribe is called when a subscribe operation is received.
	OnSubscribe func(path string) error

	// OnInvoke is called when an invoke operation is received.
	OnInvoke func(path string, params any) (any, error)
}

// NewDevice creates a new mock device.
func NewDevice(id string) *Device {
	return &Device{
		ID:               id,
		Endpoints:        make(map[uint8]*Endpoint),
		State:            make(map[string]any),
		ReceivedMessages: make([]Message, 0),
		SendQueue:        make([]Message, 0),
	}
}

// AddEndpoint adds an endpoint to the device.
func (d *Device) AddEndpoint(id uint8, epType string) *Endpoint {
	d.mu.Lock()
	defer d.mu.Unlock()

	ep := &Endpoint{
		ID:       id,
		Type:     epType,
		Features: make(map[string]*Feature),
	}
	d.Endpoints[id] = ep
	return ep
}

// AddFeature adds a feature to an endpoint.
func (d *Device) AddFeature(endpointID uint8, name string, featureMap uint32) *Feature {
	d.mu.Lock()
	defer d.mu.Unlock()

	ep, exists := d.Endpoints[endpointID]
	if !exists {
		return nil
	}

	f := &Feature{
		Name:       name,
		Attributes: make(map[string]any),
		FeatureMap: featureMap,
	}
	ep.Features[name] = f
	return f
}

// SetAttribute sets an attribute value.
func (d *Device) SetAttribute(endpointID uint8, feature, attribute string, value any) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ep, exists := d.Endpoints[endpointID]
	if !exists {
		return
	}

	f, exists := ep.Features[feature]
	if !exists {
		return
	}

	f.Attributes[attribute] = value
}

// GetAttribute gets an attribute value.
func (d *Device) GetAttribute(endpointID uint8, feature, attribute string) (any, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ep, exists := d.Endpoints[endpointID]
	if !exists {
		return nil, false
	}

	f, exists := ep.Features[feature]
	if !exists {
		return nil, false
	}

	v, exists := f.Attributes[attribute]
	return v, exists
}

// RecordMessage records a received message.
func (d *Device) RecordMessage(msg Message) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ReceivedMessages = append(d.ReceivedMessages, msg)
}

// QueueResponse queues a response message.
func (d *Device) QueueResponse(msg Message) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.SendQueue = append(d.SendQueue, msg)
}

// PopResponse removes and returns the next queued response.
func (d *Device) PopResponse() (Message, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.SendQueue) == 0 {
		return Message{}, false
	}

	msg := d.SendQueue[0]
	d.SendQueue = d.SendQueue[1:]
	return msg, true
}

// GetMessages returns all received messages.
func (d *Device) GetMessages() []Message {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]Message, len(d.ReceivedMessages))
	copy(result, d.ReceivedMessages)
	return result
}

// ClearMessages clears all received messages.
func (d *Device) ClearMessages() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ReceivedMessages = d.ReceivedMessages[:0]
}

// SetState sets a custom state value.
func (d *Device) SetState(key string, value any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.State[key] = value
}

// GetState gets a custom state value.
func (d *Device) GetState(key string) (any, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v, ok := d.State[key]
	return v, ok
}

// HandleRead processes a read request.
func (d *Device) HandleRead(ctx context.Context, path string) (any, error) {
	d.RecordMessage(Message{Type: "read", Path: path})

	if d.Handlers.OnRead != nil {
		return d.Handlers.OnRead(path)
	}

	// Default: return attribute value if path matches
	return nil, nil
}

// HandleWrite processes a write request.
func (d *Device) HandleWrite(ctx context.Context, path string, value any) error {
	d.RecordMessage(Message{Type: "write", Path: path, Payload: value})

	if d.Handlers.OnWrite != nil {
		return d.Handlers.OnWrite(path, value)
	}

	return nil
}

// HandleSubscribe processes a subscribe request.
func (d *Device) HandleSubscribe(ctx context.Context, path string) error {
	d.RecordMessage(Message{Type: "subscribe", Path: path})

	if d.Handlers.OnSubscribe != nil {
		return d.Handlers.OnSubscribe(path)
	}

	return nil
}

// HandleInvoke processes an invoke request.
func (d *Device) HandleInvoke(ctx context.Context, path string, params any) (any, error) {
	d.RecordMessage(Message{Type: "invoke", Path: path, Payload: params})

	if d.Handlers.OnInvoke != nil {
		return d.Handlers.OnInvoke(path, params)
	}

	return nil, nil
}

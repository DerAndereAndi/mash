package mock

import (
	"context"
	"sync"
)

// Controller represents a mock MASH controller for testing.
type Controller struct {
	// ID is the controller identifier.
	ID string

	// Zone is the controller's zone type.
	Zone string

	// Priority is the zone priority (1=highest).
	Priority int

	// ConnectedDevices tracks connected devices.
	ConnectedDevices map[string]*Device

	// Subscriptions tracks active subscriptions.
	Subscriptions map[string][]string // device -> paths

	// SentMessages tracks messages sent by the controller.
	SentMessages []Message

	// ReceivedNotifications tracks received notifications.
	ReceivedNotifications []Notification

	// Handlers are callbacks for controller operations.
	Handlers ControllerHandlers

	mu sync.RWMutex
}

// Notification represents a subscription notification.
type Notification struct {
	// DeviceID is the source device.
	DeviceID string

	// Path is the notification path.
	Path string

	// Value is the notification value.
	Value any

	// Sequence is the notification sequence number.
	Sequence uint32
}

// ControllerHandlers holds callbacks for controller operations.
type ControllerHandlers struct {
	// OnNotification is called when a notification is received.
	OnNotification func(deviceID, path string, value any)

	// OnDeviceConnected is called when a device connects.
	OnDeviceConnected func(deviceID string)

	// OnDeviceDisconnected is called when a device disconnects.
	OnDeviceDisconnected func(deviceID string)
}

// NewController creates a new mock controller.
func NewController(id, zone string, priority int) *Controller {
	return &Controller{
		ID:                    id,
		Zone:                  zone,
		Priority:              priority,
		ConnectedDevices:      make(map[string]*Device),
		Subscriptions:         make(map[string][]string),
		SentMessages:          make([]Message, 0),
		ReceivedNotifications: make([]Notification, 0),
	}
}

// ConnectDevice connects a device to the controller.
func (c *Controller) ConnectDevice(device *Device) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ConnectedDevices[device.ID] = device

	if c.Handlers.OnDeviceConnected != nil {
		c.Handlers.OnDeviceConnected(device.ID)
	}
}

// DisconnectDevice disconnects a device from the controller.
func (c *Controller) DisconnectDevice(deviceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.ConnectedDevices, deviceID)
	delete(c.Subscriptions, deviceID)

	if c.Handlers.OnDeviceDisconnected != nil {
		c.Handlers.OnDeviceDisconnected(deviceID)
	}
}

// GetDevice gets a connected device by ID.
func (c *Controller) GetDevice(deviceID string) *Device {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ConnectedDevices[deviceID]
}

// Read performs a read operation on a device.
func (c *Controller) Read(ctx context.Context, deviceID, path string) (any, error) {
	c.mu.Lock()
	c.SentMessages = append(c.SentMessages, Message{Type: "read", Path: path})
	device := c.ConnectedDevices[deviceID]
	c.mu.Unlock()

	if device == nil {
		return nil, ErrDeviceNotConnected
	}

	return device.HandleRead(ctx, path)
}

// Write performs a write operation on a device.
func (c *Controller) Write(ctx context.Context, deviceID, path string, value any) error {
	c.mu.Lock()
	c.SentMessages = append(c.SentMessages, Message{Type: "write", Path: path, Payload: value})
	device := c.ConnectedDevices[deviceID]
	c.mu.Unlock()

	if device == nil {
		return ErrDeviceNotConnected
	}

	return device.HandleWrite(ctx, path, value)
}

// Subscribe creates a subscription on a device.
func (c *Controller) Subscribe(ctx context.Context, deviceID, path string) error {
	c.mu.Lock()
	c.SentMessages = append(c.SentMessages, Message{Type: "subscribe", Path: path})
	device := c.ConnectedDevices[deviceID]
	if device != nil {
		c.Subscriptions[deviceID] = append(c.Subscriptions[deviceID], path)
	}
	c.mu.Unlock()

	if device == nil {
		return ErrDeviceNotConnected
	}

	return device.HandleSubscribe(ctx, path)
}

// Invoke performs an invoke operation on a device.
func (c *Controller) Invoke(ctx context.Context, deviceID, path string, params any) (any, error) {
	c.mu.Lock()
	c.SentMessages = append(c.SentMessages, Message{Type: "invoke", Path: path, Payload: params})
	device := c.ConnectedDevices[deviceID]
	c.mu.Unlock()

	if device == nil {
		return nil, ErrDeviceNotConnected
	}

	return device.HandleInvoke(ctx, path, params)
}

// ReceiveNotification processes an incoming notification.
func (c *Controller) ReceiveNotification(deviceID, path string, value any, sequence uint32) {
	c.mu.Lock()
	c.ReceivedNotifications = append(c.ReceivedNotifications, Notification{
		DeviceID: deviceID,
		Path:     path,
		Value:    value,
		Sequence: sequence,
	})
	c.mu.Unlock()

	if c.Handlers.OnNotification != nil {
		c.Handlers.OnNotification(deviceID, path, value)
	}
}

// GetNotifications returns all received notifications.
func (c *Controller) GetNotifications() []Notification {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Notification, len(c.ReceivedNotifications))
	copy(result, c.ReceivedNotifications)
	return result
}

// ClearNotifications clears all received notifications.
func (c *Controller) ClearNotifications() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ReceivedNotifications = c.ReceivedNotifications[:0]
}

// GetSentMessages returns all sent messages.
func (c *Controller) GetSentMessages() []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Message, len(c.SentMessages))
	copy(result, c.SentMessages)
	return result
}

// ClearSentMessages clears all sent messages.
func (c *Controller) ClearSentMessages() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SentMessages = c.SentMessages[:0]
}

// GetSubscriptions returns subscriptions for a device.
func (c *Controller) GetSubscriptions(deviceID string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	subs := c.Subscriptions[deviceID]
	result := make([]string, len(subs))
	copy(result, subs)
	return result
}

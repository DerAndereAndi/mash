package examples

import (
	"context"
	"errors"
	"sync"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// DeviceClient is an interface for interacting with a remote device.
// Both interaction.Client and service.DeviceSession implement this interface.
type DeviceClient interface {
	Read(ctx context.Context, endpointID uint8, featureID uint8, attrIDs []uint16) (map[uint16]any, error)
	Write(ctx context.Context, endpointID uint8, featureID uint8, attrs map[uint16]any) (map[uint16]any, error)
	Subscribe(ctx context.Context, endpointID uint8, featureID uint8, opts *interaction.SubscribeOptions) (uint32, map[uint16]any, error)
	Unsubscribe(ctx context.Context, subscriptionID uint32) error
	Invoke(ctx context.Context, endpointID uint8, featureID uint8, commandID uint8, params map[string]any) (any, error)
}

// CEM represents a Central Energy Manager that controls other devices.
// It demonstrates how to build a MASH controller that:
//   - Discovers and connects to controllable devices
//   - Establishes zone relationships
//   - Sets limits and setpoints on devices
//   - Monitors device state via subscriptions
type CEM struct {
	mu sync.RWMutex

	device *model.Device

	// Features
	deviceInfo *features.DeviceInfo

	// Connected devices (zone members)
	connectedDevices map[string]*ConnectedDevice

	// Zone configuration
	zoneType features.LimitCause
}

// ConnectedDevice represents a device the CEM is controlling.
type ConnectedDevice struct {
	DeviceID string
	Client   DeviceClient

	// Cached device info
	VendorName  string
	ProductName string
	DeviceType  features.DeviceType

	// Current state from subscriptions
	CurrentPower          int64
	EffectiveLimit        *int64
	ControlState          features.ControlState
	ChargingState         features.ChargingState
	EVStateOfCharge       *uint8
	EVTargetEnergyRequest *int64

	// Active subscriptions
	SubscriptionIDs []uint32
}

// CEMConfig contains configuration for creating a CEM.
type CEMConfig struct {
	DeviceID     string
	VendorName   string
	ProductName  string
	SerialNumber string
	VendorID     uint32
	ProductID    uint32
}

// NewCEM creates a new CEM device with the given configuration.
func NewCEM(cfg CEMConfig) *CEM {
	cem := &CEM{
		connectedDevices: make(map[string]*ConnectedDevice),
		zoneType:         features.LimitCauseLocalOptimization,
	}

	// Create device
	cem.device = model.NewDevice(cfg.DeviceID, cfg.VendorID, cfg.ProductID)

	// Setup root endpoint with DeviceInfo
	cem.setupDeviceInfo(cfg)

	return cem
}

func (c *CEM) setupDeviceInfo(cfg CEMConfig) {
	c.deviceInfo = features.NewDeviceInfo()
	c.deviceInfo.Feature.SetFeatureMap(uint32(model.FeatureMapCore)) // CEM is a MASH controller
	_ = c.deviceInfo.SetDeviceID(cfg.DeviceID)
	_ = c.deviceInfo.SetVendorName(cfg.VendorName)
	_ = c.deviceInfo.SetProductName(cfg.ProductName)
	_ = c.deviceInfo.SetSerialNumber(cfg.SerialNumber)
	_ = c.deviceInfo.SetVendorID(cfg.VendorID)
	_ = c.deviceInfo.SetProductID(cfg.ProductID)
	_ = c.deviceInfo.SetSoftwareVersion("1.0.0")

	c.device.RootEndpoint().AddFeature(c.deviceInfo.Feature)

	// Update endpoint info
	endpoints := make([]*model.EndpointInfo, 0)
	for _, ep := range c.device.Endpoints() {
		endpoints = append(endpoints, ep.Info())
	}
	_ = c.deviceInfo.SetEndpoints(endpoints)
}

// Device returns the underlying MASH device.
func (c *CEM) Device() *model.Device {
	return c.device
}

// SetZoneType sets the cause/priority for limits set by this CEM.
func (c *CEM) SetZoneType(zoneType features.LimitCause) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.zoneType = zoneType
}

// ConnectDevice establishes a zone relationship with a device.
// In a real implementation, this would happen after SHIP connection and pairing.
func (c *CEM) ConnectDevice(deviceID string, client DeviceClient) (*ConnectedDevice, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.connectedDevices[deviceID]; exists {
		return nil, errors.New("device already connected")
	}

	device := &ConnectedDevice{
		DeviceID:     deviceID,
		Client:       client,
		ControlState: features.ControlStateAutonomous,
	}

	c.connectedDevices[deviceID] = device
	return device, nil
}

// DisconnectDevice removes a device from the zone.
func (c *CEM) DisconnectDevice(deviceID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	device, exists := c.connectedDevices[deviceID]
	if !exists {
		return errors.New("device not connected")
	}

	// Unsubscribe from all active subscriptions
	ctx := context.Background()
	for _, subID := range device.SubscriptionIDs {
		_ = device.Client.Unsubscribe(ctx, subID)
	}

	delete(c.connectedDevices, deviceID)
	return nil
}

// GetConnectedDevice returns a connected device by ID.
func (c *CEM) GetConnectedDevice(deviceID string) (*ConnectedDevice, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	device, exists := c.connectedDevices[deviceID]
	return device, exists
}

// ConnectedDeviceIDs returns the IDs of all connected devices.
func (c *CEM) ConnectedDeviceIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.connectedDevices))
	for id := range c.connectedDevices {
		ids = append(ids, id)
	}
	return ids
}

// ReadDeviceInfo reads and caches device information from a connected device.
func (c *CEM) ReadDeviceInfo(ctx context.Context, deviceID string) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	// Read DeviceInfo from endpoint 0
	attrs, err := device.Client.Read(ctx, 0, uint8(model.FeatureDeviceInfo), nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := attrs[features.DeviceInfoAttrVendorName].(string); ok {
		device.VendorName = v
	}
	if v, ok := attrs[features.DeviceInfoAttrProductName].(string); ok {
		device.ProductName = v
	}

	return nil
}

// ReadEnergyControlState reads the current EnergyControl state from a device.
func (c *CEM) ReadEnergyControlState(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	attrs, err := device.Client.Read(ctx, endpointID, uint8(model.FeatureEnergyControl), nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if rawVal, exists := attrs[features.EnergyControlAttrDeviceType]; exists {
		if v, ok := wire.ToUint8Public(rawVal); ok {
			device.DeviceType = features.DeviceType(v)
		}
	}
	if rawVal, exists := attrs[features.EnergyControlAttrControlState]; exists {
		if v, ok := wire.ToUint8Public(rawVal); ok {
			device.ControlState = features.ControlState(v)
		}
	}
	if rawVal, exists := attrs[features.EnergyControlAttrEffectiveConsumptionLimit]; exists {
		if v, ok := wire.ToInt64(rawVal); ok {
			device.EffectiveLimit = &v
		}
	}

	return nil
}

// SubscribeToMeasurement subscribes to power measurements from a device.
func (c *CEM) SubscribeToMeasurement(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	subID, values, err := device.Client.Subscribe(ctx, endpointID, uint8(model.FeatureMeasurement), &interaction.SubscribeOptions{
		AttributeIDs: []uint16{features.MeasurementAttrACActivePower},
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	device.SubscriptionIDs = append(device.SubscriptionIDs, subID)

	// Process priming report using type-safe coercion
	if rawVal, exists := values[features.MeasurementAttrACActivePower]; exists {
		if v, ok := wire.ToInt64(rawVal); ok {
			device.CurrentPower = v
		}
	}
	c.mu.Unlock()

	return nil
}

// SubscribeToChargingSession subscribes to charging session updates.
func (c *CEM) SubscribeToChargingSession(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	subID, values, err := device.Client.Subscribe(ctx, endpointID, uint8(model.FeatureChargingSession), &interaction.SubscribeOptions{
		AttributeIDs: []uint16{
			features.ChargingSessionAttrState,
			features.ChargingSessionAttrEVStateOfCharge,
			features.ChargingSessionAttrEVTargetEnergyRequest,
		},
	})
	if err != nil {
		return err
	}

	c.mu.Lock()
	device.SubscriptionIDs = append(device.SubscriptionIDs, subID)

	// Process priming report using type-safe coercion
	if rawVal, exists := values[features.ChargingSessionAttrState]; exists {
		if v, ok := wire.ToUint8Public(rawVal); ok {
			device.ChargingState = features.ChargingState(v)
		}
	}
	if rawVal, exists := values[features.ChargingSessionAttrEVStateOfCharge]; exists {
		if v, ok := wire.ToUint8Public(rawVal); ok {
			device.EVStateOfCharge = &v
		}
	}
	if rawVal, exists := values[features.ChargingSessionAttrEVTargetEnergyRequest]; exists {
		if v, ok := wire.ToInt64(rawVal); ok {
			device.EVTargetEnergyRequest = &v
		}
	}
	c.mu.Unlock()

	return nil
}

// SetPowerLimit sets a consumption limit on a device.
func (c *CEM) SetPowerLimit(ctx context.Context, deviceID string, endpointID uint8, limitMW int64) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	zoneType := c.zoneType
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	result, err := device.Client.Invoke(ctx, endpointID, uint8(model.FeatureEnergyControl),
		features.EnergyControlCmdSetLimit, map[string]any{
			"consumptionLimit": limitMW,
			"cause":            uint8(zoneType),
		})
	if err != nil {
		return err
	}

	// Update cached state from response using type-safe coercion
	if resultMap, ok := result.(map[string]any); ok {
		c.mu.Lock()
		if rawVal, exists := resultMap["effectiveConsumptionLimit"]; exists {
			if v, ok := wire.ToInt64(rawVal); ok {
				device.EffectiveLimit = &v
			}
		}
		device.ControlState = features.ControlStateLimited
		c.mu.Unlock()
	}

	return nil
}

// ClearPowerLimit removes the consumption limit from a device.
func (c *CEM) ClearPowerLimit(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	_, err := device.Client.Invoke(ctx, endpointID, uint8(model.FeatureEnergyControl),
		features.EnergyControlCmdClearLimit, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	device.EffectiveLimit = nil
	device.ControlState = features.ControlStateControlled
	c.mu.Unlock()

	return nil
}

// PauseDevice pauses a device's operation.
func (c *CEM) PauseDevice(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	_, err := device.Client.Invoke(ctx, endpointID, uint8(model.FeatureEnergyControl),
		features.EnergyControlCmdPause, nil)
	return err
}

// ResumeDevice resumes a paused device.
func (c *CEM) ResumeDevice(ctx context.Context, deviceID string, endpointID uint8) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	_, err := device.Client.Invoke(ctx, endpointID, uint8(model.FeatureEnergyControl),
		features.EnergyControlCmdResume, nil)
	return err
}

// SetChargingMode sets the charging optimization mode on an EVSE.
func (c *CEM) SetChargingMode(ctx context.Context, deviceID string, endpointID uint8, mode features.ChargingMode, surplusThreshold *int64) error {
	c.mu.RLock()
	device, exists := c.connectedDevices[deviceID]
	c.mu.RUnlock()

	if !exists {
		return errors.New("device not connected")
	}

	params := map[string]any{
		"mode": uint8(mode),
	}
	if surplusThreshold != nil {
		params["surplusThreshold"] = *surplusThreshold
	}

	result, err := device.Client.Invoke(ctx, endpointID, uint8(model.FeatureChargingSession),
		features.ChargingSessionCmdSetChargingMode, params)
	if err != nil {
		return err
	}

	// Check if successful
	if resultMap, ok := result.(map[string]any); ok {
		if success, ok := resultMap["success"].(bool); ok && !success {
			if reason, ok := resultMap["reason"].(string); ok {
				return errors.New(reason)
			}
			return errors.New("charging mode not accepted")
		}
	}

	return nil
}

// HandleNotification processes incoming notifications from subscribed devices.
// Call this when a notification is received from a connected device.
// Uses wire.ToInt64/ToUint8Public for type-safe CBOR value extraction.
func (c *CEM) HandleNotification(deviceID string, endpointID uint8, featureID uint8, changes map[uint16]any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	device, exists := c.connectedDevices[deviceID]
	if !exists {
		return
	}

	switch model.FeatureType(featureID) {
	case model.FeatureMeasurement:
		if rawVal, exists := changes[features.MeasurementAttrACActivePower]; exists {
			if v, ok := wire.ToInt64(rawVal); ok {
				device.CurrentPower = v
			}
		}

	case model.FeatureEnergyControl:
		if rawVal, exists := changes[features.EnergyControlAttrControlState]; exists {
			if v, ok := wire.ToUint8Public(rawVal); ok {
				device.ControlState = features.ControlState(v)
			}
		}
		if rawVal, exists := changes[features.EnergyControlAttrEffectiveConsumptionLimit]; exists {
			if v, ok := wire.ToInt64(rawVal); ok {
				device.EffectiveLimit = &v
			}
		}

	case model.FeatureChargingSession:
		if rawVal, exists := changes[features.ChargingSessionAttrState]; exists {
			if v, ok := wire.ToUint8Public(rawVal); ok {
				device.ChargingState = features.ChargingState(v)
			}
		}
		if rawVal, exists := changes[features.ChargingSessionAttrEVStateOfCharge]; exists {
			if v, ok := wire.ToUint8Public(rawVal); ok {
				device.EVStateOfCharge = &v
			}
		}
		if rawVal, exists := changes[features.ChargingSessionAttrEVTargetEnergyRequest]; exists {
			if v, ok := wire.ToInt64(rawVal); ok {
				device.EVTargetEnergyRequest = &v
			}
		}
	}
}

// GetTotalPower returns the sum of power consumption across all connected devices.
func (c *CEM) GetTotalPower() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total int64
	for _, device := range c.connectedDevices {
		total += device.CurrentPower
	}
	return total
}

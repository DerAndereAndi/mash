package service

import (
	"crypto/tls"
	"errors"
	"log/slog"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
)

// Service errors.
var (
	ErrNotStarted       = errors.New("service not started")
	ErrAlreadyStarted   = errors.New("service already started")
	ErrCommissionFailed = errors.New("commissioning failed")
	ErrNotConnected     = errors.New("not connected")
	ErrDeviceNotFound   = errors.New("device not found")
	ErrZoneFull         = errors.New("maximum zones reached")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrSessionClosed    = errors.New("session closed")
)

// ServiceState represents the service state.
type ServiceState uint8

const (
	// StateIdle - service created but not started.
	StateIdle ServiceState = iota

	// StateStarting - service is starting up.
	StateStarting

	// StateRunning - service is running normally.
	StateRunning

	// StateStopping - service is shutting down.
	StateStopping

	// StateStopped - service has stopped.
	StateStopped
)

// String returns the state name.
func (s ServiceState) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateStarting:
		return "STARTING"
	case StateRunning:
		return "RUNNING"
	case StateStopping:
		return "STOPPING"
	case StateStopped:
		return "STOPPED"
	default:
		return "UNKNOWN"
	}
}

// DeviceConfig configures a DeviceService.
type DeviceConfig struct {
	// ListenAddress is the address to listen on (e.g., ":8443").
	ListenAddress string

	// TLSConfig provides TLS configuration for the server.
	// If nil, the service will generate a self-signed certificate.
	TLSConfig *tls.Config

	// Discriminator identifies this device for commissioning (0-4095).
	Discriminator uint16

	// SetupCode is the 8-digit setup code for SPAKE2+.
	SetupCode string

	// Categories lists device categories for mDNS discovery.
	Categories []discovery.DeviceCategory

	// SerialNumber is the device serial number.
	SerialNumber string

	// Brand is the vendor/brand name.
	Brand string

	// Model is the model name.
	Model string

	// DeviceName is an optional user-configurable name.
	DeviceName string

	// MaxZones is the maximum number of zones (default: 5).
	MaxZones int

	// FailsafeTimeout is the default failsafe duration.
	FailsafeTimeout time.Duration

	// ReconnectBackoff configures reconnection timing.
	ReconnectBackoff BackoffConfig

	// HeartbeatInterval is the keep-alive interval.
	HeartbeatInterval time.Duration

	// CommissioningWindowDuration is how long commissioning mode stays open.
	CommissioningWindowDuration time.Duration

	// EnableAutoReconnect enables automatic reconnection to zones.
	EnableAutoReconnect bool

	// Logger is the optional logger for debug output.
	// If nil, logging is disabled.
	Logger *slog.Logger
}

// ControllerConfig configures a ControllerService.
type ControllerConfig struct {
	// TLSConfig provides TLS configuration for the client.
	TLSConfig *tls.Config

	// ZoneType is the type of zone this controller creates.
	ZoneType cert.ZoneType

	// ZoneName is the user-friendly zone name.
	ZoneName string

	// VendorProduct is optional vendor:product ID.
	VendorProduct string

	// ControllerName is optional controller name.
	ControllerName string

	// DiscoveryTimeout is the timeout for mDNS discovery.
	DiscoveryTimeout time.Duration

	// ConnectionTimeout is the timeout for connecting to devices.
	ConnectionTimeout time.Duration

	// ReconnectBackoff configures reconnection timing.
	ReconnectBackoff BackoffConfig

	// HeartbeatInterval is the keep-alive interval.
	HeartbeatInterval time.Duration

	// SubscriptionMinInterval is the minimum subscription notification interval.
	SubscriptionMinInterval time.Duration

	// SubscriptionMaxInterval is the maximum subscription notification interval.
	SubscriptionMaxInterval time.Duration

	// EnableAutoReconnect enables automatic reconnection to devices.
	EnableAutoReconnect bool

	// EnableBounceBackSuppression enables subscription bounce-back suppression.
	EnableBounceBackSuppression bool

	// Logger is the optional logger for debug output.
	// If nil, logging is disabled.
	Logger *slog.Logger
}

// BackoffConfig configures exponential backoff for reconnection.
type BackoffConfig struct {
	// InitialInterval is the first retry delay.
	InitialInterval time.Duration

	// MaxInterval is the maximum retry delay.
	MaxInterval time.Duration

	// Multiplier is the backoff multiplier (e.g., 2.0 for doubling).
	Multiplier float64

	// MaxRetries is the maximum number of retries (0 = unlimited).
	MaxRetries int
}

// DefaultDeviceConfig returns a DeviceConfig with sensible defaults.
func DefaultDeviceConfig() DeviceConfig {
	return DeviceConfig{
		ListenAddress:               ":8443",
		MaxZones:                    5,
		FailsafeTimeout:             2 * time.Hour,
		HeartbeatInterval:           30 * time.Second,
		CommissioningWindowDuration: 120 * time.Second,
		EnableAutoReconnect:         true,
		ReconnectBackoff: BackoffConfig{
			InitialInterval: 1 * time.Second,
			MaxInterval:     5 * time.Minute,
			Multiplier:      2.0,
			MaxRetries:      0, // Unlimited
		},
	}
}

// DefaultControllerConfig returns a ControllerConfig with sensible defaults.
func DefaultControllerConfig() ControllerConfig {
	return ControllerConfig{
		ZoneType:                    cert.ZoneTypeHomeManager,
		DiscoveryTimeout:            10 * time.Second,
		ConnectionTimeout:           30 * time.Second,
		HeartbeatInterval:           30 * time.Second,
		SubscriptionMinInterval:     1 * time.Second,
		SubscriptionMaxInterval:     60 * time.Second,
		EnableAutoReconnect:         true,
		EnableBounceBackSuppression: true,
		ReconnectBackoff: BackoffConfig{
			InitialInterval: 1 * time.Second,
			MaxInterval:     5 * time.Minute,
			Multiplier:      2.0,
			MaxRetries:      0,
		},
	}
}

// Validate checks if the device config is valid.
func (c *DeviceConfig) Validate() error {
	if c.Discriminator > discovery.MaxDiscriminator {
		return ErrInvalidConfig
	}
	if len(c.SetupCode) != discovery.SetupCodeLength {
		return ErrInvalidConfig
	}
	if c.SerialNumber == "" || c.Brand == "" || c.Model == "" {
		return ErrInvalidConfig
	}
	if len(c.Categories) == 0 {
		return ErrInvalidConfig
	}
	return nil
}

// Validate checks if the controller config is valid.
func (c *ControllerConfig) Validate() error {
	if c.ZoneName == "" {
		return ErrInvalidConfig
	}
	if c.ZoneType < cert.ZoneTypeGridOperator || c.ZoneType > cert.ZoneTypeUserApp {
		return ErrInvalidConfig
	}
	return nil
}

// ConnectedDevice represents a device connected to a controller.
type ConnectedDevice struct {
	// ID is the device ID (fingerprint-derived).
	ID string

	// ZoneID is the zone ID this device belongs to.
	ZoneID string

	// Host is the device hostname.
	Host string

	// Port is the device port.
	Port uint16

	// Addresses contains resolved IP addresses.
	Addresses []string

	// VendorProduct is the vendor:product ID if available.
	VendorProduct string

	// Firmware is the firmware version if available.
	Firmware string

	// FeatureMap is the device's feature map.
	FeatureMap uint32

	// EndpointCount is the number of endpoints.
	EndpointCount uint8

	// Connected indicates if currently connected.
	Connected bool

	// LastSeen is when the device was last seen.
	LastSeen time.Time
}

// ConnectedZone represents a zone connected to a device.
type ConnectedZone struct {
	// ID is the zone ID (fingerprint-derived).
	ID string

	// Type is the zone type.
	Type cert.ZoneType

	// Priority is the zone's priority (derived from type).
	Priority uint8

	// Connected indicates if currently connected.
	Connected bool

	// LastSeen is when the zone was last seen.
	LastSeen time.Time

	// FailsafeActive indicates if failsafe is active for this zone.
	FailsafeActive bool
}

// Event types for service callbacks.
type EventType uint8

const (
	// EventConnected - connection established.
	EventConnected EventType = iota

	// EventDisconnected - connection lost.
	EventDisconnected

	// EventCommissioned - zone added.
	EventCommissioned

	// EventDecommissioned - zone removed.
	EventDecommissioned

	// EventValueChanged - attribute value changed.
	EventValueChanged

	// EventFailsafeStarted - failsafe timer started.
	EventFailsafeStarted

	// EventFailsafeTriggered - failsafe triggered (connection timeout).
	EventFailsafeTriggered

	// EventFailsafeCleared - failsafe cleared.
	EventFailsafeCleared

	// EventCommissioningOpened - commissioning window opened.
	EventCommissioningOpened

	// EventCommissioningClosed - commissioning window closed.
	EventCommissioningClosed

	// EventDeviceDiscovered - new device discovered via mDNS.
	EventDeviceDiscovered
)

// String returns the event type name.
func (e EventType) String() string {
	switch e {
	case EventConnected:
		return "CONNECTED"
	case EventDisconnected:
		return "DISCONNECTED"
	case EventCommissioned:
		return "COMMISSIONED"
	case EventDecommissioned:
		return "DECOMMISSIONED"
	case EventValueChanged:
		return "VALUE_CHANGED"
	case EventFailsafeStarted:
		return "FAILSAFE_STARTED"
	case EventFailsafeTriggered:
		return "FAILSAFE_TRIGGERED"
	case EventFailsafeCleared:
		return "FAILSAFE_CLEARED"
	case EventCommissioningOpened:
		return "COMMISSIONING_OPENED"
	case EventCommissioningClosed:
		return "COMMISSIONING_CLOSED"
	case EventDeviceDiscovered:
		return "DEVICE_DISCOVERED"
	default:
		return "UNKNOWN"
	}
}

// Event represents a service event.
type Event struct {
	// Type is the event type.
	Type EventType

	// ZoneID is the zone ID (for zone-related events).
	ZoneID string

	// DeviceID is the device ID (for device-related events).
	DeviceID string

	// EndpointID is the endpoint ID (for value change events).
	EndpointID uint8

	// FeatureID is the feature ID (for value change events).
	FeatureID uint16

	// AttributeID is the attribute ID (for value change events).
	AttributeID uint16

	// Value is the new value (for value change events).
	Value any

	// DiscoveredService contains the discovered service info (for discovery events).
	DiscoveredService any

	// Error is set if the event is an error.
	Error error
}

// EventHandler handles service events.
type EventHandler func(Event)

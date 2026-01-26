package service

import (
	"context"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/subscription"
)

// ControllerService orchestrates a MASH controller (EMS).
type ControllerService struct {
	mu sync.RWMutex

	config ControllerConfig
	state  ServiceState

	// Zone identity
	zoneID   string
	zoneName string

	// Discovery
	browser          discovery.Browser
	discoveryManager *discovery.DiscoveryManager
	advertiser       discovery.Advertiser

	// Connected devices
	connectedDevices map[string]*ConnectedDevice

	// Subscription management
	subscriptionManager *subscription.Manager

	// Event handlers
	eventHandlers []EventHandler

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewControllerService creates a new controller service.
func NewControllerService(config ControllerConfig) (*ControllerService, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	svc := &ControllerService{
		config:           config,
		state:            StateIdle,
		zoneName:         config.ZoneName,
		connectedDevices: make(map[string]*ConnectedDevice),
	}

	// Initialize subscription manager
	subConfig := subscription.DefaultConfig()
	subConfig.SuppressBounceBack = config.EnableBounceBackSuppression
	svc.subscriptionManager = subscription.NewManagerWithConfig(subConfig)

	return svc, nil
}

// State returns the current service state.
func (s *ControllerService) State() ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// ZoneID returns the controller's zone ID.
func (s *ControllerService) ZoneID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zoneID
}

// ZoneName returns the controller's zone name.
func (s *ControllerService) ZoneName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zoneName
}

// OnEvent registers an event handler.
func (s *ControllerService) OnEvent(handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandlers = append(s.eventHandlers, handler)
}

// Start starts the controller service.
func (s *ControllerService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateIdle && s.state != StateStopped {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}
	s.state = StateStarting
	s.mu.Unlock()

	// Create cancellable context
	s.ctx, s.cancel = context.WithCancel(ctx)

	// TODO: Generate/load zone certificate
	// TODO: Initialize zone ID from certificate fingerprint
	// TODO: Start advertising as commissioner

	s.mu.Lock()
	s.state = StateRunning
	s.mu.Unlock()

	return nil
}

// Stop stops the controller service.
func (s *ControllerService) Stop() error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return ErrNotStarted
	}
	s.state = StateStopping
	s.mu.Unlock()

	// Cancel context
	if s.cancel != nil {
		s.cancel()
	}

	// Stop browser
	if s.browser != nil {
		s.browser.Stop()
	}

	// Stop discovery advertising
	if s.discoveryManager != nil {
		s.discoveryManager.Stop()
	}

	// Clear subscriptions
	s.subscriptionManager.ClearAll()

	s.mu.Lock()
	s.state = StateStopped
	s.mu.Unlock()

	return nil
}

// Discover discovers commissionable devices.
func (s *ControllerService) Discover(ctx context.Context, filter discovery.FilterFunc) ([]*discovery.CommissionableService, error) {
	s.mu.RLock()
	if s.state != StateRunning {
		s.mu.RUnlock()
		return nil, ErrNotStarted
	}
	browser := s.browser
	s.mu.RUnlock()

	if browser == nil {
		return nil, ErrNotStarted
	}

	// Create timeout context
	timeout := s.config.DiscoveryTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start browsing
	results, err := browser.BrowseCommissionable(browseCtx)
	if err != nil {
		return nil, err
	}

	// Collect results
	var services []*discovery.CommissionableService
	for svc := range results {
		if filter == nil || filter(svc) {
			services = append(services, svc)
		}
	}

	return services, nil
}

// DiscoverByDiscriminator finds a specific device by discriminator.
func (s *ControllerService) DiscoverByDiscriminator(ctx context.Context, discriminator uint16) (*discovery.CommissionableService, error) {
	s.mu.RLock()
	if s.state != StateRunning {
		s.mu.RUnlock()
		return nil, ErrNotStarted
	}
	browser := s.browser
	s.mu.RUnlock()

	if browser == nil {
		return nil, ErrNotStarted
	}

	return browser.FindByDiscriminator(ctx, discriminator)
}

// Commission commissions a device using a setup code.
func (s *ControllerService) Commission(ctx context.Context, service *discovery.CommissionableService, setupCode string) (*ConnectedDevice, error) {
	s.mu.RLock()
	if s.state != StateRunning {
		s.mu.RUnlock()
		return nil, ErrNotStarted
	}
	s.mu.RUnlock()

	// TODO: Connect to device via TLS (InsecureSkipVerify initially)
	// TODO: Perform SPAKE2+ exchange with setupCode
	// TODO: Verify shared secret
	// TODO: Request device certificate
	// TODO: Generate zone certificate for device
	// TODO: Install operational certificate
	// TODO: Reconnect with mutual TLS

	// Placeholder - create device record
	device := &ConnectedDevice{
		ID:        "", // Would come from certificate fingerprint
		ZoneID:    s.zoneID,
		Host:      service.Host,
		Port:      service.Port,
		Addresses: service.Addresses,
		Connected: true,
		LastSeen:  time.Now(),
	}

	// Store device
	s.mu.Lock()
	s.connectedDevices[device.ID] = device
	s.mu.Unlock()

	// Emit event
	s.emitEvent(Event{
		Type:     EventCommissioned,
		DeviceID: device.ID,
	})

	return device, nil
}

// Decommission removes a device from the zone.
func (s *ControllerService) Decommission(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, exists := s.connectedDevices[deviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	// TODO: Send decommission command to device
	// TODO: Revoke device certificate

	delete(s.connectedDevices, deviceID)

	s.emitEvent(Event{
		Type:     EventDecommissioned,
		DeviceID: device.ID,
	})

	return nil
}

// DeviceCount returns the number of connected devices.
func (s *ControllerService) DeviceCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connectedDevices)
}

// GetDevice returns information about a connected device.
func (s *ControllerService) GetDevice(deviceID string) *ConnectedDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if d, exists := s.connectedDevices[deviceID]; exists {
		// Return a copy
		copy := *d
		return &copy
	}
	return nil
}

// GetAllDevices returns all connected devices.
func (s *ControllerService) GetAllDevices() []*ConnectedDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ConnectedDevice, 0, len(s.connectedDevices))
	for _, d := range s.connectedDevices {
		copy := *d
		result = append(result, &copy)
	}
	return result
}

// HandleDeviceConnect handles a device connection.
func (s *ControllerService) HandleDeviceConnect(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if device, exists := s.connectedDevices[deviceID]; exists {
		device.Connected = true
		device.LastSeen = time.Now()
	}

	s.emitEvent(Event{
		Type:     EventConnected,
		DeviceID: deviceID,
	})
}

// HandleDeviceDisconnect handles a device disconnection.
func (s *ControllerService) HandleDeviceDisconnect(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if device, exists := s.connectedDevices[deviceID]; exists {
		device.Connected = false
		device.LastSeen = time.Now()
	}

	s.emitEvent(Event{
		Type:     EventDisconnected,
		DeviceID: deviceID,
	})
}

// emitEvent sends an event to all registered handlers.
func (s *ControllerService) emitEvent(event Event) {
	for _, handler := range s.eventHandlers {
		go handler(event)
	}
}

// SetBrowser sets the discovery browser (for testing/DI).
func (s *ControllerService) SetBrowser(browser discovery.Browser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.browser = browser
}

// SetAdvertiser sets the discovery advertiser (for testing/DI).
func (s *ControllerService) SetAdvertiser(advertiser discovery.Advertiser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advertiser = advertiser
	s.discoveryManager = discovery.NewDiscoveryManager(advertiser)
}

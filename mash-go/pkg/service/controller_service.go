package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/persistence"
	"github.com/mash-protocol/mash-go/pkg/subscription"
	"github.com/mash-protocol/mash-go/pkg/transport"
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

	// Background discovery state (commissionable)
	discoveryActive bool
	discoveryCancel context.CancelFunc

	// Operational discovery state (for reconnection)
	operationalDiscoveryActive bool
	operationalDiscoveryCancel context.CancelFunc

	// Connected devices
	connectedDevices map[string]*ConnectedDevice

	// Device sessions for operational messaging
	deviceSessions map[string]*DeviceSession

	// Subscription management
	subscriptionManager *subscription.Manager

	// Event handlers
	eventHandlers []EventHandler

	// Persistence (optional, set by CLI)
	certStore  cert.ControllerStore
	stateStore *persistence.ControllerStateStore

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
		deviceSessions:   make(map[string]*DeviceSession),
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
	certStore := s.certStore
	config := s.config
	existingZoneID := s.zoneID
	s.mu.Unlock()

	// Create cancellable context
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Generate or load Zone CA if certStore is configured
	if certStore != nil {
		zoneCA, err := certStore.GetZoneCA()
		if err != nil && err != cert.ErrCertNotFound {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return fmt.Errorf("failed to load zone CA: %w", err)
		}

		if zoneCA == nil {
			// Generate new Zone CA
			// Use zone name as the zone ID for generation
			zoneID := config.ZoneName
			if existingZoneID != "" {
				zoneID = existingZoneID // Prefer existing zone ID from state
			}

			zoneCA, err = cert.GenerateZoneCA(zoneID, config.ZoneType)
			if err != nil {
				s.mu.Lock()
				s.state = StateIdle
				s.mu.Unlock()
				return fmt.Errorf("failed to generate zone CA: %w", err)
			}

			// Save the Zone CA
			if err := certStore.SetZoneCA(zoneCA); err != nil {
				s.mu.Lock()
				s.state = StateIdle
				s.mu.Unlock()
				return fmt.Errorf("failed to save zone CA: %w", err)
			}
			if err := certStore.Save(); err != nil {
				s.mu.Lock()
				s.state = StateIdle
				s.mu.Unlock()
				return fmt.Errorf("failed to persist zone CA: %w", err)
			}
		}

		// Use the Zone CA's zone ID if we don't have one from state
		if existingZoneID == "" {
			s.mu.Lock()
			s.zoneID = zoneCA.ZoneID
			s.mu.Unlock()
		}
	}

	// Initialize mDNS browser if not already set (e.g., by tests)
	if s.browser == nil {
		browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
		if err != nil {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}
		s.browser = browser
	}

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
	// Note: Browser handles aggregation/deduplication by instance name
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

	// Parse setup code
	code, err := strconv.ParseUint(setupCode, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid setup code", ErrCommissionFailed)
	}

	// Build address to connect to
	addr := fmt.Sprintf("%s:%d", service.Host, service.Port)
	if len(service.Addresses) > 0 {
		// Prefer IP address over hostname
		addr = fmt.Sprintf("%s:%d", service.Addresses[0], service.Port)
	}

	// Connect via TLS with InsecureSkipVerify (security from PASE)
	tlsConfig := transport.NewCommissioningTLSConfig()
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("%w: connection failed: %v", ErrCommissionFailed, err)
	}

	// Create client and server identities for PASE
	// These must match the identities used by the device's verifier
	clientIdentity := []byte("mash-controller")
	serverIdentity := []byte("mash-device")

	// Create PASE client session
	session, err := commissioning.NewPASEClientSession(
		commissioning.SetupCode(code),
		clientIdentity,
		serverIdentity,
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: failed to create PASE session: %v", ErrCommissionFailed, err)
	}

	// Perform PASE handshake
	sharedSecret, err := session.Handshake(ctx, conn)
	if err != nil {
		conn.Close()
		return nil, ErrCommissionFailed
	}

	// Derive IDs from shared secret
	deviceID := deriveDeviceID(sharedSecret)
	zoneID := deriveZoneID(sharedSecret)

	// Create framed connection wrapper for operational messaging
	framedConn := newFramedConnection(conn)

	// Create device session for operational messaging
	deviceSession := NewDeviceSession(deviceID, framedConn)

	// Create device record
	device := &ConnectedDevice{
		ID:        deviceID,
		ZoneID:    zoneID,
		Host:      service.Host,
		Port:      service.Port,
		Addresses: service.Addresses,
		Connected: true,
		LastSeen:  time.Now(),
	}

	// Store device and session
	s.mu.Lock()
	s.connectedDevices[device.ID] = device
	s.deviceSessions[device.ID] = deviceSession
	s.zoneID = zoneID // Store our zone ID
	s.mu.Unlock()

	// Start message loop in background to receive responses/notifications
	go s.runDeviceMessageLoop(deviceID, framedConn, deviceSession)

	// Emit event
	s.emitEvent(Event{
		Type:     EventCommissioned,
		DeviceID: device.ID,
	})

	return device, nil
}

// runDeviceMessageLoop reads messages from the device and dispatches to the session.
func (s *ControllerService) runDeviceMessageLoop(deviceID string, conn *framedConnection, session *DeviceSession) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		data, err := conn.ReadFrame()
		if err != nil {
			// Connection closed or error
			s.handleDeviceSessionClose(deviceID)
			return
		}

		// Dispatch to session
		session.OnMessage(data)
	}
}

// handleDeviceSessionClose cleans up when a device session closes.
func (s *ControllerService) handleDeviceSessionClose(deviceID string) {
	s.mu.Lock()
	if session, exists := s.deviceSessions[deviceID]; exists {
		session.Close()
		delete(s.deviceSessions, deviceID)
	}
	s.mu.Unlock()

	// Notify disconnect
	s.HandleDeviceDisconnect(deviceID)
}

// GetSession returns the session for a connected device.
// Returns nil if the device is not connected or has no session.
func (s *ControllerService) GetSession(deviceID string) *DeviceSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deviceSessions[deviceID]
}

// Decommission removes a device from the zone.
func (s *ControllerService) Decommission(deviceID string) error {
	s.mu.Lock()

	device, exists := s.connectedDevices[deviceID]
	if !exists {
		s.mu.Unlock()
		return ErrDeviceNotFound
	}

	// Close and remove the session if it exists
	if session, hasSession := s.deviceSessions[deviceID]; hasSession {
		session.Close()
		delete(s.deviceSessions, deviceID)
	}

	// TODO: Remove subscriptions for this device
	// TODO: Send decommission command to device
	// TODO: Revoke device certificate

	delete(s.connectedDevices, deviceID)
	s.mu.Unlock()

	// Save state to persist the removal
	_ = s.SaveState() // Ignore error - device is already removed from memory

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

// StartDiscovery starts background mDNS discovery.
// Discovered devices are emitted as EventDeviceDiscovered events.
// Use StopDiscovery to stop the background discovery.
func (s *ControllerService) StartDiscovery(ctx context.Context, filter discovery.FilterFunc) error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return ErrNotStarted
	}
	if s.discoveryActive {
		s.mu.Unlock()
		return nil // Already discovering
	}

	browser := s.browser
	if browser == nil {
		s.mu.Unlock()
		return ErrNotStarted
	}

	// Create cancellable context for this discovery session
	discoveryCtx, cancel := context.WithCancel(ctx)
	s.discoveryActive = true
	s.discoveryCancel = cancel
	s.mu.Unlock()

	// Start browsing in background
	go s.runDiscoveryLoop(discoveryCtx, filter)

	return nil
}

// runDiscoveryLoop runs the background discovery and emits events.
func (s *ControllerService) runDiscoveryLoop(ctx context.Context, filter discovery.FilterFunc) {
	results, err := s.browser.BrowseCommissionable(ctx)
	if err != nil {
		s.mu.Lock()
		s.discoveryActive = false
		s.discoveryCancel = nil
		s.mu.Unlock()
		return
	}

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.discoveryActive = false
			s.discoveryCancel = nil
			s.mu.Unlock()
			return

		case svc, ok := <-results:
			if !ok {
				s.mu.Lock()
				s.discoveryActive = false
				s.discoveryCancel = nil
				s.mu.Unlock()
				return
			}

			// Apply filter if provided
			if filter != nil && !filter(svc) {
				continue
			}

			// Emit discovery event
			s.emitEvent(Event{
				Type:              EventDeviceDiscovered,
				DiscoveredService: svc,
			})
		}
	}
}

// StopDiscovery stops background mDNS discovery.
func (s *ControllerService) StopDiscovery() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.discoveryCancel != nil {
		s.discoveryCancel()
		s.discoveryCancel = nil
	}
	s.discoveryActive = false
}

// IsDiscovering returns true if background discovery is active.
func (s *ControllerService) IsDiscovering() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.discoveryActive
}

// StartOperationalDiscovery starts background mDNS discovery for known devices.
// This browses for devices advertising on _mash._tcp with this controller's zone ID.
// Known devices that are found will trigger reconnection attempts.
func (s *ControllerService) StartOperationalDiscovery(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateRunning {
		s.mu.Unlock()
		return ErrNotStarted
	}
	if s.operationalDiscoveryActive {
		s.mu.Unlock()
		return nil // Already discovering
	}

	browser := s.browser
	zoneID := s.zoneID
	if browser == nil || zoneID == "" {
		s.mu.Unlock()
		return ErrNotStarted
	}

	// Create cancellable context for this discovery session
	discoveryCtx, cancel := context.WithCancel(ctx)
	s.operationalDiscoveryActive = true
	s.operationalDiscoveryCancel = cancel
	s.mu.Unlock()

	// Start browsing in background
	go s.runOperationalDiscoveryLoop(discoveryCtx, zoneID)

	return nil
}

// runOperationalDiscoveryLoop runs operational discovery and handles reconnection.
func (s *ControllerService) runOperationalDiscoveryLoop(ctx context.Context, zoneID string) {
	results, err := s.browser.BrowseOperational(ctx, zoneID)
	if err != nil {
		s.mu.Lock()
		s.operationalDiscoveryActive = false
		s.operationalDiscoveryCancel = nil
		s.mu.Unlock()
		return
	}

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.operationalDiscoveryActive = false
			s.operationalDiscoveryCancel = nil
			s.mu.Unlock()
			return

		case svc, ok := <-results:
			if !ok {
				s.mu.Lock()
				s.operationalDiscoveryActive = false
				s.operationalDiscoveryCancel = nil
				s.mu.Unlock()
				return
			}

			// Check if this is a known device
			s.mu.RLock()
			device, isKnown := s.connectedDevices[svc.DeviceID]
			alreadyConnected := isKnown && device.Connected
			s.mu.RUnlock()

			if !isKnown {
				continue // Not our device
			}

			if alreadyConnected {
				continue // Already connected
			}

			// Emit rediscovery event
			s.emitEvent(Event{
				Type:              EventDeviceRediscovered,
				DeviceID:          svc.DeviceID,
				DiscoveredService: svc,
			})

			// Attempt reconnection in background
			go s.attemptReconnection(ctx, svc)
		}
	}
}

// attemptReconnection tries to reconnect to a known device.
func (s *ControllerService) attemptReconnection(ctx context.Context, svc *discovery.OperationalService) {
	// Build the connection address
	addr := fmt.Sprintf("%s:%d", svc.Host, svc.Port)

	// Create TLS config for operational connection
	// Use the same pattern as commissioning for now
	// TODO: Implement proper certificate validation with zone CA
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true, // TODO: Validate against known device cert
		NextProtos:         []string{transport.ALPNProtocol},
	}

	// Attempt connection
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		// Connection failed - device might not be ready yet
		return
	}

	// Update device state
	s.mu.Lock()
	device, exists := s.connectedDevices[svc.DeviceID]
	if exists {
		device.Connected = true
		device.Host = svc.Host
		device.Port = svc.Port
		device.LastSeen = time.Now()
	}
	s.mu.Unlock()

	if !exists {
		conn.Close()
		return
	}

	// Create framed connection wrapper for operational messaging
	framedConn := newFramedConnection(conn)

	// Create device session
	session := NewDeviceSession(svc.DeviceID, framedConn)

	s.mu.Lock()
	s.deviceSessions[svc.DeviceID] = session
	s.mu.Unlock()

	// Emit reconnected event
	s.emitEvent(Event{
		Type:     EventDeviceReconnected,
		DeviceID: svc.DeviceID,
	})

	// Start message loop - blocks until connection closes
	s.runDeviceMessageLoop(svc.DeviceID, framedConn, session)

	// Clean up on disconnect
	s.handleDeviceSessionClose(svc.DeviceID)
}

// StopOperationalDiscovery stops background operational discovery.
func (s *ControllerService) StopOperationalDiscovery() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.operationalDiscoveryCancel != nil {
		s.operationalDiscoveryCancel()
		s.operationalDiscoveryCancel = nil
	}
	s.operationalDiscoveryActive = false
}

// IsOperationalDiscovering returns true if operational discovery is active.
func (s *ControllerService) IsOperationalDiscovering() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.operationalDiscoveryActive
}

// SetCertStore sets the certificate store for persistence.
func (s *ControllerService) SetCertStore(store cert.ControllerStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.certStore = store
}

// SetStateStore sets the state store for persistence.
func (s *ControllerService) SetStateStore(store *persistence.ControllerStateStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateStore = store
}

// SaveState persists the current controller state.
// This should be called on graceful shutdown and after commissioning changes.
func (s *ControllerService) SaveState() error {
	s.mu.RLock()
	store := s.stateStore
	if store == nil {
		s.mu.RUnlock()
		return nil // No store configured, no-op
	}

	state := &persistence.ControllerState{
		SavedAt: time.Now(),
		ZoneID:  s.zoneID,
	}

	// Save device memberships
	for _, device := range s.connectedDevices {
		dm := persistence.DeviceMembership{
			DeviceID:   device.ID,
			DeviceType: device.DeviceType,
			JoinedAt:   device.LastSeen, // Use LastSeen as proxy for JoinedAt
			LastSeenAt: device.LastSeen,
		}
		state.Devices = append(state.Devices, dm)
	}

	s.mu.RUnlock()

	return store.Save(state)
}

// LoadState restores the controller state from persistence.
// This should be called during Start() if a state store is configured.
func (s *ControllerService) LoadState() error {
	s.mu.Lock()
	store := s.stateStore
	s.mu.Unlock()

	if store == nil {
		return nil // No store configured, no-op
	}

	state, err := store.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return nil // No saved state
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Restore zone ID
	s.zoneID = state.ZoneID

	// Restore device list (as disconnected - they'll reconnect)
	for _, dm := range state.Devices {
		s.connectedDevices[dm.DeviceID] = &ConnectedDevice{
			ID:         dm.DeviceID,
			DeviceType: dm.DeviceType,
			Connected:  false, // Will be updated on reconnect
			LastSeen:   dm.LastSeenAt,
		}
	}

	return nil
}

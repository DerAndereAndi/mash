package service

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/duration"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/subscription"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/zone"
)

// DeviceService orchestrates a MASH device.
type DeviceService struct {
	mu sync.RWMutex

	config DeviceConfig
	device *model.Device
	state  ServiceState

	// Zone management
	zoneManager *zone.Manager

	// Discovery management
	discoveryManager *discovery.DiscoveryManager
	advertiser       discovery.Advertiser

	// TLS server for commissioning and operational connections
	tlsListener net.Listener
	tlsConfig   *tls.Config
	tlsCert     tls.Certificate

	// PASE commissioning
	verifier *commissioning.Verifier
	serverID []byte

	// Timer management - one failsafe timer per zone
	failsafeTimers  map[string]*failsafe.Timer
	durationManager *duration.Manager

	// Subscription management
	subscriptionManager *subscription.Manager

	// Connected zones
	connectedZones map[string]*ConnectedZone

	// Zone sessions for operational messaging
	zoneSessions map[string]*ZoneSession

	// Zone ID to index mapping (for duration timers which use uint8)
	zoneIndexMap  map[string]uint8
	nextZoneIndex uint8

	// Event handlers
	eventHandlers []EventHandler

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDeviceService creates a new device service.
func NewDeviceService(device *model.Device, config DeviceConfig) (*DeviceService, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	svc := &DeviceService{
		config:         config,
		device:         device,
		state:          StateIdle,
		zoneManager:    zone.NewManager(),
		connectedZones: make(map[string]*ConnectedZone),
		zoneSessions:   make(map[string]*ZoneSession),
		failsafeTimers: make(map[string]*failsafe.Timer),
		zoneIndexMap:   make(map[string]uint8),
	}

	// Initialize duration manager with expiry callback
	svc.durationManager = duration.NewManager()
	svc.durationManager.OnExpiry(func(zoneIndex uint8, cmdType duration.CommandType, value any) {
		svc.handleDurationExpiry(zoneIndex, cmdType, value)
	})

	// Initialize subscription manager
	subConfig := subscription.DefaultConfig()
	svc.subscriptionManager = subscription.NewManagerWithConfig(subConfig)

	return svc, nil
}

// Device returns the underlying device model.
func (s *DeviceService) Device() *model.Device {
	return s.device
}

// State returns the current service state.
func (s *DeviceService) State() ServiceState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// OnEvent registers an event handler.
func (s *DeviceService) OnEvent(handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandlers = append(s.eventHandlers, handler)
}

// Start starts the device service.
func (s *DeviceService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state != StateIdle && s.state != StateStopped {
		s.mu.Unlock()
		return ErrAlreadyStarted
	}
	s.state = StateStarting
	s.mu.Unlock()

	// Create cancellable context
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Generate server identity for PASE
	// Use a fixed identity for commissioning that both sides agree on
	s.serverID = []byte("mash-device")

	// Generate verifier from setup code
	setupCode, err := strconv.ParseUint(s.config.SetupCode, 10, 32)
	if err != nil {
		s.mu.Lock()
		s.state = StateIdle
		s.mu.Unlock()
		return err
	}

	// Client identity is generic for commissioning (controller will provide its own)
	// Both sides must use the same identities for PASE to work
	clientIdentity := []byte("mash-controller")
	s.verifier, err = commissioning.GenerateVerifier(
		commissioning.SetupCode(setupCode),
		clientIdentity,
		s.serverID,
	)
	if err != nil {
		s.mu.Lock()
		s.state = StateIdle
		s.mu.Unlock()
		return err
	}

	// Generate self-signed certificate for commissioning mode
	s.tlsCert, err = generateSelfSignedCert()
	if err != nil {
		s.mu.Lock()
		s.state = StateIdle
		s.mu.Unlock()
		return err
	}

	// Create TLS config for commissioning (no client cert required)
	s.tlsConfig = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{s.tlsCert},
		ClientAuth:   tls.NoClientCert, // During commissioning, no client cert
		NextProtos:   []string{transport.ALPNProtocol},
	}

	// Start TLS listener
	listener, err := net.Listen("tcp", s.config.ListenAddress)
	if err != nil {
		s.mu.Lock()
		s.state = StateIdle
		s.mu.Unlock()
		return err
	}
	s.tlsListener = listener

	// Start accepting connections in background
	go s.acceptLoop()

	// Initialize discovery advertiser if not already set (e.g., by tests)
	if s.advertiser == nil {
		advertiser, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
		if err != nil {
			s.tlsListener.Close()
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}
		s.advertiser = advertiser
		s.discoveryManager = discovery.NewDiscoveryManager(advertiser)

		// Parse port from actual listener address
		port := parsePort(s.tlsListener.Addr().String())

		s.discoveryManager.SetCommissionableInfo(&discovery.CommissionableInfo{
			Discriminator: s.config.Discriminator,
			Categories:    s.config.Categories,
			Serial:        s.config.SerialNumber,
			Brand:         s.config.Brand,
			Model:         s.config.Model,
			DeviceName:    s.config.DeviceName,
			Port:          port,
		})
	}

	s.mu.Lock()
	s.state = StateRunning
	s.mu.Unlock()

	return nil
}

// acceptLoop accepts incoming TLS connections.
func (s *DeviceService) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.tlsListener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles an incoming connection.
func (s *DeviceService) handleConnection(conn net.Conn) {
	// TLS handshake
	tlsConn := tls.Server(conn, s.tlsConfig)
	if err := tlsConn.HandshakeContext(s.ctx); err != nil {
		conn.Close()
		return
	}

	// Verify TLS version and ALPN
	state := tlsConn.ConnectionState()
	if err := transport.VerifyConnection(state); err != nil {
		tlsConn.Close()
		return
	}

	// Check if we're in commissioning mode
	s.mu.RLock()
	inCommissioningMode := s.discoveryManager != nil && s.discoveryManager.IsCommissioningMode()
	s.mu.RUnlock()

	if inCommissioningMode {
		s.handleCommissioningConnection(tlsConn)
	} else {
		// Operational mode - verify client certificate
		// TODO: Implement operational connection handling
		tlsConn.Close()
	}
}

// handleCommissioningConnection handles PASE commissioning over TLS.
func (s *DeviceService) handleCommissioningConnection(conn *tls.Conn) {
	// Create PASE server session
	paseSession, err := commissioning.NewPASEServerSession(s.verifier, s.serverID)
	if err != nil {
		conn.Close()
		return
	}

	// Perform PASE handshake
	sharedSecret, err := paseSession.Handshake(s.ctx, conn)
	if err != nil {
		// PASE failed - wrong setup code or protocol error
		conn.Close()
		return
	}

	// Derive zone ID from shared secret
	zoneID := deriveZoneID(sharedSecret)

	// Register the zone connection
	// For now, assume HomeManager type since we don't have certificate-based zone typing yet
	s.HandleZoneConnect(zoneID, cert.ZoneTypeHomeManager)

	// Create framed connection wrapper for operational messaging
	framedConn := newFramedConnection(conn)

	// Create zone session for this connection
	zoneSession := NewZoneSession(zoneID, framedConn, s.device)

	// Store the session
	s.mu.Lock()
	s.zoneSessions[zoneID] = zoneSession
	s.mu.Unlock()

	// Start message loop - blocks until connection closes
	s.runZoneMessageLoop(zoneID, framedConn, zoneSession)

	// Clean up on disconnect
	s.handleZoneSessionClose(zoneID)
}

// runZoneMessageLoop reads messages from the connection and dispatches to the session.
func (s *DeviceService) runZoneMessageLoop(zoneID string, conn *framedConnection, session *ZoneSession) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		data, err := conn.ReadFrame()
		if err != nil {
			// Connection closed or error
			return
		}

		// Dispatch to session
		session.OnMessage(data)
	}
}

// handleZoneSessionClose cleans up when a zone session closes.
func (s *DeviceService) handleZoneSessionClose(zoneID string) {
	s.mu.Lock()
	if session, exists := s.zoneSessions[zoneID]; exists {
		session.Close()
		delete(s.zoneSessions, zoneID)
	}
	s.mu.Unlock()

	// Notify disconnect
	s.HandleZoneDisconnect(zoneID)
}

// GetZoneSession returns the session for a connected zone.
func (s *DeviceService) GetZoneSession(zoneID string) *ZoneSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zoneSessions[zoneID]
}

// TLSAddr returns the TLS server's listen address.
func (s *DeviceService) TLSAddr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.tlsListener != nil {
		return s.tlsListener.Addr()
	}
	return nil
}

// ServerIdentity returns the server identity used for PASE.
func (s *DeviceService) ServerIdentity() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.serverID
}

// Stop stops the device service.
func (s *DeviceService) Stop() error {
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

	// Close TLS listener
	if s.tlsListener != nil {
		s.tlsListener.Close()
	}

	// Stop all failsafe timers
	s.mu.Lock()
	for _, timer := range s.failsafeTimers {
		timer.Reset()
	}
	s.mu.Unlock()

	// Clear subscriptions
	s.subscriptionManager.ClearAll()

	// Stop discovery advertising
	if s.discoveryManager != nil {
		s.discoveryManager.Stop()
	}

	s.mu.Lock()
	s.state = StateStopped
	s.mu.Unlock()

	return nil
}

// EnterCommissioningMode opens the commissioning window.
func (s *DeviceService) EnterCommissioningMode() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateRunning {
		return ErrNotStarted
	}

	if s.discoveryManager != nil {
		if err := s.discoveryManager.EnterCommissioningMode(s.ctx); err != nil {
			return err
		}
	}

	s.emitEvent(Event{Type: EventCommissioningOpened})
	return nil
}

// ExitCommissioningMode closes the commissioning window.
func (s *DeviceService) ExitCommissioningMode() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.discoveryManager != nil {
		if err := s.discoveryManager.ExitCommissioningMode(); err != nil {
			return err
		}
	}

	s.emitEvent(Event{Type: EventCommissioningClosed})
	return nil
}

// ZoneCount returns the number of connected zones.
func (s *DeviceService) ZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connectedZones)
}

// GetZone returns information about a connected zone.
func (s *DeviceService) GetZone(zoneID string) *ConnectedZone {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if z, exists := s.connectedZones[zoneID]; exists {
		// Return a copy
		copy := *z
		return &copy
	}
	return nil
}

// GetAllZones returns all connected zones.
func (s *DeviceService) GetAllZones() []*ConnectedZone {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ConnectedZone, 0, len(s.connectedZones))
	for _, z := range s.connectedZones {
		copy := *z
		result = append(result, &copy)
	}
	return result
}

// HandleZoneConnect handles a new zone connection.
func (s *DeviceService) HandleZoneConnect(zoneID string, zoneType cert.ZoneType) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create connected zone record
	cz := &ConnectedZone{
		ID:        zoneID,
		Type:      zoneType,
		Priority:  zoneType.Priority(),
		Connected: true,
		LastSeen:  time.Now(),
	}
	s.connectedZones[zoneID] = cz

	// Assign zone index if not already assigned
	if _, exists := s.zoneIndexMap[zoneID]; !exists {
		s.zoneIndexMap[zoneID] = s.nextZoneIndex
		s.nextZoneIndex++
	}

	// Create failsafe timer for this zone
	timer := failsafe.NewTimer()
	if err := timer.SetDuration(s.config.FailsafeTimeout); err == nil {
		timer.OnFailsafeEnter(func(_ failsafe.Limits) {
			s.handleFailsafe(zoneID)
		})
		timer.Start()
		s.failsafeTimers[zoneID] = timer
	}

	s.emitEvent(Event{
		Type:   EventConnected,
		ZoneID: zoneID,
	})
}

// HandleZoneDisconnect handles a zone disconnection.
func (s *DeviceService) HandleZoneDisconnect(zoneID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cz, exists := s.connectedZones[zoneID]; exists {
		cz.Connected = false
		cz.LastSeen = time.Now()
	}

	// The failsafe timer was already started on connect
	// It will trigger if no reconnect happens

	s.emitEvent(Event{
		Type:   EventDisconnected,
		ZoneID: zoneID,
	})
}

// handleFailsafe handles a failsafe timer trigger.
func (s *DeviceService) handleFailsafe(zoneID string) {
	s.mu.Lock()

	if cz, exists := s.connectedZones[zoneID]; exists {
		cz.FailsafeActive = true
	}

	// Get zone index for duration timer cancellation
	zoneIndex := s.zoneIndexMap[zoneID]

	s.mu.Unlock()

	// Cancel duration timers for this zone
	s.durationManager.CancelZoneTimers(zoneIndex)

	s.emitEvent(Event{
		Type:   EventFailsafeTriggered,
		ZoneID: zoneID,
	})
}

// handleDurationExpiry handles a duration timer expiry.
func (s *DeviceService) handleDurationExpiry(zoneIndex uint8, _ duration.CommandType, _ any) {
	s.mu.RLock()

	// Find zone by index
	var zoneID string
	for id, idx := range s.zoneIndexMap {
		if idx == zoneIndex {
			zoneID = id
			break
		}
	}

	s.mu.RUnlock()

	// Clear the value in the device model
	// This would trigger recalculation of effective values
	// TODO: Implement value clearing based on cmdType

	s.emitEvent(Event{
		Type:   EventValueChanged,
		ZoneID: zoneID,
		Value:  nil, // Cleared
	})
}

// RefreshFailsafe refreshes the failsafe timer for a zone.
func (s *DeviceService) RefreshFailsafe(zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cz, exists := s.connectedZones[zoneID]
	if !exists {
		return ErrDeviceNotFound
	}

	timer, hasTimer := s.failsafeTimers[zoneID]
	if !hasTimer {
		return ErrDeviceNotFound
	}

	if cz.FailsafeActive {
		// Clear failsafe state
		cz.FailsafeActive = false
		timer.Stop() // This will enter grace period

		s.emitEvent(Event{
			Type:   EventFailsafeCleared,
			ZoneID: zoneID,
		})
	}

	// Restart the timer
	timer.Reset()
	timer.Start()

	return nil
}

// SetDurationTimer sets a duration timer for a command.
func (s *DeviceService) SetDurationTimer(zoneID string, cmdType duration.CommandType, dur time.Duration, value any) error {
	s.mu.RLock()
	_, exists := s.connectedZones[zoneID]
	zoneIndex, hasIndex := s.zoneIndexMap[zoneID]
	s.mu.RUnlock()

	if !exists || !hasIndex {
		return ErrDeviceNotFound
	}

	return s.durationManager.SetTimer(zoneIndex, cmdType, dur, value)
}

// CancelDurationTimer cancels a duration timer.
func (s *DeviceService) CancelDurationTimer(zoneID string, cmdType duration.CommandType) error {
	s.mu.RLock()
	zoneIndex, exists := s.zoneIndexMap[zoneID]
	s.mu.RUnlock()

	if !exists {
		return ErrDeviceNotFound
	}

	return s.durationManager.CancelTimer(zoneIndex, cmdType)
}

// emitEvent sends an event to all registered handlers.
func (s *DeviceService) emitEvent(event Event) {
	for _, handler := range s.eventHandlers {
		go handler(event)
	}
}

// SetAdvertiser sets the discovery advertiser (for testing/DI).
func (s *DeviceService) SetAdvertiser(advertiser discovery.Advertiser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.advertiser = advertiser
	s.discoveryManager = discovery.NewDiscoveryManager(advertiser)
	s.discoveryManager.SetCommissionableInfo(&discovery.CommissionableInfo{
		Discriminator: s.config.Discriminator,
		Categories:    s.config.Categories,
		Serial:        s.config.SerialNumber,
		Brand:         s.config.Brand,
		Model:         s.config.Model,
		DeviceName:    s.config.DeviceName,
		Port:          8443,
	})
}

// SetFailsafeTimer sets a custom failsafe timer for a zone (for testing/DI).
// This allows injecting test timers with short durations.
func (s *DeviceService) SetFailsafeTimer(zoneID string, timer *failsafe.Timer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop existing timer if any
	if existing, ok := s.failsafeTimers[zoneID]; ok {
		existing.Reset()
	}

	// Set up callback
	timer.OnFailsafeEnter(func(_ failsafe.Limits) {
		s.handleFailsafe(zoneID)
	})

	s.failsafeTimers[zoneID] = timer
}

// GetFailsafeTimer returns the failsafe timer for a zone (for testing).
func (s *DeviceService) GetFailsafeTimer(zoneID string) *failsafe.Timer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failsafeTimers[zoneID]
}

// parsePort extracts the port from a listen address (e.g., ":8443" -> 8443).
func parsePort(addr string) uint16 {
	// Handle formats: ":8443", "0.0.0.0:8443", "localhost:8443"
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			var port uint16
			for j := i + 1; j < len(addr); j++ {
				port = port*10 + uint16(addr[j]-'0')
			}
			return port
		}
	}
	return 8443 // Default port
}

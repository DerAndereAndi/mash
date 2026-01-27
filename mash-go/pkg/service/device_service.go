package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
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
	"github.com/mash-protocol/mash-go/pkg/persistence"
	"github.com/mash-protocol/mash-go/pkg/subscription"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
	"github.com/mash-protocol/mash-go/pkg/zone"
)

// DeviceService orchestrates a MASH device.
type DeviceService struct {
	mu sync.RWMutex

	config DeviceConfig
	device *model.Device
	state  ServiceState

	// Device identity (derived from certificate fingerprint)
	deviceID string

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

	// Logger for debug output (optional)
	logger *slog.Logger

	// Persistence (optional, set by CLI)
	certStore  cert.Store
	stateStore *persistence.DeviceStateStore

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
		logger:         config.Logger,
	}

	// Initialize duration manager with expiry callback
	svc.durationManager = duration.NewManager()
	svc.durationManager.OnExpiry(func(zoneIndex uint8, cmdType duration.CommandType, value any) {
		svc.handleDurationExpiry(zoneIndex, cmdType, value)
	})

	// Initialize subscription manager
	subConfig := subscription.DefaultConfig()
	svc.subscriptionManager = subscription.NewManagerWithConfig(subConfig)

	// Register service-level commands on DeviceInfo feature
	svc.registerDeviceCommands()

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

	// Get or generate the device identity certificate
	// This certificate is used for TLS and defines the device ID
	s.mu.RLock()
	certStore := s.certStore
	s.mu.RUnlock()

	var deviceCert *x509.Certificate
	var deviceKey *ecdsa.PrivateKey

	if certStore != nil {
		// Try to load existing certificate
		deviceCert, deviceKey, err = certStore.GetDeviceIdentity()
		if err != nil && err != cert.ErrCertNotFound {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return fmt.Errorf("failed to load device certificate: %w", err)
		}
	}

	if deviceCert == nil || deviceKey == nil {
		// Generate new self-signed certificate
		s.tlsCert, err = generateSelfSignedCert()
		if err != nil {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}

		// Save to cert store if available
		if certStore != nil && len(s.tlsCert.Certificate) > 0 {
			parsedCert, parseErr := x509.ParseCertificate(s.tlsCert.Certificate[0])
			if parseErr == nil {
				if key, ok := s.tlsCert.PrivateKey.(*ecdsa.PrivateKey); ok {
					_ = certStore.SetDeviceIdentity(parsedCert, key)
					_ = certStore.Save()
				}
			}
		}
	} else {
		// Use loaded certificate
		s.tlsCert = tls.Certificate{
			Certificate: [][]byte{deviceCert.Raw},
			PrivateKey:  deviceKey,
		}
	}

	// Derive device ID from the certificate's public key
	if len(s.tlsCert.Certificate) > 0 {
		parsedCert, err := x509.ParseCertificate(s.tlsCert.Certificate[0])
		if err == nil {
			s.deviceID, _ = discovery.DeviceIDFromPublicKey(parsedCert)
		}
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
		// Operational mode - handle reconnection from known zones
		s.handleOperationalConnection(tlsConn)
	}
}

// handleOperationalConnection handles a reconnection from a known zone.
func (s *DeviceService) handleOperationalConnection(conn *tls.Conn) {
	s.mu.RLock()
	// Find a known zone that isn't currently connected
	var targetZoneID string
	for zoneID, cz := range s.connectedZones {
		if !cz.Connected {
			targetZoneID = zoneID
			break
		}
	}
	s.mu.RUnlock()

	if targetZoneID == "" {
		// No known disconnected zones - reject connection
		s.debugLog("handleOperationalConnection: no disconnected zones to reconnect")
		conn.Close()
		return
	}

	// Mark zone as connected
	s.mu.Lock()
	if cz, exists := s.connectedZones[targetZoneID]; exists {
		cz.Connected = true
		cz.LastSeen = time.Now()
	}

	// Restart failsafe timer for this zone
	if timer, hasTimer := s.failsafeTimers[targetZoneID]; hasTimer {
		timer.Reset()
		timer.Start()
	}
	s.mu.Unlock()

	s.debugLog("handleOperationalConnection: zone reconnected", "zoneID", targetZoneID)

	// Create framed connection wrapper for operational messaging
	framedConn := newFramedConnection(conn)

	// Create zone session for this connection
	zoneSession := NewZoneSession(targetZoneID, framedConn, s.device)
	zoneSession.SetLogger(s.logger)

	// Initialize renewal handler for certificate renewal support
	zoneSession.InitializeRenewalHandler(s.buildDeviceIdentity())

	// Store the session
	s.mu.Lock()
	s.zoneSessions[targetZoneID] = zoneSession
	s.mu.Unlock()

	// Emit connected event
	s.emitEvent(Event{
		Type:   EventConnected,
		ZoneID: targetZoneID,
	})

	// Start message loop - blocks until connection closes
	s.runZoneMessageLoop(targetZoneID, framedConn, zoneSession)

	// Clean up on disconnect
	s.handleZoneSessionClose(targetZoneID)
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
	// For now, assume Local type since we don't have certificate-based zone typing yet
	s.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Persist state immediately after commissioning
	_ = s.SaveState()

	// Exit commissioning mode after successful commission
	// The device should no longer advertise as commissionable
	if err := s.ExitCommissioningMode(); err != nil {
		s.debugLog("handleCommissioningConnection: failed to exit commissioning mode", "error", err)
	}

	// Create framed connection wrapper for operational messaging
	framedConn := newFramedConnection(conn)

	// Create zone session for this connection
	zoneSession := NewZoneSession(zoneID, framedConn, s.device)
	zoneSession.SetLogger(s.logger)

	// Initialize renewal handler for certificate renewal support
	zoneSession.InitializeRenewalHandler(s.buildDeviceIdentity())

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

	// Start operational mDNS advertising for this zone
	// This allows controllers to discover the device for reconnection
	if s.discoveryManager != nil {
		port := uint16(0)
		if s.tlsListener != nil {
			port = parsePort(s.tlsListener.Addr().String())
		}

		opInfo := &discovery.OperationalInfo{
			ZoneID:        zoneID,
			DeviceID:      s.deviceID,
			VendorProduct: fmt.Sprintf("%04x:%04x", s.device.VendorID(), s.device.ProductID()),
			EndpointCount: uint8(s.device.EndpointCount()),
			Port:          port,
		}

		// AddZone is async-safe and will start advertising
		if err := s.discoveryManager.AddZone(s.ctx, opInfo); err != nil {
			s.debugLog("HandleZoneConnect: failed to start operational advertising",
				"zoneID", zoneID, "error", err)
		}
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

// debugLog logs a debug message if logging is enabled.
func (s *DeviceService) debugLog(msg string, args ...any) {
	if s.logger != nil {
		s.logger.Debug(msg, args...)
	}
}

// NotifyAttributeChange updates an attribute and sends notifications to subscribed zones.
// This should be called when device-side logic (e.g., simulation) changes a value.
func (s *DeviceService) NotifyAttributeChange(endpointID uint8, featureID uint8, attrID uint16, value any) error {
	s.debugLog("NotifyAttributeChange called",
		"endpointID", endpointID,
		"featureID", featureID,
		"attrID", attrID,
		"valueType", slog.AnyValue(value).Kind().String(),
		"value", value)

	// Update the device model
	endpoint, err := s.device.GetEndpoint(endpointID)
	if err != nil {
		s.debugLog("NotifyAttributeChange: endpoint not found", "endpointID", endpointID, "error", err)
		return err
	}

	feature, err := endpoint.GetFeatureByID(featureID)
	if err != nil {
		s.debugLog("NotifyAttributeChange: feature not found", "featureID", featureID, "error", err)
		return err
	}

	// Use SetAttributeInternal to bypass access checks for device-side updates
	// (measurement attributes are read-only for controllers but writable internally)
	if err := feature.SetAttributeInternal(attrID, value); err != nil {
		s.debugLog("NotifyAttributeChange: failed to set attribute", "attrID", attrID, "error", err)
		return err
	}

	// Send notifications to all subscribed zones
	s.mu.RLock()
	sessions := make([]*ZoneSession, 0, len(s.zoneSessions))
	for _, session := range s.zoneSessions {
		sessions = append(sessions, session)
	}
	s.mu.RUnlock()

	s.debugLog("NotifyAttributeChange: checking zone sessions", "sessionCount", len(sessions))

	changes := map[uint16]any{attrID: value}
	notificationsSent := 0

	for _, session := range sessions {
		// Get matching subscriptions from this session's handler
		matchingSubIDs := session.handler.GetMatchingSubscriptions(endpointID, featureID, attrID)
		s.debugLog("NotifyAttributeChange: found matching subscriptions",
			"zoneID", session.ZoneID(),
			"matchCount", len(matchingSubIDs),
			"subscriptionIDs", matchingSubIDs)

		for _, subID := range matchingSubIDs {
			notif := &wire.Notification{
				SubscriptionID: subID,
				EndpointID:     endpointID,
				FeatureID:      featureID,
				Changes:        changes,
			}
			// Send notification (log errors but don't fail - zone may have disconnected)
			if err := session.SendNotification(notif); err != nil {
				s.debugLog("NotifyAttributeChange: failed to send notification",
					"zoneID", session.ZoneID(),
					"subscriptionID", subID,
					"error", err)
			} else {
				notificationsSent++
				s.debugLog("NotifyAttributeChange: notification sent",
					"zoneID", session.ZoneID(),
					"subscriptionID", subID)
			}
		}
	}

	s.debugLog("NotifyAttributeChange: complete", "notificationsSent", notificationsSent)
	return nil
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

// SetCertStore sets the certificate store for persistence.
func (s *DeviceService) SetCertStore(store cert.Store) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.certStore = store
}

// SetStateStore sets the state store for persistence.
func (s *DeviceService) SetStateStore(store *persistence.DeviceStateStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateStore = store
}

// SaveState persists the current device state.
// This should be called on graceful shutdown and after commissioning changes.
func (s *DeviceService) SaveState() error {
	s.mu.RLock()
	store := s.stateStore
	if store == nil {
		s.mu.RUnlock()
		return nil // No store configured, no-op
	}

	state := &persistence.DeviceState{
		SavedAt:       time.Now(),
		ZoneIndexMap:  make(map[string]uint8),
		FailsafeState: make(map[string]persistence.FailsafeSnapshot),
	}

	// Save zone index map
	for zoneID, idx := range s.zoneIndexMap {
		state.ZoneIndexMap[zoneID] = idx
	}

	// Save zone memberships
	for zoneID, cz := range s.connectedZones {
		zm := persistence.ZoneMembership{
			ZoneID:   zoneID,
			ZoneType: uint8(cz.Type),
			JoinedAt: cz.LastSeen, // Use LastSeen as proxy for JoinedAt
		}
		state.Zones = append(state.Zones, zm)
	}

	// Save failsafe timer states
	for zoneID, timer := range s.failsafeTimers {
		snap := timer.Snapshot()
		state.FailsafeState[zoneID] = persistence.FailsafeSnapshot{
			State:     uint8(snap.State),
			Duration:  snap.Duration,
			StartedAt: snap.StartedAt,
			Remaining: snap.Remaining,
			Limits: persistence.FailsafeLimits{
				ConsumptionLimit:    snap.Limits.ConsumptionLimit,
				ProductionLimit:     snap.Limits.ProductionLimit,
				HasConsumptionLimit: snap.Limits.HasConsumptionLimit,
				HasProductionLimit:  snap.Limits.HasProductionLimit,
			},
		}
	}

	s.mu.RUnlock()

	return store.Save(state)
}

// LoadState restores the device state from persistence.
// This should be called during Start() if a state store is configured.
func (s *DeviceService) LoadState() error {
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

	// Track restored zones to emit events after unlock
	var restoredZones []string

	// Restore zone index map
	for zoneID, idx := range state.ZoneIndexMap {
		s.zoneIndexMap[zoneID] = idx
		if idx >= s.nextZoneIndex {
			s.nextZoneIndex = idx + 1
		}
	}

	// Restore zone memberships (marked as not connected since no active connection)
	if len(state.Zones) > 0 {
		// New format: zones are explicitly saved
		for _, zm := range state.Zones {
			zoneType := cert.ZoneType(zm.ZoneType)
			cz := &ConnectedZone{
				ID:        zm.ZoneID,
				Type:      zoneType,
				Priority:  zoneType.Priority(),
				Connected: false, // Not connected until controller reconnects
				LastSeen:  zm.JoinedAt,
			}
			s.connectedZones[zm.ZoneID] = cz
			restoredZones = append(restoredZones, zm.ZoneID)
		}
	} else if len(state.ZoneIndexMap) > 0 {
		// Backward compatibility: derive zones from zone_index_map
		// We don't have zone type info, assume Local as default
		for zoneID := range state.ZoneIndexMap {
			zoneType := cert.ZoneTypeLocal
			cz := &ConnectedZone{
				ID:        zoneID,
				Type:      zoneType,
				Priority:  zoneType.Priority(),
				Connected: false,
				LastSeen:  state.SavedAt,
			}
			s.connectedZones[zoneID] = cz
			restoredZones = append(restoredZones, zoneID)
		}
	}

	// Restore failsafe timers
	for zoneID, snap := range state.FailsafeState {
		// Only restore timers that were running or in failsafe
		if snap.State == uint8(failsafe.StateNormal) {
			continue
		}

		timer := failsafe.NewTimer()
		timerSnap := &failsafe.TimerSnapshot{
			State:     failsafe.State(snap.State),
			Duration:  snap.Duration,
			StartedAt: snap.StartedAt,
			Remaining: snap.Remaining,
			Limits: failsafe.Limits{
				ConsumptionLimit:    snap.Limits.ConsumptionLimit,
				ProductionLimit:     snap.Limits.ProductionLimit,
				HasConsumptionLimit: snap.Limits.HasConsumptionLimit,
				HasProductionLimit:  snap.Limits.HasProductionLimit,
			},
		}

		// Set up callback before restore
		zoneIDCopy := zoneID // Capture for closure
		timer.OnFailsafeEnter(func(_ failsafe.Limits) {
			s.handleFailsafe(zoneIDCopy)
		})

		if err := timer.Restore(timerSnap); err != nil {
			s.debugLog("LoadState: failed to restore failsafe timer",
				"zoneID", zoneID, "error", err)
			continue
		}

		s.failsafeTimers[zoneID] = timer
	}

	s.mu.Unlock()

	// Emit events for restored zones (after releasing lock)
	for _, zoneID := range restoredZones {
		s.emitEvent(Event{
			Type:   EventZoneRestored,
			ZoneID: zoneID,
		})
	}

	return nil
}

// RemoveZone removes a zone from this device.
// It closes the session, removes from connectedZones, stops the failsafe timer,
// and stops operational mDNS advertising for this zone.
func (s *DeviceService) RemoveZone(zoneID string) error {
	s.mu.Lock()

	// Check if zone exists
	if _, exists := s.connectedZones[zoneID]; !exists {
		s.mu.Unlock()
		return ErrDeviceNotFound
	}

	// Close zone session if exists
	if session, exists := s.zoneSessions[zoneID]; exists {
		session.Close()
		delete(s.zoneSessions, zoneID)
	}

	// Stop and remove failsafe timer
	if timer, exists := s.failsafeTimers[zoneID]; exists {
		timer.Reset()
		delete(s.failsafeTimers, zoneID)
	}

	// Cancel any duration timers for this zone and remove from index map
	if zoneIndex, exists := s.zoneIndexMap[zoneID]; exists {
		s.durationManager.CancelZoneTimers(zoneIndex)
		delete(s.zoneIndexMap, zoneID)
	}

	// Stop operational mDNS advertising for this zone
	if s.discoveryManager != nil {
		if err := s.discoveryManager.RemoveZone(zoneID); err != nil {
			s.debugLog("RemoveZone: failed to stop operational advertising",
				"zoneID", zoneID, "error", err)
		}
	}

	// Remove from connected zones
	delete(s.connectedZones, zoneID)
	s.mu.Unlock()

	// Save state to persist the removal
	_ = s.SaveState() // Ignore error - zone is already removed from memory

	// Emit event
	s.emitEvent(Event{
		Type:   EventZoneRemoved,
		ZoneID: zoneID,
	})

	return nil
}

// StartOperationalAdvertising starts mDNS operational advertising for all known zones.
// This should be called after Start() when the device has restored zones from persistence.
// It allows controllers to rediscover the device for reconnection.
func (s *DeviceService) StartOperationalAdvertising() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.discoveryManager == nil {
		return nil // No discovery manager, skip
	}

	port := uint16(0)
	if s.tlsListener != nil {
		port = parsePort(s.tlsListener.Addr().String())
	}

	for zoneID := range s.connectedZones {
		opInfo := &discovery.OperationalInfo{
			ZoneID:        zoneID,
			DeviceID:      s.deviceID,
			VendorProduct: fmt.Sprintf("%04x:%04x", s.device.VendorID(), s.device.ProductID()),
			EndpointCount: uint8(s.device.EndpointCount()),
			Port:          port,
		}

		if err := s.discoveryManager.AddZone(s.ctx, opInfo); err != nil {
			s.debugLog("StartOperationalAdvertising: failed to start advertising",
				"zoneID", zoneID, "error", err)
		}
	}

	return nil
}

// ListZoneIDs returns a list of all connected zone IDs.
func (s *DeviceService) ListZoneIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.connectedZones))
	for id := range s.connectedZones {
		ids = append(ids, id)
	}
	return ids
}

// buildDeviceIdentity creates a DeviceIdentity from the device's information.
func (s *DeviceService) buildDeviceIdentity() *cert.DeviceIdentity {
	return &cert.DeviceIdentity{
		DeviceID:  s.deviceID,
		VendorID:  s.device.VendorID(),
		ProductID: s.device.ProductID(),
	}
}

package service

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/duration"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/mash-protocol/mash-go/pkg/log"
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
	browser          discovery.Browser

	// Pairing request browsing
	pairingRequestActive bool
	pairingRequestCancel context.CancelFunc

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

	// Protocol logger for structured event capture (optional)
	protocolLogger log.Logger

	// Persistence (optional, set by CLI)
	certStore  cert.Store
	stateStore *persistence.DeviceStateStore

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Security Hardening (DEC-047)
	// Connection tracking
	commissioningConnActive  bool       // Only one commissioning connection allowed
	lastCommissioningAttempt time.Time  // For connection cooldown
	connectionMu             sync.Mutex // Protects connection tracking fields

	// PASE attempt tracking
	paseTracker *PASEAttemptTracker

	// Transport-level connection cap (DEC-062)
	activeConns atomic.Int32

	// Stale connection reaper (DEC-064)
	connTracker *connTracker
}

// generateConnectionID generates a random connection ID for logging.
func generateConnectionID() string {
	b := make([]byte, 8) // 8 bytes = 16 hex chars
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
		connTracker:    newConnTracker(),
		logger:         config.Logger,
		protocolLogger: config.ProtocolLogger,
	}

	// Initialize duration manager with expiry callback
	svc.durationManager = duration.NewManager()
	svc.durationManager.OnExpiry(func(zoneIndex uint8, cmdType duration.CommandType, value any) {
		svc.handleDurationExpiry(zoneIndex, cmdType, value)
	})

	// Initialize subscription manager
	subConfig := subscription.DefaultConfig()
	svc.subscriptionManager = subscription.NewManagerWithConfig(subConfig)

	// Initialize PASE attempt tracker (DEC-047)
	// Skip in test mode to avoid blocking rapid commissioning cycles.
	if config.PASEBackoffEnabled && !config.TestMode {
		svc.paseTracker = NewPASEAttemptTracker(config.PASEBackoffTiers)
	}

	// In test mode, extend the commissioning window to 24h if still at the default 15m,
	// so the test harness has time to run the full suite without the window expiring.
	if config.TestMode && svc.config.CommissioningWindowDuration == 15*time.Minute {
		svc.config.CommissioningWindowDuration = 24 * time.Hour
	}

	// In test mode, bump MaxZones to 3 (GRID + LOCAL + TEST) if still at default 2 (DEC-060).
	if config.TestMode && svc.config.MaxZones == 2 {
		svc.config.MaxZones = 3
	}

	// Register service-level commands on DeviceInfo feature
	svc.registerDeviceCommands()

	// Subscribe to attribute changes on all features so that command handlers
	// (e.g., SetLimit) that modify attributes internally trigger EventValueChanged
	// events for the interactive display and other event listeners.
	svc.subscribeToFeatureChanges()

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

	// Get cert store for loading zone memberships
	s.mu.RLock()
	certStore := s.certStore
	s.mu.RUnlock()

	// Check if we have any zone memberships (i.e., we're commissioned)
	var zones []string
	if certStore != nil {
		zones = certStore.ListZones()
	}

	if len(zones) > 0 {
		// COMMISSIONED: Load operational certs from zones
		// Use the first zone's operational cert for the device ID
		firstZone := zones[0]
		opCert, err := certStore.GetOperationalCert(firstZone)
		if err != nil {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return fmt.Errorf("failed to load operational certificate for zone %s: %w", firstZone, err)
		}

		// Use operational cert for TLS
		s.tlsCert = opCert.TLSCertificate()

		// Device ID is extracted from operational cert (Matter-style: embedded in CommonName)
		s.deviceID, _ = cert.ExtractDeviceID(opCert.Certificate)

		// TODO: In Phase 4, implement per-zone TLS config for multi-zone support
		// For now, use the first zone's cert for all connections
	} else {
		// UNCOMMISSIONED: Generate throwaway commissioning cert (not persisted)
		// This cert is only used for TLS during PASE commissioning.
		// The device ID will be assigned during cert exchange after PASE.
		var err error
		s.tlsCert, err = generateSelfSignedCert(s.config.Discriminator)
		if err != nil {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}

		// No device ID until commissioned - will be derived from operational cert
		s.deviceID = ""
	}

	// Create TLS config for commissioning (no client cert required)
	s.tlsConfig = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{s.tlsCert},
		ClientAuth:   tls.RequestClientCert, // Request but don't require; used to distinguish operational reconnections
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

	// Start stale connection reaper if enabled (DEC-064)
	if s.config.StaleConnectionTimeout > 0 {
		go s.runStaleConnectionReaper()
	}

	// Initialize discovery advertiser if not already set (e.g., by tests)
	if s.advertiser == nil {
		advConfig := discovery.DefaultAdvertiserConfig()
		if s.config.TestMode {
			advConfig.Quiet = true
		}
		advertiser, err := discovery.NewMDNSAdvertiser(advConfig)
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

		// Set commissioning window duration from config
		if s.config.CommissioningWindowDuration > 0 {
			s.discoveryManager.SetCommissioningWindowDuration(s.config.CommissioningWindowDuration)
		}

		// Register callback for commissioning timeout
		s.discoveryManager.OnCommissioningTimeout(func() {
			s.emitEvent(Event{
				Type:   EventCommissioningClosed,
				Reason: "timeout",
			})
		})
	}

	s.mu.Lock()
	s.state = StateRunning
	s.mu.Unlock()

	// Start pairing request listening if configured
	if s.config.ListenForPairingRequests {
		_ = s.StartPairingRequestListening(s.ctx)
	}

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

		// Transport-level connection cap (DEC-062): reject at TCP level before TLS.
		// Both check and increment happen here in acceptLoop (single goroutine), so no TOCTOU race.
		if s.activeConns.Load() >= int32(s.config.MaxZones+1) {
			_ = conn.Close()
			continue
		}
		s.activeConns.Add(1)
		s.connTracker.Add(conn)

		go s.handleConnection(conn)
	}
}

// handleConnection handles an incoming connection.
func (s *DeviceService) handleConnection(conn net.Conn) {
	defer s.activeConns.Add(-1)     // DEC-062: decrement on any exit path
	defer s.connTracker.Remove(conn) // DEC-064: safety net removal

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
	var discoveryState string
	if s.discoveryManager != nil {
		discoveryState = s.discoveryManager.State().String()
	}
	s.mu.RUnlock()

	// If the peer presented a client certificate, this is an operational
	// reconnection from a known zone (not a new PASE commissioning).
	peerHasCert := len(state.PeerCertificates) > 0

	s.debugLog("handleConnection: routing decision",
		"discoveryState", discoveryState,
		"inCommissioningMode", inCommissioningMode,
		"peerHasCert", peerHasCert,
		"remoteAddr", tlsConn.RemoteAddr().String())

	if inCommissioningMode && !peerHasCert {
		s.debugLog("handleConnection: -> commissioning handler")
		s.handleCommissioningConnection(tlsConn)
	} else {
		// Operational mode - handle reconnection from known zones.
		// Also used when in commissioning mode but the peer presented
		// a certificate (indicating an operational reconnection).
		s.debugLog("handleConnection: -> operational handler")
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
		// No known disconnected zones - reject connection.
		// Log the full zone state map for diagnostics.
		s.mu.RLock()
		zoneStates := make([]string, 0, len(s.connectedZones))
		for zid, cz := range s.connectedZones {
			zoneStates = append(zoneStates, fmt.Sprintf("%s(connected=%v)", zid, cz.Connected))
		}
		s.mu.RUnlock()
		s.debugLog("handleOperationalConnection: no disconnected zones to reconnect",
			"zoneCount", len(zoneStates),
			"zones", zoneStates)
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

	// Set zone type from connected zone metadata
	s.mu.RLock()
	if cz, exists := s.connectedZones[targetZoneID]; exists {
		zoneSession.SetZoneType(cz.Type)
	}
	s.mu.RUnlock()

	// Set snapshot policy and protocol logger if configured
	zoneSession.SetSnapshotPolicy(s.config.SnapshotPolicy)
	if s.protocolLogger != nil {
		connID := generateConnectionID()
		zoneSession.SetProtocolLogger(s.protocolLogger, connID)
	}

	// Initialize renewal handler for certificate renewal support
	zoneSession.InitializeRenewalHandler(s.buildDeviceIdentity())

	// Set callback to persist certificate after renewal
	zoneSession.SetOnCertRenewalSuccess(s.handleCertRenewalSuccess)

	// Set callback to emit events when attributes are written
	zoneSession.SetOnWrite(s.makeWriteCallback(targetZoneID))

	// Set callback to emit events when commands are invoked
	zoneSession.SetOnInvoke(s.makeInvokeCallback(targetZoneID))

	// Store the session
	s.mu.Lock()
	s.zoneSessions[targetZoneID] = zoneSession
	s.mu.Unlock()

	// Emit connected event
	s.emitEvent(Event{
		Type:   EventConnected,
		ZoneID: targetZoneID,
	})

	// DEC-064: Remove from tracker before entering the operational message loop.
	// Operational connections must not be reaped by the stale connection reaper.
	s.connTracker.Remove(conn)

	// Start message loop - blocks until connection closes
	s.runZoneMessageLoop(targetZoneID, framedConn, zoneSession)

	// Clean up on disconnect
	s.handleZoneSessionClose(targetZoneID)
}

// handleCommissioningConnection handles PASE commissioning over TLS.
// After PASE succeeds, it performs the certificate exchange to receive an
// operational certificate from the controller's Zone CA.
//
// DEC-061: The commissioning lock is NOT acquired until the first PASERequest
// message arrives. This prevents idle TLS connections from blocking commissioning.
// Flow: Create PASE -> WaitForPASERequest (no lock) -> Acquire lock -> CompleteHandshake -> Cert exchange -> Release lock.
func (s *DeviceService) handleCommissioningConnection(conn *tls.Conn) {
	// Phase 1: Create PASE session and wait for first message (no lock held)
	paseSession, err := commissioning.NewPASEServerSession(s.verifier, s.serverID)
	if err != nil {
		conn.Close()
		return
	}

	// DEC-061: Wait for PASERequest with short timeout (no lock held)
	firstMsgCtx := s.ctx
	if s.config.PASEFirstMessageTimeout > 0 {
		var cancel context.CancelFunc
		firstMsgCtx, cancel = context.WithTimeout(s.ctx, s.config.PASEFirstMessageTimeout)
		defer cancel()
	}

	req, err := paseSession.WaitForPASERequest(firstMsgCtx, conn)
	if err != nil {
		// Timeout or connection closed before PASERequest -- no lock was held
		s.debugLog("handleCommissioningConnection: WaitForPASERequest failed (no lock held)", "error", err)
		conn.Close()
		return
	}

	// Phase 2: PASERequest received -- now acquire the commissioning lock
	if ok, reason := s.acceptCommissioningConnection(); !ok {
		s.debugLog("handleCommissioningConnection: rejected after PASERequest", "reason", reason)
		retryAfterMs := s.computeBusyRetryAfter()
		_ = commissioning.WriteCommissioningError(conn, commissioning.ErrCodeBusy, reason, retryAfterMs)
		conn.Close()
		return
	}
	defer s.releaseCommissioningConnection()

	// DEC-047: Overall handshake timeout (starts at PASERequest, not TLS accept)
	handshakeCtx := s.ctx
	if s.config.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		handshakeCtx, cancel = context.WithTimeout(s.ctx, s.config.HandshakeTimeout)
		defer cancel()
	}

	// DEC-047: Apply PASE backoff delay before processing
	if s.paseTracker != nil {
		delay := s.paseTracker.GetDelay()
		if delay > 0 {
			s.debugLog("handleCommissioningConnection: applying backoff delay", "delay", delay)
			select {
			case <-time.After(delay):
			case <-handshakeCtx.Done():
				conn.Close()
				return
			}
		}
	}

	// Phase 3: Complete PASE handshake (lock held)
	sharedSecret, err := paseSession.CompleteHandshake(handshakeCtx, conn, req)
	if err != nil {
		// PASE failed - wrong setup code or protocol error
		// DEC-047: Record failure for backoff
		if s.paseTracker != nil {
			s.paseTracker.RecordFailure()
		}
		conn.Close()
		return
	}

	// DEC-047: Reset PASE tracker on successful authentication
	s.ResetPASETracker()

	// Derive zone ID from shared secret
	zoneID := deriveZoneID(sharedSecret)

	s.debugLog("handleCommissioningConnection: PASE succeeded, starting cert exchange", "zoneID", zoneID)

	// Create framed connection FIRST - needed for cert exchange messages
	framedConn := newFramedConnection(conn)

	// Perform certificate exchange with controller
	// This is the critical step that gives us an operational cert from the Zone CA
	operationalCert, zoneCA, err := s.performCertExchange(framedConn, zoneID)
	if err != nil {
		s.debugLog("handleCommissioningConnection: cert exchange failed", "error", err)
		conn.Close()
		return
	}

	// Extract device ID from operational certificate (Matter-style: embedded in CommonName)
	deviceID, err := cert.ExtractDeviceID(operationalCert.Certificate)
	if err != nil {
		s.debugLog("handleCommissioningConnection: failed to extract device ID", "error", err)
		conn.Close()
		return
	}

	s.debugLog("handleCommissioningConnection: cert exchange complete",
		"deviceID", deviceID,
		"zoneID", zoneID,
		"certExpires", operationalCert.Certificate.NotAfter)

	// Update service device ID (use first zone's ID as primary)
	s.mu.Lock()
	if s.deviceID == "" {
		s.deviceID = deviceID
	}
	// Always update TLS cert to use the latest operational cert.
	// On re-commission (e.g., second zone), the device gets a new cert
	// from the new zone's CA -- we must present that cert on future TLS
	// connections, otherwise the controller sees a stale certificate.
	s.tlsCert = operationalCert.TLSCertificate()
	s.tlsConfig.Certificates = []tls.Certificate{s.tlsCert}
	s.mu.Unlock()

	// Store Zone CA for future verification of controller connections
	_ = zoneCA // Zone CA already stored in performCertExchange

	// Extract zone type from the Zone CA certificate's OU field.
	// Falls back to LOCAL if extraction fails (backward compatibility).
	zoneType := cert.ZoneTypeLocal
	if zt, err := cert.ExtractZoneTypeFromCert(zoneCA); err == nil {
		zoneType = zt
	}

	// Reject TEST zones unless the device is in test mode (DEC-060).
	if zoneType == cert.ZoneTypeTest && !s.config.TestMode {
		s.debugLog("handleCommissioningConnection: TEST zone rejected (TestMode disabled)", "zoneID", zoneID)
		conn.Close()
		return
	}

	// Register the zone connection
	s.HandleZoneConnect(zoneID, zoneType)

	// Persist state immediately after commissioning
	_ = s.SaveState()

	// Exit commissioning mode after successful commission
	// The device should no longer advertise as commissionable
	if err := s.ExitCommissioningMode(); err != nil {
		s.debugLog("handleCommissioningConnection: failed to exit commissioning mode", "error", err)
	}

	// In test mode, schedule auto-reentry into commissioning mode so the next
	// test can commission without waiting for manual intervention.
	if s.config.TestMode {
		s.debugLog("handleCommissioningConnection: scheduling test-mode auto-reentry goroutine", "zoneID", zoneID)
		go func() {
			s.debugLog("handleCommissioningConnection: auto-reentry goroutine started, sleeping 500ms", "zoneID", zoneID)
			time.Sleep(500 * time.Millisecond)
			s.debugLog("handleCommissioningConnection: auto-reentry goroutine woke up, calling EnterCommissioningMode", "zoneID", zoneID)
			if err := s.EnterCommissioningMode(); err != nil {
				s.debugLog("handleCommissioningConnection: test-mode auto-reenter failed", "zoneID", zoneID, "error", err)
			} else {
				s.debugLog("handleCommissioningConnection: test-mode auto-reenter succeeded", "zoneID", zoneID)
			}
		}()
	}

	// Create zone session for this connection
	zoneSession := NewZoneSession(zoneID, framedConn, s.device)
	zoneSession.SetLogger(s.logger)
	zoneSession.SetZoneType(zoneType)

	// Set snapshot policy and protocol logger if configured
	zoneSession.SetSnapshotPolicy(s.config.SnapshotPolicy)
	if s.protocolLogger != nil {
		connID := generateConnectionID()
		zoneSession.SetProtocolLogger(s.protocolLogger, connID)
	}

	// Initialize renewal handler for certificate renewal support
	zoneSession.InitializeRenewalHandler(s.buildDeviceIdentity())

	// Set callback to persist certificate after renewal
	zoneSession.SetOnCertRenewalSuccess(s.handleCertRenewalSuccess)

	// Set callback to emit events when attributes are written
	zoneSession.SetOnWrite(s.makeWriteCallback(zoneID))

	// Set callback to emit events when commands are invoked
	zoneSession.SetOnInvoke(s.makeInvokeCallback(zoneID))

	// Store the session
	s.mu.Lock()
	s.zoneSessions[zoneID] = zoneSession
	s.mu.Unlock()

	// Release the commissioning connection guard before entering the
	// message loop. The commissioning phase (PASE + cert exchange) is
	// complete; holding the guard during the operational message loop
	// would block all subsequent commissioning attempts for other zones.
	s.releaseCommissioningConnection()

	// DEC-064: Remove from tracker before entering the operational message loop.
	// Operational connections must not be reaped by the stale connection reaper.
	s.connTracker.Remove(conn)

	// Start message loop - blocks until connection closes
	s.runZoneMessageLoop(zoneID, framedConn, zoneSession)

	// Clean up on disconnect
	s.handleZoneSessionClose(zoneID)
}

// performCertExchange handles the certificate exchange protocol with the controller.
// It receives the Zone CA, generates a new key pair, sends a CSR, and installs
// the signed operational certificate.
//
// Protocol flow:
// 1. Receive CertRenewalRequest with ZoneCA from controller
// 2. Generate NEW key pair (not reusing commissioning key)
// 3. Send CertRenewalCSR with device's CSR
// 4. Receive CertRenewalInstall with signed operational cert
// 5. Verify and store operational cert + Zone CA
// 6. Send CertRenewalAck
func (s *DeviceService) performCertExchange(conn *framedConnection, zoneID string) (*cert.OperationalCert, *x509.Certificate, error) {
	// Step 1: Wait for CertRenewalRequest from controller
	data, err := conn.ReadFrame()
	if err != nil {
		return nil, nil, fmt.Errorf("read cert renewal request: %w", err)
	}

	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return nil, nil, fmt.Errorf("decode cert renewal request: %w", err)
	}

	certReq, ok := msg.(*commissioning.CertRenewalRequest)
	if !ok {
		return nil, nil, fmt.Errorf("expected CertRenewalRequest, got %T", msg)
	}

	// Verify we received the Zone CA
	if len(certReq.ZoneCA) == 0 {
		return nil, nil, fmt.Errorf("CertRenewalRequest missing Zone CA")
	}

	// Parse the Zone CA certificate
	zoneCA, err := x509.ParseCertificate(certReq.ZoneCA)
	if err != nil {
		return nil, nil, fmt.Errorf("parse Zone CA: %w", err)
	}

	s.debugLog("performCertExchange: received Zone CA",
		"issuer", zoneCA.Issuer.String(),
		"notAfter", zoneCA.NotAfter)

	// Step 2: Generate NEW key pair for this zone
	// Important: We generate a fresh key pair, NOT reusing the commissioning key
	keyPair, err := cert.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("generate key pair: %w", err)
	}

	// Step 3: Create and send CSR
	csrInfo := &cert.CSRInfo{
		Identity: cert.DeviceIdentity{
			DeviceID:  "", // Will be derived from cert
			VendorID:  s.device.VendorID(),
			ProductID: s.device.ProductID(),
		},
	}

	csrDER, err := cert.CreateCSR(keyPair, csrInfo)
	if err != nil {
		return nil, nil, fmt.Errorf("create CSR: %w", err)
	}

	csrResp := &commissioning.CertRenewalCSR{
		MsgType: commissioning.MsgCertRenewalCSR,
		CSR:     csrDER,
	}

	csrData, err := cbor.Marshal(csrResp)
	if err != nil {
		return nil, nil, fmt.Errorf("encode CSR: %w", err)
	}

	if err := conn.Send(csrData); err != nil {
		return nil, nil, fmt.Errorf("send CSR: %w", err)
	}

	// Step 4: Wait for signed certificate
	data, err = conn.ReadFrame()
	if err != nil {
		return nil, nil, fmt.Errorf("read cert install: %w", err)
	}

	msg, err = commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return nil, nil, fmt.Errorf("decode cert install: %w", err)
	}

	certInstall, ok := msg.(*commissioning.CertRenewalInstall)
	if !ok {
		return nil, nil, fmt.Errorf("expected CertRenewalInstall, got %T", msg)
	}

	// Parse the new operational certificate
	newCert, err := x509.ParseCertificate(certInstall.NewCert)
	if err != nil {
		return nil, nil, fmt.Errorf("parse operational cert: %w", err)
	}

	// Verify the certificate is signed by the Zone CA
	roots := x509.NewCertPool()
	roots.AddCert(zoneCA)
	if _, err := newCert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		return nil, nil, fmt.Errorf("verify operational cert: %w", err)
	}

	// Step 5: Store operational cert and Zone CA
	operationalCert := &cert.OperationalCert{
		Certificate: newCert,
		PrivateKey:  keyPair.PrivateKey,
		ZoneID:      zoneID,
	}

	s.mu.RLock()
	certStore := s.certStore
	s.mu.RUnlock()

	if certStore != nil {
		// Store operational cert for this zone
		if err := certStore.SetOperationalCert(operationalCert); err != nil {
			return nil, nil, fmt.Errorf("store operational cert: %w", err)
		}

		// Store Zone CA for this zone
		if err := certStore.SetZoneCACert(zoneID, zoneCA); err != nil {
			return nil, nil, fmt.Errorf("store Zone CA: %w", err)
		}

		// Persist to disk
		if err := certStore.Save(); err != nil {
			return nil, nil, fmt.Errorf("save cert store: %w", err)
		}
	}

	// Step 6: Send acknowledgment
	ack := &commissioning.CertRenewalAck{
		MsgType:        commissioning.MsgCertRenewalAck,
		Status:         commissioning.RenewalStatusSuccess,
		ActiveSequence: certInstall.Sequence,
	}

	ackData, err := cbor.Marshal(ack)
	if err != nil {
		return nil, nil, fmt.Errorf("encode ack: %w", err)
	}

	if err := conn.Send(ackData); err != nil {
		return nil, nil, fmt.Errorf("send ack: %w", err)
	}

	s.debugLog("performCertExchange: complete",
		"zoneID", zoneID,
		"certExpires", newCert.NotAfter)

	return operationalCert, zoneCA, nil
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

// handleCertRenewalSuccess persists a renewed certificate to the cert store.
// This is called by the ZoneSession after successful certificate renewal.
func (s *DeviceService) handleCertRenewalSuccess(zoneID string, handler *DeviceRenewalHandler) {
	s.mu.RLock()
	certStore := s.certStore
	// Get ZoneType from connectedZones (source of truth for zone metadata)
	zoneType := cert.ZoneTypeLocal // default
	if cz, exists := s.connectedZones[zoneID]; exists {
		zoneType = cz.Type
	}
	s.mu.RUnlock()

	if certStore == nil {
		s.debugLog("handleCertRenewalSuccess: no cert store, skipping persistence")
		return
	}

	// Get the new certificate and key from the handler
	newCert := handler.ActiveCert()
	newKey := handler.ActiveKey()
	if newCert == nil || newKey == nil {
		s.debugLog("handleCertRenewalSuccess: no active cert/key in handler")
		return
	}

	// Create new operational cert
	opCert := &cert.OperationalCert{
		Certificate: newCert,
		PrivateKey:  newKey,
		ZoneID:      zoneID,
		ZoneType:    zoneType,
	}

	// Store and persist
	if err := certStore.SetOperationalCert(opCert); err != nil {
		s.debugLog("handleCertRenewalSuccess: failed to store cert", "error", err)
		return
	}

	if err := certStore.Save(); err != nil {
		s.debugLog("handleCertRenewalSuccess: failed to save cert store", "error", err)
		return
	}

	s.debugLog("handleCertRenewalSuccess: certificate renewed and persisted",
		"zoneID", zoneID,
		"subject", newCert.Subject.CommonName,
		"notAfter", newCert.NotAfter)
}

// makeWriteCallback creates a write callback that emits events for attribute changes.
func (s *DeviceService) makeWriteCallback(zoneID string) WriteCallback {
	return func(endpointID uint8, featureID uint8, attrs map[uint16]any) {
		// Emit an event for each written attribute
		for attrID, value := range attrs {
			s.emitEvent(Event{
				Type:        EventValueChanged,
				ZoneID:      zoneID,
				EndpointID:  endpointID,
				FeatureID:   uint16(featureID),
				AttributeID: attrID,
				Value:       value,
			})
		}
	}
}

// makeInvokeCallback creates an invoke callback that emits events for command invocations.
func (s *DeviceService) makeInvokeCallback(zoneID string) InvokeCallback {
	return func(endpointID uint8, featureID uint8, commandID uint8, params map[string]any, result any) {
		s.emitEvent(Event{
			Type:          EventCommandInvoked,
			ZoneID:        zoneID,
			EndpointID:    endpointID,
			FeatureID:     uint16(featureID),
			CommandID:     commandID,
			CommandParams: params,
			Value:         result,
		})
	}
}

// featureChangeSubscriber implements model.FeatureSubscriber to bridge
// model-layer attribute changes to the service event system.
type featureChangeSubscriber struct {
	svc        *DeviceService
	endpointID uint8
}

func (f *featureChangeSubscriber) OnAttributeChanged(featureType model.FeatureType, attrID uint16, value any) {
	f.svc.emitEvent(Event{
		Type:        EventValueChanged,
		EndpointID:  f.endpointID,
		FeatureID:   uint16(featureType),
		AttributeID: attrID,
		Value:       value,
	})

	// Bridge to zone session notifications so subscribed controllers
	// receive push updates for attribute changes from command handlers,
	// timer callbacks, and interactive commands.
	f.svc.notifyZoneSessions(f.endpointID, uint8(featureType), attrID, value)
}

// subscribeToFeatureChanges registers a FeatureSubscriber on all features
// across all endpoints so that internal attribute changes (from command handlers)
// emit EventValueChanged events.
func (s *DeviceService) subscribeToFeatureChanges() {
	for _, ep := range s.device.Endpoints() {
		for _, feat := range ep.Features() {
			feat.Subscribe(&featureChangeSubscriber{
				svc:        s,
				endpointID: ep.ID(),
			})
		}
	}
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

// ActiveConns returns the current number of active connections (DEC-062).
func (s *DeviceService) ActiveConns() int32 {
	return s.activeConns.Load()
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

	// Stop pairing request listening
	_ = s.StopPairingRequestListening()

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

	var discoveryState string
	if s.discoveryManager != nil {
		discoveryState = s.discoveryManager.State().String()
	}
	zoneCount := len(s.connectedZones)

	s.debugLog("EnterCommissioningMode: called",
		"serviceState", s.state.String(),
		"discoveryState", discoveryState,
		"zoneCount", zoneCount)

	if s.state != StateRunning {
		s.debugLog("EnterCommissioningMode: rejected - service not running", "state", s.state.String())
		return ErrNotStarted
	}

	if s.discoveryManager != nil {
		if err := s.discoveryManager.EnterCommissioningMode(s.ctx); err != nil {
			s.debugLog("EnterCommissioningMode: failed", "error", err)
			return err
		}
	}

	s.debugLog("EnterCommissioningMode: success")
	s.emitEvent(Event{Type: EventCommissioningOpened})
	return nil
}

// ExitCommissioningMode closes the commissioning window.
func (s *DeviceService) ExitCommissioningMode() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var discoveryState string
	if s.discoveryManager != nil {
		discoveryState = s.discoveryManager.State().String()
	}
	s.debugLog("ExitCommissioningMode: called", "discoveryState", discoveryState)

	if s.discoveryManager != nil {
		if err := s.discoveryManager.ExitCommissioningMode(); err != nil {
			s.debugLog("ExitCommissioningMode: failed", "error", err)
			return err
		}
	}

	// DEC-047: Reset PASE tracker when commissioning window closes
	s.ResetPASETracker()

	s.debugLog("ExitCommissioningMode: success")
	s.emitEvent(Event{Type: EventCommissioningClosed, Reason: "commissioned"})
	return nil
}

// ZoneCount returns the number of paired (commissioned) zones.
// Note: This includes both online and offline zones.
func (s *DeviceService) ZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connectedZones)
}

// ConnectedZoneCount returns the number of currently connected zones.
func (s *DeviceService) ConnectedZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, z := range s.connectedZones {
		if z.Connected {
			count++
		}
	}
	return count
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
	// Reject TEST zones unless in test mode (DEC-060).
	if zoneType == cert.ZoneTypeTest && !s.config.TestMode {
		s.debugLog("HandleZoneConnect: TEST zone rejected", "zoneID", zoneID)
		return
	}

	s.mu.Lock()

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

	// Extract device ID for this zone from operational cert
	// Device ID is zone-specific - embedded in the certificate's CommonName by controller
	deviceID := s.deviceID // Fallback to service device ID
	if s.certStore != nil {
		if opCert, err := s.certStore.GetOperationalCert(zoneID); err == nil {
			extractedID, _ := cert.ExtractDeviceID(opCert.Certificate)
			if extractedID != "" {
				deviceID = extractedID
				// Update service device ID if not set (first zone)
				if s.deviceID == "" {
					s.deviceID = extractedID
				}
			}
		}
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
			DeviceID:      deviceID,
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

	s.mu.Unlock()

	s.emitEvent(Event{
		Type:   EventConnected,
		ZoneID: zoneID,
	})

	// Update pairing request listening state based on zone count
	// Must be called after releasing lock since it acquires its own lock
	s.updatePairingRequestListening()
}

// HandleZoneDisconnect handles a zone disconnection.
func (s *DeviceService) HandleZoneDisconnect(zoneID string) {
	s.debugLog("HandleZoneDisconnect: called", "zoneID", zoneID)

	s.mu.Lock()

	if cz, exists := s.connectedZones[zoneID]; exists {
		cz.Connected = false
		cz.LastSeen = time.Now()
	}

	// The failsafe timer was already started on connect
	// It will trigger if no reconnect happens

	testMode := s.config.TestMode
	s.mu.Unlock()

	s.emitEvent(Event{
		Type:   EventDisconnected,
		ZoneID: zoneID,
	})

	// In test mode, immediately remove the zone so stale zones don't
	// block commissioning in subsequent test runs.
	if testMode {
		s.debugLog("HandleZoneDisconnect: test-mode auto-remove triggered", "zoneID", zoneID)
		_ = s.RemoveZone(zoneID)
	}
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

	// Use SetAttributeInternal to bypass access checks for device-side updates.
	// SetAttributeInternal triggers notifyAttributeChanged(), which calls the
	// featureChangeSubscriber, which in turn calls notifyZoneSessions() to push
	// notifications to all subscribed controllers.
	if err := feature.SetAttributeInternal(attrID, value); err != nil {
		s.debugLog("NotifyAttributeChange: failed to set attribute", "attrID", attrID, "error", err)
		return err
	}

	return nil
}

// NotifyZoneAttributeChange sends attribute change notifications to a specific zone's session.
// This is used for per-zone attributes (like myConsumptionLimit) where each zone sees different values.
func (s *DeviceService) NotifyZoneAttributeChange(zoneID string, endpointID uint8, featureID uint8, changes map[uint16]any) {
	s.mu.RLock()
	session, ok := s.zoneSessions[zoneID]
	s.mu.RUnlock()
	if !ok {
		return
	}

	for attrID, value := range changes {
		matchingSubIDs := session.handler.GetMatchingSubscriptions(endpointID, featureID, attrID)
		for _, subID := range matchingSubIDs {
			notif := &wire.Notification{
				SubscriptionID: subID,
				EndpointID:     endpointID,
				FeatureID:      featureID,
				Changes:        map[uint16]any{attrID: value},
			}
			if err := session.SendNotification(notif); err != nil {
				s.debugLog("NotifyZoneAttributeChange: failed to send",
					"zoneID", zoneID, "attrID", attrID, "error", err)
			}
		}
	}
}

// notifyZoneSessions sends a notification to all zone sessions with subscriptions
// matching the given endpoint, feature, and attribute. The attribute value must
// already be set on the feature before calling this method.
func (s *DeviceService) notifyZoneSessions(endpointID uint8, featureID uint8, attrID uint16, value any) {
	s.mu.RLock()
	sessions := make([]*ZoneSession, 0, len(s.zoneSessions))
	for _, session := range s.zoneSessions {
		sessions = append(sessions, session)
	}
	s.mu.RUnlock()

	if len(sessions) == 0 {
		return
	}

	changes := map[uint16]any{attrID: value}
	notificationsSent := 0

	for _, session := range sessions {
		// Get matching subscriptions from this session's handler
		matchingSubIDs := session.handler.GetMatchingSubscriptions(endpointID, featureID, attrID)

		for _, subID := range matchingSubIDs {
			notif := &wire.Notification{
				SubscriptionID: subID,
				EndpointID:     endpointID,
				FeatureID:      featureID,
				Changes:        changes,
			}
			// Send notification (log errors but don't fail - zone may have disconnected)
			if err := session.SendNotification(notif); err != nil {
				s.debugLog("notifyZoneSessions: failed to send notification",
					"zoneID", session.ZoneID(),
					"subscriptionID", subID,
					"error", err)
			} else {
				notificationsSent++
			}
		}
	}

	if notificationsSent > 0 {
		s.debugLog("notifyZoneSessions: complete",
			"endpointID", endpointID,
			"featureID", featureID,
			"attrID", attrID,
			"notificationsSent", notificationsSent)
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

	// Set commissioning window duration from config
	if s.config.CommissioningWindowDuration > 0 {
		s.discoveryManager.SetCommissioningWindowDuration(s.config.CommissioningWindowDuration)
	}

	// Register callback for commissioning timeout
	s.discoveryManager.OnCommissioningTimeout(func() {
		s.emitEvent(Event{
			Type:   EventCommissioningClosed,
			Reason: "timeout",
		})
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

// GetCertStore returns the certificate store.
func (s *DeviceService) GetCertStore() cert.Store {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.certStore
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

	// Update pairing request listening state based on zone count
	s.updatePairingRequestListening()

	// DEC-059: Auto re-enter commissioning mode when last zone is removed.
	// A device with no zones must become commissionable again.
	s.mu.RLock()
	noZonesLeft := len(s.connectedZones) == 0
	s.mu.RUnlock()
	s.debugLog("RemoveZone: auto-reentry check",
		"zoneID", zoneID,
		"noZonesLeft", noZonesLeft)
	if noZonesLeft {
		if err := s.EnterCommissioningMode(); err != nil {
			s.debugLog("RemoveZone: EnterCommissioningMode failed", "zoneID", zoneID, "error", err)
		}
	}

	return nil
}

// StartOperationalAdvertising starts mDNS operational advertising for all known zones.
// This should be called after Start() when the device has restored zones from persistence.
// It allows controllers to rediscover the device for reconnection.
func (s *DeviceService) StartOperationalAdvertising() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.discoveryManager == nil {
		s.debugLog("StartOperationalAdvertising: no discovery manager, skipping")
		return nil // No discovery manager, skip
	}

	port := uint16(0)
	if s.tlsListener != nil {
		port = parsePort(s.tlsListener.Addr().String())
	}

	s.debugLog("StartOperationalAdvertising: advertising zones",
		"deviceID", s.deviceID,
		"zoneCount", len(s.connectedZones),
		"port", port)

	for zoneID := range s.connectedZones {
		opInfo := &discovery.OperationalInfo{
			ZoneID:        zoneID,
			DeviceID:      s.deviceID,
			VendorProduct: fmt.Sprintf("%04x:%04x", s.device.VendorID(), s.device.ProductID()),
			EndpointCount: uint8(s.device.EndpointCount()),
			Port:          port,
		}

		s.debugLog("StartOperationalAdvertising: advertising zone",
			"zoneID", zoneID,
			"deviceID", s.deviceID)

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

// SetBrowser sets the discovery browser (for testing/DI).
func (s *DeviceService) SetBrowser(browser discovery.Browser) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.browser = browser
}

// StartPairingRequestListening starts listening for pairing requests.
// When a pairing request with a matching discriminator is discovered,
// the device will automatically open its commissioning window.
func (s *DeviceService) StartPairingRequestListening(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Already listening
	if s.pairingRequestActive {
		return nil
	}

	// Check if at max zones - don't listen if we can't accept more
	if len(s.connectedZones) >= s.config.MaxZones {
		s.debugLog("StartPairingRequestListening: at max zones, not starting")
		return nil
	}

	// Need a browser to listen
	if s.browser == nil {
		s.debugLog("StartPairingRequestListening: no browser available")
		return nil
	}

	// Create cancellable context for browsing
	browseCtx, cancel := context.WithCancel(ctx)
	s.pairingRequestCancel = cancel
	s.pairingRequestActive = true

	// Start browsing in background
	go s.runPairingRequestListener(browseCtx)

	s.debugLog("StartPairingRequestListening: started")
	return nil
}

// StopPairingRequestListening stops listening for pairing requests.
func (s *DeviceService) StopPairingRequestListening() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.pairingRequestActive {
		return nil
	}

	if s.pairingRequestCancel != nil {
		s.pairingRequestCancel()
		s.pairingRequestCancel = nil
	}

	s.pairingRequestActive = false
	s.debugLog("StopPairingRequestListening: stopped")
	return nil
}

// IsPairingRequestListening returns true if the device is actively listening for pairing requests.
func (s *DeviceService) IsPairingRequestListening() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pairingRequestActive
}

// runPairingRequestListener runs the pairing request browser in the background.
func (s *DeviceService) runPairingRequestListener(ctx context.Context) {
	s.mu.RLock()
	browser := s.browser
	discriminator := s.config.Discriminator
	s.mu.RUnlock()

	if browser == nil {
		return
	}

	// BrowsePairingRequests calls the callback for each discovered pairing request
	// It blocks until the context is cancelled
	err := browser.BrowsePairingRequests(ctx, func(svc discovery.PairingRequestService) {
		s.handlePairingRequestDiscovered(svc, discriminator)
	})

	if err != nil && err != context.Canceled {
		s.debugLog("runPairingRequestListener: browse error", "error", err)
	}

	// Mark as inactive when browsing stops
	s.mu.Lock()
	s.pairingRequestActive = false
	s.mu.Unlock()
}

// handlePairingRequestDiscovered handles a discovered pairing request.
func (s *DeviceService) handlePairingRequestDiscovered(svc discovery.PairingRequestService, ourDiscriminator uint16) {
	s.debugLog("handlePairingRequestDiscovered: received pairing request",
		"theirDiscriminator", svc.Discriminator,
		"ourDiscriminator", ourDiscriminator,
		"zoneID", svc.ZoneID)

	// Check discriminator match
	if svc.Discriminator != ourDiscriminator {
		s.debugLog("handlePairingRequestDiscovered: discriminator mismatch, ignoring")
		return
	}

	s.mu.RLock()
	// Rate limiting: check if commissioning window is already open
	commissioningOpen := s.discoveryManager != nil && s.discoveryManager.IsCommissioningMode()
	// Check if at max zones
	atMaxZones := len(s.connectedZones) >= s.config.MaxZones
	s.mu.RUnlock()

	if commissioningOpen {
		s.debugLog("handlePairingRequestDiscovered: commissioning window already open, ignoring")
		return
	}

	if atMaxZones {
		s.debugLog("handlePairingRequestDiscovered: at max zones, ignoring")
		return
	}

	// Open commissioning window
	s.debugLog("handlePairingRequestDiscovered: opening commissioning window")
	if err := s.EnterCommissioningMode(); err != nil {
		s.debugLog("handlePairingRequestDiscovered: failed to enter commissioning mode", "error", err)
	}
}

// updatePairingRequestListening updates the listening state based on zone count.
// Called after zone changes to start/stop listening as needed.
func (s *DeviceService) updatePairingRequestListening() {
	if !s.config.ListenForPairingRequests {
		return
	}

	s.mu.RLock()
	zoneCount := len(s.connectedZones)
	maxZones := s.config.MaxZones
	active := s.pairingRequestActive
	ctx := s.ctx
	s.mu.RUnlock()

	if zoneCount >= maxZones && active {
		// At max zones - stop listening
		s.debugLog("updatePairingRequestListening: stopping (at max zones)")
		_ = s.StopPairingRequestListening()
	} else if zoneCount < maxZones && !active && ctx != nil {
		// Below max zones and not listening - start
		s.debugLog("updatePairingRequestListening: starting (below max zones)")
		_ = s.StartPairingRequestListening(ctx)
	}
}

// =============================================================================
// Security Hardening (DEC-047)
// =============================================================================

// acceptCommissioningConnection checks if a new commissioning connection should be accepted.
// Returns true if the connection can proceed, false if it should be rejected.
// The rejection reason is returned as a string for diagnostic logging.
//
// Connection protection rules:
// 1. Only one commissioning connection at a time
// 2. Connection cooldown (500ms default) between attempts
// 3. All zone slots must not be full (commissioning would fail anyway)
func (s *DeviceService) acceptCommissioningConnection() (bool, string) {
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()

	// Check 1: Is commissioning already in progress?
	if s.commissioningConnActive {
		return false, "commissioning already in progress"
	}

	// Check 2: Connection cooldown (skip in test mode)
	if s.config.ConnectionCooldown > 0 && !s.config.TestMode {
		elapsed := time.Since(s.lastCommissioningAttempt)
		if elapsed < s.config.ConnectionCooldown {
			return false, fmt.Sprintf("cooldown active (%s remaining)", s.config.ConnectionCooldown-elapsed)
		}
	}

	// Check 3: Is there a zone slot available?
	s.mu.RLock()
	zoneCount := len(s.connectedZones)
	maxZones := s.config.MaxZones
	s.mu.RUnlock()

	if zoneCount >= maxZones {
		return false, fmt.Sprintf("zone slots full (%d/%d)", zoneCount, maxZones)
	}

	// Accept the connection
	s.commissioningConnActive = true
	s.lastCommissioningAttempt = time.Now()
	return true, ""
}

// releaseCommissioningConnection marks the commissioning connection as complete.
// Call this when commissioning finishes (success or failure).
func (s *DeviceService) releaseCommissioningConnection() {
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()
	s.commissioningConnActive = false
}

// ResetPASETracker resets the PASE attempt tracker.
// Called when commissioning window closes or commissioning succeeds.
func (s *DeviceService) ResetPASETracker() {
	if s.paseTracker != nil {
		s.paseTracker.Reset()
	}
}

// runStaleConnectionReaper periodically closes pre-operational connections that
// have exceeded the StaleConnectionTimeout. This is a safety net for connections
// that never complete commissioning (DEC-064).
func (s *DeviceService) runStaleConnectionReaper() {
	ticker := time.NewTicker(s.config.ReaperInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if closed := s.connTracker.CloseStale(s.config.StaleConnectionTimeout); closed > 0 {
				s.debugLog("staleConnectionReaper: closed connections", "count", closed)
			}
		}
	}
}

// computeBusyRetryAfter returns the RetryAfter hint (in ms) for a busy rejection.
// The value depends on the rejection reason:
//   - Commissioning in progress: HandshakeTimeout ms (wait for current handshake to finish)
//   - Cooldown active: remaining cooldown ms
//   - Zones full: 0 (no point retrying until a zone is decommissioned)
func (s *DeviceService) computeBusyRetryAfter() uint32 {
	s.connectionMu.Lock()
	defer s.connectionMu.Unlock()

	// Check if cooldown is active
	if s.config.ConnectionCooldown > 0 {
		elapsed := time.Since(s.lastCommissioningAttempt)
		remaining := s.config.ConnectionCooldown - elapsed
		if remaining > 0 {
			return uint32(remaining.Milliseconds())
		}
	}

	// Check if commissioning is in progress
	if s.commissioningConnActive {
		return uint32(s.config.HandshakeTimeout.Milliseconds())
	}

	// Zones full or unknown reason -- no point retrying
	return 0
}

// randomErrorDelay returns a random duration between ErrorDelayMin and ErrorDelayMax.
// This is used to add jitter to error responses to prevent timing attacks (DEC-047).
func (s *DeviceService) randomErrorDelay() time.Duration {
	if !s.config.GenericErrors {
		return 0
	}
	if s.config.ErrorDelayMin >= s.config.ErrorDelayMax {
		return s.config.ErrorDelayMin
	}

	// Generate random delay in the range [min, max]
	delayRange := s.config.ErrorDelayMax - s.config.ErrorDelayMin
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	// Convert to uint64 and take modulo to get random offset
	randomOffset := time.Duration(0)
	for _, b := range randomBytes {
		randomOffset = (randomOffset << 8) | time.Duration(b)
	}
	randomOffset = randomOffset % (delayRange + 1)

	return s.config.ErrorDelayMin + randomOffset
}

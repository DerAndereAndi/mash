package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/duration"
	"github.com/mash-protocol/mash-go/pkg/failsafe"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/persistence"
	"github.com/mash-protocol/mash-go/pkg/service/dispatch"
	"github.com/mash-protocol/mash-go/pkg/subscription"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

var (
	errCommissionTestZoneDisabled = errors.New("commissioning: test zone rejected (no valid enable-key)")
	errCommissionZoneTypeExists   = errors.New("commissioning: zone type already exists")
	errCommissionZoneSlotsFull    = errors.New("commissioning: zone slots full")
)

const defaultDisconnectReentryHoldoff = 3 * time.Second

// DeviceService orchestrates a MASH device.
type DeviceService struct {
	mu sync.RWMutex

	config DeviceConfig
	device *model.Device
	state  ServiceState

	// Device identity (derived from certificate fingerprint)
	deviceID string

	// Discovery management
	discoveryManager *discovery.DiscoveryManager
	advertiser       discovery.Advertiser
	browser          discovery.Browser

	// Pairing request browsing
	pairingRequestActive bool
	pairingRequestCancel context.CancelFunc

	// Single unified listener with ALPN-based routing (DEC-067).
	// Commissioning (mash-comm/1) and operational (mash/1) connections share
	// one port. GetConfigForClient routes based on the client's ALPN.
	listener             net.Listener
	operationalTLSConfig *tls.Config
	commissioningOpen    atomic.Bool     // true when commissioning window is open
	commissioningEpoch   atomic.Uint64   // incremented on each EnterCommissioningMode
	commissioningCert    tls.Certificate // Stable, generated once at startup
	tlsCert              tls.Certificate // Operational cert (from zone CA)

	// PASE commissioning
	verifier *commissioning.Verifier
	serverID []byte

	// Timer management - one failsafe timer per zone
	failsafeTimers  map[string]*failsafe.Timer
	durationManager *duration.Manager

	// Subscription management
	subscriptionManager dispatch.SubscriptionTracker

	// Connected zones
	connectedZones map[string]*ConnectedZone

	// Zone sessions for operational messaging
	zoneSessions map[string]*ZoneSession

	// Zone ID to index mapping (for duration timers which use uint8)
	zoneIndexMap  map[string]uint8
	nextZoneIndex uint8

	// Event handlers
	eventHandlers []EventHandler

	// autoReentryPending is set after commissioning completes in test mode.
	// The next handleOperationalConnection consumes it to re-enter
	// commissioning mode without a sleep-based delay.
	autoReentryPending bool

	// disconnectReentryHoldoff suppresses auto-reentry in HandleZoneDisconnect
	// for a short period after explicit commissioning exit.
	disconnectReentryHoldoff      time.Duration
	disconnectReentryBlockedUntil time.Time
	disconnectReentryTimer        *time.Timer

	// Logger for debug output (optional)
	logger *slog.Logger

	// Protocol logger for structured event capture (optional)
	protocolLogger log.Logger

	// LimitResolver (optional, set by CLI via SetLimitResolver)
	limitResolver *features.LimitResolver

	// Persistence (optional, set by CLI)
	certStore  cert.Store
	stateStore *persistence.DeviceStateStore

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Test clock offset for certificate validation (set via TriggerAdjustClockBase)
	clockOffset time.Duration

	// Security Hardening (DEC-047)
	// Connection tracking
	commissioningConnActive  bool       // Only one commissioning connection allowed
	commissioningGeneration  uint64     // Monotonic counter to prevent stale release
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
		config:                   config,
		device:                   device,
		state:                    StateIdle,
		connectedZones:           make(map[string]*ConnectedZone),
		zoneSessions:             make(map[string]*ZoneSession),
		failsafeTimers:           make(map[string]*failsafe.Timer),
		zoneIndexMap:             make(map[string]uint8),
		connTracker:              newConnTracker(),
		certStore:                cert.NewMemoryStore(),
		logger:                   config.Logger,
		protocolLogger:           config.ProtocolLogger,
		disconnectReentryHoldoff: defaultDisconnectReentryHoldoff,
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
	// Backoff only triggers on failed attempts (wrong setup code), so it
	// does not slow down normal commissioning cycles that succeed immediately.
	if svc.config.PASEBackoffEnabled {
		svc.paseTracker = NewPASEAttemptTracker(svc.config.PASEBackoffTiers)
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

// isEnableKeyValid returns true if a valid enable-key is configured.
// A valid key is non-empty and not all zeros.
func (s *DeviceService) isEnableKeyValid() bool {
	key := s.config.TestEnableKey
	return key != "" && key != "00000000000000000000000000000000"
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

	// Resolve listen addresses for backward compatibility.
	s.resolveListenAddresses()

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

	// DEC-067: Generate stable commissioning certificate once at startup.
	// This cert is reused across all commissioning windows.
	s.commissioningCert, err = generateSelfSignedCert(s.config.Discriminator)
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
		s.deviceID, _ = cert.ExtractDeviceID(opCert.Certificate)

		// Build operational TLS config and start unified listener.
		s.buildOperationalTLSConfig()
		if err := s.ensureListenerStarted(); err != nil {
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}
	} else {
		// UNCOMMISSIONED: No operational cert yet. The operational listener
		// will be started after the first zone is commissioned.
		s.deviceID = ""
	}

	// Start stale connection reaper if enabled (DEC-064)
	if s.config.StaleConnectionTimeout > 0 {
		go s.runStaleConnectionReaper()
	}

	// Initialize discovery advertiser if not already set (e.g., by tests)
	if s.advertiser == nil {
		advConfig := discovery.DefaultAdvertiserConfig()
		advertiser, err := discovery.NewMDNSAdvertiser(advConfig)
		if err != nil {
			s.stopListener()
			s.mu.Lock()
			s.state = StateIdle
			s.mu.Unlock()
			return err
		}
		s.advertiser = advertiser
		s.discoveryManager = discovery.NewDiscoveryManager(advertiser)

		// Use operational port for commissionable info (same port, ALPN routing).
		commPort := parsePort(s.config.OperationalListenAddress)
		s.discoveryManager.SetCommissionableInfo(&discovery.CommissionableInfo{
			Discriminator: s.config.Discriminator,
			Categories:    s.config.Categories,
			Serial:        s.config.SerialNumber,
			Brand:         s.config.Brand,
			Model:         s.config.Model,
			DeviceName:    s.config.DeviceName,
			Port:          commPort,
		})

		// Set commissioning window duration from config
		if s.config.CommissioningWindowDuration > 0 {
			s.discoveryManager.SetCommissioningWindowDuration(s.config.CommissioningWindowDuration)
		}

		// Register callback for commissioning timeout
		s.discoveryManager.OnCommissioningTimeout(func() {
			// Close commissioning gate when the window times out.
			s.commissioningOpen.Store(false)
			// Stop listener if no zones exist.
			s.mu.RLock()
			zoneCount := len(s.connectedZones)
			s.mu.RUnlock()
			if zoneCount == 0 {
				s.stopListener()
			}
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

// resolveListenAddresses applies backward-compat mapping for ListenAddress.
func (s *DeviceService) resolveListenAddresses() {
	if s.config.ListenAddress != "" {
		if s.config.OperationalListenAddress == "" || s.config.OperationalListenAddress == ":8443" {
			s.config.OperationalListenAddress = s.config.ListenAddress
		}
	}
}

// buildOperationalTLSConfig creates the operational TLS config with RequireAndVerifyClientCert.
// The config includes operational certificates from ALL zones so the device
// can present the correct cert during TLS handshake regardless of which zone
// the controller is reconnecting to.
func (s *DeviceService) buildOperationalTLSConfig() {
	// Build CA pool from known zone CAs for client cert verification.
	caPool := x509.NewCertPool()
	if s.certStore != nil {
		for _, ca := range s.certStore.GetAllZoneCAs() {
			caPool.AddCert(ca)
		}
	}

	// Put the most recently set cert (s.tlsCert) first, then add
	// remaining zone certs. In TLS 1.3 the server cert is sent before
	// the client cert, so Go picks the first cert matching the negotiated
	// signature algorithm. Putting the newest cert first ensures fresh
	// commissions present the correct cert to the reconnecting controller.
	var certs []tls.Certificate
	deviceCertByID := make(map[string]*tls.Certificate)
	addCert := func(tc tls.Certificate) {
		certs = append(certs, tc)
		if deviceID, ok := tlsCertDeviceID(tc); ok {
			certCopy := tc
			deviceCertByID[deviceID] = &certCopy
			deviceCertByID[strings.ToLower(deviceID)] = &certCopy
		}
	}

	hasTLSCert := len(s.tlsCert.Certificate) > 0
	if hasTLSCert {
		addCert(s.tlsCert) // newest cert first
	}
	if s.certStore != nil {
		for _, zoneID := range s.certStore.ListZones() {
			if opCert, err := s.certStore.GetOperationalCert(zoneID); err == nil {
				tc := opCert.TLSCertificate()
				if !hasTLSCert || !sameTLSCert(tc, s.tlsCert) {
					addCert(tc)
				}
			}
		}
	}

	s.operationalTLSConfig = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
		Certificates: certs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		NextProtos:   []string{transport.ALPNProtocol},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if hello != nil && hello.ServerName != "" {
				if certByID, ok := deviceCertByID[hello.ServerName]; ok {
					return certByID, nil
				}
				if certByID, ok := deviceCertByID[strings.ToLower(hello.ServerName)]; ok {
					return certByID, nil
				}
			}
			if len(certs) == 0 {
				return nil, fmt.Errorf("no operational certificate configured")
			}
			return &certs[0], nil
		},
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return fmt.Errorf("no peer certificate")
			}
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("parse peer certificate: %w", err)
			}
			if cert.IsCA {
				return fmt.Errorf("controller certificate has CA:TRUE (must be end-entity)")
			}
			// Check certificate time validity using the device's clock offset
			// (set via TriggerAdjustClockBase). The 300s tolerance matches the
			// spec's clock-skew allowance.
			s.mu.RLock()
			offset := s.clockOffset
			s.mu.RUnlock()
			now := time.Now().Add(offset)
			const clockSkewTolerance = 300 * time.Second
			if now.Before(cert.NotBefore) && cert.NotBefore.Sub(now) > clockSkewTolerance {
				return fmt.Errorf("certificate not yet valid (notBefore=%s, now=%s)", cert.NotBefore.UTC(), now.UTC())
			}
			if now.After(cert.NotAfter) && now.Sub(cert.NotAfter) > clockSkewTolerance {
				return fmt.Errorf("certificate has expired (notAfter=%s, now=%s)", cert.NotAfter.UTC(), now.UTC())
			}
			return nil
		},
	}
}

// sameTLSCert returns true if two tls.Certificate values wrap the same leaf.
func sameTLSCert(a, b tls.Certificate) bool {
	if len(a.Certificate) == 0 || len(b.Certificate) == 0 {
		return false
	}
	return bytes.Equal(a.Certificate[0], b.Certificate[0])
}

func tlsCertDeviceID(c tls.Certificate) (string, bool) {
	if len(c.Certificate) == 0 {
		return "", false
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil || leaf.Subject.CommonName == "" {
		return "", false
	}
	return leaf.Subject.CommonName, true
}

// refreshTLSCert updates s.tlsCert to a remaining zone's operational cert.
// Called after RemoveZone to avoid presenting a stale cert from a removed zone.
// Caller must hold s.mu.Lock.
func (s *DeviceService) refreshTLSCert() {
	if s.certStore == nil {
		return
	}
	for _, zoneID := range s.certStore.ListZones() {
		if opCert, err := s.certStore.GetOperationalCert(zoneID); err == nil {
			s.tlsCert = opCert.TLSCertificate()
			s.buildOperationalTLSConfig()
			return
		}
	}
	// No zones left -- config will be rebuilt on next commissioning.
}

// getConfigForClient is the TLS callback that routes connections based on ALPN.
// Commissioning connections (mash-comm/1) get a NoClientCert config with the
// self-signed commissioning cert. Operational connections (mash/1) get a
// RequireAndVerifyClientCert config with the zone CA-signed cert.
func (s *DeviceService) getConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	wantsComm := false
	for _, proto := range hello.SupportedProtos {
		if proto == transport.ALPNCommissioningProtocol {
			wantsComm = true
			break
		}
	}

	if wantsComm {
		if !s.commissioningOpen.Load() {
			return nil, fmt.Errorf("commissioning not active")
		}
		return &tls.Config{
			MinVersion:   tls.VersionTLS13,
			MaxVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{s.commissioningCert},
			ClientAuth:   tls.NoClientCert,
			NextProtos:   []string{transport.ALPNCommissioningProtocol},
		}, nil
	}

	// Operational path
	if s.operationalTLSConfig == nil {
		return nil, fmt.Errorf("device not commissioned")
	}
	return s.operationalTLSConfig, nil
}

// ensureListenerStarted starts the unified TCP listener if not already running.
func (s *DeviceService) ensureListenerStarted() error {
	if s.listener != nil {
		return nil // Already running
	}
	listener, err := net.Listen("tcp", s.config.OperationalListenAddress)
	if err != nil {
		return fmt.Errorf("listener: %w", err)
	}
	s.listener = listener

	// Base TLS config with GetConfigForClient for ALPN routing.
	baseTLS := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		GetConfigForClient: s.getConfigForClient,
	}

	go s.acceptLoop(listener, baseTLS)
	s.debugLog("ensureListenerStarted: listening", "addr", listener.Addr().String())
	return nil
}

// stopListener closes the unified listener.
func (s *DeviceService) stopListener() {
	if s.listener != nil {
		s.listener.Close()
		s.listener = nil
	}
}

// acceptLoop accepts connections on the unified listener and dispatches
// them through TLS handshake with ALPN-based routing.
func (s *DeviceService) acceptLoop(listener net.Listener, baseTLS *tls.Config) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			// Listener closed or context cancelled.
			return
		}

		// Transport-level connection cap (DEC-062): reject at TCP level before TLS.
		cap := int32(s.config.MaxZones + 1)
		current := s.activeConns.Load()
		if current >= cap {
			s.debugLog("acceptLoop: connection rejected (cap)",
				"activeConns", current, "cap", cap,
				"remoteAddr", conn.RemoteAddr().String())
			_ = conn.Close()
			continue
		}
		s.activeConns.Add(1)
		s.connTracker.Add(conn)

		go s.handleIncomingConnection(conn, baseTLS)
	}
}

// verifyClientCert validates a client certificate against known Zone CAs.
// Returns nil if the certificate is valid, or an error describing why it was rejected.
func (s *DeviceService) verifyClientCert(peerCert *x509.Certificate) error {
	// Build a CA pool from all known Zone CAs.
	caPool := x509.NewCertPool()
	foundCA := false
	if s.certStore != nil {
		for _, ca := range s.certStore.GetAllZoneCAs() {
			caPool.AddCert(ca)
			foundCA = true
		}
	}

	if !foundCA {
		// No Zone CAs known yet -- allow connection (first commissioning
		// may present a self-signed cert before cert exchange completes).
		return nil
	}

	// Read clock offset under lock (may be set by test triggers).
	s.mu.RLock()
	offset := s.clockOffset
	s.mu.RUnlock()

	// Verify the client certificate against known Zone CAs.
	// Apply clock offset (from test triggers) to simulate clock skew.
	now := time.Now().Add(offset)
	opts := x509.VerifyOptions{
		Roots:       caPool,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		CurrentTime: now,
	}
	_, err := peerCert.Verify(opts)
	if err == nil {
		return nil
	}

	// Clock skew tolerance: accept certs whose NotBefore is up to 300s in
	// the future (device clock behind controller), or whose NotAfter is up
	// to 300s in the past (device clock ahead of controller).
	const clockSkewTolerance = 300 * time.Second
	if now.Before(peerCert.NotBefore) && peerCert.NotBefore.Sub(now) <= clockSkewTolerance {
		opts.CurrentTime = peerCert.NotBefore
		if _, err2 := peerCert.Verify(opts); err2 == nil {
			return nil
		}
	}
	if now.After(peerCert.NotAfter) && now.Sub(peerCert.NotAfter) <= clockSkewTolerance {
		opts.CurrentTime = peerCert.NotAfter
		if _, err2 := peerCert.Verify(opts); err2 == nil {
			return nil
		}
	}
	return err
}

// handleIncomingConnection handles TLS handshake for any incoming connection.
// ALPN-based routing (via GetConfigForClient) determines whether this is a
// commissioning or operational connection after the TLS handshake completes.
func (s *DeviceService) handleIncomingConnection(conn net.Conn, baseTLS *tls.Config) {
	activeConnReleased := false
	remoteAddr := conn.RemoteAddr().String()
	defer func() {
		if !activeConnReleased {
			newVal := s.activeConns.Add(-1)
			s.debugLog("handleIncomingConnection: defer released activeConn",
				"activeConns", newVal, "remoteAddr", remoteAddr)
		}
	}()
	releaseActiveConn := func() {
		if !activeConnReleased {
			s.activeConns.Add(-1)
			activeConnReleased = true
		}
	}
	defer s.connTracker.Remove(conn)

	// TLS handshake with per-connection timeout to prevent slowloris-style
	// attacks from holding cap slots indefinitely.
	handshakeCtx := s.ctx
	if s.config.TLSHandshakeTimeout > 0 {
		var cancel context.CancelFunc
		handshakeCtx, cancel = context.WithTimeout(s.ctx, s.config.TLSHandshakeTimeout)
		defer cancel()
	}
	s.debugLog("handleIncomingConnection: starting TLS handshake",
		"timeout", s.config.TLSHandshakeTimeout,
		"remoteAddr", remoteAddr)
	tlsConn := tls.Server(conn, baseTLS)
	if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
		conn.Close()
		return
	}

	state := tlsConn.ConnectionState()
	if err := transport.VerifyALPN(state); err != nil {
		tlsConn.Close()
		return
	}

	// Route based on negotiated ALPN protocol.
	if state.NegotiatedProtocol == transport.ALPNCommissioningProtocol {
		s.debugLog("handleIncomingConnection: -> commissioning handler", "remoteAddr", remoteAddr)
		s.handleCommissioningConnection(conn, tlsConn, releaseActiveConn)
		return
	}

	// Operational path: verify client certificate.
	if len(state.PeerCertificates) == 0 {
		s.debugLog("handleIncomingConnection: no client certificate", "remoteAddr", remoteAddr)
		tlsConn.Close()
		return
	}
	if len(state.PeerCertificates) > 2 {
		s.debugLog("handleIncomingConnection: certificate chain too long",
			"depth", len(state.PeerCertificates))
		tlsConn.Close()
		return
	}
	if err := s.verifyClientCert(state.PeerCertificates[0]); err != nil {
		s.debugLog("handleIncomingConnection: client cert rejected",
			"cn", state.PeerCertificates[0].Subject.CommonName,
			"err", err)
		tlsConn.Close()
		return
	}

	s.debugLog("handleIncomingConnection: -> operational handler", "remoteAddr", remoteAddr)
	s.handleOperationalConnection(conn, tlsConn, releaseActiveConn)
}

// Addr returns the unified listener's address.
func (s *DeviceService) Addr() net.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// TLSAddr returns the listener's address.
func (s *DeviceService) TLSAddr() net.Addr {
	return s.Addr()
}

// OperationalAddr returns the listener's address (same port as commissioning).
func (s *DeviceService) OperationalAddr() net.Addr {
	return s.Addr()
}

// CommissioningAddr returns the listener's address (same port as operational).
func (s *DeviceService) CommissioningAddr() net.Addr {
	return s.Addr()
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

	// Close the unified listener.
	s.stopListener()

	// Stop all failsafe timers
	s.mu.Lock()
	if s.disconnectReentryTimer != nil {
		s.disconnectReentryTimer.Stop()
		s.disconnectReentryTimer = nil
	}
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

// SetLimitResolver sets the LimitResolver so that TriggerResetTestState and
// RemoveZone can clear resolver state (limits, timers) alongside attribute state.
func (s *DeviceService) SetLimitResolver(lr *features.LimitResolver) {
	s.limitResolver = lr
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

// buildDeviceIdentity creates a DeviceIdentity from the device's information.
func (s *DeviceService) buildDeviceIdentity() *cert.DeviceIdentity {
	return &cert.DeviceIdentity{
		DeviceID:  s.deviceID,
		VendorID:  s.device.VendorID(),
		ProductID: s.device.ProductID(),
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

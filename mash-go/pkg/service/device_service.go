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
	"github.com/mash-protocol/mash-go/pkg/wire"
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

// matchZoneByPeerCert identifies the zone a reconnecting controller belongs to
// by matching the peer certificate's AuthorityKeyId against each zone's Zone CA
// SubjectKeyId. Returns the zone ID of the matching disconnected zone, or ""
// if no match is found.
//
// This replaces non-deterministic Go map iteration with cryptographic identity:
// each Zone CA has a unique SubjectKeyId (SHA-256 of the CA's public key), and
// every operational cert signed by that CA carries the same value as AuthorityKeyId.
//
// Caller must hold s.mu.RLock or s.mu.Lock.
func (s *DeviceService) matchZoneByPeerCert(peerCert *x509.Certificate) string {
	if peerCert == nil || len(peerCert.AuthorityKeyId) == 0 {
		return ""
	}

	for zoneID, cz := range s.connectedZones {
		if cz.Connected {
			continue
		}
		zoneCACert, err := s.certStore.GetZoneCACert(zoneID)
		if err != nil {
			continue
		}
		if bytes.Equal(peerCert.AuthorityKeyId, zoneCACert.SubjectKeyId) {
			return zoneID
		}
	}
	return ""
}

// matchConnectedZoneByPeerCert identifies a connected zone matching the peer
// certificate. This handles ungraceful disconnects where the device did not
// detect the client going away: the zone stays Connected=true but the old TCP
// socket is dead. When the controller reconnects with a fresh TLS connection,
// we match by AuthorityKeyId to find the stale session and replace it.
//
// Caller must hold s.mu.RLock or s.mu.Lock.
func (s *DeviceService) matchConnectedZoneByPeerCert(peerCert *x509.Certificate) string {
	if peerCert == nil || len(peerCert.AuthorityKeyId) == 0 {
		return ""
	}

	for zoneID, cz := range s.connectedZones {
		if !cz.Connected {
			continue
		}
		zoneCACert, err := s.certStore.GetZoneCACert(zoneID)
		if err != nil {
			continue
		}
		if bytes.Equal(peerCert.AuthorityKeyId, zoneCACert.SubjectKeyId) {
			return zoneID
		}
	}
	return ""
}

// handleOperationalConnection handles a reconnection from a known zone.
// rawConn is the underlying net.Conn from Accept, used for connTracker removal.
func (s *DeviceService) handleOperationalConnection(rawConn net.Conn, conn *tls.Conn, releaseActiveConn func()) {
	// Identify which zone this connection belongs to using the peer certificate's
	// AuthorityKeyId (matches the Zone CA's SubjectKeyId).
	var targetZoneID string
	var needsSessionReplace bool
	peerCerts := conn.ConnectionState().PeerCertificates
	s.mu.RLock()
	if len(peerCerts) > 0 {
		targetZoneID = s.matchZoneByPeerCert(peerCerts[0])
	}
	// Fallback: if cert-based matching fails (e.g. missing AuthorityKeyId),
	// pick any disconnected zone (preserves backward compatibility).
	if targetZoneID == "" {
		for zoneID, cz := range s.connectedZones {
			if !cz.Connected {
				targetZoneID = zoneID
				break
			}
		}
	}
	// Second chance: if no disconnected zone found, check if a connected zone
	// matches the peer cert. This handles ungraceful disconnects where the
	// device didn't detect the client going away.
	if targetZoneID == "" && len(peerCerts) > 0 {
		targetZoneID = s.matchConnectedZoneByPeerCert(peerCerts[0])
		needsSessionReplace = targetZoneID != ""
	}
	s.mu.RUnlock()

	if targetZoneID == "" {
		// No known zones match - reject connection.
		// Log the full zone state map for diagnostics.
		s.mu.RLock()
		zoneStates := make([]string, 0, len(s.connectedZones))
		for zid, cz := range s.connectedZones {
			zoneStates = append(zoneStates, fmt.Sprintf("%s(connected=%v)", zid, cz.Connected))
		}
		s.mu.RUnlock()
		s.debugLog("handleOperationalConnection: no matching zones to reconnect",
			"zoneCount", len(zoneStates),
			"zones", zoneStates)
		conn.Close()
		return
	}

	// If replacing an existing connected session (ungraceful disconnect recovery),
	// close the old session first so the zone transitions to disconnected state.
	if needsSessionReplace {
		s.debugLog("handleOperationalConnection: replacing stale session for connected zone", "zoneID", targetZoneID)
		s.handleZoneSessionClose(targetZoneID, nil)
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

	// Test mode auto-reentry: consume the flag set by handleCommissioningConnection
	// and re-enter commissioning mode now that the operational connection is live.
	s.mu.Lock()
	pending := s.autoReentryPending
	if pending {
		s.autoReentryPending = false
	}
	s.mu.Unlock()
	if pending && !s.isZonesFull() {
		if err := s.EnterCommissioningMode(); err != nil {
			s.debugLog("handleOperationalConnection: auto-reentry failed", "error", err)
		}
	}

	// DEC-064: Remove from tracker before entering the operational message loop.
	// Operational connections must not be reaped by the stale connection reaper.
	// Use rawConn (the original net.Conn from Accept) since the tracker keys
	// on that, not the *tls.Conn wrapper.
	s.connTracker.Remove(rawConn)

	// DEC-062: Release the connection cap slot before the message loop.
	// Operational connections are tracked by the zone system and should not
	// consume slots intended for limiting new incoming connections.
	releaseActiveConn()

	// Start message loop - blocks until connection closes
	s.runZoneMessageLoop(targetZoneID, framedConn, zoneSession)

	// Ensure the TLS connection is closed so the peer receives close_notify.
	// This is idempotent (framedConnection.Close guards with a bool).
	framedConn.Close()

	// Clean up on disconnect
	s.handleZoneSessionClose(targetZoneID, zoneSession)
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

		// Handle control messages (Ping/Pong/Close) at the transport level
		// before dispatching protocol messages to the session.
		msgType, peekErr := wire.PeekMessageType(data)
		if peekErr == nil && msgType == wire.MessageTypeControl {
			if ctrlMsg, decErr := wire.DecodeControlMessage(data); decErr == nil {
				switch ctrlMsg.Type {
				case wire.ControlPing:
					// Respond with pong.
					pongMsg := &wire.ControlMessage{Type: wire.ControlPong, Sequence: ctrlMsg.Sequence}
					if pongData, encErr := wire.EncodeControlMessage(pongMsg); encErr == nil {
						conn.Send(pongData)
					}
				case wire.ControlClose:
					// Acknowledge close and disconnect.
					closeAck := &wire.ControlMessage{Type: wire.ControlClose}
					if ackData, encErr := wire.EncodeControlMessage(closeAck); encErr == nil {
						conn.Send(ackData)
					}
					// conn.Send writes synchronously through TLS to the TCP
					// send buffer. conn.Close sends TLS close_notify then TCP
					// FIN; the kernel delivers buffered data before the FIN.
					conn.Close()
					return
				case wire.ControlPong:
					// Ignore pongs.
				}
				continue
			}
		}

		// Dispatch protocol messages to session.
		session.OnMessage(data)

		// If the zone was removed during message processing (e.g. RemoveZone
		// command), close the connection and exit. The response has already
		// been sent by OnMessage before we reach this point.
		s.mu.RLock()
		_, zoneExists := s.connectedZones[zoneID]
		s.mu.RUnlock()
		if !zoneExists {
			conn.Close()
			return
		}
	}
}

// handleZoneSessionClose cleans up when a zone session closes.
// When closingSession is non-nil, cleanup only runs if it is still the active
// session mapped for zoneID (protects against stale loop exits after reconnect).
func (s *DeviceService) handleZoneSessionClose(zoneID string, closingSession *ZoneSession) {
	s.mu.Lock()
	var sessionToClose *ZoneSession
	if session, exists := s.zoneSessions[zoneID]; exists {
		if closingSession != nil && session != closingSession {
			s.mu.Unlock()
			return
		}
		sessionToClose = session
		delete(s.zoneSessions, zoneID)
	}
	s.mu.Unlock()

	if sessionToClose == nil {
		return
	}

	// Close session outside the lock (dispatcher.Stop may block).
	sessionToClose.Close()

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

	renewedDeviceID, err := cert.ExtractDeviceID(newCert)
	if err != nil {
		s.debugLog("handleCertRenewalSuccess: failed to extract renewed device ID", "zoneID", zoneID, "error", err)
		return
	}

	var (
		dm     *discovery.DiscoveryManager
		port   uint16
		ctx    context.Context
		update bool
	)

	s.mu.Lock()
	s.tlsCert = opCert.TLSCertificate()
	s.buildOperationalTLSConfig()
	s.deviceID = renewedDeviceID

	if s.discoveryManager != nil {
		dm = s.discoveryManager
		port = parsePort(s.config.OperationalListenAddress)
		if s.listener != nil {
			port = parsePort(s.listener.Addr().String())
		}
		ctx = s.ctx
		update = true
	}
	s.mu.Unlock()

	if !update {
		return
	}

	opInfo := &discovery.OperationalInfo{
		ZoneID:        zoneID,
		DeviceID:      renewedDeviceID,
		VendorProduct: fmt.Sprintf("%04x:%04x", s.device.VendorID(), s.device.ProductID()),
		EndpointCount: uint8(s.device.EndpointCount()),
		Port:          port,
	}
	if err := dm.UpdateZone(opInfo); err != nil {
		if errors.Is(err, discovery.ErrNotFound) {
			if addErr := dm.AddZone(ctx, opInfo); addErr != nil {
				s.debugLog("handleCertRenewalSuccess: failed to add operational advertising after renewal",
					"zoneID", zoneID, "error", addErr)
			}
			return
		}
		s.debugLog("handleCertRenewalSuccess: failed to update operational advertising after renewal",
			"zoneID", zoneID, "error", err)
	}
}

// makeWriteCallback creates a write callback that emits events for attribute changes.
func (s *DeviceService) makeWriteCallback(zoneID string) dispatch.WriteCallback {
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
func (s *DeviceService) makeInvokeCallback(zoneID string) dispatch.InvokeCallback {
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
	s.handleZoneConnectInternal(zoneID, zoneType, true)
}

// RegisterZoneAwaitingConnection registers a zone after commissioning but before
// the operational TLS connection is established. The zone is marked as disconnected
// so handleOperationalConnection can accept the reconnection. (DEC-066)
func (s *DeviceService) RegisterZoneAwaitingConnection(zoneID string, zoneType cert.ZoneType) {
	s.handleZoneConnectInternal(zoneID, zoneType, false)
}

// handleZoneConnectInternal is the shared implementation for zone registration.
func (s *DeviceService) handleZoneConnectInternal(zoneID string, zoneType cert.ZoneType, connected bool) {
	// Reject TEST zones unless a valid enable-key is configured (DEC-060).
	if zoneType == cert.ZoneTypeTest && !s.isEnableKeyValid() {
		s.debugLog("HandleZoneConnect: TEST zone rejected (no valid enable-key)", "zoneID", zoneID)
		return
	}

	s.mu.Lock()

	// Create connected zone record
	cz := &ConnectedZone{
		ID:        zoneID,
		Type:      zoneType,
		Priority:  zoneType.Priority(),
		Connected: connected,
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

	// Prepare operational advertising info while under lock.
	var opInfo *discovery.OperationalInfo
	if s.discoveryManager != nil {
		port := uint16(0)
		if s.listener != nil {
			port = parsePort(s.listener.Addr().String())
		}

		opInfo = &discovery.OperationalInfo{
			ZoneID:        zoneID,
			DeviceID:      deviceID,
			VendorProduct: fmt.Sprintf("%04x:%04x", s.device.VendorID(), s.device.ProductID()),
			EndpointCount: uint8(s.device.EndpointCount()),
			Port:          port,
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

	ctx := s.ctx
	dm := s.discoveryManager
	s.mu.Unlock()

	// Start operational mDNS advertising outside the lock because mDNS
	// operations can take >1s on macOS and would block new connections.
	if dm != nil && opInfo != nil {
		if err := dm.AddZone(ctx, opInfo); err != nil {
			if errors.Is(err, discovery.ErrAlreadyExists) {
				if updateErr := dm.UpdateZone(opInfo); updateErr != nil {
					s.debugLog("HandleZoneConnect: failed to update operational advertising",
						"zoneID", zoneID, "error", updateErr)
				}
			} else {
				s.debugLog("HandleZoneConnect: failed to start operational advertising",
					"zoneID", zoneID, "error", err)
			}
		}
	}

	// Only emit EventConnected when an actual connection is established.
	// For RegisterZoneAwaitingConnection (DEC-066), connected=false and we
	// emit EventCommissioned separately after the commissioning flow completes.
	if connected {
		s.emitEvent(Event{
			Type:   EventConnected,
			ZoneID: zoneID,
		})
	}

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

	s.mu.Unlock()

	s.emitEvent(Event{
		Type:   EventDisconnected,
		ZoneID: zoneID,
	})

	// Note: In test mode we no longer auto-remove the zone on disconnect.
	// The test runner sends explicit RemoveZone via closeActiveZoneConns
	// between tests. Auto-removing here prevents reconnection scenarios
	// (failsafe tests, reconnect tests) because handleOperationalConnection
	// requires a disconnected-but-not-removed zone.

	// With enable-key active, auto-re-enter commissioning mode when all
	// zones are disconnected. The runner may not be able to send explicit
	// RemoveZone (dead connection), so the device must be ready for new
	// PASE without waiting for the runner to clean up.
	if s.isEnableKeyValid() {
		s.mu.RLock()
		allDisconnected := true
		for _, cz := range s.connectedZones {
			if cz.Connected {
				allDisconnected = false
				break
			}
		}
		blockedUntil := s.disconnectReentryBlockedUntil
		s.mu.RUnlock()
		if allDisconnected {
			if !blockedUntil.IsZero() && time.Now().Before(blockedUntil) {
				s.debugLog("HandleZoneDisconnect: auto-reentry suppressed during post-exit holdoff",
					"blockedUntil", blockedUntil.Format(time.RFC3339Nano))
				s.scheduleDisconnectReentry(blockedUntil)
				return
			}
			s.debugLog("HandleZoneDisconnect: all zones disconnected, re-entering commissioning mode")
			_ = s.EnterCommissioningMode()
		}
	}
}

func (s *DeviceService) scheduleDisconnectReentry(blockedUntil time.Time) {
	delay := time.Until(blockedUntil)
	if delay <= 0 {
		go s.tryAutoReenterCommissioningAfterHoldoff()
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disconnectReentryTimer != nil {
		s.disconnectReentryTimer.Stop()
	}
	s.disconnectReentryTimer = time.AfterFunc(delay, s.tryAutoReenterCommissioningAfterHoldoff)
}

func (s *DeviceService) tryAutoReenterCommissioningAfterHoldoff() {
	s.mu.Lock()
	s.disconnectReentryTimer = nil
	blockedUntil := s.disconnectReentryBlockedUntil
	allDisconnected := true
	for _, cz := range s.connectedZones {
		if cz.Connected {
			allDisconnected = false
			break
		}
	}
	s.mu.Unlock()

	if !s.isEnableKeyValid() || !allDisconnected {
		return
	}
	if !blockedUntil.IsZero() && time.Now().Before(blockedUntil) {
		s.scheduleDisconnectReentry(blockedUntil)
		return
	}

	s.debugLog("HandleZoneDisconnect: holdoff expired, re-entering commissioning mode")
	_ = s.EnterCommissioningMode()
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

	for _, session := range sessions {
		session.dispatcher.NotifyChange(endpointID, uint16(featureID), attrID, value)
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

	// Capture zone session reference and remove from map under lock.
	// Actual close happens asynchronously outside the lock because
	// session.Close() can block on dispatcher shutdown.
	var sessionToClose *ZoneSession
	if session, exists := s.zoneSessions[zoneID]; exists {
		sessionToClose = session
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

	// Capture limitResolver reference; ClearZone is called outside the lock
	// because its OnZoneMyChange callback calls NotifyZoneAttributeChange
	// which acquires s.mu.RLock() -- would deadlock if called under s.mu.Lock().
	lr := s.limitResolver

	// Remove from connected zones
	delete(s.connectedZones, zoneID)

	// Keep TLS identity coherent for immediate follow-up handshakes.
	if s.certStore != nil {
		_ = s.certStore.RemoveOperationalCert(zoneID)
	}
	s.refreshTLSCert()

	// Capture references needed by async cleanup.
	dm := s.discoveryManager
	hasAvailableSlots := s.nonTestZoneCountLocked() < s.config.MaxZones
	s.mu.Unlock()

	// Clear LimitResolver state outside the lock.
	if lr != nil {
		lr.ClearZone(zoneID)
	}

	// Preserve existing RemoveZone behavior for callers that expect
	// immediate removal signal and commissioning re-entry.
	s.emitEvent(Event{
		Type:   EventZoneRemoved,
		ZoneID: zoneID,
	})
	s.updatePairingRequestListening()
	s.debugLog("RemoveZone: auto-reentry check",
		"zoneID", zoneID,
		"hasAvailableSlots", hasAvailableSlots)
	if hasAvailableSlots {
		if err := s.EnterCommissioningMode(); err != nil {
			s.debugLog("RemoveZone: EnterCommissioningMode failed", "zoneID", zoneID, "error", err)
		}
	}

	// Complete potentially slow side effects in the background so RemoveZone
	// response ACK can be sent without waiting on mDNS/session/persistence work.
	go s.finishRemoveZoneCleanup(zoneID, sessionToClose, dm)

	return nil
}

func (s *DeviceService) finishRemoveZoneCleanup(zoneID string, sessionToClose *ZoneSession, dm *discovery.DiscoveryManager) {
	if sessionToClose != nil {
		sessionToClose.Close()
	}

	// Stop operational mDNS advertising for this zone. mDNS goodbye/stop may
	// block and must never hold up RemoveZone response timing.
	if dm != nil {
		if err := dm.RemoveZone(zoneID); err != nil {
			s.debugLog("RemoveZone: failed to stop operational advertising",
				"zoneID", zoneID, "error", err)
		}
	}

	// Save state to persist the removal.
	_ = s.SaveState() // Ignore error - zone is already removed from memory
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

// =============================================================================
// Security Hardening (DEC-047)
// =============================================================================

// hasZoneOfType returns true if a zone of the given type already exists.
// Used to enforce DEC-043: max 1 zone per type.
func (s *DeviceService) hasZoneOfType(zt cert.ZoneType) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, cz := range s.connectedZones {
		if cz.Type == zt {
			return true
		}
	}
	return false
}

// isZonesFull returns true when all zone slots are occupied.
// TEST zones don't count against MaxZones (they're an extra observer slot).
func (s *DeviceService) isZonesFull() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nonTestZoneCountLocked() >= s.config.MaxZones
}

// nonTestZoneCountLocked returns the count of non-TEST zones.
// Caller must hold s.mu (read or write lock).
func (s *DeviceService) nonTestZoneCountLocked() int {
	count := 0
	for _, cz := range s.connectedZones {
		if cz.Type != cert.ZoneTypeTest {
			count++
		}
	}
	return count
}

// evictDisconnectedZone removes disconnected non-TEST zones to free slots for
// new commissioning. Returns the first evicted zone ID, or "" if none found.
// TEST zones are skipped because they may be deliberately disconnected during
// tier transitions. Used in test mode only to recover from orphaned zones left
// by dead runner connections that couldn't send explicit RemoveZone.
func (s *DeviceService) evictDisconnectedZone() string {
	s.mu.Lock()
	var toEvict []string
	for zoneID, cz := range s.connectedZones {
		if !cz.Connected && cz.Type != cert.ZoneTypeTest {
			toEvict = append(toEvict, zoneID)
		}
	}
	s.mu.Unlock()

	var first string
	for _, zoneID := range toEvict {
		s.debugLog("evictDisconnectedZone: removing", "zoneID", zoneID)
		_ = s.RemoveZone(zoneID)
		if first == "" {
			first = zoneID
		}
	}
	return first
}

// evictDisconnectedZonesOfType removes disconnected zones of the provided
// type. Returns the first evicted zone ID, or "" if none were evicted.
func (s *DeviceService) evictDisconnectedZonesOfType(zt cert.ZoneType) string {
	s.mu.Lock()
	var toEvict []string
	for zoneID, cz := range s.connectedZones {
		if !cz.Connected && cz.Type == zt {
			toEvict = append(toEvict, zoneID)
		}
	}
	s.mu.Unlock()

	var first string
	for _, zoneID := range toEvict {
		s.debugLog("evictDisconnectedZonesOfType: removing", "zoneID", zoneID, "zoneType", zt)
		_ = s.RemoveZone(zoneID)
		if first == "" {
			first = zoneID
		}
	}
	return first
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

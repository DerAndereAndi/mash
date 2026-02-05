// Package runner provides test execution against real MASH devices/controllers.
package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/internal/testharness/reporter"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Runner executes test cases against a target device or controller.
type Runner struct {
	config       *Config
	engine       *engine.Engine
	engineConfig *engine.EngineConfig
	reporter     reporter.Reporter
	conn         *Connection
	resolver     *Resolver
	messageID    uint32 // Atomic counter for message IDs
	paseState    *PASEState
	pics         *loader.PICSFile // Cached PICS for handler access

	// zoneCA is the Zone CA generated during commissioning cert exchange.
	zoneCA *cert.ZoneCA

	// controllerCert is the controller operational cert generated during commissioning.
	controllerCert *cert.OperationalCert

	// zoneCAPool holds the Zone CA certificate pool obtained during commissioning.
	// Used for TLS verification on operational (non-commissioning) connections.
	zoneCAPool *x509.CertPool

	// activeZoneConns tracks zone connections across tests so they can be
	// cleaned up between test cases. Without this, connections from a prior
	// test leak and prevent the device from accepting new ones (all zones
	// appear "connected" on the device side).
	activeZoneConns map[string]*Connection

	// lastDeviceConnClose records when closeActiveZoneConns last closed
	// real device connections. This allows PrecondTwoZonesConnected to
	// wait for the device to process zone removals even when the current
	// test's hadActive is false.
	lastDeviceConnClose time.Time

	// activeZoneIDs maps zone names to their derived zone IDs (from PASE
	// session keys). Used by closeActiveZoneConns to send explicit
	// RemoveZone commands before closing connections.
	activeZoneIDs map[string]string

	// commissionZoneType overrides the zone type used when generating the
	// Zone CA during performCertExchange. Defaults to ZoneTypeLocal if zero.
	commissionZoneType cert.ZoneType

	// deviceStateModified is set when a trigger modifies device state.
	// Used by setupPreconditions to send TriggerResetTestState between tests.
	deviceStateModified bool
}

// Config configures the test runner.
type Config struct {
	// Target is the address of the device/controller under test.
	Target string

	// Mode is "device" or "controller".
	Mode string

	// PICSFile is the path to the PICS file.
	PICSFile string

	// TestDir is the path to test case directory.
	TestDir string

	// Pattern filters test cases by name.
	Pattern string

	// Timeout is the default test timeout.
	Timeout time.Duration

	// SuiteTimeout is the overall test suite timeout (0 = auto-calculate).
	SuiteTimeout time.Duration

	// Verbose enables verbose output.
	Verbose bool

	// Debug enables detailed debug logging of connection lifecycle,
	// precondition transitions, zone management, and PASE attempts.
	// Output includes timestamped runner state snapshots.
	Debug bool

	// Output is where to write results.
	Output io.Writer

	// OutputFormat is "text", "json", or "junit".
	OutputFormat string

	// InsecureSkipVerify skips TLS certificate verification.
	InsecureSkipVerify bool

	// SetupCode is the PASE password/setup code (8-digit numeric string).
	SetupCode string

	// ClientIdentity for PASE (defaults to "test-client").
	ClientIdentity string

	// ServerIdentity for PASE (defaults to "test-device").
	ServerIdentity string

	// EnableKey is the hex-encoded 128-bit key for TestControl triggers.
	// Defaults to "00112233445566778899aabbccddeeff" (well-known test key).
	EnableKey string

	// AutoPICS enables automatic PICS discovery from the device.
	// When true, the runner commissions the device at startup and reads
	// its capabilities to build a PICS file dynamically.
	AutoPICS bool

	// ProtocolLogger receives structured protocol events for debugging.
	// Set to nil to disable protocol logging.
	ProtocolLogger log.Logger
}

// Connection represents a connection to the target.
type Connection struct {
	conn      net.Conn
	tlsConn   *tls.Conn
	framer    *transport.Framer
	connected bool

	// operational indicates this connection was established with operational
	// TLS (zone CA-verified, mutual auth) after commissioning completed.
	operational bool

	// pendingNotifications buffers notifications that were read while
	// waiting for an invoke response (e.g. during sendTriggerViaZone).
	// handleWaitForNotificationAsZone drains this before reading the wire.
	pendingNotifications [][]byte
}

// New creates a new test runner.
func New(config *Config) *Runner {
	// Create engine with config
	engineConfig := engine.DefaultConfig()
	engineConfig.DefaultTimeout = config.Timeout
	engineConfig.SuiteTimeout = config.SuiteTimeout

	// Load PICS if provided
	var pics *loader.PICSFile
	if config.PICSFile != "" {
		var err error
		pics, err = loader.LoadPICS(config.PICSFile)
		if err == nil {
			engineConfig.PICS = pics
		}
	}

	r := &Runner{
		config:          config,
		engine:          engine.NewWithConfig(engineConfig),
		engineConfig:    engineConfig,
		conn:            &Connection{},
		resolver:        NewResolver(),
		activeZoneConns: make(map[string]*Connection),
		activeZoneIDs:   make(map[string]string),
		pics:            pics,
	}

	// Set precondition callback (must be after r is created since it's a method on r).
	// This works because NewWithConfig stores the *EngineConfig pointer.
	engineConfig.SetupPreconditions = r.setupPreconditions

	// Stream each test result as it completes.
	engineConfig.OnTestComplete = func(result *engine.TestResult) {
		r.reporter.ReportTest(result)
	}

	// Register enhanced checkers
	engine.RegisterEnhancedCheckers(r.engine)

	// Create reporter
	switch config.OutputFormat {
	case "json":
		r.reporter = reporter.NewJSONReporter(config.Output, true)
	case "junit":
		r.reporter = reporter.NewJUnitReporter(config.Output)
	default:
		r.reporter = reporter.NewTextReporter(config.Output, config.Verbose)
	}

	// Register action handlers
	r.registerHandlers()

	return r
}

// nextMessageID returns the next message ID.
func (r *Runner) nextMessageID() uint32 {
	return atomic.AddUint32(&r.messageID, 1)
}

// Run executes all matching test cases and returns the suite result.
func (r *Runner) Run(ctx context.Context) (*engine.SuiteResult, error) {
	// Auto-PICS: discover device capabilities before loading tests.
	if r.config.AutoPICS {
		if err := r.runAutoPICS(ctx); err != nil {
			return nil, fmt.Errorf("auto-PICS discovery failed: %w", err)
		}
	}

	// Load test cases
	cases, err := loader.LoadDirectory(r.config.TestDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tests: %w", err)
	}

	// Filter by mode (device/controller) to skip tests for the wrong role.
	if r.config.Mode != "" {
		cases = filterByMode(cases, r.config.Mode)
	}

	// Filter by pattern if provided
	if r.config.Pattern != "" {
		cases = filterByPattern(cases, r.config.Pattern)
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no test cases found matching pattern %q", r.config.Pattern)
	}

	// Sort tests by precondition level to minimize state transitions.
	SortByPreconditionLevel(cases)

	// Run the test suite
	result := r.engine.RunSuite(ctx, cases)
	result.SuiteName = fmt.Sprintf("MASH Conformance Tests (%s)", r.config.Target)

	// Report summary only -- individual tests were already streamed via OnTestComplete.
	r.reporter.ReportSummary(result)

	return result, nil
}

// runAutoPICS commissions the device, reads its capabilities, builds a PICS
// file, and then disconnects so tests can manage their own connections.
func (r *Runner) runAutoPICS(ctx context.Context) error {
	state := engine.NewExecutionState(ctx)

	// Commission the device to establish a session.
	if err := r.ensureCommissioned(ctx, state); err != nil {
		return fmt.Errorf("commissioning for auto-PICS: %w", err)
	}

	pf, err := r.buildAutoPICS(ctx)
	if err != nil {
		r.ensureDisconnected()
		return err
	}

	// Install the discovered PICS.
	r.pics = pf
	r.engineConfig.PICS = pf

	if r.config.Verbose {
		logAutoPICS(pf)
	} else {
		stdlog.Printf("Auto-PICS: discovered %d items from device", len(pf.Items))
	}

	// Force the device into commissioning mode while we still have a live
	// zone session. This handles stale zones from previous test runs:
	// the device persists zone memberships, so on restart it may have
	// zones that sendRemoveZone won't clear (it only removes our zone).
	// Without the trigger, the stale zones keep the device in operational
	// mode, causing PASE failures in subsequent tests.
	if r.config.EnableKey != "" {
		r.debugf("auto-PICS: sending TriggerEnterCommissioningMode on live zone session")
		_, _ = r.sendTrigger(ctx, features.TriggerEnterCommissioningMode, state)
	}

	// Send RemoveZone so the device drops our auto-PICS zone, then
	// disconnect. The trigger above ensures commissioning mode persists
	// even if stale zones remain after our zone is removed.
	r.debugf("auto-PICS: sending RemoveZone and disconnecting")
	r.debugSnapshot("auto-PICS BEFORE cleanup")
	r.sendRemoveZone()
	r.ensureDisconnected()
	r.debugSnapshot("auto-PICS AFTER cleanup")

	// DEC-065: The auto-PICS commissioning triggers a connection cooldown
	// (500ms) on the device. Wait for it to expire so subsequent test steps
	// that attempt commissioning don't hit a stale cooldown rejection.
	const autoPICSCooldownWait = 550 * time.Millisecond
	r.debugf("auto-PICS: waiting %s for connection cooldown to expire", autoPICSCooldownWait)
	time.Sleep(autoPICSCooldownWait)

	return nil
}

// framerSyncConn adapts a transport.Framer to the service.SyncConnection interface.
// This allows the cert exchange protocol (IssueInitialCertSync) to use the
// existing PASE connection's framer for synchronous send/receive.
type framerSyncConn struct {
	framer *transport.Framer
}

// Send writes data as a framed message.
func (f *framerSyncConn) Send(data []byte) error {
	return f.framer.WriteFrame(data)
}

// ReadFrame reads a framed message.
func (f *framerSyncConn) ReadFrame() ([]byte, error) {
	return f.framer.ReadFrame()
}

// operationalTLSConfig builds a TLS config for operational (post-commissioning) connections.
// It uses the Zone CA for chain validation but skips hostname verification since
// MASH identifies peers by device ID in the certificate CN, not DNS hostname.
func (r *Runner) operationalTLSConfig() *tls.Config {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{transport.ALPNProtocol},
		// Explicit curve preferences to match the MASH spec and avoid
		// Go 1.24+ defaulting to post-quantum X25519MLKEM768.
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
	}

	if r.zoneCAPool != nil {
		// Skip hostname verification but validate the cert chain against Zone CA.
		tlsConfig.InsecureSkipVerify = true
		tlsConfig.VerifyPeerCertificate = r.verifyPeerCertAgainstZoneCA
	} else if r.config.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	// Present controller cert for mutual TLS if available.
	if r.controllerCert != nil {
		tlsConfig.Certificates = []tls.Certificate{r.controllerCert.TLSCertificate()}
	}

	return tlsConfig
}

// verifyPeerCertAgainstZoneCA validates the peer certificate chain against
// the Zone CA pool without checking hostname. This is used for operational
// connections where MASH identifies peers by device ID in the cert CN.
func (r *Runner) verifyPeerCertAgainstZoneCA(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("no peer certificates presented")
	}

	// Parse the leaf certificate.
	leaf, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("parse peer certificate: %w", err)
	}

	// Build intermediate pool from any additional certs.
	intermediates := x509.NewCertPool()
	for _, raw := range rawCerts[1:] {
		c, err := x509.ParseCertificate(raw)
		if err != nil {
			continue
		}
		intermediates.AddCert(c)
	}

	// Verify the chain against the Zone CA pool.
	opts := x509.VerifyOptions{
		Roots:         r.zoneCAPool,
		Intermediates: intermediates,
	}

	if _, err := leaf.Verify(opts); err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	return nil
}

// Close cleans up runner resources. It sends RemoveZone on the main
// connection before closing so the device can re-enter commissioning
// mode for the next test run.
func (r *Runner) Close() error {
	r.closeActiveZoneConns()
	if r.conn != nil {
		if r.conn.connected {
			r.sendRemoveZone()
		}
		return r.conn.Close()
	}
	return nil
}

// Close closes the connection. It is idempotent: calling Close on an
// already-closed connection is a no-op.
func (c *Connection) Close() error {
	c.connected = false
	if c.tlsConn != nil {
		err := c.tlsConn.Close()
		c.tlsConn = nil
		c.conn = nil
		c.framer = nil
		return err
	}
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.framer = nil
		return err
	}
	return nil
}

// registerHandlers registers all action handlers with the engine.
func (r *Runner) registerHandlers() {
	// Connection handlers
	r.engine.RegisterHandler("connect", r.handleConnect)
	r.engine.RegisterHandler("disconnect", r.handleDisconnect)

	// Protocol operation handlers
	r.engine.RegisterHandler("read", r.handleRead)
	r.engine.RegisterHandler("write", r.handleWrite)
	r.engine.RegisterHandler("subscribe", r.handleSubscribe)
	r.engine.RegisterHandler("invoke", r.handleInvoke)

	// PASE commissioning handlers
	r.registerPASEHandlers()

	// Utility handlers
	r.engine.RegisterHandler("wait", r.handleWait)
	r.engine.RegisterHandler("verify", r.handleVerify)

	// Register custom expectation checkers
	r.engine.RegisterChecker(CheckerConnectionEstablished, r.checkConnectionEstablished)
	r.engine.RegisterChecker(CheckerResponseReceived, r.checkResponseReceived)

	// Certificate renewal handlers
	r.registerRenewalHandlers()

	// Security testing handlers (DEC-047)
	r.registerSecurityHandlers()

	// TestControl trigger handlers
	r.registerTriggerHandlers()

	// Extended handler groups
	r.registerUtilityHandlers()
	r.registerDiscoveryHandlers()
	r.registerZoneHandlers()
	r.registerDeviceHandlers()
	r.registerControllerHandlers()
	r.registerConnectionHandlers()
	r.registerCertHandlers()
	r.registerNetworkHandlers()
}

// handleConnect establishes a connection to the target.
func (r *Runner) handleConnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Simulate connection failure when device_port_closed is set.
	if portClosed, _ := state.Get(PrecondDevicePortClosed); portClosed == true {
		return map[string]any{
			KeyConnectionEstablished: false,
			KeyConnected:             false,
			KeyTLSHandshakeSuccess:   false,
			KeyError:                 ErrCodeConnectionFailed,
			KeyErrorCode:             ErrCodeConnectionFailed,
			KeyErrorDetail:           "connection refused (simulated)",
		}, nil
	}

	target := r.config.Target
	if t, ok := step.Params["target"].(string); ok && t != "" {
		target = t
	}
	// Also construct from host + port when specified.
	if h, ok := step.Params["host"].(string); ok && h != "" {
		port := 8443
		if p, ok := step.Params["port"]; ok {
			port = int(toFloat(p))
		}
		target = fmt.Sprintf("%s:%d", h, port)
	}

	insecure := r.config.InsecureSkipVerify
	if i, ok := step.Params["insecure"].(bool); ok {
		insecure = i
	}

	// Check if this is a commissioning connection
	commissioning := false
	if c, ok := step.Params[KeyCommissioning].(bool); ok {
		commissioning = c
	}

	// Create TLS config
	var tlsConfig *tls.Config
	if commissioning {
		// Use proper commissioning TLS config for PASE
		tlsConfig = transport.NewCommissioningTLSConfig()
	} else if r.zoneCAPool != nil {
		// Use operational TLS config with Zone CA validation (no hostname check).
		tlsConfig = r.operationalTLSConfig()
	} else {
		// When no Zone CA exists, accept self-signed certs since there's
		// no trusted root to verify against yet.
		skipVerify := true
		if !insecure {
			// Only enforce verification if explicitly set to false AND
			// accept_self_signed is also explicitly false.
			if a, ok := step.Params["accept_self_signed"].(bool); ok && !a {
				skipVerify = false
			}
		}
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: skipVerify,
			NextProtos:         []string{transport.ALPNProtocol},
			CurvePreferences:   []tls.CurveID{tls.X25519, tls.CurveP256},
		}
	}

	// Apply TLS version constraints from params.
	// Accept both "tls_version" (full override) and "tls_max_version" (max only).
	if v, ok := step.Params["tls_version"].(string); ok {
		switch v {
		case "1.2":
			tlsConfig.MinVersion = tls.VersionTLS12
			tlsConfig.MaxVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MinVersion = tls.VersionTLS13
			tlsConfig.MaxVersion = tls.VersionTLS13
		case "1.2,1.3":
			tlsConfig.MinVersion = tls.VersionTLS12
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}
	if v, ok := step.Params["tls_max_version"].(string); ok {
		switch v {
		case "1.2":
			tlsConfig.MaxVersion = tls.VersionTLS12
			if tlsConfig.MinVersion > tls.VersionTLS12 {
				tlsConfig.MinVersion = tls.VersionTLS12
			}
		case "1.3":
			tlsConfig.MaxVersion = tls.VersionTLS13
		}
	}

	// Apply key exchange group constraints from params.
	if groups, ok := step.Params["key_exchange_groups"].([]any); ok && len(groups) > 0 {
		var curvePrefs []tls.CurveID
		for _, g := range groups {
			if name, ok := g.(string); ok {
				if id, found := parseCurveID(name); found {
					curvePrefs = append(curvePrefs, id)
				}
			}
		}
		if len(curvePrefs) > 0 {
			tlsConfig.CurvePreferences = curvePrefs
		}
	}

	// Apply ALPN override from params.
	// When alpn is a non-empty string, use it. When the key is present but
	// null or empty (alpn: null in YAML), clear NextProtos so no ALPN
	// extension is sent. When the key is absent, keep the default.
	if val, hasALPN := step.Params["alpn"]; hasALPN {
		if alpn, ok := val.(string); ok && alpn != "" {
			tlsConfig.NextProtos = []string{alpn}
		} else {
			tlsConfig.NextProtos = nil
		}
	}

	// Apply cipher suite constraints from params.
	// Note: Go's TLS 1.3 implementation ignores CipherSuites -- all built-in
	// TLS 1.3 ciphers are always offered. This only affects TLS 1.2 connections.
	if suites, ok := step.Params["cipher_suites"].([]any); ok && len(suites) > 0 {
		var cipherIDs []uint16
		for _, s := range suites {
			if name, ok := s.(string); ok {
				if id, found := parseCipherSuite(name); found {
					cipherIDs = append(cipherIDs, id)
				}
			}
		}
		if len(cipherIDs) > 0 {
			tlsConfig.CipherSuites = cipherIDs
		}
	}

	// Override client certificate if specified in params.
	// cert_chain and client_cert are mutually exclusive; cert_chain takes priority.
	if chainSpec, ok := step.Params["cert_chain"].([]any); ok && len(chainSpec) > 0 {
		var specs []string
		for _, s := range chainSpec {
			if name, ok := s.(string); ok {
				specs = append(specs, name)
			}
		}
		certChain, chainErr := buildCertChain(specs, r.controllerCert, r.zoneCA)
		if chainErr != nil {
			return nil, fmt.Errorf("building cert chain: %w", chainErr)
		}
		tlsConfig.Certificates = []tls.Certificate{certChain}
	} else if clientCertType, ok := step.Params["client_cert"].(string); ok && clientCertType != "" {
		switch clientCertType {
		case "controller_operational":
			// Use default r.controllerCert -- already set by operationalTLSConfig.
		case "none":
			tlsConfig.Certificates = nil
		default:
			if r.zoneCA == nil {
				return nil, fmt.Errorf("client_cert %q requires a zone CA (commission first)", clientCertType)
			}
			testCert, certErr := generateTestClientCert(clientCertType, r.zoneCA)
			if certErr != nil {
				return nil, fmt.Errorf("generating test client cert %q: %w", clientCertType, certErr)
			}
			tlsConfig.Certificates = []tls.Certificate{testCert}
		}
	}

	// Record start time for verify_timing.
	startTime := time.Now()
	state.Set("start_time", startTime)

	// Connect with timeout
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", target, tlsConfig)
	if err != nil {
		state.Set("end_time", time.Now())
		errorCode := classifyConnectError(err)
		// Return connection failure as outputs so tests can assert on TLS errors.
		return map[string]any{
			KeyConnectionEstablished: false,
			KeyConnected:             false,
			KeyTLSHandshakeSuccess:   false,
			KeyTLSHandshakeFailed:    true,
			KeyTarget:                target,
			KeyError:                 errorCode,
			KeyErrorCode:             errorCode,
			KeyErrorDetail:           err.Error(),
			KeyTLSError:              err.Error(),
			KeyTLSAlert:              extractTLSAlert(err),
		}, nil
	}

	// Record end time for verify_timing.
	state.Set("end_time", time.Now())

	r.conn.tlsConn = conn
	r.conn.framer = transport.NewFramer(conn)
	r.conn.connected = true

	// Set up protocol logging if configured
	if r.config.ProtocolLogger != nil {
		connID := generateConnectionID()
		r.conn.framer.SetLogger(r.config.ProtocolLogger, connID)
	}

	// Store connection info in state
	state.Set(StateConnection, r.conn)

	// Extract TLS connection details.
	cs := conn.ConnectionState()
	hasPeerCerts := len(cs.PeerCertificates) > 0

	// Chain validation: either Go's standard path populated VerifiedChains,
	// or our custom VerifyPeerCertificate callback validated the chain
	// (when InsecureSkipVerify is true, Go doesn't populate VerifiedChains).
	chainValidated := len(cs.VerifiedChains) > 0 || (hasPeerCerts && tlsConfig.InsecureSkipVerify && tlsConfig.VerifyPeerCertificate != nil)

	// Mutual auth: we presented a client cert and the connection succeeded.
	// If the device requires client auth (RequireAndVerifyClientCert), a
	// rejected client cert would have caused a handshake failure above.
	presentedClientCert := len(tlsConfig.Certificates) > 0
	mutualAuth := presentedClientCert && hasPeerCerts

	// Server cert details.
	serverCertCNPrefix := ""
	serverCertCN := ""
	serverCertSelfSigned := false
	if hasPeerCerts {
		cert := cs.PeerCertificates[0]
		serverCertCN = cert.Subject.CommonName
		// Extract prefix: everything up to and including the first hyphen.
		// E.g., "MASH-1234" -> "MASH-", "no-hyphen" -> "no-hyphen".
		if idx := strings.Index(cert.Subject.CommonName, "-"); idx >= 0 {
			serverCertCNPrefix = cert.Subject.CommonName[:idx+1]
		} else {
			serverCertCNPrefix = cert.Subject.CommonName
		}
		// Self-signed: issuer and subject are identical in the raw ASN.1.
		serverCertSelfSigned = bytes.Equal(cert.RawIssuer, cert.RawSubject)
	}

	// Check for CN/discriminator mismatch when expected_discriminator is provided.
	cnMismatchWarning := false
	if expectedDisc, ok := step.Params["expected_discriminator"]; ok && hasPeerCerts {
		expectedCN := fmt.Sprintf("MASH-%d", int(toFloat(expectedDisc)))
		if serverCertCN != expectedCN {
			cnMismatchWarning = true
		}
	}

	return map[string]any{
		KeyConnectionEstablished:  true,
		KeyConnected:              true,
		KeyTLSHandshakeSuccess:    true,
		KeyTarget:                 target,
		KeyTLSVersion:             tlsVersionName(cs.Version),
		KeyNegotiatedVersion:      tlsVersionName(cs.Version),
		KeyNegotiatedCipher:       tls.CipherSuiteName(cs.CipherSuite),
		KeyNegotiatedGroup:        curveIDName(cs.CurveID),
		KeyNegotiatedProtocol:     cs.NegotiatedProtocol,
		KeyNegotiatedALPN:         cs.NegotiatedProtocol,
		KeyMutualAuth:             mutualAuth,
		KeyControllerCertVerified: presentedClientCert && chainValidated,
		KeyChainValidated:         chainValidated,
		KeySelfSignedAccepted:     serverCertSelfSigned && hasPeerCerts,
		KeyServerCertCNPrefix:     serverCertCNPrefix,
		KeyServerCertSelfSigned:   serverCertSelfSigned,
		KeyHasPeerCerts:           hasPeerCerts,
		KeyCNMismatchWarning:      cnMismatchWarning,
		KeyFullHandshake:          !cs.DidResume,
		KeyPSKUsed:                cs.DidResume,
		KeyEarlyDataAccepted:      false, // Go does not support TLS 1.3 0-RTT
	}, nil
}

// handleDisconnect closes the connection.
func (r *Runner) handleDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyDisconnected: true, KeyConnectionClosed: true}, nil
	}

	err := r.conn.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to disconnect: %w", err)
	}

	return map[string]any{KeyDisconnected: true, KeyConnectionClosed: true}, nil
}

// sendRequest sends an encoded request and reads the decoded response.
// On IO errors (send/receive), the connection is marked as dead so that
// subsequent tests will reconnect instead of reusing a broken connection.
func (r *Runner) sendRequest(data []byte, op string) (*wire.Response, error) {
	if err := r.conn.framer.WriteFrame(data); err != nil {
		r.conn.connected = false
		return nil, fmt.Errorf("failed to send %s request: %w", op, err)
	}

	respData, err := r.conn.framer.ReadFrame()
	if err != nil {
		r.conn.connected = false
		return nil, fmt.Errorf("failed to read %s response: %w", op, err)
	}

	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode %s response: %w", op, err)
	}

	return resp, nil
}

// handleRead sends a read request and returns the response.
func (r *Runner) handleRead(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint, feature, and attribute names
	endpointID, err := r.resolver.ResolveEndpoint(params[KeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params[KeyFeature])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	attribute, _ := params["attribute"].(string)

	// Resolve attribute name to numeric ID for filtered reads.
	var attrID uint16
	var hasAttr bool
	if attribute != "" {
		attrID, err = r.resolver.ResolveAttribute(params[KeyFeature], attribute)
		if err != nil {
			return nil, fmt.Errorf("resolving attribute: %w", err)
		}
		hasAttr = true
	}

	// Create read request
	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpRead,
		EndpointID: endpointID,
		FeatureID:  featureID,
	}

	// RC5b: Override MessageID if specified in params.
	if mid, ok := params["message_id"]; ok {
		req.MessageID = uint32(toFloat(mid))
	}

	// When a specific attribute is requested, include it in the read payload
	// so the device can return just that attribute.
	if hasAttr {
		req.Payload = &wire.ReadPayload{AttributeIDs: []uint16{attrID}}
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		// RC5b: MessageID=0 is rejected by EncodeRequest (wire validation).
		// Return as output, not a Go error, so YAML can assert on it.
		return map[string]any{
			KeyReadSuccess:    false,
			KeyConnectionError: true,
			KeyError:          err.Error(),
		}, nil
	}

	// RC5c: simulate_no_response -- send request but don't wait for response.
	if toBool(params["simulate_no_response"]) {
		if sendErr := r.conn.framer.WriteFrame(data); sendErr != nil {
			return map[string]any{
				KeyReadSuccess: false,
				KeyError:       sendErr.Error(),
			}, nil
		}
		// Wait for context cancellation (step timeout).
		<-ctx.Done()
		return map[string]any{
			KeyReadSuccess:  false,
			KeyError:        "REQUEST_TIMEOUT",
			KeyTimeoutAfter: "10s",
		}, nil
	}

	resp, err := r.sendRequest(data, "read")
	if err != nil {
		return nil, err
	}

	outputs := map[string]any{
		KeyReadSuccess:       resp.IsSuccess(),
		KeyResponse:          resp,
		KeyValue:             resp.Payload,
		KeyStatus:            resp.Status,
		KeyResponseMessageID: resp.MessageID,
	}

	// Add error code when the response indicates failure.
	if !resp.IsSuccess() {
		outputs[KeyErrorCode] = resp.Status.String()
	}

	// When a specific attribute was requested, extract its value from the
	// response payload map. The payload is typically map[any]any with
	// uint64 keys after CBOR decoding.
	if hasAttr && resp.Payload != nil {
		if extracted, ok := extractAttributeValue(resp.Payload, attrID); ok {
			outputs[KeyValue] = extracted
		}
		outputs[attribute+"_present"] = resp.Payload != nil
	}

	return outputs, nil
}

// extractAttributeValue extracts a single attribute value from a read response
// payload map. CBOR decoding may produce map[any]any with uint64 keys.
func extractAttributeValue(payload any, attrID uint16) (any, bool) {
	switch m := payload.(type) {
	case map[any]any:
		// CBOR-decoded map with integer keys (most common after wire round-trip)
		if v, ok := m[uint64(attrID)]; ok {
			return v, true
		}
		// Try other integer types that CBOR decoders may produce
		if v, ok := m[int64(attrID)]; ok {
			return v, true
		}
		if v, ok := m[int(attrID)]; ok {
			return v, true
		}
	case map[uint16]any:
		if v, ok := m[attrID]; ok {
			return v, true
		}
	case map[string]any:
		// String-keyed map (less common, but possible)
		if v, ok := m[fmt.Sprintf("%d", attrID)]; ok {
			return v, true
		}
	}
	return nil, false
}

// handleWrite sends a write request.
func (r *Runner) handleWrite(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint and feature names
	endpointID, err := r.resolver.ResolveEndpoint(params[KeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params[KeyFeature])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	value := params[KeyValue]

	// Build the wire payload. When an attribute name is given, wrap the
	// value in a WritePayload map (map[uint16]any{attrID: value}) so the
	// device receives the correct CBOR-encoded structure.
	var payload any = value
	if attribute, ok := params["attribute"].(string); ok && attribute != "" {
		attrID, attrErr := r.resolver.ResolveAttribute(params[KeyFeature], attribute)
		if attrErr != nil {
			return nil, fmt.Errorf("resolving attribute: %w", attrErr)
		}
		payload = wire.WritePayload{attrID: value}
	}

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpWrite,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Payload:    payload,
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode write request: %w", err)
	}

	resp, err := r.sendRequest(data, "write")
	if err != nil {
		// RC5f: Return output map (not Go error) for EOF/closed so YAML
		// expectations can check the error.
		errMsg := err.Error()
		if strings.Contains(errMsg, "EOF") || strings.Contains(errMsg, "closed") ||
			strings.Contains(errMsg, "reset") {
			return map[string]any{
				KeyWriteSuccess: false,
				KeyError:        errMsg,
			}, nil
		}
		return nil, err
	}

	return map[string]any{
		KeyWriteSuccess: resp.IsSuccess(),
		KeyResponse:     resp,
		KeyStatus:       resp.Status,
	}, nil
}

// handleSubscribe sends a subscribe request.
func (r *Runner) handleSubscribe(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint and feature names
	endpointID, err := r.resolver.ResolveEndpoint(params[KeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params[KeyFeature])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpSubscribe,
		EndpointID: endpointID,
		FeatureID:  featureID,
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode subscribe request: %w", err)
	}

	resp, err := r.sendRequest(data, "subscribe")
	if err != nil {
		return nil, err
	}

	// Extract subscription ID and priming data from the response payload.
	// The subscribe response payload is {1: subscriptionId, 2: currentValues}
	// (a SubscribeResponsePayload). The raw CBOR decodes as map[any]any.
	subscriptionID := extractSubscriptionID(resp.Payload)
	primingData := extractPrimingData(resp.Payload)

	// Store priming data in state so wait_report can use it as an initial
	// report if no subsequent notification frame arrives.
	if primingData != nil {
		state.Set("_priming_data", primingData)
	}

	return map[string]any{
		KeySubscribeSuccess:       resp.IsSuccess(),
		KeySubscriptionID:         subscriptionID,
		KeySubscriptionIDReturned: subscriptionID != nil,
		KeyStatus:                 resp.Status,
	}, nil
}

// extractSubscriptionID extracts the subscription ID from a subscribe response
// payload. The payload is typically a map {1: subscriptionId, 2: currentValues}
// decoded from CBOR as map[any]any.
func extractSubscriptionID(payload any) any {
	switch p := payload.(type) {
	case map[any]any:
		if id, ok := p[uint64(1)]; ok {
			return id
		}
	}
	// Fallback: return the payload itself (may be a simple ID).
	return payload
}

// extractPrimingData extracts the current values (priming report) from a
// subscribe response payload. Key 2 in the SubscribeResponsePayload.
func extractPrimingData(payload any) any {
	switch p := payload.(type) {
	case map[any]any:
		if vals, ok := p[uint64(2)]; ok {
			return vals
		}
	}
	return nil
}

// handleInvoke sends an invoke request.
func (r *Runner) handleInvoke(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint and feature names
	endpointID, err := r.resolver.ResolveEndpoint(params[KeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params[KeyFeature])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	// Resolve command name to ID and wrap in InvokePayload.
	var payload any
	if commandRaw, hasCommand := params["command"]; hasCommand {
		commandID, cmdErr := r.resolver.ResolveCommand(params[KeyFeature], commandRaw)
		if cmdErr != nil {
			return nil, fmt.Errorf("resolving command: %w", cmdErr)
		}
		// Read args (primary) or params (fallback) for command parameters.
		args, _ := params["args"]
		if args == nil {
			args, _ = params["params"]
		}
		payload = &wire.InvokePayload{
			CommandID:  commandID,
			Parameters: args,
		}
	} else {
		// Legacy: raw payload passthrough (no command field).
		payload = params["params"]
	}

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Payload:    payload,
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode invoke request: %w", err)
	}

	resp, err := r.sendRequest(data, "invoke")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		KeyInvokeSuccess: resp.IsSuccess(),
		KeyResult:        resp.Payload,
		KeyStatus:        resp.Status,
	}, nil
}

// Utility handlers
func (r *Runner) handleWait(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	durationMs := paramFloat(step.Params, "duration_ms", 0)
	if durationMs <= 0 {
		// Also accept duration_seconds (used by commissioning window tests).
		if sec := paramFloat(step.Params, "duration_seconds", 0); sec > 0 {
			durationMs = sec * 1000
		} else {
			durationMs = 1000
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(durationMs) * time.Millisecond):
	}

	return map[string]any{KeyWaited: true}, nil
}

func (r *Runner) handleVerify(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Verify action just passes through expectations
	return map[string]any{}, nil
}

// Custom expectation checkers
func (r *Runner) checkConnectionEstablished(key string, expected any, state *engine.ExecutionState) *engine.ExpectResult {
	actual, exists := state.Get(KeyConnectionEstablished)
	if !exists {
		return &engine.ExpectResult{
			Key:      key,
			Expected: expected,
			Actual:   nil,
			Passed:   false,
			Message:  "connection not established",
		}
	}

	passed := actual == expected
	return &engine.ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   passed,
		Message:  fmt.Sprintf("connection_established = %v", actual),
	}
}

func (r *Runner) checkResponseReceived(key string, expected any, state *engine.ExecutionState) *engine.ExpectResult {
	actual, exists := state.Get(KeyResponse)
	return &engine.ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   actual,
		Passed:   exists,
		Message:  fmt.Sprintf("response received: %v", exists),
	}
}

// filterByPattern filters test cases by name pattern.
func filterByPattern(cases []*loader.TestCase, pattern string) []*loader.TestCase {
	var filtered []*loader.TestCase
	for _, tc := range cases {
		if matchPattern(tc.ID, pattern) || matchPattern(tc.Name, pattern) {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// filterByMode filters test cases based on the runner mode (device/controller).
// In device mode, tests requiring controller-only PICS codes (MASH.C.* or C.*) are skipped.
// In controller mode, tests requiring device-only PICS codes (MASH.S.* or D.*) are skipped.
func filterByMode(cases []*loader.TestCase, mode string) []*loader.TestCase {
	var skipPrefixes []string
	switch mode {
	case "device":
		skipPrefixes = []string{"MASH.C.", "C."}
	case "controller":
		skipPrefixes = []string{"MASH.S.", "D."}
	default:
		return cases
	}

	var filtered []*loader.TestCase
	for _, tc := range cases {
		skip := false
		for _, req := range tc.PICSRequirements {
			for _, prefix := range skipPrefixes {
				if strings.HasPrefix(req, prefix) {
					skip = true
					break
				}
			}
			if skip {
				break
			}
		}
		if !skip {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// matchPattern performs simple glob matching.
func matchPattern(name, pattern string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}

	// Check for * at start and end
	hasPrefix := len(pattern) > 0 && pattern[0] == '*'
	hasSuffix := len(pattern) > 0 && pattern[len(pattern)-1] == '*'

	if hasPrefix && hasSuffix && len(pattern) > 2 {
		// *foo* - contains
		return contains(name, pattern[1:len(pattern)-1])
	}
	if hasPrefix {
		// *foo - suffix match
		return hasSuffixStr(name, pattern[1:])
	}
	if hasSuffix {
		// foo* - prefix match
		return hasPrefixStr(name, pattern[:len(pattern)-1])
	}

	return name == pattern
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func hasPrefixStr(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffixStr(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// generateConnectionID generates a unique connection ID for logging.
func generateConnectionID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// tlsVersionName returns a short TLS version string matching test expectations.
func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS10:
		return "1.0"
	default:
		return fmt.Sprintf("unknown (0x%04x)", version)
	}
}

// curveIDName returns a human-readable name for a TLS curve ID,
// using names that match the YAML test expectations.
func curveIDName(id tls.CurveID) string {
	switch id {
	case tls.CurveP256:
		return "secp256r1"
	case tls.CurveP384:
		return "secp384r1"
	case tls.CurveP521:
		return "secp521r1"
	case tls.X25519:
		return "x25519"
	case 4588: // X25519MLKEM768 -- post-quantum hybrid (Go 1.24+ default)
		return "X25519MLKEM768"
	default:
		return fmt.Sprintf("unknown (%d)", id)
	}
}

// parseCurveID maps a curve name string (from YAML) to a tls.CurveID.
func parseCurveID(name string) (tls.CurveID, bool) {
	switch strings.ToLower(name) {
	case "secp256r1", "p-256", "p256":
		return tls.CurveP256, true
	case "secp384r1", "p-384", "p384":
		return tls.CurveP384, true
	case "secp521r1", "p-521", "p521":
		return tls.CurveP521, true
	case "x25519":
		return tls.X25519, true
	default:
		return 0, false
	}
}

// parseCipherSuite maps a cipher suite name string (from YAML) to its TLS ID.
func parseCipherSuite(name string) (uint16, bool) {
	switch name {
	case "TLS_AES_128_GCM_SHA256":
		return tls.TLS_AES_128_GCM_SHA256, true
	case "TLS_AES_256_GCM_SHA384":
		return tls.TLS_AES_256_GCM_SHA384, true
	case "TLS_CHACHA20_POLY1305_SHA256":
		return tls.TLS_CHACHA20_POLY1305_SHA256, true
	default:
		return 0, false
	}
}

// extractTLSAlert extracts a TLS alert description from an error.
func extractTLSAlert(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Go TLS errors contain alert descriptions like "bad certificate",
	// "certificate expired", "unknown authority", etc.
	alertKeywords := []string{
		"bad certificate", "certificate expired", "unknown authority",
		"handshake failure", "protocol version", "no application protocol",
		"certificate unknown", "decrypt error", "illegal parameter",
		"close notify",
	}
	for _, kw := range alertKeywords {
		if contains(msg, kw) {
			return strings.ReplaceAll(kw, " ", "_")
		}
	}
	// Return the full error as fallback.
	return msg
}

// classifyConnectError returns a short error code for connection failures.
func classifyConnectError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") ||
		strings.Contains(msg, "unreachable"):
		return ErrCodeTimeout
	case strings.Contains(msg, "connection refused"):
		return ErrCodeConnectionFailed
	case strings.Contains(msg, "tls") || strings.Contains(msg, "certificate"):
		return ErrCodeTLSError
	default:
		return ErrCodeConnectionError
	}
}

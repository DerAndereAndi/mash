// Package runner provides test execution against real MASH devices/controllers.
package runner

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/internal/testharness/reporter"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Runner executes test cases against a target device or controller.
type Runner struct {
	config    *Config
	engine    *engine.Engine
	reporter  reporter.Reporter
	conn      *Connection
	resolver  *Resolver
	messageID uint32 // Atomic counter for message IDs
	paseState *PASEState
	pics      *loader.PICSFile // Cached PICS for handler access
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

	// Verbose enables verbose output.
	Verbose bool

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
}

// New creates a new test runner.
func New(config *Config) *Runner {
	// Create engine with config
	engineConfig := engine.DefaultConfig()
	engineConfig.DefaultTimeout = config.Timeout

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
		config:   config,
		engine:   engine.NewWithConfig(engineConfig),
		conn:     &Connection{},
		resolver: NewResolver(),
		pics:     pics,
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
	// Load test cases
	cases, err := loader.LoadDirectory(r.config.TestDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tests: %w", err)
	}

	// Filter by pattern if provided
	if r.config.Pattern != "" {
		cases = filterByPattern(cases, r.config.Pattern)
	}

	if len(cases) == 0 {
		return nil, fmt.Errorf("no test cases found matching pattern %q", r.config.Pattern)
	}

	// Run the test suite
	result := r.engine.RunSuite(ctx, cases)
	result.SuiteName = fmt.Sprintf("MASH Conformance Tests (%s)", r.config.Target)

	// Report results
	r.reporter.ReportSuite(result)

	return result, nil
}

// Close cleans up runner resources.
func (r *Runner) Close() error {
	if r.conn != nil && r.conn.connected {
		return r.conn.Close()
	}
	return nil
}

// Close closes the connection.
func (c *Connection) Close() error {
	c.connected = false
	if c.tlsConn != nil {
		return c.tlsConn.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
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
	r.engine.RegisterChecker("connection_established", r.checkConnectionEstablished)
	r.engine.RegisterChecker("response_received", r.checkResponseReceived)

	// Certificate renewal handlers
	r.registerRenewalHandlers()

	// Security testing handlers (DEC-047)
	r.registerSecurityHandlers()

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
	target := r.config.Target
	if t, ok := step.Params["target"].(string); ok && t != "" {
		target = t
	}

	insecure := r.config.InsecureSkipVerify
	if i, ok := step.Params["insecure"].(bool); ok {
		insecure = i
	}

	// Check if this is a commissioning connection
	commissioning := false
	if c, ok := step.Params["commissioning"].(bool); ok {
		commissioning = c
	}

	// Create TLS config
	var tlsConfig *tls.Config
	if commissioning {
		// Use proper commissioning TLS config for PASE
		tlsConfig = transport.NewCommissioningTLSConfig()
	} else {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: insecure,
			NextProtos:         []string{transport.ALPNProtocol},
		}
	}

	// Connect with timeout
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", target, tlsConfig)
	if err != nil {
		// Return connection failure as outputs so tests can assert on TLS errors.
		return map[string]any{
			"connection_established": false,
			"connected":              false,
			"tls_handshake_success":  false,
			"target":                 target,
			"error":                  err.Error(),
			"tls_error":              err.Error(),
			"tls_alert":              extractTLSAlert(err),
		}, nil
	}

	r.conn.tlsConn = conn
	r.conn.framer = transport.NewFramer(conn)
	r.conn.connected = true

	// Set up protocol logging if configured
	if r.config.ProtocolLogger != nil {
		connID := generateConnectionID()
		r.conn.framer.SetLogger(r.config.ProtocolLogger, connID)
	}

	// Store connection info in state
	state.Set("connection", r.conn)

	// Extract TLS connection details.
	cs := conn.ConnectionState()
	hasPeerCerts := len(cs.PeerCertificates) > 0
	mutualAuth := hasPeerCerts && len(cs.VerifiedChains) > 0
	chainValidated := len(cs.VerifiedChains) > 0

	// Server cert details.
	serverCertCNPrefix := ""
	serverCertSelfSigned := false
	if hasPeerCerts {
		cert := cs.PeerCertificates[0]
		serverCertCNPrefix = cert.Subject.CommonName
		serverCertSelfSigned = cert.IsCA || cert.Issuer.CommonName == cert.Subject.CommonName
	}

	return map[string]any{
		"connection_established": true,
		"connected":              true,
		"tls_handshake_success":  true,
		"target":                 target,
		"tls_version":            cs.Version,
		"negotiated_version":     tlsVersionName(cs.Version),
		"negotiated_cipher":      tls.CipherSuiteName(cs.CipherSuite),
		"negotiated_group":       curveIDName(cs.CurveID),
		"negotiated_protocol":    cs.NegotiatedProtocol,
		"negotiated_alpn":        cs.NegotiatedProtocol,
		"mutual_auth":            mutualAuth,
		"chain_validated":        chainValidated,
		"self_signed_accepted":   serverCertSelfSigned && hasPeerCerts,
		"server_cert_cn_prefix":  serverCertCNPrefix,
		"server_cert_self_signed": serverCertSelfSigned,
		"has_peer_certs":         hasPeerCerts,
	}, nil
}

// handleDisconnect closes the connection.
func (r *Runner) handleDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{"disconnected": true}, nil
	}

	err := r.conn.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to disconnect: %w", err)
	}

	return map[string]any{"disconnected": true}, nil
}

// handleRead sends a read request and returns the response.
func (r *Runner) handleRead(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint, feature, and attribute names
	endpointID, err := r.resolver.ResolveEndpoint(params["endpoint"])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params["feature"])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	attribute, _ := params["attribute"].(string)

	// Create read request
	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpRead,
		EndpointID: endpointID,
		FeatureID:  featureID,
	}

	// Encode and send
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode read request: %w", err)
	}

	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("failed to send read request: %w", err)
	}

	// Read response
	respData, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	outputs := map[string]any{
		"read_success": resp.IsSuccess(),
		"response":     resp,
		"value":        resp.Payload,
		"status":       resp.Status,
	}

	// Add attribute-specific outputs
	if attribute != "" {
		outputs[attribute+"_present"] = resp.Payload != nil
	}

	return outputs, nil
}

// handleWrite sends a write request.
func (r *Runner) handleWrite(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint and feature names
	endpointID, err := r.resolver.ResolveEndpoint(params["endpoint"])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params["feature"])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	value := params["value"]

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpWrite,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Payload:    value,
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode write request: %w", err)
	}

	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("failed to send write request: %w", err)
	}

	// Read response
	respData, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]any{
		"write_success": resp.IsSuccess(),
		"response":      resp,
		"status":        resp.Status,
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
	endpointID, err := r.resolver.ResolveEndpoint(params["endpoint"])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params["feature"])
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

	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("failed to send subscribe request: %w", err)
	}

	respData, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]any{
		"subscribe_success": resp.IsSuccess(),
		"subscription_id":   resp.Payload,
		"status":            resp.Status,
	}, nil
}

// handleInvoke sends an invoke request.
func (r *Runner) handleInvoke(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Interpolate parameters
	params := engine.InterpolateParams(step.Params, state)

	// Resolve endpoint and feature names
	endpointID, err := r.resolver.ResolveEndpoint(params["endpoint"])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}

	featureID, err := r.resolver.ResolveFeature(params["feature"])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	invokeParams := params["params"]

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Payload:    invokeParams,
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode invoke request: %w", err)
	}

	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("failed to send invoke request: %w", err)
	}

	respData, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return map[string]any{
		"invoke_success": resp.IsSuccess(),
		"result":         resp.Payload,
		"status":         resp.Status,
	}, nil
}


// Utility handlers
func (r *Runner) handleWait(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	durationMs, _ := step.Params["duration_ms"].(float64)
	if durationMs <= 0 {
		durationMs = 1000
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(durationMs) * time.Millisecond):
	}

	return map[string]any{"waited": true}, nil
}

func (r *Runner) handleVerify(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Verify action just passes through expectations
	return map[string]any{}, nil
}

// Custom expectation checkers
func (r *Runner) checkConnectionEstablished(key string, expected any, state *engine.ExecutionState) *engine.ExpectResult {
	actual, exists := state.Get("connection_established")
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
	actual, exists := state.Get("response")
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

// tlsVersionName returns a human-readable TLS version string.
func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("unknown (0x%04x)", version)
	}
}

// curveIDName returns a human-readable name for a TLS curve ID.
func curveIDName(id tls.CurveID) string {
	switch id {
	case tls.CurveP256:
		return "P-256"
	case tls.CurveP384:
		return "P-384"
	case tls.CurveP521:
		return "P-521"
	case tls.X25519:
		return "X25519"
	default:
		return fmt.Sprintf("unknown (%d)", id)
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
			return kw
		}
	}
	// Return the full error as fallback.
	return msg
}

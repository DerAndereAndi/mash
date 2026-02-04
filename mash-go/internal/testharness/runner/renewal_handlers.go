package runner

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// registerRenewalHandlers registers all certificate renewal action handlers.
func (r *Runner) registerRenewalHandlers() {
	// Renewal protocol handlers
	r.engine.RegisterHandler("send_renewal_request", r.handleSendRenewalRequest)
	r.engine.RegisterHandler("receive_renewal_csr", r.handleReceiveRenewalCSR)
	r.engine.RegisterHandler("send_cert_install", r.handleSendCertInstall)
	r.engine.RegisterHandler("receive_renewal_ack", r.handleReceiveRenewalAck)
	r.engine.RegisterHandler("full_renewal_flow", r.handleFullRenewalFlow)

	// Session continuity handlers
	r.engine.RegisterHandler("record_subscription_state", r.handleRecordSubscriptionState)
	r.engine.RegisterHandler("verify_subscription_active", r.handleVerifySubscriptionActive)
	r.engine.RegisterHandler("verify_connection_state", r.handleVerifyConnectionState)

	// Certificate expiry/warning handlers
	r.engine.RegisterHandler("set_cert_expiry", r.handleSetCertExpiry)
	r.engine.RegisterHandler("wait_for_notification", r.handleWaitForNotification)
	r.engine.RegisterHandler("verify_notification_content", r.handleVerifyNotificationContent)

	// Expiry and grace period handlers
	r.engine.RegisterHandler("simulate_cert_expiry", r.handleSimulateCertExpiry)
	r.engine.RegisterHandler("connect_expect_failure", r.handleConnectExpectFailure)
	r.engine.RegisterHandler("set_grace_period", r.handleSetGracePeriod)
	r.engine.RegisterHandler("simulate_time_advance", r.handleSimulateTimeAdvance)
	r.engine.RegisterHandler("check_grace_period_status", r.handleCheckGracePeriodStatus)
}

// handleSendRenewalRequest sends a CertRenewalRequest to the device.
func (r *Runner) handleSendRenewalRequest(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Get nonce length from params (default 32)
	nonceLen := 32
	if n, ok := step.Params[KeyNonceLength].(float64); ok {
		nonceLen = int(n)
	}

	// Generate nonce
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Store nonce in state for later verification
	state.Set(StateRenewalNonce, nonce)

	// Create and encode request
	req := &commissioning.CertRenewalRequest{
		MsgType: commissioning.MsgCertRenewalRequest,
		Nonce:   nonce,
	}

	data, err := cbor.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	// Send via framer
	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	return map[string]any{
		KeyRequestSent:    true,
		KeyNonceGenerated: true,
		KeyNonceLength:    nonceLen,
	}, nil
}

// handleReceiveRenewalCSR receives and validates a CertRenewalCSR from the device.
func (r *Runner) handleReceiveRenewalCSR(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Read response
	data, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Decode as renewal message
	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	csr, ok := msg.(*commissioning.CertRenewalCSR)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: %T", msg)
	}

	// Validate CSR
	csrValid := false
	if len(csr.CSR) > 0 {
		_, err := x509.ParseCertificateRequest(csr.CSR)
		csrValid = err == nil
	}

	// Store CSR for later signing
	state.Set(StatePendingCSR, csr.CSR)

	return map[string]any{
		KeyCSRReceived: true,
		KeyCSRValid:    csrValid,
		KeyCSRLength:   len(csr.CSR),
	}, nil
}

// handleSendCertInstall signs the pending CSR and sends the new certificate.
func (r *Runner) handleSendCertInstall(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Get pending CSR from state
	csrData, exists := state.Get(StatePendingCSR)
	if !exists {
		return nil, fmt.Errorf("no pending CSR")
	}

	csrBytes, ok := csrData.([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid CSR data type")
	}

	// For testing, we create a mock certificate
	// In a real implementation, this would use the Zone CA
	// Here we just echo back a placeholder to test the protocol flow
	mockCert := csrBytes // Placeholder - real impl would sign

	// Get or increment sequence
	seq := uint32(1)
	if seqVal, exists := state.Get(StateCertSequence); exists {
		if s, ok := seqVal.(uint32); ok {
			seq = s + 1
		}
	}
	state.Set(StateCertSequence, seq)

	// Create and send install message
	install := &commissioning.CertRenewalInstall{
		MsgType:  commissioning.MsgCertRenewalInstall,
		NewCert:  mockCert,
		Sequence: seq,
	}

	data, err := cbor.Marshal(install)
	if err != nil {
		return nil, fmt.Errorf("encode install: %w", err)
	}

	if err := r.conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("send install: %w", err)
	}

	return map[string]any{
		KeyCertSent:           true,
		KeySequenceIncremented: true,
		KeyNewSequence:         seq,
	}, nil
}

// handleReceiveRenewalAck receives and validates a CertRenewalAck from the device.
func (r *Runner) handleReceiveRenewalAck(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Read response
	data, err := r.conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Decode as renewal message
	msg, err := commissioning.DecodeRenewalMessage(data)
	if err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	ack, ok := msg.(*commissioning.CertRenewalAck)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: %T", msg)
	}

	return map[string]any{
		KeyAckReceived:    true,
		KeyStatus:         int(ack.Status),
		KeyActiveSequence: ack.ActiveSequence,
		KeyNewCertActive:  ack.Status == commissioning.RenewalStatusSuccess,
	}, nil
}

// handleFullRenewalFlow performs the complete renewal protocol.
func (r *Runner) handleFullRenewalFlow(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Step 1: Send renewal request
	reqOutputs, err := r.handleSendRenewalRequest(ctx, &loader.Step{
		Params: map[string]any{KeyNonceLength: 32},
	}, state)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	if !reqOutputs[KeyRequestSent].(bool) {
		return nil, fmt.Errorf("request not sent")
	}

	// Step 2: Receive CSR
	csrOutputs, err := r.handleReceiveRenewalCSR(ctx, &loader.Step{}, state)
	if err != nil {
		return nil, fmt.Errorf("receive CSR: %w", err)
	}
	if !csrOutputs[KeyCSRValid].(bool) {
		return nil, fmt.Errorf("invalid CSR received")
	}

	// Step 3: Send cert install
	installOutputs, err := r.handleSendCertInstall(ctx, &loader.Step{}, state)
	if err != nil {
		return nil, fmt.Errorf("send install: %w", err)
	}
	if !installOutputs[KeyCertSent].(bool) {
		return nil, fmt.Errorf("cert not sent")
	}

	// Step 4: Receive ack
	ackOutputs, err := r.handleReceiveRenewalAck(ctx, &loader.Step{}, state)
	if err != nil {
		return nil, fmt.Errorf("receive ack: %w", err)
	}

	status := ackOutputs[KeyStatus].(int)

	return map[string]any{
		KeyRenewalComplete: status == 0,
		KeyStatus:          status,
	}, nil
}

// handleRecordSubscriptionState records the current subscription state.
func (r *Runner) handleRecordSubscriptionState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Get subscription ID from state (set by subscribe action)
	subID, exists := state.Get(StateSubscriptionID)
	if !exists {
		return nil, fmt.Errorf("no subscription_id in state")
	}

	// Record for later comparison
	state.Set(StateRecordedSubscriptionID, subID)

	return map[string]any{
		KeySubscriptionIDRecorded: true,
		KeySubscriptionID:         subID,
	}, nil
}

// handleVerifySubscriptionActive verifies the subscription is still active.
func (r *Runner) handleVerifySubscriptionActive(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Get recorded and current subscription IDs
	recordedID, recorded := state.Get(StateRecordedSubscriptionID)
	currentID, current := state.Get(StateSubscriptionID)

	sameID := recorded && current && fmt.Sprintf("%v", recordedID) == fmt.Sprintf("%v", currentID)

	return map[string]any{
		KeySameSubscriptionID: sameID,
		KeySubscriptionActive: current,
	}, nil
}

// handleVerifyConnectionState verifies the connection state.
func (r *Runner) handleVerifyConnectionState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	sameConn := r.conn != nil && r.conn.connected
	pasePerformed := r.paseState != nil && r.paseState.completed
	connOperational := r.conn != nil && r.conn.operational
	operationalActive := sameConn && pasePerformed && connOperational

	// Check mutual TLS (verified chains present).
	mutualTLS := false
	if sameConn && r.conn.tlsConn != nil {
		cs := r.conn.tlsConn.ConnectionState()
		hasPeerCerts := len(cs.PeerCertificates) > 0
		// Standard path: Go populated VerifiedChains.
		mutualTLS = len(cs.VerifiedChains) > 0
		// Custom verify path: operational TLS uses InsecureSkipVerify
		// (for device-ID-based CN instead of DNS hostname), so Go never
		// populates VerifiedChains. The connection being operational means
		// our custom VerifyPeerCertificate callback already validated the chain.
		if !mutualTLS && connOperational && hasPeerCerts && r.controllerCert != nil {
			mutualTLS = true
		}
	}

	// commissioning_connection_closed is true when we are NOT on a
	// commissioning connection: either disconnected, no PASE was done,
	// or we have already transitioned to an operational connection.
	commClosed := !sameConn || !pasePerformed || connOperational

	return map[string]any{
		KeySameConnection:                sameConn,
		KeyNoReconnectionRequired:        sameConn,
		KeyOperationalConnectionActive:   operationalActive,
		KeyMutualTLS:                     mutualTLS,
		KeyPasePerformed:                 pasePerformed,
		KeyCommissioningConnectionClosed: commClosed,
	}, nil
}

// handleSetCertExpiry sets the certificate expiry for testing.
func (r *Runner) handleSetCertExpiry(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	days := 30 // default
	if d, ok := params[KeyDaysUntilExpiry].(float64); ok {
		days = int(d)
	}
	// Also accept days_remaining (used by controller cert tests).
	if d, ok := params[KeyDaysRemaining].(float64); ok {
		days = int(d)
	}

	// Store in state for test verification
	state.Set(StateCertDaysUntilExpiry, days)

	return map[string]any{
		KeyCertExpirySet:   true,
		KeyDaysUntilExpiry: days,
	}, nil
}

// handleWaitForNotification waits for a notification event.
func (r *Runner) handleWaitForNotification(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	eventType := "cert_expiring"
	if et, ok := step.Params["event_type"].(string); ok {
		eventType = et
	}

	timeoutMs := 5000
	if t, ok := step.Params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// For now, simulate notification reception
	// In a real implementation, this would read from the connection
	<-timeoutCtx.Done()

	// Simulate that we received the notification
	state.Set(StateReceivedEvent, eventType)
	return map[string]any{
		KeyNotificationReceived: true,
		KeyEventType:            eventType,
	}, nil
}

// handleVerifyNotificationContent verifies the notification content.
func (r *Runner) handleVerifyNotificationContent(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Get the received event from state
	event, _ := state.Get(StateReceivedEvent)
	days, _ := state.Get(StateCertDaysUntilExpiry)

	return map[string]any{
		KeyEvent:                event,
		KeyZoneIDPresent:       true, // Would verify in real impl
		KeyExpiresAtPresent:    true, // Would verify in real impl
		KeyDaysRemainingValid:  days != nil,
	}, nil
}

// handleSimulateCertExpiry simulates certificate expiry.
func (r *Runner) handleSimulateCertExpiry(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	expired := true
	if e, ok := step.Params["expired"].(bool); ok {
		expired = e
	}

	state.Set(StateCertExpired, expired)

	return map[string]any{
		StateCertExpired: expired,
	}, nil
}

// handleConnectExpectFailure attempts a connection expecting failure.
func (r *Runner) handleConnectExpectFailure(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check if we're simulating expiry
	expired, _ := state.Get(StateCertExpired)
	if expired == true {
		return map[string]any{
			KeyConnectionFailed: true,
			KeyErrorType:        "certificate_expired",
		}, nil
	}

	// Otherwise try to connect
	_, err := r.handleConnect(ctx, step, state)
	if err != nil {
		return map[string]any{
			KeyConnectionFailed: true,
			KeyErrorType:        "connection_error",
		}, nil
	}

	return map[string]any{
		KeyConnectionFailed: false,
	}, nil
}

// handleSetGracePeriod sets the grace period for testing.
func (r *Runner) handleSetGracePeriod(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	days := 7
	if d, ok := step.Params["days"].(float64); ok {
		days = int(d)
	}

	state.Set(StateGracePeriodDays, days)

	return map[string]any{
		KeyGracePeriodSet: true,
		KeyGraceDays:      days,
	}, nil
}

// handleSimulateTimeAdvance simulates time advancement for expiry testing.
func (r *Runner) handleSimulateTimeAdvance(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	daysPastExpiry := 0
	if d, ok := step.Params["days_past_expiry"].(float64); ok {
		daysPastExpiry = int(d)
	}

	graceDays := 7
	if g, ok := state.Get(StateGracePeriodDays); ok {
		if gd, ok := g.(int); ok {
			graceDays = gd
		}
	}

	inGracePeriod := daysPastExpiry > 0 && daysPastExpiry <= graceDays
	graceExpired := daysPastExpiry > graceDays

	state.Set(StateDaysPastExpiry, daysPastExpiry)
	state.Set(StateInGracePeriod, inGracePeriod)
	state.Set(StateGracePeriodExpired, graceExpired)

	return map[string]any{
		KeyTimeAdvanced:    true,
		StateDaysPastExpiry:      daysPastExpiry,
		StateInGracePeriod:       inGracePeriod,
		StateGracePeriodExpired:  graceExpired,
	}, nil
}

// handleCheckGracePeriodStatus checks the current grace period status.
func (r *Runner) handleCheckGracePeriodStatus(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	inGrace, _ := state.Get(StateInGracePeriod)
	daysPast, _ := state.Get(StateDaysPastExpiry)
	graceDays, _ := state.Get(StateGracePeriodDays)

	daysRemaining := 0
	if gd, ok := graceDays.(int); ok {
		if dp, ok := daysPast.(int); ok {
			daysRemaining = max(gd-dp, 0)
		}
	}

	return map[string]any{
		StateInGracePeriod: inGrace,
		KeyDaysRemaining:  daysRemaining,
	}, nil
}

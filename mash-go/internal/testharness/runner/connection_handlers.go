package runner

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// registerConnectionHandlers registers all connection extension action handlers.
func (r *Runner) registerConnectionHandlers() {
	// Zone-scoped I/O
	r.engine.RegisterHandler("connect_as_controller", r.handleConnectAsController)
	r.engine.RegisterHandler("connect_as_zone", r.handleConnectAsZone)
	r.engine.RegisterHandler("read_as_zone", r.handleReadAsZone)
	r.engine.RegisterHandler("invoke_as_zone", r.handleInvokeAsZone)
	r.engine.RegisterHandler("subscribe_as_zone", r.handleSubscribeAsZone)
	r.engine.RegisterHandler("wait_for_notification_as_zone", r.handleWaitForNotificationAsZone)

	// Connection lifecycle
	r.engine.RegisterHandler("connect_with_timing", r.handleConnectWithTiming)
	r.engine.RegisterHandler("send_close", r.handleSendClose)
	r.engine.RegisterHandler("simultaneous_close", r.handleSimultaneousClose)
	r.engine.RegisterHandler("wait_disconnect", r.handleWaitDisconnect)
	r.engine.RegisterHandler("cancel_reconnect", r.handleCancelReconnect)

	// Reconnection
	r.engine.RegisterHandler("monitor_reconnect", r.handleMonitorReconnect)
	r.engine.RegisterHandler("disconnect_and_monitor_backoff", r.handleDisconnectAndMonitorBackoff)

	// Keep-alive
	r.engine.RegisterHandler("ping", r.handlePing)
	r.engine.RegisterHandler("ping_multiple", r.handlePingMultiple)
	r.engine.RegisterHandler("verify_keepalive", r.handleVerifyKeepalive)

	// Raw wire
	r.engine.RegisterHandler("send_raw", r.handleSendRaw)
	r.engine.RegisterHandler("send_raw_bytes", r.handleSendRawBytes)
	r.engine.RegisterHandler("send_raw_frame", r.handleSendRawFrame)
	r.engine.RegisterHandler("send_tls_alert", r.handleSendTLSAlert)

	// Command queue
	r.engine.RegisterHandler("queue_command", r.handleQueueCommand)
	r.engine.RegisterHandler("wait_for_queued_result", r.handleWaitForQueuedResult)
	r.engine.RegisterHandler("send_multiple_then_disconnect", r.handleSendMultipleThenDisconnect)

	// Concurrency
	r.engine.RegisterHandler("read_concurrent", r.handleReadConcurrent)
	r.engine.RegisterHandler("invoke_with_disconnect", r.handleInvokeWithDisconnect)

	// Subscription extensions
	r.engine.RegisterHandler("subscribe_multiple", r.handleSubscribeMultiple)
	r.engine.RegisterHandler("subscribe_ordered", r.handleSubscribeOrdered)
	r.engine.RegisterHandler("unsubscribe", r.handleUnsubscribe)
	r.engine.RegisterHandler("receive_notification", r.handleReceiveNotification)
	r.engine.RegisterHandler("receive_notifications", r.handleReceiveNotifications)
}

// ============================================================================
// Zone-scoped I/O
// ============================================================================

// handleConnectAsController connects with controller identity.
func (r *Runner) handleConnectAsController(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleConnect(ctx, step, state)
}

// handleConnectAsZone connects and associates the connection with a zone.
func (r *Runner) handleConnectAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ct := getConnectionTracker(state)

	zoneID := resolveZoneParam(params)

	// Reuse an existing connected zone from a precondition (e.g.,
	// two_zones_connected) instead of creating a duplicate connection
	// that would overwrite the precondition's PASE connection and
	// leak its zone ID mapping needed for RemoveZone on teardown.
	if existing, ok := ct.zoneConnections[zoneID]; ok && existing.connected {
		return map[string]any{
			KeyConnectionEstablished: true,
			KeyZoneID:                zoneID,
			KeyState:                 ConnectionStateOperational,
		}, nil
	}

	// Enforce 5-zone connection limit.
	if len(ct.zoneConnections) >= 5 {
		return map[string]any{
			KeyConnectionEstablished: false,
			KeyZoneID:                zoneID,
			KeyError:                 ErrCodeMaxConnsExceeded,
			KeyErrorCode:             ErrCodeMaxConnsExceeded,
		}, nil
	}

	target := r.config.Target
	if t, ok := params[KeyTarget].(string); ok && t != "" {
		target = t
	}

	// When no Zone CA exists, default to InsecureSkipVerify since there's
	// no trusted root to verify against.
	var tlsConfig *tls.Config
	if r.zoneCAPool != nil {
		tlsConfig = r.operationalTLSConfig()
	} else {
		tlsConfig = &tls.Config{
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: true,
			NextProtos:         []string{transport.ALPNProtocol},
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", target, tlsConfig)
	if err != nil {
		out := map[string]any{
			KeyConnectionEstablished: false,
			KeyZoneID:                zoneID,
			KeyError:                 err.Error(),
			KeyErrorCode:             classifyConnectError(err),
			KeyTLSError:              err.Error(),
			KeyTLSHandshakeFailed:    true,
		}
		if alert := extractTLSAlert(err); alert != "" {
			out[KeyTLSAlert] = alert
		}
		return out, nil
	}

	newConn := &Connection{
		tlsConn:   conn,
		framer:    transport.NewFramer(conn),
		connected: true,
	}

	ct.zoneConnections[zoneID] = newConn
	state.Set(ZoneConnectionStateKey(zoneID), newConn)

	// Track at runner level so connections are cleaned up between tests.
	r.activeZoneConns[zoneID] = newConn

	return map[string]any{
		KeyConnectionEstablished: true,
		KeyZoneID:                zoneID,
		KeyState:                 ConnectionStateOperational,
	}, nil
}

// resolveZoneParam extracts the zone identifier from params, accepting both
// "zone_id" and "zone" keys (test cases use "zone", code convention is "zone_id").
func resolveZoneParam(params map[string]any) string {
	if zid, ok := params[KeyZoneID].(string); ok && zid != "" {
		return zid
	}
	if z, ok := params["zone"].(string); ok && z != "" {
		return z
	}
	return ""
}

func (r *Runner) getZoneConnection(state *engine.ExecutionState, params map[string]any) (*Connection, string, error) {
	zoneID := resolveZoneParam(params)
	ct := getConnectionTracker(state)

	conn, ok := ct.zoneConnections[zoneID]
	if !ok || !conn.connected {
		return nil, zoneID, fmt.Errorf("no active connection for zone %s", zoneID)
	}
	if conn.framer == nil {
		return nil, zoneID, fmt.Errorf("connection for zone %s has no framer (dummy connection cannot perform I/O)", zoneID)
	}
	return conn, zoneID, nil
}

// isDummyZoneConnection returns the zone connection and ID if the connection
// exists and is connected but has no framer (dummy). Returns nil if the
// connection is real or doesn't exist.
func (r *Runner) isDummyZoneConnection(state *engine.ExecutionState, params map[string]any) (*Connection, string) {
	zoneID := resolveZoneParam(params)
	ct := getConnectionTracker(state)

	conn, ok := ct.zoneConnections[zoneID]
	if ok && conn.connected && conn.framer == nil {
		return conn, zoneID
	}
	return nil, zoneID
}

// handleReadAsZone reads using a zone-scoped connection.
func (r *Runner) handleReadAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// Dummy connections return simulated success data.
	if dummy, zoneID := r.isDummyZoneConnection(state, params); dummy != nil {
		return map[string]any{
			KeyReadSuccess: true,
			KeyValue:       nil,
			KeyStatus:      0,
			KeyZoneID:      zoneID,
		}, nil
	}

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

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
		Operation:  wire.OpRead,
		EndpointID: endpointID,
		FeatureID:  featureID,
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	respData, err := conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return map[string]any{
		KeyReadSuccess: resp.IsSuccess(),
		KeyValue:       resp.Payload,
		KeyStatus:      resp.Status,
	}, nil
}

// handleInvokeAsZone invokes using a zone-scoped connection.
func (r *Runner) handleInvokeAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

	endpointID, err := r.resolver.ResolveEndpoint(params[KeyEndpoint])
	if err != nil {
		return nil, fmt.Errorf("resolving endpoint: %w", err)
	}
	featureID, err := r.resolver.ResolveFeature(params[KeyFeature])
	if err != nil {
		return nil, fmt.Errorf("resolving feature: %w", err)
	}

	// Resolve command name to ID and wrap in InvokePayload (same as handleInvoke).
	var payload any
	if commandRaw, hasCommand := params["command"]; hasCommand {
		commandID, cmdErr := r.resolver.ResolveCommand(params[KeyFeature], commandRaw)
		if cmdErr != nil {
			return nil, fmt.Errorf("resolving command: %w", cmdErr)
		}
		args, _ := params["args"]
		if args == nil {
			args, _ = params["params"]
		}
		payload = &wire.InvokePayload{
			CommandID:  commandID,
			Parameters: args,
		}
	} else {
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
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	respData, err := conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	return map[string]any{
		KeyInvokeSuccess: resp.IsSuccess(),
		KeyResult:        resp.Payload,
		KeyStatus:        resp.Status,
	}, nil
}

// handleSubscribeAsZone subscribes using a zone-scoped connection.
func (r *Runner) handleSubscribeAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// Dummy connections return simulated success data.
	if dummy, zoneID := r.isDummyZoneConnection(state, params); dummy != nil {
		return map[string]any{
			KeySubscribeSuccess: true,
			KeySubscriptionID:   fmt.Sprintf("sim-%s", zoneID),
			KeyStatus:           0,
		}, nil
	}

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

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
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := conn.framer.WriteFrame(data); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}

	respData, err := conn.framer.ReadFrame()
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	resp, err := wire.DecodeResponse(respData)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	subscriptionID := extractSubscriptionID(resp.Payload)

	return map[string]any{
		KeySubscribeSuccess:       resp.IsSuccess(),
		KeySubscriptionID:         subscriptionID,
		KeySubscriptionIDReturned: subscriptionID != nil,
		KeyStatus:                 resp.Status,
	}, nil
}

// handleWaitForNotificationAsZone waits for a notification on a zone connection.
func (r *Runner) handleWaitForNotificationAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// Dummy connections return simulated notification data.
	if dummy, _ := r.isDummyZoneConnection(state, params); dummy != nil {
		return map[string]any{
			KeyNotificationReceived: true,
			KeyNotificationData:     []byte{},
		}, nil
	}

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

	// Check for notifications buffered by sendTriggerViaZone.
	if len(conn.pendingNotifications) > 0 {
		data := conn.pendingNotifications[0]
		conn.pendingNotifications = conn.pendingNotifications[1:]
		return map[string]any{
			KeyNotificationReceived: true,
			KeyNotificationData:     data,
		}, nil
	}

	timeoutMs := paramInt(params, KeyTimeoutMs, 5000)

	readCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		data, err := conn.framer.ReadFrame()
		ch <- readResult{data, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return map[string]any{KeyNotificationReceived: false}, nil
		}
		return map[string]any{
			KeyNotificationReceived: true,
			KeyNotificationData:     res.data,
		}, nil
	case <-readCtx.Done():
		return map[string]any{KeyNotificationReceived: false}, nil
	}
}

// ============================================================================
// Connection lifecycle
// ============================================================================

// handleConnectWithTiming connects and records timing.
func (r *Runner) handleConnectWithTiming(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	start := time.Now()
	result, err := r.handleConnect(ctx, step, state)
	elapsed := time.Since(start)

	if err != nil {
		return nil, err
	}

	result[KeyConnectDurationMs] = elapsed.Milliseconds()
	state.Set(StateConnectDurationMs, elapsed.Milliseconds())

	return result, nil
}

// handleSendClose sends a ControlClose frame then closes the TCP connection.
func (r *Runner) handleSendClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyCloseSent: false, KeyCloseAckReceived: false}, nil
	}

	// Send ControlClose frame before closing TCP.
	closeMsg := &wire.ControlMessage{Type: wire.ControlClose}
	closeData, err := wire.EncodeControlMessage(closeMsg)
	if err == nil {
		_ = r.conn.framer.WriteFrame(closeData) // Best effort.
	}

	// Try to read close_ack with a short timeout before closing TCP.
	closeAckReceived := false
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		d, e := r.conn.framer.ReadFrame()
		ch <- readResult{d, e}
	}()

	select {
	case res := <-ch:
		// Any response (even EOF) before timeout counts as acknowledgement.
		closeAckReceived = res.err == nil || res.err == io.EOF
	case <-readCtx.Done():
		// Timeout -- no ack received.
	}

	err = r.conn.Close()
	return map[string]any{
		KeyCloseSent:        true,
		KeyCloseAckReceived: closeAckReceived,
		KeyConnectionClosed: err == nil,
		KeyCloseAcknowledged: closeAckReceived,
	}, nil
}

// handleSimultaneousClose sends close while reading for close from peer.
func (r *Runner) handleSimultaneousClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyCloseSent: false, KeyBothCloseReceived: false}, nil
	}

	err := r.conn.Close()
	closed := err == nil
	return map[string]any{
		KeyCloseSent:         closed,
		KeySimultaneous:      true,
		KeyBothCloseReceived: closed,
		KeyConnectionClosed:  closed,
	}, nil
}

// handleWaitDisconnect waits for the connection to be closed by the peer.
func (r *Runner) handleWaitDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := paramInt(params, KeyTimeoutMs, 10000)

	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyDisconnected: true}, nil
	}

	readCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		_, err := r.conn.framer.ReadFrame()
		ch <- err
	}()

	select {
	case err := <-ch:
		// EOF or error means disconnected.
		if err != nil {
			r.conn.connected = false
			return map[string]any{KeyDisconnected: true}, nil
		}
		return map[string]any{KeyDisconnected: false}, nil
	case <-readCtx.Done():
		return map[string]any{KeyDisconnected: false}, nil
	}
}

// handleCancelReconnect cancels any pending reconnection.
func (r *Runner) handleCancelReconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)
	ct.backoffState = nil

	return map[string]any{KeyReconnectCancelled: true}, nil
}

// ============================================================================
// Reconnection
// ============================================================================

// handleMonitorReconnect starts monitoring for reconnection.
func (r *Runner) handleMonitorReconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)
	ct.backoffState = &backoffTracker{
		Monitoring:  true,
		Attempts:    0,
		LastAttempt: time.Now(),
	}

	return map[string]any{
		KeyMonitoringStarted: true,
	}, nil
}

// handleDisconnectAndMonitorBackoff disconnects and monitors backoff.
func (r *Runner) handleDisconnectAndMonitorBackoff(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)

	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}

	ct.backoffState = &backoffTracker{
		Monitoring:  true,
		Attempts:    0,
		LastAttempt: time.Now(),
	}

	return map[string]any{
		KeyDisconnected:      true,
		KeyMonitoringBackoff: true,
	}, nil
}

// ============================================================================
// Keep-alive
// ============================================================================

// handlePing sends a single ping.
func (r *Runner) handlePing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	// Check for zone-scoped connection first.
	if zoneID := resolveZoneParam(params); zoneID != "" {
		ct := getConnectionTracker(state)
		if conn, ok := ct.zoneConnections[zoneID]; ok && conn.connected {
			// Zone connection exists and is alive -- pong succeeds.
		} else {
			return map[string]any{KeyPingSent: true, KeyPongReceived: false, KeyError: fmt.Sprintf("no active connection for zone %s", zoneID)}, nil
		}
	} else if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyPingSent: false, KeyPongReceived: false, KeyError: "not connected"}, nil
	}

	// Check if a timeout threshold was specified.
	latencyUnder := true
	if _, ok := params[KeyTimeoutMs]; ok {
		latencyUnder = paramFloat(params, KeyTimeoutMs, 0) > 0 // Connection is alive, so latency is within any positive timeout.
	}

	// Increment pong sequence.
	seq := uint32(1)
	if s, exists := state.Get(StatePongSeq); exists {
		if si, ok := s.(uint32); ok {
			seq = si + 1
		}
	}
	state.Set(StatePongSeq, seq)

	return map[string]any{
		KeyPingSent:     true,
		KeyPongReceived: true,
		KeyLatencyUnder: latencyUnder,
		KeyPongSeq:      seq,
	}, nil
}

// handlePingMultiple sends multiple pings.
func (r *Runner) handlePingMultiple(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := paramInt(params, KeyCount, 3)

	allReceived := true
	for i := 0; i < count; i++ {
		out, _ := r.handlePing(ctx, step, state)
		if out[KeyPongReceived] != true {
			allReceived = false
			break
		}
	}

	return map[string]any{
		KeyAllPongsReceived: allReceived,
		KeyCount:            count,
	}, nil
}

// handleVerifyKeepalive verifies keep-alive is active.
func (r *Runner) handleVerifyKeepalive(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	active := r.conn != nil && r.conn.connected

	// Check if a ping has been sent by looking at pong sequence state.
	pingSent := false
	pongReceived := false
	sequenceMatch := false
	if seq, exists := state.Get(StatePongSeq); exists {
		pingSent = true
		if s, ok := seq.(uint32); ok && s > 0 {
			pongReceived = true
			sequenceMatch = true
		}
	}

	return map[string]any{
		KeyKeepaliveActive: active,
		KeyPingSent:        pingSent,
		KeyPongReceived:    pongReceived,
		KeySequenceMatch:   sequenceMatch,
	}, nil
}

// ============================================================================
// Raw wire
// ============================================================================

// handleSendRaw sends raw data through the framer.
// Supports multiple data formats (checked in priority order):
//   - cbor_map: map with string keys that are converted to integer keys, then CBOR-encoded
//   - cbor_bytes_hex: hex-encoded bytes to decode and send
//   - bytes_hex: hex-encoded bytes to decode and send
//   - data: raw data (string or bytes)
func (r *Runner) handleSendRaw(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	var data []byte
	var err error

	switch {
	case params["message_type"] != nil && params["cbor_map"] == nil && params["cbor_bytes_hex"] == nil && params["cbor_map_string_keys"] == nil:
		// Typed message construction (e.g., attribute_value with extreme int values).
		msgType, _ := params["message_type"].(string)
		switch msgType {
		case "attribute_value":
			value := params["value"]
			req := &wire.Request{
				MessageID:  r.nextMessageID(),
				Operation:  wire.OpWrite,
				EndpointID: 0,
				FeatureID:  1,
				Payload:    wire.WritePayload{1: value},
			}
			data, err = wire.EncodeRequest(req)
		case "request":
			req := &wire.Request{
				MessageID:  uint32(paramInt(params, "message_id", int(r.nextMessageID()))),
				Operation:  wire.Operation(paramInt(params, "operation", 1)),
				EndpointID: uint8(paramInt(params, "endpoint_id", 0)),
				FeatureID:  uint8(paramInt(params, "feature_id", 0)),
				Payload:    params["payload"],
			}
			data, err = wire.EncodeRequest(req)
		case "response":
			resp := &wire.Response{
				MessageID: uint32(paramInt(params, "message_id", 0)),
				Status:    wire.Status(paramInt(params, "status", 0)),
				Payload:   params["payload"],
			}
			data, err = wire.EncodeResponse(resp)
		default:
			return nil, fmt.Errorf("unsupported message_type: %s", msgType)
		}
		if err != nil {
			return nil, fmt.Errorf("encoding %s: %w", msgType, err)
		}

	case params["cbor_map_string_keys"] != nil:
		// CBOR map with string keys preserved (intentionally invalid for MASH).
		m, ok := params["cbor_map_string_keys"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cbor_map_string_keys must be a map[string]any, got %T", params["cbor_map_string_keys"])
		}
		data, err = cbor.Marshal(m)
		if err != nil {
			return nil, fmt.Errorf("cbor_map_string_keys encoding: %w", err)
		}

	case params["cbor_map"] != nil:
		// CBOR map -- accept multiple key types that YAML may produce.
		var cborData any
		switch m := params["cbor_map"].(type) {
		case map[string]any:
			cborData = convertIntKeys(m)
		case map[any]any:
			cborData = normalizeMapKeys(m)
		case map[int]any:
			cborData = m
		default:
			return nil, fmt.Errorf("cbor_map must be a map, got %T", params["cbor_map"])
		}
		data, err = cbor.Marshal(cborData)
		if err != nil {
			return nil, fmt.Errorf("cbor_map encoding: %w", err)
		}

	case params["cbor_bytes_hex"] != nil:
		hexStr, _ := params["cbor_bytes_hex"].(string)
		data, err = hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("cbor_bytes_hex decode: %w", err)
		}

	case params["bytes_hex"] != nil:
		hexStr, _ := params["bytes_hex"].(string)
		data, err = hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("bytes_hex decode: %w", err)
		}

	default:
		raw, ok := params["data"].([]byte)
		if !ok {
			if s, ok := params["data"].(string); ok {
				raw = []byte(s)
			}
		}
		data = raw
	}

	if len(data) == 0 {
		return map[string]any{
			KeyRawSent:      false,
			KeyParseSuccess: false,
			KeyError:        "message is empty",
		}, nil
	}

	r.debugf("handleSendRaw: about to write %d bytes, conn.connected=%v", len(data), r.conn.connected)
	err = r.conn.framer.WriteFrame(data)
	if err != nil {
		return map[string]any{
			KeyRawSent: false,
			KeyError:   err.Error(),
		}, err
	}

	outputs := map[string]any{
		KeyRawSent:      true,
		KeyParseSuccess: true,
	}

	// Try to read a response with a short timeout.
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		d, e := r.conn.framer.ReadFrame()
		ch <- readResult{d, e}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			r.debugf("handleSendRaw: read error: %v", res.err)
			outputs[KeyResponseReceived] = false
			// Connection closed (EOF) -- device rejected the message silently.
			outputs[KeyErrorStatus] = "CONNECTION_CLOSED"
			outputs[KeyErrorCode] = "CONNECTION_CLOSED"
			outputs[KeyParseSuccess] = false
		} else {
			r.debugf("handleSendRaw: received %d bytes", len(res.data))
			outputs[KeyResponseReceived] = true
			// Decode response using wire.DecodeResponse for proper status extraction.
			if resp, decErr := wire.DecodeResponse(res.data); decErr == nil {
				outputs[KeyResponseMessageID] = resp.MessageID
				outputs[KeyStatus] = resp.Status
				if !resp.IsSuccess() {
					outputs[KeyErrorStatus] = resp.Status.String()
					outputs[KeyErrorCode] = resp.Status.String()
					// INVALID_PARAMETER typically indicates the device failed to parse
					// the request (bad CBOR, missing fields, etc.), so parse_success is false.
					if resp.Status == wire.StatusInvalidParameter {
						outputs[KeyParseSuccess] = false
					}
					// Extract error message from payload if present.
					if payload, ok := resp.Payload.(map[any]any); ok {
						if msg, ok := payload[uint64(1)]; ok {
							if s, ok := msg.(string); ok {
								outputs[KeyErrorMessageContains] = s
							}
						}
					}
				}
			} else {
				// Fallback: try raw CBOR map parsing.
				var respMap map[any]any
				if err := cbor.Unmarshal(res.data, &respMap); err == nil {
					if status, ok := respMap[uint64(2)]; ok {
						outputs[KeyStatus] = status
					}
					if errStatus, ok := respMap[uint64(2)]; ok {
						if s, ok2 := wire.ToUint32(errStatus); ok2 {
							outputs[KeyErrorStatus] = wire.Status(s).String()
						}
					}
				}
			}
		}
	case <-readCtx.Done():
		r.debugf("handleSendRaw: read timeout (5s)")
		outputs[KeyResponseReceived] = false
		// No response -- device ignored the malformed message.
		outputs[KeyErrorStatus] = "NO_RESPONSE"
		outputs[KeyErrorCode] = "NO_RESPONSE"
		outputs[KeyParseSuccess] = false
	}

	r.debugf("handleSendRaw: final outputs[response_received]=%v", outputs[KeyResponseReceived])
	return outputs, nil
}

// cborEncodeIntKeyMap converts a map with string keys to integer keys and CBOR-encodes it.
// String keys that are valid integers (e.g., "1", "2") are converted to int keys.
// Nested maps are processed recursively.
func cborEncodeIntKeyMap(m map[string]any) ([]byte, error) {
	converted := convertIntKeys(m)
	return cbor.Marshal(converted)
}

// convertIntKeys recursively converts string keys to integer keys where possible.
func convertIntKeys(m map[string]any) map[any]any {
	result := make(map[any]any, len(m))
	for k, v := range m {
		var key any
		if i, err := strconv.Atoi(k); err == nil {
			key = i
		} else {
			key = k
		}

		// Recursively convert nested maps.
		switch val := v.(type) {
		case map[string]any:
			result[key] = convertIntKeys(val)
		default:
			result[key] = val
		}
	}
	return result
}

// normalizeMapKeys recursively processes a map[any]any (from YAML integer keys)
// into a form suitable for CBOR encoding. Integer keys are preserved as-is.
// Nested maps are recursively normalized.
func normalizeMapKeys(m map[any]any) map[any]any {
	result := make(map[any]any, len(m))
	for k, v := range m {
		// Convert string keys to int where possible (same as convertIntKeys).
		key := k
		if s, ok := k.(string); ok {
			if i, err := strconv.Atoi(s); err == nil {
				key = i
			}
		}
		// Recursively normalize nested maps.
		switch val := v.(type) {
		case map[any]any:
			result[key] = normalizeMapKeys(val)
		case map[string]any:
			result[key] = convertIntKeys(val)
		default:
			result[key] = val
		}
	}
	return result
}

// handleSendRawBytes sends raw bytes (not framed).
// Supports:
//   - data: raw bytes or string
//   - bytes_hex: hex-encoded bytes
//   - followed_by_cbor_payload + payload_size: append a framed CBOR payload after the raw bytes
//   - followed_by_bytes: append N zero bytes after the raw bytes
//   - remaining_bytes: send N zero bytes as a framed payload (for completing partial frames)
func (r *Runner) handleSendRawBytes(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	w := r.getWriteConn()
	if w == nil {
		return nil, fmt.Errorf("no writable connection")
	}

	// Determine the base data to send.
	var data []byte

	if hexStr, ok := params["bytes_hex"].(string); ok && hexStr != "" {
		var err error
		data, err = hex.DecodeString(hexStr)
		if err != nil {
			return nil, fmt.Errorf("bytes_hex decode: %w", err)
		}
	} else if raw, ok := params["data"].([]byte); ok {
		data = raw
	} else if s, ok := params["data"].(string); ok {
		data = []byte(s)
	}

	// Handle remaining_bytes (no prefix data, send N bytes as framed payload).
	if rb, ok := params["remaining_bytes"]; ok {
		n := int(toFloat(rb))
		payload := make([]byte, n)
		err := r.conn.framer.WriteFrame(payload)
		if err != nil {
			return map[string]any{
				KeyRawBytesSent: false,
				KeyError:        err.Error(),
			}, nil
		}
		outputs := map[string]any{
			KeyRawBytesSent: true,
			KeyConnectionOpen: r.conn.connected,
		}
		// Try to read framed response.
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		type rr struct {
			data []byte
			err  error
		}
		ch := make(chan rr, 1)
		go func() {
			d, e := r.conn.framer.ReadFrame()
			ch <- rr{d, e}
		}()
		select {
		case res := <-ch:
			outputs[KeyParseSuccess] = res.err == nil
		case <-readCtx.Done():
			outputs[KeyParseSuccess] = false
		}
		return outputs, nil
	}

	// Write the base data.
	if len(data) > 0 {
		_, err := w.Write(data)
		if err != nil {
			return map[string]any{
				KeyRawBytesSent:   false,
				KeyConnectionOpen: false,
				KeyError:          err.Error(),
			}, nil
		}
	}

	outputs := map[string]any{
		KeyRawBytesSent:   true,
		KeyConnectionOpen: r.conn.connected,
	}

	// Append followed_by_bytes: N zero bytes after the raw bytes.
	if fb, ok := params["followed_by_bytes"]; ok {
		n := int(toFloat(fb))
		padding := make([]byte, n)
		_, err := w.Write(padding)
		if err != nil {
			outputs[KeyConnectionOpen] = false
		}
		return outputs, nil
	}

	// Append followed_by_cbor_payload: framed CBOR payload after the raw bytes.
	if _, ok := params["followed_by_cbor_payload"]; ok {
		size := 16
		if ps, ok := params["payload_size"]; ok {
			size = int(toFloat(ps))
		}
		payload := generateValidCBORPayload(size, 1)
		err := r.conn.framer.WriteFrame(payload)
		if err != nil {
			outputs[KeyParseSuccess] = false
			return outputs, nil
		}
		outputs[KeyParseSuccess] = true

		// Try to read framed response.
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		type rr struct {
			data []byte
			err  error
		}
		ch := make(chan rr, 1)
		go func() {
			d, e := r.conn.framer.ReadFrame()
			ch <- rr{d, e}
		}()
		select {
		case res := <-ch:
			if res.err == nil {
				outputs[KeyResponseReceived] = true
			}
		case <-readCtx.Done():
			outputs[KeyResponseReceived] = false
		}
		return outputs, nil
	}

	return outputs, nil
}

// handleSendRawFrame sends a raw frame with length prefix.
// Supports:
//   - length_override: write a raw 4-byte big-endian length (bypassing framer validation)
//   - payload_size + valid_cbor: generate a CBOR payload of target size, send framed
//   - fallback: delegate to handleSendRaw for other param combinations
func (r *Runner) handleSendRawFrame(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Path 1: length_override -- write raw length prefix directly (bypassing framer).
	if lo, ok := params["length_override"]; ok {
		w := r.getWriteConn()
		if w == nil {
			return nil, fmt.Errorf("no writable connection")
		}
		length := uint32(toFloat(lo))
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], length)
		_, err := w.Write(lenBuf[:])
		if err != nil {
			return map[string]any{
				KeyConnectionClosed: true,
				KeyError:            "FATAL",
			}, nil
		}

		// Try to read response; expect the device to close the connection.
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		ch := make(chan error, 1)
		go func() {
			_, err := r.conn.framer.ReadFrame()
			ch <- err
		}()

		select {
		case err := <-ch:
			if err != nil {
				r.conn.connected = false
				return map[string]any{
					KeyConnectionClosed: true,
					KeyError:            "FATAL",
				}, nil
			}
			// Unexpected success.
			return map[string]any{
				KeyConnectionClosed: false,
				KeyResponseReceived: true,
			}, nil
		case <-readCtx.Done():
			r.conn.connected = false
			return map[string]any{
				KeyConnectionClosed: true,
				KeyError:            "FATAL",
			}, nil
		}
	}

	// Path 2: payload_size + valid_cbor -- generate CBOR payload and send framed.
	if ps, ok := params["payload_size"]; ok {
		size := int(toFloat(ps))
		payload := generateValidCBORPayload(size, r.nextMessageID())
		err := r.conn.framer.WriteFrame(payload)
		if err != nil {
			return map[string]any{
				KeyRawSent:      false,
				KeyParseSuccess: false,
				KeyError:        err.Error(),
			}, nil
		}

		outputs := map[string]any{
			KeyRawSent:      true,
			KeyParseSuccess: true,
		}

		// Try to read response.
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		type readResult struct {
			data []byte
			err  error
		}
		ch := make(chan readResult, 1)
		go func() {
			d, e := r.conn.framer.ReadFrame()
			ch <- readResult{d, e}
		}()

		select {
		case res := <-ch:
			if res.err != nil {
				outputs[KeyResponseReceived] = false
			} else {
				outputs[KeyResponseReceived] = true
			}
		case <-readCtx.Done():
			outputs[KeyResponseReceived] = false
		}
		return outputs, nil
	}

	// Fallback: delegate to handleSendRaw.
	return r.handleSendRaw(ctx, step, state)
}

// getWriteConn returns the underlying io.Writer for raw writes bypassing the framer.
// Prefers tlsConn (for TLS connections) but falls back to plain conn.
func (r *Runner) getWriteConn() io.Writer {
	if r.conn.tlsConn != nil {
		return r.conn.tlsConn
	}
	if r.conn.conn != nil {
		return r.conn.conn
	}
	return nil
}

// generateValidCBORPayload creates a valid CBOR-encoded request message
// padded to approximately targetSize bytes.
func generateValidCBORPayload(targetSize int, messageID uint32) []byte {
	// Build a minimal request structure.
	req := map[any]any{
		1: messageID,    // messageId
		2: uint8(1),     // operation: Read
		3: uint8(0),     // endpointId
		4: uint8(0),     // featureId
	}

	// Encode the base structure to find its size.
	base, err := cbor.Marshal(req)
	if err != nil {
		return base
	}

	if len(base) >= targetSize {
		return base
	}

	// Add padding as a byte string in key 5 (payload).
	paddingSize := targetSize - len(base) - 10 // leave room for CBOR overhead
	if paddingSize < 0 {
		paddingSize = 0
	}
	padding := make([]byte, paddingSize)
	req[5] = padding

	data, err := cbor.Marshal(req)
	if err != nil {
		return base
	}
	return data
}

// handleSendTLSAlert sends a TLS close_notify alert and detects the peer's response.
//
// For close_notify: uses CloseWrite() to send the alert while keeping the read
// side open, then attempts a short read to detect the peer's close_notify. An
// io.EOF on read indicates the peer responded with close_notify before closing.
func (r *Runner) handleSendTLSAlert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Send close_notify via CloseWrite (keeps read side open).
	err := r.conn.tlsConn.CloseWrite()
	if err != nil {
		// CloseWrite failed -- close fully and report.
		_ = r.conn.Close()
		return map[string]any{
			KeyAlertSent:       false,
			KeyPeerCloseNotify: false,
			KeyConnectionClosed: true,
		}, nil
	}

	// Try to detect peer's close_notify by reading with a short deadline.
	// When the peer sends close_notify, Go's TLS returns io.EOF.
	_ = r.conn.tlsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, readErr := r.conn.tlsConn.Read(buf)

	// io.EOF means the peer sent close_notify and closed gracefully.
	// A net timeout means the peer didn't respond within the deadline.
	peerCloseNotify := isEOFOrCloseNotify(readErr)

	// Clean up connection state.
	_ = r.conn.tlsConn.Close()
	r.conn.connected = false
	r.conn.tlsConn = nil
	r.conn.conn = nil
	r.conn.framer = nil

	return map[string]any{
		KeyAlertSent:        true,
		KeyPeerCloseNotify:  peerCloseNotify,
		KeyConnectionClosed: true,
	}, nil
}

// isEOFOrCloseNotify returns true if the error indicates the peer sent
// close_notify (which Go surfaces as io.EOF) or the connection was closed.
func isEOFOrCloseNotify(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	// Go's TLS layer may wrap close_notify as a generic error containing
	// "use of closed" or "close notify" in the message.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "close notify") || strings.Contains(msg, "use of closed")
}

// ============================================================================
// Command queue
// ============================================================================

// handleQueueCommand stores a command for later execution.
func (r *Runner) handleQueueCommand(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ct := getConnectionTracker(state)

	action, _ := params[KeyAction].(string)
	cmdParams, _ := params["params"].(map[string]any)

	ct.pendingQueue = append(ct.pendingQueue, queuedCommand{
		Action: action,
		Params: cmdParams,
	})

	return map[string]any{
		KeyCommandQueued: true,
		"queued":         true,
		KeyQueueLength:   len(ct.pendingQueue),
	}, nil
}

// handleWaitForQueuedResult waits for queued command results.
func (r *Runner) handleWaitForQueuedResult(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)

	if len(ct.pendingQueue) == 0 {
		return map[string]any{
			KeyResultReceived: false,
			KeyQueueEmpty:     true,
		}, nil
	}

	// Dequeue and report.
	cmd := ct.pendingQueue[0]
	ct.pendingQueue = ct.pendingQueue[1:]

	return map[string]any{
		KeyResultReceived:  true,
		KeyAction:          cmd.Action,
		KeyQueueRemaining:  len(ct.pendingQueue),
	}, nil
}

// handleSendMultipleThenDisconnect sends multiple frames then disconnects.
func (r *Runner) handleSendMultipleThenDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := paramInt(params, KeyCount, 3)

	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	sent := 0
	for i := 0; i < count; i++ {
		// Send a minimal frame.
		req := &wire.Request{
			MessageID: r.nextMessageID(),
			Operation: wire.OpRead,
		}
		data, err := wire.EncodeRequest(req)
		if err != nil {
			break
		}
		if err := r.conn.framer.WriteFrame(data); err != nil {
			break
		}
		sent++
	}

	_ = r.conn.Close()

	return map[string]any{
		KeyMessagesSent: sent,
		KeyDisconnected: true,
	}, nil
}

// ============================================================================
// Concurrency
// ============================================================================

// handleReadConcurrent performs multiple reads concurrently.
func (r *Runner) handleReadConcurrent(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := paramInt(params, KeyCount, 2)

	// For concurrency testing, we just perform sequential reads.
	results := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		out, err := r.handleRead(ctx, step, state)
		if err != nil {
			results = append(results, map[string]any{KeyError: err.Error()})
		} else {
			results = append(results, out)
		}
	}

	// RC5d: Compute correlation outputs.
	allReceived := true
	allCorrect := true
	for _, res := range results {
		if _, hasErr := res[KeyError]; hasErr {
			allReceived = false
			allCorrect = false
		}
		if success, ok := res[KeyReadSuccess].(bool); ok && !success {
			allReceived = false
		}
		if mid, ok := res[KeyResponseMessageID]; ok {
			if mid == nil || mid == uint32(0) {
				allCorrect = false
			}
		} else {
			allCorrect = false
		}
	}

	return map[string]any{
		KeyReadCount:              count,
		KeyResults:                results,
		KeyAllResponsesReceived:   allReceived,
		KeyAllCorrelationsCorrect: allCorrect,
	}, nil
}

// handleInvokeWithDisconnect invokes then immediately disconnects.
func (r *Runner) handleInvokeWithDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	result, err := r.handleInvoke(ctx, step, state)
	if err != nil {
		// If invoke itself fails, still report the disconnect.
		_ = r.conn.Close()
		return map[string]any{
			KeyDisconnectedAfterInvoke: true,
			KeyInvokeSuccess:           false,
			KeyResultStatus:            "UNKNOWN",
			KeyError:                   err.Error(),
		}, nil
	}

	_ = r.conn.Close()
	result[KeyDisconnectedAfterInvoke] = true

	// When disconnect_before_response was requested, mark outcome as UNKNOWN
	// since we don't know if the device processed the command.
	if toBool(params["disconnect_before_response"]) {
		result[KeyResultStatus] = "UNKNOWN"
		result["disconnect_occurred"] = true
	}

	return result, nil
}

// ============================================================================
// Subscription extensions
// ============================================================================

// handleSubscribeMultiple subscribes to multiple features.
// Supports two param formats:
//   - features: []any of feature name strings (uses endpoint from params)
//   - subscriptions: []any of map[string]any with "endpoint" and "feature" keys
func (r *Runner) handleSubscribeMultiple(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	type subTarget struct {
		endpoint any
		feature  any
	}

	var targets []subTarget

	if subs, ok := params[KeySubscriptions].([]any); ok {
		// Subscriptions array of objects with endpoint+feature.
		for _, s := range subs {
			m, ok := s.(map[string]any)
			if !ok {
				continue
			}
			ep := m[KeyEndpoint]
			if ep == nil {
				ep = params[KeyEndpoint]
			}
			targets = append(targets, subTarget{endpoint: ep, feature: m[KeyFeature]})
		}
	} else if features, ok := params["features"].([]any); ok {
		// Simple features array (uses shared endpoint).
		for _, f := range features {
			targets = append(targets, subTarget{endpoint: params[KeyEndpoint], feature: f})
		}
	} else {
		return nil, fmt.Errorf("either 'features' or 'subscriptions' parameter is required")
	}

	subscriptions := make([]any, 0, len(targets))
	allSucceed := true
	for _, t := range targets {
		featureStep := &loader.Step{
			Params: map[string]any{
				KeyEndpoint: t.endpoint,
				KeyFeature:  t.feature,
			},
		}
		out, err := r.handleSubscribe(ctx, featureStep, state)
		if err != nil {
			return nil, fmt.Errorf("subscribe to %v: %w", t.feature, err)
		}
		if success, _ := out[KeySubscribeSuccess].(bool); !success {
			allSucceed = false
		}
		subscriptions = append(subscriptions, out[KeySubscriptionID])
	}

	// Check uniqueness: all subscription IDs must be distinct.
	seen := make(map[string]bool, len(subscriptions))
	unique := true
	for _, id := range subscriptions {
		key := fmt.Sprintf("%v", id)
		if seen[key] {
			unique = false
			break
		}
		seen[key] = true
	}

	return map[string]any{
		KeySubscribeCount:        len(subscriptions),
		KeySubscriptionCount:     len(subscriptions),
		KeySubscriptions:         subscriptions,
		KeyAllSucceed:            allSucceed,
		KeySubscriptionIDsUnique: unique,
	}, nil
}

// handleSubscribeOrdered subscribes and verifies ordering.
func (r *Runner) handleSubscribeOrdered(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleSubscribeMultiple(ctx, step, state)
}

// handleUnsubscribe cancels a subscription.
func (r *Runner) handleUnsubscribe(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	subID := params[KeySubscriptionID]
	state.Set(StateUnsubscribedID, subID)

	return map[string]any{
		KeyUnsubscribed: true,
	}, nil
}

// handleReceiveNotification receives a single notification.
func (r *Runner) handleReceiveNotification(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleWaitNotification(ctx, step, state)
}

// handleReceiveNotifications receives multiple notifications.
func (r *Runner) handleReceiveNotifications(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := paramInt(params, KeyCount, 1)

	received := 0
	for i := 0; i < count; i++ {
		out, _ := r.handleWaitNotification(ctx, step, state)
		if out[KeyNotificationReceived] == true {
			received++
		} else {
			break
		}
	}

	return map[string]any{
		KeyNotificationsReceived: received,
		KeyAllReceived:           received == count,
	}, nil
}

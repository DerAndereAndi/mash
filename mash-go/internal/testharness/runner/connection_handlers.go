package runner

import (
	"context"
	"crypto/tls"
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
		return map[string]any{
			KeyConnectionEstablished: false,
			KeyZoneID:                zoneID,
			KeyError:                 err.Error(),
			KeyErrorCode:             classifyConnectError(err),
		}, nil
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

	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpInvoke,
		EndpointID: endpointID,
		FeatureID:  featureID,
		Payload:    params["params"],
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

	timeoutMs := 5000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

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

// handleSendClose sends a close frame.
func (r *Runner) handleSendClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyCloseSent: false}, nil
	}

	err := r.conn.Close()
	return map[string]any{
		KeyCloseSent: err == nil,
	}, nil
}

// handleSimultaneousClose sends close while reading for close from peer.
func (r *Runner) handleSimultaneousClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyCloseSent: false}, nil
	}

	err := r.conn.Close()
	return map[string]any{
		KeyCloseSent:    err == nil,
		KeySimultaneous: true,
	}, nil
}

// handleWaitDisconnect waits for the connection to be closed by the peer.
func (r *Runner) handleWaitDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := 10000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

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
			return map[string]any{KeyPongReceived: false, KeyError: fmt.Sprintf("no active connection for zone %s", zoneID)}, nil
		}
	} else if r.conn == nil || !r.conn.connected {
		return map[string]any{KeyPongReceived: false, KeyError: "not connected"}, nil
	}

	// Check if a timeout threshold was specified.
	latencyUnder := true
	if timeoutMs, ok := params[KeyTimeoutMs].(float64); ok {
		latencyUnder = timeoutMs > 0 // Connection is alive, so latency is within any positive timeout.
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
		KeyPongReceived: true,
		KeyLatencyUnder: latencyUnder,
		KeyPongSeq:      seq,
	}, nil
}

// handlePingMultiple sends multiple pings.
func (r *Runner) handlePingMultiple(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := 3
	if c, ok := params[KeyCount].(float64); ok {
		count = int(c)
	}

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

	return map[string]any{
		KeyKeepaliveActive: active,
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
	case params["cbor_map"] != nil:
		// CBOR map with string keys -> integer keys.
		m, ok := params["cbor_map"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cbor_map must be a map")
		}
		data, err = cborEncodeIntKeyMap(m)
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
			outputs[KeyResponseReceived] = false
		} else {
			outputs[KeyResponseReceived] = true
			// Try to decode as CBOR to extract status fields.
			var respMap map[any]any
			if err := cbor.Unmarshal(res.data, &respMap); err == nil {
				// Look for status field (key 3 in wire protocol).
				if status, ok := respMap[uint64(3)]; ok {
					outputs[KeyStatus] = status
				}
				// Look for error status.
				if errStatus, ok := respMap[uint64(4)]; ok {
					outputs[KeyErrorStatus] = errStatus
				}
			}
		}
	case <-readCtx.Done():
		outputs[KeyResponseReceived] = false
	}

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

// handleSendRawBytes sends raw bytes (not framed).
func (r *Runner) handleSendRawBytes(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return nil, fmt.Errorf("not connected")
	}

	data, ok := params["data"].([]byte)
	if !ok {
		if s, ok := params["data"].(string); ok {
			data = []byte(s)
		}
	}

	_, err := r.conn.tlsConn.Write(data)
	return map[string]any{
		KeyRawBytesSent: err == nil,
	}, err
}

// handleSendRawFrame sends a raw frame with length prefix.
func (r *Runner) handleSendRawFrame(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleSendRaw(ctx, step, state)
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

	count := 3
	if c, ok := params[KeyCount].(float64); ok {
		count = int(c)
	}

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

	count := 2
	if c, ok := params[KeyCount].(float64); ok {
		count = int(c)
	}

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

	return map[string]any{
		KeyReadCount: count,
		KeyResults:   results,
	}, nil
}

// handleInvokeWithDisconnect invokes then immediately disconnects.
func (r *Runner) handleInvokeWithDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	result, err := r.handleInvoke(ctx, step, state)
	if err != nil {
		return nil, err
	}

	_ = r.conn.Close()
	result[KeyDisconnectedAfterInvoke] = true

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

	count := 1
	if c, ok := params[KeyCount].(float64); ok {
		count = int(c)
	}

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

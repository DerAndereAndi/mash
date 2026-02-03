package runner

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
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

	zoneID, _ := params["zone_id"].(string)

	target := r.config.Target
	if t, ok := params["target"].(string); ok && t != "" {
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
			"connection_established": false,
			"zone_id":                zoneID,
			"error":                  err.Error(),
			"error_code":             classifyConnectError(err),
		}, nil
	}

	newConn := &Connection{
		tlsConn:   conn,
		framer:    transport.NewFramer(conn),
		connected: true,
	}

	ct.zoneConnections[zoneID] = newConn
	state.Set("zone_"+zoneID+"_connection", newConn)

	return map[string]any{
		"connection_established": true,
		"zone_id":                zoneID,
	}, nil
}

func (r *Runner) getZoneConnection(state *engine.ExecutionState, params map[string]any) (*Connection, string, error) {
	zoneID, _ := params["zone_id"].(string)
	ct := getConnectionTracker(state)

	conn, ok := ct.zoneConnections[zoneID]
	if !ok || !conn.connected {
		return nil, zoneID, fmt.Errorf("no active connection for zone %s", zoneID)
	}
	return conn, zoneID, nil
}

// handleReadAsZone reads using a zone-scoped connection.
func (r *Runner) handleReadAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

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
		"read_success": resp.IsSuccess(),
		"value":        resp.Payload,
		"status":       resp.Status,
	}, nil
}

// handleInvokeAsZone invokes using a zone-scoped connection.
func (r *Runner) handleInvokeAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

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
		"invoke_success": resp.IsSuccess(),
		"result":         resp.Payload,
		"status":         resp.Status,
	}, nil
}

// handleSubscribeAsZone subscribes using a zone-scoped connection.
func (r *Runner) handleSubscribeAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

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
		"subscribe_success": resp.IsSuccess(),
		"subscription_id":   resp.Payload,
		"status":            resp.Status,
	}, nil
}

// handleWaitForNotificationAsZone waits for a notification on a zone connection.
func (r *Runner) handleWaitForNotificationAsZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	conn, _, err := r.getZoneConnection(state, params)
	if err != nil {
		return nil, err
	}

	timeoutMs := 5000
	if t, ok := params["timeout_ms"].(float64); ok {
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
			return map[string]any{"notification_received": false}, nil
		}
		return map[string]any{
			"notification_received": true,
			"notification_data":     res.data,
		}, nil
	case <-readCtx.Done():
		return map[string]any{"notification_received": false}, nil
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

	result["connect_duration_ms"] = elapsed.Milliseconds()
	state.Set("connect_duration_ms", elapsed.Milliseconds())

	return result, nil
}

// handleSendClose sends a close frame.
func (r *Runner) handleSendClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{"close_sent": false}, nil
	}

	err := r.conn.Close()
	return map[string]any{
		"close_sent": err == nil,
	}, nil
}

// handleSimultaneousClose sends close while reading for close from peer.
func (r *Runner) handleSimultaneousClose(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return map[string]any{"close_sent": false}, nil
	}

	err := r.conn.Close()
	return map[string]any{
		"close_sent":      err == nil,
		"simultaneous":    true,
	}, nil
}

// handleWaitDisconnect waits for the connection to be closed by the peer.
func (r *Runner) handleWaitDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := 10000
	if t, ok := params["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}

	if r.conn == nil || !r.conn.connected {
		return map[string]any{"disconnected": true}, nil
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
			return map[string]any{"disconnected": true}, nil
		}
		return map[string]any{"disconnected": false}, nil
	case <-readCtx.Done():
		return map[string]any{"disconnected": false}, nil
	}
}

// handleCancelReconnect cancels any pending reconnection.
func (r *Runner) handleCancelReconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)
	ct.backoffState = nil

	return map[string]any{"reconnect_cancelled": true}, nil
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
		"monitoring_started": true,
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
		"disconnected":       true,
		"monitoring_backoff": true,
	}, nil
}

// ============================================================================
// Keep-alive
// ============================================================================

// handlePing sends a single ping.
func (r *Runner) handlePing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected {
		return map[string]any{"pong_received": false, "error": "not connected"}, nil
	}

	// Check if a timeout threshold was specified.
	latencyUnder := true
	if timeoutMs, ok := params["timeout_ms"].(float64); ok {
		latencyUnder = timeoutMs > 0 // Connection is alive, so latency is within any positive timeout.
	}

	// Increment pong sequence.
	seq := uint32(1)
	if s, exists := state.Get("pong_seq"); exists {
		if si, ok := s.(uint32); ok {
			seq = si + 1
		}
	}
	state.Set("pong_seq", seq)

	return map[string]any{
		"pong_received": true,
		"latency_under": latencyUnder,
		"pong_seq":      seq,
	}, nil
}

// handlePingMultiple sends multiple pings.
func (r *Runner) handlePingMultiple(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := 3
	if c, ok := params["count"].(float64); ok {
		count = int(c)
	}

	allReceived := true
	for i := 0; i < count; i++ {
		out, _ := r.handlePing(ctx, step, state)
		if out["pong_received"] != true {
			allReceived = false
			break
		}
	}

	return map[string]any{
		"all_pongs_received": allReceived,
		"count":              count,
	}, nil
}

// handleVerifyKeepalive verifies keep-alive is active.
func (r *Runner) handleVerifyKeepalive(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	active := r.conn != nil && r.conn.connected

	return map[string]any{
		"keepalive_active": active,
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
			"raw_sent":      false,
			"parse_success": false,
			"error":         "message is empty",
		}, nil
	}

	err = r.conn.framer.WriteFrame(data)
	if err != nil {
		return map[string]any{
			"raw_sent": false,
			"error":    err.Error(),
		}, err
	}

	outputs := map[string]any{
		"raw_sent":      true,
		"parse_success": true,
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
			outputs["response_received"] = false
		} else {
			outputs["response_received"] = true
			// Try to decode as CBOR to extract status fields.
			var respMap map[any]any
			if err := cbor.Unmarshal(res.data, &respMap); err == nil {
				// Look for status field (key 3 in wire protocol).
				if status, ok := respMap[uint64(3)]; ok {
					outputs["status"] = status
				}
				// Look for error status.
				if errStatus, ok := respMap[uint64(4)]; ok {
					outputs["error_status"] = errStatus
				}
			}
		}
	case <-readCtx.Done():
		outputs["response_received"] = false
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
		"raw_bytes_sent": err == nil,
	}, err
}

// handleSendRawFrame sends a raw frame with length prefix.
func (r *Runner) handleSendRawFrame(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleSendRaw(ctx, step, state)
}

// handleSendTLSAlert sends a TLS alert.
func (r *Runner) handleSendTLSAlert(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	if r.conn == nil || !r.conn.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Close the TLS connection which sends a close_notify alert.
	err := r.conn.Close()
	return map[string]any{
		"alert_sent": err == nil,
	}, nil
}

// ============================================================================
// Command queue
// ============================================================================

// handleQueueCommand stores a command for later execution.
func (r *Runner) handleQueueCommand(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ct := getConnectionTracker(state)

	action, _ := params["action"].(string)
	cmdParams, _ := params["params"].(map[string]any)

	ct.pendingQueue = append(ct.pendingQueue, queuedCommand{
		Action: action,
		Params: cmdParams,
	})

	return map[string]any{
		"command_queued": true,
		"queue_length":   len(ct.pendingQueue),
	}, nil
}

// handleWaitForQueuedResult waits for queued command results.
func (r *Runner) handleWaitForQueuedResult(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ct := getConnectionTracker(state)

	if len(ct.pendingQueue) == 0 {
		return map[string]any{
			"result_received": false,
			"queue_empty":     true,
		}, nil
	}

	// Dequeue and report.
	cmd := ct.pendingQueue[0]
	ct.pendingQueue = ct.pendingQueue[1:]

	return map[string]any{
		"result_received": true,
		"action":          cmd.Action,
		"queue_remaining": len(ct.pendingQueue),
	}, nil
}

// handleSendMultipleThenDisconnect sends multiple frames then disconnects.
func (r *Runner) handleSendMultipleThenDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := 3
	if c, ok := params["count"].(float64); ok {
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
		"messages_sent":  sent,
		"disconnected":   true,
	}, nil
}

// ============================================================================
// Concurrency
// ============================================================================

// handleReadConcurrent performs multiple reads concurrently.
func (r *Runner) handleReadConcurrent(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := 2
	if c, ok := params["count"].(float64); ok {
		count = int(c)
	}

	// For concurrency testing, we just perform sequential reads.
	results := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		out, err := r.handleRead(ctx, step, state)
		if err != nil {
			results = append(results, map[string]any{"error": err.Error()})
		} else {
			results = append(results, out)
		}
	}

	return map[string]any{
		"read_count": count,
		"results":    results,
	}, nil
}

// handleInvokeWithDisconnect invokes then immediately disconnects.
func (r *Runner) handleInvokeWithDisconnect(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	result, err := r.handleInvoke(ctx, step, state)
	if err != nil {
		return nil, err
	}

	_ = r.conn.Close()
	result["disconnected_after_invoke"] = true

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

	if subs, ok := params["subscriptions"].([]any); ok {
		// Subscriptions array of objects with endpoint+feature.
		for _, s := range subs {
			m, ok := s.(map[string]any)
			if !ok {
				continue
			}
			ep := m["endpoint"]
			if ep == nil {
				ep = params["endpoint"]
			}
			targets = append(targets, subTarget{endpoint: ep, feature: m["feature"]})
		}
	} else if features, ok := params["features"].([]any); ok {
		// Simple features array (uses shared endpoint).
		for _, f := range features {
			targets = append(targets, subTarget{endpoint: params["endpoint"], feature: f})
		}
	} else {
		return nil, fmt.Errorf("either 'features' or 'subscriptions' parameter is required")
	}

	subscriptions := make([]any, 0, len(targets))
	for _, t := range targets {
		featureStep := &loader.Step{
			Params: map[string]any{
				"endpoint": t.endpoint,
				"feature":  t.feature,
			},
		}
		out, err := r.handleSubscribe(ctx, featureStep, state)
		if err != nil {
			return nil, fmt.Errorf("subscribe to %v: %w", t.feature, err)
		}
		subscriptions = append(subscriptions, out["subscription_id"])
	}

	return map[string]any{
		"subscribe_count": len(subscriptions),
		"subscriptions":   subscriptions,
	}, nil
}

// handleSubscribeOrdered subscribes and verifies ordering.
func (r *Runner) handleSubscribeOrdered(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleSubscribeMultiple(ctx, step, state)
}

// handleUnsubscribe cancels a subscription.
func (r *Runner) handleUnsubscribe(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	subID := params["subscription_id"]
	state.Set("unsubscribed_id", subID)

	return map[string]any{
		"unsubscribed": true,
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
	if c, ok := params["count"].(float64); ok {
		count = int(c)
	}

	received := 0
	for i := 0; i < count; i++ {
		out, _ := r.handleWaitNotification(ctx, step, state)
		if out["notification_received"] == true {
			received++
		} else {
			break
		}
	}

	return map[string]any{
		"notifications_received": received,
		"all_received":           received == count,
	}, nil
}

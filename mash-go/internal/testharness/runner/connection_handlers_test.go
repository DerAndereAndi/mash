package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleQueueCommand(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Queue a command.
	step := &loader.Step{
		Params: map[string]any{
			"action": "read",
			"params": map[string]any{"endpoint": "1", "feature": "measurement"},
		},
	}
	out, err := r.handleQueueCommand(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["command_queued"] != true {
		t.Error("expected command_queued=true")
	}
	if out["queue_length"] != 1 {
		t.Errorf("expected queue_length=1, got %v", out["queue_length"])
	}

	// Queue another.
	_, _ = r.handleQueueCommand(context.Background(), step, state)

	ct := getConnectionTracker(state)
	if len(ct.pendingQueue) != 2 {
		t.Errorf("expected 2 queued commands, got %d", len(ct.pendingQueue))
	}
}

func TestHandleWaitForQueuedResult(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Empty queue.
	out, _ := r.handleWaitForQueuedResult(context.Background(), &loader.Step{}, state)
	if out["queue_empty"] != true {
		t.Error("expected queue_empty=true")
	}

	// Add and dequeue.
	ct := getConnectionTracker(state)
	ct.pendingQueue = append(ct.pendingQueue, queuedCommand{Action: "read"})

	out, _ = r.handleWaitForQueuedResult(context.Background(), &loader.Step{}, state)
	if out["result_received"] != true {
		t.Error("expected result_received=true")
	}
	if out["action"] != "read" {
		t.Errorf("expected action=read, got %v", out["action"])
	}
	if len(ct.pendingQueue) != 0 {
		t.Error("expected queue to be empty after dequeue")
	}
}

func TestHandleMonitorReconnect(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleMonitorReconnect(context.Background(), &loader.Step{}, state)
	if out["monitoring_started"] != true {
		t.Error("expected monitoring_started=true")
	}

	ct := getConnectionTracker(state)
	if ct.backoffState == nil || !ct.backoffState.Monitoring {
		t.Error("expected backoff state to be monitoring")
	}
}

func TestHandleCancelReconnect(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	ct := getConnectionTracker(state)
	ct.backoffState = &backoffTracker{Monitoring: true}

	out, _ := r.handleCancelReconnect(context.Background(), &loader.Step{}, state)
	if out["reconnect_cancelled"] != true {
		t.Error("expected reconnect_cancelled=true")
	}
	if ct.backoffState != nil {
		t.Error("expected backoffState to be nil after cancel")
	}
}

func TestHandlePing(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Not connected -> pong_received false.
	out, _ := r.handlePing(context.Background(), &loader.Step{}, state)
	if out["pong_received"] != false {
		t.Error("expected pong_received=false when not connected")
	}

	// Simulate connected.
	r.conn.connected = true
	out, _ = r.handlePing(context.Background(), &loader.Step{}, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true when connected")
	}
}

func TestHandlePing_EnrichedFields(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	// First ping should set pong_seq=1.
	step := &loader.Step{Params: map[string]any{"timeout_ms": float64(5000)}}
	out, _ := r.handlePing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true")
	}
	if out["latency_under"] != true {
		t.Error("expected latency_under=true")
	}
	if out["pong_seq"] != uint32(1) {
		t.Errorf("expected pong_seq=1, got %v (%T)", out["pong_seq"], out["pong_seq"])
	}

	// Second ping should increment to 2.
	out, _ = r.handlePing(context.Background(), step, state)
	if out["pong_seq"] != uint32(2) {
		t.Errorf("expected pong_seq=2, got %v", out["pong_seq"])
	}
}

func TestHandlePingMultiple(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"count": float64(5)}}
	out, _ := r.handlePingMultiple(context.Background(), step, state)
	if out["all_pongs_received"] != true {
		t.Error("expected all_pongs_received=true")
	}
	if out["count"] != 5 {
		t.Errorf("expected count=5, got %v", out["count"])
	}
}

func TestHandleVerifyKeepalive(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, _ := r.handleVerifyKeepalive(context.Background(), &loader.Step{}, state)
	if out["keepalive_active"] != false {
		t.Error("expected keepalive_active=false when not connected")
	}

	r.conn.connected = true
	out, _ = r.handleVerifyKeepalive(context.Background(), &loader.Step{}, state)
	if out["keepalive_active"] != true {
		t.Error("expected keepalive_active=true when connected")
	}
}

func TestHandleSendClose(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Not connected.
	out, _ := r.handleSendClose(context.Background(), &loader.Step{}, state)
	if out["close_sent"] != false {
		t.Error("expected close_sent=false when not connected")
	}
}

func TestHandleUnsubscribe(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"subscription_id": "sub-123"}}
	out, _ := r.handleUnsubscribe(context.Background(), step, state)
	if out["unsubscribed"] != true {
		t.Error("expected unsubscribed=true")
	}

	id, _ := state.Get("unsubscribed_id")
	if id != "sub-123" {
		t.Errorf("expected sub-123 in state, got %v", id)
	}
}

// ============================================================================
// Phase 3: CBOR encoding tests
// ============================================================================

func TestCborEncodeIntKeyMap_IntegerKeys(t *testing.T) {
	m := map[string]any{"1": 5, "2": "hello"}
	data, err := cborEncodeIntKeyMap(m)
	if err != nil {
		t.Fatalf("cborEncodeIntKeyMap: %v", err)
	}

	// Decode and verify keys are integers.
	var decoded map[any]any
	if err := cbor.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// CBOR integer keys can decode as various int types.
	found1 := false
	found2 := false
	for k, v := range decoded {
		switch ki := k.(type) {
		case int64:
			if ki == 1 {
				found1 = true
				if vi, ok := v.(uint64); !ok || vi != 5 {
					t.Errorf("key 1: expected 5, got %v (%T)", v, v)
				}
			} else if ki == 2 {
				found2 = true
				if vs, ok := v.(string); !ok || vs != "hello" {
					t.Errorf("key 2: expected 'hello', got %v (%T)", v, v)
				}
			}
		case uint64:
			if ki == 1 {
				found1 = true
			} else if ki == 2 {
				found2 = true
			}
		}
	}

	if !found1 || !found2 {
		t.Errorf("expected integer keys 1 and 2, got decoded map: %v", decoded)
	}
}

func TestCborEncodeIntKeyMap_NestedMaps(t *testing.T) {
	m := map[string]any{
		"1": map[string]any{"10": "inner"},
	}
	data, err := cborEncodeIntKeyMap(m)
	if err != nil {
		t.Fatalf("cborEncodeIntKeyMap: %v", err)
	}

	// Should produce valid CBOR.
	var decoded any
	if err := cbor.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

func TestCborEncodeIntKeyMap_StringKeys(t *testing.T) {
	// Non-numeric keys should be preserved as strings.
	m := map[string]any{"name": "test"}
	data, err := cborEncodeIntKeyMap(m)
	if err != nil {
		t.Fatalf("cborEncodeIntKeyMap: %v", err)
	}

	var decoded map[any]any
	if err := cbor.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded["name"] != "test" {
		t.Errorf("expected string key 'name' preserved, got %v", decoded)
	}
}

func TestHandleSendRaw_EmptyData(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	// No data params at all -> empty message error.
	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["raw_sent"] != false {
		t.Error("expected raw_sent=false for empty data")
	}
	if out[KeyError] != "message is empty" {
		t.Errorf("expected 'message is empty' error, got %v", out[KeyError])
	}
}

func TestHandleSendRaw_NotConnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"data": "test"}}
	_, err := r.handleSendRaw(context.Background(), step, state)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestHandleSendRaw_HexDecode(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	// Invalid hex should error.
	step := &loader.Step{Params: map[string]any{"cbor_bytes_hex": "gg"}}
	_, err := r.handleSendRaw(context.Background(), step, state)
	if err == nil {
		t.Error("expected error for invalid hex")
	}
}

// ============================================================================
// Phase 7: subscribe_multiple param format tests
// ============================================================================

func TestHandleConnectAsZone_ErrorOutputs(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "127.0.0.1:1" // unreachable port
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		KeyZoneID: "zone-test",
		"target":  "127.0.0.1:1",
	}}
	out, err := r.handleConnectAsZone(context.Background(), step, state)
	// Should return output map, not an error.
	if err != nil {
		t.Fatalf("expected nil error (output map), got: %v", err)
	}
	if out[KeyConnectionEstablished] != false {
		t.Error("expected connection_established=false")
	}
	if out[KeyZoneID] != "zone-test" {
		t.Errorf("expected zone_id=zone-test, got %v", out[KeyZoneID])
	}
	if _, ok := out["error_code"]; !ok {
		t.Error("expected error_code key in output")
	}
}

// C2: handleConnect constructs target from host+port params.
func TestHandleConnect_HostPortParams(t *testing.T) {
	r := newTestRunner()
	r.config.Target = "original:9999"
	state := newTestState()

	// host+port should override config target.
	step := &loader.Step{Params: map[string]any{
		"host": "192.0.2.1",
		"port": 8443,
	}}
	out, _ := r.handleConnect(context.Background(), step, state)
	// Connection will fail (unreachable), but the target in output should reflect host:port.
	if out["target"] != "192.0.2.1:8443" {
		t.Errorf("expected target=192.0.2.1:8443, got %v", out["target"])
	}
}

// C2: handleConnect with host+port using int port (YAML integer).
func TestHandleConnect_HostPortIntPort(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		"host": "10.0.0.1",
		"port": int(9443), // YAML int type
	}}
	out, _ := r.handleConnect(context.Background(), step, state)
	if out["target"] != "10.0.0.1:9443" {
		t.Errorf("expected target=10.0.0.1:9443, got %v", out["target"])
	}
}

// C4: connect_as_zone success output contains state: OPERATIONAL.
func TestConnectAsZone_StateOutput(t *testing.T) {
	// We can't easily test a successful connection without a real server,
	// but we can verify the zone limit logic.
	r := newTestRunner()
	state := newTestState()

	// Fill up 5 zone connections.
	ct := getConnectionTracker(state)
	for i := 0; i < 5; i++ {
		ct.zoneConnections[fmt.Sprintf("zone%d", i)] = &Connection{connected: true}
	}

	// 6th zone should be rejected (C5).
	step := &loader.Step{Params: map[string]any{
		KeyZoneID: "zone5",
	}}
	out, err := r.handleConnectAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyConnectionEstablished] != false {
		t.Error("expected connection_established=false for 6th zone")
	}
	if out[KeyErrorCode] != ErrCodeMaxConnsExceeded {
		t.Errorf("expected error_code=MAX_CONNECTIONS_EXCEEDED, got %v", out[KeyErrorCode])
	}
}

// C5: Zone limit enforcement allows exactly 5.
func TestConnectAsZone_ZoneLimit(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	ct := getConnectionTracker(state)

	// With fewer than 5 zones, the limit check should not trigger.
	// (Connection will still fail due to no real server, but the error
	// should NOT be MAX_CONNECTIONS_EXCEEDED.)
	ct.zoneConnections["zone0"] = &Connection{connected: true}
	step := &loader.Step{Params: map[string]any{
		KeyZoneID: "zone1",
		"target":  "127.0.0.1:1",
	}}
	out, _ := r.handleConnectAsZone(context.Background(), step, state)
	if out[KeyErrorCode] == ErrCodeMaxConnsExceeded {
		t.Error("should not hit zone limit with only 1 existing zone")
	}
}

// C10: send_ping routes to zone-specific connection.
func TestPing_ZoneRouting(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up a zone connection in the tracker.
	ct := getConnectionTracker(state)
	zoneConn := &Connection{connected: true}
	ct.zoneConnections["zone1"] = zoneConn

	// Ping with connection=zone1 should find the zone connection.
	step := &loader.Step{Params: map[string]any{"connection": "zone1"}}
	out, _ := r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true when zone connection exists")
	}

	// Ping with zone=zone1 should also work (alias).
	step = &loader.Step{Params: map[string]any{"zone": "zone1"}}
	out, _ = r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true with zone param")
	}

	// Non-existent zone falls back to main connection.
	r.conn.connected = true
	step = &loader.Step{Params: map[string]any{"connection": "nonexistent"}}
	out, _ = r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true (fallback to main connection)")
	}

	// No connection at all.
	r.conn.connected = false
	step = &loader.Step{Params: map[string]any{"connection": "nonexistent"}}
	out, _ = r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != false {
		t.Error("expected pong_received=false when no connection available")
	}
}

// C10: send_ping without connection param uses main connection.
func TestPing_FallbackToMainConnection(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	// No connection/zone param -> falls back to r.conn.
	step := &loader.Step{Params: map[string]any{}}
	out, _ := r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true with main connection fallback")
	}
}

func TestSubscribeMultiple_NeitherParam(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		"endpoint": float64(1),
	}}
	_, err := r.handleSubscribeMultiple(context.Background(), step, state)
	if err == nil {
		t.Error("expected error when neither features nor subscriptions provided")
	}
}

package runner

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
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
	r.conn.state = ConnOperational
	out, _ = r.handlePing(context.Background(), &loader.Step{}, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true when connected")
	}
}

func TestHandlePing_EnrichedFields(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
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
	r.conn.state = ConnOperational
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

	r.conn.state = ConnOperational
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

	// Non-numeric subscription_id should fail validation.
	step := &loader.Step{Params: map[string]any{"subscription_id": "sub-123"}}
	out, _ := r.handleUnsubscribe(context.Background(), step, state)
	if out["unsubscribe_success"] != false {
		t.Error("expected unsubscribe_success=false for non-numeric subscription_id")
	}

	// Numeric subscription_id should proceed (but fail at send because not connected).
	step2 := &loader.Step{Params: map[string]any{"subscription_id": 42}}
	out2, _ := r.handleUnsubscribe(context.Background(), step2, state)
	if out2["unsubscribe_success"] != false {
		t.Error("expected unsubscribe_success=false when not connected")
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
	r.conn.state = ConnOperational
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
	r.conn.state = ConnOperational
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
		ct.zoneConnections[fmt.Sprintf("zone%d", i)] = &Connection{state: ConnOperational}
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
	ct.zoneConnections["zone0"] = &Connection{state: ConnOperational}
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
	zoneConn := &Connection{state: ConnOperational}
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
	r.conn.state = ConnOperational
	step = &loader.Step{Params: map[string]any{"connection": "nonexistent"}}
	out, _ = r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true (fallback to main connection)")
	}

	// No connection at all.
	r.conn.state = ConnDisconnected
	step = &loader.Step{Params: map[string]any{"connection": "nonexistent"}}
	out, _ = r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != false {
		t.Error("expected pong_received=false when no connection available")
	}
}

// C10: send_ping without connection param uses main connection.
func TestPing_FallbackToMainConnection(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	state := newTestState()

	// No connection/zone param -> falls back to r.conn.
	step := &loader.Step{Params: map[string]any{}}
	out, _ := r.handleSendPing(context.Background(), step, state)
	if out["pong_received"] != true {
		t.Error("expected pong_received=true with main connection fallback")
	}
}

func TestSubscribeAsZone_DummyConnection(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up a dummy zone connection (connected but no framer).
	ct := getConnectionTracker(state)
	ct.zoneConnections["GRID"] = &Connection{state: ConnOperational}

	step := &loader.Step{Params: map[string]any{
		KeyZoneID:   "GRID",
		KeyEndpoint: float64(1),
		KeyFeature:  "measurement",
	}}
	out, err := r.handleSubscribeAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error for dummy connection: %v", err)
	}
	if out[KeySubscribeSuccess] != true {
		t.Error("expected subscribe_success=true for dummy connection")
	}
	if out[KeySubscriptionID] == nil || out[KeySubscriptionID] == "" {
		t.Error("expected non-empty subscription_id for dummy connection")
	}
}

func TestReadAsZone_DummyConnection(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up a dummy zone connection (connected but no framer).
	ct := getConnectionTracker(state)
	ct.zoneConnections["LOCAL"] = &Connection{state: ConnOperational}

	step := &loader.Step{Params: map[string]any{
		KeyZoneID:   "LOCAL",
		KeyEndpoint: float64(1),
		KeyFeature:  "measurement",
	}}
	out, err := r.handleReadAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error for dummy connection: %v", err)
	}
	if out[KeyReadSuccess] != true {
		t.Error("expected read_success=true for dummy connection")
	}
}

func TestWaitForNotificationAsZone_DummyConnection(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up a dummy zone connection (connected but no framer).
	ct := getConnectionTracker(state)
	ct.zoneConnections["GRID"] = &Connection{state: ConnOperational}

	step := &loader.Step{Params: map[string]any{
		KeyZoneID: "GRID",
	}}
	out, err := r.handleWaitForNotificationAsZone(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error for dummy connection: %v", err)
	}
	if out[KeyNotificationReceived] != true {
		t.Error("expected notification_received=true for dummy connection")
	}
}

func TestReadAsZone_AfterOtherZoneDisconnect(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set up dummy connections for GRID and LOCAL.
	ct := getConnectionTracker(state)
	ct.zoneConnections["GRID"] = &Connection{state: ConnOperational}
	ct.zoneConnections["LOCAL"] = &Connection{state: ConnOperational}

	// Disconnect GRID.
	disconnectStep := &loader.Step{Params: map[string]any{"zone": "GRID"}}
	_, err := r.handleDisconnectZone(context.Background(), disconnectStep, state)
	if err != nil {
		t.Fatalf("unexpected error disconnecting GRID: %v", err)
	}

	// Read as LOCAL -- should still succeed (dummy connection).
	readStep := &loader.Step{Params: map[string]any{
		KeyZoneID:   "LOCAL",
		KeyEndpoint: float64(1),
		KeyFeature:  "measurement",
	}}
	out, err := r.handleReadAsZone(context.Background(), readStep, state)
	if err != nil {
		t.Fatalf("unexpected error reading as LOCAL: %v", err)
	}
	if out[KeyReadSuccess] != true {
		t.Error("expected read_success=true for LOCAL after GRID disconnect")
	}
}

func TestSubscribeMultiple_NeitherParam(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	state := newTestState()

	step := &loader.Step{Params: map[string]any{
		"endpoint": float64(1),
	}}
	_, err := r.handleSubscribeMultiple(context.Background(), step, state)
	if err == nil {
		t.Error("expected error when neither features nor subscriptions provided")
	}
}

// ============================================================================
// Helper: set up a Runner with a net.Pipe-backed framer for raw wire tests.
// Returns the runner and the server-side of the pipe (for reading/writing).
// ============================================================================

func newPipedRunner() (*Runner, net.Conn) {
	client, server := net.Pipe()
	r := newTestRunner()
	r.conn = &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}
	return r, server
}

// serverEchoResponse reads one frame from the server side, decodes the request
// messageID, and writes back a success response with the same messageID.
func serverEchoResponse(server net.Conn) {
	framer := transport.NewFramer(server)
	reqData, err := framer.ReadFrame()
	if err != nil {
		return
	}
	// Decode messageID from raw CBOR (key 1).
	var raw map[any]any
	if err := cbor.Unmarshal(reqData, &raw); err != nil {
		return
	}
	var msgID uint32
	for k, v := range raw {
		if ki, ok := wire.ToUint32(k); ok && ki == 1 {
			if vi, ok := wire.ToUint32(v); ok {
				msgID = vi
			}
		}
	}
	resp := &wire.Response{MessageID: msgID, Status: wire.StatusSuccess}
	respData, _ := wire.EncodeResponse(resp)
	_ = framer.WriteFrame(respData)
}

// ============================================================================
// Phase 1 (RC1): YAML integer keys in cbor_map
// ============================================================================

func TestHandleSendRaw_CborMapIntegerKeys(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	// map[any]any with int keys -- what yaml.v3 produces for {1: 1, 2: 1, 3: 0, 4: 1}.
	step := &loader.Step{Params: map[string]any{
		"cbor_map": map[any]any{1: 1, 2: 1, 3: 0, 4: 1},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
}

func TestHandleSendRaw_CborMapIntKeys(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	// map[int]any -- alternate type.
	step := &loader.Step{Params: map[string]any{
		"cbor_map": map[int]any{1: 1, 2: 1, 3: 0, 4: 1},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
}

func TestHandleSendRaw_CborMapMixedNestedKeys(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	// map[any]any with extra unknown key (TC-CBOR-004 scenario).
	step := &loader.Step{Params: map[string]any{
		"cbor_map": map[any]any{1: 1, 2: 1, 3: 0, 4: 1, 99: "future"},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
}

// ============================================================================
// Phase 2 (RC4): message_type + value params in send_raw
// ============================================================================

func TestHandleSendRaw_AttributeValueMaxInt64(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"message_type": "attribute_value",
		"value":        int64(9223372036854775807),
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
}

func TestHandleSendRaw_AttributeValueNegativeInt64(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"message_type": "attribute_value",
		"value":        int64(-9223372036854775808),
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
}

// ============================================================================
// Phase 3 (RC2): send_raw_frame tests
// ============================================================================

func TestHandleSendRawFrame_ValidSmallPayload(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		data, err := framer.ReadFrame()
		if err != nil {
			return
		}
		if len(data) < 200 {
			t.Errorf("expected payload >= 200 bytes, got %d", len(data))
		}
		// Send success response.
		resp := &wire.Response{MessageID: 1, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"payload_size": float64(256),
		"valid_cbor":   true,
	}}
	out, err := r.handleSendRawFrame(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
	if out[KeyResponseReceived] != true {
		t.Errorf("expected response_received=true, got %v", out[KeyResponseReceived])
	}
}

func TestHandleSendRawFrame_MaxSizePayload(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		framer := transport.NewFramerWithMaxSize(server, 70000)
		_, err := framer.ReadFrame()
		if err != nil {
			return
		}
		resp := &wire.Response{MessageID: 1, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"payload_size": float64(65536),
		"valid_cbor":   true,
	}}
	out, err := r.handleSendRawFrame(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
}

func TestHandleSendRawFrame_OversizedLength(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		// Read the raw 4-byte length prefix.
		var lenBuf [4]byte
		if _, err := io.ReadFull(server, lenBuf[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(lenBuf[:])
		if length != 65537 {
			t.Errorf("expected length_override=65537, got %d", length)
		}
		// Close connection to signal rejection.
		server.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"length_override": float64(65537),
		"payload_size":    float64(0),
	}}
	out, err := r.handleSendRawFrame(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyConnectionClosed] != true {
		t.Errorf("expected connection_closed=true, got %v", out[KeyConnectionClosed])
	}
}

func TestHandleSendRawFrame_ZeroLength(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		var lenBuf [4]byte
		if _, err := io.ReadFull(server, lenBuf[:]); err != nil {
			return
		}
		length := binary.BigEndian.Uint32(lenBuf[:])
		if length != 0 {
			t.Errorf("expected length_override=0, got %d", length)
		}
		server.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"length_override": float64(0),
	}}
	out, err := r.handleSendRawFrame(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyConnectionClosed] != true {
		t.Errorf("expected connection_closed=true, got %v", out[KeyConnectionClosed])
	}
}

// ============================================================================
// Phase 4 (RC3): send_raw_bytes tests
// ============================================================================

func TestHandleSendRawBytes_BytesHex(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.conn.tlsConn = nil // raw bytes uses getWriteConn which falls back to conn
	state := newTestState()

	go func() {
		buf := make([]byte, 3)
		_, _ = io.ReadFull(server, buf)
	}()

	step := &loader.Step{Params: map[string]any{
		"bytes_hex": "000001",
	}}
	out, err := r.handleSendRawBytes(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawBytesSent] != true {
		t.Errorf("expected raw_bytes_sent=true, got %v", out[KeyRawBytesSent])
	}
	if out[KeyConnectionOpen] != true {
		t.Errorf("expected connection_open=true, got %v", out[KeyConnectionOpen])
	}
}

func TestHandleSendRawBytes_FollowedByCborPayload(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.conn.tlsConn = nil
	state := newTestState()

	go func() {
		// Read the raw byte prefix.
		buf := make([]byte, 1)
		_, _ = io.ReadFull(server, buf)
		// Then read the framed CBOR payload.
		framer := transport.NewFramer(server)
		data, err := framer.ReadFrame()
		if err != nil {
			return
		}
		_ = data
		// Write back a success response.
		resp := &wire.Response{MessageID: 1, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"bytes_hex":                "05",
		"followed_by_cbor_payload": true,
		"payload_size":             float64(5),
	}}
	out, err := r.handleSendRawBytes(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawBytesSent] != true {
		t.Errorf("expected raw_bytes_sent=true, got %v", out[KeyRawBytesSent])
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
}

func TestHandleSendRawBytes_FollowedByBytes(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.conn.tlsConn = nil
	state := newTestState()

	go func() {
		// Read the 4-byte length prefix + 5 bytes of padding.
		buf := make([]byte, 4+5)
		_, _ = io.ReadFull(server, buf)
	}()

	step := &loader.Step{Params: map[string]any{
		"bytes_hex":        "00000010",
		"followed_by_bytes": float64(5),
	}}
	out, err := r.handleSendRawBytes(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawBytesSent] != true {
		t.Errorf("expected raw_bytes_sent=true, got %v", out[KeyRawBytesSent])
	}
	if out[KeyConnectionOpen] != true {
		t.Errorf("expected connection_open=true, got %v", out[KeyConnectionOpen])
	}
}

func TestHandleSendRawBytes_RemainingBytes(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.conn.tlsConn = nil
	state := newTestState()

	go func() {
		// Read the framed payload (length prefix + remaining bytes).
		framer := transport.NewFramer(server)
		_, err := framer.ReadFrame()
		if err != nil {
			return
		}
		// Write back a success response.
		resp := &wire.Response{MessageID: 1, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"remaining_bytes": float64(11),
	}}
	out, err := r.handleSendRawBytes(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawBytesSent] != true {
		t.Errorf("expected raw_bytes_sent=true, got %v", out[KeyRawBytesSent])
	}
	if out[KeyParseSuccess] != true {
		t.Errorf("expected parse_success=true, got %v", out[KeyParseSuccess])
	}
}

// ============================================================================
// Phase 5 (RC5): Missing output keys
// ============================================================================

// RC5a: response_message_id in handleRead.
func TestHandleRead_ResponseMessageID(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		reqData, err := framer.ReadFrame()
		if err != nil {
			return
		}
		req, err := wire.DecodeRequest(reqData)
		if err != nil {
			return
		}
		resp := &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusSuccess,
			Payload:   map[uint16]any{1: "test-vendor"},
		}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"endpoint":  float64(0),
		"feature":   "DeviceInfo",
		"attribute": "vendorName",
	}}
	out, err := r.handleRead(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyReadSuccess] != true {
		t.Errorf("expected read_success=true, got %v", out[KeyReadSuccess])
	}
	if out[KeyResponseMessageID] == nil {
		t.Error("expected response_message_id to be present")
	}
}

// RC5a: error response preserves message ID (TC-MSG-005).
func TestHandleRead_ErrorResponsePreservesMessageID(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		reqData, err := framer.ReadFrame()
		if err != nil {
			return
		}
		req, err := wire.DecodeRequest(reqData)
		if err != nil {
			return
		}
		resp := &wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusInvalidEndpoint,
		}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"endpoint": float64(255),
		"feature":  "DeviceInfo",
	}}
	out, err := r.handleRead(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyResponseMessageID] == nil {
		t.Error("expected response_message_id on error response")
	}
	if out[KeyReadSuccess] != false {
		t.Errorf("expected read_success=false for invalid endpoint, got %v", out[KeyReadSuccess])
	}
	if out[KeyErrorCode] == nil {
		t.Error("expected error_code for error response")
	}
}

// RC5b: MessageID=0 should result in connection_error.
func TestHandleRead_MessageIDZeroRejected(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	// Server just reads and closes -- test expects local rejection.
	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		server.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"message_id": float64(0),
		"endpoint":   float64(0),
		"feature":    "DeviceInfo",
		"attribute":  "vendorName",
	}}
	out, err := r.handleRead(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyConnectionError] != true {
		t.Errorf("expected connection_error=true for MessageID=0, got %v", out[KeyConnectionError])
	}
}

// RC5c: simulate_no_response causes timeout.
func TestHandleRead_SimulateNoResponse(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	// Server reads the request but never responds.
	// Closing server in defer will unblock the read.
	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		// Don't respond -- let context timeout trigger.
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	step := &loader.Step{Params: map[string]any{
		"simulate_no_response": true,
		"endpoint":             float64(0),
		"feature":              "DeviceInfo",
		"attribute":            "vendorName",
	}}
	out, err := r.handleRead(ctx, step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyReadSuccess] != false {
		t.Errorf("expected read_success=false, got %v", out[KeyReadSuccess])
	}
	errVal, _ := out[KeyError].(string)
	if errVal != "TIMEOUT" {
		t.Errorf("expected error=TIMEOUT, got %v", out[KeyError])
	}
	if out[KeyTimeoutAfter] == nil {
		t.Error("expected timeout_after to be present")
	}
}

// RC5d: handleReadConcurrent outputs correlation keys.
func TestHandleReadConcurrent_CorrelationOutputs(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	// Server echoes matching MessageIDs for 3 sequential requests.
	go func() {
		framer := transport.NewFramer(server)
		for i := 0; i < 3; i++ {
			reqData, err := framer.ReadFrame()
			if err != nil {
				return
			}
			req, err := wire.DecodeRequest(reqData)
			if err != nil {
				return
			}
			resp := &wire.Response{
				MessageID: req.MessageID,
				Status:    wire.StatusSuccess,
				Payload:   map[uint16]any{1: "test"},
			}
			respData, _ := wire.EncodeResponse(resp)
			_ = framer.WriteFrame(respData)
		}
	}()

	step := &loader.Step{Params: map[string]any{
		"count":    float64(3),
		"endpoint": float64(0),
		"feature":  "DeviceInfo",
	}}
	out, err := r.handleReadConcurrent(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyAllResponsesReceived] != true {
		t.Errorf("expected all_responses_received=true, got %v", out[KeyAllResponsesReceived])
	}
	if out[KeyAllCorrelationsCorrect] != true {
		t.Errorf("expected all_correlations_correct=true, got %v", out[KeyAllCorrelationsCorrect])
	}
}

// RC5e: error status extraction in handleSendRaw.
func TestHandleSendRaw_ErrorStatusExtraction(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		reqData, err := framer.ReadFrame()
		if err != nil {
			return
		}
		_ = reqData
		// Send error response.
		resp := &wire.Response{MessageID: 1, Status: wire.StatusInvalidParameter}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"cbor_map": map[string]any{"1": 1, "2": 1, "3": 0, "4": 1},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyResponseReceived] != true {
		t.Errorf("expected response_received=true, got %v", out[KeyResponseReceived])
	}
	if out[KeyErrorStatus] == nil {
		t.Error("expected error_status to be present")
	}
}

// RC5e: connection closed during send_raw response read.
func TestHandleSendRaw_NoResponseConnectionClosed(t *testing.T) {
	r, server := newPipedRunner()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		// Close immediately without responding.
		server.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"cbor_map": map[string]any{"1": 1, "2": 1, "3": 0, "4": 1},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyResponseReceived] != false {
		t.Errorf("expected response_received=false, got %v", out[KeyResponseReceived])
	}
}

// RC5e: cbor_map_string_keys sends string-keyed CBOR (intentionally invalid).
func TestHandleSendRaw_CborMapStringKeys(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		resp := &wire.Response{MessageID: 1, Status: wire.StatusInvalidParameter}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"cbor_map_string_keys": map[string]any{"messageId": 1, "operation": 1},
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Errorf("expected raw_sent=true, got %v", out[KeyRawSent])
	}
}

// RC5f: handleWrite with nil value.
func TestHandleWrite_NullValue(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	r.resolver = NewResolver()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		reqData, err := framer.ReadFrame()
		if err != nil {
			return
		}
		req, err := wire.DecodeRequest(reqData)
		if err != nil {
			return
		}
		resp := &wire.Response{MessageID: req.MessageID, Status: wire.StatusSuccess}
		respData, _ := wire.EncodeResponse(resp)
		_ = framer.WriteFrame(respData)
	}()

	step := &loader.Step{Params: map[string]any{
		"endpoint":  float64(1),
		"feature":   "EnergyControl",
		"attribute": "myConsumptionLimit",
		"value":     nil,
	}}
	out, err := r.handleWrite(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyWriteSuccess] != true {
		t.Errorf("expected write_success=true, got %v", out[KeyWriteSuccess])
	}
}

// RC5f: handleWrite returns output map (not Go error) on EOF.
func TestHandleWrite_EOF(t *testing.T) {
	r, server := newPipedRunner()
	r.resolver = NewResolver()
	state := newTestState()

	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame()
		server.Close()
	}()

	step := &loader.Step{Params: map[string]any{
		"endpoint":  float64(1),
		"feature":   "EnergyControl",
		"attribute": "myConsumptionLimit",
		"value":     float64(1000),
	}}
	out, err := r.handleWrite(context.Background(), step, state)
	// Should return output map, not a Go error.
	if err != nil {
		t.Fatalf("expected nil error (output map for EOF), got: %v", err)
	}
	if out[KeyWriteSuccess] != false {
		t.Errorf("expected write_success=false on EOF, got %v", out[KeyWriteSuccess])
	}
	if errStr, ok := out[KeyError].(string); !ok || errStr == "" {
		t.Error("expected error key with EOF description")
	}
}

// ============================================================================
// Phase 3 (Task 3.5): send_close sends ControlClose frame
// ============================================================================

func TestHandleSendClose_SendsControlCloseFrame(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	// Read the ControlClose frame from the server side.
	frameCh := make(chan []byte, 1)
	go func() {
		framer := transport.NewFramer(server)
		data, err := framer.ReadFrame()
		if err != nil {
			frameCh <- nil
			return
		}
		frameCh <- data
	}()

	out, err := r.handleSendClose(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyCloseSent] != true {
		t.Error("expected close_sent=true")
	}
	if out[KeyConnectionClosed] != true {
		t.Error("expected connection_closed=true")
	}

	// Verify the ControlClose frame was sent.
	frameData := <-frameCh
	if frameData == nil {
		t.Fatal("expected to receive ControlClose frame")
	}
	msg, decErr := wire.DecodeControlMessage(frameData)
	if decErr != nil {
		t.Fatalf("failed to decode control message: %v", decErr)
	}
	if msg.Type != wire.ControlClose {
		t.Errorf("expected ControlClose type (%d), got %d", wire.ControlClose, msg.Type)
	}
}

// ============================================================================
// Phase 3 (Tasks 3.6-3.7): message_type request/response in send_raw
// ============================================================================

func TestHandleSendRaw_MessageTypeRequest(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	go serverEchoResponse(server)

	step := &loader.Step{Params: map[string]any{
		"message_type": "request",
		"operation":    1,
		"endpoint_id":  0,
		"feature_id":   1,
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Error("expected raw_sent=true")
	}
}

func TestHandleSendRaw_MessageTypeResponse(t *testing.T) {
	r, server := newPipedRunner()
	defer server.Close()
	state := newTestState()

	// For response, server reads a frame and we don't expect echo back.
	go func() {
		framer := transport.NewFramer(server)
		_, _ = framer.ReadFrame() // consume the response frame
	}()

	step := &loader.Step{Params: map[string]any{
		"message_type": "response",
		"message_id":   99999999,
		"status":       0,
	}}
	out, err := r.handleSendRaw(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyRawSent] != true {
		t.Error("expected raw_sent=true")
	}
}

// ============================================================================
// Phase 5: Address classification helpers
// ============================================================================

func TestClassifyRemoteAddress(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{"nil", nil, "unknown"},
		{"ipv4", &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 8443}, "ipv4"},
		{"link_local", &net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 8443}, "link_local"},
		{"global_or_ula", &net.TCPAddr{IP: net.ParseIP("fd12:3456:789a::1"), Port: 8443}, "global_or_ula"},
		{"global_2001", &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 8443}, "global_or_ula"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRemoteAddress(tt.addr)
			if got != tt.want {
				t.Errorf("classifyRemoteAddress(%v) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestCheckInterfaceCorrect(t *testing.T) {
	// No interface_from param -> always true.
	got := checkInterfaceCorrect(&net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 8443}, map[string]any{})
	if !got {
		t.Error("expected true when no interface_from param")
	}

	// With interface_from and link-local + zone -> true.
	got = checkInterfaceCorrect(
		&net.TCPAddr{IP: net.ParseIP("fe80::1"), Port: 8443, Zone: "eth0"},
		map[string]any{"interface_from": "eth0"},
	)
	if !got {
		t.Error("expected true for link-local with zone")
	}

	// With interface_from but non-link-local -> true (not applicable).
	got = checkInterfaceCorrect(
		&net.TCPAddr{IP: net.ParseIP("fd12::1"), Port: 8443},
		map[string]any{"interface_from": "eth0"},
	)
	if !got {
		t.Error("expected true for non-link-local address")
	}

	// nil addr -> true.
	got = checkInterfaceCorrect(nil, map[string]any{"interface_from": "eth0"})
	if !got {
		t.Error("expected true for nil addr")
	}
}


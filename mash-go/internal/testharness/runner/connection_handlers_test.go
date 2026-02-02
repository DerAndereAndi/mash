package runner

import (
	"context"
	"testing"

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

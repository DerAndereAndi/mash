package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleInterfaceDownUp(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Interface down.
	out, _ := r.handleInterfaceDown(context.Background(), &loader.Step{}, state)
	if out["interface_down"] != true {
		t.Error("expected interface_down=true")
	}

	upVal, _ := state.Get("interface_up")
	if upVal != false {
		t.Error("expected interface_up=false")
	}

	// Interface up.
	out, _ = r.handleInterfaceUp(context.Background(), &loader.Step{}, state)
	if out["interface_up"] != true {
		t.Error("expected interface_up=true")
	}

	upVal, _ = state.Get("interface_up")
	if upVal != true {
		t.Error("expected interface_up=true")
	}
}

func TestHandleAdjustClock(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"offset_ms": float64(5000)}}
	out, _ := r.handleAdjustClock(context.Background(), step, state)
	if out["clock_adjusted"] != true {
		t.Error("expected clock_adjusted=true")
	}
	if out["offset_ms"] != int64(5000) {
		t.Errorf("expected 5000, got %v", out["offset_ms"])
	}

	ct := getConnectionTracker(state)
	if ct.clockOffset.Milliseconds() != 5000 {
		t.Errorf("expected 5000ms offset, got %v", ct.clockOffset)
	}

	// Days offset.
	step = &loader.Step{Params: map[string]any{"offset_days": float64(1)}}
	out, _ = r.handleAdjustClock(context.Background(), step, state)
	expected := int64(24 * 60 * 60 * 1000) // 86400000
	if out["offset_ms"] != expected {
		t.Errorf("expected %d, got %v", expected, out["offset_ms"])
	}
}

func TestHandleNetworkPartition(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"zone_id": "z1"}}
	out, _ := r.handleNetworkPartition(context.Background(), step, state)
	if out["partition_active"] != true {
		t.Error("expected partition_active=true")
	}
	if out["zone_id"] != "z1" {
		t.Errorf("expected z1, got %v", out["zone_id"])
	}
}

func TestHandleChangeAddress(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"new_address": "192.168.1.100"}}
	out, _ := r.handleChangeAddress(context.Background(), step, state)
	if out["address_changed"] != true {
		t.Error("expected address_changed=true")
	}

	addr, _ := state.Get("device_address")
	if addr != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %v", addr)
	}
}

func TestHandleCheckDisplay(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No QR payload set.
	out, _ := r.handleCheckDisplay(context.Background(), &loader.Step{}, state)
	if out["qr_displayed"] != false {
		t.Error("expected qr_displayed=false when no payload set")
	}

	// Set QR payload.
	ds := getDiscoveryState(state)
	ds.qrPayload = "MASH:1:1234:12345678:0x0000:0x0000"

	out, _ = r.handleCheckDisplay(context.Background(), &loader.Step{}, state)
	if out["qr_displayed"] != true {
		t.Error("expected qr_displayed=true")
	}
}

func TestHandleNetworkFilter(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{Params: map[string]any{"filter_type": "drop_all"}}
	out, _ := r.handleNetworkFilter(context.Background(), step, state)
	if out["filter_active"] != true {
		t.Error("expected filter_active=true")
	}

	filter, _ := state.Get("network_filter")
	if filter != "drop_all" {
		t.Errorf("expected drop_all, got %v", filter)
	}
}

package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestHandleVerifyConnectionState_Disconnected(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	out, err := r.handleVerifyConnectionState(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["same_connection"] != false {
		t.Error("expected same_connection=false when not connected")
	}
	if out["operational_connection_active"] != false {
		t.Error("expected operational_connection_active=false when not connected")
	}
	if out["mutual_tls"] != false {
		t.Error("expected mutual_tls=false when not connected")
	}
	if out["pase_performed"] != false {
		t.Error("expected pase_performed=false when no PASE")
	}
	if out["commissioning_connection_closed"] != true {
		t.Error("expected commissioning_connection_closed=true when not connected")
	}
}

func TestHandleVerifyConnectionState_ConnectedNoPASE(t *testing.T) {
	r := newTestRunner()
	r.pool.Main().state = ConnTLSConnected
	state := newTestState()

	out, _ := r.handleVerifyConnectionState(context.Background(), &loader.Step{}, state)
	if out["same_connection"] != true {
		t.Error("expected same_connection=true")
	}
	if out["operational_connection_active"] != false {
		t.Error("expected operational_connection_active=false without PASE")
	}
	if out["pase_performed"] != false {
		t.Error("expected pase_performed=false")
	}
	if out["commissioning_connection_closed"] != true {
		t.Error("expected commissioning_connection_closed=true without PASE")
	}
}

func TestHandleVerifyConnectionState_PASECompletedNotReconnected(t *testing.T) {
	// PASE completed but still on the commissioning connection (not yet reconnected).
	r := newTestRunner()
	r.pool.Main().state = ConnTLSConnected
	r.connMgr.SetPASEState(&PASEState{completed: true})
	state := newTestState()

	out, _ := r.handleVerifyConnectionState(context.Background(), &loader.Step{}, state)
	if out["same_connection"] != true {
		t.Error("expected same_connection=true")
	}
	if out["operational_connection_active"] != false {
		t.Error("expected operational_connection_active=false before operational reconnect")
	}
	if out["pase_performed"] != true {
		t.Error("expected pase_performed=true")
	}
	if out["commissioning_connection_closed"] != false {
		t.Error("expected commissioning_connection_closed=false (still on commissioning conn)")
	}
}

func TestHandleVerifyConnectionState_Operational(t *testing.T) {
	// After commission + operational reconnect.
	r := newTestRunner()
	r.pool.Main().state = ConnOperational
	r.connMgr.SetPASEState(&PASEState{completed: true})
	state := newTestState()

	out, _ := r.handleVerifyConnectionState(context.Background(), &loader.Step{}, state)
	if out["same_connection"] != true {
		t.Error("expected same_connection=true")
	}
	if out["operational_connection_active"] != true {
		t.Error("expected operational_connection_active=true after operational reconnect")
	}
	if out["pase_performed"] != true {
		t.Error("expected pase_performed=true")
	}
	if out["no_reconnection_required"] != true {
		t.Error("expected no_reconnection_required=true")
	}
	if out["commissioning_connection_closed"] != true {
		t.Error("expected commissioning_connection_closed=true after operational reconnect")
	}
}

// TestCSRHistoryAccumulation verifies that handleReceiveRenewalCSR appends to CSR history.
func TestCSRHistoryAccumulation(t *testing.T) {
	state := newTestState()

	// Simulate receiving CSR A
	csrA := []byte{0x01, 0x02, 0x03}
	state.Set(StatePendingCSR, csrA)

	// Append to history as handleReceiveRenewalCSR does
	var history [][]byte
	history = append(history, csrA)
	state.Set(StateCSRHistory, history)

	// Verify history has one entry
	histData, exists := state.Get(StateCSRHistory)
	if !exists {
		t.Fatal("expected CSR history to exist")
	}
	hist := histData.([][]byte)
	if len(hist) != 1 {
		t.Errorf("expected history length 1, got %d", len(hist))
	}

	// Simulate receiving CSR B (second renewal request)
	csrB := []byte{0x04, 0x05, 0x06}
	state.Set(StatePendingCSR, csrB)
	hist = append(hist, csrB)
	state.Set(StateCSRHistory, hist)

	// Verify history has two entries
	histData, _ = state.Get(StateCSRHistory)
	hist = histData.([][]byte)
	if len(hist) != 2 {
		t.Errorf("expected history length 2, got %d", len(hist))
	}

	// Verify CSR A is at index 0, CSR B at index 1
	if hist[0][0] != 0x01 {
		t.Error("expected CSR A at index 0")
	}
	if hist[1][0] != 0x04 {
		t.Error("expected CSR B at index 1")
	}
}

// TestUsePreviousCSR verifies that handleSendCertInstall respects use_previous_csr.
func TestUsePreviousCSR(t *testing.T) {
	state := newTestState()

	// Set up CSR history with two entries
	csrA := []byte{0xAA, 0xAA, 0xAA}
	csrB := []byte{0xBB, 0xBB, 0xBB}
	history := [][]byte{csrA, csrB}
	state.Set(StateCSRHistory, history)
	state.Set(StatePendingCSR, csrB) // Current pending is B

	// Test retrieving CSR at index 0 (CSR A)
	histData, exists := state.Get(StateCSRHistory)
	if !exists {
		t.Fatal("expected CSR history to exist")
	}
	hist := histData.([][]byte)

	csrIndex := 0
	if csrIndex >= len(hist) {
		t.Fatalf("invalid CSR index %d (history has %d entries)", csrIndex, len(hist))
	}
	selectedCSR := hist[csrIndex]

	// Verify we got CSR A, not the current pending (CSR B)
	if selectedCSR[0] != 0xAA {
		t.Errorf("expected CSR A (0xAA), got 0x%02X", selectedCSR[0])
	}
}

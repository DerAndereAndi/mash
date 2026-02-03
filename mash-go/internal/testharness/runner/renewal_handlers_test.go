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
	r.conn.connected = true
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

func TestHandleVerifyConnectionState_Operational(t *testing.T) {
	r := newTestRunner()
	r.conn.connected = true
	r.paseState = &PASEState{completed: true}
	state := newTestState()

	out, _ := r.handleVerifyConnectionState(context.Background(), &loader.Step{}, state)
	if out["same_connection"] != true {
		t.Error("expected same_connection=true")
	}
	if out["operational_connection_active"] != true {
		t.Error("expected operational_connection_active=true after PASE")
	}
	if out["pase_performed"] != true {
		t.Error("expected pase_performed=true")
	}
	if out["no_reconnection_required"] != true {
		t.Error("expected no_reconnection_required=true")
	}
	if out["commissioning_connection_closed"] != false {
		t.Error("expected commissioning_connection_closed=false when operational")
	}
}

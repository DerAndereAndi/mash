package runner

import (
	"context"
	"crypto/x509"
	"net"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// newPipeConnection creates a Connection backed by a net.Pipe with a real framer.
// The returned net.Conn is the "server" side; the test should feed response
// frames into it so the framer's ReadFrame returns them.
func newPipeConnection() (*Connection, net.Conn) {
	client, server := net.Pipe()
	return &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}, server
}

// echoSuccessResponse reads one request frame from the server side of the pipe,
// then writes back a success response. This is needed because net.Pipe is
// synchronous: the client's WriteFrame blocks until the server reads, and
// the client's ReadFrame blocks until the server writes.
func echoSuccessResponse(server net.Conn) {
	framer := transport.NewFramer(server)
	// Read the request (discard it).
	reqData, err := framer.ReadFrame()
	if err != nil || len(reqData) == 0 {
		return
	}
	// Decode to get the message ID for correlation.
	req, err := wire.DecodeRequest(reqData)
	msgID := uint32(1)
	if err == nil {
		msgID = req.MessageID
	}
	resp := &wire.Response{
		MessageID: msgID,
		Status:    wire.StatusSuccess,
	}
	data, err := wire.EncodeResponse(resp)
	if err != nil {
		return
	}
	_ = framer.WriteFrame(data)
}

// TestEnterCommissioningMode_UsesSuiteConn verifies that
// handleEnterCommissioningMode uses the suite zone connection for trigger
// delivery when Main() is disconnected.
//
// The test checks two invariants:
// 1. deviceStateModified becomes true (set by sendTriggerViaZone on success)
// 2. zoneCAPool is preserved (not overwritten)
func TestEnterCommissioningMode_UsesSuiteConn(t *testing.T) {
	r := newTestRunner()
	r.config.EnableKey = "00112233445566778899aabbccddeeff"
	r.config.Target = "127.0.0.1:9999"

	// Main() is disconnected (simulates detached connection for infrastructure tests).
	r.pool.Main().state = ConnDisconnected

	// Simulate existing zone CA from the suite zone commission.
	origZoneCAPool := x509.NewCertPool()
	r.connMgr.SetZoneCAPool(origZoneCAPool)

	// Place the suite zone connection on suite.SetConn() (not in pool).
	zoneConn, server := newPipeConnection()
	r.suite.SetConn(zoneConn)

	state := engine.NewExecutionState(context.Background())
	step := &loader.Step{
		Action: ActionEnterCommissioningMode,
	}

	// Feed a success response from the "server" side for the trigger invoke.
	go echoSuccessResponse(server)

	out, err := r.handleEnterCommissioningMode(context.Background(), step, state)
	server.Close()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyCommissioningModeEntered] != true {
		t.Error("expected commissioning_mode_entered=true")
	}

	// Verify suite conn was used (not simulated): sendTriggerViaZone sets this on success.
	if !r.connMgr.DeviceStateModified() {
		t.Error("expected deviceStateModified=true (proves suite conn was used, not stub simulation)")
	}

	// Verify zone CA pool was NOT corrupted.
	if r.connMgr.ZoneCAPool() != origZoneCAPool {
		t.Error("expected zoneCAPool to be preserved")
	}
}

// TestTriggerTestEvent_UsesSuiteConn verifies that handleTriggerTestEvent uses
// the suite zone connection for trigger delivery when Main() is disconnected.
func TestTriggerTestEvent_UsesSuiteConn(t *testing.T) {
	r := newTestRunner()
	r.config.EnableKey = "00112233445566778899aabbccddeeff"
	r.config.Target = "127.0.0.1:9999"

	// Main() is disconnected.
	r.pool.Main().state = ConnDisconnected

	origZoneCAPool := x509.NewCertPool()
	r.connMgr.SetZoneCAPool(origZoneCAPool)

	// Place the suite zone connection on suite.SetConn() (not in pool).
	zoneConn, server := newPipeConnection()
	r.suite.SetConn(zoneConn)

	state := engine.NewExecutionState(context.Background())
	step := &loader.Step{
		Action: ActionTriggerTestEvent,
		Params: map[string]any{
			KeyEventTrigger: features.TriggerEnterCommissioningMode,
		},
	}

	go echoSuccessResponse(server)

	out, err := r.handleTriggerTestEvent(context.Background(), step, state)
	server.Close()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyTriggerSent] != true {
		t.Error("expected trigger_sent=true")
	}
	if out[KeySuccess] != true {
		t.Error("expected success=true")
	}

	// Verify suite conn was actually used.
	if !r.connMgr.DeviceStateModified() {
		t.Error("expected deviceStateModified=true (proves suite conn was used, not stub simulation)")
	}
	if r.connMgr.ZoneCAPool() != origZoneCAPool {
		t.Error("expected zoneCAPool to be preserved")
	}
}

// TestExitCommissioningMode_UsesSuiteConn verifies that
// handleExitCommissioningMode uses the suite zone connection for trigger
// delivery when Main() is disconnected.
func TestExitCommissioningMode_UsesSuiteConn(t *testing.T) {
	r := newTestRunner()
	r.config.EnableKey = "00112233445566778899aabbccddeeff"
	r.config.Target = "127.0.0.1:9999"

	// Main() is disconnected.
	r.pool.Main().state = ConnDisconnected

	origZoneCAPool := x509.NewCertPool()
	r.connMgr.SetZoneCAPool(origZoneCAPool)

	// Place the suite zone connection on suite.SetConn() (not in pool).
	zoneConn, server := newPipeConnection()
	r.suite.SetConn(zoneConn)

	state := engine.NewExecutionState(context.Background())
	step := &loader.Step{
		Action: ActionExitCommissioningMode,
	}

	go echoSuccessResponse(server)

	out, err := r.handleExitCommissioningMode(context.Background(), step, state)
	server.Close()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyCommissioningModeExited] != true {
		t.Error("expected commissioning_mode_exited=true")
	}

	// Verify suite conn was actually used.
	if !r.connMgr.DeviceStateModified() {
		t.Error("expected deviceStateModified=true (proves suite conn was used, not stub simulation)")
	}
	if r.connMgr.ZoneCAPool() != origZoneCAPool {
		t.Error("expected zoneCAPool to be preserved")
	}
}

package runner

import (
	"strings"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestSnapshot_Empty(t *testing.T) {
	r := newTestRunner()
	s := r.snapshot()

	if s.MainConn.Connected {
		t.Error("expected connected=false")
	}
	if s.PASECompleted {
		t.Error("expected PASECompleted=false")
	}
	if s.HasZoneCA || s.HasControllerCert || s.HasZoneCAPool {
		t.Error("expected no cert state")
	}
	if len(s.ActiveZones) != 0 {
		t.Errorf("expected 0 active zones, got %d", len(s.ActiveZones))
	}
	if s.HasPhantomSocket() {
		t.Error("no phantom socket expected on fresh runner")
	}
}

func TestSnapshot_Connected(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational

	s := r.snapshot()
	if !s.MainConn.Connected {
		t.Error("expected connected=true")
	}
}

func TestSnapshot_Commissioned(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{
		completed:  true,
		sessionKey: []byte{1, 2, 3},
	}
	r.zoneCA = &cert.ZoneCA{}

	s := r.snapshot()
	if !s.PASECompleted {
		t.Error("expected PASECompleted=true")
	}
	if !s.HasSessionKey {
		t.Error("expected HasSessionKey=true")
	}
	if !s.HasZoneCA {
		t.Error("expected HasZoneCA=true")
	}
}

func TestSnapshot_ActiveZones(t *testing.T) {
	r := newTestRunner()
	r.activeZoneIDs = make(map[string]string)
	r.activeZoneConns["GRID"] = &Connection{state: ConnOperational}
	r.activeZoneConns["LOCAL"] = &Connection{state: ConnOperational}
	r.activeZoneIDs["GRID"] = "abc123"

	s := r.snapshot()
	if len(s.ActiveZones) != 2 {
		t.Errorf("expected 2 zones, got %d", len(s.ActiveZones))
	}
	if !s.ActiveZones["GRID"].Connected {
		t.Error("expected GRID connected")
	}
	if s.ActiveZoneIDs["GRID"] != "abc123" {
		t.Error("expected GRID zone ID")
	}
}

func TestSnapshot_PhantomSocket(t *testing.T) {
	r := newTestRunner()
	// Simulate the phantom socket bug: connected=false but conn still set.
	r.conn.state = ConnDisconnected
	// We can't easily set a real net.Conn, but we can test via ConnSnapshot.
	snap := RunnerSnapshot{
		MainConn: ConnSnapshot{
			Connected:  false,
			HasTLSConn: true,
			HasRawConn: true,
		},
	}
	if !snap.HasPhantomSocket() {
		t.Error("expected phantom socket detection")
	}

	snap.MainConn.HasTLSConn = false
	snap.MainConn.HasRawConn = false
	if snap.HasPhantomSocket() {
		t.Error("no phantom socket when sockets are nil")
	}
}

func TestSnapshot_PhantomZoneSocket(t *testing.T) {
	snap := RunnerSnapshot{
		ActiveZones: map[string]ConnSnapshot{
			"GRID":  {Connected: true, HasTLSConn: true},
			"LOCAL": {Connected: false, HasTLSConn: true}, // phantom
		},
	}

	name, has := snap.HasPhantomZoneSocket()
	if !has {
		t.Fatal("expected phantom zone socket")
	}
	if name != "LOCAL" {
		t.Errorf("expected LOCAL, got %s", name)
	}
}

func TestSnapshot_String(t *testing.T) {
	r := newTestRunner()
	r.conn.state = ConnOperational
	r.paseState = &PASEState{completed: true, sessionKey: []byte{1}}
	r.activeZoneConns["GRID"] = &Connection{state: ConnOperational}

	s := r.snapshot()
	str := s.String()

	// Verify key pieces are present.
	for _, want := range []string{"conn={", "pase={", "certs={", "GRID:"} {
		if !strings.Contains(str, want) {
			t.Errorf("String() missing %q, got: %s", want, str)
		}
	}
}

func TestDebugf_NoConfig(t *testing.T) {
	r := &Runner{}
	// Should not panic.
	r.debugf("test %d", 1)
}

func TestDebugf_DebugDisabled(t *testing.T) {
	r := newTestRunner()
	r.config.Debug = false
	// Should not panic.
	r.debugf("test %d", 1)
}

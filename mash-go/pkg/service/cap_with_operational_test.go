package service

import (
	"crypto/tls"
	"testing"
	"time"
)

// TestFillConnections_FullCapAfterClosingSuiteZone verifies that the full device
// cap (MaxZones+1) is available after the suite zone TCP connection is closed.
//
// Root cause of TC-CONN-CAP-001: The YAML test expects connections_opened=3
// (the full cap with MaxZones=2), but the suite zone's operational connection
// occupies 1 slot, leaving only 2 available. The coordinator fix closes the
// suite zone TCP in backward transitions to L1, freeing the cap slot.
//
// This test simulates the coordinator's fix: open a suite zone, close it,
// then verify the full cap is available.
func TestFillConnections_FullCapAfterClosingSuiteZone(t *testing.T) {
	svc := startCappedDevice(t, 2) // cap = MaxZones+1 = 3
	addr := svc.CommissioningAddr()

	// Simulate suite zone: hold 1 TLS connection.
	suiteConn := capDialTLS(t, addr)

	if !waitForActiveConns(svc, 1, 2*time.Second) {
		t.Fatalf("ActiveConns = %d, want 1", svc.ActiveConns())
	}

	// Close suite zone TCP (this is what the coordinator backward transition does).
	suiteConn.Close()
	if !waitForActiveConns(svc, 0, 2*time.Second) {
		t.Fatalf("ActiveConns = %d after suite close, want 0", svc.ActiveConns())
	}

	// Now the full cap should be available.
	opened := 0
	var conns []*tls.Conn
	for i := 0; i < 10; i++ {
		conn, err := capTryDialTLS(addr)
		if err != nil {
			break
		}
		conns = append(conns, conn)
		opened++
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	cap := svc.config.MaxZones + 1 // 3
	if opened != cap {
		t.Errorf("after closing suite zone, opened %d connections; want %d (full cap)", opened, cap)
	}
}

// TestBusyExchange_PossibleAfterClosingSuiteZone verifies that 2 concurrent
// commissioning connections are possible with MaxZones=1 (cap=2) after the
// suite zone TCP is closed.
//
// Root cause of TC-CONN-BUSY-003: With cap=2 and 1 slot used by the suite zone,
// only 1 commissioning connection fits. The busy test needs conn1 + conn2
// simultaneously. The coordinator fix closes the suite zone TCP before L1 tests.
func TestBusyExchange_PossibleAfterClosingSuiteZone(t *testing.T) {
	svc := startCappedDevice(t, 1) // cap = MaxZones+1 = 2
	addr := svc.CommissioningAddr()

	// Simulate suite zone: hold 1 connection.
	suiteConn := capDialTLS(t, addr)

	if !waitForActiveConns(svc, 1, 2*time.Second) {
		t.Fatalf("ActiveConns = %d, want 1", svc.ActiveConns())
	}

	// Close suite zone TCP (coordinator backward transition fix).
	suiteConn.Close()
	if !waitForActiveConns(svc, 0, 2*time.Second) {
		t.Fatalf("ActiveConns = %d after suite close, want 0", svc.ActiveConns())
	}

	// With cap=2, both commissioning connections should now fit.
	conn1, err := capTryDialTLS(addr)
	if err != nil {
		t.Fatalf("conn1 should succeed after suite zone closed: %v", err)
	}
	defer conn1.Close()

	conn2, err := capTryDialTLS(addr)
	if err != nil {
		t.Fatalf("conn2 should succeed (cap=2, 0 used by suite): %v", err)
	}
	defer conn2.Close()
}

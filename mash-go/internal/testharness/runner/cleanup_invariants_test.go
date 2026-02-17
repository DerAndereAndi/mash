package runner

import (
	"crypto/tls"
	"testing"
)

func TestCleanupReport_DetectsPhantomMainSocket(t *testing.T) {
	r := newTestRunner()
	r.pool.Main().tlsConn = &tls.Conn{}
	r.pool.Main().state = ConnDisconnected

	report := r.BuildCleanupReport()
	if !report.HasPhantomMainSocket {
		t.Fatal("expected phantom main socket to be detected")
	}
	if report.IsClean() {
		t.Fatal("expected report to be dirty")
	}
}

func TestCleanupReport_DetectsPhantomZoneSocket(t *testing.T) {
	r := newTestRunner()
	r.pool.TrackZone("zone-a", &Connection{
		state:   ConnDisconnected,
		tlsConn: &tls.Conn{},
	}, "zone-a-id")

	report := r.BuildCleanupReport()
	if !report.HasPhantomZoneSocket {
		t.Fatal("expected phantom zone socket to be detected")
	}
	if report.PhantomZoneConnKey != "zone-a" {
		t.Fatalf("expected phantom zone key zone-a, got %q", report.PhantomZoneConnKey)
	}
	if report.IsClean() {
		t.Fatal("expected report to be dirty")
	}
}

func TestCleanupReport_DetectsNonEmptyZonePool(t *testing.T) {
	r := newTestRunner()
	r.pool.TrackZone("zone-a", &Connection{state: ConnDisconnected}, "zone-a-id")

	report := r.BuildCleanupReport()
	if report.ActiveZoneCount != 1 {
		t.Fatalf("expected active zone count 1, got %d", report.ActiveZoneCount)
	}
	if report.IsClean() {
		t.Fatal("expected report to be dirty")
	}
}

func TestCleanupReport_DetectsIncompletePASE(t *testing.T) {
	r := newTestRunner()
	r.connMgr.SetPASEState(&PASEState{completed: false})

	report := r.BuildCleanupReport()
	if !report.HasIncompletePASE {
		t.Fatal("expected incomplete PASE to be detected")
	}
	if report.IsClean() {
		t.Fatal("expected report to be dirty")
	}
}

func TestCleanupReport_DetectsResidualSuiteConnection(t *testing.T) {
	r := newTestRunner()
	r.suite.SetConn(&Connection{state: ConnDisconnected})

	report := r.BuildCleanupReport()
	if !report.HasResidualSuiteConnection {
		t.Fatal("expected residual suite connection to be detected")
	}
	if report.IsClean() {
		t.Fatal("expected report to be dirty")
	}
}

func TestCleanupReport_AllClean(t *testing.T) {
	r := newTestRunner()

	report := r.BuildCleanupReport()
	if !report.IsClean() {
		t.Fatalf("expected clean report, got issues: %v", report.Issues)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected no issues, got: %v", report.Issues)
	}
}

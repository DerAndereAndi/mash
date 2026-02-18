package runner

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestDecideCommissionZoneType_ExplicitAlwaysWins(t *testing.T) {
	got := decideCommissionZoneType(cert.ZoneTypeGrid, true, cert.ZoneTypeTest)
	if got != cert.ZoneTypeGrid {
		t.Fatalf("expected explicit GRID to win, got %v", got)
	}
}

func TestDecideCommissionZoneType_ImplicitWithSuiteTestDefaultsToLocal(t *testing.T) {
	got := decideCommissionZoneType(0, true, cert.ZoneTypeTest)
	if got != cert.ZoneTypeLocal {
		t.Fatalf("expected implicit suite TEST to map to LOCAL, got %v", got)
	}
}

func TestDecideCommissionZoneType_ImplicitWithoutSuiteKeepsCurrent(t *testing.T) {
	got := decideCommissionZoneType(0, false, cert.ZoneTypeTest)
	if got != cert.ZoneTypeTest {
		t.Fatalf("expected current zone type to be preserved without suite, got %v", got)
	}
}

func TestDecideCommissionZoneType_ImplicitZeroDefaultsToLocal(t *testing.T) {
	got := decideCommissionZoneType(0, false, 0)
	if got != cert.ZoneTypeLocal {
		t.Fatalf("expected zero default to LOCAL, got %v", got)
	}
}

func TestApplyCommissionZoneType_ImplicitSuiteTestDefaultsToLocal(t *testing.T) {
	r := newTestRunner()
	r.connMgr.SetCommissionZoneType(cert.ZoneTypeTest)
	r.suite.Record("suite-zone", CryptoState{})

	requested := r.applyCommissionZoneType(map[string]any{})
	if requested != "" {
		t.Fatalf("expected no requested zone type, got %q", requested)
	}
	if got := r.connMgr.CommissionZoneType(); got != cert.ZoneTypeLocal {
		t.Fatalf("expected commission zone type LOCAL, got %v", got)
	}
}

func TestApplyCommissionZoneType_ExplicitTestPreserved(t *testing.T) {
	r := newTestRunner()
	r.connMgr.SetCommissionZoneType(cert.ZoneTypeLocal)
	r.suite.Record("suite-zone", CryptoState{})

	requested := r.applyCommissionZoneType(map[string]any{KeyZoneType: "TEST"})
	if requested != "TEST" {
		t.Fatalf("expected requested TEST, got %q", requested)
	}
	if got := r.connMgr.CommissionZoneType(); got != cert.ZoneTypeTest {
		t.Fatalf("expected commission zone type TEST, got %v", got)
	}
}

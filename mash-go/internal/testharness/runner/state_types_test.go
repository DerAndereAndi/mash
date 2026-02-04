package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
)

func newTestState() *engine.ExecutionState {
	return engine.NewExecutionState(context.Background())
}

func TestGetDiscoveryState(t *testing.T) {
	state := newTestState()

	// First call should create new state.
	ds := getDiscoveryState(state)
	if ds == nil {
		t.Fatal("expected non-nil discoveryState")
	}
	if ds.active {
		t.Error("expected active to be false")
	}
	if len(ds.services) != 0 {
		t.Error("expected empty services")
	}

	// Second call should return same state.
	ds.active = true
	ds2 := getDiscoveryState(state)
	if !ds2.active {
		t.Error("expected same state instance")
	}
}

func TestGetZoneState(t *testing.T) {
	state := newTestState()

	zs := getZoneState(state)
	if zs == nil {
		t.Fatal("expected non-nil zoneState")
	}
	if len(zs.zones) != 0 {
		t.Error("expected empty zones map")
	}
	if zs.maxZones != 5 {
		t.Errorf("expected maxZones=5, got %d", zs.maxZones)
	}

	// Mutate and verify persistence.
	zs.zones["test-zone"] = &zoneInfo{ZoneID: "test-zone", ZoneType: ZoneTypeLocal}
	zs2 := getZoneState(state)
	if _, ok := zs2.zones["test-zone"]; !ok {
		t.Error("expected zone to persist")
	}
}

func TestGetDeviceState(t *testing.T) {
	state := newTestState()

	ds := getDeviceState(state)
	if ds == nil {
		t.Fatal("expected non-nil deviceState")
	}
	if ds.operatingState != OperatingStateStandby {
		t.Errorf("expected STANDBY, got %s", ds.operatingState)
	}
	if ds.controlState != ControlStateAutonomous {
		t.Errorf("expected AUTONOMOUS, got %s", ds.controlState)
	}
	if ds.processState != ProcessStateNone {
		t.Errorf("expected NONE, got %s", ds.processState)
	}
	if len(ds.faults) != 0 {
		t.Error("expected empty faults")
	}
	if ds.evConnected {
		t.Error("expected evConnected false")
	}

	// Mutate and verify persistence.
	ds.controlState = ControlStateControlled
	ds2 := getDeviceState(state)
	if ds2.controlState != ControlStateControlled {
		t.Error("expected state to persist")
	}
}

func TestGetConnectionTracker(t *testing.T) {
	state := newTestState()

	ct := getConnectionTracker(state)
	if ct == nil {
		t.Fatal("expected non-nil connectionTracker")
	}
	if len(ct.zoneConnections) != 0 {
		t.Error("expected empty zoneConnections")
	}
	if len(ct.pendingQueue) != 0 {
		t.Error("expected empty pendingQueue")
	}
	if ct.backoffState != nil {
		t.Error("expected nil backoffState")
	}

	// Mutate and verify persistence.
	ct.zoneConnections["z1"] = &Connection{connected: true}
	ct2 := getConnectionTracker(state)
	if _, ok := ct2.zoneConnections["z1"]; !ok {
		t.Error("expected connection to persist")
	}
}

func TestGetControllerState(t *testing.T) {
	state := newTestState()

	cs := getControllerState(state)
	if cs == nil {
		t.Fatal("expected non-nil controllerState")
	}
	if cs.commissioningWindowDuration != 15*60*1000000000 { // 15 minutes in ns
		t.Errorf("expected 15m, got %v", cs.commissioningWindowDuration)
	}
	if len(cs.devices) != 0 {
		t.Error("expected empty devices map")
	}

	// Mutate and verify persistence.
	cs.controllerID = "ctrl-1"
	cs2 := getControllerState(state)
	if cs2.controllerID != "ctrl-1" {
		t.Error("expected controllerID to persist")
	}
}

func TestZonePriority(t *testing.T) {
	if zonePriority[ZoneTypeGrid] <= zonePriority[ZoneTypeLocal] {
		t.Error("GRID should have higher priority than LOCAL")
	}
	if zonePriority[ZoneTypeLocal] <= zonePriority[ZoneTypeTest] {
		t.Error("LOCAL should have higher priority than TEST")
	}
}

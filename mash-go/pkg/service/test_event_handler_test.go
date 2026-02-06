package service

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// newDeviceServiceWithChargingSession creates a DeviceService whose device has
// an EV_CHARGER endpoint (ID 1) with a ChargingSession feature. This is the
// minimal setup needed to exercise handleChargingSessionTrigger.
func newDeviceServiceWithChargingSession(t *testing.T) *DeviceService {
	t.Helper()

	device := model.NewDevice("test-device", 0x1234, 0x5678)

	charger := model.NewEndpoint(1, model.EndpointEVCharger, "Test Charger")
	cs := features.NewChargingSession()
	charger.AddFeature(cs.Feature)
	if err := device.AddEndpoint(charger); err != nil {
		t.Fatalf("AddEndpoint failed: %v", err)
	}

	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}
	return svc
}

// readChargingState reads the ChargingSession state attribute from endpoint 1.
func readChargingState(t *testing.T, svc *DeviceService) features.ChargingState {
	t.Helper()

	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}
	feat, err := ep.GetFeatureByID(uint8(model.FeatureChargingSession))
	if err != nil {
		t.Fatalf("GetFeatureByID(ChargingSession) failed: %v", err)
	}
	val, err := feat.ReadAttribute(features.ChargingSessionAttrState)
	if err != nil {
		t.Fatalf("ReadAttribute(state) failed: %v", err)
	}
	return features.ChargingState(val.(uint8))
}

func TestHandleChargingSessionTrigger_EVPlugIn(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	// Default state should be NOT_PLUGGED_IN.
	if got := readChargingState(t, svc); got != features.ChargingStateNotPluggedIn {
		t.Fatalf("expected initial state NOT_PLUGGED_IN, got %v", got)
	}

	if err := svc.dispatchTrigger(ctx, features.TriggerEVPlugIn); err != nil {
		t.Fatalf("dispatchTrigger(EVPlugIn) error: %v", err)
	}

	if got := readChargingState(t, svc); got != features.ChargingStatePluggedInNoDemand {
		t.Errorf("expected state PLUGGED_IN_NO_DEMAND after plug in, got %v", got)
	}
}

func TestHandleChargingSessionTrigger_EVUnplug(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	// First plug in so we start from a non-default state.
	if err := svc.dispatchTrigger(ctx, features.TriggerEVPlugIn); err != nil {
		t.Fatalf("dispatchTrigger(EVPlugIn) error: %v", err)
	}

	if err := svc.dispatchTrigger(ctx, features.TriggerEVUnplug); err != nil {
		t.Fatalf("dispatchTrigger(EVUnplug) error: %v", err)
	}

	if got := readChargingState(t, svc); got != features.ChargingStateNotPluggedIn {
		t.Errorf("expected state NOT_PLUGGED_IN after unplug, got %v", got)
	}
}

func TestHandleChargingSessionTrigger_EVRequestCharge(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	if err := svc.dispatchTrigger(ctx, features.TriggerEVRequestCharge); err != nil {
		t.Fatalf("dispatchTrigger(EVRequestCharge) error: %v", err)
	}

	if got := readChargingState(t, svc); got != features.ChargingStatePluggedInDemand {
		t.Errorf("expected state PLUGGED_IN_DEMAND after request charge, got %v", got)
	}
}

func TestHandleChargingSessionTrigger_UnknownTrigger(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	// Use a trigger with the ChargingSession domain but an unknown sub-opcode.
	unknownTrigger := uint64(0x0006_0000_0000_FFFF)
	err := svc.dispatchTrigger(ctx, unknownTrigger)
	if err == nil {
		t.Fatal("expected error for unknown charging session trigger")
	}
}

func TestHandleChargingSessionTrigger_NoFeature(t *testing.T) {
	// Create a device without any ChargingSession feature.
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	ctx := context.Background()
	err = svc.dispatchTrigger(ctx, features.TriggerEVPlugIn)
	if err == nil {
		t.Fatal("expected error when no ChargingSession feature exists")
	}
}

// --- ChargingSession timestamp tests ---

// readSessionStartTime reads sessionStartTime from endpoint 1's ChargingSession feature.
func readSessionStartTime(t *testing.T, svc *DeviceService) (uint64, bool) {
	t.Helper()
	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}
	feat, err := ep.GetFeatureByID(uint8(model.FeatureChargingSession))
	if err != nil {
		t.Fatalf("GetFeatureByID(ChargingSession) failed: %v", err)
	}
	val, err := feat.ReadAttribute(features.ChargingSessionAttrSessionStartTime)
	if err != nil {
		t.Fatalf("ReadAttribute(sessionStartTime) failed: %v", err)
	}
	if val == nil {
		return 0, false
	}
	return val.(uint64), true
}

// readSessionEndTime reads sessionEndTime from endpoint 1's ChargingSession feature.
func readSessionEndTime(t *testing.T, svc *DeviceService) (uint64, bool) {
	t.Helper()
	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}
	feat, err := ep.GetFeatureByID(uint8(model.FeatureChargingSession))
	if err != nil {
		t.Fatalf("GetFeatureByID(ChargingSession) failed: %v", err)
	}
	val, err := feat.ReadAttribute(features.ChargingSessionAttrSessionEndTime)
	if err != nil {
		t.Fatalf("ReadAttribute(sessionEndTime) failed: %v", err)
	}
	if val == nil {
		return 0, false
	}
	return val.(uint64), true
}

func TestHandleChargingSessionTrigger_EVPlugIn_SetsStartTime(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	// Before plug-in, sessionStartTime should be nil (nullable attribute).
	if _, ok := readSessionStartTime(t, svc); ok {
		t.Fatal("expected sessionStartTime to be nil before plug in")
	}

	before := uint64(time.Now().Unix())
	if err := svc.dispatchTrigger(ctx, features.TriggerEVPlugIn); err != nil {
		t.Fatalf("dispatchTrigger(EVPlugIn) error: %v", err)
	}
	after := uint64(time.Now().Unix())

	ts, ok := readSessionStartTime(t, svc)
	if !ok {
		t.Fatal("expected sessionStartTime to be set after plug in")
	}
	if ts < before || ts > after {
		t.Errorf("sessionStartTime %d not in expected range [%d, %d]", ts, before, after)
	}
}

func TestHandleChargingSessionTrigger_EVUnplug_SetsEndTime(t *testing.T) {
	svc := newDeviceServiceWithChargingSession(t)
	ctx := context.Background()

	// Plug in first so unplug is meaningful.
	if err := svc.dispatchTrigger(ctx, features.TriggerEVPlugIn); err != nil {
		t.Fatalf("dispatchTrigger(EVPlugIn) error: %v", err)
	}

	before := uint64(time.Now().Unix())
	if err := svc.dispatchTrigger(ctx, features.TriggerEVUnplug); err != nil {
		t.Fatalf("dispatchTrigger(EVUnplug) error: %v", err)
	}
	after := uint64(time.Now().Unix())

	ts, ok := readSessionEndTime(t, svc)
	if !ok {
		t.Fatal("expected sessionEndTime to be set after unplug")
	}
	if ts < before || ts > after {
		t.Errorf("sessionEndTime %d not in expected range [%d, %d]", ts, before, after)
	}
}

// --- EnergyControl trigger tests ---

// newDeviceServiceWithEnergyControl creates a DeviceService whose device has
// an EV_CHARGER endpoint (ID 1) with an EnergyControl feature.
func newDeviceServiceWithEnergyControl(t *testing.T) *DeviceService {
	t.Helper()

	device := model.NewDevice("test-device", 0x1234, 0x5678)

	charger := model.NewEndpoint(1, model.EndpointEVCharger, "Test Charger")
	ec := features.NewEnergyControl()
	charger.AddFeature(ec.Feature)
	if err := device.AddEndpoint(charger); err != nil {
		t.Fatalf("AddEndpoint failed: %v", err)
	}

	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}
	return svc
}

// readControlState reads the EnergyControl ControlState attribute from endpoint 1.
func readControlState(t *testing.T, svc *DeviceService) features.ControlState {
	t.Helper()

	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}
	feat, err := ep.GetFeatureByID(uint8(model.FeatureEnergyControl))
	if err != nil {
		t.Fatalf("GetFeatureByID(EnergyControl) failed: %v", err)
	}
	val, err := feat.ReadAttribute(features.EnergyControlAttrControlState)
	if err != nil {
		t.Fatalf("ReadAttribute(controlState) failed: %v", err)
	}
	return features.ControlState(val.(uint8))
}

// readProcessState reads the EnergyControl ProcessState attribute from endpoint 1.
func readProcessState(t *testing.T, svc *DeviceService) features.ProcessState {
	t.Helper()

	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}
	feat, err := ep.GetFeatureByID(uint8(model.FeatureEnergyControl))
	if err != nil {
		t.Fatalf("GetFeatureByID(EnergyControl) failed: %v", err)
	}
	val, err := feat.ReadAttribute(features.EnergyControlAttrProcessState)
	if err != nil {
		t.Fatalf("ReadAttribute(processState) failed: %v", err)
	}
	return features.ProcessState(val.(uint8))
}

func TestHandleEnergyControlTrigger_ControlStateLimited(t *testing.T) {
	svc := newDeviceServiceWithEnergyControl(t)
	ctx := context.Background()

	// Default state should be AUTONOMOUS.
	if got := readControlState(t, svc); got != features.ControlStateAutonomous {
		t.Fatalf("expected initial control state AUTONOMOUS, got %v", got)
	}

	if err := svc.dispatchTrigger(ctx, features.TriggerControlStateLimited); err != nil {
		t.Fatalf("dispatchTrigger(ControlStateLimited) error: %v", err)
	}

	if got := readControlState(t, svc); got != features.ControlStateLimited {
		t.Errorf("expected control state LIMITED, got %v", got)
	}
}

func TestHandleEnergyControlTrigger_ProcessStateRunning(t *testing.T) {
	svc := newDeviceServiceWithEnergyControl(t)
	ctx := context.Background()

	// Default state should be NONE.
	if got := readProcessState(t, svc); got != features.ProcessStateNone {
		t.Fatalf("expected initial process state NONE, got %v", got)
	}

	if err := svc.dispatchTrigger(ctx, features.TriggerProcessStateRunning); err != nil {
		t.Fatalf("dispatchTrigger(ProcessStateRunning) error: %v", err)
	}

	if got := readProcessState(t, svc); got != features.ProcessStateRunning {
		t.Errorf("expected process state RUNNING, got %v", got)
	}
}

func TestHandleEnergyControlTrigger_UnknownTrigger(t *testing.T) {
	svc := newDeviceServiceWithEnergyControl(t)
	ctx := context.Background()

	unknownTrigger := uint64(0x0005_0000_0000_FFFF)
	err := svc.dispatchTrigger(ctx, unknownTrigger)
	if err == nil {
		t.Fatal("expected error for unknown energy control trigger")
	}
}

func TestHandleEnergyControlTrigger_NoFeature(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	ctx := context.Background()
	err = svc.dispatchTrigger(ctx, features.TriggerControlStateControlled)
	if err == nil {
		t.Fatal("expected error when no EnergyControl feature exists")
	}
}

// --- Reset trigger tests ---

// newDeviceServiceWithAllFeatures creates a DeviceService whose device has an
// EV_CHARGER endpoint (ID 1) with Status, Measurement, ChargingSession, and
// EnergyControl features. This is the setup needed to exercise TriggerResetTestState.
func newDeviceServiceWithAllFeatures(t *testing.T) *DeviceService {
	t.Helper()

	device := model.NewDevice("test-device", 0x1234, 0x5678)

	charger := model.NewEndpoint(1, model.EndpointEVCharger, "Test Charger")
	charger.AddFeature(features.NewStatus().Feature)
	charger.AddFeature(features.NewMeasurement().Feature)
	charger.AddFeature(features.NewChargingSession().Feature)
	charger.AddFeature(features.NewEnergyControl().Feature)
	if err := device.AddEndpoint(charger); err != nil {
		t.Fatalf("AddEndpoint failed: %v", err)
	}

	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}
	return svc
}

func TestTriggerResetTestState(t *testing.T) {
	svc := newDeviceServiceWithAllFeatures(t)
	ctx := context.Background()

	// Move all features away from their default states.
	if err := svc.dispatchTrigger(ctx, features.TriggerFault); err != nil {
		t.Fatalf("dispatchTrigger(Fault) error: %v", err)
	}
	if err := svc.dispatchTrigger(ctx, features.TriggerSetPower1000); err != nil {
		t.Fatalf("dispatchTrigger(SetPower1000) error: %v", err)
	}
	if err := svc.dispatchTrigger(ctx, features.TriggerEVPlugIn); err != nil {
		t.Fatalf("dispatchTrigger(EVPlugIn) error: %v", err)
	}
	if err := svc.dispatchTrigger(ctx, features.TriggerControlStateLimited); err != nil {
		t.Fatalf("dispatchTrigger(ControlStateLimited) error: %v", err)
	}
	if err := svc.dispatchTrigger(ctx, features.TriggerProcessStateRunning); err != nil {
		t.Fatalf("dispatchTrigger(ProcessStateRunning) error: %v", err)
	}

	// Reset all state.
	if err := svc.dispatchTrigger(ctx, features.TriggerResetTestState); err != nil {
		t.Fatalf("dispatchTrigger(ResetTestState) error: %v", err)
	}

	// Verify Status reset to STANDBY.
	ep, err := svc.Device().GetEndpoint(1)
	if err != nil {
		t.Fatalf("GetEndpoint(1) failed: %v", err)
	}

	statusFeat, err := ep.GetFeatureByID(uint8(model.FeatureStatus))
	if err != nil {
		t.Fatalf("GetFeatureByID(Status) failed: %v", err)
	}
	osVal, err := statusFeat.ReadAttribute(features.StatusAttrOperatingState)
	if err != nil {
		t.Fatalf("ReadAttribute(operatingState) failed: %v", err)
	}
	if features.OperatingState(osVal.(uint8)) != features.OperatingStateStandby {
		t.Errorf("expected operating state STANDBY after reset, got %v", osVal)
	}

	// Verify Measurement reset to zero.
	measFeat, err := ep.GetFeatureByID(uint8(model.FeatureMeasurement))
	if err != nil {
		t.Fatalf("GetFeatureByID(Measurement) failed: %v", err)
	}
	powerVal, err := measFeat.ReadAttribute(features.MeasurementAttrACActivePower)
	if err != nil {
		t.Fatalf("ReadAttribute(acActivePower) failed: %v", err)
	}
	if powerVal.(int64) != 0 {
		t.Errorf("expected power 0 after reset, got %v", powerVal)
	}

	// Verify ChargingSession reset to NOT_PLUGGED_IN.
	if got := readChargingState(t, svc); got != features.ChargingStateNotPluggedIn {
		t.Errorf("expected charging state NOT_PLUGGED_IN after reset, got %v", got)
	}

	// Verify EnergyControl reset to AUTONOMOUS/NONE.
	if got := readControlState(t, svc); got != features.ControlStateAutonomous {
		t.Errorf("expected control state AUTONOMOUS after reset, got %v", got)
	}
	if got := readProcessState(t, svc); got != features.ProcessStateNone {
		t.Errorf("expected process state NONE after reset, got %v", got)
	}
}

func TestTriggerFactoryReset_ClearsAllZones(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.TestEnableKey = "test-enable-key"

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	ctx := context.Background()

	// Simulate two connected zones by directly adding to connectedZones.
	svc.mu.Lock()
	svc.connectedZones["zone-1"] = &ConnectedZone{ID: "zone-1", Connected: true}
	svc.connectedZones["zone-2"] = &ConnectedZone{ID: "zone-2", Connected: true}
	svc.mu.Unlock()

	// Verify zones exist.
	if len(svc.ListZoneIDs()) != 2 {
		t.Fatalf("expected 2 zones before reset, got %d", len(svc.ListZoneIDs()))
	}

	// Send factory reset trigger.
	if err := svc.dispatchTrigger(ctx, features.TriggerFactoryReset); err != nil {
		t.Fatalf("dispatchTrigger(FactoryReset) error: %v", err)
	}

	// Verify all zones removed.
	if len(svc.ListZoneIDs()) != 0 {
		t.Errorf("expected 0 zones after factory reset, got %d", len(svc.ListZoneIDs()))
	}
}

func TestTriggerAdjustClock_SetsOffset(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	// Encode +400 seconds in trigger
	trigger := features.TriggerAdjustClockBase | uint64(uint32(int32(400)))
	if err := svc.dispatchTrigger(context.Background(), trigger); err != nil {
		t.Fatalf("dispatchTrigger: %v", err)
	}

	expected := 400 * time.Second
	if svc.clockOffset != expected {
		t.Errorf("clockOffset = %v, want %v", svc.clockOffset, expected)
	}
}

func TestTriggerAdjustClock_NegativeOffset(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	// Encode -200 seconds via two's complement
	offsetSec := int32(-200)
	trigger := features.TriggerAdjustClockBase | uint64(uint32(offsetSec))
	if err := svc.dispatchTrigger(context.Background(), trigger); err != nil {
		t.Fatalf("dispatchTrigger: %v", err)
	}

	expected := -200 * time.Second
	if svc.clockOffset != expected {
		t.Errorf("clockOffset = %v, want %v", svc.clockOffset, expected)
	}
}

func TestTriggerResetTestState_ClearsClockOffset(t *testing.T) {
	svc := newDeviceServiceWithAllFeatures(t)
	ctx := context.Background()

	// Set a clock offset first
	trigger := features.TriggerAdjustClockBase | uint64(uint32(int32(400)))
	if err := svc.dispatchTrigger(ctx, trigger); err != nil {
		t.Fatalf("dispatchTrigger(AdjustClock): %v", err)
	}
	if svc.clockOffset == 0 {
		t.Fatal("clockOffset should be non-zero after adjust")
	}

	// Reset should clear it
	if err := svc.dispatchTrigger(ctx, features.TriggerResetTestState); err != nil {
		t.Fatalf("dispatchTrigger(ResetTestState): %v", err)
	}
	if svc.clockOffset != 0 {
		t.Errorf("clockOffset = %v after reset, want 0", svc.clockOffset)
	}
}

func TestVerifyClientCert_UsesClockOffset(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	// Generate a zone CA and store it
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeLocal)
	if err != nil {
		t.Fatalf("GenerateZoneCA: %v", err)
	}
	if err := svc.certStore.SetZoneCACert("test-zone", zoneCA.Certificate); err != nil {
		t.Fatalf("SetZoneCACert: %v", err)
	}

	// Generate a valid controller cert (valid for OperationalCertValidity = 365 days)
	controllerCert, err := cert.GenerateControllerOperationalCert(zoneCA, "test-ctrl")
	if err != nil {
		t.Fatalf("GenerateControllerOperationalCert: %v", err)
	}

	// Without offset, cert should be valid
	if err := svc.verifyClientCert(controllerCert.Certificate); err != nil {
		t.Fatalf("cert should be valid without offset: %v", err)
	}

	// With +366d offset, cert should be expired (NotAfter is +365d from cert creation)
	svc.clockOffset = 366 * 24 * time.Hour
	if err := svc.verifyClientCert(controllerCert.Certificate); err == nil {
		t.Error("cert should be rejected with +366d clock offset")
	}

	// Reset offset, should be valid again
	svc.clockOffset = 0
	if err := svc.verifyClientCert(controllerCert.Certificate); err != nil {
		t.Errorf("cert should be valid after clearing offset: %v", err)
	}
}

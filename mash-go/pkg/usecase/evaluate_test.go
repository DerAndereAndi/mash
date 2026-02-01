package usecase

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// testDevice builds a minimal device with the given features on endpoint 1.
func testDevice(epType model.EndpointType, feats ...*model.Feature) *model.Device {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	ep := model.NewEndpoint(1, epType, "Test")
	for _, f := range feats {
		ep.AddFeature(f)
	}
	_ = device.AddEndpoint(ep)
	return device
}

func TestEvaluateDevice_EVSE(t *testing.T) {
	// Build an EVSE-like device with EnergyControl, Electrical, Measurement, ChargingSession
	ec := features.NewEnergyControl()
	ec.SetCapabilities(true, true, false, false, true, false, true)

	elec := features.NewElectrical()
	_ = elec.SetNominalMaxConsumption(22_000_000)

	meas := features.NewMeasurement()

	cs := features.NewChargingSession()

	status := features.NewStatus()

	device := testDevice(model.EndpointEVCharger,
		ec.Feature, elec.Feature, meas.Feature, cs.Feature, status.Feature,
	)

	decls := EvaluateDevice(device, Registry)

	// An EVSE with these features should match LPC, MPD, EVC at minimum
	declMap := make(map[uint16]*model.UseCaseDecl)
	for _, d := range decls {
		declMap[d.ID] = d
	}

	expectedUCs := map[UseCaseName]UseCaseID{
		LPC: LPCID,
		MPD: MPDID,
		EVC: EVCID,
	}
	for name, id := range expectedUCs {
		d, ok := declMap[uint16(id)]
		if !ok {
			t.Errorf("expected use case %s (0x%02X) to match", name, id)
			continue
		}
		if d.EndpointID != 1 {
			t.Errorf("%s: EndpointID = %d, want 1", name, d.EndpointID)
		}
		if d.Major != 1 {
			t.Errorf("%s: Major = %d, want 1", name, d.Major)
		}
		if d.Minor != 0 {
			t.Errorf("%s: Minor = %d, want 0", name, d.Minor)
		}
		// Verify scenarios bitmap is non-zero (at least BASE)
		if d.Scenarios == 0 {
			t.Errorf("%s: Scenarios should be non-zero", name)
		}
	}
}

func TestEvaluateDevice_MeasurementOnly(t *testing.T) {
	// Device with only Measurement on an EV_CHARGER endpoint
	meas := features.NewMeasurement()
	device := testDevice(model.EndpointEVCharger, meas.Feature)

	decls := EvaluateDevice(device, Registry)

	// Should match MPD only (Measurement required in BASE)
	if len(decls) != 1 {
		var ids []uint16
		for _, d := range decls {
			ids = append(ids, d.ID)
		}
		t.Fatalf("expected 1 use case (MPD), got %d: %v", len(decls), ids)
	}
	if decls[0].ID != uint16(MPDID) {
		t.Errorf("expected MPD (0x%02X), got 0x%02X", MPDID, decls[0].ID)
	}
}

func TestEvaluateController_AllRegistered(t *testing.T) {
	decls := EvaluateController(Registry)

	if len(decls) != len(Registry) {
		t.Fatalf("expected %d declarations, got %d", len(Registry), len(decls))
	}

	// Build lookup by ID
	declMap := make(map[uint16]*model.UseCaseDecl)
	for _, d := range decls {
		declMap[d.ID] = d
	}

	// Verify all registry entries are present with correct versions
	for name, def := range Registry {
		d, ok := declMap[uint16(def.ID)]
		if !ok {
			t.Errorf("missing declaration for %s (0x%02X)", name, def.ID)
			continue
		}
		if d.Major != def.Major {
			t.Errorf("%s: Major = %d, want %d", name, d.Major, def.Major)
		}
		if d.Minor != def.Minor {
			t.Errorf("%s: Minor = %d, want %d", name, d.Minor, def.Minor)
		}
		// Verify scenarios bitmap includes all defined scenarios
		if d.Scenarios != uint32(def.DefinedScenarioMask()) {
			t.Errorf("%s: Scenarios = 0x%08X, want 0x%08X", name, d.Scenarios, def.DefinedScenarioMask())
		}
	}
}

func TestEvaluateController_EndpointIDZero(t *testing.T) {
	decls := EvaluateController(Registry)

	for _, d := range decls {
		if d.EndpointID != 0 {
			t.Errorf("0x%02X: EndpointID = %d, want 0", d.ID, d.EndpointID)
		}
	}
}

func TestEvaluateController_SubsetRegistry(t *testing.T) {
	// Custom smaller registry
	subset := map[UseCaseName]*UseCaseDef{
		"LPC": Registry["LPC"],
		"MPD": Registry["MPD"],
	}

	decls := EvaluateController(subset)

	if len(decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(decls))
	}

	// Should be sorted by ID: LPC (0x01), MPD (0x03)
	if decls[0].ID != uint16(LPCID) {
		t.Errorf("decls[0].ID = 0x%02X, want 0x%02X", decls[0].ID, LPCID)
	}
	if decls[1].ID != uint16(MPDID) {
		t.Errorf("decls[1].ID = 0x%02X, want 0x%02X", decls[1].ID, MPDID)
	}

	for _, d := range decls {
		if d.EndpointID != 0 {
			t.Errorf("0x%02X: EndpointID = %d, want 0", d.ID, d.EndpointID)
		}
	}
}

func TestEvaluateDevice_CorrectVersions(t *testing.T) {
	// Verify versions come from the registry
	ec := features.NewEnergyControl()
	ec.SetCapabilities(true, false, false, false, false, false, false)

	elec := features.NewElectrical()
	_ = elec.SetNominalMaxConsumption(10_000_000)

	device := testDevice(model.EndpointHeatPump, ec.Feature, elec.Feature)

	decls := EvaluateDevice(device, Registry)

	for _, d := range decls {
		// Find matching registry entry by ID
		var def *UseCaseDef
		for _, regDef := range Registry {
			if uint16(regDef.ID) == d.ID {
				def = regDef
				break
			}
		}
		if def == nil {
			t.Errorf("declaration 0x%02X not in registry", d.ID)
			continue
		}
		if d.Major != def.Major {
			t.Errorf("0x%02X: Major = %d, want %d", d.ID, d.Major, def.Major)
		}
		if d.Minor != def.Minor {
			t.Errorf("0x%02X: Minor = %d, want %d", d.ID, d.Minor, def.Minor)
		}
	}
}

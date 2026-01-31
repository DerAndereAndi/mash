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
	declMap := make(map[string]*model.UseCaseDecl)
	for _, d := range decls {
		declMap[d.Name] = d
	}

	expectedUCs := []string{"LPC", "MPD", "EVC"}
	for _, name := range expectedUCs {
		d, ok := declMap[name]
		if !ok {
			t.Errorf("expected use case %s to match", name)
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
	}
}

func TestEvaluateDevice_MeasurementOnly(t *testing.T) {
	// Device with only Measurement on an EV_CHARGER endpoint
	meas := features.NewMeasurement()
	device := testDevice(model.EndpointEVCharger, meas.Feature)

	decls := EvaluateDevice(device, Registry)

	// Should match MPD only (Measurement required, rest optional)
	if len(decls) != 1 {
		var names []string
		for _, d := range decls {
			names = append(names, d.Name)
		}
		t.Fatalf("expected 1 use case (MPD), got %d: %v", len(decls), names)
	}
	if decls[0].Name != "MPD" {
		t.Errorf("expected MPD, got %s", decls[0].Name)
	}
}

func TestEvaluateController_AllRegistered(t *testing.T) {
	decls := EvaluateController(Registry)

	if len(decls) != len(Registry) {
		t.Fatalf("expected %d declarations, got %d", len(Registry), len(decls))
	}

	// Build lookup
	declMap := make(map[string]*model.UseCaseDecl)
	for _, d := range decls {
		declMap[d.Name] = d
	}

	// Verify all registry entries are present with correct versions
	for name, def := range Registry {
		d, ok := declMap[string(name)]
		if !ok {
			t.Errorf("missing declaration for %s", name)
			continue
		}
		if d.Major != def.Major {
			t.Errorf("%s: Major = %d, want %d", name, d.Major, def.Major)
		}
		if d.Minor != def.Minor {
			t.Errorf("%s: Minor = %d, want %d", name, d.Minor, def.Minor)
		}
	}
}

func TestEvaluateController_EndpointIDZero(t *testing.T) {
	decls := EvaluateController(Registry)

	for _, d := range decls {
		if d.EndpointID != 0 {
			t.Errorf("%s: EndpointID = %d, want 0", d.Name, d.EndpointID)
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

	// Should be sorted by name: LPC, MPD
	if decls[0].Name != "LPC" {
		t.Errorf("decls[0].Name = %s, want LPC", decls[0].Name)
	}
	if decls[1].Name != "MPD" {
		t.Errorf("decls[1].Name = %s, want MPD", decls[1].Name)
	}

	for _, d := range decls {
		if d.EndpointID != 0 {
			t.Errorf("%s: EndpointID = %d, want 0", d.Name, d.EndpointID)
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
		def, ok := Registry[UseCaseName(d.Name)]
		if !ok {
			t.Errorf("declaration %s not in registry", d.Name)
			continue
		}
		if d.Major != def.Major {
			t.Errorf("%s: Major = %d, want %d", d.Name, d.Major, def.Major)
		}
		if d.Minor != def.Minor {
			t.Errorf("%s: Minor = %d, want %d", d.Name, d.Minor, def.Minor)
		}
	}
}

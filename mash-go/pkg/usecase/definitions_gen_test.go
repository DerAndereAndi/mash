package usecase

import "testing"

// TestRegistry_AllKeysHaveConstants verifies every registry key has a matching UseCaseName constant.
func TestRegistry_AllKeysHaveConstants(t *testing.T) {
	knownConstants := map[UseCaseName]bool{
		LPC: true, LPP: true, MPD: true, EVC: true,
		COB: true, FLOA: true, ITPCM: true, OHPCF: true,
		PODF: true, POEN: true, TOUT: true,
	}
	for key := range Registry {
		if !knownConstants[key] {
			t.Errorf("Registry key %q has no matching UseCaseName constant", key)
		}
	}
}

func TestRegistry_ContainsLPC(t *testing.T) {
	def, ok := Registry[LPC]
	if !ok {
		t.Fatal("Registry missing LPC")
	}
	if def.Name != LPC {
		t.Errorf("name = %q, want LPC", def.Name)
	}
	if def.ID != LPCID {
		t.Errorf("ID = 0x%02X, want 0x%02X", def.ID, LPCID)
	}
	if def.FullName != "Limit Power Consumption" {
		t.Errorf("fullName = %q", def.FullName)
	}
}

func TestRegistry_ContainsLPP(t *testing.T) {
	def, ok := Registry[LPP]
	if !ok {
		t.Fatal("Registry missing LPP")
	}
	if def.Name != LPP {
		t.Errorf("name = %q, want LPP", def.Name)
	}
	if def.ID != LPPID {
		t.Errorf("ID = 0x%02X, want 0x%02X", def.ID, LPPID)
	}
}

func TestRegistry_ContainsMPD(t *testing.T) {
	def, ok := Registry[MPD]
	if !ok {
		t.Fatal("Registry missing MPD")
	}
	if def.Name != MPD {
		t.Errorf("name = %q, want MPD", def.Name)
	}
	if def.FullName != "Monitor Power Device" {
		t.Errorf("fullName = %q, want Monitor Power Device", def.FullName)
	}
}

func TestRegistry_ContainsEVC(t *testing.T) {
	def, ok := Registry[EVC]
	if !ok {
		t.Fatal("Registry missing EVC")
	}
	if def.Name != EVC {
		t.Errorf("name = %q, want EVC", def.Name)
	}
	if def.FullName != "EV Charging" {
		t.Errorf("fullName = %q, want EV Charging", def.FullName)
	}
}

func TestLPC_EnergyControlRequired(t *testing.T) {
	def := Registry[LPC]
	base := def.BaseScenario()
	if base == nil {
		t.Fatal("LPC missing BASE scenario")
	}
	var ec *FeatureRequirement
	for i := range base.Features {
		if base.Features[i].FeatureName == "EnergyControl" {
			ec = &base.Features[i]
			break
		}
	}
	if ec == nil {
		t.Fatal("LPC BASE missing EnergyControl feature")
	}
	if ec.FeatureID != 0x05 {
		t.Errorf("EnergyControl FeatureID = 0x%02x, want 0x05", ec.FeatureID)
	}
	if !ec.Required {
		t.Error("EnergyControl should be required")
	}

	// Check acceptsLimits attribute
	if len(ec.Attributes) < 1 {
		t.Fatal("missing attributes")
	}
	al := ec.Attributes[0]
	if al.Name != "acceptsLimits" {
		t.Errorf("attribute name = %q", al.Name)
	}
	if al.AttrID != 10 {
		t.Errorf("acceptsLimits AttrID = %d, want 10", al.AttrID)
	}
	if al.RequiredValue == nil || !*al.RequiredValue {
		t.Error("acceptsLimits RequiredValue should be true")
	}
}

func TestLPC_ElectricalRequired(t *testing.T) {
	def := Registry[LPC]
	base := def.BaseScenario()
	if base == nil {
		t.Fatal("LPC missing BASE scenario")
	}
	var elec *FeatureRequirement
	for i := range base.Features {
		if base.Features[i].FeatureName == "Electrical" {
			elec = &base.Features[i]
			break
		}
	}
	if elec == nil {
		t.Fatal("LPC BASE missing Electrical feature")
	}
	if !elec.Required {
		t.Error("Electrical should be required")
	}
	if len(elec.Attributes) < 1 {
		t.Fatal("missing attributes")
	}
	if elec.Attributes[0].Name != "nominalMaxConsumption" {
		t.Errorf("attribute name = %q", elec.Attributes[0].Name)
	}
	if elec.Attributes[0].AttrID != 10 {
		t.Errorf("nominalMaxConsumption AttrID = %d, want 10", elec.Attributes[0].AttrID)
	}
}

func TestLPC_MeasurementScenario(t *testing.T) {
	def := Registry[LPC]
	// Measurement should be in the MEASUREMENT scenario (bit 1), not in BASE
	if len(def.Scenarios) < 2 {
		t.Fatalf("LPC should have at least 2 scenarios, got %d", len(def.Scenarios))
	}
	// Find MEASUREMENT scenario
	var measScenario *ScenarioDef
	for i := range def.Scenarios {
		if def.Scenarios[i].Name == "MEASUREMENT" {
			measScenario = &def.Scenarios[i]
			break
		}
	}
	if measScenario == nil {
		t.Fatal("LPC missing MEASUREMENT scenario")
	}
	if measScenario.Bit != 1 {
		t.Errorf("MEASUREMENT scenario bit = %d, want 1", measScenario.Bit)
	}
	if len(measScenario.Features) == 0 {
		t.Fatal("MEASUREMENT scenario has no features")
	}
	if measScenario.Features[0].FeatureName != "Measurement" {
		t.Errorf("MEASUREMENT scenario feature = %q, want Measurement", measScenario.Features[0].FeatureName)
	}
}

func TestLPC_Commands(t *testing.T) {
	def := Registry[LPC]
	expected := map[string]bool{
		"limit":    true,
		"clear":    true,
		"capacity": true,
		"override": true,
		"lpc-demo": true,
	}
	if len(def.Commands) != len(expected) {
		t.Errorf("commands count = %d, want %d", len(def.Commands), len(expected))
	}
	for _, cmd := range def.Commands {
		if !expected[cmd] {
			t.Errorf("unexpected command %q", cmd)
		}
	}
}

func TestMPD_NoCommands(t *testing.T) {
	def := Registry[MPD]
	if len(def.Commands) != 0 {
		t.Errorf("MPD should have no commands, got %v", def.Commands)
	}
}

func TestEVC_BaseScenario(t *testing.T) {
	def := Registry[EVC]
	base := def.BaseScenario()
	if base == nil {
		t.Fatal("EVC missing BASE scenario")
	}
	// Check EnergyControl is required in BASE
	var ec *FeatureRequirement
	for i := range base.Features {
		if base.Features[i].FeatureName == "EnergyControl" {
			ec = &base.Features[i]
			break
		}
	}
	if ec == nil {
		t.Fatal("EVC BASE missing EnergyControl feature")
	}
	if !ec.Required {
		t.Error("EnergyControl should be required")
	}
	// Check acceptsLimits = true
	if len(ec.Attributes) < 1 {
		t.Fatal("missing attributes")
	}
	al := ec.Attributes[0]
	if al.Name != "acceptsLimits" {
		t.Errorf("attribute name = %q, want acceptsLimits", al.Name)
	}
	if al.RequiredValue == nil || !*al.RequiredValue {
		t.Error("acceptsLimits RequiredValue should be true")
	}
}

func TestEVC_HasMultipleScenarios(t *testing.T) {
	def := Registry[EVC]
	if len(def.Scenarios) < 4 {
		t.Errorf("EVC should have at least 4 scenarios, got %d", len(def.Scenarios))
	}
}

func TestEVC_EndpointTypes(t *testing.T) {
	def := Registry[EVC]
	if len(def.EndpointTypes) != 1 || def.EndpointTypes[0] != "EV_CHARGER" {
		t.Errorf("endpoint types = %v, want [EV_CHARGER]", def.EndpointTypes)
	}
}

func TestMPD_EndpointTypes(t *testing.T) {
	def := Registry[MPD]
	expected := map[string]bool{
		"GRID_CONNECTION": true,
		"INVERTER":        true,
		"PV_STRING":       true,
		"BATTERY":         true,
		"EV_CHARGER":      true,
		"HEAT_PUMP":       true,
		"WATER_HEATER":    true,
		"HVAC":            true,
		"APPLIANCE":       true,
		"SUB_METER":       true,
	}
	if len(def.EndpointTypes) != len(expected) {
		t.Errorf("endpoint types count = %d, want %d", len(def.EndpointTypes), len(expected))
	}
	for _, et := range def.EndpointTypes {
		if !expected[et] {
			t.Errorf("unexpected endpoint type %q", et)
		}
	}
}

func TestMPD_MeasurementRequiredInBase(t *testing.T) {
	def := Registry[MPD]
	base := def.BaseScenario()
	if base == nil {
		t.Fatal("MPD missing BASE scenario")
	}
	var meas *FeatureRequirement
	for i := range base.Features {
		if base.Features[i].FeatureName == "Measurement" {
			meas = &base.Features[i]
			break
		}
	}
	if meas == nil {
		t.Fatal("MPD BASE missing Measurement feature")
	}
	if !meas.Required {
		t.Error("Measurement should be required for MPD BASE")
	}
}

func TestLPP_EndpointTypes(t *testing.T) {
	def := Registry[LPP]
	expected := map[string]bool{
		"INVERTER": true,
		"BATTERY":  true,
	}
	if len(def.EndpointTypes) != 2 {
		t.Errorf("endpoint types count = %d, want 2", len(def.EndpointTypes))
	}
	for _, et := range def.EndpointTypes {
		if !expected[et] {
			t.Errorf("unexpected endpoint type %q", et)
		}
	}
}

func TestRegistry_AllHaveVersion(t *testing.T) {
	for name, def := range Registry {
		if def.Major != 1 {
			t.Errorf("%s: Major = %d, want 1", name, def.Major)
		}
		if def.Minor != 0 {
			t.Errorf("%s: Minor = %d, want 0", name, def.Minor)
		}
	}
}

func TestRegistry_AllHaveID(t *testing.T) {
	for name, def := range Registry {
		if def.ID == 0 {
			t.Errorf("%s: ID should be non-zero", name)
		}
	}
}

func TestRegistry_AllHaveBaseScenario(t *testing.T) {
	for name, def := range Registry {
		if def.BaseScenario() == nil {
			t.Errorf("%s: missing BASE scenario", name)
		}
	}
}

func TestLPP_ElectricalNominalMaxProduction(t *testing.T) {
	def := Registry[LPP]
	base := def.BaseScenario()
	if base == nil {
		t.Fatal("LPP missing BASE scenario")
	}
	var elec *FeatureRequirement
	for i := range base.Features {
		if base.Features[i].FeatureName == "Electrical" {
			elec = &base.Features[i]
			break
		}
	}
	if elec == nil {
		t.Fatal("LPP BASE missing Electrical feature")
	}
	if len(elec.Attributes) < 1 {
		t.Fatal("missing attributes")
	}
	if elec.Attributes[0].Name != "nominalMaxProduction" {
		t.Errorf("attribute name = %q, want nominalMaxProduction", elec.Attributes[0].Name)
	}
	if elec.Attributes[0].AttrID != 11 {
		t.Errorf("nominalMaxProduction AttrID = %d, want 11", elec.Attributes[0].AttrID)
	}
}

func TestNameToID_Mapping(t *testing.T) {
	if NameToID[LPC] != LPCID {
		t.Errorf("NameToID[LPC] = 0x%02X, want 0x%02X", NameToID[LPC], LPCID)
	}
	if NameToID[EVC] != EVCID {
		t.Errorf("NameToID[EVC] = 0x%02X, want 0x%02X", NameToID[EVC], EVCID)
	}
}

func TestIDToName_Mapping(t *testing.T) {
	if IDToName[LPCID] != LPC {
		t.Errorf("IDToName[0x01] = %q, want LPC", IDToName[LPCID])
	}
	if IDToName[EVCID] != EVC {
		t.Errorf("IDToName[0x04] = %q, want EVC", IDToName[EVCID])
	}
}

package usecase

import "testing"

func TestRegistry_ContainsLPC(t *testing.T) {
	def, ok := Registry[LPC]
	if !ok {
		t.Fatal("Registry missing LPC")
	}
	if def.Name != LPC {
		t.Errorf("name = %q, want LPC", def.Name)
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
	var ec *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "EnergyControl" {
			ec = &def.Features[i]
			break
		}
	}
	if ec == nil {
		t.Fatal("LPC missing EnergyControl feature")
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
	var elec *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Electrical" {
			elec = &def.Features[i]
			break
		}
	}
	if elec == nil {
		t.Fatal("LPC missing Electrical feature")
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

func TestLPC_MeasurementOptional(t *testing.T) {
	def := Registry[LPC]
	var meas *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Measurement" {
			meas = &def.Features[i]
			break
		}
	}
	if meas == nil {
		t.Fatal("LPC missing Measurement feature")
	}
	if meas.Required {
		t.Error("Measurement should not be required")
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

func TestEVC_ChargingSessionRequired(t *testing.T) {
	def := Registry[EVC]
	var cs *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "ChargingSession" {
			cs = &def.Features[i]
			break
		}
	}
	if cs == nil {
		t.Fatal("EVC missing ChargingSession feature")
	}
	if !cs.Required {
		t.Error("ChargingSession should be required")
	}
	// Check evDemandMode attribute is present
	found := false
	for _, attr := range cs.Attributes {
		if attr.Name == "evDemandMode" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ChargingSession missing evDemandMode attribute")
	}
}

func TestEVC_EnergyControlRequired(t *testing.T) {
	def := Registry[EVC]
	var ec *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "EnergyControl" {
			ec = &def.Features[i]
			break
		}
	}
	if ec == nil {
		t.Fatal("EVC missing EnergyControl feature")
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

func TestEVC_SignalsOptional(t *testing.T) {
	def := Registry[EVC]
	var sig *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Signals" {
			sig = &def.Features[i]
			break
		}
	}
	if sig == nil {
		t.Fatal("EVC missing Signals feature")
	}
	if sig.Required {
		t.Error("Signals should be optional")
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

func TestMPD_MeasurementRequired(t *testing.T) {
	def := Registry[MPD]
	var meas *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Measurement" {
			meas = &def.Features[i]
			break
		}
	}
	if meas == nil {
		t.Fatal("MPD missing Measurement feature")
	}
	if !meas.Required {
		t.Error("Measurement should be required for MPD")
	}
}

func TestMPD_StatusOptional(t *testing.T) {
	def := Registry[MPD]
	var status *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Status" {
			status = &def.Features[i]
			break
		}
	}
	if status == nil {
		t.Fatal("MPD missing Status feature")
	}
	if status.Required {
		t.Error("Status should be optional for MPD")
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

func TestLPP_ElectricalNominalMaxProduction(t *testing.T) {
	def := Registry[LPP]
	var elec *FeatureRequirement
	for i := range def.Features {
		if def.Features[i].FeatureName == "Electrical" {
			elec = &def.Features[i]
			break
		}
	}
	if elec == nil {
		t.Fatal("LPP missing Electrical feature")
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

package usecase

import (
	"strings"
	"testing"
)

const testLPCYAML = `
name: LPC
fullName: Limit Power Consumption
specVersion: "1.0"
description: Controller limits active power consumption of a device.

endpointTypes:
  - INVERTER
  - EV_CHARGER
  - BATTERY
  - HEAT_PUMP
  - WATER_HEATER
  - HVAC
  - APPLIANCE

features:
  - feature: EnergyControl
    required: true
    attributes:
      - name: acceptsLimits
        requiredValue: true
    commands:
      - setLimit
      - clearLimit
    subscribe: all

  - feature: Electrical
    required: true
    attributes:
      - name: nominalMaxConsumption

  - feature: Measurement
    required: false
    subscribe: all

commands:
  - limit
  - clear
  - capacity
  - override
  - lpc-demo
`

func TestParseUseCaseYAML(t *testing.T) {
	raw, err := ParseRawUseCaseDef([]byte(testLPCYAML))
	if err != nil {
		t.Fatalf("ParseRawUseCaseDef: %v", err)
	}

	if raw.Name != "LPC" {
		t.Errorf("name = %q, want LPC", raw.Name)
	}
	if raw.FullName != "Limit Power Consumption" {
		t.Errorf("fullName = %q, want 'Limit Power Consumption'", raw.FullName)
	}
	if raw.SpecVersion != "1.0" {
		t.Errorf("specVersion = %q, want '1.0'", raw.SpecVersion)
	}
	if len(raw.EndpointTypes) != 7 {
		t.Errorf("endpointTypes length = %d, want 7", len(raw.EndpointTypes))
	}
	if len(raw.Features) != 3 {
		t.Errorf("features length = %d, want 3", len(raw.Features))
	}
	if len(raw.Commands) != 5 {
		t.Errorf("commands length = %d, want 5", len(raw.Commands))
	}

	// Check first feature
	ec := raw.Features[0]
	if ec.Feature != "EnergyControl" {
		t.Errorf("feature[0].Feature = %q, want EnergyControl", ec.Feature)
	}
	if !ec.Required {
		t.Error("feature[0].Required should be true")
	}
	if len(ec.Attributes) != 1 {
		t.Errorf("feature[0].Attributes length = %d, want 1", len(ec.Attributes))
	}
	if ec.Attributes[0].Name != "acceptsLimits" {
		t.Errorf("feature[0].Attributes[0].Name = %q, want acceptsLimits", ec.Attributes[0].Name)
	}
	if ec.Attributes[0].RequiredValue == nil || !*ec.Attributes[0].RequiredValue {
		t.Error("feature[0].Attributes[0].RequiredValue should be true")
	}
	if len(ec.Commands) != 2 {
		t.Errorf("feature[0].Commands length = %d, want 2", len(ec.Commands))
	}
	if ec.Subscribe != "all" {
		t.Errorf("feature[0].Subscribe = %q, want \"all\"", ec.Subscribe)
	}
}

func TestResolveNames_Valid(t *testing.T) {
	raw, err := ParseRawUseCaseDef([]byte(testLPCYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	def, err := ResolveUseCaseDef(raw)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if def.Name != LPC {
		t.Errorf("name = %q, want LPC", def.Name)
	}

	// Check EnergyControl feature
	if len(def.Features) != 3 {
		t.Fatalf("features count = %d, want 3", len(def.Features))
	}

	ec := def.Features[0]
	if ec.FeatureName != "EnergyControl" {
		t.Errorf("feature name = %q, want EnergyControl", ec.FeatureName)
	}
	if ec.FeatureID != 0x05 {
		t.Errorf("feature ID = 0x%02x, want 0x05", ec.FeatureID)
	}
	if !ec.Required {
		t.Error("EnergyControl should be required")
	}

	// Check acceptsLimits attribute
	if len(ec.Attributes) != 1 {
		t.Fatalf("ec attributes count = %d, want 1", len(ec.Attributes))
	}
	if ec.Attributes[0].AttrID != 10 {
		t.Errorf("acceptsLimits AttrID = %d, want 10", ec.Attributes[0].AttrID)
	}
	if ec.Attributes[0].RequiredValue == nil || !*ec.Attributes[0].RequiredValue {
		t.Error("acceptsLimits RequiredValue should be true")
	}

	// Check commands
	if len(ec.Commands) != 2 {
		t.Fatalf("ec commands count = %d, want 2", len(ec.Commands))
	}
	if ec.Commands[0].CommandID != 1 { // setLimit
		t.Errorf("setLimit CommandID = %d, want 1", ec.Commands[0].CommandID)
	}
	if ec.Commands[1].CommandID != 2 { // clearLimit
		t.Errorf("clearLimit CommandID = %d, want 2", ec.Commands[1].CommandID)
	}

	// Check subscriptions (subscribe: all)
	if !ec.SubscribeAll {
		t.Error("EnergyControl SubscribeAll should be true")
	}

	// Check Electrical feature
	elec := def.Features[1]
	if elec.FeatureID != 0x03 {
		t.Errorf("Electrical FeatureID = 0x%02x, want 0x03", elec.FeatureID)
	}
	if !elec.Required {
		t.Error("Electrical should be required")
	}
	if len(elec.Attributes) != 1 {
		t.Fatalf("elec attributes count = %d, want 1", len(elec.Attributes))
	}
	if elec.Attributes[0].AttrID != 10 { // nominalMaxConsumption
		t.Errorf("nominalMaxConsumption AttrID = %d, want 10", elec.Attributes[0].AttrID)
	}

	// Check Measurement feature
	meas := def.Features[2]
	if meas.FeatureID != 0x04 {
		t.Errorf("Measurement FeatureID = 0x%02x, want 0x04", meas.FeatureID)
	}
	if meas.Required {
		t.Error("Measurement should not be required")
	}
}

func TestResolveNames_InvalidFeature(t *testing.T) {
	yaml := `
name: BAD
fullName: Bad Use Case
specVersion: "1.0"
features:
  - feature: NonExistentFeature
    required: true
commands: []
`
	raw, err := ParseRawUseCaseDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = ResolveUseCaseDef(raw)
	if err == nil {
		t.Fatal("expected error for unknown feature")
	}
	if !strings.Contains(err.Error(), "NonExistentFeature") {
		t.Errorf("error should mention feature name, got: %v", err)
	}
}

func TestResolveNames_InvalidAttribute(t *testing.T) {
	yaml := `
name: BAD
fullName: Bad Use Case
specVersion: "1.0"
features:
  - feature: EnergyControl
    required: true
    attributes:
      - name: nonExistentAttribute
commands: []
`
	raw, err := ParseRawUseCaseDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = ResolveUseCaseDef(raw)
	if err == nil {
		t.Fatal("expected error for unknown attribute")
	}
	if !strings.Contains(err.Error(), "nonExistentAttribute") {
		t.Errorf("error should mention attribute name, got: %v", err)
	}
}

func TestResolveNames_InvalidCommand(t *testing.T) {
	yaml := `
name: BAD
fullName: Bad Use Case
specVersion: "1.0"
features:
  - feature: EnergyControl
    required: true
    commands:
      - nonExistentCommand
commands: []
`
	raw, err := ParseRawUseCaseDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = ResolveUseCaseDef(raw)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "nonExistentCommand") {
		t.Errorf("error should mention command name, got: %v", err)
	}
}

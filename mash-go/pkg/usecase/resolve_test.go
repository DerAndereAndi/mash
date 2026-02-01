package usecase

import (
	"strings"
	"testing"
)

const testGPLYAML = `
name: GPL
id: 0x01
fullName: Grid Power Limitation
specVersion: "1.0"
major: 1
minor: 0
description: Controller limits active power consumption and/or production of a device.

endpointTypes:
  - INVERTER
  - EV_CHARGER
  - BATTERY
  - HEAT_PUMP
  - WATER_HEATER
  - HVAC
  - APPLIANCE
  - GRID_CONNECTION

scenarios:
  - bit: 0
    name: BASE
    description: Shared control infrastructure.
    requiresAny:
      - CONSUMPTION
      - PRODUCTION
    features:
      - feature: EnergyControl
        required: true
        attributes:
          - name: acceptsLimits
            requiredValue: true
          - name: failsafeDuration
        commands:
          - setLimit
          - clearLimit
        subscribe: all

  - bit: 1
    name: CONSUMPTION
    description: Power consumption limiting.
    requires:
      - BASE
    features:
      - feature: Electrical
        required: true
        attributes:
          - name: nominalMaxConsumption

  - bit: 2
    name: PRODUCTION
    description: Power production limiting.
    requires:
      - BASE
    endpointTypes:
      - INVERTER
      - BATTERY
      - GRID_CONNECTION
    features:
      - feature: Electrical
        required: true
        attributes:
          - name: nominalMaxProduction

  - bit: 3
    name: MEASUREMENT
    description: Measurement telemetry.
    features:
      - feature: Measurement
        subscribe: all

commands:
  - limit
  - clear
  - capacity
  - override
  - failsafe
`

func TestParseUseCaseYAML(t *testing.T) {
	raw, err := ParseRawUseCaseDef([]byte(testGPLYAML))
	if err != nil {
		t.Fatalf("ParseRawUseCaseDef: %v", err)
	}

	if raw.Name != "GPL" {
		t.Errorf("name = %q, want GPL", raw.Name)
	}
	if raw.ID != 0x01 {
		t.Errorf("id = 0x%02x, want 0x01", raw.ID)
	}
	if raw.FullName != "Grid Power Limitation" {
		t.Errorf("fullName = %q, want 'Grid Power Limitation'", raw.FullName)
	}
	if raw.SpecVersion != "1.0" {
		t.Errorf("specVersion = %q, want '1.0'", raw.SpecVersion)
	}
	if len(raw.EndpointTypes) != 8 {
		t.Errorf("endpointTypes length = %d, want 8", len(raw.EndpointTypes))
	}
	if len(raw.Scenarios) != 4 {
		t.Errorf("scenarios length = %d, want 4", len(raw.Scenarios))
	}
	if len(raw.Commands) != 5 {
		t.Errorf("commands length = %d, want 5", len(raw.Commands))
	}

	// Check BASE scenario
	base := raw.Scenarios[0]
	if base.Name != "BASE" {
		t.Errorf("scenarios[0].Name = %q, want BASE", base.Name)
	}
	if len(base.RequiresAny) != 2 {
		t.Errorf("BASE.RequiresAny length = %d, want 2", len(base.RequiresAny))
	}
	if len(base.Features) != 1 {
		t.Fatalf("scenarios[0].Features length = %d, want 1", len(base.Features))
	}
	ec := base.Features[0]
	if ec.Feature != "EnergyControl" {
		t.Errorf("feature[0].Feature = %q, want EnergyControl", ec.Feature)
	}
	if !ec.Required {
		t.Error("feature[0].Required should be true")
	}
	if len(ec.Attributes) != 2 {
		t.Errorf("feature[0].Attributes length = %d, want 2", len(ec.Attributes))
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

	// Check CONSUMPTION scenario
	cons := raw.Scenarios[1]
	if cons.Name != "CONSUMPTION" {
		t.Errorf("scenarios[1].Name = %q, want CONSUMPTION", cons.Name)
	}
	if len(cons.Requires) != 1 || cons.Requires[0] != "BASE" {
		t.Errorf("CONSUMPTION.Requires = %v, want [BASE]", cons.Requires)
	}

	// Check PRODUCTION scenario
	prod := raw.Scenarios[2]
	if prod.Name != "PRODUCTION" {
		t.Errorf("scenarios[2].Name = %q, want PRODUCTION", prod.Name)
	}
	if len(prod.EndpointTypes) != 3 {
		t.Errorf("PRODUCTION.EndpointTypes length = %d, want 3", len(prod.EndpointTypes))
	}
}

func TestResolveNames_Valid(t *testing.T) {
	raw, err := ParseRawUseCaseDef([]byte(testGPLYAML))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	def, err := ResolveUseCaseDef(raw)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if def.Name != GPL {
		t.Errorf("name = %q, want GPL", def.Name)
	}
	if def.ID != 0x01 {
		t.Errorf("ID = 0x%02x, want 0x01", def.ID)
	}

	// Check scenarios
	if len(def.Scenarios) != 4 {
		t.Fatalf("scenarios count = %d, want 4", len(def.Scenarios))
	}

	base := def.Scenarios[0]
	if base.Name != "BASE" {
		t.Errorf("scenario[0].Name = %q, want BASE", base.Name)
	}
	if base.Bit != 0 {
		t.Errorf("scenario[0].Bit = %d, want 0", base.Bit)
	}
	if len(base.RequiresAny) != 2 {
		t.Errorf("BASE.RequiresAny = %v, want 2 items", base.RequiresAny)
	}
	if len(base.Features) != 1 {
		t.Fatalf("scenario[0] features count = %d, want 1", len(base.Features))
	}

	// Check EnergyControl feature
	ec := base.Features[0]
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
	if len(ec.Attributes) != 2 {
		t.Fatalf("ec attributes count = %d, want 2", len(ec.Attributes))
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

	// Check CONSUMPTION scenario
	cons := def.Scenarios[1]
	if cons.Name != "CONSUMPTION" {
		t.Errorf("scenario[1].Name = %q, want CONSUMPTION", cons.Name)
	}
	if cons.Bit != 1 {
		t.Errorf("scenario[1].Bit = %d, want 1", cons.Bit)
	}
	if len(cons.Requires) != 1 || cons.Requires[0] != "BASE" {
		t.Errorf("CONSUMPTION.Requires = %v", cons.Requires)
	}

	// Check PRODUCTION scenario
	prod := def.Scenarios[2]
	if prod.Name != "PRODUCTION" {
		t.Errorf("scenario[2].Name = %q, want PRODUCTION", prod.Name)
	}
	if len(prod.EndpointTypes) != 3 {
		t.Errorf("PRODUCTION.EndpointTypes = %v", prod.EndpointTypes)
	}

	// Check MEASUREMENT scenario
	meas := def.Scenarios[3]
	if meas.Name != "MEASUREMENT" {
		t.Errorf("scenario[3].Name = %q, want MEASUREMENT", meas.Name)
	}
	if meas.Bit != 3 {
		t.Errorf("scenario[3].Bit = %d, want 3", meas.Bit)
	}
	if len(meas.Features) != 1 {
		t.Fatalf("scenario[3] features count = %d, want 1", len(meas.Features))
	}
	if meas.Features[0].FeatureID != 0x04 {
		t.Errorf("Measurement FeatureID = 0x%02x, want 0x04", meas.Features[0].FeatureID)
	}
}

func TestResolveNames_LegacyFlatFormat(t *testing.T) {
	// Test backward compatibility: flat features list wraps into BASE scenario
	yaml := `
name: TEST
fullName: Test Use Case
specVersion: "1.0"
features:
  - feature: EnergyControl
    required: true
    attributes:
      - name: acceptsLimits
        requiredValue: true
    commands:
      - setLimit
    subscribe: all
commands: []
`
	raw, err := ParseRawUseCaseDef([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	def, err := ResolveUseCaseDef(raw)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if len(def.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario (auto-wrapped BASE), got %d", len(def.Scenarios))
	}
	if def.Scenarios[0].Name != "BASE" {
		t.Errorf("scenario name = %q, want BASE", def.Scenarios[0].Name)
	}
	if def.Scenarios[0].Bit != 0 {
		t.Errorf("scenario bit = %d, want 0", def.Scenarios[0].Bit)
	}
	if len(def.Scenarios[0].Features) != 1 {
		t.Errorf("features count = %d, want 1", len(def.Scenarios[0].Features))
	}
}

func TestResolveNames_InvalidFeature(t *testing.T) {
	yaml := `
name: BAD
fullName: Bad Use Case
specVersion: "1.0"
scenarios:
  - bit: 0
    name: BASE
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
scenarios:
  - bit: 0
    name: BASE
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
scenarios:
  - bit: 0
    name: BASE
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

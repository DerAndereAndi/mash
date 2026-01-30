package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

// docsDir returns the absolute path to docs/features/ relative to this test file.
func docsDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs", "features")
}

func TestParseFeatureDef_MinimalFeature(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x42
revision: 2
mandatory: true
description: "A test feature"
attributes:
  - id: 1
    name: testAttr
    type: uint8
    access: readOnly
    mandatory: true
    description: "A test attribute"
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}

	if def.Name != "TestFeature" {
		t.Errorf("name = %q, want TestFeature", def.Name)
	}
	if def.ID != 0x42 {
		t.Errorf("id = 0x%02x, want 0x42", def.ID)
	}
	if def.Revision != 2 {
		t.Errorf("revision = %d, want 2", def.Revision)
	}
	if !def.Mandatory {
		t.Error("mandatory = false, want true")
	}
	if def.Description != "A test feature" {
		t.Errorf("description = %q, want %q", def.Description, "A test feature")
	}
	if len(def.Attributes) != 1 {
		t.Fatalf("len(attributes) = %d, want 1", len(def.Attributes))
	}

	attr := def.Attributes[0]
	if attr.ID != 1 {
		t.Errorf("attr.id = %d, want 1", attr.ID)
	}
	if attr.Name != "testAttr" {
		t.Errorf("attr.name = %q, want testAttr", attr.Name)
	}
	if attr.Type != "uint8" {
		t.Errorf("attr.type = %q, want uint8", attr.Type)
	}
	if attr.Access != "readOnly" {
		t.Errorf("attr.access = %q, want readOnly", attr.Access)
	}
	if !attr.Mandatory {
		t.Error("attr.mandatory = false, want true")
	}
}

func TestParseFeatureDef_WithEnums(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
enums:
  - name: TestEnum
    type: uint8
    description: "Test enum"
    values:
      - { name: FOO, value: 0x00, description: "Foo val" }
      - { name: BAR, value: 0x01, description: "Bar val" }
      - { name: BAZ, value: 0x02 }
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Enums) != 1 {
		t.Fatalf("len(enums) = %d, want 1", len(def.Enums))
	}

	enum := def.Enums[0]
	if enum.Name != "TestEnum" {
		t.Errorf("enum.name = %q, want TestEnum", enum.Name)
	}
	if enum.Type != "uint8" {
		t.Errorf("enum.type = %q, want uint8", enum.Type)
	}
	if len(enum.Values) != 3 {
		t.Fatalf("len(values) = %d, want 3", len(enum.Values))
	}
	if enum.Values[0].Name != "FOO" || enum.Values[0].Value != 0 {
		t.Errorf("values[0] = %+v, want FOO/0x00", enum.Values[0])
	}
	if enum.Values[1].Name != "BAR" || enum.Values[1].Value != 1 {
		t.Errorf("values[1] = %+v, want BAR/0x01", enum.Values[1])
	}
	if enum.Values[2].Description != "" {
		t.Errorf("values[2].description = %q, want empty", enum.Values[2].Description)
	}
}

func TestParseFeatureDef_WithNullableMapAttribute(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
attributes:
  - id: 10
    name: perPhaseValues
    type: map
    mapKeyType: Phase
    mapValueType: int64
    access: readOnly
    nullable: true
    description: "Per-phase values"
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Attributes) != 1 {
		t.Fatalf("len(attributes) = %d, want 1", len(def.Attributes))
	}

	attr := def.Attributes[0]
	if attr.Type != "map" {
		t.Errorf("type = %q, want map", attr.Type)
	}
	if attr.MapKeyType != "Phase" {
		t.Errorf("mapKeyType = %q, want Phase", attr.MapKeyType)
	}
	if attr.MapValueType != "int64" {
		t.Errorf("mapValueType = %q, want int64", attr.MapValueType)
	}
	if !attr.Nullable {
		t.Error("nullable = false, want true")
	}
}

func TestParseFeatureDef_WithCommand(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
commands:
  - id: 1
    name: testCommand
    mandatory: true
    description: "A test command"
    parameters:
      - { name: limit, type: int64, required: false, description: "The limit" }
      - { name: cause, type: uint8, enum: LimitCause, required: true }
    response:
      - { name: applied, type: bool, required: true }
      - { name: reason, type: uint8, enum: RejectReason, required: false }
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(def.Commands))
	}

	cmd := def.Commands[0]
	if cmd.ID != 1 {
		t.Errorf("cmd.id = %d, want 1", cmd.ID)
	}
	if cmd.Name != "testCommand" {
		t.Errorf("cmd.name = %q, want testCommand", cmd.Name)
	}
	if !cmd.Mandatory {
		t.Error("cmd.mandatory = false, want true")
	}
	if len(cmd.Parameters) != 2 {
		t.Fatalf("len(params) = %d, want 2", len(cmd.Parameters))
	}
	if cmd.Parameters[0].Name != "limit" || cmd.Parameters[0].Type != "int64" || cmd.Parameters[0].Required {
		t.Errorf("params[0] = %+v, want limit/int64/optional", cmd.Parameters[0])
	}
	if cmd.Parameters[1].Enum != "LimitCause" || !cmd.Parameters[1].Required {
		t.Errorf("params[1] = %+v, want cause/uint8/LimitCause/required", cmd.Parameters[1])
	}
	if len(cmd.Response) != 2 {
		t.Fatalf("len(response) = %d, want 2", len(cmd.Response))
	}
	if cmd.Response[0].Name != "applied" || cmd.Response[0].Type != "bool" {
		t.Errorf("response[0] = %+v, want applied/bool", cmd.Response[0])
	}
}

func TestParseSharedTypes(t *testing.T) {
	yaml := `
version: "1.0"
enums:
  - name: Phase
    type: uint8
    description: "Device phase"
    values:
      - { name: A, value: 0x00 }
      - { name: B, value: 0x01 }
      - { name: C, value: 0x02 }
  - name: Direction
    type: uint8
    values:
      - { name: CONSUMPTION, value: 0x00 }
      - { name: PRODUCTION, value: 0x01 }
`
	shared, err := ParseSharedTypes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSharedTypes failed: %v", err)
	}
	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}
	if len(shared.Enums) != 2 {
		t.Fatalf("len(enums) = %d, want 2", len(shared.Enums))
	}
	if shared.Enums[0].Name != "Phase" || len(shared.Enums[0].Values) != 3 {
		t.Errorf("enums[0] = %+v, want Phase with 3 values", shared.Enums[0])
	}
}

func TestParseFeatureDef_AllAccessModes(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
attributes:
  - id: 1
    name: readOnlyAttr
    type: uint8
    access: readOnly
  - id: 2
    name: readWriteAttr
    type: string
    access: readWrite
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if def.Attributes[0].Access != "readOnly" {
		t.Errorf("attr[0].access = %q, want readOnly", def.Attributes[0].Access)
	}
	if def.Attributes[1].Access != "readWrite" {
		t.Errorf("attr[1].access = %q, want readWrite", def.Attributes[1].Access)
	}
}

func TestParseFeatureDef_DefaultValues(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
attributes:
  - id: 1
    name: intDefault
    type: uint8
    access: readOnly
    default: 42
  - id: 2
    name: strDefault
    type: string
    access: readOnly
    default: "1.0"
  - id: 3
    name: enumDefault
    type: uint8
    enum: OperatingState
    access: readOnly
    default: UNKNOWN
  - id: 4
    name: boolDefault
    type: bool
    access: readOnly
    default: false
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Attributes) != 4 {
		t.Fatalf("len(attributes) = %d, want 4", len(def.Attributes))
	}

	// Integer default
	if v, ok := def.Attributes[0].Default.(int); !ok || v != 42 {
		t.Errorf("attr[0].default = %v (%T), want 42 (int)", def.Attributes[0].Default, def.Attributes[0].Default)
	}

	// String default
	if v, ok := def.Attributes[1].Default.(string); !ok || v != "1.0" {
		t.Errorf("attr[1].default = %v (%T), want \"1.0\" (string)", def.Attributes[1].Default, def.Attributes[1].Default)
	}

	// Enum default (comes as string from YAML)
	if v, ok := def.Attributes[2].Default.(string); !ok || v != "UNKNOWN" {
		t.Errorf("attr[2].default = %v (%T), want UNKNOWN (string)", def.Attributes[2].Default, def.Attributes[2].Default)
	}

	// Bool default
	if v, ok := def.Attributes[3].Default.(bool); !ok || v != false {
		t.Errorf("attr[3].default = %v (%T), want false (bool)", def.Attributes[3].Default, def.Attributes[3].Default)
	}
}

func TestParseFeatureDef_MissingName(t *testing.T) {
	yaml := `
id: 0x01
revision: 1
`
	_, err := ParseFeatureDef([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseFeatureDef_MinMax(t *testing.T) {
	yaml := `
name: TestFeature
id: 0x01
revision: 1
attributes:
  - id: 1
    name: bounded
    type: uint8
    access: readOnly
    min: 1
    max: 3
`
	def, err := ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	attr := def.Attributes[0]
	if v, ok := attr.Min.(int); !ok || v != 1 {
		t.Errorf("min = %v, want 1", attr.Min)
	}
	if v, ok := attr.Max.(int); !ok || v != 3 {
		t.Errorf("max = %v, want 3", attr.Max)
	}
}

// --- Integration tests against real YAML files ---

func TestParseStatusFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "status", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "Status" {
		t.Errorf("name = %q, want Status", def.Name)
	}
	if def.ID != 0x02 {
		t.Errorf("id = 0x%02x, want 0x02", def.ID)
	}
	if def.Revision != 1 {
		t.Errorf("revision = %d, want 1", def.Revision)
	}
	if def.Mandatory {
		t.Error("mandatory = true, want false")
	}

	// 1 enum: OperatingState with 9 values
	if len(def.Enums) != 1 {
		t.Fatalf("len(enums) = %d, want 1", len(def.Enums))
	}
	if def.Enums[0].Name != "OperatingState" {
		t.Errorf("enum.name = %q, want OperatingState", def.Enums[0].Name)
	}
	if len(def.Enums[0].Values) != 9 {
		t.Errorf("len(enum.values) = %d, want 9", len(def.Enums[0].Values))
	}

	// 4 attributes
	if len(def.Attributes) != 4 {
		t.Fatalf("len(attributes) = %d, want 4", len(def.Attributes))
	}

	// operatingState: enum ref, mandatory, default
	opState := def.Attributes[0]
	if opState.Name != "operatingState" {
		t.Errorf("attr[0].name = %q, want operatingState", opState.Name)
	}
	if opState.Enum != "OperatingState" {
		t.Errorf("attr[0].enum = %q, want OperatingState", opState.Enum)
	}
	if !opState.Mandatory {
		t.Error("attr[0].mandatory = false, want true")
	}
	if v, ok := opState.Default.(string); !ok || v != "UNKNOWN" {
		t.Errorf("attr[0].default = %v, want UNKNOWN", opState.Default)
	}

	// stateDetail: nullable
	if !def.Attributes[1].Nullable {
		t.Error("stateDetail should be nullable")
	}

	// faultCode: nullable
	if !def.Attributes[2].Nullable {
		t.Error("faultCode should be nullable")
	}

	// faultMessage: string, nullable
	if def.Attributes[3].Type != "string" {
		t.Errorf("faultMessage type = %q, want string", def.Attributes[3].Type)
	}
	if !def.Attributes[3].Nullable {
		t.Error("faultMessage should be nullable")
	}
}

func TestParseSharedTypesFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "_shared", "1.0.yaml")
	shared, err := LoadSharedTypes(path)
	if err != nil {
		t.Fatalf("LoadSharedTypes failed: %v", err)
	}

	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}

	// Expect 5 shared enums: Phase, GridPhase, PhasePair, Direction, AsymmetricSupport
	if len(shared.Enums) != 5 {
		t.Fatalf("len(enums) = %d, want 5", len(shared.Enums))
	}

	enumNames := make(map[string]int)
	for _, e := range shared.Enums {
		enumNames[e.Name] = len(e.Values)
	}

	expected := map[string]int{
		"Phase":              3,
		"GridPhase":          3,
		"PhasePair":          3,
		"Direction":          3,
		"AsymmetricSupport":  4,
	}
	for name, count := range expected {
		if got, ok := enumNames[name]; !ok {
			t.Errorf("missing enum %s", name)
		} else if got != count {
			t.Errorf("enum %s has %d values, want %d", name, got, count)
		}
	}
}

func TestParseDeviceInfoFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "device-info", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "DeviceInfo" {
		t.Errorf("name = %q, want DeviceInfo", def.Name)
	}
	if def.ID != 0x01 {
		t.Errorf("id = 0x%02x, want 0x01", def.ID)
	}
	if !def.Mandatory {
		t.Error("mandatory = false, want true")
	}
	if len(def.Attributes) != 12 {
		t.Errorf("len(attributes) = %d, want 12", len(def.Attributes))
	}
	if len(def.Commands) != 1 {
		t.Errorf("len(commands) = %d, want 1", len(def.Commands))
	}
	if def.Commands[0].Name != "removeZone" {
		t.Errorf("cmd.name = %q, want removeZone", def.Commands[0].Name)
	}
	if def.Commands[0].ID != 0x10 {
		t.Errorf("cmd.id = 0x%02x, want 0x10", def.Commands[0].ID)
	}
}

func TestParseElectricalFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "electrical", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "Electrical" {
		t.Errorf("name = %q, want Electrical", def.Name)
	}
	if def.ID != 0x03 {
		t.Errorf("id = 0x%02x, want 0x03", def.ID)
	}
	if len(def.Attributes) != 12 {
		t.Errorf("len(attributes) = %d, want 12", len(def.Attributes))
	}

	// Check phaseMapping is a map attribute
	phaseMapping := def.Attributes[1]
	if phaseMapping.Name != "phaseMapping" {
		t.Errorf("attr[1].name = %q, want phaseMapping", phaseMapping.Name)
	}
	if phaseMapping.Type != "map" || phaseMapping.MapKeyType != "Phase" || phaseMapping.MapValueType != "GridPhase" {
		t.Errorf("phaseMapping type/key/value = %s/%s/%s, want map/Phase/GridPhase",
			phaseMapping.Type, phaseMapping.MapKeyType, phaseMapping.MapValueType)
	}

	// Check supportedDirections has enum ref
	suppDir := def.Attributes[4]
	if suppDir.Enum != "Direction" {
		t.Errorf("supportedDirections.enum = %q, want Direction", suppDir.Enum)
	}
}

func TestParseMeasurementFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "measurement", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "Measurement" {
		t.Errorf("name = %q, want Measurement", def.Name)
	}
	if def.ID != 0x04 {
		t.Errorf("id = 0x%02x, want 0x04", def.ID)
	}
	if len(def.Attributes) != 24 {
		t.Errorf("len(attributes) = %d, want 24", len(def.Attributes))
	}

	// All attributes except acActivePower (mandatory) should be nullable
	for i, attr := range def.Attributes {
		if !attr.Nullable {
			if attr.Name != "acActivePower" {
				t.Errorf("attr[%d] %q should be nullable", i, attr.Name)
			}
		}
	}

	// Check map types
	acActivePPP := def.Attributes[3] // acActivePowerPerPhase
	if acActivePPP.Type != "map" || acActivePPP.MapKeyType != "Phase" {
		t.Errorf("acActivePowerPerPhase: type=%s key=%s, want map/Phase",
			acActivePPP.Type, acActivePPP.MapKeyType)
	}
}

func TestParseEnergyControlFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "energy-control", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "EnergyControl" {
		t.Errorf("name = %q, want EnergyControl", def.Name)
	}
	if def.ID != 0x05 {
		t.Errorf("id = 0x%02x, want 0x05", def.ID)
	}
	if len(def.Enums) != 8 {
		t.Errorf("len(enums) = %d, want 8", len(def.Enums))
	}
	if len(def.Commands) != 11 {
		t.Errorf("len(commands) = %d, want 11", len(def.Commands))
	}

	// Check setLimit command has params and response
	setLimit := def.Commands[0]
	if setLimit.Name != "setLimit" {
		t.Errorf("cmd[0].name = %q, want setLimit", setLimit.Name)
	}
	if len(setLimit.Parameters) != 4 {
		t.Errorf("setLimit params = %d, want 4", len(setLimit.Parameters))
	}
	if len(setLimit.Response) != 5 {
		t.Errorf("setLimit response = %d, want 5", len(setLimit.Response))
	}
}

func TestParseChargingSessionFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "charging-session", "1.0.yaml")
	def, err := LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}

	if def.Name != "ChargingSession" {
		t.Errorf("name = %q, want ChargingSession", def.Name)
	}
	if def.ID != 0x06 {
		t.Errorf("id = 0x%02x, want 0x06", def.ID)
	}
	if len(def.Enums) != 4 {
		t.Errorf("len(enums) = %d, want 4", len(def.Enums))
	}
	if len(def.Attributes) != 25 {
		t.Errorf("len(attributes) = %d, want 25", len(def.Attributes))
	}
	if len(def.Commands) != 1 {
		t.Errorf("len(commands) = %d, want 1", len(def.Commands))
	}
	if def.Commands[0].Name != "setChargingMode" {
		t.Errorf("cmd.name = %q, want setChargingMode", def.Commands[0].Name)
	}
}

func TestParseProtocolVersions_ModelTypes(t *testing.T) {
	yamlStr := `
versions:
  "1.0":
    description: "MASH Protocol v1.0"
    features:
      DeviceInfo: "1.0"
    shared: "1.0"
    feature_types:
      - { name: DeviceInfo, id: 0x01, description: "Device identity" }
      - { name: Status, id: 0x02, description: "Operating state" }
    endpoint_types:
      - { name: DEVICE_ROOT, id: 0x00, description: "Root endpoint" }
      - { name: EV_CHARGER, id: 0x05, description: "EVSE" }
      - { name: HVAC, id: 0x08, description: "HVAC system" }
`
	pv, err := ParseProtocolVersions([]byte(yamlStr))
	if err != nil {
		t.Fatalf("ParseProtocolVersions failed: %v", err)
	}

	ver, ok := pv.Versions["1.0"]
	if !ok {
		t.Fatal("version 1.0 not found")
	}

	// Feature types
	if len(ver.FeatureTypes) != 2 {
		t.Fatalf("len(feature_types) = %d, want 2", len(ver.FeatureTypes))
	}
	if ver.FeatureTypes[0].Name != "DeviceInfo" {
		t.Errorf("feature_types[0].name = %q, want DeviceInfo", ver.FeatureTypes[0].Name)
	}
	if ver.FeatureTypes[0].ID != 0x01 {
		t.Errorf("feature_types[0].id = 0x%02x, want 0x01", ver.FeatureTypes[0].ID)
	}
	if ver.FeatureTypes[0].Description != "Device identity" {
		t.Errorf("feature_types[0].description = %q, want %q", ver.FeatureTypes[0].Description, "Device identity")
	}
	if ver.FeatureTypes[1].Name != "Status" || ver.FeatureTypes[1].ID != 0x02 {
		t.Errorf("feature_types[1] = %+v, want Status/0x02", ver.FeatureTypes[1])
	}

	// Endpoint types
	if len(ver.EndpointTypes) != 3 {
		t.Fatalf("len(endpoint_types) = %d, want 3", len(ver.EndpointTypes))
	}
	if ver.EndpointTypes[0].Name != "DEVICE_ROOT" || ver.EndpointTypes[0].ID != 0x00 {
		t.Errorf("endpoint_types[0] = %+v, want DEVICE_ROOT/0x00", ver.EndpointTypes[0])
	}
	if ver.EndpointTypes[1].Name != "EV_CHARGER" || ver.EndpointTypes[1].ID != 0x05 {
		t.Errorf("endpoint_types[1] = %+v, want EV_CHARGER/0x05", ver.EndpointTypes[1])
	}
	if ver.EndpointTypes[2].Name != "HVAC" || ver.EndpointTypes[2].ID != 0x08 {
		t.Errorf("endpoint_types[2] = %+v, want HVAC/0x08", ver.EndpointTypes[2])
	}
}

func TestParseProtocolVersionsFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "protocol-versions.yaml")
	pv, err := LoadProtocolVersions(path)
	if err != nil {
		t.Fatalf("LoadProtocolVersions failed: %v", err)
	}

	ver, ok := pv.Versions["1.0"]
	if !ok {
		t.Fatal("version 1.0 not found")
	}
	if len(ver.Features) != 6 {
		t.Errorf("len(features) = %d, want 6", len(ver.Features))
	}
	if ver.Features["Status"] != "1.0" {
		t.Errorf("Status version = %q, want 1.0", ver.Features["Status"])
	}
	if ver.Shared != "1.0" {
		t.Errorf("shared = %q, want 1.0", ver.Shared)
	}
}

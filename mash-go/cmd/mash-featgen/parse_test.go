package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/specparse"
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Enums) != 1 {
		t.Fatalf("len(enums) = %d, want 1", len(def.Enums))
	}
	if def.Enums[0].Name != "TestEnum" {
		t.Errorf("enum.name = %q, want TestEnum", def.Enums[0].Name)
	}
	if len(def.Enums[0].Values) != 3 {
		t.Fatalf("len(values) = %d, want 3", len(def.Enums[0].Values))
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	attr := def.Attributes[0]
	if attr.Type != "map" || attr.MapKeyType != "Phase" || attr.MapValueType != "int64" {
		t.Errorf("attr = %+v, want map/Phase/int64", attr)
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(def.Commands))
	}
	cmd := def.Commands[0]
	if cmd.Name != "testCommand" || !cmd.Mandatory {
		t.Errorf("cmd = %+v, want testCommand/mandatory", cmd)
	}
	if len(cmd.Parameters) != 2 || len(cmd.Response) != 2 {
		t.Errorf("params=%d response=%d, want 2/2", len(cmd.Parameters), len(cmd.Response))
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseFeatureDef failed: %v", err)
	}
	if len(def.Attributes) != 4 {
		t.Fatalf("len(attributes) = %d, want 4", len(def.Attributes))
	}
	if v, ok := def.Attributes[0].Default.(int); !ok || v != 42 {
		t.Errorf("attr[0].default = %v (%T), want 42 (int)", def.Attributes[0].Default, def.Attributes[0].Default)
	}
	if v, ok := def.Attributes[1].Default.(string); !ok || v != "1.0" {
		t.Errorf("attr[1].default = %v (%T), want \"1.0\" (string)", def.Attributes[1].Default, def.Attributes[1].Default)
	}
	if v, ok := def.Attributes[2].Default.(string); !ok || v != "UNKNOWN" {
		t.Errorf("attr[2].default = %v (%T), want UNKNOWN (string)", def.Attributes[2].Default, def.Attributes[2].Default)
	}
	if v, ok := def.Attributes[3].Default.(bool); !ok || v != false {
		t.Errorf("attr[3].default = %v (%T), want false (bool)", def.Attributes[3].Default, def.Attributes[3].Default)
	}
}

func TestParseFeatureDef_MissingName(t *testing.T) {
	yaml := `
id: 0x01
revision: 1
`
	_, err := specparse.ParseFeatureDef([]byte(yaml))
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
	def, err := specparse.ParseFeatureDef([]byte(yaml))
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
	shared, err := specparse.ParseSharedTypes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSharedTypes failed: %v", err)
	}
	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}
	if len(shared.Enums) != 2 {
		t.Fatalf("len(enums) = %d, want 2", len(shared.Enums))
	}
}

// --- Integration tests against real YAML files ---

func TestParseStatusFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "status", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}
	if def.Name != "Status" {
		t.Errorf("name = %q, want Status", def.Name)
	}
	if def.ID != 0x02 {
		t.Errorf("id = 0x%02x, want 0x02", def.ID)
	}
}

func TestParseSharedTypesFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "_shared", "1.0.yaml")
	shared, err := specparse.LoadSharedTypes(path)
	if err != nil {
		t.Fatalf("LoadSharedTypes failed: %v", err)
	}
	if shared.Version != "1.0" {
		t.Errorf("version = %q, want 1.0", shared.Version)
	}
	if len(shared.Enums) != 5 {
		t.Fatalf("len(enums) = %d, want 5", len(shared.Enums))
	}
}

func TestParseDeviceInfoFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "device-info", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
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
	if len(def.Attributes) != 13 {
		t.Errorf("len(attributes) = %d, want 13", len(def.Attributes))
	}
	if len(def.Commands) != 1 {
		t.Errorf("len(commands) = %d, want 1", len(def.Commands))
	}
}

func TestParseElectricalFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "electrical", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
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
}

func TestParseMeasurementFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "measurement", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
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
}

func TestParseEnergyControlFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "energy-control", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}
	if def.Name != "EnergyControl" {
		t.Errorf("name = %q, want EnergyControl", def.Name)
	}
	if def.ID != 0x05 {
		t.Errorf("id = 0x%02x, want 0x05", def.ID)
	}
	if len(def.Enums) != 9 {
		t.Errorf("len(enums) = %d, want 9", len(def.Enums))
	}
	if len(def.Commands) != 11 {
		t.Errorf("len(commands) = %d, want 11", len(def.Commands))
	}
}

func TestParseChargingSessionFeature(t *testing.T) {
	path := filepath.Join(docsDir(t), "charging-session", "1.0.yaml")
	def, err := specparse.LoadFeatureDef(path)
	if err != nil {
		t.Fatalf("LoadFeatureDef failed: %v", err)
	}
	if def.Name != "ChargingSession" {
		t.Errorf("name = %q, want ChargingSession", def.Name)
	}
	if def.ID != 0x06 {
		t.Errorf("id = 0x%02x, want 0x06", def.ID)
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
	pv, err := specparse.ParseProtocolVersions([]byte(yamlStr))
	if err != nil {
		t.Fatalf("ParseProtocolVersions failed: %v", err)
	}

	ver, ok := pv.Versions["1.0"]
	if !ok {
		t.Fatal("version 1.0 not found")
	}
	if len(ver.FeatureTypes) != 2 {
		t.Fatalf("len(feature_types) = %d, want 2", len(ver.FeatureTypes))
	}
	if len(ver.EndpointTypes) != 3 {
		t.Fatalf("len(endpoint_types) = %d, want 3", len(ver.EndpointTypes))
	}
}

func TestParseProtocolVersionsFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "protocol-versions.yaml")
	pv, err := specparse.LoadProtocolVersions(path)
	if err != nil {
		t.Fatalf("LoadProtocolVersions failed: %v", err)
	}
	ver, ok := pv.Versions["1.0"]
	if !ok {
		t.Fatal("version 1.0 not found")
	}
	if len(ver.Features) != 10 {
		t.Errorf("len(features) = %d, want 10", len(ver.Features))
	}
	if ver.Shared != "1.0" {
		t.Errorf("shared = %q, want 1.0", ver.Shared)
	}
}

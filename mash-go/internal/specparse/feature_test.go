package specparse

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
	if len(def.Enums) != 1 {
		t.Fatalf("len(enums) = %d, want 1", len(def.Enums))
	}
	if def.Enums[0].Name != "OperatingState" {
		t.Errorf("enum.name = %q, want OperatingState", def.Enums[0].Name)
	}
	if len(def.Attributes) != 4 {
		t.Fatalf("len(attributes) = %d, want 4", len(def.Attributes))
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
	if len(def.Enums) != 9 {
		t.Errorf("len(enums) = %d, want 9", len(def.Enums))
	}
	if len(def.Commands) != 11 {
		t.Errorf("len(commands) = %d, want 11", len(def.Commands))
	}
}

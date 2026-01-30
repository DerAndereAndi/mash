package main

import (
	"strings"
	"testing"
)

func TestDeriveSpecManifest_StatusFeature(t *testing.T) {
	features := []*RawFeatureDef{statusDef()}

	output, err := DeriveSpecManifest(features, "1.0", "MASH Protocol Specification v1.0 – Initial release")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	mustContain(t, output, `version: "1.0"`)
	mustContain(t, output, `description: "MASH Protocol Specification v1.0`)
	mustContain(t, output, "Status:")
	mustContain(t, output, "id: 0x02")
	mustContain(t, output, "revision: 1")

	// Mandatory attributes
	mustContain(t, output, "{ id: 1, name: operatingState }")

	// Optional attributes
	mustContain(t, output, "{ id: 2, name: stateDetail }")
	mustContain(t, output, "{ id: 3, name: faultCode }")
	mustContain(t, output, "{ id: 4, name: faultMessage }")
}

func TestDeriveSpecManifest_MandatoryFeature(t *testing.T) {
	def := &RawFeatureDef{
		Name:      "DeviceInfo",
		ID:        0x01,
		Revision:  1,
		Mandatory: true,
		Attributes: []RawAttributeDef{
			{ID: 1, Name: "deviceId", Type: "string", Access: "readOnly", Mandatory: true},
			{ID: 5, Name: "vendorId", Type: "uint32", Access: "readOnly"},
		},
		Commands: []RawCommandDef{
			{ID: 0x10, Name: "removeZone", Mandatory: true},
		},
	}

	output, err := DeriveSpecManifest([]*RawFeatureDef{def}, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	mustContain(t, output, "mandatory: true")
	mustContain(t, output, "{ id: 1, name: deviceId }")
	mustContain(t, output, "{ id: 5, name: vendorId }")
	mustContain(t, output, "{ id: 0x10, name: removeZone }")
}

func TestDeriveSpecManifest_WithCommands(t *testing.T) {
	def := &RawFeatureDef{
		Name:     "EnergyControl",
		ID:       0x05,
		Revision: 1,
		Attributes: []RawAttributeDef{
			{ID: 1, Name: "deviceType", Type: "uint8", Access: "readOnly", Mandatory: true},
		},
		Commands: []RawCommandDef{
			{ID: 1, Name: "setLimit", Mandatory: true},
			{ID: 2, Name: "clearLimit", Mandatory: true},
			{ID: 3, Name: "setCurrentLimits"},
		},
	}

	output, err := DeriveSpecManifest([]*RawFeatureDef{def}, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	// Mandatory commands
	mustContain(t, output, "{ id: 1, name: setLimit }")
	mustContain(t, output, "{ id: 2, name: clearLimit }")

	// Optional commands
	mustContain(t, output, "{ id: 3, name: setCurrentLimits }")
}

func TestDeriveSpecManifest_NoOptionalAttributes(t *testing.T) {
	def := &RawFeatureDef{
		Name:     "Simple",
		ID:       0x10,
		Revision: 1,
		Attributes: []RawAttributeDef{
			{ID: 1, Name: "value", Type: "uint8", Access: "readOnly", Mandatory: true},
		},
	}

	output, err := DeriveSpecManifest([]*RawFeatureDef{def}, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	mustContain(t, output, "{ id: 1, name: value }")
	// Should not contain "optional:" for attributes since there are none
	if strings.Contains(output, "optional:") {
		t.Error("output should not contain optional section when no optional attributes exist")
	}
}

func TestDeriveSpecManifest_NoCommands(t *testing.T) {
	def := statusDef()

	output, err := DeriveSpecManifest([]*RawFeatureDef{def}, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	// Status has no commands, so no commands section
	if strings.Contains(output, "commands:") {
		t.Error("output should not contain commands section when no commands exist")
	}
}

func TestDeriveSpecManifest_FeatureOrder(t *testing.T) {
	features := []*RawFeatureDef{
		{Name: "Status", ID: 0x02, Revision: 1, Attributes: []RawAttributeDef{{ID: 1, Name: "state", Mandatory: true}}},
		{Name: "DeviceInfo", ID: 0x01, Revision: 1, Mandatory: true, Attributes: []RawAttributeDef{{ID: 1, Name: "id", Mandatory: true}}},
	}

	output, err := DeriveSpecManifest(features, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	// DeviceInfo (id 0x01) should come before Status (id 0x02)
	diIdx := strings.Index(output, "DeviceInfo:")
	sIdx := strings.Index(output, "Status:")
	if diIdx < 0 || sIdx < 0 {
		t.Fatal("expected both DeviceInfo and Status in output")
	}
	if diIdx > sIdx {
		t.Error("DeviceInfo should appear before Status (sorted by feature ID)")
	}
}

func TestDeriveSpecManifest_HexIDs(t *testing.T) {
	def := &RawFeatureDef{
		Name:     "DeviceInfo",
		ID:       0x01,
		Revision: 1,
		Mandatory: true,
		Attributes: []RawAttributeDef{
			{ID: 1, Name: "deviceId", Type: "string", Access: "readOnly", Mandatory: true},
		},
		Commands: []RawCommandDef{
			{ID: 0x10, Name: "removeZone", Mandatory: true},
		},
	}

	output, err := DeriveSpecManifest([]*RawFeatureDef{def}, "1.0", "test")
	if err != nil {
		t.Fatalf("DeriveSpecManifest failed: %v", err)
	}

	// Feature ID should be hex
	mustContain(t, output, "id: 0x01")
	// Command ID 0x10 = 16, but should be formatted as hex in the spec
	mustContain(t, output, "id: 0x10")
}

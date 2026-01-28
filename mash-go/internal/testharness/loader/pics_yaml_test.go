package loader_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestParsePICSYAML_DeviceMetadata tests parsing YAML with device metadata.
func TestParsePICSYAML_DeviceMetadata(t *testing.T) {
	yaml := `
device:
  vendor: "Example Corp"
  product: "Smart Charger Pro"
  model: "SCP-11"
  version: "1.0.0"
items:
  MASH.S.TRANS.SC: true
`
	pf, err := loader.ParsePICS([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse YAML PICS: %v", err)
	}

	if pf.Device.Vendor != "Example Corp" {
		t.Errorf("Device vendor mismatch: expected 'Example Corp', got '%s'", pf.Device.Vendor)
	}
	if pf.Device.Product != "Smart Charger Pro" {
		t.Errorf("Device product mismatch: expected 'Smart Charger Pro', got '%s'", pf.Device.Product)
	}
	if pf.Device.Model != "SCP-11" {
		t.Errorf("Device model mismatch: expected 'SCP-11', got '%s'", pf.Device.Model)
	}
	if pf.Device.Version != "1.0.0" {
		t.Errorf("Device version mismatch: expected '1.0.0', got '%s'", pf.Device.Version)
	}
}

// TestParsePICSYAML_ItemTypes tests parsing items with various value types.
func TestParsePICSYAML_ItemTypes(t *testing.T) {
	yaml := `
items:
  MASH.S.BOOL.TRUE: true
  MASH.S.BOOL.FALSE: false
  MASH.S.INT.SMALL: 3
  MASH.S.INT.LARGE: 32000
  MASH.S.FLOAT: 3.14
  MASH.S.STRING: "some value"
`
	pf, err := loader.ParsePICS([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse YAML PICS: %v", err)
	}

	// Check boolean true
	if v, ok := pf.Items["MASH.S.BOOL.TRUE"]; !ok || v != true {
		t.Error("MASH.S.BOOL.TRUE should be true")
	}

	// Check boolean false
	if v, ok := pf.Items["MASH.S.BOOL.FALSE"]; !ok || v != false {
		t.Errorf("MASH.S.BOOL.FALSE should be false, got %v", pf.Items["MASH.S.BOOL.FALSE"])
	}

	// Check integer
	if v, ok := pf.Items["MASH.S.INT.SMALL"]; !ok || v != 3 {
		t.Errorf("MASH.S.INT.SMALL should be 3, got %v", pf.Items["MASH.S.INT.SMALL"])
	}

	// Check large integer
	if v, ok := pf.Items["MASH.S.INT.LARGE"]; !ok || v != 32000 {
		t.Errorf("MASH.S.INT.LARGE should be 32000, got %v", pf.Items["MASH.S.INT.LARGE"])
	}

	// Check float (stored as string in pkg/pics)
	if _, ok := pf.Items["MASH.S.FLOAT"]; !ok {
		t.Error("MASH.S.FLOAT should exist")
	}

	// Check string
	if v, ok := pf.Items["MASH.S.STRING"]; !ok || v != "some value" {
		t.Errorf("MASH.S.STRING should be 'some value', got %v", pf.Items["MASH.S.STRING"])
	}
}

// TestParsePICSYAML_HierarchicalKeys tests parsing hierarchical PICS keys.
func TestParsePICSYAML_HierarchicalKeys(t *testing.T) {
	yaml := `
items:
  MASH.S.ELEC.A01: true
  MASH.S.ELEC.A02: true
  MASH.C.CTRL.A01: true
  MASH.S.TRANS.TLS13: true
`
	pf, err := loader.ParsePICS([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse YAML PICS: %v", err)
	}

	expectedKeys := []string{
		"MASH.S.ELEC.A01",
		"MASH.S.ELEC.A02",
		"MASH.C.CTRL.A01",
		"MASH.S.TRANS.TLS13",
	}

	for _, key := range expectedKeys {
		if _, exists := pf.Items[key]; !exists {
			t.Errorf("Expected key %s to exist in Items", key)
		}
	}
}

// TestParsePICS_DetectsFormat tests auto-detection of YAML vs key=value format.
func TestParsePICS_DetectsFormat(t *testing.T) {
	// Test key=value format
	// Note: "1" is parsed as boolean true in pkg/pics
	keyValue := `
# Key=value format
MASH.S.TRANS.SC=1
MASH.S.ELEC.PHASES=3
`
	pf1, err := loader.ParsePICS([]byte(keyValue))
	if err != nil {
		t.Fatalf("Failed to parse key=value PICS: %v", err)
	}
	// "1" is parsed as boolean true
	if v, ok := pf1.Items["MASH.S.TRANS.SC"]; !ok || v != true {
		t.Errorf("Key=value format: MASH.S.TRANS.SC should be true, got %v", pf1.Items["MASH.S.TRANS.SC"])
	}
	if v, ok := pf1.Items["MASH.S.ELEC.PHASES"]; !ok || v != 3 {
		t.Errorf("Key=value format: MASH.S.ELEC.PHASES should be 3, got %v", pf1.Items["MASH.S.ELEC.PHASES"])
	}

	// Test YAML format
	yamlFmt := `
items:
  MASH.S.TRANS.SC: true
  MASH.S.ELEC.PHASES: 3
`
	pf2, err := loader.ParsePICS([]byte(yamlFmt))
	if err != nil {
		t.Fatalf("Failed to parse YAML PICS: %v", err)
	}
	if v, ok := pf2.Items["MASH.S.TRANS.SC"]; !ok || v != true {
		t.Error("YAML format: MASH.S.TRANS.SC should be true")
	}
	if v, ok := pf2.Items["MASH.S.ELEC.PHASES"]; !ok || v != 3 {
		t.Errorf("YAML format: MASH.S.ELEC.PHASES should be 3, got %v", pf2.Items["MASH.S.ELEC.PHASES"])
	}
}

// TestParsePICSYAML_InvalidYAML tests error handling for invalid YAML.
func TestParsePICSYAML_InvalidYAML(t *testing.T) {
	// Truly invalid YAML: unclosed bracket
	invalidYAML := `
items:
  MASH.S.TRANS.SC: true
  MASH.S.OTHER: [unclosed bracket
`
	_, err := loader.ParsePICS([]byte(invalidYAML))
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

// TestParsePICSYAML_MissingItems tests handling of YAML without items section.
func TestParsePICSYAML_MissingItems(t *testing.T) {
	// YAML with only device section, no items
	yamlOnlyDevice := `
device:
  vendor: "Test"
`
	pf, err := loader.ParsePICS([]byte(yamlOnlyDevice))
	if err != nil {
		t.Fatalf("Should handle missing items gracefully: %v", err)
	}
	if pf.Items == nil {
		t.Error("Items map should be initialized even if missing in YAML")
	}
	if len(pf.Items) != 0 {
		t.Errorf("Items should be empty, got %d items", len(pf.Items))
	}
}

// TestParsePICSYAML_EmptyFile tests handling of empty YAML content.
func TestParsePICSYAML_EmptyFile(t *testing.T) {
	pf, err := loader.ParsePICS([]byte(""))
	if err != nil {
		t.Fatalf("Should handle empty content gracefully: %v", err)
	}
	if pf.Items == nil {
		t.Error("Items map should be initialized for empty content")
	}
}

// TestParsePICSYAML_CommentsPreserved tests that YAML comments don't cause issues.
func TestParsePICSYAML_CommentsPreserved(t *testing.T) {
	yaml := `
# This is a PICS file for testing
device:
  vendor: "Test Corp"  # inline comment

# Communication section
items:
  # Boolean items
  MASH.S.TRANS.SC: true      # Secure connection
  MASH.S.TRANS.PASE: true    # PASE supported
`
	pf, err := loader.ParsePICS([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to parse YAML with comments: %v", err)
	}
	if pf.Device.Vendor != "Test Corp" {
		t.Errorf("Vendor mismatch, got '%s'", pf.Device.Vendor)
	}
	if v, ok := pf.Items["MASH.S.TRANS.SC"]; !ok || v != true {
		t.Error("MASH.S.TRANS.SC should be true")
	}
}

// TestLoadPICS_YAMLFile tests loading actual YAML PICS file from testdata.
func TestLoadPICS_YAMLFile(t *testing.T) {
	// Load the actual ev-charger.yaml file
	pf, err := loader.LoadPICS("../../../testdata/pics/ev-charger.yaml")
	if err != nil {
		t.Fatalf("Failed to load ev-charger.yaml: %v", err)
	}

	// Check device metadata
	if pf.Device.Vendor != "Example Corp" {
		t.Errorf("Device vendor mismatch: expected 'Example Corp', got '%s'", pf.Device.Vendor)
	}
	if pf.Device.Product != "Smart Charger Pro" {
		t.Errorf("Device product mismatch: expected 'Smart Charger Pro', got '%s'", pf.Device.Product)
	}

	// Check some PICS items (now using MASH.* format)
	if v, ok := pf.Items["MASH.S.TRANS.SC"]; !ok || v != true {
		t.Error("MASH.S.TRANS.SC should be true")
	}
	if v, ok := pf.Items["MASH.S.ELEC.PHASES"]; !ok || v != 3 {
		t.Errorf("MASH.S.ELEC.PHASES should be 3, got %v", pf.Items["MASH.S.ELEC.PHASES"])
	}
	if v, ok := pf.Items["MASH.S.ELEC.MAX_CURRENT"]; !ok || v != 32 {
		t.Errorf("MASH.S.ELEC.MAX_CURRENT should be 32, got %v", pf.Items["MASH.S.ELEC.MAX_CURRENT"])
	}
}

// TestParsePICS_LegacyDFormatRejected tests that D.* legacy format is rejected.
func TestParsePICS_LegacyDFormatRejected(t *testing.T) {
	yaml := `
items:
  D.COMM.SC: true
`
	_, err := loader.ParsePICS([]byte(yaml))
	if err == nil {
		t.Error("Expected error for D.* legacy format, got nil")
	}
}

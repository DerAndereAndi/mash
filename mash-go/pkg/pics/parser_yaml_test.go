package pics

import (
	"testing"
)

func TestParseYAML_Basic(t *testing.T) {
	input := `
items:
  MASH.S: 1
  MASH.S.VERSION: 1
  MASH.S.CTRL: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if pics.Format != FormatYAML {
		t.Errorf("Format = %v, want FormatYAML", pics.Format)
	}

	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}

	if !pics.HasFeature("CTRL") {
		t.Error("expected CTRL feature to be present")
	}
}

func TestParseYAML_DeviceMetadata(t *testing.T) {
	input := `
device:
  vendor: "Example Corp"
  product: "Smart Charger Pro"
  model: "SCP-11"
  version: "1.0.0"
items:
  MASH.S: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if pics.Device == nil {
		t.Fatal("expected Device to be non-nil")
	}

	if pics.Device.Vendor != "Example Corp" {
		t.Errorf("Device.Vendor = %s, want Example Corp", pics.Device.Vendor)
	}
	if pics.Device.Product != "Smart Charger Pro" {
		t.Errorf("Device.Product = %s, want Smart Charger Pro", pics.Device.Product)
	}
	if pics.Device.Model != "SCP-11" {
		t.Errorf("Device.Model = %s, want SCP-11", pics.Device.Model)
	}
	if pics.Device.Version != "1.0.0" {
		t.Errorf("Device.Version = %s, want 1.0.0", pics.Device.Version)
	}
}

func TestParseYAML_NoDeviceMetadata(t *testing.T) {
	input := `
items:
  MASH.S: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if pics.Device != nil {
		t.Errorf("expected Device to be nil, got %+v", pics.Device)
	}
}

func TestParseYAML_ValueTypes(t *testing.T) {
	input := `
items:
  MASH.S: true
  MASH.S.DISABLED: false
  MASH.S.VERSION: 1
  MASH.S.ENDPOINTS: 2
  MASH.S.NAME: "Test Device"
  MASH.S.FACTOR: 0.25
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	// Boolean true
	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be true")
	}

	// Boolean false
	if pics.Has("MASH.S.DISABLED") {
		t.Error("expected MASH.S.DISABLED to be false")
	}

	// Integer
	version := pics.GetInt("MASH.S.VERSION")
	if version != 1 {
		t.Errorf("VERSION = %d, want 1", version)
	}

	endpoints := pics.GetInt("MASH.S.ENDPOINTS")
	if endpoints != 2 {
		t.Errorf("ENDPOINTS = %d, want 2", endpoints)
	}

	// String
	name := pics.GetString("MASH.S.NAME")
	if name != "Test Device" {
		t.Errorf("NAME = %s, want Test Device", name)
	}

	// Float (stored as string)
	factor := pics.GetString("MASH.S.FACTOR")
	if factor != "0.25" {
		t.Errorf("FACTOR = %s, want 0.25", factor)
	}
}

func TestParseYAML_LineNumbers(t *testing.T) {
	input := `# Comment line 1
device:
  vendor: "Test"
items:
  MASH.S: 1
  MASH.S.VERSION: 1
  MASH.S.CTRL: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	// Check that line numbers are tracked
	entry, ok := pics.ByCode["MASH.S"]
	if !ok {
		t.Fatal("expected to find MASH.S")
	}
	if entry.LineNumber == 0 {
		t.Error("expected LineNumber to be set for MASH.S")
	}

	// The exact line number depends on YAML parsing, but it should be > 0
	entry, ok = pics.ByCode["MASH.S.CTRL"]
	if !ok {
		t.Fatal("expected to find MASH.S.CTRL")
	}
	if entry.LineNumber == 0 {
		t.Error("expected LineNumber to be set for MASH.S.CTRL")
	}
}

func TestParseYAML_InvalidSyntax(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid yaml",
			input: "items:\n  bad: [unclosed",
		},
		{
			name:  "tabs instead of spaces",
			input: "items:\n\tMASH.S: 1",
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.parseYAML([]byte(tt.input))
			if err == nil {
				t.Error("expected error for invalid YAML syntax")
			}
		})
	}
}

func TestParseYAML_InvalidCode(t *testing.T) {
	input := `
items:
  INVALID_CODE: 1
`
	parser := NewParser()
	_, err := parser.parseYAML([]byte(input))
	if err == nil {
		t.Error("expected error for invalid PICS code")
	}
}

func TestParseYAML_DeviceCapabilityCodes(t *testing.T) {
	// D.* and C.* codes are device/controller capability shorthand flags.
	input := `
device:
  vendor: "Test Corp"
items:
  D.COMM.SC: true
  D.ZONE.MULTI: true
  C.BIDIR.EXPOSE: true
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pics.ByCode["D.COMM.SC"] == (Entry{}) {
		t.Error("D.COMM.SC should be parsed")
	}
	if pics.ByCode["C.BIDIR.EXPOSE"] == (Entry{}) {
		t.Error("C.BIDIR.EXPOSE should be parsed")
	}
}

func TestParseYAML_AttributesAndCommands(t *testing.T) {
	input := `
items:
  MASH.S: 1
  MASH.S.CTRL: 1
  MASH.S.CTRL.A01: 1
  MASH.S.CTRL.A02: 1
  MASH.S.CTRL.C01.Rsp: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if !pics.HasAttribute("CTRL", "01") {
		t.Error("expected CTRL.A01 to be present")
	}
	if !pics.HasAttribute("CTRL", "02") {
		t.Error("expected CTRL.A02 to be present")
	}
	if !pics.HasCommand("CTRL", "01") {
		t.Error("expected CTRL.C01.Rsp to be present")
	}
}

func TestParseYAML_FeatureFlags(t *testing.T) {
	input := `
items:
  MASH.S: 1
  MASH.S.CTRL: 1
  MASH.S.CTRL.F00: 1
  MASH.S.CTRL.F03: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if !pics.HasFeatureFlag("CTRL", "00") {
		t.Error("expected CTRL.F00 to be present")
	}
	if !pics.HasFeatureFlag("CTRL", "03") {
		t.Error("expected CTRL.F03 to be present")
	}
}

func TestParseYAML_EndpointTypeDeclaration(t *testing.T) {
	input := `
items:
  MASH.S: 1
  MASH.S.E01: EV_CHARGER
  MASH.S.E02: INVERTER
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if pics.EndpointType(1) != "EV_CHARGER" {
		t.Errorf("EndpointType(1) = %s, want EV_CHARGER", pics.EndpointType(1))
	}
	if pics.EndpointType(2) != "INVERTER" {
		t.Errorf("EndpointType(2) = %s, want INVERTER", pics.EndpointType(2))
	}

	ids := pics.EndpointIDs()
	if len(ids) != 2 {
		t.Fatalf("len(EndpointIDs()) = %d, want 2", len(ids))
	}
}

func TestParseYAML_EndpointFeatureCodes(t *testing.T) {
	input := `
items:
  MASH.S: 1
  MASH.S.TRANS: 1
  MASH.S.E01: EV_CHARGER
  MASH.S.E01.CTRL: 1
  MASH.S.E01.MEAS: 1
  MASH.S.E01.CTRL.A01: 1
  MASH.S.E01.MEAS.A01: 1
  MASH.S.E01.CTRL.C01.Rsp: 1
  MASH.S.E01.CTRL.F03: 1
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	// Device-level features
	foundTrans := false
	for _, f := range pics.Features {
		if f == "TRANS" {
			foundTrans = true
			break
		}
	}
	if !foundTrans {
		t.Errorf("expected TRANS in device-level Features, got %v", pics.Features)
	}

	// Endpoint features
	ep := pics.Endpoints[1]
	if ep == nil {
		t.Fatal("expected endpoint 1 to exist")
	}
	if ep.Type != "EV_CHARGER" {
		t.Errorf("endpoint 1 type = %s, want EV_CHARGER", ep.Type)
	}

	// Verify parsed codes
	entry, ok := pics.ByCode["MASH.S.E01.CTRL.A01"]
	if !ok {
		t.Fatal("expected MASH.S.E01.CTRL.A01 in ByCode")
	}
	if entry.Code.EndpointID != 1 {
		t.Errorf("EndpointID = %d, want 1", entry.Code.EndpointID)
	}
	if entry.Code.Feature != "CTRL" {
		t.Errorf("Feature = %s, want CTRL", entry.Code.Feature)
	}

	// Verify command with qualifier
	if !pics.Has("MASH.S.E01.CTRL.C01.Rsp") {
		t.Error("expected MASH.S.E01.CTRL.C01.Rsp to be present")
	}

	// Verify feature flag
	if !pics.Has("MASH.S.E01.CTRL.F03") {
		t.Error("expected MASH.S.E01.CTRL.F03 to be present")
	}

	// Verify CTRL tracked in endpoint features
	found := false
	for _, f := range ep.Features {
		if f == "CTRL" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CTRL in endpoint features, got %v", ep.Features)
	}
}

func TestParseYAML_EmptyItems(t *testing.T) {
	input := `
device:
  vendor: "Test"
items: {}
`
	parser := NewParser()
	pics, err := parser.parseYAML([]byte(input))
	if err != nil {
		t.Fatalf("parseYAML failed: %v", err)
	}

	if len(pics.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(pics.Entries))
	}
}


package version

import (
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// Loading tests
// ---------------------------------------------------------------------------

func TestLoadCurrentSpec(t *testing.T) {
	spec, err := LoadCurrentSpec()
	if err != nil {
		t.Fatalf("LoadCurrentSpec() error: %v", err)
	}
	if spec.Version != "1.0" {
		t.Errorf("Version = %q, want %q", spec.Version, "1.0")
	}
	if spec.Description == "" {
		t.Error("Description is empty")
	}
}

func TestLoadSpec_Valid(t *testing.T) {
	spec, err := LoadSpec("1.0")
	if err != nil {
		t.Fatalf("LoadSpec(1.0) error: %v", err)
	}
	if spec.Version != "1.0" {
		t.Errorf("Version = %q, want %q", spec.Version, "1.0")
	}
}

func TestLoadSpec_NotFound(t *testing.T) {
	_, err := LoadSpec("99.99")
	if err == nil {
		t.Fatal("LoadSpec(99.99) should return error")
	}
}

func TestAvailableSpecs(t *testing.T) {
	versions, err := AvailableSpecs()
	if err != nil {
		t.Fatalf("AvailableSpecs() error: %v", err)
	}
	found := false
	for _, v := range versions {
		if v == "1.0" {
			found = true
		}
	}
	if !found {
		t.Errorf("AvailableSpecs() = %v, want to contain %q", versions, "1.0")
	}
}

// ---------------------------------------------------------------------------
// Content tests -- verify the 1.0 manifest
// ---------------------------------------------------------------------------

func TestSpec10_AllSixFeaturesPresent(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	want := []string{
		"DeviceInfo", "Status", "Electrical",
		"Measurement", "EnergyControl", "ChargingSession",
	}
	for _, name := range want {
		if _, ok := spec.Features[name]; !ok {
			t.Errorf("feature %q missing from spec 1.0", name)
		}
	}
	if len(spec.Features) != len(want) {
		t.Errorf("spec 1.0 has %d features, want %d", len(spec.Features), len(want))
	}
}

func TestSpec10_MandatoryFeatures(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	mandatory := spec.MandatoryFeatures()
	if len(mandatory) != 1 {
		t.Fatalf("MandatoryFeatures() = %v, want exactly [DeviceInfo]", mandatory)
	}
	if mandatory[0] != "DeviceInfo" {
		t.Errorf("MandatoryFeatures()[0] = %q, want %q", mandatory[0], "DeviceInfo")
	}
}

func TestSpec10_DeviceInfoMandatory(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	di, ok := spec.Features["DeviceInfo"]
	if !ok {
		t.Fatal("DeviceInfo feature missing")
	}
	if di.ID != 0x01 {
		t.Errorf("DeviceInfo ID = 0x%02x, want 0x01", di.ID)
	}
	if di.Revision != 1 {
		t.Errorf("DeviceInfo Revision = %d, want 1", di.Revision)
	}
	if !di.Mandatory {
		t.Error("DeviceInfo should be mandatory")
	}
}

func TestSpec10_DeviceInfoMandatoryAttributes(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	di := spec.Features["DeviceInfo"]

	wantIDs := map[uint16]string{
		1:  "deviceId",
		2:  "vendorName",
		3:  "productName",
		4:  "serialNumber",
		10: "softwareVersion",
		12: "specVersion",
		20: "endpoints",
	}

	gotIDs := make(map[uint16]string)
	for _, a := range di.Attributes.Mandatory {
		gotIDs[a.ID] = a.Name
	}

	for id, name := range wantIDs {
		got, ok := gotIDs[id]
		if !ok {
			t.Errorf("mandatory attribute %s (ID %d) missing", name, id)
		} else if got != name {
			t.Errorf("attribute ID %d name = %q, want %q", id, got, name)
		}
	}
}

func TestSpec10_DeviceInfoOptionalAttributes(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	di := spec.Features["DeviceInfo"]

	wantIDs := map[uint16]string{
		5:  "vendorId",
		6:  "productId",
		11: "hardwareVersion",
		30: "location",
		31: "label",
	}

	gotIDs := make(map[uint16]string)
	for _, a := range di.Attributes.Optional {
		gotIDs[a.ID] = a.Name
	}

	for id, name := range wantIDs {
		got, ok := gotIDs[id]
		if !ok {
			t.Errorf("optional attribute %s (ID %d) missing", name, id)
		} else if got != name {
			t.Errorf("attribute ID %d name = %q, want %q", id, got, name)
		}
	}
}

func TestSpec10_DeviceInfoCommands(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	di := spec.Features["DeviceInfo"]

	if len(di.Commands.Mandatory) != 1 {
		t.Fatalf("DeviceInfo mandatory commands = %d, want 1", len(di.Commands.Mandatory))
	}
	cmd := di.Commands.Mandatory[0]
	if cmd.ID != 0x10 {
		t.Errorf("removeZone ID = 0x%02x, want 0x10", cmd.ID)
	}
	if cmd.Name != "removeZone" {
		t.Errorf("removeZone Name = %q, want %q", cmd.Name, "removeZone")
	}
}

func TestSpec10_EnergyControlOptional(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	ec, ok := spec.Features["EnergyControl"]
	if !ok {
		t.Fatal("EnergyControl feature missing")
	}
	if ec.ID != 0x05 {
		t.Errorf("EnergyControl ID = 0x%02x, want 0x05", ec.ID)
	}
	if ec.Mandatory {
		t.Error("EnergyControl should NOT be mandatory")
	}
}

func TestSpec10_EnergyControlMandatoryCommands(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	ec := spec.Features["EnergyControl"]

	wantCmds := map[uint8]string{
		1: "setLimit",
		2: "clearLimit",
	}

	gotCmds := make(map[uint8]string)
	for _, c := range ec.Commands.Mandatory {
		gotCmds[c.ID] = c.Name
	}

	for id, name := range wantCmds {
		got, ok := gotCmds[id]
		if !ok {
			t.Errorf("mandatory command %s (ID %d) missing", name, id)
		} else if got != name {
			t.Errorf("command ID %d name = %q, want %q", id, got, name)
		}
	}
}

func TestSpec10_ChargingSessionMandatoryAttrs(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	cs := spec.Features["ChargingSession"]

	wantIDs := map[uint16]string{
		1: "state",
		2: "sessionId",
	}

	gotIDs := make(map[uint16]string)
	for _, a := range cs.Attributes.Mandatory {
		gotIDs[a.ID] = a.Name
	}

	for id, name := range wantIDs {
		got, ok := gotIDs[id]
		if !ok {
			t.Errorf("mandatory attribute %s (ID %d) missing", name, id)
		} else if got != name {
			t.Errorf("attribute ID %d name = %q, want %q", id, got, name)
		}
	}
}

// ---------------------------------------------------------------------------
// Validation tests
// ---------------------------------------------------------------------------

func TestValidateDevice_AllMandatoryPresent(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	device := DeviceCapabilities{
		SpecVersion: "1.0",
		Features: map[string]FeatureCapabilities{
			"DeviceInfo": {
				Revision:   1,
				Attributes: []uint16{1, 2, 3, 4, 10, 12, 20},
				Commands:   []uint8{0x10},
			},
		},
	}

	result := ValidateDevice(spec, device)
	if !result.Valid {
		t.Errorf("expected valid, got errors: %v", result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}
}

func TestValidateDevice_MissingMandatoryFeature(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	device := DeviceCapabilities{
		SpecVersion: "1.0",
		Features:    map[string]FeatureCapabilities{},
	}

	result := ValidateDevice(spec, device)
	if result.Valid {
		t.Error("expected invalid when DeviceInfo is missing")
	}
	assertContainsSubstring(t, result.Errors, "DeviceInfo")
}

func TestValidateDevice_MissingMandatoryAttribute(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	// Missing specVersion(12) and endpoints(20)
	device := DeviceCapabilities{
		SpecVersion: "1.0",
		Features: map[string]FeatureCapabilities{
			"DeviceInfo": {
				Revision:   1,
				Attributes: []uint16{1, 2, 3, 4, 10},
				Commands:   []uint8{0x10},
			},
		},
	}

	result := ValidateDevice(spec, device)
	if result.Valid {
		t.Error("expected invalid when mandatory attributes missing")
	}
	assertContainsSubstring(t, result.Errors, "specVersion")
	assertContainsSubstring(t, result.Errors, "endpoints")
}

func TestValidateDevice_OptionalFeatureMissingMandatoryAttr(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	device := DeviceCapabilities{
		SpecVersion: "1.0",
		Features: map[string]FeatureCapabilities{
			"DeviceInfo": {
				Revision:   1,
				Attributes: []uint16{1, 2, 3, 4, 10, 12, 20},
				Commands:   []uint8{0x10},
			},
			// Status is present but missing operatingState(1)
			"Status": {
				Revision:   1,
				Attributes: []uint16{},
				Commands:   []uint8{},
			},
		},
	}

	result := ValidateDevice(spec, device)
	if result.Valid {
		t.Error("expected invalid when Status present but missing operatingState")
	}
	assertContainsSubstring(t, result.Errors, "operatingState")
}

func TestValidateDevice_RevisionMismatch(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	device := DeviceCapabilities{
		SpecVersion: "1.0",
		Features: map[string]FeatureCapabilities{
			"DeviceInfo": {
				Revision:   2, // spec says 1
				Attributes: []uint16{1, 2, 3, 4, 10, 12, 20},
				Commands:   []uint8{0x10},
			},
		},
	}

	result := ValidateDevice(spec, device)
	// Revision mismatch is a warning, not an error
	if !result.Valid {
		t.Errorf("revision mismatch should be warning, not error; errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 {
		t.Error("expected warning for revision mismatch")
	}
	assertContainsSubstring(t, result.Warnings, "revision")
}

// ---------------------------------------------------------------------------
// Helper: FeatureByID
// ---------------------------------------------------------------------------

func TestFeatureByID(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")

	name, fs, ok := spec.FeatureByID(0x05)
	if !ok {
		t.Fatal("FeatureByID(0x05) not found")
	}
	if name != "EnergyControl" {
		t.Errorf("name = %q, want %q", name, "EnergyControl")
	}
	if fs.ID != 0x05 {
		t.Errorf("ID = %d, want 5", fs.ID)
	}

	_, _, ok = spec.FeatureByID(0xFF)
	if ok {
		t.Error("FeatureByID(0xFF) should return false")
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func mustLoadSpec(t *testing.T, ver string) *SpecManifest {
	t.Helper()
	spec, err := LoadSpec(ver)
	if err != nil {
		t.Fatalf("LoadSpec(%q) error: %v", ver, err)
	}
	return spec
}

func assertContainsSubstring(t *testing.T, items []string, substr string) {
	t.Helper()
	for _, s := range items {
		if contains(s, substr) {
			return
		}
	}
	t.Errorf("expected an item containing %q in %v", substr, items)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// MandatoryFeatures returns sorted list for deterministic testing
// ---------------------------------------------------------------------------

func TestSpec10_MandatoryFeaturesSorted(t *testing.T) {
	spec := mustLoadSpec(t, "1.0")
	mf := spec.MandatoryFeatures()
	if !sort.StringsAreSorted(mf) {
		t.Errorf("MandatoryFeatures() not sorted: %v", mf)
	}
}

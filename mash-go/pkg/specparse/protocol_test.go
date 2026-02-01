package specparse

import (
	"path/filepath"
	"testing"
)

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
	if ver.FeatureTypes[0].Name != "DeviceInfo" || ver.FeatureTypes[0].ID != 0x01 {
		t.Errorf("feature_types[0] = %+v, want DeviceInfo/0x01", ver.FeatureTypes[0])
	}

	// Endpoint types
	if len(ver.EndpointTypes) != 3 {
		t.Fatalf("len(endpoint_types) = %d, want 3", len(ver.EndpointTypes))
	}
	if ver.EndpointTypes[0].Name != "DEVICE_ROOT" || ver.EndpointTypes[0].ID != 0x00 {
		t.Errorf("endpoint_types[0] = %+v, want DEVICE_ROOT/0x00", ver.EndpointTypes[0])
	}
}

func TestParseProtocolVersions_UseCases(t *testing.T) {
	yamlStr := `
versions:
  "1.0":
    description: "MASH Protocol v1.0"
    features:
      DeviceInfo: "1.0"
    shared: "1.0"
    usecases:
      GPL: { id: 0x01, major: 1, minor: 0 }
      EVC: { id: 0x03, major: 1, minor: 0 }
    use_case_types:
      - { name: GPL, id: 0x01, description: "Grid Power Limitation" }
      - { name: EVC, id: 0x03, description: "EV Charging" }
`
	pv, err := ParseProtocolVersions([]byte(yamlStr))
	if err != nil {
		t.Fatalf("ParseProtocolVersions failed: %v", err)
	}

	ver := pv.Versions["1.0"]

	// UseCases map
	if len(ver.UseCases) != 2 {
		t.Fatalf("len(usecases) = %d, want 2", len(ver.UseCases))
	}
	gpl, ok := ver.UseCases["GPL"]
	if !ok {
		t.Fatal("GPL not found in usecases")
	}
	if gpl.ID != 0x01 || gpl.Major != 1 || gpl.Minor != 0 {
		t.Errorf("GPL = %+v, want {ID:1, Major:1, Minor:0}", gpl)
	}

	// UseCaseTypes
	if len(ver.UseCaseTypes) != 2 {
		t.Fatalf("len(use_case_types) = %d, want 2", len(ver.UseCaseTypes))
	}
	if ver.UseCaseTypes[0].Name != "GPL" || ver.UseCaseTypes[0].ID != 0x01 {
		t.Errorf("use_case_types[0] = %+v, want GPL/0x01", ver.UseCaseTypes[0])
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
	if len(ver.Features) != 9 {
		t.Errorf("len(features) = %d, want 9", len(ver.Features))
	}
	if ver.Shared != "1.0" {
		t.Errorf("shared = %q, want 1.0", ver.Shared)
	}

	// New fields from real file
	if len(ver.UseCases) != 10 {
		t.Errorf("len(usecases) = %d, want 10", len(ver.UseCases))
	}
	if len(ver.UseCaseTypes) != 10 {
		t.Errorf("len(use_case_types) = %d, want 10", len(ver.UseCaseTypes))
	}
}

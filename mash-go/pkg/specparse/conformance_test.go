package specparse

import (
	"path/filepath"
	"testing"
)

func TestParseEndpointConformance(t *testing.T) {
	yaml := `
specVersion: "1.0"
endpointTypes:
  EV_CHARGER:
    Measurement:
      mandatory:
        - acActivePower
      recommended:
        - acCurrentPerPhase
    Electrical:
      mandatory:
        - phaseCount
        - nominalMaxConsumption
`
	ec, err := ParseEndpointConformance([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseEndpointConformance failed: %v", err)
	}

	if ec.SpecVersion != "1.0" {
		t.Errorf("specVersion = %q, want 1.0", ec.SpecVersion)
	}

	evCharger, ok := ec.EndpointTypes["EV_CHARGER"]
	if !ok {
		t.Fatal("EV_CHARGER not found")
	}

	measurement, ok := evCharger["Measurement"]
	if !ok {
		t.Fatal("EV_CHARGER.Measurement not found")
	}
	if len(measurement.Mandatory) != 1 || measurement.Mandatory[0] != "acActivePower" {
		t.Errorf("Measurement.mandatory = %v, want [acActivePower]", measurement.Mandatory)
	}
	if len(measurement.Recommended) != 1 || measurement.Recommended[0] != "acCurrentPerPhase" {
		t.Errorf("Measurement.recommended = %v, want [acCurrentPerPhase]", measurement.Recommended)
	}

	electrical, ok := evCharger["Electrical"]
	if !ok {
		t.Fatal("EV_CHARGER.Electrical not found")
	}
	if len(electrical.Mandatory) != 2 {
		t.Errorf("Electrical.mandatory = %v, want 2 entries", electrical.Mandatory)
	}
}

func TestParseEndpointConformanceFile(t *testing.T) {
	path := filepath.Join(docsDir(t), "endpoint-conformance.yaml")
	ec, err := LoadEndpointConformance(path)
	if err != nil {
		t.Fatalf("LoadEndpointConformance failed: %v", err)
	}

	if ec.SpecVersion != "1.0" {
		t.Errorf("specVersion = %q, want 1.0", ec.SpecVersion)
	}

	// Should have 10 endpoint types (all except DEVICE_ROOT)
	if len(ec.EndpointTypes) != 10 {
		t.Errorf("len(endpointTypes) = %d, want 10", len(ec.EndpointTypes))
	}

	// EV_CHARGER should have Measurement conformance
	evCharger, ok := ec.EndpointTypes["EV_CHARGER"]
	if !ok {
		t.Fatal("EV_CHARGER not found")
	}
	measurement, ok := evCharger["Measurement"]
	if !ok {
		t.Fatal("EV_CHARGER.Measurement not found")
	}
	if len(measurement.Mandatory) != 1 || measurement.Mandatory[0] != "acActivePower" {
		t.Errorf("EV_CHARGER.Measurement.mandatory = %v, want [acActivePower]", measurement.Mandatory)
	}
}

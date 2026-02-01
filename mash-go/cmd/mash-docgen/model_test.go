package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

// docsRoot returns the absolute path to the docs/ directory relative to this test file.
func docsRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs")
}

func TestBuildDocModel_LoadsAllFeatures(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	if len(m.Features) != 9 {
		t.Errorf("len(Features) = %d, want 9", len(m.Features))
	}

	// Should be sorted by ID
	for i := 1; i < len(m.Features); i++ {
		if m.Features[i].ID < m.Features[i-1].ID {
			t.Errorf("features not sorted by ID: %s (0x%02X) before %s (0x%02X)",
				m.Features[i-1].Name, m.Features[i-1].ID,
				m.Features[i].Name, m.Features[i].ID)
		}
	}
}

func TestBuildDocModel_LoadsUseCases(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	if len(m.UseCases) != 11 {
		t.Errorf("len(UseCases) = %d, want 11", len(m.UseCases))
	}

	// Should be sorted by ID
	for i := 1; i < len(m.UseCases); i++ {
		if m.UseCases[i].ID < m.UseCases[i-1].ID {
			t.Errorf("use cases not sorted by ID: %s (0x%02X) before %s (0x%02X)",
				m.UseCases[i-1].Name, m.UseCases[i-1].ID,
				m.UseCases[i].Name, m.UseCases[i].ID)
		}
	}
}

func TestBuildDocModel_LoadsEndpointConformance(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	if m.Conformance == nil {
		t.Fatal("Conformance is nil")
	}

	evCharger, ok := m.Conformance.EndpointTypes["EV_CHARGER"]
	if !ok {
		t.Fatal("EV_CHARGER not found in conformance")
	}

	measurement, ok := evCharger["Measurement"]
	if !ok {
		t.Fatal("EV_CHARGER does not have Measurement conformance")
	}

	if len(measurement.Mandatory) == 0 {
		t.Error("EV_CHARGER Measurement has no mandatory attributes")
	}
}

func TestBuildDocModel_CrossReferences(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	ecRefs, ok := m.FeatureUseCaseRefs["EnergyControl"]
	if !ok {
		t.Fatal("EnergyControl not found in FeatureUseCaseRefs")
	}

	if len(ecRefs) < 5 {
		t.Errorf("EnergyControl referenced by %d use cases, want >= 5", len(ecRefs))
	}
}

func TestBuildDocModel_FeatureByName(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	di, ok := m.FeatureByName["DeviceInfo"]
	if !ok {
		t.Fatal("DeviceInfo not found in FeatureByName")
	}
	if di.ID != 0x01 {
		t.Errorf("DeviceInfo.ID = 0x%02X, want 0x01", di.ID)
	}
}

func TestBuildDocModel_HasTypes(t *testing.T) {
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}

	if len(m.FeatureTypes) != 9 {
		t.Errorf("len(FeatureTypes) = %d, want 9", len(m.FeatureTypes))
	}
	if len(m.EndpointTypes) != 11 {
		t.Errorf("len(EndpointTypes) = %d, want 11", len(m.EndpointTypes))
	}
	if len(m.UseCaseTypes) != 11 {
		t.Errorf("len(UseCaseTypes) = %d, want 11", len(m.UseCaseTypes))
	}
}

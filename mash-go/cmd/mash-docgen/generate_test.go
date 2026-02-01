package main

import (
	"strings"
	"testing"

	usecasePkg "github.com/mash-protocol/mash-go/pkg/usecase"
)

func testModel(t *testing.T) *DocModel {
	t.Helper()
	m, err := BuildDocModel(docsRoot(t), "1.0")
	if err != nil {
		t.Fatalf("BuildDocModel failed: %v", err)
	}
	return m
}

func mustContain(t *testing.T, output, substr string) {
	t.Helper()
	if !strings.Contains(output, substr) {
		t.Errorf("output does not contain %q\nOutput (first 2000 chars):\n%s", substr, truncate(output, 2000))
	}
}

func mustNotContain(t *testing.T, output, substr string) {
	t.Helper()
	if strings.Contains(output, substr) {
		t.Errorf("output should not contain %q", substr)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... (truncated)"
}

// --- Feature page tests ---

func TestGenerateFeaturePage_Header(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["Status"], m)

	mustContain(t, output, "# Status")
	mustContain(t, output, "0x02")
	mustContain(t, output, "**Revision** | 1")
}

func TestGenerateFeaturePage_AttributeTable(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["Status"], m)

	mustContain(t, output, "## Attributes")
	mustContain(t, output, "| ID")
	mustContain(t, output, "| Name")
	mustContain(t, output, "operatingState")
	mustContain(t, output, "stateDetail")
	mustContain(t, output, "readOnly")
}

func TestGenerateFeaturePage_EnumTable(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["Status"], m)

	mustContain(t, output, "## Enums")
	mustContain(t, output, "### OperatingState")
	mustContain(t, output, "0x00")
	mustContain(t, output, "UNKNOWN")
}

func TestGenerateFeaturePage_CommandTable(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["EnergyControl"], m)

	mustContain(t, output, "## Commands")
	mustContain(t, output, "setLimit")
	mustContain(t, output, "Parameters")
}

func TestGenerateFeaturePage_MapAttribute(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["Measurement"], m)

	mustContain(t, output, "map[Phase]int64")
}

func TestGenerateFeaturePage_NoCommands(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["Measurement"], m)

	mustNotContain(t, output, "## Commands")
}

func TestGenerateFeaturePage_UseCaseBacklinks(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["EnergyControl"], m)

	mustContain(t, output, "Referenced by")
	mustContain(t, output, "GPL")
}

func TestGenerateFeaturePage_DesignDocLink(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["EnergyControl"], m)

	mustContain(t, output, "energy-control.md")
}

// --- Use case page tests ---

func findUseCaseByName(m *DocModel, name string) *usecasePkg.RawUseCaseDef {
	for _, uc := range m.UseCases {
		if uc.Name == name {
			return uc
		}
	}
	return nil
}

func TestGenerateUseCasePage_Header(t *testing.T) {
	m := testModel(t)
	uc := findUseCaseByName(m, "GPL")
	if uc == nil {
		t.Fatal("GPL not found")
	}
	output := GenerateUseCasePage(uc, m)

	mustContain(t, output, "# GPL")
	mustContain(t, output, "Grid Power Limitation")
	mustContain(t, output, "0x01")
}

func TestGenerateUseCasePage_EndpointTypes(t *testing.T) {
	m := testModel(t)
	uc := findUseCaseByName(m, "GPL")
	if uc == nil {
		t.Fatal("GPL not found")
	}
	output := GenerateUseCasePage(uc, m)

	mustContain(t, output, "Endpoint Types")
	mustContain(t, output, "EV_CHARGER")
	mustContain(t, output, "INVERTER")
}

func TestGenerateUseCasePage_ScenarioBreakdown(t *testing.T) {
	m := testModel(t)
	uc := findUseCaseByName(m, "GPL")
	if uc == nil {
		t.Fatal("GPL not found")
	}
	output := GenerateUseCasePage(uc, m)

	mustContain(t, output, "## Scenarios")
	mustContain(t, output, "Bit 0: BASE")
	mustContain(t, output, "EnergyControl")
}

func TestGenerateUseCasePage_FeatureRequirements(t *testing.T) {
	m := testModel(t)
	uc := findUseCaseByName(m, "GPL")
	if uc == nil {
		t.Fatal("GPL not found")
	}
	output := GenerateUseCasePage(uc, m)

	mustContain(t, output, "acceptsLimits")
	mustContain(t, output, "setLimit")
}

func TestGenerateFeaturePage_Description(t *testing.T) {
	m := testModel(t)
	output := GenerateFeaturePage(m.FeatureByName["EnergyControl"], m)

	// EnergyControl has a description in its YAML
	if len(output) < 100 {
		t.Errorf("output too short (%d chars), expected substantial content", len(output))
	}
}

// --- Index page tests ---

func TestGenerateFeatureIndexPage(t *testing.T) {
	m := testModel(t)
	output := GenerateFeatureIndexPage(m)

	mustContain(t, output, "# Feature Reference")
	mustContain(t, output, "DeviceInfo")
	mustContain(t, output, "0x01")
	mustContain(t, output, "device-info.md")
}

func TestGenerateUseCaseIndexPage(t *testing.T) {
	m := testModel(t)
	output := GenerateUseCaseIndexPage(m)

	mustContain(t, output, "# Use Case Reference")
	mustContain(t, output, "GPL")
	mustContain(t, output, "Grid Power Limitation")
	mustContain(t, output, "gpl.md")
}

func TestGenerateEndpointIndexPage(t *testing.T) {
	m := testModel(t)
	output := GenerateEndpointIndexPage(m)

	mustContain(t, output, "# Endpoint Types")
	mustContain(t, output, "EV_CHARGER")
	mustContain(t, output, "ev-charger.md")
}

// --- Endpoint page tests ---

func TestGenerateEndpointPage_ConformanceTable(t *testing.T) {
	m := testModel(t)
	output := GenerateEndpointPage("EV_CHARGER", m)

	mustContain(t, output, "# EV_CHARGER")
	mustContain(t, output, "Measurement")
	mustContain(t, output, "acActivePower")
	mustContain(t, output, "Mandatory")
}

func TestGenerateEndpointPage_ApplicableUseCases(t *testing.T) {
	m := testModel(t)
	output := GenerateEndpointPage("EV_CHARGER", m)

	mustContain(t, output, "Use Cases")
	mustContain(t, output, "EVC")
}

func TestGenerateEndpointPage_NoConformance(t *testing.T) {
	m := testModel(t)
	// DEVICE_ROOT has no conformance rules in the YAML
	output := GenerateEndpointPage("DEVICE_ROOT", m)

	mustContain(t, output, "# DEVICE_ROOT")
}

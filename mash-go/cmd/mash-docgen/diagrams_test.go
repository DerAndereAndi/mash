package main

import (
	"testing"
)

// --- State machine diagrams (Step 4.1) ---

func TestControlStateDiagram(t *testing.T) {
	output := ControlStateDiagram()

	mustContain(t, output, "stateDiagram-v2")
	mustContain(t, output, "AUTONOMOUS")
	mustContain(t, output, "CONTROLLED")
	mustContain(t, output, "LIMITED")
	mustContain(t, output, "FAILSAFE")
	mustContain(t, output, "OVERRIDE")

	// Key transitions
	mustContain(t, output, "AUTONOMOUS --> CONTROLLED")
	mustContain(t, output, "CONTROLLED --> LIMITED")
	mustContain(t, output, "LIMITED --> CONTROLLED")
	mustContain(t, output, "CONTROLLED --> FAILSAFE")
	mustContain(t, output, "LIMITED --> FAILSAFE")
	mustContain(t, output, "FAILSAFE --> AUTONOMOUS")
	mustContain(t, output, "LIMITED --> OVERRIDE")

	// Must be fenced for MkDocs
	mustContain(t, output, "```mermaid")
	mustContain(t, output, "```\n")
}

func TestProcessStateDiagram(t *testing.T) {
	output := ProcessStateDiagram()

	mustContain(t, output, "stateDiagram-v2")
	mustContain(t, output, "NONE")
	mustContain(t, output, "AVAILABLE")
	mustContain(t, output, "SCHEDULED")
	mustContain(t, output, "RUNNING")
	mustContain(t, output, "PAUSED")
	mustContain(t, output, "COMPLETED")
	mustContain(t, output, "ABORTED")

	// Key transitions
	mustContain(t, output, "NONE --> AVAILABLE")
	mustContain(t, output, "AVAILABLE --> SCHEDULED")
	mustContain(t, output, "SCHEDULED --> RUNNING")
	mustContain(t, output, "RUNNING --> PAUSED")
	mustContain(t, output, "RUNNING --> COMPLETED")
	mustContain(t, output, "PAUSED --> RUNNING")

	// Must be fenced for MkDocs
	mustContain(t, output, "```mermaid")
}

// --- Device composition diagrams (Step 4.2) ---

func TestDeviceCompositionDiagram(t *testing.T) {
	m := testModel(t)
	output := DeviceCompositionDiagram("EV_CHARGER", m)

	mustContain(t, output, "graph TD")
	mustContain(t, output, "EV_CHARGER")
	mustContain(t, output, "Measurement")
	mustContain(t, output, "acActivePower")
	mustContain(t, output, "```mermaid")
}

func TestDeviceCompositionDiagram_NoConformance(t *testing.T) {
	m := testModel(t)
	output := DeviceCompositionDiagram("DEVICE_ROOT", m)

	// Should still produce a valid diagram with just the endpoint node
	mustContain(t, output, "graph TD")
	mustContain(t, output, "DEVICE_ROOT")
}

// --- Feature cross-reference diagram (Step 4.3) ---

func TestFeatureCrossRefDiagram(t *testing.T) {
	m := testModel(t)
	output := FeatureCrossRefDiagram(m)

	mustContain(t, output, "graph LR")
	mustContain(t, output, "GPL")
	mustContain(t, output, "EnergyControl")
	mustContain(t, output, "```mermaid")
}

// --- Use case scenario map (Step 4.4) ---

func TestScenarioMapDiagram(t *testing.T) {
	m := testModel(t)
	uc := findUseCaseByName(m, "GPL")
	if uc == nil {
		t.Fatal("GPL not found")
	}
	output := ScenarioMapDiagram(uc)

	mustContain(t, output, "graph LR")
	mustContain(t, output, "BASE")
	mustContain(t, output, "EnergyControl")
	mustContain(t, output, "```mermaid")
}

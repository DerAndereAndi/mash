package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEndToEnd_AllPages(t *testing.T) {
	m := testModel(t)
	outputDir := t.TempDir()

	if err := generateAll(m, outputDir); err != nil {
		t.Fatalf("generateAll failed: %v", err)
	}

	// Verify feature pages exist
	expectedFeatures := []string{
		"device-info", "status", "electrical", "measurement",
		"energy-control", "charging-session", "tariff", "signals", "plan",
	}
	for _, slug := range expectedFeatures {
		path := filepath.Join(outputDir, "features", slug+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected feature file %s: %v", slug, err)
			continue
		}
		if !strings.Contains(string(data), "## Attributes") {
			t.Errorf("%s missing ## Attributes section", slug)
		}
	}

	// Verify endpoint pages exist
	expectedEndpoints := []string{
		"device-root", "grid-connection", "inverter", "pv-string",
		"battery", "ev-charger", "heat-pump", "water-heater",
		"hvac", "appliance", "sub-meter",
	}
	for _, slug := range expectedEndpoints {
		path := filepath.Join(outputDir, "endpoints", slug+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected endpoint file %s not created", slug)
		}
	}

	// Verify index pages exist
	for _, indexPath := range []string{
		filepath.Join(outputDir, "features", "index.md"),
		filepath.Join(outputDir, "usecases", "index.md"),
		filepath.Join(outputDir, "endpoints", "index.md"),
		filepath.Join(outputDir, "index.md"),
	} {
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			t.Errorf("expected index file %s not created", indexPath)
		}
	}

	// Verify use case pages exist
	expectedUseCases := []string{
		"gpl", "mpd", "evc", "cob", "floa",
		"itpcm", "ohpcf", "podf", "poen", "tout",
	}
	for _, slug := range expectedUseCases {
		path := filepath.Join(outputDir, "usecases", slug+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected use case file %s: %v", slug, err)
			continue
		}
		if !strings.Contains(string(data), "## Scenarios") {
			t.Errorf("%s missing ## Scenarios section", slug)
		}
	}
}

func TestEndToEnd_FeaturePages(t *testing.T) {
	m := testModel(t)
	outputDir := t.TempDir()

	if err := generateAllFeaturePages(m, outputDir); err != nil {
		t.Fatalf("generateAllFeaturePages failed: %v", err)
	}

	expectedFeatures := []string{
		"device-info", "status", "electrical", "measurement",
		"energy-control", "charging-session", "tariff", "signals", "plan",
	}

	for _, slug := range expectedFeatures {
		path := filepath.Join(outputDir, "features", slug+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("expected file %s: %v", path, err)
			continue
		}
		content := string(data)
		if !strings.Contains(content, "## Attributes") {
			t.Errorf("%s missing ## Attributes section", slug)
		}
	}
}

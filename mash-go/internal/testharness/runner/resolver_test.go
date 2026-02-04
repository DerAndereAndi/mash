package runner_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/runner"
)

// TestResolveFeatureName tests feature name to ID resolution.
func TestResolveFeatureName(t *testing.T) {
	tests := []struct {
		name     string
		expected uint8
	}{
		{"DeviceInfo", 0x01},
		{"Status", 0x02},
		{"Electrical", 0x03},
		{"Measurement", 0x04},
		{"EnergyControl", 0x05},
		{"ChargingSession", 0x06},
		{"Tariff", 0x07},
		{"Signals", 0x08},
		{"Plan", 0x09},
	}

	resolver := runner.NewResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := resolver.ResolveFeature(tt.name)
			if err != nil {
				t.Fatalf("ResolveFeature(%s) returned error: %v", tt.name, err)
			}
			if id != tt.expected {
				t.Errorf("ResolveFeature(%s) = 0x%02x, want 0x%02x", tt.name, id, tt.expected)
			}
		})
	}
}

// TestResolveFeatureNumeric tests numeric passthrough.
func TestResolveFeatureNumeric(t *testing.T) {
	resolver := runner.NewResolver()

	tests := []struct {
		input    interface{}
		expected uint8
	}{
		{float64(1), 0x01},   // YAML parses numbers as float64
		{float64(4), 0x04},
		{float64(255), 0xFF},
		{int(3), 0x03},       // Just in case
		{uint8(5), 0x05},
	}

	for _, tt := range tests {
		id, err := resolver.ResolveFeature(tt.input)
		if err != nil {
			t.Errorf("ResolveFeature(%v) returned error: %v", tt.input, err)
		}
		if id != tt.expected {
			t.Errorf("ResolveFeature(%v) = 0x%02x, want 0x%02x", tt.input, id, tt.expected)
		}
	}
}

// TestResolveFeatureUnknown tests error handling for unknown features.
func TestResolveFeatureUnknown(t *testing.T) {
	resolver := runner.NewResolver()

	_, err := resolver.ResolveFeature("UnknownFeature")
	if err == nil {
		t.Error("ResolveFeature(UnknownFeature) should return error")
	}
}

// TestResolveFeatureCaseInsensitive tests case-insensitive feature names.
func TestResolveFeatureCaseInsensitive(t *testing.T) {
	resolver := runner.NewResolver()

	cases := []string{
		"measurement",
		"MEASUREMENT",
		"Measurement",
		"MeAsUrEmEnT",
	}

	for _, name := range cases {
		id, err := resolver.ResolveFeature(name)
		if err != nil {
			t.Errorf("ResolveFeature(%s) returned error: %v", name, err)
		}
		if id != 0x04 {
			t.Errorf("ResolveFeature(%s) = 0x%02x, want 0x04", name, id)
		}
	}
}

// TestResolveAttributeMeasurement tests Measurement attribute resolution.
func TestResolveAttributeMeasurement(t *testing.T) {
	resolver := runner.NewResolver()

	tests := []struct {
		name     string
		expected uint16
	}{
		{"acActivePower", 1},   // MeasurementAttrACActivePower
		{"stateOfCharge", 50},  // MeasurementAttrStateOfCharge
		{"dcPower", 40},        // MeasurementAttrDCPower
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := resolver.ResolveAttribute("Measurement", tt.name)
			if err != nil {
				t.Fatalf("ResolveAttribute(Measurement, %s) returned error: %v", tt.name, err)
			}
			if id != tt.expected {
				t.Errorf("ResolveAttribute(Measurement, %s) = 0x%04x, want 0x%04x", tt.name, id, tt.expected)
			}
		})
	}
}

// TestResolveAttributeNumeric tests numeric attribute passthrough.
func TestResolveAttributeNumeric(t *testing.T) {
	resolver := runner.NewResolver()

	tests := []struct {
		input    interface{}
		expected uint16
	}{
		{float64(1), 0x0001},
		{float64(0x32), 0x0032},
		{float64(65535), 0xFFFF},
		{int(100), 0x0064},
	}

	for _, tt := range tests {
		id, err := resolver.ResolveAttribute("Measurement", tt.input)
		if err != nil {
			t.Errorf("ResolveAttribute(Measurement, %v) returned error: %v", tt.input, err)
		}
		if id != tt.expected {
			t.Errorf("ResolveAttribute(Measurement, %v) = 0x%04x, want 0x%04x", tt.input, id, tt.expected)
		}
	}
}

// TestResolveAttributeUnknown tests error handling for unknown attributes.
func TestResolveAttributeUnknown(t *testing.T) {
	resolver := runner.NewResolver()

	_, err := resolver.ResolveAttribute("Measurement", "unknownAttribute")
	if err == nil {
		t.Error("ResolveAttribute(Measurement, unknownAttribute) should return error")
	}
}

// TestResolveAttributeCaseInsensitive tests case-insensitive attribute names.
func TestResolveAttributeCaseInsensitive(t *testing.T) {
	resolver := runner.NewResolver()

	cases := []string{
		"acactivepower",
		"ACACTIVEPOWER",
		"acActivePower",
		"AcActivePower",
	}

	for _, name := range cases {
		id, err := resolver.ResolveAttribute("Measurement", name)
		if err != nil {
			t.Errorf("ResolveAttribute(Measurement, %s) returned error: %v", name, err)
		}
		if id != 0x0001 {
			t.Errorf("ResolveAttribute(Measurement, %s) = 0x%04x, want 0x0001", name, id)
		}
	}
}

// TestResolveEndpointName tests endpoint name to ID resolution.
func TestResolveEndpointName(t *testing.T) {
	tests := []struct {
		name     string
		expected uint8
	}{
		{"DeviceRoot", 0x00},
		{"EVCharger", 0x05},
		{"Battery", 0x04},
	}

	resolver := runner.NewResolver()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := resolver.ResolveEndpoint(tt.name)
			if err != nil {
				t.Fatalf("ResolveEndpoint(%s) returned error: %v", tt.name, err)
			}
			if id != tt.expected {
				t.Errorf("ResolveEndpoint(%s) = 0x%02x, want 0x%02x", tt.name, id, tt.expected)
			}
		})
	}
}

// TestResolveEndpointNumeric tests numeric endpoint passthrough.
func TestResolveEndpointNumeric(t *testing.T) {
	resolver := runner.NewResolver()

	id, err := resolver.ResolveEndpoint(float64(1))
	if err != nil {
		t.Fatalf("ResolveEndpoint(1) returned error: %v", err)
	}
	if id != 1 {
		t.Errorf("ResolveEndpoint(1) = %d, want 1", id)
	}
}

// TestResolveCommand tests command name to ID resolution.
func TestResolveCommand(t *testing.T) {
	resolver := runner.NewResolver()

	tests := []struct {
		name    string
		feature interface{}
		command interface{}
		wantID  uint8
		wantErr bool
	}{
		{"Tariff SetTariff", "Tariff", "SetTariff", 1, false},
		{"Signals ClearSignals", "Signals", "ClearSignals", 4, false},
		{"EnergyControl Stop", "EnergyControl", "Stop", 11, false},
		{"numeric feature and command", float64(0x08), float64(1), 1, false},
		{"uint8 command passthrough", "Tariff", uint8(1), 1, false},
		{"unknown command", "Tariff", "NonExistent", 0, true},
		{"unknown feature", "UnknownFeature", "SetTariff", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := resolver.ResolveCommand(tt.feature, tt.command)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveCommand(%v, %v) error = %v, wantErr %v", tt.feature, tt.command, err, tt.wantErr)
			}
			if !tt.wantErr && id != tt.wantID {
				t.Errorf("ResolveCommand(%v, %v) = %d, want %d", tt.feature, tt.command, id, tt.wantID)
			}
		})
	}
}

// TestResolveGlobalAttributes tests global attributes available on all features.
func TestResolveGlobalAttributes(t *testing.T) {
	resolver := runner.NewResolver()

	features := []string{"DeviceInfo", "Measurement", "Electrical", "Status", "EnergyControl"}
	globalAttrs := []struct {
		name     string
		expected uint16
	}{
		{"featureMap", 0xFFFC},
		{"attributeList", 0xFFFB},
	}

	for _, feature := range features {
		for _, attr := range globalAttrs {
			id, err := resolver.ResolveAttribute(feature, attr.name)
			if err != nil {
				t.Errorf("ResolveAttribute(%s, %s) returned error: %v", feature, attr.name, err)
				continue
			}
			if id != attr.expected {
				t.Errorf("ResolveAttribute(%s, %s) = 0x%04x, want 0x%04x", feature, attr.name, id, attr.expected)
			}
		}
	}
}

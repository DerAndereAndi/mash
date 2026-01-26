package inspect

import (
	"strings"
	"testing"
)

func TestFormatValue(t *testing.T) {
	f := &Formatter{}

	tests := []struct {
		name     string
		value    any
		unit     string
		expected string
	}{
		{
			name:     "int64 power in mW",
			value:    int64(5000000),
			unit:     "mW",
			expected: "5000000 mW (5.0 kW)",
		},
		{
			name:     "int64 negative power (production)",
			value:    int64(-7500000),
			unit:     "mW",
			expected: "-7500000 mW (-7.5 kW)",
		},
		{
			name:     "int64 current in mA",
			value:    int64(32000),
			unit:     "mA",
			expected: "32000 mA (32.0 A)",
		},
		{
			name:     "int64 energy in mWh",
			value:    int64(10000000000),
			unit:     "mWh",
			expected: "10000000000 mWh (10000.0 kWh)",
		},
		{
			name:     "uint16 voltage",
			value:    uint16(230),
			unit:     "V",
			expected: "230 V",
		},
		{
			name:     "uint8 frequency",
			value:    uint8(50),
			unit:     "Hz",
			expected: "50 Hz",
		},
		{
			name:     "bool true",
			value:    true,
			unit:     "",
			expected: "true",
		},
		{
			name:     "bool false",
			value:    false,
			unit:     "",
			expected: "false",
		},
		{
			name:     "string",
			value:    "EVSE-123",
			unit:     "",
			expected: "\"EVSE-123\"",
		},
		{
			name:     "nil",
			value:    nil,
			unit:     "",
			expected: "null",
		},
		{
			name:     "int64 no unit",
			value:    int64(42),
			unit:     "",
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.FormatValue(tt.value, tt.unit)
			if got != tt.expected {
				t.Errorf("FormatValue(%v, %q) = %q, want %q", tt.value, tt.unit, got, tt.expected)
			}
		})
	}
}

func TestFormatPowerHumanReadable(t *testing.T) {
	tests := []struct {
		mW       int64
		expected string
	}{
		{0, "0 W"},
		{500, "0.5 W"},
		{1000, "1.0 W"},
		{1500000, "1.5 kW"},
		{22000000, "22.0 kW"},
		{-5000000, "-5.0 kW"},
		{1500000000, "1500.0 kW"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatPowerHumanReadable(tt.mW)
			if got != tt.expected {
				t.Errorf("FormatPowerHumanReadable(%d) = %q, want %q", tt.mW, got, tt.expected)
			}
		})
	}
}

func TestFormatOperatingState(t *testing.T) {
	tests := []struct {
		state    uint8
		expected string
	}{
		{0, "UNKNOWN"},
		{1, "OFFLINE"},
		{2, "STANDBY"},
		{3, "STARTING"},
		{4, "RUNNING"},
		{5, "PAUSED"},
		{6, "SHUTTING_DOWN"},
		{7, "FAULT"},
		{8, "MAINTENANCE"},
		{99, "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatOperatingState(tt.state)
			if got != tt.expected {
				t.Errorf("FormatOperatingState(%d) = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

func TestFormatControlState(t *testing.T) {
	tests := []struct {
		state    uint8
		expected string
	}{
		{0, "AUTONOMOUS"},
		{1, "CONTROLLED"},
		{2, "LIMITED"},
		{3, "FAILSAFE"},
		{4, "OVERRIDE"},
		{99, "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatControlState(tt.state)
			if got != tt.expected {
				t.Errorf("FormatControlState(%d) = %q, want %q", tt.state, got, tt.expected)
			}
		})
	}
}

func TestFormatDirection(t *testing.T) {
	tests := []struct {
		dir      uint8
		expected string
	}{
		{0, "CONSUMPTION"},
		{1, "PRODUCTION"},
		{2, "BIDIRECTIONAL"},
		{99, "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatDirection(tt.dir)
			if got != tt.expected {
				t.Errorf("FormatDirection(%d) = %q, want %q", tt.dir, got, tt.expected)
			}
		})
	}
}

func TestFormatEndpointType(t *testing.T) {
	tests := []struct {
		epType   uint8
		expected string
	}{
		{0, "DEVICE_ROOT"},
		{1, "GRID_CONNECTION"},
		{2, "INVERTER"},
		{3, "PV_STRING"},
		{4, "BATTERY"},
		{5, "EV_CHARGER"},
		{6, "HEAT_PUMP"},
		{7, "WATER_HEATER"},
		{8, "HVAC"},
		{9, "APPLIANCE"},
		{10, "SUB_METER"},
		{255, "UNKNOWN"}, // Model returns "UNKNOWN" for unknown values
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatEndpointType(tt.epType)
			if got != tt.expected {
				t.Errorf("FormatEndpointType(%d) = %q, want %q", tt.epType, got, tt.expected)
			}
		})
	}
}

func TestFormatFeatureType(t *testing.T) {
	tests := []struct {
		featType uint8
		expected string
	}{
		{1, "DeviceInfo"},    // FeatureDeviceInfo = 0x0001
		{2, "Status"},        // FeatureStatus = 0x0002
		{3, "Electrical"},    // FeatureElectrical = 0x0003
		{4, "Measurement"},   // FeatureMeasurement = 0x0004
		{5, "EnergyControl"}, // FeatureEnergyControl = 0x0005
		{255, "Unknown"},     // Model returns "Unknown" for unknown values
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatFeatureType(tt.featType)
			if got != tt.expected {
				t.Errorf("FormatFeatureType(%d) = %q, want %q", tt.featType, got, tt.expected)
			}
		})
	}
}

func TestFormatterIndent(t *testing.T) {
	f := &Formatter{}

	got := f.Indent(2, "hello")
	if !strings.HasPrefix(got, "    ") {
		t.Errorf("Indent should add 4 spaces at depth 2, got %q", got)
	}
	if !strings.HasSuffix(got, "hello") {
		t.Errorf("Indent should preserve content, got %q", got)
	}
}

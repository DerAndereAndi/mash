package specparse

import "testing"

func TestFeatureDirName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"DeviceInfo", "device-info"},
		{"Status", "status"},
		{"EnergyControl", "energy-control"},
		{"ChargingSession", "charging-session"},
		{"Electrical", "electrical"},
		{"Measurement", "measurement"},
	}
	for _, tt := range tests {
		got := FeatureDirName(tt.in)
		if got != tt.want {
			t.Errorf("FeatureDirName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

package inspect

import (
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Path
		wantErr bool
	}{
		{
			name:  "full path with numeric IDs",
			input: "1/2/3",
			want: &Path{
				EndpointID:  1,
				FeatureID:   2,
				AttributeID: 3,
			},
		},
		{
			name:  "endpoint only",
			input: "1",
			want: &Path{
				EndpointID: 1,
				IsPartial:  true,
			},
		},
		{
			name:  "endpoint and feature",
			input: "1/2",
			want: &Path{
				EndpointID: 1,
				FeatureID:  2,
				IsPartial:  true,
			},
		},
		{
			name:  "with device ID prefix",
			input: "evse-1234/1/2/3",
			want: &Path{
				DeviceID:    "evse-1234",
				EndpointID:  1,
				FeatureID:   2,
				AttributeID: 3,
			},
		},
		{
			name:  "command path",
			input: "1/3/cmd/1",
			want: &Path{
				EndpointID: 1,
				FeatureID:  3,
				CommandID:  1,
				IsCommand:  true,
			},
		},
		{
			name:    "empty path",
			input:   "",
			wantErr: true,
		},
		{
			name:  "hex attribute ID",
			input: "1/2/0xFFFC",
			want: &Path{
				EndpointID:  1,
				FeatureID:   2,
				AttributeID: 0xFFFC,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.DeviceID != tt.want.DeviceID {
				t.Errorf("DeviceID = %q, want %q", got.DeviceID, tt.want.DeviceID)
			}
			if got.EndpointID != tt.want.EndpointID {
				t.Errorf("EndpointID = %d, want %d", got.EndpointID, tt.want.EndpointID)
			}
			if got.FeatureID != tt.want.FeatureID {
				t.Errorf("FeatureID = %d, want %d", got.FeatureID, tt.want.FeatureID)
			}
			if got.AttributeID != tt.want.AttributeID {
				t.Errorf("AttributeID = %d, want %d", got.AttributeID, tt.want.AttributeID)
			}
			if got.CommandID != tt.want.CommandID {
				t.Errorf("CommandID = %d, want %d", got.CommandID, tt.want.CommandID)
			}
			if got.IsCommand != tt.want.IsCommand {
				t.Errorf("IsCommand = %v, want %v", got.IsCommand, tt.want.IsCommand)
			}
			if got.IsPartial != tt.want.IsPartial {
				t.Errorf("IsPartial = %v, want %v", got.IsPartial, tt.want.IsPartial)
			}
		})
	}
}

func TestPathString(t *testing.T) {
	tests := []struct {
		name string
		path *Path
		want string
	}{
		{
			name: "full local path",
			path: &Path{
				EndpointID:  1,
				FeatureID:   2,
				AttributeID: 3,
			},
			want: "1/2/3",
		},
		{
			name: "with device ID",
			path: &Path{
				DeviceID:    "evse-1234",
				EndpointID:  1,
				FeatureID:   2,
				AttributeID: 20,
			},
			want: "evse-1234/1/2/20",
		},
		{
			name: "partial endpoint only",
			path: &Path{
				EndpointID: 1,
				IsPartial:  true,
			},
			want: "1",
		},
		{
			name: "partial endpoint and feature",
			path: &Path{
				EndpointID: 1,
				FeatureID:  2,
				IsPartial:  true,
			},
			want: "1/2",
		},
		{
			name: "command path",
			path: &Path{
				EndpointID: 1,
				FeatureID:  3,
				CommandID:  1,
				IsCommand:  true,
			},
			want: "1/3/cmd/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.path.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePathWithNames(t *testing.T) {
	// Initialize name tables before running these tests
	initNameTables()

	tests := []struct {
		name    string
		input   string
		want    *Path
		wantErr bool
	}{
		{
			name:  "endpoint name: deviceRoot",
			input: "deviceRoot/deviceInfo/1",
			want: &Path{
				EndpointID:  0,
				FeatureID:   1, // FeatureDeviceInfo
				AttributeID: 1,
			},
		},
		{
			name:  "feature name: measurement",
			input: "1/measurement/1",
			want: &Path{
				EndpointID:  1,
				FeatureID:   4, // FeatureMeasurement
				AttributeID: 1,
			},
		},
		{
			name:  "attribute name: acActivePower",
			input: "1/measurement/acActivePower",
			want: &Path{
				EndpointID:  1,
				FeatureID:   4, // FeatureMeasurement
				AttributeID: 1, // MeasurementAttrACActivePower
			},
		},
		{
			name:  "all names: evCharger/energyControl/effectiveConsumptionLimit",
			input: "evCharger/energyControl/effectiveConsumptionLimit",
			want: &Path{
				EndpointID:  5, // EndpointEVCharger
				FeatureID:   5, // FeatureEnergyControl
				AttributeID: 20,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.EndpointID != tt.want.EndpointID {
				t.Errorf("EndpointID = %d, want %d", got.EndpointID, tt.want.EndpointID)
			}
			if got.FeatureID != tt.want.FeatureID {
				t.Errorf("FeatureID = %d, want %d", got.FeatureID, tt.want.FeatureID)
			}
			if got.AttributeID != tt.want.AttributeID {
				t.Errorf("AttributeID = %d, want %d", got.AttributeID, tt.want.AttributeID)
			}
		})
	}
}

func TestIsValidPath(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1/2/3", true},
		{"1/2", true},
		{"1", true},
		{"", false},
		{"deviceRoot", true},       // Valid endpoint name
		{"abc", false},             // Unknown endpoint, no path parts
		{"abc/1/2/3", true},        // Device ID with valid path
		{"/1/2/3", false},
		{"1//2", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParsePath(tt.input)
			valid := err == nil
			if valid != tt.valid {
				t.Errorf("IsValid(%q) = %v, want %v (err=%v)", tt.input, valid, tt.valid, err)
			}
		})
	}
}

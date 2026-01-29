package discovery

import (
	"testing"
	"time"
)

// =============================================================================
// Commissioning Window Timing Constants (DEC-048)
// =============================================================================
// These tests verify the commissioning window duration constants align with
// Matter specification 5.4.2.3.1 and DEC-048 requirements.

func TestDefaultCommissioningWindowDuration(t *testing.T) {
	// DEC-048: Default should be 15 minutes (aligned with Matter)
	want := 15 * time.Minute
	if CommissioningWindowDuration != want {
		t.Errorf("CommissioningWindowDuration = %v, want %v", CommissioningWindowDuration, want)
	}
}

func TestMinCommissioningWindowDuration(t *testing.T) {
	// DEC-048: Minimum should be 3 minutes (aligned with Matter)
	want := 3 * time.Minute
	if MinCommissioningWindowDuration != want {
		t.Errorf("MinCommissioningWindowDuration = %v, want %v", MinCommissioningWindowDuration, want)
	}
}

func TestMaxCommissioningWindowDuration(t *testing.T) {
	// DEC-048: Maximum should be 3 hours (MASH-specific; Matter uses 15 min)
	// MASH allows longer for professional installer scenarios
	want := 3 * time.Hour
	if MaxCommissioningWindowDuration != want {
		t.Errorf("MaxCommissioningWindowDuration = %v, want %v", MaxCommissioningWindowDuration, want)
	}
}

func TestCommissioningWindowDurationWithinBounds(t *testing.T) {
	// Verify default is within min/max bounds
	if CommissioningWindowDuration < MinCommissioningWindowDuration {
		t.Errorf("CommissioningWindowDuration (%v) < MinCommissioningWindowDuration (%v)",
			CommissioningWindowDuration, MinCommissioningWindowDuration)
	}
	if CommissioningWindowDuration > MaxCommissioningWindowDuration {
		t.Errorf("CommissioningWindowDuration (%v) > MaxCommissioningWindowDuration (%v)",
			CommissioningWindowDuration, MaxCommissioningWindowDuration)
	}
}

// PairingRequestInfo validation tests

func TestPairingRequestInfoValidate(t *testing.T) {
	tests := []struct {
		name    string
		info    PairingRequestInfo
		wantErr bool
	}{
		{
			name: "ValidBasic",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "A1B2C3D4E5F6A7B8",
				Host:          "controller.local",
			},
			wantErr: false,
		},
		{
			name: "ValidWithZoneName",
			info: PairingRequestInfo{
				Discriminator: 0,
				ZoneID:        "1234567890abcdef",
				ZoneName:      "Home Energy",
				Host:          "ems.local",
			},
			wantErr: false,
		},
		{
			name: "ValidMaxDiscriminator",
			info: PairingRequestInfo{
				Discriminator: 4095,
				ZoneID:        "FEDCBA0987654321",
				Host:          "controller.local",
			},
			wantErr: false,
		},
		{
			name: "InvalidDiscriminatorTooHigh",
			info: PairingRequestInfo{
				Discriminator: 4096,
				ZoneID:        "A1B2C3D4E5F6A7B8",
				Host:          "controller.local",
			},
			wantErr: true,
		},
		{
			name: "InvalidZoneIDTooShort",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "A1B2C3D4",
				Host:          "controller.local",
			},
			wantErr: true,
		},
		{
			name: "InvalidZoneIDTooLong",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "A1B2C3D4E5F6A7B8EXTRA",
				Host:          "controller.local",
			},
			wantErr: true,
		},
		{
			name: "InvalidZoneIDNotHex",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "GHIJKLMNOPQRSTUV",
				Host:          "controller.local",
			},
			wantErr: true,
		},
		{
			name: "InvalidEmptyZoneID",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "",
				Host:          "controller.local",
			},
			wantErr: true,
		},
		{
			name: "InvalidEmptyHost",
			info: PairingRequestInfo{
				Discriminator: 1234,
				ZoneID:        "A1B2C3D4E5F6A7B8",
				Host:          "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.info.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// PairingRequestTXT encoding tests

func TestPairingRequestTXTRoundtrip(t *testing.T) {
	info := &PairingRequestInfo{
		Discriminator: 2048,
		ZoneID:        "A1B2C3D4E5F6A7B8",
		ZoneName:      "Home Energy",
		Host:          "controller.local",
	}

	txt := EncodePairingRequestTXT(info)

	// Verify TXT records
	if txt[TXTKeyDiscriminator] != "2048" {
		t.Errorf("D = %q, want \"2048\"", txt[TXTKeyDiscriminator])
	}
	if txt[TXTKeyZoneID] != "A1B2C3D4E5F6A7B8" {
		t.Errorf("ZI = %q, want \"A1B2C3D4E5F6A7B8\"", txt[TXTKeyZoneID])
	}
	if txt[TXTKeyZoneName] != "Home Energy" {
		t.Errorf("ZN = %q, want \"Home Energy\"", txt[TXTKeyZoneName])
	}

	// Decode and verify
	decoded, err := DecodePairingRequestTXT(txt)
	if err != nil {
		t.Fatalf("DecodePairingRequestTXT() error = %v", err)
	}

	if decoded.Discriminator != info.Discriminator {
		t.Errorf("Discriminator = %d, want %d", decoded.Discriminator, info.Discriminator)
	}
	if decoded.ZoneID != info.ZoneID {
		t.Errorf("ZoneID = %q, want %q", decoded.ZoneID, info.ZoneID)
	}
	if decoded.ZoneName != info.ZoneName {
		t.Errorf("ZoneName = %q, want %q", decoded.ZoneName, info.ZoneName)
	}
}

func TestPairingRequestTXTWithoutOptional(t *testing.T) {
	info := &PairingRequestInfo{
		Discriminator: 0,
		ZoneID:        "1234567890ABCDEF",
		Host:          "controller.local",
	}

	txt := EncodePairingRequestTXT(info)

	// ZoneName should not be present
	if _, ok := txt[TXTKeyZoneName]; ok {
		t.Error("ZN should not be present when ZoneName is empty")
	}

	decoded, err := DecodePairingRequestTXT(txt)
	if err != nil {
		t.Fatalf("DecodePairingRequestTXT() error = %v", err)
	}

	if decoded.ZoneName != "" {
		t.Errorf("ZoneName = %q, want empty string", decoded.ZoneName)
	}
}

func TestDecodePairingRequestTXTMissingRequired(t *testing.T) {
	tests := []struct {
		name string
		txt  TXTRecordMap
	}{
		{"MissingD", TXTRecordMap{"ZI": "A1B2C3D4E5F6A7B8"}},
		{"MissingZI", TXTRecordMap{"D": "1234"}},
		{"ShortZI", TXTRecordMap{"D": "1234", "ZI": "A1B2"}},
		{"InvalidHexZI", TXTRecordMap{"D": "1234", "ZI": "GHIJKLMNOPQRSTUV"}},
		{"DiscriminatorTooHigh", TXTRecordMap{"D": "5000", "ZI": "A1B2C3D4E5F6A7B8"}},
		{"DiscriminatorNonNumeric", TXTRecordMap{"D": "abc", "ZI": "A1B2C3D4E5F6A7B8"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodePairingRequestTXT(tt.txt)
			if err == nil {
				t.Error("DecodePairingRequestTXT() should fail with missing/invalid required field")
			}
		})
	}
}

// PairingRequestInstanceName tests

func TestPairingRequestInstanceName(t *testing.T) {
	got := PairingRequestInstanceName("A1B2C3D4E5F6A7B8", 1234)
	want := "A1B2C3D4E5F6A7B8-1234"
	if got != want {
		t.Errorf("PairingRequestInstanceName() = %q, want %q", got, want)
	}
}

func TestParsePairingRequestInstanceName(t *testing.T) {
	tests := []struct {
		name        string
		wantZoneID  string
		wantDiscrim uint16
		wantErr     bool
	}{
		{"A1B2C3D4E5F6A7B8-1234", "A1B2C3D4E5F6A7B8", 1234, false},
		{"1234567890ABCDEF-0", "1234567890ABCDEF", 0, false},
		{"fedcba0987654321-4095", "fedcba0987654321", 4095, false},
		{"invalid", "", 0, true},
		{"short-1234", "", 0, true},            // ZoneID too short
		{"A1B2C3D4E5F6A7B8-5000", "", 0, true}, // Discriminator out of range
		{"GHIJKLMNOPQRSTUV-1234", "", 0, true}, // Invalid hex in ZoneID
		{"A1B2C3D4E5F6A7B8-abc", "", 0, true},  // Non-numeric discriminator
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zoneID, discrim, err := ParsePairingRequestInstanceName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePairingRequestInstanceName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if zoneID != tt.wantZoneID {
					t.Errorf("zoneID = %q, want %q", zoneID, tt.wantZoneID)
				}
				if discrim != tt.wantDiscrim {
					t.Errorf("discriminator = %d, want %d", discrim, tt.wantDiscrim)
				}
			}
		})
	}
}

// PairingRequestService tests

func TestPairingRequestServiceFromInfo(t *testing.T) {
	info := &PairingRequestInfo{
		Discriminator: 1234,
		ZoneID:        "A1B2C3D4E5F6A7B8",
		ZoneName:      "Home Energy",
		Host:          "controller.local",
	}

	service := &PairingRequestService{
		InstanceName:  PairingRequestInstanceName(info.ZoneID, info.Discriminator),
		Host:          info.Host,
		Port:          0, // Always 0 for pairing requests
		Addresses:     []string{"192.168.1.100"},
		Discriminator: info.Discriminator,
		ZoneID:        info.ZoneID,
		ZoneName:      info.ZoneName,
	}

	if service.InstanceName != "A1B2C3D4E5F6A7B8-1234" {
		t.Errorf("InstanceName = %q, want \"A1B2C3D4E5F6A7B8-1234\"", service.InstanceName)
	}
	if service.Port != 0 {
		t.Errorf("Port = %d, want 0", service.Port)
	}
	if service.Discriminator != 1234 {
		t.Errorf("Discriminator = %d, want 1234", service.Discriminator)
	}
	if service.ZoneID != "A1B2C3D4E5F6A7B8" {
		t.Errorf("ZoneID = %q, want \"A1B2C3D4E5F6A7B8\"", service.ZoneID)
	}
	if service.ZoneName != "Home Energy" {
		t.Errorf("ZoneName = %q, want \"Home Energy\"", service.ZoneName)
	}
}

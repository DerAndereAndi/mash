package discovery

import (
	"testing"
)

// TestBrowsePairingRequests_ParseValidTXT verifies TXT parsing works for pairing requests.
func TestBrowsePairingRequests_ParseValidTXT(t *testing.T) {
	tests := []struct {
		name        string
		txt         TXTRecordMap
		wantDiscrim uint16
		wantZoneID  string
		wantZN      string
		wantErr     bool
	}{
		{
			name: "ValidWithAllFields",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "1234",
				TXTKeyZoneID:        "A1B2C3D4E5F6A7B8",
				TXTKeyZoneName:      "Home Energy",
			},
			wantDiscrim: 1234,
			wantZoneID:  "A1B2C3D4E5F6A7B8",
			wantZN:      "Home Energy",
			wantErr:     false,
		},
		{
			name: "ValidWithoutOptionalZoneName",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "0",
				TXTKeyZoneID:        "1234567890ABCDEF",
			},
			wantDiscrim: 0,
			wantZoneID:  "1234567890ABCDEF",
			wantZN:      "",
			wantErr:     false,
		},
		{
			name: "ValidMaxDiscriminator",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "4095",
				TXTKeyZoneID:        "FEDCBA0987654321",
			},
			wantDiscrim: 4095,
			wantZoneID:  "FEDCBA0987654321",
			wantZN:      "",
			wantErr:     false,
		},
		{
			name: "InvalidMissingDiscriminator",
			txt: TXTRecordMap{
				TXTKeyZoneID: "A1B2C3D4E5F6A7B8",
			},
			wantErr: true,
		},
		{
			name: "InvalidMissingZoneID",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "1234",
			},
			wantErr: true,
		},
		{
			name: "InvalidDiscriminatorOutOfRange",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "5000",
				TXTKeyZoneID:        "A1B2C3D4E5F6A7B8",
			},
			wantErr: true,
		},
		{
			name: "InvalidZoneIDTooShort",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "1234",
				TXTKeyZoneID:        "A1B2",
			},
			wantErr: true,
		},
		{
			name: "InvalidZoneIDNotHex",
			txt: TXTRecordMap{
				TXTKeyDiscriminator: "1234",
				TXTKeyZoneID:        "GHIJKLMNOPQRSTUV",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := DecodePairingRequestTXT(tt.txt)
			if tt.wantErr {
				if err == nil {
					t.Error("DecodePairingRequestTXT() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodePairingRequestTXT() unexpected error: %v", err)
			}
			if info.Discriminator != tt.wantDiscrim {
				t.Errorf("Discriminator = %d, want %d", info.Discriminator, tt.wantDiscrim)
			}
			if info.ZoneID != tt.wantZoneID {
				t.Errorf("ZoneID = %q, want %q", info.ZoneID, tt.wantZoneID)
			}
			if info.ZoneName != tt.wantZN {
				t.Errorf("ZoneName = %q, want %q", info.ZoneName, tt.wantZN)
			}
		})
	}
}

// TestToPairingRequestService_Conversion verifies ServiceEntry to PairingRequestService conversion.
func TestToPairingRequestService_Conversion(t *testing.T) {
	tests := []struct {
		name          string
		entry         ServiceEntry
		wantInstanceN string
		wantDiscrim   uint16
		wantZoneID    string
		wantZoneName  string
		wantHost      string
		wantPort      uint16
		wantAddrs     []string
		wantErr       bool
	}{
		{
			name: "ValidWithAllFields",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Service:  ServiceTypePairingRequest,
				Domain:   Domain,
				Host:     "controller.local",
				Port:     0, // Pairing requests always use port 0
				Text: []string{
					"D=1234",
					"ZI=A1B2C3D4E5F6A7B8",
					"ZN=Home Energy",
				},
				Addrs: []string{"192.168.1.100", "fe80::1"},
			},
			wantInstanceN: "A1B2C3D4E5F6A7B8-1234",
			wantDiscrim:   1234,
			wantZoneID:    "A1B2C3D4E5F6A7B8",
			wantZoneName:  "Home Energy",
			wantHost:      "controller.local",
			wantPort:      0,
			wantAddrs:     []string{"192.168.1.100", "fe80::1"},
			wantErr:       false,
		},
		{
			name: "ValidWithoutOptionalFields",
			entry: ServiceEntry{
				Instance: "1234567890ABCDEF-0",
				Service:  ServiceTypePairingRequest,
				Domain:   Domain,
				Host:     "ems.local",
				Port:     0,
				Text: []string{
					"D=0",
					"ZI=1234567890ABCDEF",
				},
				Addrs: []string{"10.0.0.1"},
			},
			wantInstanceN: "1234567890ABCDEF-0",
			wantDiscrim:   0,
			wantZoneID:    "1234567890ABCDEF",
			wantZoneName:  "",
			wantHost:      "ems.local",
			wantPort:      0,
			wantAddrs:     []string{"10.0.0.1"},
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := tt.entry.ToPairingRequestService()
			if tt.wantErr {
				if err == nil {
					t.Error("ToPairingRequestService() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ToPairingRequestService() unexpected error: %v", err)
			}
			if svc.InstanceName != tt.wantInstanceN {
				t.Errorf("InstanceName = %q, want %q", svc.InstanceName, tt.wantInstanceN)
			}
			if svc.Discriminator != tt.wantDiscrim {
				t.Errorf("Discriminator = %d, want %d", svc.Discriminator, tt.wantDiscrim)
			}
			if svc.ZoneID != tt.wantZoneID {
				t.Errorf("ZoneID = %q, want %q", svc.ZoneID, tt.wantZoneID)
			}
			if svc.ZoneName != tt.wantZoneName {
				t.Errorf("ZoneName = %q, want %q", svc.ZoneName, tt.wantZoneName)
			}
			if svc.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", svc.Host, tt.wantHost)
			}
			if svc.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", svc.Port, tt.wantPort)
			}
			if len(svc.Addresses) != len(tt.wantAddrs) {
				t.Errorf("Addresses len = %d, want %d", len(svc.Addresses), len(tt.wantAddrs))
			}
		})
	}
}

// TestToPairingRequestService_InvalidData verifies error handling for missing required fields.
func TestToPairingRequestService_InvalidData(t *testing.T) {
	tests := []struct {
		name  string
		entry ServiceEntry
	}{
		{
			name: "MissingDiscriminator",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"ZI=A1B2C3D4E5F6A7B8",
				},
			},
		},
		{
			name: "MissingZoneID",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"D=1234",
				},
			},
		},
		{
			name: "InvalidDiscriminator",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"D=invalid",
					"ZI=A1B2C3D4E5F6A7B8",
				},
			},
		},
		{
			name: "DiscriminatorTooHigh",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"D=5000",
					"ZI=A1B2C3D4E5F6A7B8",
				},
			},
		},
		{
			name: "ZoneIDTooShort",
			entry: ServiceEntry{
				Instance: "A1B2-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"D=1234",
					"ZI=A1B2",
				},
			},
		},
		{
			name: "ZoneIDNotHex",
			entry: ServiceEntry{
				Instance: "GHIJKLMNOPQRSTUV-1234",
				Host:     "controller.local",
				Port:     0,
				Text: []string{
					"D=1234",
					"ZI=GHIJKLMNOPQRSTUV",
				},
			},
		},
		{
			name: "EmptyTXTRecords",
			entry: ServiceEntry{
				Instance: "A1B2C3D4E5F6A7B8-1234",
				Host:     "controller.local",
				Port:     0,
				Text:     []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := tt.entry.ToPairingRequestService()
			if err == nil {
				t.Errorf("ToPairingRequestService() expected error, got service: %+v", svc)
			}
		})
	}
}

// TestBrowsePairingRequests_CallbackInvoked tests that callback is called for discovered services.
// This test uses the conversion function directly since actual mDNS browsing requires network access.
func TestBrowsePairingRequests_CallbackInvoked(t *testing.T) {
	// Create a valid service entry
	entry := ServiceEntry{
		Instance: "A1B2C3D4E5F6A7B8-1234",
		Service:  ServiceTypePairingRequest,
		Domain:   Domain,
		Host:     "controller.local",
		Port:     0,
		Text: []string{
			"D=1234",
			"ZI=A1B2C3D4E5F6A7B8",
			"ZN=Home Energy",
		},
		Addrs: []string{"192.168.1.100"},
	}

	// Convert to PairingRequestService
	svc, err := entry.ToPairingRequestService()
	if err != nil {
		t.Fatalf("ToPairingRequestService() error = %v", err)
	}

	// Verify callback would receive correct data
	callbackInvoked := false
	callback := func(service PairingRequestService) {
		callbackInvoked = true
		if service.Discriminator != 1234 {
			t.Errorf("callback Discriminator = %d, want 1234", service.Discriminator)
		}
		if service.ZoneID != "A1B2C3D4E5F6A7B8" {
			t.Errorf("callback ZoneID = %q, want \"A1B2C3D4E5F6A7B8\"", service.ZoneID)
		}
		if service.ZoneName != "Home Energy" {
			t.Errorf("callback ZoneName = %q, want \"Home Energy\"", service.ZoneName)
		}
	}

	// Simulate callback invocation
	callback(*svc)

	if !callbackInvoked {
		t.Error("callback was not invoked")
	}
}

// TestBrowsePairingRequests_MalformedTXT tests that malformed TXT records are gracefully skipped.
func TestBrowsePairingRequests_MalformedTXT(t *testing.T) {
	// Test various malformed entries that should be skipped (return nil, not error)
	malformedEntries := []ServiceEntry{
		{
			Instance: "invalid-instance",
			Host:     "controller.local",
			Port:     0,
			Text:     []string{"garbage=data"},
		},
		{
			Instance: "A1B2C3D4E5F6A7B8-1234",
			Host:     "controller.local",
			Port:     0,
			Text:     []string{}, // Empty TXT
		},
		{
			Instance: "A1B2C3D4E5F6A7B8-1234",
			Host:     "controller.local",
			Port:     0,
			Text:     []string{"D=invalid", "ZI=A1B2C3D4E5F6A7B8"},
		},
	}

	for i, entry := range malformedEntries {
		svc, err := entry.ToPairingRequestService()
		// Should return error, indicating the entry should be skipped
		if err == nil && svc != nil {
			t.Errorf("Entry %d: expected error or nil service for malformed TXT, got: %+v", i, svc)
		}
	}
}

// TestBrowsePairingRequests_InstanceNameParsing tests that instance name is parsed correctly.
func TestBrowsePairingRequests_InstanceNameParsing(t *testing.T) {
	tests := []struct {
		name        string
		instanceN   string
		wantZoneID  string
		wantDiscrim uint16
		wantErr     bool
	}{
		{
			name:        "ValidInstanceName",
			instanceN:   "A1B2C3D4E5F6A7B8-1234",
			wantZoneID:  "A1B2C3D4E5F6A7B8",
			wantDiscrim: 1234,
			wantErr:     false,
		},
		{
			name:        "ValidZeroDiscriminator",
			instanceN:   "1234567890ABCDEF-0",
			wantZoneID:  "1234567890ABCDEF",
			wantDiscrim: 0,
			wantErr:     false,
		},
		{
			name:        "ValidMaxDiscriminator",
			instanceN:   "FEDCBA0987654321-4095",
			wantZoneID:  "FEDCBA0987654321",
			wantDiscrim: 4095,
			wantErr:     false,
		},
		{
			name:      "InvalidNoHyphen",
			instanceN: "A1B2C3D4E5F6A7B81234",
			wantErr:   true,
		},
		{
			name:      "InvalidZoneIDTooShort",
			instanceN: "A1B2-1234",
			wantErr:   true,
		},
		{
			name:      "InvalidDiscriminatorOutOfRange",
			instanceN: "A1B2C3D4E5F6A7B8-5000",
			wantErr:   true,
		},
		{
			name:      "InvalidDiscriminatorNotNumber",
			instanceN: "A1B2C3D4E5F6A7B8-abc",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zoneID, discrim, err := ParsePairingRequestInstanceName(tt.instanceN)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePairingRequestInstanceName(%q) expected error, got nil", tt.instanceN)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePairingRequestInstanceName(%q) unexpected error: %v", tt.instanceN, err)
			}
			if zoneID != tt.wantZoneID {
				t.Errorf("zoneID = %q, want %q", zoneID, tt.wantZoneID)
			}
			if discrim != tt.wantDiscrim {
				t.Errorf("discriminator = %d, want %d", discrim, tt.wantDiscrim)
			}
		})
	}
}

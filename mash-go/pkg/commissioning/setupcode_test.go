package commissioning

import (
	"testing"
)

func TestGenerateSetupCode(t *testing.T) {
	// Generate multiple codes and check they're valid
	codes := make(map[SetupCode]bool)
	for i := 0; i < 100; i++ {
		code, err := GenerateSetupCode()
		if err != nil {
			t.Fatalf("GenerateSetupCode failed: %v", err)
		}
		if err := code.Validate(); err != nil {
			t.Errorf("generated code %d is invalid: %v", code, err)
		}
		codes[code] = true
	}

	// Check we got variety (statistically, 100 random codes should be unique)
	if len(codes) < 90 {
		t.Errorf("expected more unique codes, got %d", len(codes))
	}
}

func TestParseSetupCode(t *testing.T) {
	tests := []struct {
		input   string
		want    SetupCode
		wantErr bool
	}{
		{"00000000", 0, false},
		{"12345678", 12345678, false},
		{"99999999", 99999999, false},
		{"00000001", 1, false},
		{"  12345678  ", 12345678, false}, // with whitespace

		// Invalid cases
		{"1234567", 0, true},   // too short
		{"123456789", 0, true}, // too long
		{"", 0, true},          // empty
		{"1234567a", 0, true},  // non-numeric
		{"-1234567", 0, true},  // negative
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseSetupCode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSetupCode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseSetupCode(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetupCodeString(t *testing.T) {
	tests := []struct {
		code SetupCode
		want string
	}{
		{0, "00000000"},
		{1, "00000001"},
		{12345678, "12345678"},
		{99999999, "99999999"},
	}

	for _, tt := range tests {
		got := tt.code.String()
		if got != tt.want {
			t.Errorf("SetupCode(%d).String() = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestSetupCodeBytes(t *testing.T) {
	code := SetupCode(12345678)
	want := []byte("12345678")
	got := code.Bytes()

	if string(got) != string(want) {
		t.Errorf("SetupCode.Bytes() = %q, want %q", got, want)
	}
}

func TestSetupCodeValidate(t *testing.T) {
	tests := []struct {
		code    SetupCode
		wantErr bool
	}{
		{0, false},
		{12345678, false},
		{99999999, false},
		{100000000, true}, // exceeds max
	}

	for _, tt := range tests {
		err := tt.code.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("SetupCode(%d).Validate() error = %v, wantErr %v", tt.code, err, tt.wantErr)
		}
	}
}

func TestParseQRCode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *QRCodeData
		wantErr bool
	}{
		{
			name:  "valid with hex IDs",
			input: "MASH:1:1234:12345678:0x1234:0x5678",
			want: &QRCodeData{
				Version:       1,
				Discriminator: 1234,
				SetupCode:     12345678,
				VendorID:      0x1234,
				ProductID:     0x5678,
			},
		},
		{
			name:  "valid with decimal IDs",
			input: "MASH:1:4095:00000001:1234:5678",
			want: &QRCodeData{
				Version:       1,
				Discriminator: 4095,
				SetupCode:     1,
				VendorID:      1234,
				ProductID:     5678,
			},
		},
		{
			name:  "zero discriminator",
			input: "MASH:1:0:99999999:0x0000:0x0000",
			want: &QRCodeData{
				Version:       1,
				Discriminator: 0,
				SetupCode:     99999999,
				VendorID:      0,
				ProductID:     0,
			},
		},
		{
			name:  "with whitespace",
			input: "  MASH:1:1234:12345678:0x1234:0x5678  ",
			want: &QRCodeData{
				Version:       1,
				Discriminator: 1234,
				SetupCode:     12345678,
				VendorID:      0x1234,
				ProductID:     0x5678,
			},
		},

		// Invalid cases
		{name: "wrong prefix", input: "FOO:1:1234:12345678:0x1234:0x5678", wantErr: true},
		{name: "unsupported version", input: "MASH:2:1234:12345678:0x1234:0x5678", wantErr: true},
		{name: "discriminator too large", input: "MASH:1:4096:12345678:0x1234:0x5678", wantErr: true},
		{name: "setup code too short", input: "MASH:1:1234:1234567:0x1234:0x5678", wantErr: true},
		{name: "setup code too long", input: "MASH:1:1234:123456789:0x1234:0x5678", wantErr: true},
		{name: "missing field", input: "MASH:1:1234:12345678:0x1234", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseQRCode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseQRCode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version = %d, want %d", got.Version, tt.want.Version)
			}
			if got.Discriminator != tt.want.Discriminator {
				t.Errorf("Discriminator = %d, want %d", got.Discriminator, tt.want.Discriminator)
			}
			if got.SetupCode != tt.want.SetupCode {
				t.Errorf("SetupCode = %d, want %d", got.SetupCode, tt.want.SetupCode)
			}
			if got.VendorID != tt.want.VendorID {
				t.Errorf("VendorID = %d, want %d", got.VendorID, tt.want.VendorID)
			}
			if got.ProductID != tt.want.ProductID {
				t.Errorf("ProductID = %d, want %d", got.ProductID, tt.want.ProductID)
			}
		})
	}
}

func TestQRCodeDataString(t *testing.T) {
	qr := &QRCodeData{
		Version:       1,
		Discriminator: 1234,
		SetupCode:     12345678,
		VendorID:      0x1234,
		ProductID:     0x5678,
	}

	got := qr.String()
	want := "MASH:1:1234:12345678:0x1234:0x5678"

	if got != want {
		t.Errorf("QRCodeData.String() = %q, want %q", got, want)
	}

	// Verify round-trip
	parsed, err := ParseQRCode(got)
	if err != nil {
		t.Fatalf("ParseQRCode failed on generated string: %v", err)
	}
	if parsed.SetupCode != qr.SetupCode {
		t.Errorf("round-trip SetupCode = %d, want %d", parsed.SetupCode, qr.SetupCode)
	}
}

func TestGenerateDiscriminator(t *testing.T) {
	discriminators := make(map[uint16]bool)
	for i := 0; i < 100; i++ {
		d, err := GenerateDiscriminator()
		if err != nil {
			t.Fatalf("GenerateDiscriminator failed: %v", err)
		}
		if d > DiscriminatorMax {
			t.Errorf("discriminator %d exceeds max %d", d, DiscriminatorMax)
		}
		discriminators[d] = true
	}

	// Should have variety
	if len(discriminators) < 50 {
		t.Errorf("expected more unique discriminators, got %d", len(discriminators))
	}
}

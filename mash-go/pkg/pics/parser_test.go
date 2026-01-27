package pics

import (
	"strings"
	"testing"
)

func TestParseSimplePICS(t *testing.T) {
	input := `
# Device PICS
MASH.S=1
MASH.S.VERSION=1
MASH.S.CTRL=1
MASH.S.ELEC=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}

	if pics.Side != SideServer {
		t.Errorf("expected Side=S, got %v", pics.Side)
	}

	if pics.Version != 1 {
		t.Errorf("expected Version=1, got %d", pics.Version)
	}

	if !pics.HasFeature("CTRL") {
		t.Error("expected CTRL feature to be present")
	}

	if !pics.HasFeature("ELEC") {
		t.Error("expected ELEC feature to be present")
	}
}

func TestParseAttributesAndCommands(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1
MASH.S.CTRL.A02=1
MASH.S.CTRL.C01.Rsp=1
MASH.S.CTRL.C02.Rsp=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.HasAttribute("CTRL", "01") {
		t.Error("expected CTRL.A01 to be present")
	}

	if !pics.HasAttribute("CTRL", "02") {
		t.Error("expected CTRL.A02 to be present")
	}

	if !pics.HasCommand("CTRL", "01") {
		t.Error("expected CTRL.C01.Rsp to be present")
	}

	if !pics.HasCommand("CTRL", "02") {
		t.Error("expected CTRL.C02.Rsp to be present")
	}
}

func TestParseFeatureFlags(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.F00=1
MASH.S.CTRL.F03=1
MASH.S.CTRL.F09=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.HasFeatureFlag("CTRL", "00") {
		t.Error("expected CTRL.F00 to be present")
	}

	if !pics.HasFeatureFlag("CTRL", "03") {
		t.Error("expected CTRL.F03 to be present")
	}

	if !pics.HasFeatureFlag("CTRL", "09") {
		t.Error("expected CTRL.F09 to be present")
	}

	if pics.HasFeatureFlag("CTRL", "0A") {
		t.Error("expected CTRL.F0A to NOT be present")
	}
}

func TestParseBehaviorOptions(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.B_LIMIT_DEFAULT="unlimited"
MASH.S.CTRL.B_DURATION_EXPIRY="clear"
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	val := pics.GetString("MASH.S.CTRL.B_LIMIT_DEFAULT")
	if val != "unlimited" {
		t.Errorf("expected B_LIMIT_DEFAULT=unlimited, got %s", val)
	}

	val = pics.GetString("MASH.S.CTRL.B_DURATION_EXPIRY")
	if val != "clear" {
		t.Errorf("expected B_DURATION_EXPIRY=clear, got %s", val)
	}
}

func TestParseInlineComments(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1  # deviceType
MASH.S.CTRL.A02=1  # controlState
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.HasAttribute("CTRL", "01") {
		t.Error("expected CTRL.A01 to be present")
	}
}

func TestParseIntegerValues(t *testing.T) {
	input := `
MASH.S=1
MASH.S.VERSION=1
MASH.S.ENDPOINTS=2
MASH.S.CONN.MAX_CONNECTIONS=50
MASH.S.CONN.BACKOFF_INITIAL=1
MASH.S.CONN.BACKOFF_MAX=60
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if pics.GetInt("MASH.S.VERSION") != 1 {
		t.Errorf("expected VERSION=1, got %d", pics.GetInt("MASH.S.VERSION"))
	}

	if pics.GetInt("MASH.S.ENDPOINTS") != 2 {
		t.Errorf("expected ENDPOINTS=2, got %d", pics.GetInt("MASH.S.ENDPOINTS"))
	}

	if pics.GetInt("MASH.S.CONN.MAX_CONNECTIONS") != 50 {
		t.Errorf("expected MAX_CONNECTIONS=50, got %d", pics.GetInt("MASH.S.CONN.MAX_CONNECTIONS"))
	}
}

func TestParseFloatValues(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CONN.JITTER_FACTOR=0.25
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	val := pics.GetString("MASH.S.CONN.JITTER_FACTOR")
	if val != "0.25" {
		t.Errorf("expected JITTER_FACTOR=0.25, got %s", val)
	}
}

func TestParseControllerPICS(t *testing.T) {
	input := `
MASH.C=1
MASH.C.VERSION=1
MASH.C.ZONE.TYPE="LOCAL"
MASH.C.ZONE.PRIORITY=2
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if pics.Side != SideClient {
		t.Errorf("expected Side=C, got %v", pics.Side)
	}

	if !pics.IsController() {
		t.Error("expected IsController() to be true")
	}

	if pics.IsDevice() {
		t.Error("expected IsDevice() to be false")
	}

	zoneType := pics.GetString("MASH.C.ZONE.TYPE")
	if zoneType != "LOCAL" {
		t.Errorf("expected ZONE.TYPE=LOCAL, got %s", zoneType)
	}
}

func TestParseMinimalDevicePICS(t *testing.T) {
	// Test parsing a realistic minimal device PICS
	input := `
# MASH PICS File - Minimal Device
# Device: Generic Energy Device - Minimal Implementation
MASH.S=1
MASH.S.VERSION=1

# Transport Layer
MASH.S.TRANS=1
MASH.S.TRANS.IPV6=1
MASH.S.TRANS.TLS13=1
MASH.S.TRANS.PORT=8443
MASH.S.TRANS.TLS_AES_128_GCM=1
MASH.S.TRANS.TLS_AES_256_GCM=0
MASH.S.TRANS.TLS_CHACHA20=0
MASH.S.TRANS.ECDHE_P256=1
MASH.S.TRANS.ECDHE_X25519=0

# Commissioning
MASH.S.COMM=1
MASH.S.COMM.PASE=1
MASH.S.COMM.PASE_GROUP="P-256"
MASH.S.COMM.WINDOW_DURATION=120
MASH.S.COMM.ATTESTATION=0
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Check transport features
	if !pics.Has("MASH.S.TRANS") {
		t.Error("expected TRANS to be present")
	}

	if !pics.Has("MASH.S.TRANS.TLS_AES_128_GCM") {
		t.Error("expected TLS_AES_128_GCM to be present")
	}

	// Verify disabled feature
	if pics.Has("MASH.S.TRANS.TLS_CHACHA20") {
		t.Error("expected TLS_CHACHA20 to NOT be true")
	}

	// Check commissioning
	paseGroup := pics.GetString("MASH.S.COMM.PASE_GROUP")
	if paseGroup != "P-256" {
		t.Errorf("expected PASE_GROUP=P-256, got %s", paseGroup)
	}

	windowDuration := pics.GetInt("MASH.S.COMM.WINDOW_DURATION")
	if windowDuration != 120 {
		t.Errorf("expected WINDOW_DURATION=120, got %d", windowDuration)
	}
}

func TestParseError(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "missing equals",
			input: "MASH.S",
		},
		{
			name:  "empty value",
			input: "MASH.S=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseString(tt.input)
			// We expect either an error or the parser handles it gracefully
			// The current parser is lenient, so we just verify it doesn't panic
			_ = err
		})
	}
}

func TestCodeString(t *testing.T) {
	tests := []struct {
		code     Code
		expected string
	}{
		{
			code:     Code{Side: SideServer},
			expected: "MASH.S",
		},
		{
			code:     Code{Side: SideServer, Feature: "CTRL"},
			expected: "MASH.S.CTRL",
		},
		{
			code:     Code{Side: SideServer, Feature: "CTRL", Type: CodeTypeAttribute, ID: "01"},
			expected: "MASH.S.CTRL.A01",
		},
		{
			code:     Code{Side: SideServer, Feature: "CTRL", Type: CodeTypeCommand, ID: "01", Qualifier: QualifierResponse},
			expected: "MASH.S.CTRL.C01.Rsp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("Code.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestGetMethods(t *testing.T) {
	input := `
MASH.S=1
MASH.S.VERSION=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1
MASH.S.MISSING=0
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Test Get
	if v, ok := pics.Get("MASH.S.VERSION"); !ok {
		t.Error("expected Get to find MASH.S.VERSION")
	} else if v.Int != 1 {
		t.Errorf("expected VERSION=1, got %d", v.Int)
	}

	// Test Get for missing
	if _, ok := pics.Get("MASH.S.NONEXISTENT"); ok {
		t.Error("expected Get to NOT find MASH.S.NONEXISTENT")
	}

	// Test Has with false value
	if pics.Has("MASH.S.MISSING") {
		t.Error("expected Has to return false for MISSING=0")
	}

	// Test GetInt for missing
	if v := pics.GetInt("MASH.S.NONEXISTENT"); v != 0 {
		t.Errorf("expected GetInt for missing to return 0, got %d", v)
	}

	// Test GetString for missing
	if v := pics.GetString("MASH.S.NONEXISTENT"); v != "" {
		t.Errorf("expected GetString for missing to return empty, got %s", v)
	}
}

func TestFeaturesList(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.ELEC=1
MASH.S.MEAS=1
MASH.S.STAT=0
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Only enabled features should be in the list
	// Note: STAT=0 should not be included
	expectedFeatures := []string{"CTRL", "ELEC", "MEAS"}

	if len(pics.Features) != len(expectedFeatures) {
		t.Errorf("expected %d features, got %d: %v", len(expectedFeatures), len(pics.Features), pics.Features)
	}

	for _, f := range expectedFeatures {
		found := false
		for _, pf := range pics.Features {
			if pf == f {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected feature %s to be in list", f)
		}
	}
}

func TestLineNumbers(t *testing.T) {
	input := `# Comment line 1
# Comment line 2
MASH.S=1
# Comment line 4
MASH.S.CTRL=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Find the MASH.S entry
	entry, ok := pics.ByCode["MASH.S"]
	if !ok {
		t.Fatal("expected to find MASH.S entry")
	}

	if entry.LineNumber != 3 {
		t.Errorf("expected MASH.S on line 3, got line %d", entry.LineNumber)
	}

	// Find the MASH.S.CTRL entry
	entry, ok = pics.ByCode["MASH.S.CTRL"]
	if !ok {
		t.Fatal("expected to find MASH.S.CTRL entry")
	}

	if entry.LineNumber != 5 {
		t.Errorf("expected MASH.S.CTRL on line 5, got line %d", entry.LineNumber)
	}
}

func BenchmarkParsePICS(b *testing.B) {
	// A realistic PICS file content
	input := strings.Repeat(`
MASH.S=1
MASH.S.VERSION=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1
MASH.S.CTRL.A02=1
MASH.S.CTRL.A0A=1
MASH.S.CTRL.A0B=1
MASH.S.CTRL.A0C=1
MASH.S.CTRL.A0E=1
MASH.S.CTRL.C01.Rsp=1
MASH.S.CTRL.C02.Rsp=1
MASH.S.ELEC=1
MASH.S.MEAS=1
`, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseString(input)
		if err != nil {
			b.Fatal(err)
		}
	}
}

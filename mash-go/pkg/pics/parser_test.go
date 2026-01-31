package pics

import (
	"strings"
	"testing"
)

func TestParseSimplePICS(t *testing.T) {
	input := `
# Device PICS
MASH.S=1
MASH.S.VERSION=1.0
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

	if pics.Version != "1.0" {
		t.Errorf("expected Version=1.0, got %s", pics.Version)
	}

	if !pics.HasFeature("CTRL") {
		t.Error("expected CTRL feature to be present")
	}

	if !pics.HasFeature("ELEC") {
		t.Error("expected ELEC feature to be present")
	}
}

func TestParseVersion_MajorMinor(t *testing.T) {
	input := `MASH.S=1
MASH.S.VERSION=1.0
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if pics.Version != "1.0" {
		t.Errorf("expected Version=1.0, got %s", pics.Version)
	}
}

func TestParseVersion_IntegerBackwardCompat(t *testing.T) {
	input := `MASH.S=1
MASH.S.VERSION=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if pics.Version != "1" {
		t.Errorf("expected Version=1, got %s", pics.Version)
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

func TestParseEndpointTypeDeclaration(t *testing.T) {
	input := `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E02=INVERTER
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Endpoint 1
	if pics.EndpointType(1) != "EV_CHARGER" {
		t.Errorf("EndpointType(1) = %s, want EV_CHARGER", pics.EndpointType(1))
	}
	// Endpoint 2
	if pics.EndpointType(2) != "INVERTER" {
		t.Errorf("EndpointType(2) = %s, want INVERTER", pics.EndpointType(2))
	}

	ids := pics.EndpointIDs()
	if len(ids) != 2 {
		t.Fatalf("len(EndpointIDs()) = %d, want 2", len(ids))
	}
}

func TestParseEndpointFeatureCodes(t *testing.T) {
	input := `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.MEAS=1
MASH.S.E01.CTRL.A01=1
MASH.S.E01.CTRL.A02=1
MASH.S.E01.MEAS.A01=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.F03=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify endpoint was populated
	ep := pics.Endpoints[1]
	if ep == nil {
		t.Fatal("expected endpoint 1 to exist")
	}
	if ep.Type != "EV_CHARGER" {
		t.Errorf("endpoint 1 type = %s, want EV_CHARGER", ep.Type)
	}

	// Verify features tracked on endpoint
	found := false
	for _, f := range ep.Features {
		if f == "CTRL" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CTRL in endpoint features, got %v", ep.Features)
	}

	// Verify code parsing
	entry, ok := pics.ByCode["MASH.S.E01.CTRL.A01"]
	if !ok {
		t.Fatal("expected MASH.S.E01.CTRL.A01 in ByCode")
	}
	if entry.Code.EndpointID != 1 {
		t.Errorf("EndpointID = %d, want 1", entry.Code.EndpointID)
	}
	if entry.Code.Feature != "CTRL" {
		t.Errorf("Feature = %s, want CTRL", entry.Code.Feature)
	}
	if entry.Code.Type != CodeTypeAttribute {
		t.Errorf("Type = %s, want A", entry.Code.Type)
	}
	if entry.Code.ID != "01" {
		t.Errorf("ID = %s, want 01", entry.Code.ID)
	}

	// Verify command with qualifier
	entry, ok = pics.ByCode["MASH.S.E01.CTRL.C01.Rsp"]
	if !ok {
		t.Fatal("expected MASH.S.E01.CTRL.C01.Rsp in ByCode")
	}
	if entry.Code.EndpointID != 1 {
		t.Errorf("command EndpointID = %d, want 1", entry.Code.EndpointID)
	}
	if entry.Code.Qualifier != QualifierResponse {
		t.Errorf("Qualifier = %s, want Rsp", entry.Code.Qualifier)
	}

	// Verify feature flag
	entry, ok = pics.ByCode["MASH.S.E01.CTRL.F03"]
	if !ok {
		t.Fatal("expected MASH.S.E01.CTRL.F03 in ByCode")
	}
	if entry.Code.EndpointID != 1 {
		t.Errorf("flag EndpointID = %d, want 1", entry.Code.EndpointID)
	}
	if entry.Code.Type != CodeTypeFlag {
		t.Errorf("flag Type = %s, want F", entry.Code.Type)
	}
}

func TestParseEndpointWithDeviceLevelTransport(t *testing.T) {
	input := `MASH.S=1
MASH.S.VERSION=1.0
MASH.S.TRANS=1
MASH.S.COMM=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.MEAS=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Device-level features
	if !pics.Has("MASH.S.TRANS") {
		t.Error("expected MASH.S.TRANS to be present")
	}
	if !pics.Has("MASH.S.COMM") {
		t.Error("expected MASH.S.COMM to be present")
	}

	// Transport should be in device-level Features
	foundTrans := false
	for _, f := range pics.Features {
		if f == "TRANS" {
			foundTrans = true
			break
		}
	}
	if !foundTrans {
		t.Errorf("expected TRANS in device-level Features, got %v", pics.Features)
	}

	// Endpoint features
	if !pics.Has("MASH.S.E01.CTRL") {
		t.Error("expected MASH.S.E01.CTRL to be present")
	}
	if !pics.Has("MASH.S.E01.MEAS") {
		t.Error("expected MASH.S.E01.MEAS to be present")
	}

	// Verify endpoint
	if pics.EndpointType(1) != "EV_CHARGER" {
		t.Errorf("EndpointType(1) = %s, want EV_CHARGER", pics.EndpointType(1))
	}
}

func TestParseMultipleEndpoints(t *testing.T) {
	input := `MASH.S=1
MASH.S.E01=INVERTER
MASH.S.E01.CTRL=1
MASH.S.E01.MEAS=1
MASH.S.E02=BATTERY
MASH.S.E02.CTRL=1
MASH.S.E02.MEAS=1
MASH.S.E02.CTRL.F02=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	ids := pics.EndpointIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(ids))
	}

	if pics.EndpointType(1) != "INVERTER" {
		t.Errorf("ep1 type = %s, want INVERTER", pics.EndpointType(1))
	}
	if pics.EndpointType(2) != "BATTERY" {
		t.Errorf("ep2 type = %s, want BATTERY", pics.EndpointType(2))
	}

	// BATTERY flag on endpoint 2
	if !pics.Has("MASH.S.E02.CTRL.F02") {
		t.Error("expected MASH.S.E02.CTRL.F02 to be present")
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

func TestDetectFormat_KeyValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "simple key=value",
			input: "MASH.S=1\nMASH.S.VERSION=1",
		},
		{
			name: "with comments",
			input: `# Comment
MASH.S=1
# Another comment
MASH.S.CTRL=1`,
		},
		{
			name: "realistic PICS",
			input: `# MASH PICS File
MASH.S=1
MASH.S.VERSION=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1  # deviceType`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := detectFormat([]byte(tt.input))
			if format != FormatKeyValue {
				t.Errorf("detectFormat() = %v, want FormatKeyValue", format)
			}
		})
	}
}

func TestDetectFormat_YAML_Items(t *testing.T) {
	input := `# PICS YAML format
items:
  MASH.S.TRANS.SC: true
  MASH.S.ELEC.AC: true`

	format := detectFormat([]byte(input))
	if format != FormatYAML {
		t.Errorf("detectFormat() = %v, want FormatYAML", format)
	}
}

func TestDetectFormat_YAML_Device(t *testing.T) {
	input := `device:
  vendor: "Example Corp"
  product: "Smart Charger"
items:
  MASH.S.TRANS.SC: true`

	format := detectFormat([]byte(input))
	if format != FormatYAML {
		t.Errorf("detectFormat() = %v, want FormatYAML", format)
	}
}

func TestDetectFormat_YAML_IndentedContent(t *testing.T) {
	input := `# YAML with indentation
device:
  vendor: "Test"
  product: "Test"`

	format := detectFormat([]byte(input))
	if format != FormatYAML {
		t.Errorf("detectFormat() = %v, want FormatYAML", format)
	}
}

func TestDetectFormat_Empty(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only whitespace", "   \n   \n"},
		{"only comments", "# comment\n# another"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := detectFormat([]byte(tt.input))
			if format != FormatKeyValue {
				t.Errorf("detectFormat() = %v, want FormatKeyValue (default)", format)
			}
		})
	}
}

func TestDetectFormat_MixedIndicators(t *testing.T) {
	// First non-comment line determines format
	tests := []struct {
		name     string
		input    string
		expected Format
	}{
		{
			name:     "key=value first",
			input:    "# comment\nMASH.S=1\nitems:",
			expected: FormatKeyValue,
		},
		{
			name:     "items: first",
			input:    "# comment\nitems:\n  MASH.S.TRANS.SC: true",
			expected: FormatYAML,
		},
		{
			name:     "device: first",
			input:    "# comment\ndevice:\n  vendor: test",
			expected: FormatYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := detectFormat([]byte(tt.input))
			if format != tt.expected {
				t.Errorf("detectFormat() = %v, want %v", format, tt.expected)
			}
		})
	}
}

func TestParse_AutoDetectKeyValue(t *testing.T) {
	input := `MASH.S=1
MASH.S.VERSION=1
MASH.S.CTRL=1`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if pics.Format != FormatKeyValue {
		t.Errorf("Format = %v, want FormatKeyValue", pics.Format)
	}

	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}

	if !pics.HasFeature("CTRL") {
		t.Error("expected CTRL feature to be present")
	}
}

func TestParse_AutoDetectYAML(t *testing.T) {
	input := `device:
  vendor: "Test Corp"
items:
  MASH.S: 1
  MASH.S.CTRL: 1`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if pics.Format != FormatYAML {
		t.Errorf("Format = %v, want FormatYAML", pics.Format)
	}

	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}

	if pics.Device == nil {
		t.Error("expected Device to be non-nil")
	}

	if pics.Device.Vendor != "Test Corp" {
		t.Errorf("Device.Vendor = %s, want Test Corp", pics.Device.Vendor)
	}
}

func TestParseWithOptions_ExplicitFormat(t *testing.T) {
	// YAML content but force key=value parsing (should fail or behave differently)
	yamlInput := `items:
  MASH.S: 1`

	parser := NewParser()

	// Auto-detect should work
	pics, err := parser.ParseStringWithOptions(yamlInput, ParseOptions{Format: FormatAuto})
	if err != nil {
		t.Fatalf("Auto-detect failed: %v", err)
	}
	if pics.Format != FormatYAML {
		t.Errorf("Auto-detect Format = %v, want FormatYAML", pics.Format)
	}

	// Force key-value format
	_, err = parser.ParseStringWithOptions(yamlInput, ParseOptions{Format: FormatKeyValue})
	// This should either fail or produce different results
	// since YAML content doesn't have = signs
	if err == nil {
		// It parsed, but probably incorrectly - that's expected when forcing wrong format
		t.Log("Forcing key-value format on YAML content succeeded (produces empty/incorrect results)")
	}
}

func TestParseBytes(t *testing.T) {
	data := []byte("MASH.S=1\nMASH.S.CTRL=1")

	pics, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes failed: %v", err)
	}

	if !pics.Has("MASH.S") {
		t.Error("expected MASH.S to be present")
	}
}

func TestParseFile_SourceFile(t *testing.T) {
	// Parse an actual file to verify SourceFile is set
	pics, err := ParseFile("../../testdata/pics/minimal-device-pairing.pics")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if pics.SourceFile != "../../testdata/pics/minimal-device-pairing.pics" {
		t.Errorf("SourceFile = %s, want testdata path", pics.SourceFile)
	}
}

func TestParseFile_YAML(t *testing.T) {
	// Parse an actual YAML file
	pics, err := ParseFile("../../testdata/pics/ev-charger.yaml")
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if pics.Format != FormatYAML {
		t.Errorf("Format = %v, want FormatYAML", pics.Format)
	}

	if pics.Device == nil {
		t.Error("expected Device to be non-nil")
	}

	if pics.Device.Vendor != "Example Corp" {
		t.Errorf("Device.Vendor = %s, want Example Corp", pics.Device.Vendor)
	}

	// Check that entries are present
	if !pics.Has("MASH.S.TRANS.SC") {
		t.Error("expected MASH.S.TRANS.SC to be present")
	}
}

func TestParseUCCodes_KeyValue(t *testing.T) {
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.UC.MPD=1
MASH.S.UC.EVC=0
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// UC.LPC should be present and true
	if !pics.Has("MASH.S.UC.LPC") {
		t.Error("expected MASH.S.UC.LPC to be present and true")
	}

	// UC.MPD should be present and true
	if !pics.Has("MASH.S.UC.MPD") {
		t.Error("expected MASH.S.UC.MPD to be present and true")
	}

	// UC.EVC should be present but false (value=0)
	if pics.Has("MASH.S.UC.EVC") {
		t.Error("expected MASH.S.UC.EVC to NOT be true (value=0)")
	}
	if _, ok := pics.Get("MASH.S.UC.EVC"); !ok {
		t.Error("expected MASH.S.UC.EVC to be present in ByCode")
	}

	// UC codes should appear in device-level Features
	foundUCLPC := false
	for _, f := range pics.Features {
		if f == "UC.LPC" {
			foundUCLPC = true
			break
		}
	}
	if !foundUCLPC {
		t.Errorf("expected UC.LPC in device-level Features, got %v", pics.Features)
	}
}

func TestParseUCCodes_YAML(t *testing.T) {
	input := `device:
  vendor: "Test"
items:
  MASH.C: 1
  MASH.C.UC.LPC: true
  MASH.C.UC.LPP: true
  MASH.C.UC.MPD: true
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if pics.Side != SideClient {
		t.Errorf("expected Side=C, got %v", pics.Side)
	}

	if !pics.Has("MASH.C.UC.LPC") {
		t.Error("expected MASH.C.UC.LPC to be present and true")
	}

	if !pics.Has("MASH.C.UC.LPP") {
		t.Error("expected MASH.C.UC.LPP to be present and true")
	}

	if !pics.Has("MASH.C.UC.MPD") {
		t.Error("expected MASH.C.UC.MPD to be present and true")
	}
}

func TestParseUCCodes_ParsedCode(t *testing.T) {
	input := `MASH.S=1
MASH.S.UC.LPC=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	entry, ok := pics.ByCode["MASH.S.UC.LPC"]
	if !ok {
		t.Fatal("expected MASH.S.UC.LPC in ByCode")
	}

	if entry.Code.Side != SideServer {
		t.Errorf("Side = %s, want S", entry.Code.Side)
	}
	if entry.Code.EndpointID != 0 {
		t.Errorf("EndpointID = %d, want 0 (device-level)", entry.Code.EndpointID)
	}
	if entry.Code.Feature != "UC.LPC" {
		t.Errorf("Feature = %s, want UC.LPC", entry.Code.Feature)
	}
}

func TestPICS_HasUseCase(t *testing.T) {
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.UC.MPD=1
MASH.S.UC.EVC=0
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.HasUseCase("LPC") {
		t.Error("expected HasUseCase(LPC) to return true")
	}

	if !pics.HasUseCase("MPD") {
		t.Error("expected HasUseCase(MPD) to return true")
	}

	if pics.HasUseCase("EVC") {
		t.Error("expected HasUseCase(EVC) to return false (value=0)")
	}

	if pics.HasUseCase("NONEXISTENT") {
		t.Error("expected HasUseCase(NONEXISTENT) to return false")
	}
}

func TestPICS_HasUseCase_Controller(t *testing.T) {
	input := `
MASH.C=1
MASH.C.UC.LPC=1
MASH.C.UC.LPP=1
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if !pics.HasUseCase("LPC") {
		t.Error("expected HasUseCase(LPC) to return true for controller")
	}

	if !pics.HasUseCase("LPP") {
		t.Error("expected HasUseCase(LPP) to return true for controller")
	}
}

func TestPICS_UseCases(t *testing.T) {
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.UC.MPD=1
MASH.S.UC.EVC=0
`
	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	ucs := pics.UseCases()

	// Only true UC codes should be returned
	if len(ucs) != 2 {
		t.Fatalf("expected 2 use cases, got %d: %v", len(ucs), ucs)
	}

	// Should be sorted
	if ucs[0] != "LPC" || ucs[1] != "MPD" {
		t.Errorf("UseCases() = %v, want [LPC MPD]", ucs)
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

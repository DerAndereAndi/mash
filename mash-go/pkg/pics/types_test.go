package pics

import (
	"testing"
)

func TestFormat_String(t *testing.T) {
	tests := []struct {
		format   Format
		expected string
	}{
		{FormatAuto, "auto"},
		{FormatKeyValue, "key-value"},
		{FormatYAML, "yaml"},
		{Format(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.format.String(); got != tt.expected {
				t.Errorf("Format.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestDeviceMetadata_Empty(t *testing.T) {
	dm := DeviceMetadata{}

	if dm.Vendor != "" {
		t.Errorf("expected empty Vendor, got %s", dm.Vendor)
	}
	if dm.Product != "" {
		t.Errorf("expected empty Product, got %s", dm.Product)
	}
	if dm.Model != "" {
		t.Errorf("expected empty Model, got %s", dm.Model)
	}
	if dm.Version != "" {
		t.Errorf("expected empty Version, got %s", dm.Version)
	}
}

func TestDeviceMetadata_Populated(t *testing.T) {
	dm := DeviceMetadata{
		Vendor:  "Example Corp",
		Product: "Smart Charger Pro",
		Model:   "SCP-11",
		Version: "1.0.0",
	}

	if dm.Vendor != "Example Corp" {
		t.Errorf("Vendor = %s, want Example Corp", dm.Vendor)
	}
	if dm.Product != "Smart Charger Pro" {
		t.Errorf("Product = %s, want Smart Charger Pro", dm.Product)
	}
	if dm.Model != "SCP-11" {
		t.Errorf("Model = %s, want SCP-11", dm.Model)
	}
	if dm.Version != "1.0.0" {
		t.Errorf("Version = %s, want 1.0.0", dm.Version)
	}
}

func TestPICS_WithDevice(t *testing.T) {
	pics := NewPICS()

	// Initially nil
	if pics.Device != nil {
		t.Error("expected Device to be nil initially")
	}

	// Set device metadata
	pics.Device = &DeviceMetadata{
		Vendor:  "Test Vendor",
		Product: "Test Product",
	}

	if pics.Device == nil {
		t.Fatal("expected Device to be non-nil after setting")
	}
	if pics.Device.Vendor != "Test Vendor" {
		t.Errorf("Device.Vendor = %s, want Test Vendor", pics.Device.Vendor)
	}
}

func TestPICS_Format(t *testing.T) {
	pics := NewPICS()

	// Default format is auto (0)
	if pics.Format != FormatAuto {
		t.Errorf("expected Format to be FormatAuto, got %v", pics.Format)
	}

	pics.Format = FormatYAML
	if pics.Format != FormatYAML {
		t.Errorf("expected Format to be FormatYAML, got %v", pics.Format)
	}
}

func TestPICS_SourceFile(t *testing.T) {
	pics := NewPICS()

	// Initially empty
	if pics.SourceFile != "" {
		t.Errorf("expected SourceFile to be empty, got %s", pics.SourceFile)
	}

	pics.SourceFile = "/path/to/file.pics"
	if pics.SourceFile != "/path/to/file.pics" {
		t.Errorf("SourceFile = %s, want /path/to/file.pics", pics.SourceFile)
	}
}

func TestValue_IsBool(t *testing.T) {
	tests := []struct {
		name     string
		value    Value
		expected bool
	}{
		{"zero", Value{Raw: "0"}, true},
		{"one", Value{Raw: "1"}, true},
		{"integer", Value{Raw: "42"}, false},
		{"true", Value{Raw: "true"}, false},
		{"string", Value{Raw: "foo"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.IsBool(); got != tt.expected {
				t.Errorf("IsBool() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCode_String(t *testing.T) {
	tests := []struct {
		name     string
		code     Code
		expected string
	}{
		{
			name:     "server only",
			code:     Code{Side: SideServer},
			expected: "MASH.S",
		},
		{
			name:     "client only",
			code:     Code{Side: SideClient},
			expected: "MASH.C",
		},
		{
			name:     "with feature",
			code:     Code{Side: SideServer, Feature: "CTRL"},
			expected: "MASH.S.CTRL",
		},
		{
			name:     "with attribute",
			code:     Code{Side: SideServer, Feature: "CTRL", Type: CodeTypeAttribute, ID: "01"},
			expected: "MASH.S.CTRL.A01",
		},
		{
			name:     "command with qualifier",
			code:     Code{Side: SideServer, Feature: "CTRL", Type: CodeTypeCommand, ID: "01", Qualifier: QualifierResponse},
			expected: "MASH.S.CTRL.C01.Rsp",
		},
		{
			name:     "feature flag",
			code:     Code{Side: SideServer, Feature: "CTRL", Type: CodeTypeFlag, ID: "03"},
			expected: "MASH.S.CTRL.F03",
		},
		{
			name: "behavior code",
			code: Code{
				Raw:     "MASH.S.CTRL.B_LIMIT_DEFAULT",
				Side:    SideServer,
				Feature: "CTRL",
				Type:    CodeTypeBehavior,
				ID:      "B_LIMIT_DEFAULT",
			},
			expected: "MASH.S.CTRL.B_LIMIT_DEFAULT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("Code.String() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestSide_Constants(t *testing.T) {
	if SideServer != "S" {
		t.Errorf("SideServer = %s, want S", SideServer)
	}
	if SideClient != "C" {
		t.Errorf("SideClient = %s, want C", SideClient)
	}
}

func TestCodeType_Constants(t *testing.T) {
	if CodeTypeFeature != "" {
		t.Errorf("CodeTypeFeature = %s, want empty", CodeTypeFeature)
	}
	if CodeTypeAttribute != "A" {
		t.Errorf("CodeTypeAttribute = %s, want A", CodeTypeAttribute)
	}
	if CodeTypeCommand != "C" {
		t.Errorf("CodeTypeCommand = %s, want C", CodeTypeCommand)
	}
	if CodeTypeFlag != "F" {
		t.Errorf("CodeTypeFlag = %s, want F", CodeTypeFlag)
	}
	if CodeTypeEvent != "E" {
		t.Errorf("CodeTypeEvent = %s, want E", CodeTypeEvent)
	}
	if CodeTypeBehavior != "B" {
		t.Errorf("CodeTypeBehavior = %s, want B", CodeTypeBehavior)
	}
}

func TestQualifier_Constants(t *testing.T) {
	if QualifierNone != "" {
		t.Errorf("QualifierNone = %s, want empty", QualifierNone)
	}
	if QualifierResponse != "Rsp" {
		t.Errorf("QualifierResponse = %s, want Rsp", QualifierResponse)
	}
	if QualifierTransmit != "Tx" {
		t.Errorf("QualifierTransmit = %s, want Tx", QualifierTransmit)
	}
}

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
		{
			name:     "endpoint feature",
			code:     Code{Side: SideServer, EndpointID: 1, Feature: "CTRL"},
			expected: "MASH.S.E01.CTRL",
		},
		{
			name:     "endpoint attribute",
			code:     Code{Side: SideServer, EndpointID: 1, Feature: "MEAS", Type: CodeTypeAttribute, ID: "01"},
			expected: "MASH.S.E01.MEAS.A01",
		},
		{
			name:     "endpoint feature flag",
			code:     Code{Side: SideServer, EndpointID: 2, Feature: "CTRL", Type: CodeTypeFlag, ID: "02"},
			expected: "MASH.S.E02.CTRL.F02",
		},
		{
			name:     "endpoint command with qualifier",
			code:     Code{Side: SideServer, EndpointID: 1, Feature: "CTRL", Type: CodeTypeCommand, ID: "01", Qualifier: QualifierResponse},
			expected: "MASH.S.E01.CTRL.C01.Rsp",
		},
		{
			name:     "endpoint hex FF",
			code:     Code{Side: SideServer, EndpointID: 0xFF, Feature: "ELEC"},
			expected: "MASH.S.EFF.ELEC",
		},
		{
			name:     "endpoint zero is device-level",
			code:     Code{Side: SideServer, EndpointID: 0, Feature: "CTRL"},
			expected: "MASH.S.CTRL",
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

func TestEndpointPICS(t *testing.T) {
	ep := &EndpointPICS{
		ID:       1,
		Type:     "EV_CHARGER",
		Features: []string{"CTRL", "ELEC", "MEAS"},
	}

	if ep.ID != 1 {
		t.Errorf("ID = %d, want 1", ep.ID)
	}
	if ep.Type != "EV_CHARGER" {
		t.Errorf("Type = %s, want EV_CHARGER", ep.Type)
	}
	if len(ep.Features) != 3 {
		t.Errorf("len(Features) = %d, want 3", len(ep.Features))
	}
}

func TestPICS_Endpoints(t *testing.T) {
	p := NewPICS()

	if p.Endpoints == nil {
		t.Fatal("expected Endpoints to be initialized")
	}

	if len(p.Endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(p.Endpoints))
	}

	// Add an endpoint
	p.Endpoints[1] = &EndpointPICS{
		ID:       1,
		Type:     "INVERTER",
		Features: []string{"CTRL", "MEAS"},
	}

	if len(p.Endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(p.Endpoints))
	}
}

func TestPICS_EndpointHas(t *testing.T) {
	p := NewPICS()
	p.Side = SideServer
	p.Endpoints[1] = &EndpointPICS{
		ID:       1,
		Type:     "EV_CHARGER",
		Features: []string{"CTRL", "MEAS"},
	}
	// Simulate parsed entries
	p.ByCode["MASH.S.E01.CTRL.A01"] = Entry{
		Code:  Code{Side: SideServer, EndpointID: 1, Feature: "CTRL", Type: CodeTypeAttribute, ID: "01"},
		Value: Value{Bool: true, Raw: "1"},
	}

	if !p.EndpointHas(1, "MASH.S.E01.CTRL.A01") {
		t.Error("expected EndpointHas to return true for existing code")
	}
	if p.EndpointHas(1, "MASH.S.E01.CTRL.A02") {
		t.Error("expected EndpointHas to return false for missing code")
	}
	if p.EndpointHas(2, "MASH.S.E01.CTRL.A01") {
		t.Error("expected EndpointHas to return false for wrong endpoint")
	}
}

func TestPICS_EndpointType(t *testing.T) {
	p := NewPICS()
	p.Endpoints[1] = &EndpointPICS{ID: 1, Type: "BATTERY"}
	p.Endpoints[2] = &EndpointPICS{ID: 2, Type: "INVERTER"}

	if got := p.EndpointType(1); got != "BATTERY" {
		t.Errorf("EndpointType(1) = %s, want BATTERY", got)
	}
	if got := p.EndpointType(2); got != "INVERTER" {
		t.Errorf("EndpointType(2) = %s, want INVERTER", got)
	}
	if got := p.EndpointType(3); got != "" {
		t.Errorf("EndpointType(3) = %s, want empty", got)
	}
}

func TestPICS_EndpointIDs(t *testing.T) {
	p := NewPICS()
	p.Endpoints[3] = &EndpointPICS{ID: 3, Type: "BATTERY"}
	p.Endpoints[1] = &EndpointPICS{ID: 1, Type: "INVERTER"}
	p.Endpoints[2] = &EndpointPICS{ID: 2, Type: "EV_CHARGER"}

	ids := p.EndpointIDs()
	if len(ids) != 3 {
		t.Fatalf("len(EndpointIDs()) = %d, want 3", len(ids))
	}
	// Should be sorted
	if ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("EndpointIDs() = %v, want [1 2 3]", ids)
	}
}

func TestPICS_EndpointsWithFeature(t *testing.T) {
	p := NewPICS()
	p.Endpoints[1] = &EndpointPICS{ID: 1, Type: "EV_CHARGER", Features: []string{"CTRL", "MEAS", "CHRG"}}
	p.Endpoints[2] = &EndpointPICS{ID: 2, Type: "INVERTER", Features: []string{"CTRL", "MEAS"}}

	eps := p.EndpointsWithFeature("CHRG")
	if len(eps) != 1 {
		t.Fatalf("len(EndpointsWithFeature(CHRG)) = %d, want 1", len(eps))
	}
	if eps[0].ID != 1 {
		t.Errorf("EndpointsWithFeature(CHRG)[0].ID = %d, want 1", eps[0].ID)
	}

	eps = p.EndpointsWithFeature("CTRL")
	if len(eps) != 2 {
		t.Fatalf("len(EndpointsWithFeature(CTRL)) = %d, want 2", len(eps))
	}

	eps = p.EndpointsWithFeature("SIG")
	if len(eps) != 0 {
		t.Errorf("len(EndpointsWithFeature(SIG)) = %d, want 0", len(eps))
	}
}

func TestApplicationFeatures(t *testing.T) {
	expected := []string{"ELEC", "MEAS", "CTRL", "STAT", "INFO", "CHRG", "SIG", "TAR", "PLAN"}
	for _, f := range expected {
		if !ApplicationFeatures[f] {
			t.Errorf("ApplicationFeatures[%s] = false, want true", f)
		}
	}

	// Transport features should not be in the set
	transport := []string{"TRANS", "COMM", "CERT", "ZONE", "CONN", "DISC"}
	for _, f := range transport {
		if ApplicationFeatures[f] {
			t.Errorf("ApplicationFeatures[%s] = true, want false", f)
		}
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

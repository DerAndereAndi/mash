package inspect

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// createTestDevice creates an EVSE device for testing.
func createTestDevice() *model.Device {
	evse := examples.NewEVSE(examples.EVSEConfig{
		DeviceID:           "evse-test-123",
		VendorName:         "Test Vendor",
		ProductName:        "Test EVSE",
		SerialNumber:       "SN-12345",
		VendorID:           0x1234,
		ProductID:          0x0001,
		PhaseCount:         3,
		NominalVoltage:     230,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1380000,
	})
	return evse.Device()
}

func TestNewInspector(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	if insp == nil {
		t.Fatal("NewInspector returned nil")
	}
	if insp.Device() != device {
		t.Error("Device() should return the underlying device")
	}
}

func TestInspectorInspectDevice(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	tree := insp.InspectDevice()

	if tree == nil {
		t.Fatal("InspectDevice returned nil")
	}
	if tree.DeviceID != "evse-test-123" {
		t.Errorf("DeviceID = %q, want %q", tree.DeviceID, "evse-test-123")
	}
	if len(tree.Endpoints) < 2 {
		t.Errorf("Expected at least 2 endpoints (root + EVSE), got %d", len(tree.Endpoints))
	}
}

func TestInspectorInspectEndpoint(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Inspect endpoint 0 (device root)
	epInfo, err := insp.InspectEndpoint(0)
	if err != nil {
		t.Fatalf("InspectEndpoint(0) error: %v", err)
	}
	if epInfo.ID != 0 {
		t.Errorf("ID = %d, want 0", epInfo.ID)
	}
	if epInfo.Type != model.EndpointDeviceRoot {
		t.Errorf("Type = %v, want %v", epInfo.Type, model.EndpointDeviceRoot)
	}

	// Inspect endpoint 1 (EV charger)
	epInfo, err = insp.InspectEndpoint(1)
	if err != nil {
		t.Fatalf("InspectEndpoint(1) error: %v", err)
	}
	if epInfo.ID != 1 {
		t.Errorf("ID = %d, want 1", epInfo.ID)
	}
	if epInfo.Type != model.EndpointEVCharger {
		t.Errorf("Type = %v, want %v", epInfo.Type, model.EndpointEVCharger)
	}

	// Inspect non-existent endpoint
	_, err = insp.InspectEndpoint(99)
	if err == nil {
		t.Error("InspectEndpoint(99) should return error for non-existent endpoint")
	}
}

func TestInspectorInspectFeature(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Inspect Measurement feature on endpoint 1
	featInfo, err := insp.InspectFeature(1, uint8(model.FeatureMeasurement))
	if err != nil {
		t.Fatalf("InspectFeature error: %v", err)
	}
	if featInfo.Type != model.FeatureMeasurement {
		t.Errorf("Type = %v, want %v", featInfo.Type, model.FeatureMeasurement)
	}
	if len(featInfo.Attributes) == 0 {
		t.Error("Expected attributes in feature info")
	}

	// Non-existent feature
	_, err = insp.InspectFeature(1, 99)
	if err == nil {
		t.Error("InspectFeature should return error for non-existent feature")
	}
}

func TestInspectorReadAttribute(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Read phaseCount from Electrical feature
	path := &Path{
		EndpointID:  1,
		FeatureID:   uint8(model.FeatureElectrical),
		AttributeID: 1, // phaseCount
	}

	value, meta, err := insp.ReadAttribute(path)
	if err != nil {
		t.Fatalf("ReadAttribute error: %v", err)
	}
	if value == nil {
		t.Error("ReadAttribute returned nil value")
	}
	if meta == nil {
		t.Error("ReadAttribute returned nil metadata")
	}
	if phaseCount, ok := value.(uint8); !ok || phaseCount != 3 {
		t.Errorf("phaseCount = %v, want 3", value)
	}

	// Read non-existent attribute
	path.AttributeID = 9999
	_, _, err = insp.ReadAttribute(path)
	if err == nil {
		t.Error("ReadAttribute should return error for non-existent attribute")
	}
}

func TestInspectorReadAllAttributes(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Read all attributes from Electrical feature
	attrs, err := insp.ReadAllAttributes(1, uint8(model.FeatureElectrical))
	if err != nil {
		t.Fatalf("ReadAllAttributes error: %v", err)
	}
	if len(attrs) == 0 {
		t.Error("Expected attributes")
	}

	// Check that phaseCount is present
	if _, ok := attrs[1]; !ok {
		t.Error("Expected phaseCount (attr 1) in results")
	}
}

func TestInspectorWriteAttribute(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Try to write to a read-only attribute (should fail)
	path := &Path{
		EndpointID:  1,
		FeatureID:   uint8(model.FeatureElectrical),
		AttributeID: 1, // phaseCount - read-only
	}
	err := insp.WriteAttribute(path, uint8(2))
	if err == nil {
		t.Error("WriteAttribute to read-only attr should fail")
	}

	// Write to a writable attribute (if one exists)
	// Most EVSE attributes are read-only; let's test DeviceInfo label
	path = &Path{
		EndpointID:  0,
		FeatureID:   uint8(model.FeatureDeviceInfo),
		AttributeID: 31, // label - writable
	}
	err = insp.WriteAttribute(path, "Test Label")
	if err != nil {
		t.Errorf("WriteAttribute to writable attr failed: %v", err)
	}

	// Verify the write
	value, _, err := insp.ReadAttribute(path)
	if err != nil {
		t.Fatalf("ReadAttribute after write failed: %v", err)
	}
	if v, ok := value.(string); !ok || v != "Test Label" {
		t.Errorf("value = %v, want %q", value, "Test Label")
	}
}

func TestInspectorGetDeviceTree(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	tree := insp.InspectDevice()

	// Verify structure
	if tree.DeviceID == "" {
		t.Error("DeviceID should not be empty")
	}

	// Check endpoint 0 has DeviceInfo
	var foundDeviceInfo bool
	for _, ep := range tree.Endpoints {
		if ep.ID == 0 {
			for _, feat := range ep.Features {
				if feat.Type == model.FeatureDeviceInfo {
					foundDeviceInfo = true
					break
				}
			}
		}
	}
	if !foundDeviceInfo {
		t.Error("Endpoint 0 should have DeviceInfo feature")
	}
}

func TestInspectorPath(t *testing.T) {
	device := createTestDevice()
	insp := NewInspector(device)

	// Test reading via path string (uses name resolution)
	path, err := ParsePath("1/electrical/phasecount")
	if err != nil {
		t.Fatalf("ParsePath error: %v", err)
	}

	value, _, err := insp.ReadAttribute(path)
	if err != nil {
		t.Fatalf("ReadAttribute error: %v", err)
	}
	if value != uint8(3) {
		t.Errorf("phaseCount = %v, want 3", value)
	}
}

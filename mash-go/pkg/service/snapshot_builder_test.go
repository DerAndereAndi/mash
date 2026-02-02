package service

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

func TestBuildDeviceSnapshot_Basic(t *testing.T) {
	device := model.NewDevice("test-device-001", 0x1234, 0x0001)

	// Add DeviceInfo to root endpoint
	di := features.NewDeviceInfo()
	_ = di.SetDeviceID("test-device-001")
	_ = di.SetVendorName("Test Vendor")
	_ = di.SetSerialNumber("SN-12345")
	device.RootEndpoint().AddFeature(di.Feature)

	// Add EVSE endpoint with Electrical and Measurement
	evse := model.NewEndpoint(1, model.EndpointEVCharger, "Test EVSE")
	electrical := features.NewElectrical()
	_ = electrical.SetPhaseCount(3)
	_ = electrical.SetNominalVoltage(230)
	evse.AddFeature(electrical.Feature)

	measurement := features.NewMeasurement()
	evse.AddFeature(measurement.Feature)

	_ = device.AddEndpoint(evse)

	snap := buildDeviceSnapshot(device)
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	if snap.DeviceID != "test-device-001" {
		t.Errorf("DeviceID: got %q, want %q", snap.DeviceID, "test-device-001")
	}

	// 2 endpoints: root + EVSE
	if len(snap.Endpoints) != 2 {
		t.Fatalf("Endpoints: got %d, want 2", len(snap.Endpoints))
	}

	// Endpoint 0: DEVICE_ROOT
	ep0 := snap.Endpoints[0]
	if ep0.ID != 0 {
		t.Errorf("Endpoint 0 ID: got %d, want 0", ep0.ID)
	}
	if ep0.Type != uint8(model.EndpointDeviceRoot) {
		t.Errorf("Endpoint 0 Type: got 0x%02x, want 0x%02x", ep0.Type, uint8(model.EndpointDeviceRoot))
	}
	if len(ep0.Features) == 0 {
		t.Fatal("Endpoint 0 has no features")
	}
	// Should have DeviceInfo feature
	foundDeviceInfo := false
	for _, f := range ep0.Features {
		if f.ID == uint16(model.FeatureDeviceInfo) {
			foundDeviceInfo = true
			if len(f.AttributeList) == 0 {
				t.Error("DeviceInfo attributeList is empty")
			}
		}
	}
	if !foundDeviceInfo {
		t.Error("Endpoint 0 missing DeviceInfo feature")
	}

	// Endpoint 1: EV_CHARGER
	ep1 := snap.Endpoints[1]
	if ep1.ID != 1 {
		t.Errorf("Endpoint 1 ID: got %d, want 1", ep1.ID)
	}
	if ep1.Type != uint8(model.EndpointEVCharger) {
		t.Errorf("Endpoint 1 Type: got 0x%02x, want 0x%02x", ep1.Type, uint8(model.EndpointEVCharger))
	}
	if len(ep1.Features) != 2 {
		t.Fatalf("Endpoint 1 features: got %d, want 2", len(ep1.Features))
	}

	// Features should be sorted by type ID
	if ep1.Features[0].ID > ep1.Features[1].ID {
		t.Errorf("features not sorted: ID %d before ID %d", ep1.Features[0].ID, ep1.Features[1].ID)
	}

	// Each feature should have an attributeList
	for _, f := range ep1.Features {
		if len(f.AttributeList) == 0 {
			t.Errorf("feature 0x%02x has empty attributeList", f.ID)
		}
	}
}

func TestBuildDeviceSnapshot_EmptyDevice(t *testing.T) {
	device := model.NewDevice("minimal-device", 0x0001, 0x0001)

	snap := buildDeviceSnapshot(device)
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	if snap.DeviceID != "minimal-device" {
		t.Errorf("DeviceID: got %q, want %q", snap.DeviceID, "minimal-device")
	}

	// Should have exactly 1 endpoint (root, auto-created by NewDevice)
	if len(snap.Endpoints) != 1 {
		t.Fatalf("Endpoints: got %d, want 1", len(snap.Endpoints))
	}
	if snap.Endpoints[0].ID != 0 {
		t.Errorf("Endpoint ID: got %d, want 0", snap.Endpoints[0].ID)
	}
	if snap.Endpoints[0].Type != uint8(model.EndpointDeviceRoot) {
		t.Errorf("Endpoint Type: got 0x%02x, want 0x%02x", snap.Endpoints[0].Type, uint8(model.EndpointDeviceRoot))
	}
}

func TestBuildDeviceSnapshot_WithUseCases(t *testing.T) {
	device := model.NewDevice("uc-device", 0x1234, 0x0001)

	di := features.NewDeviceInfo()
	_ = di.SetDeviceID("uc-device")
	_ = di.SetUseCases([]*model.UseCaseDecl{
		{EndpointID: 1, ID: 100, Major: 1, Minor: 0, Scenarios: 0x07},
		{EndpointID: 1, ID: 101, Major: 1, Minor: 1, Scenarios: 0x03},
	})
	device.RootEndpoint().AddFeature(di.Feature)

	snap := buildDeviceSnapshot(device)
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	if len(snap.UseCases) != 2 {
		t.Fatalf("UseCases: got %d, want 2", len(snap.UseCases))
	}

	uc0 := snap.UseCases[0]
	if uc0.EndpointID != 1 || uc0.ID != 100 || uc0.Major != 1 || uc0.Minor != 0 || uc0.Scenarios != 0x07 {
		t.Errorf("UseCase[0]: got %+v", uc0)
	}

	uc1 := snap.UseCases[1]
	if uc1.EndpointID != 1 || uc1.ID != 101 || uc1.Major != 1 || uc1.Minor != 1 || uc1.Scenarios != 0x03 {
		t.Errorf("UseCase[1]: got %+v", uc1)
	}
}

func TestBuildDeviceSnapshot_WithLabel(t *testing.T) {
	device := model.NewDevice("label-device", 0x1234, 0x0001)

	ep := model.NewEndpoint(1, model.EndpointEVCharger, "My Wallbox")
	_ = device.AddEndpoint(ep)

	snap := buildDeviceSnapshot(device)
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	if len(snap.Endpoints) != 2 {
		t.Fatalf("Endpoints: got %d, want 2", len(snap.Endpoints))
	}

	ep1 := snap.Endpoints[1]
	if ep1.Label != "My Wallbox" {
		t.Errorf("Label: got %q, want %q", ep1.Label, "My Wallbox")
	}

	// Endpoint 0 should have empty label
	if snap.Endpoints[0].Label != "" {
		t.Errorf("Endpoint 0 Label: got %q, want empty", snap.Endpoints[0].Label)
	}
}

func TestBuildDeviceSnapshot_SpecVersion(t *testing.T) {
	device := model.NewDevice("spec-device", 0x1234, 0x0001)

	di := features.NewDeviceInfo()
	_ = di.SetDeviceID("spec-device")
	_ = di.SetSpecVersion("1.2")
	device.RootEndpoint().AddFeature(di.Feature)

	snap := buildDeviceSnapshot(device)
	if snap == nil {
		t.Fatal("snapshot is nil")
	}

	if snap.SpecVersion != "1.2" {
		t.Errorf("SpecVersion: got %q, want %q", snap.SpecVersion, "1.2")
	}
}

func TestBuildDeviceSnapshot_NilDevice(t *testing.T) {
	snap := buildDeviceSnapshot(nil)
	if snap != nil {
		t.Errorf("expected nil for nil device, got %+v", snap)
	}
}

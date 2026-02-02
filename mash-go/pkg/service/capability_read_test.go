package service

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// createCapReadDevice creates a device with DeviceInfo populated for
// capability read tests. It has:
//   - Endpoint 0 (DEVICE_ROOT) with DeviceInfo
//   - Endpoint 1 (EV_CHARGER) with Electrical and Measurement
func createCapReadDevice() *model.Device {
	device := model.NewDevice("cap-test-device", 0x1234, 0x0001)

	// Add DeviceInfo to root endpoint
	di := features.NewDeviceInfo()
	_ = di.SetDeviceID("cap-test-device")
	_ = di.SetSpecVersion("1.0")
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

	// Populate the endpoints attribute on DeviceInfo
	var epInfos []*model.EndpointInfo
	for _, ep := range device.Endpoints() {
		epInfos = append(epInfos, ep.Info())
	}
	_ = di.SetEndpoints(epInfos)

	return device
}

func TestReadRemoteCapabilities_Basic(t *testing.T) {
	device := createCapReadDevice()

	conn := newMockResponseConnection(device)
	session := NewDeviceSession("cap-test-device", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := readRemoteCapabilities(ctx, session)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	if snap.DeviceID != "cap-test-device" {
		t.Errorf("DeviceID: got %q, want %q", snap.DeviceID, "cap-test-device")
	}
	if snap.SpecVersion != "1.0" {
		t.Errorf("SpecVersion: got %q, want %q", snap.SpecVersion, "1.0")
	}

	// 2 endpoints: root + EVSE
	if len(snap.Endpoints) != 2 {
		t.Fatalf("Endpoints: got %d, want 2", len(snap.Endpoints))
	}

	// Endpoint 0: DEVICE_ROOT with DeviceInfo
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

	// Endpoint 1: EV_CHARGER with Electrical and Measurement
	ep1 := snap.Endpoints[1]
	if ep1.ID != 1 {
		t.Errorf("Endpoint 1 ID: got %d, want 1", ep1.ID)
	}
	if ep1.Type != uint8(model.EndpointEVCharger) {
		t.Errorf("Endpoint 1 Type: got 0x%02x, want 0x%02x", ep1.Type, uint8(model.EndpointEVCharger))
	}
	if ep1.Label != "Test EVSE" {
		t.Errorf("Endpoint 1 Label: got %q, want %q", ep1.Label, "Test EVSE")
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

func TestReadRemoteCapabilities_WithUseCases(t *testing.T) {
	device := createCapReadDevice()

	// Add use cases via DeviceInfo
	di, _ := device.RootEndpoint().GetFeature(model.FeatureDeviceInfo)
	diWrap := features.DeviceInfo{Feature: di}
	_ = diWrap.SetUseCases([]*model.UseCaseDecl{
		{EndpointID: 1, ID: 100, Major: 1, Minor: 0, Scenarios: 0x07},
		{EndpointID: 1, ID: 101, Major: 1, Minor: 1, Scenarios: 0x03},
	})

	conn := newMockResponseConnection(device)
	session := NewDeviceSession("uc-device", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := readRemoteCapabilities(ctx, session)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
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

func TestReadRemoteCapabilities_ReadFailure(t *testing.T) {
	// Use a connection that returns errors
	conn := &mockDeviceSessionConn{}
	session := NewDeviceSession("fail-device", conn)
	defer session.Close()
	// No SetOnMessage -- requests will time out

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	snap := readRemoteCapabilities(ctx, session)
	if snap != nil {
		t.Errorf("expected nil snapshot on read failure, got %+v", snap)
	}
}

func TestReadRemoteCapabilities_EmptyDevice(t *testing.T) {
	// Device with only endpoint 0 and DeviceInfo
	device := model.NewDevice("minimal-device", 0x0001, 0x0001)

	di := features.NewDeviceInfo()
	_ = di.SetDeviceID("minimal-device")
	device.RootEndpoint().AddFeature(di.Feature)

	// Populate endpoints attribute
	_ = di.SetEndpoints([]*model.EndpointInfo{device.RootEndpoint().Info()})

	conn := newMockResponseConnection(device)
	session := NewDeviceSession("minimal-device", conn)
	defer session.Close()
	conn.SetOnMessage(session.OnMessage)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snap := readRemoteCapabilities(ctx, session)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}

	if snap.DeviceID != "minimal-device" {
		t.Errorf("DeviceID: got %q, want %q", snap.DeviceID, "minimal-device")
	}

	if len(snap.Endpoints) != 1 {
		t.Fatalf("Endpoints: got %d, want 1", len(snap.Endpoints))
	}
	if snap.Endpoints[0].ID != 0 {
		t.Errorf("Endpoint ID: got %d, want 0", snap.Endpoints[0].ID)
	}
	if snap.Endpoints[0].Type != uint8(model.EndpointDeviceRoot) {
		t.Errorf("Endpoint Type: got 0x%02x, want 0x%02x", snap.Endpoints[0].Type, uint8(model.EndpointDeviceRoot))
	}

	if len(snap.UseCases) != 0 {
		t.Errorf("UseCases: expected empty, got %d", len(snap.UseCases))
	}
}

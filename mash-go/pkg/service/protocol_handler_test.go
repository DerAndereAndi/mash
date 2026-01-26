package service_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Feature IDs match model.FeatureType values
const (
	featureIDElectrical = uint8(model.FeatureElectrical) // 0x0001 = 1
	featureIDDeviceInfo = uint8(model.FeatureDeviceInfo) // 0x0006 = 6
)

// createTestDevice creates a device with DeviceInfo for testing.
func createTestDevice() *model.Device {
	device := model.NewDevice("test-device-001", 0x1234, 0x0001)

	// Add DeviceInfo to root endpoint
	deviceInfo := features.NewDeviceInfo()
	_ = deviceInfo.SetDeviceID("test-device-001")
	_ = deviceInfo.SetVendorName("Test Vendor")
	_ = deviceInfo.SetProductName("Test Product")
	_ = deviceInfo.SetSerialNumber("SN-12345")
	_ = deviceInfo.SetSoftwareVersion("1.0.0")
	device.RootEndpoint().AddFeature(deviceInfo.Feature)

	// Add an EVSE endpoint with Electrical and Measurement features
	evse := model.NewEndpoint(1, model.EndpointEVCharger, "Test EVSE")

	electrical := features.NewElectrical()
	_ = electrical.SetPhaseCount(3)
	_ = electrical.SetNominalVoltage(230)
	_ = electrical.SetNominalFrequency(50)
	_ = electrical.SetNominalMaxConsumption(22000000) // 22 kW
	evse.AddFeature(electrical.Feature)

	measurement := features.NewMeasurement()
	evse.AddFeature(measurement.Feature)

	_ = device.AddEndpoint(evse)

	return device
}

func TestProtocolHandler_HandleRead_DeviceInfo(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Create read request for DeviceInfo (endpoint 0, feature 1)
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload:    nil,
	}

	resp := handler.HandleRequest(req)

	// Verify response
	if resp.MessageID != req.MessageID {
		t.Errorf("MessageID mismatch: expected %d, got %d", req.MessageID, resp.MessageID)
	}

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got status %d", resp.Status)
	}

	// Verify payload contains device info attributes
	payload, ok := resp.Payload.(map[uint16]any)
	if !ok {
		t.Fatalf("Expected map[uint16]any payload, got %T", resp.Payload)
	}

	// Check for expected attributes (DeviceID, VendorName, etc.)
	if _, exists := payload[1]; !exists { // DeviceID attribute
		t.Error("Expected DeviceID (attribute 1) in response")
	}
	if _, exists := payload[2]; !exists { // VendorName attribute
		t.Error("Expected VendorName (attribute 2) in response")
	}
}

func TestProtocolHandler_HandleRead_SpecificAttributes(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Request only specific attributes
	req := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload: &wire.ReadPayload{
			AttributeIDs: []uint16{1, 2}, // Only DeviceID and VendorName
		},
	}

	resp := handler.HandleRequest(req)

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got status %d", resp.Status)
	}

	payload, ok := resp.Payload.(map[uint16]any)
	if !ok {
		t.Fatalf("Expected map[uint16]any payload, got %T", resp.Payload)
	}

	// Should have only the requested attributes
	if len(payload) > 2 {
		t.Errorf("Expected at most 2 attributes, got %d", len(payload))
	}
}

func TestProtocolHandler_HandleRead_InvalidEndpoint(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	req := &wire.Request{
		MessageID:  3,
		Operation:  wire.OpRead,
		EndpointID: 99, // Non-existent endpoint
		FeatureID:  featureIDDeviceInfo,
	}

	resp := handler.HandleRequest(req)

	if resp.IsSuccess() {
		t.Error("Expected error response for invalid endpoint")
	}

	if resp.Status != wire.StatusInvalidEndpoint {
		t.Errorf("Expected StatusInvalidEndpoint (%d), got %d", wire.StatusInvalidEndpoint, resp.Status)
	}
}

func TestProtocolHandler_HandleRead_InvalidFeature(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	req := &wire.Request{
		MessageID:  4,
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  99, // Non-existent feature
	}

	resp := handler.HandleRequest(req)

	if resp.IsSuccess() {
		t.Error("Expected error response for invalid feature")
	}

	if resp.Status != wire.StatusInvalidFeature {
		t.Errorf("Expected StatusInvalidFeature (%d), got %d", wire.StatusInvalidFeature, resp.Status)
	}
}

func TestProtocolHandler_HandleWrite(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Write request - try to write to a writable attribute
	// DeviceInfo attributes are generally read-only, so this should fail
	req := &wire.Request{
		MessageID:  5,
		Operation:  wire.OpWrite,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload: wire.WritePayload{
			1: "new-device-id", // Try to write DeviceID
		},
	}

	resp := handler.HandleRequest(req)

	// DeviceInfo attributes are read-only, so write should fail
	if resp.IsSuccess() {
		t.Error("Expected write to read-only attribute to fail")
	}
}

func TestProtocolHandler_HandleSubscribe(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	req := &wire.Request{
		MessageID:  6,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload: &wire.SubscribePayload{
			MinInterval: 1000,  // 1 second
			MaxInterval: 60000, // 60 seconds
		},
	}

	resp := handler.HandleRequest(req)

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got status %d", resp.Status)
	}

	// Verify subscription ID is returned
	payload, ok := resp.Payload.(*wire.SubscribeResponsePayload)
	if !ok {
		t.Fatalf("Expected *wire.SubscribeResponsePayload, got %T", resp.Payload)
	}

	if payload.SubscriptionID == 0 {
		t.Error("Expected non-zero subscription ID")
	}

	// Priming report should contain current values
	if payload.CurrentValues == nil {
		t.Error("Expected current values in priming report")
	}
}

func TestProtocolHandler_HandleUnsubscribe(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// First create a subscription
	subReq := &wire.Request{
		MessageID:  7,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload: &wire.SubscribePayload{
			MinInterval: 1000,
			MaxInterval: 60000,
		},
	}
	subResp := handler.HandleRequest(subReq)

	if !subResp.IsSuccess() {
		t.Fatalf("Failed to create subscription: status %d", subResp.Status)
	}

	subPayload := subResp.Payload.(*wire.SubscribeResponsePayload)

	// Now unsubscribe (endpoint 0, feature 0 indicates unsubscribe)
	unsubReq := &wire.Request{
		MessageID:  8,
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0, // 0 indicates unsubscribe
		Payload: &wire.UnsubscribePayload{
			SubscriptionID: subPayload.SubscriptionID,
		},
	}

	unsubResp := handler.HandleRequest(unsubReq)

	if !unsubResp.IsSuccess() {
		t.Errorf("Expected success response for unsubscribe, got status %d", unsubResp.Status)
	}
}

func TestProtocolHandler_HandleInvoke(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Try to invoke a command on DeviceInfo
	// DeviceInfo typically doesn't have commands, so this should fail
	req := &wire.Request{
		MessageID:  9,
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
		Payload: &wire.InvokePayload{
			CommandID: 1, // Non-existent command
		},
	}

	resp := handler.HandleRequest(req)

	// Should fail as DeviceInfo has no commands
	if resp.IsSuccess() {
		t.Error("Expected invoke on feature without commands to fail")
	}
}

func TestProtocolHandler_InvalidOperation(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	req := &wire.Request{
		MessageID:  10,
		Operation:  wire.Operation(99), // Invalid operation
		EndpointID: 0,
		FeatureID:  featureIDDeviceInfo,
	}

	resp := handler.HandleRequest(req)

	if resp.IsSuccess() {
		t.Error("Expected error response for invalid operation")
	}

	if resp.Status != wire.StatusUnsupported {
		t.Errorf("Expected StatusUnsupported (%d), got %d", wire.StatusUnsupported, resp.Status)
	}
}

func TestProtocolHandler_ReadElectrical(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Read Electrical feature on endpoint 1
	req := &wire.Request{
		MessageID:  11,
		Operation:  wire.OpRead,
		EndpointID: 1,                      // EVSE endpoint
		FeatureID:  featureIDElectrical,
	}

	resp := handler.HandleRequest(req)

	if !resp.IsSuccess() {
		t.Errorf("Expected success response, got status %d", resp.Status)
	}

	payload, ok := resp.Payload.(map[uint16]any)
	if !ok {
		t.Fatalf("Expected map[uint16]any payload, got %T", resp.Payload)
	}

	// Check for phaseCount attribute
	if phaseCount, exists := payload[1]; exists {
		if phaseCount != uint8(3) {
			t.Errorf("Expected phaseCount 3, got %v", phaseCount)
		}
	} else {
		t.Error("Expected phaseCount attribute in response")
	}
}

func TestProtocolHandler_ZoneID(t *testing.T) {
	device := createTestDevice()
	handler := service.NewProtocolHandler(device)

	// Set zone context
	handler.SetZoneID("zone-123")

	if handler.ZoneID() != "zone-123" {
		t.Errorf("Expected zone ID 'zone-123', got '%s'", handler.ZoneID())
	}
}

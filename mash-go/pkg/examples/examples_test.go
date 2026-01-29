package examples

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

func TestEVSECreation(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000, // 32A
		MinCurrentPerPhase: 6000,  // 6A
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	if evse.Device() == nil {
		t.Fatal("expected device to be created")
	}

	// Check device has correct structure
	if evse.Device().EndpointCount() != 2 {
		t.Errorf("expected 2 endpoints (root + charger), got %d", evse.Device().EndpointCount())
	}
}

func TestCEMCreation(t *testing.T) {
	cem := NewCEM(CEMConfig{
		DeviceID:     "PEN67890.CEM-001",
		VendorName:   "Test Vendor",
		ProductName:  "Test CEM",
		SerialNumber: "SN-CEM-001",
		VendorID:     67890,
		ProductID:    200,
	})

	if cem.Device() == nil {
		t.Fatal("expected device to be created")
	}

	// CEM only has root endpoint
	if cem.Device().EndpointCount() != 1 {
		t.Errorf("expected 1 endpoint (root), got %d", cem.Device().EndpointCount())
	}
}

func TestEVSECharging(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	// Simulate EV connecting
	evse.SimulateEVConnect(40, 80000000, features.EVDemandModeDynamic) // 40% SoC, 80 kWh

	// Simulate charging at 11 kW
	evse.SimulateCharging(11000000)

	if evse.GetCurrentPower() != 11000000 {
		t.Errorf("expected power 11000000, got %d", evse.GetCurrentPower())
	}

	// Disconnect
	evse.SimulateEVDisconnect()

	if evse.GetCurrentPower() != 0 {
		t.Errorf("expected power 0 after disconnect, got %d", evse.GetCurrentPower())
	}
}

// mockRequestSender captures requests for testing (one-way, discards responses)
type mockRequestSender struct {
	server *interaction.Server
}

func (m *mockRequestSender) Send(data []byte) error {
	// Decode the request
	req, err := wire.DecodeRequest(data)
	if err != nil {
		return err
	}

	// Process through server
	resp := m.server.HandleRequest(context.Background(), req)

	// The response would normally go back through the wire
	// For testing, we just verify it worked
	_ = resp
	return nil
}

// roundTripSender routes responses back to the client, enabling Subscribe/Read via interaction.Client.
type roundTripSender struct {
	server *interaction.Server
	client *interaction.Client
}

func (m *roundTripSender) Send(data []byte) error {
	req, err := wire.DecodeRequest(data)
	if err != nil {
		return err
	}

	resp := m.server.HandleRequest(context.Background(), req)

	// Route response back to client asynchronously (like a real transport would)
	go m.client.HandleResponse(resp)
	return nil
}

func TestZoneInteraction(t *testing.T) {
	// Create EVSE and its interaction server
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	evseServer := interaction.NewServer(evse.Device())

	// Track limit changes
	var receivedLimit *int64
	evse.OnLimitChanged(func(limit *int64) {
		if limit != nil {
			val := *limit
			receivedLimit = &val
		} else {
			receivedLimit = nil
		}
	})

	// Mark EVSE as accepting control
	evse.AcceptController()

	// Create CEM
	cem := NewCEM(CEMConfig{
		DeviceID:     "PEN67890.CEM-001",
		VendorName:   "Test Vendor",
		ProductName:  "Test CEM",
		SerialNumber: "SN-CEM-001",
		VendorID:     67890,
		ProductID:    200,
	})

	// Simulate connection by creating a client connected to the EVSE server
	// In a real implementation, this would go through SHIP transport
	mockSender := &mockRequestSender{server: evseServer}
	client := interaction.NewClient(mockSender)
	client.SetTimeout(5 * time.Second)

	// CEM connects to EVSE
	_, err := cem.ConnectDevice("PEN12345.EVSE-001", client)
	if err != nil {
		t.Fatalf("ConnectDevice failed: %v", err)
	}

	// Verify connection
	ids := cem.ConnectedDeviceIDs()
	if len(ids) != 1 || ids[0] != "PEN12345.EVSE-001" {
		t.Errorf("expected connected device PEN12345.EVSE-001, got %v", ids)
	}

	// Simulate EV connecting
	evse.SimulateEVConnect(40, 80000000, features.EVDemandModeDynamic)
	evse.SimulateCharging(22000000) // Full 22 kW

	if evse.GetCurrentPower() != 22000000 {
		t.Errorf("expected power 22000000, got %d", evse.GetCurrentPower())
	}

	// CEM sets a limit via the interaction model
	ctx := context.Background()

	// Direct command invocation since we're simulating the wire protocol
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpInvoke,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureEnergyControl),
		Payload: &wire.InvokePayload{
			CommandID: features.EnergyControlCmdSetLimit,
			Parameters: map[string]any{
				"consumptionLimit": int64(11000000), // 11 kW limit
				"cause":            uint8(features.LimitCauseLocalOptimization),
			},
		},
	}

	resp := evseServer.HandleRequest(ctx, req)
	if !resp.Status.IsSuccess() {
		t.Fatalf("SetLimit failed: %v", resp.Status)
	}

	// Verify the limit was applied
	if receivedLimit == nil {
		t.Fatal("expected limit to be set")
	}
	if *receivedLimit != 11000000 {
		t.Errorf("expected limit 11000000, got %d", *receivedLimit)
	}

	// EVSE should now respect the limit
	evse.SimulateCharging(22000000) // Try to charge at 22 kW
	if evse.GetCurrentPower() != 11000000 {
		t.Errorf("expected power limited to 11000000, got %d", evse.GetCurrentPower())
	}

	// Clear limit
	clearReq := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpInvoke,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureEnergyControl),
		Payload: &wire.InvokePayload{
			CommandID:  features.EnergyControlCmdClearLimit,
			Parameters: map[string]any{},
		},
	}

	resp = evseServer.HandleRequest(ctx, clearReq)
	if !resp.Status.IsSuccess() {
		t.Fatalf("ClearLimit failed: %v", resp.Status)
	}

	if receivedLimit != nil {
		t.Error("expected limit to be cleared")
	}

	// Disconnect
	err = cem.DisconnectDevice("PEN12345.EVSE-001")
	if err != nil {
		t.Fatalf("DisconnectDevice failed: %v", err)
	}

	if len(cem.ConnectedDeviceIDs()) != 0 {
		t.Error("expected no connected devices after disconnect")
	}
}

func TestReadMeasurements(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	evseServer := interaction.NewServer(evse.Device())

	// Set some measurement values
	evse.SimulateEVConnect(60, 80000000, features.EVDemandModeDynamic)
	evse.SimulateCharging(11000000)

	// Read measurements through interaction model
	ctx := context.Background()
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpRead,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureMeasurement),
		Payload:    []uint16{1}, // ACActivePower
	}

	resp := evseServer.HandleRequest(ctx, req)
	if !resp.Status.IsSuccess() {
		t.Fatalf("Read failed: %v", resp.Status)
	}

	attrs, ok := resp.Payload.(map[uint16]any)
	if !ok {
		t.Fatal("expected map payload")
	}

	power, ok := attrs[1].(int64)
	if !ok || power != 11000000 {
		t.Errorf("expected power 11000000, got %v", attrs[1])
	}
}

func TestSubscribeToMeasurements(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	evseServer := interaction.NewServer(evse.Device())

	// Track notifications
	var notifications []*wire.Notification
	evseServer.SetNotificationHandler(func(notif *wire.Notification) {
		notifications = append(notifications, notif)
	})

	// Set initial measurement
	evse.SimulateEVConnect(60, 80000000, features.EVDemandModeDynamic)
	evse.SimulateCharging(11000000)

	// Subscribe to measurements
	ctx := context.Background()
	req := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpSubscribe,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureMeasurement),
		Payload: &wire.SubscribePayload{
			AttributeIDs: []uint16{1}, // ACActivePower
			MinInterval:  0,           // No delay for testing
			MaxInterval:  60000,
		},
	}

	resp := evseServer.HandleRequest(ctx, req)
	if !resp.Status.IsSuccess() {
		t.Fatalf("Subscribe failed: %v", resp.Status)
	}

	subResp, ok := resp.Payload.(*wire.SubscribeResponsePayload)
	if !ok {
		t.Fatal("expected SubscribeResponsePayload")
	}

	if subResp.SubscriptionID == 0 {
		t.Error("expected non-zero subscription ID")
	}

	// Priming report should contain current value
	if subResp.CurrentValues == nil {
		t.Fatal("expected priming report")
	}

	power, ok := subResp.CurrentValues[1].(int64)
	if !ok || power != 11000000 {
		t.Errorf("expected priming report with power 11000000, got %v", subResp.CurrentValues[1])
	}

	// Verify subscription count
	if evseServer.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription, got %d", evseServer.SubscriptionCount())
	}
}

func TestSubscribeToEnergyControl(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	evseServer := interaction.NewServer(evse.Device())

	// Track notifications
	var notifications []*wire.Notification
	evseServer.SetNotificationHandler(func(notif *wire.Notification) {
		notifications = append(notifications, notif)
	})

	// Create CEM and connect
	cem := NewCEM(CEMConfig{
		DeviceID:     "PEN67890.CEM-001",
		VendorName:   "Test Vendor",
		ProductName:  "Test CEM",
		SerialNumber: "SN-CEM-001",
		VendorID:     67890,
		ProductID:    200,
	})

	sender := &roundTripSender{server: evseServer}
	client := interaction.NewClient(sender)
	sender.client = client
	client.SetTimeout(5 * time.Second)

	_, err := cem.ConnectDevice("PEN12345.EVSE-001", client)
	if err != nil {
		t.Fatalf("ConnectDevice failed: %v", err)
	}

	// Mark EVSE as accepting control
	evse.AcceptController()

	// Subscribe to EnergyControl on endpoint 1
	ctx := context.Background()
	if err := cem.SubscribeToEnergyControl(ctx, "PEN12345.EVSE-001", 1); err != nil {
		t.Fatalf("SubscribeToEnergyControl failed: %v", err)
	}

	// Verify priming report populated control state
	// AcceptController() sets state to CONTROLLED, so that's what we expect
	device := cem.GetDevice("PEN12345.EVSE-001")
	if device == nil {
		t.Fatal("expected device to exist")
	}
	if device.ControlState != features.ControlStateControlled {
		t.Errorf("expected CONTROLLED control state from priming report, got %s", device.ControlState)
	}

	// Verify subscription ID is tracked
	if len(device.SubscriptionIDs) != 1 {
		t.Errorf("expected 1 subscription ID, got %d", len(device.SubscriptionIDs))
	}

	// Verify server has the subscription
	if evseServer.SubscriptionCount() != 1 {
		t.Errorf("expected 1 subscription on server, got %d", evseServer.SubscriptionCount())
	}
}

func TestPauseResume(t *testing.T) {
	evse := NewEVSE(EVSEConfig{
		DeviceID:           "PEN12345.EVSE-001",
		VendorName:         "Test Vendor",
		ProductName:        "Test Charger",
		SerialNumber:       "SN-001",
		VendorID:           12345,
		ProductID:          100,
		PhaseCount:         3,
		NominalVoltage:     400,
		MaxCurrentPerPhase: 32000,
		MinCurrentPerPhase: 6000,
		NominalMaxPower:    22000000,
		NominalMinPower:    1400000,
	})

	evseServer := interaction.NewServer(evse.Device())

	// Start charging
	evse.SimulateEVConnect(40, 80000000, features.EVDemandModeDynamic)
	evse.SimulateCharging(11000000)

	ctx := context.Background()

	// Pause
	pauseReq := &wire.Request{
		MessageID:  1,
		Operation:  wire.OpInvoke,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureEnergyControl),
		Payload: &wire.InvokePayload{
			CommandID:  features.EnergyControlCmdPause,
			Parameters: map[string]any{},
		},
	}

	resp := evseServer.HandleRequest(ctx, pauseReq)
	if !resp.Status.IsSuccess() {
		t.Fatalf("Pause failed: %v", resp.Status)
	}

	// Power should be 0 after pause
	if evse.GetCurrentPower() != 0 {
		t.Errorf("expected power 0 after pause, got %d", evse.GetCurrentPower())
	}

	// Resume
	resumeReq := &wire.Request{
		MessageID:  2,
		Operation:  wire.OpInvoke,
		EndpointID: 1,
		FeatureID:  uint8(model.FeatureEnergyControl),
		Payload: &wire.InvokePayload{
			CommandID:  features.EnergyControlCmdResume,
			Parameters: map[string]any{},
		},
	}

	resp = evseServer.HandleRequest(ctx, resumeReq)
	if !resp.Status.IsSuccess() {
		t.Fatalf("Resume failed: %v", resp.Status)
	}

	// Simulate charging resuming
	evse.SimulateCharging(11000000)

	if evse.GetCurrentPower() != 11000000 {
		t.Errorf("expected power 11000000 after resume, got %d", evse.GetCurrentPower())
	}
}

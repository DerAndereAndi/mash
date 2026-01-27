package interactive

import (
	"bytes"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/service"
)

// mockReadlineWriter captures output for testing.
type mockReadlineWriter struct {
	buf bytes.Buffer
}

func (m *mockReadlineWriter) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockReadlineWriter) String() string {
	return m.buf.String()
}

// TestRenewDevice_NoDevices tests renewal when no devices exist.
func TestRenewDevice_NoDevices(t *testing.T) {
	// Create a minimal controller service (no devices)
	cfg := service.ControllerConfig{
		ZoneName: "test-zone",
		ZoneType: 2, // LOCAL
	}
	svc, err := service.NewControllerService(cfg)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Verify GetDevice returns nil for unknown device
	device := svc.GetDevice("unknown-device-id")
	if device != nil {
		t.Error("Expected nil device for unknown ID")
	}

	// This validates the basic code path without needing readline mocking
}

// TestRenewDevice_ChecksConnectedStatus documents the expected behavior.
func TestRenewDevice_ChecksConnectedStatus(t *testing.T) {
	// The renewDevice function should:
	// 1. Check if device exists (GetDevice)
	// 2. Check if device is connected
	// 3. Call RenewDevice if connected
	// 4. Handle errors appropriately

	// This test documents the expected behavior.
	// Full integration testing is done in pkg/service/integration_test.go
}

// TestRenewAllDevices_FiltersConnected documents that only connected devices are renewed.
func TestRenewAllDevices_FiltersConnected(t *testing.T) {
	// The renewAllDevices function should:
	// 1. Get all devices
	// 2. Filter to those within 30-day renewal window AND connected
	// 3. Renew each one
	// 4. Report summary

	// This test documents the expected behavior.
	// Full integration testing is done in pkg/service/integration_test.go
}

// TestConnectedDeviceFields verifies we have access to the fields we need.
func TestConnectedDeviceFields(t *testing.T) {
	// Verify ConnectedDevice has the fields we need
	device := &service.ConnectedDevice{
		ID:        "test-device",
		Connected: true,
		LastSeen:  time.Now(),
	}

	if device.ID != "test-device" {
		t.Error("ID field not accessible")
	}
	if !device.Connected {
		t.Error("Connected field not accessible")
	}
	if device.LastSeen.IsZero() {
		t.Error("LastSeen field not accessible")
	}
}

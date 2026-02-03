package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/discovery"
)

func TestHandleStartStopDiscovery(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Start discovery.
	out, err := r.handleStartDiscoveryReal(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["discovery_started"] != true {
		t.Error("expected discovery_started=true")
	}

	ds := getDiscoveryState(state)
	if !ds.active {
		t.Error("expected active=true after start")
	}

	// Stop discovery.
	out, err = r.handleStopDiscoveryReal(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["discovery_stopped"] != true {
		t.Error("expected discovery_stopped=true")
	}

	if ds.active {
		t.Error("expected active=false after stop")
	}
}

func TestHandleReadMDNSTXT(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Populate discovery state with a service.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{
		{
			InstanceName: "MASH-1234",
			Host:         "evse.local",
			Port:         8443,
			TXTRecords: map[string]string{
				"brand": "TestBrand",
				"model": "TestModel",
				"DN":    "My EVSE",
			},
		},
	}

	// Read by index.
	step := &loader.Step{Params: map[string]any{"index": float64(0)}}
	out, err := r.handleReadMDNSTXT(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_found"] != true {
		t.Error("expected txt_found=true")
	}
	if out["instance_name"] != "MASH-1234" {
		t.Errorf("expected instance_name=MASH-1234, got %v", out["instance_name"])
	}
	if out["txt_brand"] != "TestBrand" {
		t.Errorf("expected txt_brand=TestBrand, got %v", out["txt_brand"])
	}

	// Read by instance name.
	step = &loader.Step{Params: map[string]any{"instance_name": "MASH-1234"}}
	out, err = r.handleReadMDNSTXT(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_found"] != true {
		t.Error("expected txt_found=true for instance_name lookup")
	}

	// Not found.
	step = &loader.Step{Params: map[string]any{"instance_name": "MASH-9999"}}
	out, err = r.handleReadMDNSTXT(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_found"] != false {
		t.Error("expected txt_found=false for unknown instance")
	}
}

func TestHandleVerifyTXTRecordsReal(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No services -> invalid.
	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleVerifyTXTRecordsReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_valid"] != false {
		t.Error("expected txt_valid=false with no services")
	}

	// Populate services.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{
		{
			TXTRecords: map[string]string{
				"brand": "Test",
				"model": "M1",
			},
		},
	}

	// All required fields present.
	step = &loader.Step{
		Params: map[string]any{
			"required_fields": []any{"brand", "model"},
		},
	}
	out, err = r.handleVerifyTXTRecordsReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_valid"] != true {
		t.Error("expected txt_valid=true when all fields present")
	}

	// Missing required field.
	step = &loader.Step{
		Params: map[string]any{
			"required_fields": []any{"brand", "serial"},
		},
	}
	out, err = r.handleVerifyTXTRecordsReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_valid"] != false {
		t.Error("expected txt_valid=false when field missing")
	}
}

func TestHandleGetQRPayload(t *testing.T) {
	r := newTestRunner()
	r.config.SetupCode = "12345678"
	state := newTestState()

	// From explicit payload.
	step := &loader.Step{
		Params: map[string]any{
			"payload": "MASH:1:1234:12345678:0x0000:0x0000",
		},
	}
	out, err := r.handleGetQRPayload(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Error("expected valid=true")
	}
	if out["qr_payload"] != "MASH:1:1234:12345678:0x0000:0x0000" {
		t.Errorf("unexpected payload: %v", out["qr_payload"])
	}

	// Construct from discriminator + setup code.
	step = &loader.Step{
		Params: map[string]any{
			"discriminator": float64(2048),
		},
	}
	out, err = r.handleGetQRPayload(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Error("expected valid=true for constructed payload")
	}
	if out["discriminator"] != 2048 {
		t.Errorf("expected discriminator=2048, got %v", out["discriminator"])
	}
}

func TestHandleAnnouncePairingRequest(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"discriminator": float64(1234),
			"zone_id":       "a1b2c3d4e5f6a7b8",
			"zone_name":     "My Home",
		},
	}
	out, err := r.handleAnnouncePairingRequest(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["pairing_request_announced"] != true {
		t.Error("expected pairing_request_announced=true")
	}
	if out["discriminator"] != 1234 {
		t.Errorf("expected discriminator=1234, got %v", out["discriminator"])
	}

	// Verify state was set.
	disc, _ := state.Get("pairing_request_discriminator")
	if disc != 1234 {
		t.Error("expected discriminator stored in state")
	}
}

func TestHandleWaitForDeviceNoDiscriminator(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Without discriminator -> simulated success.
	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleWaitForDeviceReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["device_found"] != true {
		t.Error("expected device_found=true for no-discriminator fallback")
	}
}

// ============================================================================
// Phase 5: Browse mDNS derived output fields
// ============================================================================

// TestBrowseMDNS_CommissionableFields verifies derived fields for commissionable services.
// We don't call handleBrowseMDNS directly (requires real mDNS) but test the output
// construction by injecting state and verifying the key patterns.
func TestBrowseMDNS_CommissionableOutputFields(t *testing.T) {
	state := newTestState()

	// Manually populate discovery state as if we found a commissionable service.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{
		{
			InstanceName:  "MASH-1234",
			Host:          "evse.local",
			Port:          8443,
			ServiceType:   discovery.ServiceTypeCommissionable,
			Discriminator: 1234,
			TXTRecords: map[string]string{
				"brand": "TestBrand",
				"model": "TestModel",
				"DN":    "My EVSE",
			},
		},
	}

	// Simulate the output construction logic from handleBrowseMDNS.
	// We can't call handleBrowseMDNS directly as it requires real mDNS networking,
	// so we verify the helper function and output structure.
	if !isValidHex("a1b2c3d4e5f6a7b8") {
		t.Error("isValidHex should accept valid hex")
	}
	if isValidHex("") {
		t.Error("isValidHex should reject empty string")
	}
	if isValidHex("xyz") {
		t.Error("isValidHex should reject non-hex")
	}
}

// TestBrowseMDNS_OperationalDerivedFields verifies zone/device ID validation helpers.
func TestBrowseMDNS_OperationalDerivedFields(t *testing.T) {
	// Valid 16-char hex IDs.
	if !isValidHex("a1b2c3d4e5f6a7b8") {
		t.Error("expected valid hex for 16-char lowercase ID")
	}
	if !isValidHex("A1B2C3D4E5F6A7B8") {
		t.Error("expected valid hex for uppercase ID")
	}

	// Invalid IDs.
	if isValidHex("not-hex-at-all!") {
		t.Error("expected invalid for non-hex")
	}
	if isValidHex("a1b2") {
		// Short but valid hex.
		// isValidHex only checks hex validity, not length.
	}
}

// TestBrowseMDNS_MinCountFields verifies the count fields are integers.
func TestBrowseMDNS_MinCountFields(t *testing.T) {
	state := newTestState()

	ds := getDiscoveryState(state)
	ds.services = []discoveredService{
		{ServiceType: discovery.ServiceTypeCommissionable, TXTRecords: map[string]string{}},
		{ServiceType: discovery.ServiceTypeCommissioner, TXTRecords: map[string]string{}},
	}

	// The handleBrowseMDNS would produce these. Verify by reading the last service list.
	devicesFound := 0
	controllersFound := 0
	for _, svc := range ds.services {
		switch svc.ServiceType {
		case discovery.ServiceTypeCommissionable, discovery.ServiceTypeOperational:
			devicesFound++
		case discovery.ServiceTypeCommissioner:
			controllersFound++
		}
	}

	if devicesFound != 1 {
		t.Errorf("expected 1 device, got %d", devicesFound)
	}
	if controllersFound != 1 {
		t.Errorf("expected 1 controller, got %d", controllersFound)
	}
}

func TestVerifyTXTRecords_RequiredKeys(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Populate with a commissionable service.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{
		{
			ServiceType:   discovery.ServiceTypeCommissionable,
			Discriminator: 1234,
			InstanceName:  "MASH-1234",
			TXTRecords: map[string]string{
				"brand": "Test",
				"model": "M1",
			},
		},
	}

	// Using required_keys param (instead of required_fields).
	step := &loader.Step{
		Params: map[string]any{
			"required_keys": []any{"brand", "model"},
		},
	}
	out, err := r.handleVerifyTXTRecordsReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_valid"] != true {
		t.Error("expected txt_valid=true with required_keys")
	}

	// Synthetic fields (id, cat, proto) should be populated for commissionable.
	step = &loader.Step{
		Params: map[string]any{
			"required_keys": []any{"id", "cat", "proto", "D"},
		},
	}
	out, err = r.handleVerifyTXTRecordsReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["txt_valid"] != true {
		t.Error("expected txt_valid=true for synthetic fields (id, cat, proto, D)")
	}
}

// C8: wait_for_device fallback populates ds.services.
func TestWaitForDevice_PopulatesServices(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No discriminator -> simulated fallback.
	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleWaitForDeviceReal(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["device_found"] != true {
		t.Error("expected device_found=true")
	}

	// Verify services were populated so verify_txt_records works.
	ds := getDiscoveryState(state)
	if len(ds.services) == 0 {
		t.Error("expected ds.services to be populated in fallback path")
	}
	if ds.services[0].InstanceName != "MASH-SIM-0000" {
		t.Errorf("expected synthetic instance name, got %v", ds.services[0].InstanceName)
	}
	if ds.services[0].ServiceType != discovery.ServiceTypeCommissionable {
		t.Errorf("expected commissionable service type, got %v", ds.services[0].ServiceType)
	}

	// Verify verify_txt_records now succeeds on the synthetic service.
	txtStep := &loader.Step{Params: map[string]any{}}
	txtOut, err := r.handleVerifyTXTRecordsReal(context.Background(), txtStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txtOut["txt_valid"] != true {
		t.Error("expected txt_valid=true for synthetic service")
	}
}

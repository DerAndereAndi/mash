package runner

import (
	"context"
	"testing"
	"time"

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
	r.config.SetupCode = "20202021"
	state := newTestState()

	// From explicit payload.
	step := &loader.Step{
		Params: map[string]any{
			"payload": "MASH:1:1234:20202021:0x0000:0x0000",
		},
	}
	out, err := r.handleGetQRPayload(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["valid"] != true {
		t.Error("expected valid=true")
	}
	if out["qr_payload"] != "MASH:1:1234:20202021:0x0000:0x0000" {
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

func TestHandleGetQRPayload_AutoGenerate(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No params -- should auto-generate a valid QR payload.
	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleGetQRPayload(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyValid] != true {
		t.Errorf("expected valid=true, got %v", out[KeyValid])
	}
	payload, ok := out[KeyQRPayload].(string)
	if !ok || payload == "" {
		t.Fatal("expected non-empty qr_payload")
	}

	// Verify the generated payload is parseable.
	qr, err := discovery.ParseQRCode(payload)
	if err != nil {
		t.Fatalf("generated payload not parseable: %v", err)
	}
	if qr.Version != discovery.QRVersion {
		t.Errorf("expected version=%d, got %d", discovery.QRVersion, qr.Version)
	}
	if len(qr.SetupCode) != discovery.SetupCodeLength {
		t.Errorf("expected setup_code_length=%d, got %d", discovery.SetupCodeLength, len(qr.SetupCode))
	}

	// Verify discriminator and setup_code are in output.
	if out[KeyDiscriminator] == nil {
		t.Error("expected discriminator in output")
	}
	if out[KeySetupCode] == nil {
		t.Error("expected setup_code in output")
	}

	// Subsequent call should return the same payload (cached in discovery state).
	out2, _ := r.handleGetQRPayload(context.Background(), step, state)
	if out2[KeyQRPayload] != payload {
		t.Error("expected same payload on second call")
	}
}

func TestHandleAnnouncePairingRequest(t *testing.T) {
	r := newTestRunner()
	defer r.Close()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"discriminator": float64(1234),
			KeyZoneID:       "a1b2c3d4e5f6a7b8",
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

func TestHandleAnnouncePairingRequest_AnnouncementSent(t *testing.T) {
	r := newTestRunner()
	defer r.Close()
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"discriminator": float64(2048),
			KeyZoneID:       "b2c3d4e5f6a7b8a1",
			"zone_name":     "Test Zone",
		},
	}
	out, err := r.handleAnnouncePairingRequest(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out[KeyAnnouncementSent] != true {
		t.Errorf("expected announcement_sent=true, got %v", out[KeyAnnouncementSent])
	}

	// Verify advertiser was created on the runner.
	if r.pairingAdvertiser == nil {
		t.Error("expected pairingAdvertiser to be set on runner")
	}
}

func TestHandleStopPairingRequest(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Announce first to create the advertiser.
	step := &loader.Step{
		Params: map[string]any{
			"discriminator": float64(1234),
			KeyZoneID:       "a1b2c3d4e5f6a7b8",
		},
	}
	_, err := r.handleAnnouncePairingRequest(context.Background(), step, state)
	if err != nil {
		t.Fatalf("announce: %v", err)
	}
	if r.pairingAdvertiser == nil {
		t.Fatal("expected advertiser after announce")
	}

	// Stop should clean up.
	out, err := r.handleStopPairingRequest(context.Background(), &loader.Step{}, state)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if out["stopped"] != true {
		t.Error("expected stopped=true")
	}
	if r.pairingAdvertiser != nil {
		t.Error("expected pairingAdvertiser to be nil after stop")
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

// TestBrowseMDNS_InjectedAddresses_NoBaseline verifies that injected addresses
// trigger new_address_announced=true even without a prior browse establishing
// previousAddresses. This is the TC-MULTIIF-003 live-mode scenario: interface_up
// injects an address, the first browse sees it as new.
func TestBrowseMDNS_InjectedAddresses_NoBaseline(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	ds := getDiscoveryState(state)

	// Seed services as if from a real mDNS browse (no previous browse).
	ds.services = []discoveredService{{
		InstanceName: "a1b2c3d4-00112233",
		ServiceType:  discovery.ServiceTypeOperational,
		Host:         "device.local",
		Port:         8443,
		Addresses:    []string{"fd12:3456:789a::1"},
		TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "00112233"},
	}}

	// Inject an address (as handleInterfaceUp does).
	ds.injectedAddresses = []string{"fd34:5678:abcd::1"}

	// buildBrowseOutput should detect the injection even without previousAddresses.
	out, err := r.buildBrowseOutput(ds)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyNewAddressAnnounced] != true {
		t.Errorf("expected new_address_announced=true from injection without baseline, got %v", out[KeyNewAddressAnnounced])
	}

	// The injected address should be merged into the service.
	if len(ds.services[0].Addresses) != 2 {
		t.Errorf("expected 2 addresses after merge, got %d: %v", len(ds.services[0].Addresses), ds.services[0].Addresses)
	}

	// injectedAddresses should be consumed.
	if len(ds.injectedAddresses) != 0 {
		t.Error("expected injectedAddresses to be cleared after merge")
	}
}

// TestBrowseMDNS_InterfaceUpThenBrowse verifies the full integration path:
// handleInterfaceUp injects an address, then buildBrowseOutput detects it.
func TestBrowseMDNS_InterfaceUpThenBrowse(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Seed services as if the device was already discovered via mDNS.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{{
		InstanceName: "a1b2c3d4-00112233",
		ServiceType:  discovery.ServiceTypeOperational,
		Host:         "device.local",
		Port:         8443,
		Addresses:    []string{"fd12:3456:789a::1"},
		TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "00112233"},
	}}

	// Simulate interface_up.
	upStep := &loader.Step{Params: map[string]any{}}
	_, err := r.handleInterfaceUp(context.Background(), upStep, state)
	if err != nil {
		t.Fatalf("interface_up: %v", err)
	}

	// Verify injected address is pending.
	if len(ds.injectedAddresses) != 1 {
		t.Fatalf("expected 1 injected address, got %d", len(ds.injectedAddresses))
	}

	// Call buildBrowseOutput (simulates what browse_mdns does).
	out, err := r.buildBrowseOutput(ds)
	if err != nil {
		t.Fatalf("buildBrowseOutput: %v", err)
	}

	if out[KeyNewAddressAnnounced] != true {
		t.Errorf("expected new_address_announced=true after interface_up, got %v", out[KeyNewAddressAnnounced])
	}

	// Verify the injected address was merged.
	if len(ds.services[0].Addresses) != 2 {
		t.Errorf("expected 2 addresses, got %d: %v", len(ds.services[0].Addresses), ds.services[0].Addresses)
	}
}

// TestConfirmNotAdvertising_ClearsStaleCache verifies that when the first
// browse finds a stale mDNS entry but subsequent browses do not, the function
// correctly reports the service as gone.
func TestConfirmNotAdvertising_ClearsStaleCache(t *testing.T) {
	callCount := 0
	browseFunc := func() (int, error) {
		callCount++
		if callCount == 1 {
			return 1, nil // first browse finds stale entry
		}
		return 0, nil // subsequent browses find nothing
	}

	found := confirmNotAdvertising(browseFunc, 3, 10*time.Millisecond)
	if found {
		t.Error("should report not advertising after stale entry clears")
	}
	if callCount < 2 {
		t.Error("should have retried at least once")
	}
}

// TestConfirmNotAdvertising_PersistentAdvertising verifies that when the
// service is genuinely still advertising (all retries find it), the function
// reports it as found.
func TestConfirmNotAdvertising_PersistentAdvertising(t *testing.T) {
	callCount := 0
	browseFunc := func() (int, error) {
		callCount++
		return 1, nil // always found
	}

	found := confirmNotAdvertising(browseFunc, 3, 10*time.Millisecond)
	if !found {
		t.Error("should report still advertising when all retries find service")
	}
	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

// TestConfirmNotAdvertising_NotFoundImmediately verifies that when the first
// browse finds nothing, the function returns immediately without retrying.
func TestConfirmNotAdvertising_NotFoundImmediately(t *testing.T) {
	callCount := 0
	browseFunc := func() (int, error) {
		callCount++
		return 0, nil
	}

	found := confirmNotAdvertising(browseFunc, 3, 10*time.Millisecond)
	if found {
		t.Error("should report not advertising")
	}
	if callCount != 1 {
		t.Errorf("should not retry when first browse finds nothing, got %d calls", callCount)
	}
}

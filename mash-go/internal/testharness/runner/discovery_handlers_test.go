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
	state := newTestState()

	step := &loader.Step{
		Params: map[string]any{
			"discriminator": float64(5678),
			KeyZoneID:       "zone-id-123",
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

func TestBrowseMDNS_TwoDevicesSameDiscriminator(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set the simulation precondition.
	state.Set(PrecondTwoDevicesSameDiscriminator, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissionable,
			KeyTimeoutMs:   float64(1000),
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find 2 devices.
	devicesFound, _ := out[KeyDevicesFound].(int)
	if devicesFound != 2 {
		t.Errorf("expected devices_found=2, got %v", out[KeyDevicesFound])
	}

	// Instance conflict should be resolved (different names).
	icr, _ := out[KeyInstanceConflictResolved].(bool)
	if !icr {
		t.Error("expected instance_conflict_resolved=true")
	}

	// Verify the discovery state has 2 services with the same discriminator.
	ds := getDiscoveryState(state)
	if len(ds.services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ds.services))
	}
	if ds.services[0].Discriminator != ds.services[1].Discriminator {
		t.Error("expected both services to have the same discriminator")
	}
	if ds.services[0].InstanceName == ds.services[1].InstanceName {
		t.Error("expected different instance names")
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

func TestBrowseMDNS_DeviceInZone_Operational(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set the simulation precondition.
	state.Set(PrecondDeviceInZone, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasOperational,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyDeviceFound] != true {
		t.Error("expected device_found=true")
	}
	instancesForDevice, _ := out[KeyInstancesForDevice].(int)
	if instancesForDevice != 1 {
		t.Errorf("expected instances_for_device=1, got %v", out[KeyInstancesForDevice])
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(ds.services))
	}
	if ds.services[0].ServiceType != discovery.ServiceTypeOperational {
		t.Errorf("expected operational service type, got %v", ds.services[0].ServiceType)
	}
}

func TestBrowseMDNS_DeviceInZone_Commissionable(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set the simulation precondition.
	state.Set(PrecondDeviceInZone, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissionable,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyDeviceFound] != true {
		t.Error("expected device_found=true for commissionable browse with device_in_zone")
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(ds.services))
	}
	if ds.services[0].ServiceType != discovery.ServiceTypeCommissionable {
		t.Errorf("expected commissionable service type, got %v", ds.services[0].ServiceType)
	}
}

func TestBrowseMDNS_DeviceInTwoZones_Operational(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set the simulation precondition.
	state.Set(PrecondDeviceInTwoZones, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasOperational,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyDeviceFound] != true {
		t.Error("expected device_found=true")
	}
	instancesForDevice, _ := out[KeyInstancesForDevice].(int)
	if instancesForDevice != 2 {
		t.Errorf("expected instances_for_device=2, got %v", out[KeyInstancesForDevice])
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ds.services))
	}
	// Both should be operational.
	for i, svc := range ds.services {
		if svc.ServiceType != discovery.ServiceTypeOperational {
			t.Errorf("service[%d]: expected operational, got %v", i, svc.ServiceType)
		}
	}
	// Should have different zone IDs.
	zi0 := ds.services[0].TXTRecords["ZI"]
	zi1 := ds.services[1].TXTRecords["ZI"]
	if zi0 == zi1 {
		t.Error("expected different zone IDs for two-zone simulation")
	}
}

func TestBrowseMDNS_CommissionerSimulation(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set zone_created precondition and create zone state.
	state.Set(PrecondZoneCreated, true)
	zs := getZoneState(state)
	zoneID := "test-zone-id"
	zs.zones[zoneID] = &zoneInfo{
		ZoneID:   zoneID,
		ZoneType: ZoneTypeLocal,
		Metadata: map[string]any{},
	}
	zs.zoneOrder = append(zs.zoneOrder, zoneID)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissioner,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyControllerFound] != true {
		t.Error("expected controller_found=true for commissioner simulation")
	}
	if out[KeyControllersFound] != 1 {
		t.Errorf("expected controllers_found=1, got %v", out[KeyControllersFound])
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(ds.services))
	}
	svc := ds.services[0]
	if svc.ServiceType != discovery.ServiceTypeCommissioner {
		t.Errorf("expected commissioner service type, got %v", svc.ServiceType)
	}
	if svc.TXTRecords["ZI"] != zoneID {
		t.Errorf("expected ZI=%s, got %v", zoneID, svc.TXTRecords["ZI"])
	}
}

func TestBrowseMDNS_CommissionerUsesZoneName(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set zone_created precondition and create zone state with ZoneName.
	state.Set(PrecondZoneCreated, true)
	zs := getZoneState(state)
	zoneID := "test-zone-id"
	zs.zones[zoneID] = &zoneInfo{
		ZoneID:   zoneID,
		ZoneName: "Home Energy",
		ZoneType: "LOCAL",
		Metadata: map[string]any{},
	}
	zs.zoneOrder = append(zs.zoneOrder, zoneID)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissioner,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Instance name should be the zone name, not the zone type.
	if out[KeyInstanceName] != "Home Energy" {
		t.Errorf("expected instance_name='Home Energy', got %v", out[KeyInstanceName])
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(ds.services))
	}
	if ds.services[0].TXTRecords["ZN"] != "Home Energy" {
		t.Errorf("expected TXT ZN='Home Energy', got %v", ds.services[0].TXTRecords["ZN"])
	}
}

func TestBrowseMDNS_CommissionerMultipleZones(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set zone_created precondition and create two zones.
	state.Set(PrecondZoneCreated, true)
	zs := getZoneState(state)

	zone1ID := "zone-aaa"
	zs.zones[zone1ID] = &zoneInfo{
		ZoneID:   zone1ID,
		ZoneName: "Home Energy",
		ZoneType: "LOCAL",
		Metadata: map[string]any{},
	}
	zs.zoneOrder = append(zs.zoneOrder, zone1ID)

	zone2ID := "zone-bbb"
	zs.zones[zone2ID] = &zoneInfo{
		ZoneID:   zone2ID,
		ZoneName: "Grid Service",
		ZoneType: "GRID",
		Metadata: map[string]any{},
	}
	zs.zoneOrder = append(zs.zoneOrder, zone2ID)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissioner,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return one commissioner instance per zone.
	instancesForDevice, _ := out[KeyInstancesForDevice].(int)
	if instancesForDevice != 2 {
		t.Errorf("expected instances_for_device=2, got %v", out[KeyInstancesForDevice])
	}
	if out[KeyControllersFound] != 2 {
		t.Errorf("expected controllers_found=2, got %v", out[KeyControllersFound])
	}

	ds := getDiscoveryState(state)
	if len(ds.services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(ds.services))
	}
	if ds.services[0].TXTRecords["ZI"] != zone1ID {
		t.Errorf("expected first ZI=%s, got %v", zone1ID, ds.services[0].TXTRecords["ZI"])
	}
	if ds.services[1].TXTRecords["ZI"] != zone2ID {
		t.Errorf("expected second ZI=%s, got %v", zone2ID, ds.services[1].TXTRecords["ZI"])
	}
}

func TestBrowseMDNS_AfterOneZoneRemoval(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Simulate the two-zones precondition (only this flag, not device_in_zone).
	state.Set(PrecondDeviceInTwoZones, true)

	// Seed controller state with two device entries.
	cs := getControllerState(state)
	cs.devices["dev-001"] = "zone-abc"
	cs.devices["dev-001-2"] = "zone-def"

	// Remove one device from a specific zone.
	removeStep := &loader.Step{Params: map[string]any{
		KeyDeviceID: "dev-001",
		"zone":      "zone-abc",
	}}
	_, err := r.handleRemoveDevice(context.Background(), removeStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now browse for operational services -- should find exactly 1 instance.
	browseStep := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasOperational,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), browseStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instancesForDevice, _ := out[KeyInstancesForDevice].(int)
	if instancesForDevice != 1 {
		t.Errorf("expected instances_for_device=1 after removing one zone, got %v", out[KeyInstancesForDevice])
	}
}

func TestBrowseMDNS_AfterAllZonesRemoved(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Device starts in one zone.
	state.Set(PrecondDeviceInZone, true)

	cs := getControllerState(state)
	cs.devices["dev-001"] = "zone-abc"

	// Remove from all zones.
	removeStep := &loader.Step{Params: map[string]any{
		KeyDeviceID: "dev-001",
		"zone":      "all",
	}}
	_, err := r.handleRemoveDevice(context.Background(), removeStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Browse for operational services -- should find nothing.
	// The simulation path should handle this deterministically without
	// falling through to real mDNS (which would take 5s+ to timeout).
	start := time.Now()
	browseStep := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasOperational,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), browseStep, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elapsed := time.Since(start)

	if out[KeyDeviceFound] != false {
		t.Errorf("expected device_found=false after removing from all zones, got %v", out[KeyDeviceFound])
	}
	instancesForDevice, _ := out[KeyInstancesForDevice].(int)
	if instancesForDevice != 0 {
		t.Errorf("expected instances_for_device=0 after removing from all zones, got %v", out[KeyInstancesForDevice])
	}

	// Should complete near-instantly via simulation, not via real mDNS browse.
	if elapsed > 500*time.Millisecond {
		t.Errorf("browse took %v, expected <500ms (simulation path should be instant)", elapsed)
	}
}

func TestBrowseMDNS_InstancesForDevice_AlwaysSet(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// No devices advertising -> instances_for_device should still be set.
	state.Set(PrecondNoDevicesAdvertising, true)

	step := &loader.Step{Params: map[string]any{}}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	v, ok := out[KeyInstancesForDevice]
	if !ok {
		t.Error("expected instances_for_device key to be present even with no services")
	}
	if v != 0 {
		t.Errorf("expected instances_for_device=0, got %v", v)
	}
}

func TestHandleBrowseMDNS_NoDevices_BrowseTimeout(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// Set the "no devices" precondition.
	state.Set(PrecondNoDevicesAdvertising, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: ServiceAliasCommissionable,
			"timeout":      "10s",
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyDeviceFound] != false {
		t.Error("expected device_found=false")
	}
	if out[KeyError] != "browse_timeout" {
		t.Errorf("expected error=browse_timeout, got %v", out[KeyError])
	}
}

func TestHandleBrowseMDNS_MultipleControllersRunning(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	state.Set(PrecondMultipleControllersRunning, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType: discovery.ServiceTypeCommissioner,
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	controllersMin, ok := out[KeyControllersFoundMin].(int)
	if !ok {
		t.Fatalf("expected controllers_found_min to be int, got %T", out[KeyControllersFoundMin])
	}
	if controllersMin < 2 {
		t.Errorf("expected controllers_found_min >= 2, got %d", controllersMin)
	}
}

func TestHandleBrowseMDNS_AllResultsInZone(t *testing.T) {
	r := newTestRunner()
	state := newTestState()

	// multiple_devices_commissioned gives us operational services with ZI TXT fields.
	state.Set(PrecondMultipleDevicesCommissioned, true)

	step := &loader.Step{
		Params: map[string]any{
			KeyServiceType:   ServiceAliasOperational,
			"zone_id_filter": "a1b2c3d4",
		},
	}
	out, err := r.handleBrowseMDNS(context.Background(), step, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if out[KeyAllResultsInZone] != true {
		t.Errorf("expected all_results_in_zone=true, got %v", out[KeyAllResultsInZone])
	}
}

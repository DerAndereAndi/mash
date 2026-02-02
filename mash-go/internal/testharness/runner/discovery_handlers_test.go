package runner

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
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

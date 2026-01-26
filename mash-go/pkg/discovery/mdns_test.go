package discovery_test

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
)

// TestMDNSAdvertiserCreate verifies the advertiser can be created with default config.
func TestMDNSAdvertiserCreate(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()
}

// TestMDNSAdvertiserCommissionable verifies advertising a commissionable device.
func TestMDNSAdvertiserCommissionable(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	ctx := context.Background()
	info := &discovery.CommissionableInfo{
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "ABC123",
		Brand:         "TestCo",
		Model:         "Model1",
		Port:          8443,
	}

	err = adv.AdvertiseCommissionable(ctx, info)
	if err != nil {
		t.Fatalf("Failed to advertise commissionable: %v", err)
	}

	// Stop should work without error
	err = adv.StopCommissionable()
	if err != nil {
		t.Errorf("Failed to stop commissionable: %v", err)
	}
}

// TestMDNSAdvertiserOperational verifies advertising an operational device.
func TestMDNSAdvertiserOperational(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	ctx := context.Background()
	info := &discovery.OperationalInfo{
		ZoneID:        "A1B2C3D4E5F6A7B8",
		DeviceID:      "F9E8D7C6B5A49382",
		VendorProduct: "TestCo:Model1",
		Port:          8443,
	}

	err = adv.AdvertiseOperational(ctx, info)
	if err != nil {
		t.Fatalf("Failed to advertise operational: %v", err)
	}

	// Stop should work without error
	err = adv.StopOperational(info.ZoneID)
	if err != nil {
		t.Errorf("Failed to stop operational: %v", err)
	}
}

// TestMDNSAdvertiserMultipleZones verifies advertising multiple operational zones.
func TestMDNSAdvertiserMultipleZones(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	ctx := context.Background()

	// Zone 1
	info1 := &discovery.OperationalInfo{
		ZoneID:   "A1B2C3D4E5F6A7B8",
		DeviceID: "F9E8D7C6B5A49382",
		Port:     8443,
	}

	// Zone 2
	info2 := &discovery.OperationalInfo{
		ZoneID:   "1234567890ABCDEF",
		DeviceID: "F9E8D7C6B5A49382",
		Port:     8443,
	}

	err = adv.AdvertiseOperational(ctx, info1)
	if err != nil {
		t.Fatalf("Failed to advertise zone 1: %v", err)
	}

	err = adv.AdvertiseOperational(ctx, info2)
	if err != nil {
		t.Fatalf("Failed to advertise zone 2: %v", err)
	}

	// Stop zone 1
	err = adv.StopOperational(info1.ZoneID)
	if err != nil {
		t.Errorf("Failed to stop zone 1: %v", err)
	}

	// Zone 2 should still be active, stopping it should work
	err = adv.StopOperational(info2.ZoneID)
	if err != nil {
		t.Errorf("Failed to stop zone 2: %v", err)
	}
}

// TestMDNSAdvertiserUpdateOperational verifies updating TXT records.
func TestMDNSAdvertiserUpdateOperational(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	ctx := context.Background()
	info := &discovery.OperationalInfo{
		ZoneID:        "A1B2C3D4E5F6A7B8",
		DeviceID:      "F9E8D7C6B5A49382",
		EndpointCount: 2,
		Port:          8443,
	}

	err = adv.AdvertiseOperational(ctx, info)
	if err != nil {
		t.Fatalf("Failed to advertise operational: %v", err)
	}

	// Update with new endpoint count
	updatedInfo := &discovery.OperationalInfo{
		ZoneID:        "A1B2C3D4E5F6A7B8",
		DeviceID:      "F9E8D7C6B5A49382",
		EndpointCount: 3,
		Port:          8443,
	}

	err = adv.UpdateOperational(info.ZoneID, updatedInfo)
	if err != nil {
		t.Errorf("Failed to update operational: %v", err)
	}
}

// TestMDNSAdvertiserCommissioner verifies advertising a commissioner service.
func TestMDNSAdvertiserCommissioner(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	ctx := context.Background()
	info := &discovery.CommissionerInfo{
		ZoneName:       "My Home",
		ZoneID:         "A1B2C3D4E5F6A7B8",
		VendorProduct:  "TestCo:Controller1",
		ControllerName: "Home Controller",
		DeviceCount:    5,
		Port:           8443,
	}

	err = adv.AdvertiseCommissioner(ctx, info)
	if err != nil {
		t.Fatalf("Failed to advertise commissioner: %v", err)
	}

	err = adv.StopCommissioner(info.ZoneID)
	if err != nil {
		t.Errorf("Failed to stop commissioner: %v", err)
	}
}

// TestMDNSBrowserCreate verifies the browser can be created with default config.
func TestMDNSBrowserCreate(t *testing.T) {
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()
}

// TestMDNSBrowserCommissionable verifies browsing for commissionable devices.
func TestMDNSBrowserCommissionable(t *testing.T) {
	// Start advertiser
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	advCtx := context.Background()
	info := &discovery.CommissionableInfo{
		Discriminator: 2345,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
		Serial:        "XYZ789",
		Brand:         "BrowseTest",
		Model:         "Model2",
		Port:          8443,
	}

	err = adv.AdvertiseCommissionable(advCtx, info)
	if err != nil {
		t.Fatalf("Failed to advertise: %v", err)
	}

	// Give mDNS time to propagate
	time.Sleep(500 * time.Millisecond)

	// Start browser
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := browser.BrowseCommissionable(ctx)
	if err != nil {
		t.Fatalf("Failed to browse: %v", err)
	}

	// Look for our device
	found := false
	for svc := range results {
		if svc.Discriminator == info.Discriminator {
			found = true
			if svc.Serial != info.Serial {
				t.Errorf("Serial mismatch: expected %q, got %q", info.Serial, svc.Serial)
			}
			if svc.Brand != info.Brand {
				t.Errorf("Brand mismatch: expected %q, got %q", info.Brand, svc.Brand)
			}
			break
		}
	}

	if !found {
		t.Error("Did not find advertised commissionable device")
	}
}

// TestMDNSBrowserOperational verifies browsing for operational devices.
func TestMDNSBrowserOperational(t *testing.T) {
	// Start advertiser
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	advCtx := context.Background()
	info := &discovery.OperationalInfo{
		ZoneID:        "BBBBCCCCDDDDEEEE",
		DeviceID:      "1111222233334444",
		VendorProduct: "OpTest:M1",
		Port:          8443,
	}

	err = adv.AdvertiseOperational(advCtx, info)
	if err != nil {
		t.Fatalf("Failed to advertise: %v", err)
	}

	// Give mDNS time to propagate
	time.Sleep(500 * time.Millisecond)

	// Start browser
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := browser.BrowseOperational(ctx, "")
	if err != nil {
		t.Fatalf("Failed to browse: %v", err)
	}

	// Look for our device
	found := false
	for svc := range results {
		if svc.ZoneID == info.ZoneID && svc.DeviceID == info.DeviceID {
			found = true
			if svc.VendorProduct != info.VendorProduct {
				t.Errorf("VendorProduct mismatch: expected %q, got %q", info.VendorProduct, svc.VendorProduct)
			}
			break
		}
	}

	if !found {
		t.Error("Did not find advertised operational device")
	}
}

// TestMDNSBrowserFindByDiscriminator verifies finding a specific device.
func TestMDNSBrowserFindByDiscriminator(t *testing.T) {
	// Start advertiser
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	advCtx := context.Background()
	info := &discovery.CommissionableInfo{
		Discriminator: 3456,
		Categories:    []discovery.DeviceCategory{discovery.CategoryInverter},
		Serial:        "FIND123",
		Brand:         "FindTest",
		Model:         "Inverter1",
		Port:          8443,
	}

	err = adv.AdvertiseCommissionable(advCtx, info)
	if err != nil {
		t.Fatalf("Failed to advertise: %v", err)
	}

	// Give mDNS time to propagate
	time.Sleep(500 * time.Millisecond)

	// Start browser
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	svc, err := browser.FindByDiscriminator(ctx, info.Discriminator)
	if err != nil {
		t.Fatalf("Failed to find device: %v", err)
	}

	if svc.Discriminator != info.Discriminator {
		t.Errorf("Discriminator mismatch: expected %d, got %d", info.Discriminator, svc.Discriminator)
	}
	if svc.Serial != info.Serial {
		t.Errorf("Serial mismatch: expected %q, got %q", info.Serial, svc.Serial)
	}
}

// TestMDNSBrowserFindByDiscriminatorTimeout verifies timeout when device not found.
func TestMDNSBrowserFindByDiscriminatorTimeout(t *testing.T) {
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	// Use short timeout since we expect it to fail
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = browser.FindByDiscriminator(ctx, 9999) // Non-existent discriminator
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestMDNSBrowserCommissioners verifies browsing for commissioner services.
func TestMDNSBrowserCommissioners(t *testing.T) {
	// Start advertiser
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	advCtx := context.Background()
	info := &discovery.CommissionerInfo{
		ZoneName:       "Test Zone",
		ZoneID:         "FFFFEEEEDDDCCCCB",
		VendorProduct:  "CommTest:C1",
		ControllerName: "Test Controller",
		DeviceCount:    3,
		Port:           8443,
	}

	err = adv.AdvertiseCommissioner(advCtx, info)
	if err != nil {
		t.Fatalf("Failed to advertise: %v", err)
	}

	// Give mDNS time to propagate
	time.Sleep(500 * time.Millisecond)

	// Start browser
	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		t.Fatalf("Failed to create browser: %v", err)
	}
	defer browser.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := browser.BrowseCommissioners(ctx)
	if err != nil {
		t.Fatalf("Failed to browse: %v", err)
	}

	// Look for our commissioner
	found := false
	for svc := range results {
		if svc.ZoneID == info.ZoneID {
			found = true
			if svc.ZoneName != info.ZoneName {
				t.Errorf("ZoneName mismatch: expected %q, got %q", info.ZoneName, svc.ZoneName)
			}
			if svc.DeviceCount != info.DeviceCount {
				t.Errorf("DeviceCount mismatch: expected %d, got %d", info.DeviceCount, svc.DeviceCount)
			}
			break
		}
	}

	if !found {
		t.Error("Did not find advertised commissioner")
	}
}

// TestMDNSAdvertiserStopNonexistent verifies stopping a non-existent service returns error.
func TestMDNSAdvertiserStopNonexistent(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	// Try to stop a service that was never started
	err = adv.StopOperational("nonexistent-zone")
	if err == nil {
		t.Error("Expected error when stopping non-existent service")
	}

	err = adv.StopCommissioner("nonexistent-zone")
	if err == nil {
		t.Error("Expected error when stopping non-existent commissioner")
	}
}

// TestMDNSAdvertiserUpdateNonexistent verifies updating a non-existent service returns error.
func TestMDNSAdvertiserUpdateNonexistent(t *testing.T) {
	adv, err := discovery.NewMDNSAdvertiser(discovery.DefaultAdvertiserConfig())
	if err != nil {
		t.Fatalf("Failed to create advertiser: %v", err)
	}
	defer adv.StopAll()

	info := &discovery.OperationalInfo{
		ZoneID:   "nonexistent",
		DeviceID: "device",
		Port:     8443,
	}

	err = adv.UpdateOperational("nonexistent", info)
	if err == nil {
		t.Error("Expected error when updating non-existent service")
	}
}

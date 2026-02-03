package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// TestDevice_PairingRequestListening_UncommissionedOpensWindow verifies that
// an uncommissioned device receiving a pairing request with matching discriminator
// opens its commissioning window.
func TestDevice_PairingRequestListening_UncommissionedOpensWindow(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.CommissioningWindowDuration = 100 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser that will invoke the callback with a matching pairing request
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// Simulate a pairing request with matching discriminator
			callback(discovery.PairingRequestService{
				InstanceName:  "A1B2C3D4E5F6A7B8-1234",
				Host:          "controller.local",
				Port:          0,
				Discriminator: config.Discriminator, // Match device's discriminator
				ZoneID:        "A1B2C3D4E5F6A7B8",
				ZoneName:      "Test Zone",
			})
			// Block until context is cancelled
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Track events
	var receivedEvent *Event
	var eventMu sync.Mutex
	eventCh := make(chan struct{}, 1)
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningOpened {
			eventCopy := e
			receivedEvent = &eventCopy
			select {
			case eventCh <- struct{}{}:
			default:
			}
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Wait for EventCommissioningOpened
	select {
	case <-eventCh:
		// Event received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for EventCommissioningOpened")
	}

	eventMu.Lock()
	defer eventMu.Unlock()

	if receivedEvent == nil {
		t.Fatal("expected to receive EventCommissioningOpened")
	}

	if receivedEvent.Type != EventCommissioningOpened {
		t.Errorf("expected EventCommissioningOpened, got %v", receivedEvent.Type)
	}
}

// TestDevice_PairingRequestListening_OneZoneOpensWindow verifies that
// a device with 1 zone (below max) receives a pairing request and opens its window.
func TestDevice_PairingRequestListening_OneZoneOpensWindow(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.CommissioningWindowDuration = 100 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up cert store with operational cert (simulates commissioned device)
	zoneID := "zone-001"
	certStore := createCertStoreWithOperationalCert(t, zoneID)
	svc.SetCertStore(certStore)

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Once()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// Wait a bit for zone to be connected, then send pairing request
			time.Sleep(50 * time.Millisecond)
			callback(discovery.PairingRequestService{
				Discriminator: config.Discriminator,
				ZoneID:        "B1B2C3D4E5F6A7B8",
				ZoneName:      "Another Zone",
			})
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Track events
	var receivedEvent *Event
	var eventMu sync.Mutex
	eventCh := make(chan struct{}, 1)
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningOpened {
			eventCopy := e
			receivedEvent = &eventCopy
			select {
			case eventCh <- struct{}{}:
			default:
			}
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect one zone (below max of 2)
	svc.HandleZoneConnect(zoneID, cert.ZoneTypeLocal)

	// Wait for EventCommissioningOpened
	select {
	case <-eventCh:
		// Event received
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for EventCommissioningOpened")
	}

	eventMu.Lock()
	defer eventMu.Unlock()

	if receivedEvent == nil {
		t.Fatal("expected to receive EventCommissioningOpened")
	}
}

// TestDevice_PairingRequestListening_MaxZonesIgnores verifies that
// a device with 2 zones (at max) ignores pairing requests.
func TestDevice_PairingRequestListening_MaxZonesIgnores(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.MaxZones = 2 // Production default: 1 GRID + 1 LOCAL // Ensure we use the actual max

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up cert store with operational certs for both zones (simulates commissioned device)
	zone1 := "zone-001"
	zone2 := "zone-002"
	certStore := createCertStoreWithOperationalCert(t, zone1)
	// Add second zone's operational cert
	ca2, _ := cert.GenerateZoneCA(zone2, cert.ZoneTypeGrid)
	kp2, _ := cert.GenerateKeyPair()
	csr2, _ := cert.CreateCSR(kp2, &cert.CSRInfo{
		Identity: cert.DeviceIdentity{DeviceID: "test-device", VendorID: 1, ProductID: 1},
		ZoneID:   zone2,
	})
	cert2, _ := cert.SignCSR(ca2, csr2)
	opCert2 := &cert.OperationalCert{
		Certificate: cert2,
		PrivateKey:  kp2.PrivateKey,
		ZoneID:      zone2,
		ZoneType:    cert.ZoneTypeGrid,
		ZoneCACert:  ca2.Certificate,
	}
	_ = certStore.SetOperationalCert(opCert2)
	svc.SetCertStore(certStore)

	// Set up mock advertiser - should NOT call AdvertiseCommissionable
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Times(2)
	// We explicitly do NOT expect AdvertiseCommissionable to be called
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser
	// BrowsePairingRequests IS called at start (with 0 zones), but once we reach
	// max zones, it stops and any pairing requests should be ignored.
	// We send a pairing request AFTER reaching max zones to verify it's ignored.
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// Wait until zones are connected, then send a pairing request
			// This should be ignored because we're at max zones
			time.Sleep(100 * time.Millisecond)
			callback(discovery.PairingRequestService{
				Discriminator: config.Discriminator,
				ZoneID:        "IGNORED_REQUEST",
			})
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Track events
	var gotCommissioningOpened bool
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningOpened {
			gotCommissioningOpened = true
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect max zones (2) - this should stop the pairing request listening
	svc.HandleZoneConnect(zone1, cert.ZoneTypeLocal)
	svc.HandleZoneConnect(zone2, cert.ZoneTypeGrid)

	// Wait for the pairing request to be sent and (hopefully) ignored
	time.Sleep(250 * time.Millisecond)

	eventMu.Lock()
	defer eventMu.Unlock()

	if gotCommissioningOpened {
		t.Error("should NOT have received EventCommissioningOpened when at max zones")
	}
}

// TestDevice_PairingRequestListening_RateLimiting verifies that
// the device ignores pairing requests when commissioning window is already open.
func TestDevice_PairingRequestListening_RateLimiting(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.CommissioningWindowDuration = 500 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - only ONE AdvertiseCommissionable call expected
	advertiseCount := 0
	var advertiseMu sync.Mutex
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ *discovery.CommissionableInfo) {
			advertiseMu.Lock()
			advertiseCount++
			advertiseMu.Unlock()
		}).Return(nil).Once() // Only once!
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser that sends multiple pairing requests
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// First request - should open window
			callback(discovery.PairingRequestService{
				Discriminator: config.Discriminator,
				ZoneID:        "A1B2C3D4E5F6A7B8",
			})
			// Wait a bit, then send second request (should be ignored - rate limited)
			time.Sleep(50 * time.Millisecond)
			callback(discovery.PairingRequestService{
				Discriminator: config.Discriminator,
				ZoneID:        "B1B2C3D4E5F6A7B9",
			})
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Wait for requests to be processed
	time.Sleep(200 * time.Millisecond)

	advertiseMu.Lock()
	count := advertiseCount
	advertiseMu.Unlock()

	if count != 1 {
		t.Errorf("expected AdvertiseCommissionable to be called once, got %d", count)
	}
}

// TestDevice_PairingRequestListening_DiscriminatorMismatch verifies that
// the device ignores pairing requests with non-matching discriminator.
func TestDevice_PairingRequestListening_DiscriminatorMismatch(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.Discriminator = 1234

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser - should NOT call AdvertiseCommissionable
	advertiser := mocks.NewMockAdvertiser(t)
	// We explicitly do NOT expect AdvertiseCommissionable to be called
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser that sends a non-matching discriminator
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// Send pairing request with different discriminator
			callback(discovery.PairingRequestService{
				Discriminator: 5678, // Different from device's 1234
				ZoneID:        "A1B2C3D4E5F6A7B8",
			})
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Track events
	var gotCommissioningOpened bool
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioningOpened {
			gotCommissioningOpened = true
		}
	})

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Wait a bit - should NOT receive EventCommissioningOpened
	time.Sleep(200 * time.Millisecond)

	eventMu.Lock()
	defer eventMu.Unlock()

	if gotCommissioningOpened {
		t.Error("should NOT have received EventCommissioningOpened for mismatched discriminator")
	}
}

// TestDevice_PairingRequestListening_StopOnMaxZones verifies that
// listening stops when the device reaches max zones.
func TestDevice_PairingRequestListening_StopOnMaxZones(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.MaxZones = 2 // Production default: 1 GRID + 1 LOCAL

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up cert store with operational certs for both zones
	zone1 := "zone-001"
	zone2 := "zone-002"
	certStore := createCertStoreWithOperationalCert(t, zone1)
	ca2, _ := cert.GenerateZoneCA(zone2, cert.ZoneTypeGrid)
	kp2, _ := cert.GenerateKeyPair()
	csr2, _ := cert.CreateCSR(kp2, &cert.CSRInfo{
		Identity: cert.DeviceIdentity{DeviceID: "test-device", VendorID: 1, ProductID: 1},
		ZoneID:   zone2,
	})
	cert2, _ := cert.SignCSR(ca2, csr2)
	opCert2 := &cert.OperationalCert{
		Certificate: cert2,
		PrivateKey:  kp2.PrivateKey,
		ZoneID:      zone2,
		ZoneType:    cert.ZoneTypeGrid,
		ZoneCACert:  ca2.Certificate,
	}
	_ = certStore.SetOperationalCert(opCert2)
	svc.SetCertStore(certStore)

	// Track if browser.Stop was called
	browserStopCalled := make(chan struct{}, 1)

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Times(2)
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			// Block until context is cancelled (meaning listening was stopped)
			<-ctx.Done()
		}).Return(nil).Once()
	browser.EXPECT().Stop().
		Run(func() {
			select {
			case browserStopCalled <- struct{}{}:
			default:
			}
		}).Return().Maybe()
	svc.SetBrowser(browser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Wait for browsing to start
	time.Sleep(50 * time.Millisecond)

	// Connect zones to reach max
	svc.HandleZoneConnect(zone1, cert.ZoneTypeLocal)
	svc.HandleZoneConnect(zone2, cert.ZoneTypeGrid)

	// Check that pairing request listening was stopped
	if svc.IsPairingRequestListening() {
		t.Error("expected pairing request listening to stop when max zones reached")
	}
}

// TestDevice_PairingRequestListening_DisabledByDefault verifies that
// pairing request listening is disabled when ListenForPairingRequests is false.
func TestDevice_PairingRequestListening_DisabledByDefault(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = false // Explicitly disabled

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Set up mock browser - should NOT call BrowsePairingRequests
	browser := mocks.NewMockBrowser(t)
	// No BrowsePairingRequests expectation - it should not be called
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Should not be listening
	if svc.IsPairingRequestListening() {
		t.Error("expected pairing request listening to be disabled")
	}
}

// TestDevice_PairingRequestListening_ResumeAfterZoneRemoved verifies that
// listening resumes when a zone is removed and device is below max.
func TestDevice_PairingRequestListening_ResumeAfterZoneRemoved(t *testing.T) {
	device := model.NewDevice("test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenForPairingRequests = true
	config.MaxZones = 2 // Production default: 1 GRID + 1 LOCAL

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Set up cert store with operational certs for both zones
	zone1 := "zone-001"
	zone2 := "zone-002"
	certStore := createCertStoreWithOperationalCert(t, zone1)
	ca2, _ := cert.GenerateZoneCA(zone2, cert.ZoneTypeGrid)
	kp2, _ := cert.GenerateKeyPair()
	csr2, _ := cert.CreateCSR(kp2, &cert.CSRInfo{
		Identity: cert.DeviceIdentity{DeviceID: "test-device", VendorID: 1, ProductID: 1},
		ZoneID:   zone2,
	})
	cert2, _ := cert.SignCSR(ca2, csr2)
	opCert2 := &cert.OperationalCert{
		Certificate: cert2,
		PrivateKey:  kp2.PrivateKey,
		ZoneID:      zone2,
		ZoneType:    cert.ZoneTypeGrid,
		ZoneCACert:  ca2.Certificate,
	}
	_ = certStore.SetOperationalCert(opCert2)
	svc.SetCertStore(certStore)

	// Set up mock advertiser
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Times(2)
	advertiser.EXPECT().StopOperational(mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track how many times BrowsePairingRequests is called
	browseCallCount := 0
	var browseMu sync.Mutex

	// Set up mock browser - may be called twice (start, stop at max, resume after removal)
	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().BrowsePairingRequests(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, callback func(discovery.PairingRequestService)) {
			browseMu.Lock()
			browseCallCount++
			browseMu.Unlock()
			<-ctx.Done()
		}).Return(nil).Maybe()
	browser.EXPECT().Stop().Return().Maybe()
	svc.SetBrowser(browser)

	// Start service
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Connect zones to reach max
	svc.HandleZoneConnect(zone1, cert.ZoneTypeLocal)
	svc.HandleZoneConnect(zone2, cert.ZoneTypeGrid)

	// Wait for listening to stop
	time.Sleep(50 * time.Millisecond)

	// Verify listening stopped
	if svc.IsPairingRequestListening() {
		t.Error("expected listening to stop at max zones")
	}

	// Remove a zone
	if err := svc.RemoveZone(zone1); err != nil {
		t.Fatalf("RemoveZone failed: %v", err)
	}

	// Wait for listening to resume
	time.Sleep(100 * time.Millisecond)

	// Should be listening again
	if !svc.IsPairingRequestListening() {
		t.Error("expected pairing request listening to resume after zone removal")
	}
}

package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/commissioning"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// TestDeviceServiceStartsTLSServer verifies that DeviceService starts a TLS server
// on Start() and accepts connections.
func TestDeviceServiceStartsTLSServer(t *testing.T) {
	device := model.NewDevice("test-device-001", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0" // Use random available port

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	// Use mock advertiser to avoid mDNS
	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Verify TLS server is running by checking the address
	addr := svc.TLSAddr()
	if addr == nil {
		t.Fatal("TLS server address is nil - server not started")
	}

	// Try to connect with commissioning TLS config
	tlsConfig := transport.NewCommissioningTLSConfig()
	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Failed to connect to TLS server: %v", err)
	}
	conn.Close()
}

// TestDeviceServicePASEHandshake verifies successful PASE handshake with correct setup code.
func TestDeviceServicePASEHandshake(t *testing.T) {
	device := model.NewDevice("test-device-002", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.SetupCode = "12345678"

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track events
	var connectedEvent bool
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventConnected {
			connectedEvent = true
		}
	})

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Connect and perform PASE handshake
	addr := svc.TLSAddr()
	if addr == nil {
		t.Fatal("TLS server address is nil")
	}

	tlsConfig := transport.NewCommissioningTLSConfig()
	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Perform PASE handshake with correct setup code
	// Use the fixed identities that match what the device verifier was set up with
	clientIdentity := []byte("mash-controller")
	serverIdentity := []byte("mash-device")

	session, err := commissioning.NewPASEClientSession(
		commissioning.SetupCode(12345678),
		clientIdentity,
		serverIdentity,
	)
	if err != nil {
		t.Fatalf("NewPASEClientSession failed: %v", err)
	}

	sharedSecret, err := session.Handshake(ctx, conn)
	if err != nil {
		t.Fatalf("PASE handshake failed: %v", err)
	}

	if len(sharedSecret) == 0 {
		t.Error("Shared secret should not be empty")
	}

	// Wait for event to be processed
	time.Sleep(100 * time.Millisecond)

	eventMu.Lock()
	connected := connectedEvent
	eventMu.Unlock()

	if !connected {
		t.Error("Expected EventConnected to be emitted")
	}
}

// TestDeviceServicePASEWrongSetupCode verifies PASE fails with wrong setup code.
func TestDeviceServicePASEWrongSetupCode(t *testing.T) {
	device := model.NewDevice("test-device-003", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.SetupCode = "12345678"

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	// Track events - should NOT see EventConnected
	var connectedEvent bool
	var eventMu sync.Mutex
	svc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventConnected {
			connectedEvent = true
		}
	})

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := svc.TLSAddr()
	if addr == nil {
		t.Fatal("TLS server address is nil")
	}

	tlsConfig := transport.NewCommissioningTLSConfig()
	conn, err := tls.Dial("tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Perform PASE handshake with WRONG setup code
	// Use the fixed identities that match what the device verifier was set up with
	clientIdentity := []byte("mash-controller")
	serverIdentity := []byte("mash-device")

	session, err := commissioning.NewPASEClientSession(
		commissioning.SetupCode(87654321), // Wrong code!
		clientIdentity,
		serverIdentity,
	)
	if err != nil {
		t.Fatalf("NewPASEClientSession failed: %v", err)
	}

	_, err = session.Handshake(ctx, conn)
	if err == nil {
		t.Error("Expected PASE handshake to fail with wrong setup code")
	}

	// Verify no connected event
	time.Sleep(100 * time.Millisecond)

	eventMu.Lock()
	connected := connectedEvent
	eventMu.Unlock()

	if connected {
		t.Error("EventConnected should NOT be emitted with wrong setup code")
	}
}

// TestControllerServiceCommission verifies successful commissioning flow.
func TestControllerServiceCommission(t *testing.T) {
	// Set up a device service to commission against
	device := model.NewDevice("test-device-004", 0x1234, 0x5678)
	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := mocks.NewMockAdvertiser(t)
	deviceAdvertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	deviceAdvertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	deviceAdvertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	deviceAdvertiser.EXPECT().StopAll().Return().Maybe()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx := context.Background()
	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	// Get device address
	addr := deviceSvc.TLSAddr()
	if addr == nil {
		t.Fatal("Device TLS address is nil")
	}

	// Set up controller service
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().Stop().Return().Maybe()
	controllerSvc.SetBrowser(browser)

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Track events
	var commissionedEvent bool
	var eventMu sync.Mutex
	controllerSvc.OnEvent(func(e Event) {
		eventMu.Lock()
		defer eventMu.Unlock()
		if e.Type == EventCommissioned {
			commissionedEvent = true
		}
	})

	// Create a discovery service result
	tcpAddr := addr.(*net.TCPAddr)
	discoveryService := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          uint16(tcpAddr.Port),
		Addresses:     []string{tcpAddr.IP.String()},
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	}

	// Commission with correct setup code
	connectedDevice, err := controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err != nil {
		t.Fatalf("Commission failed: %v", err)
	}

	if connectedDevice == nil {
		t.Fatal("Commission returned nil device")
	}

	if connectedDevice.ID == "" {
		t.Error("Device ID should not be empty")
	}

	// Wait for event
	time.Sleep(100 * time.Millisecond)

	eventMu.Lock()
	commissioned := commissionedEvent
	eventMu.Unlock()

	if !commissioned {
		t.Error("Expected EventCommissioned to be emitted")
	}

	// Verify device is stored
	if controllerSvc.DeviceCount() != 1 {
		t.Errorf("Expected 1 device, got %d", controllerSvc.DeviceCount())
	}
}

// TestControllerServiceCommissionWrongCode verifies commissioning fails with wrong code.
func TestControllerServiceCommissionWrongCode(t *testing.T) {
	// Set up a device service
	device := model.NewDevice("test-device-005", 0x1234, 0x5678)
	deviceConfig := validDeviceConfig()
	deviceConfig.ListenAddress = "localhost:0"
	deviceConfig.SetupCode = "12345678"

	deviceSvc, err := NewDeviceService(device, deviceConfig)
	if err != nil {
		t.Fatalf("NewDeviceService failed: %v", err)
	}

	deviceAdvertiser := mocks.NewMockAdvertiser(t)
	deviceAdvertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	deviceAdvertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	deviceAdvertiser.EXPECT().StopAll().Return().Maybe()
	deviceSvc.SetAdvertiser(deviceAdvertiser)

	ctx := context.Background()
	if err := deviceSvc.Start(ctx); err != nil {
		t.Fatalf("Device Start failed: %v", err)
	}
	defer func() { _ = deviceSvc.Stop() }()

	if err := deviceSvc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode failed: %v", err)
	}

	addr := deviceSvc.TLSAddr()
	if addr == nil {
		t.Fatal("Device TLS address is nil")
	}

	// Set up controller
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().Stop().Return().Maybe()
	controllerSvc.SetBrowser(browser)

	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Controller Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	tcpAddr := addr.(*net.TCPAddr)
	discoveryService := &discovery.CommissionableService{
		InstanceName:  "MASH-1234",
		Host:          "localhost",
		Port:          uint16(tcpAddr.Port),
		Addresses:     []string{tcpAddr.IP.String()},
		Discriminator: 1234,
		Categories:    []discovery.DeviceCategory{discovery.CategoryEMobility},
	}

	// Commission with WRONG setup code
	_, err = controllerSvc.Commission(ctx, discoveryService, "87654321")
	if err == nil {
		t.Error("Expected Commission to fail with wrong setup code")
	}

	if err != ErrCommissionFailed {
		t.Errorf("Expected ErrCommissionFailed, got: %v", err)
	}

	// Verify no device is stored
	if controllerSvc.DeviceCount() != 0 {
		t.Errorf("Expected 0 devices after failed commission, got %d", controllerSvc.DeviceCount())
	}
}

// TestDeriveZoneID verifies zone ID derivation is deterministic.
func TestDeriveZoneID(t *testing.T) {
	secret := []byte("test-shared-secret")

	id1 := deriveZoneID(secret)
	id2 := deriveZoneID(secret)

	if id1 != id2 {
		t.Errorf("Zone ID should be deterministic: %s != %s", id1, id2)
	}

	if len(id1) != 16 {
		t.Errorf("Zone ID should be 16 hex chars, got %d", len(id1))
	}

	// Different secrets should produce different IDs
	differentSecret := []byte("different-secret")
	id3 := deriveZoneID(differentSecret)

	if id1 == id3 {
		t.Error("Different secrets should produce different zone IDs")
	}
}

// TestDeriveDeviceID verifies device ID derivation is deterministic.
func TestDeriveDeviceID(t *testing.T) {
	secret := []byte("test-shared-secret")

	id1 := deriveDeviceID(secret)
	id2 := deriveDeviceID(secret)

	if id1 != id2 {
		t.Errorf("Device ID should be deterministic: %s != %s", id1, id2)
	}

	// Zone ID and Device ID should differ even with same secret
	zoneID := deriveZoneID(secret)
	if id1 == zoneID {
		t.Error("Device ID and Zone ID should differ")
	}
}

// TestGenerateSelfSignedCert verifies self-signed certificate generation.
func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := generateSelfSignedCert()
	if err != nil {
		t.Fatalf("generateSelfSignedCert failed: %v", err)
	}

	if len(cert.Certificate) == 0 {
		t.Error("Certificate should not be empty")
	}

	if cert.PrivateKey == nil {
		t.Error("Private key should not be nil")
	}
}

// TestCommissioningWithNotRunningDevice verifies error when device not started.
func TestCommissioningWithNotRunningDevice(t *testing.T) {
	controllerConfig := validControllerConfig()
	controllerSvc, err := NewControllerService(controllerConfig)
	if err != nil {
		t.Fatalf("NewControllerService failed: %v", err)
	}

	browser := mocks.NewMockBrowser(t)
	browser.EXPECT().Stop().Return().Maybe()
	controllerSvc.SetBrowser(browser)

	ctx := context.Background()
	if err := controllerSvc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = controllerSvc.Stop() }()

	// Try to commission a device that doesn't exist/respond
	discoveryService := &discovery.CommissionableService{
		Host:          "localhost",
		Port:          12345, // Non-existent port
		Addresses:     []string{"127.0.0.1"},
		Discriminator: 9999,
	}

	_, err = controllerSvc.Commission(ctx, discoveryService, "12345678")
	if err == nil {
		t.Error("Expected error when commissioning unreachable device")
	}
}

// Helper to print commission failure reason for debugging.
func debugCommissionError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Logf("Commission error details: %v", err)
	}
}

// assertEventWithTimeout waits for an event condition with timeout.
func assertEventWithTimeout(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("Timeout waiting for condition: %s", msg)
}

// Suppress unused import warning for fmt
var _ = fmt.Sprintf

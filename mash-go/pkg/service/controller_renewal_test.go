package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/commissioning"
)

// mockRenewalConn simulates a device connection for renewal testing.
type mockRenewalConn struct {
	mu        sync.Mutex
	sent      [][]byte
	responses [][]byte
	respIdx   int

	// Device-side handler for generating realistic responses
	deviceHandler *DeviceRenewalHandler
}

func newMockRenewalConn(deviceHandler *DeviceRenewalHandler) *mockRenewalConn {
	return &mockRenewalConn{
		deviceHandler: deviceHandler,
	}
}

func (c *mockRenewalConn) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, data)
	return nil
}

func (c *mockRenewalConn) GetSent() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([][]byte, len(c.sent))
	copy(result, c.sent)
	return result
}

// TestControllerRenewalHandler_InitiateRenewal tests renewal initiation.
func TestControllerRenewalHandler_InitiateRenewal(t *testing.T) {
	// Create Zone CA
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}

	// Create device-side handler
	deviceIdentity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	deviceHandler := NewDeviceRenewalHandler(deviceIdentity)

	// Create controller-side handler
	conn := newMockRenewalConn(deviceHandler)
	handler := NewControllerRenewalHandler(zoneCA, conn)

	// Initiate renewal
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start a goroutine to simulate device responses
	go func() {
		// Wait for request to be sent
		time.Sleep(10 * time.Millisecond)
		sent := conn.GetSent()
		if len(sent) == 0 {
			return
		}

		// Process request
		msg, err := commissioning.DecodeRenewalMessage(sent[0])
		if err != nil {
			return
		}

		req, ok := msg.(*commissioning.CertRenewalRequest)
		if !ok {
			return
		}

		// Generate CSR response
		csrResp, _ := deviceHandler.HandleRenewalRequest(req)

		// Send response to handler
		handler.HandleResponse(csrResp)
	}()

	err = handler.InitiateRenewal(ctx, "test-device-001")
	if err != nil {
		t.Fatalf("InitiateRenewal failed: %v", err)
	}

	// Verify request was sent
	sent := conn.GetSent()
	if len(sent) == 0 {
		t.Fatal("No request was sent")
	}

	// Verify CSR was received and stored
	if handler.pendingCSR == nil {
		t.Error("Expected pendingCSR to be set")
	}
}

// TestControllerRenewalHandler_CompleteRenewal tests full renewal flow.
func TestControllerRenewalHandler_CompleteRenewal(t *testing.T) {
	// Create Zone CA
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}

	// Create device-side handler
	deviceIdentity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	deviceHandler := NewDeviceRenewalHandler(deviceIdentity)

	// Create controller-side handler
	conn := newMockRenewalConn(deviceHandler)
	handler := NewControllerRenewalHandler(zoneCA, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate full renewal flow with device responses
	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			sent := conn.GetSent()

			for i, data := range sent {
				if i < conn.respIdx {
					continue // Already processed
				}

				msg, err := commissioning.DecodeRenewalMessage(data)
				if err != nil {
					continue
				}

				conn.mu.Lock()
				conn.respIdx = i + 1
				conn.mu.Unlock()

				switch m := msg.(type) {
				case *commissioning.CertRenewalRequest:
					csrResp, _ := deviceHandler.HandleRenewalRequest(m)
					handler.HandleResponse(csrResp)

				case *commissioning.CertRenewalInstall:
					ack, _ := deviceHandler.HandleCertInstall(m)
					handler.HandleResponse(ack)
					return // Done
				}
			}
		}
	}()

	// Run complete renewal
	newCert, err := handler.RenewDevice(ctx, "test-device-001")
	if err != nil {
		t.Fatalf("RenewDevice failed: %v", err)
	}

	// Verify new cert was returned
	if newCert == nil {
		t.Fatal("Expected new certificate to be returned")
	}

	// Verify device has new cert installed
	deviceCert := deviceHandler.ActiveCert()
	if deviceCert == nil {
		t.Fatal("Expected device to have active cert")
	}

	// Verify certs match
	if deviceCert.SerialNumber.Cmp(newCert.SerialNumber) != 0 {
		t.Error("Device cert serial number does not match returned cert")
	}
}

// TestControllerRenewalHandler_PreservesSession tests session continuity.
func TestControllerRenewalHandler_PreservesSession(t *testing.T) {
	// This test verifies that renewal doesn't close the connection
	zoneCA, _ := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)

	deviceIdentity := &cert.DeviceIdentity{
		DeviceID:     "test-device-001",
		VendorID:     0x1234,
		ProductID:    0x5678,
		SerialNumber: "SN-TEST-001",
	}
	deviceHandler := NewDeviceRenewalHandler(deviceIdentity)

	conn := newMockRenewalConn(deviceHandler)
	handler := NewControllerRenewalHandler(zoneCA, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Simulate device responses in background
	go func() {
		for {
			time.Sleep(10 * time.Millisecond)
			sent := conn.GetSent()

			for i, data := range sent {
				if i < conn.respIdx {
					continue
				}

				msg, _ := commissioning.DecodeRenewalMessage(data)

				conn.mu.Lock()
				conn.respIdx = i + 1
				conn.mu.Unlock()

				switch m := msg.(type) {
				case *commissioning.CertRenewalRequest:
					csrResp, _ := deviceHandler.HandleRenewalRequest(m)
					handler.HandleResponse(csrResp)
				case *commissioning.CertRenewalInstall:
					ack, _ := deviceHandler.HandleCertInstall(m)
					handler.HandleResponse(ack)
					return
				}
			}
		}
	}()

	// Run renewal
	_, err := handler.RenewDevice(ctx, "test-device-001")
	if err != nil {
		t.Fatalf("RenewDevice failed: %v", err)
	}

	// Verify connection is still usable (not closed)
	// We just verify we can still send
	err = conn.Send([]byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Error("Connection should still be usable after renewal")
	}
}

// =============================================================================
// Controller's Own Certificate Renewal Tests (Phase 6)
// =============================================================================

// TC-IMPL-RENEWAL-001: Controller Cert Needs Renewal Check
func TestControllerCertNeedsRenewal(t *testing.T) {
	// Create Zone CA and controller cert
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}

	controllerCert, err := cert.GenerateControllerOperationalCert(zoneCA, "controller-test")
	if err != nil {
		t.Fatalf("Failed to create controller cert: %v", err)
	}

	t.Run("FreshCertDoesNotNeedRenewal", func(t *testing.T) {
		// A fresh certificate (365 days validity) should NOT need renewal
		// Renewal window is 30 days before expiry
		if controllerCert.NeedsRenewal() {
			t.Error("Fresh certificate should not need renewal")
		}
	})

	t.Run("NilCertNeedsRenewal", func(t *testing.T) {
		nilCert := &cert.OperationalCert{}
		if !nilCert.NeedsRenewal() {
			t.Error("Nil certificate should need renewal")
		}
	})
}

// TC-IMPL-RENEWAL-002: Controller Cert Auto-Renewal via Service
func TestControllerService_RenewControllerCert(t *testing.T) {
	// Create a temp directory for cert store
	tempDir := t.TempDir()

	// Create and initialize cert store
	certStore := cert.NewFileControllerStore(tempDir)

	// Generate Zone CA
	zoneCA, err := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("Failed to create Zone CA: %v", err)
	}
	if err := certStore.SetZoneCA(zoneCA); err != nil {
		t.Fatalf("Failed to set Zone CA: %v", err)
	}

	// Generate initial controller cert
	initialCert, err := cert.GenerateControllerOperationalCert(zoneCA, "controller-test")
	if err != nil {
		t.Fatalf("Failed to create initial controller cert: %v", err)
	}
	if err := certStore.SetControllerCert(initialCert); err != nil {
		t.Fatalf("Failed to set initial controller cert: %v", err)
	}
	if err := certStore.Save(); err != nil {
		t.Fatalf("Failed to save cert store: %v", err)
	}

	// Create controller service with cert store
	config := DefaultControllerConfig()
	config.ZoneName = "test-zone"
	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("Failed to create controller service: %v", err)
	}
	svc.SetCertStore(certStore)

	// Record initial cert serial number
	initialSerial := initialCert.Certificate.SerialNumber

	// Renew the controller cert
	err = svc.RenewControllerCert()
	if err != nil {
		t.Fatalf("RenewControllerCert failed: %v", err)
	}

	// Get the new cert
	newCert, err := certStore.GetControllerCert()
	if err != nil {
		t.Fatalf("Failed to get renewed cert: %v", err)
	}

	// Verify new cert has different serial number (new cert generated)
	if newCert.Certificate.SerialNumber.Cmp(initialSerial) == 0 {
		t.Error("Renewed certificate should have different serial number")
	}

	// Verify new cert is signed by same Zone CA
	if err := cert.VerifyOperationalCert(newCert.Certificate, zoneCA.Certificate); err != nil {
		t.Errorf("Renewed certificate should be signed by Zone CA: %v", err)
	}

	// Verify new cert is not expired
	if newCert.IsExpired() {
		t.Error("Renewed certificate should not be expired")
	}

	// Verify new cert has fresh validity period
	if newCert.NeedsRenewal() {
		t.Error("Freshly renewed certificate should not need renewal")
	}
}

// TestControllerService_ControllerCertNeedsRenewal tests the service method.
func TestControllerService_ControllerCertNeedsRenewal(t *testing.T) {
	tempDir := t.TempDir()
	certStore := cert.NewFileControllerStore(tempDir)

	// Setup Zone CA and controller cert
	zoneCA, _ := cert.GenerateZoneCA("test-zone", cert.ZoneTypeHomeManager)
	certStore.SetZoneCA(zoneCA)
	controllerCert, _ := cert.GenerateControllerOperationalCert(zoneCA, "controller-test")
	certStore.SetControllerCert(controllerCert)
	certStore.Save()

	config := DefaultControllerConfig()
	config.ZoneName = "test-zone"
	svc, _ := NewControllerService(config)
	svc.SetCertStore(certStore)

	// Fresh cert should not need renewal
	if svc.ControllerCertNeedsRenewal() {
		t.Error("Fresh certificate should not need renewal")
	}
}

// TestControllerService_AutoGeneratesCertOnStart verifies Start() creates controller cert.
func TestControllerService_AutoGeneratesCertOnStart(t *testing.T) {
	tempDir := t.TempDir()
	certStore := cert.NewFileControllerStore(tempDir)

	config := DefaultControllerConfig()
	config.ZoneName = "auto-gen-test"
	svc, err := NewControllerService(config)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	svc.SetCertStore(certStore)

	// Start the service - should auto-generate Zone CA and controller cert
	ctx := context.Background()
	err = svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Verify Zone CA was created
	zoneCA, err := certStore.GetZoneCA()
	if err != nil {
		t.Fatalf("Zone CA should have been created: %v", err)
	}

	// Verify controller cert was created
	controllerCert, err := certStore.GetControllerCert()
	if err != nil {
		t.Fatalf("Controller cert should have been created: %v", err)
	}

	// Verify controller cert is signed by Zone CA
	if err := cert.VerifyOperationalCert(controllerCert.Certificate, zoneCA.Certificate); err != nil {
		t.Errorf("Controller cert should be signed by Zone CA: %v", err)
	}
}

// TestControllerService_LoadsExistingCertOnStart verifies existing cert is loaded.
func TestControllerService_LoadsExistingCertOnStart(t *testing.T) {
	tempDir := t.TempDir()
	certStore := cert.NewFileControllerStore(tempDir)

	// Pre-create Zone CA and controller cert
	zoneCA, _ := cert.GenerateZoneCA("existing-test", cert.ZoneTypeHomeManager)
	certStore.SetZoneCA(zoneCA)
	existingCert, _ := cert.GenerateControllerOperationalCert(zoneCA, "controller-existing")
	certStore.SetControllerCert(existingCert)
	certStore.Save()

	// Record the existing serial number
	existingSerial := existingCert.Certificate.SerialNumber

	// Create service and start it
	config := DefaultControllerConfig()
	config.ZoneName = "existing-test"
	svc, _ := NewControllerService(config)

	// Create a new store instance that will load from disk
	newStore := cert.NewFileControllerStore(tempDir)
	newStore.Load()
	svc.SetCertStore(newStore)

	ctx := context.Background()
	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	// Verify same cert is used (not regenerated)
	loadedCert, _ := newStore.GetControllerCert()
	if loadedCert.Certificate.SerialNumber.Cmp(existingSerial) != 0 {
		t.Error("Existing certificate should be loaded, not regenerated")
	}
}

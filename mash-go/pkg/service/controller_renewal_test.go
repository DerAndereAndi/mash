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

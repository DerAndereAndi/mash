package service

import (
	"context"
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// startReaperDevice creates and starts a DeviceService with short reaper timings.
func startReaperDevice(t *testing.T, staleTimeout, reaperInterval time.Duration) *DeviceService {
	t.Helper()

	device := model.NewDevice("reaper-test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.MaxZones = 2
	config.StaleConnectionTimeout = staleTimeout
	config.ReaperInterval = reaperInterval

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().AdvertiseOperational(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })

	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode: %v", err)
	}

	return svc
}

// reaperDialTLS opens a commissioning TLS connection to the device.
func reaperDialTLS(t *testing.T, addr net.Addr) *tls.Conn {
	t.Helper()
	tlsConfig := transport.NewCommissioningTLSConfig()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("reaperDialTLS: %v", err)
	}
	return conn
}

// TestStaleConnectionReaper_ClosesIdleConnection verifies that a TLS connection
// that sends no messages is closed after the stale timeout.
func TestStaleConnectionReaper_ClosesIdleConnection(t *testing.T) {
	svc := startReaperDevice(t, 500*time.Millisecond, 100*time.Millisecond)
	addr := svc.TLSAddr()

	// Open a TLS connection and send nothing.
	conn := reaperDialTLS(t, addr)
	defer conn.Close()

	// Wait for reaper to close it (stale timeout 500ms + reaper interval 100ms + margin).
	time.Sleep(800 * time.Millisecond)

	// Try to read -- should get an error because the connection was closed by the reaper.
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err == nil {
		t.Fatal("expected read error on reaped connection, got nil")
	}

	// ActiveConns should be 0.
	if !waitForActiveConns(svc, 0, 2*time.Second) {
		t.Errorf("ActiveConns after reap: got %d, want 0", svc.ActiveConns())
	}
}

// TestStaleConnectionReaper_DoesNotCloseOperationalConnections verifies that
// fully commissioned connections are not reaped.
func TestStaleConnectionReaper_DoesNotCloseOperationalConnections(t *testing.T) {
	svc := startReaperDevice(t, 500*time.Millisecond, 100*time.Millisecond)

	// Simulate an operational connection by adding a zone directly.
	// Operational connections are removed from the tracker before entering
	// the message loop, so the reaper should never see them.
	svc.HandleZoneConnect("zone-reaper-test", 2) // ZoneTypeLocal = 2

	// Wait past the stale timeout.
	time.Sleep(800 * time.Millisecond)

	// Zone should still be connected.
	zone := svc.GetZone("zone-reaper-test")
	if zone == nil {
		t.Fatal("zone should still exist after stale timeout")
	}
	if !zone.Connected {
		t.Error("zone should still be connected after stale timeout")
	}
}

// TestStaleConnectionReaper_ReaperStopsOnServiceStop verifies that stopping
// the service cleanly exits the reaper goroutine.
func TestStaleConnectionReaper_ReaperStopsOnServiceStop(t *testing.T) {
	device := model.NewDevice("reaper-stop-test", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.StaleConnectionTimeout = 1 * time.Second
	config.ReaperInterval = 50 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Stop should return cleanly without hanging on the reaper goroutine.
	done := make(chan struct{})
	go func() {
		_ = svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK - stopped cleanly
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() did not return within 5s - reaper may not have exited")
	}
}

// TestStaleConnectionReaper_DisabledWhenZero verifies that the reaper does not
// run when StaleConnectionTimeout is 0.
func TestStaleConnectionReaper_DisabledWhenZero(t *testing.T) {
	device := model.NewDevice("reaper-disabled-test", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.StaleConnectionTimeout = 0 // Disabled
	config.ReaperInterval = 50 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = svc.Stop() }()

	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode: %v", err)
	}

	addr := svc.TLSAddr()
	conn := reaperDialTLS(t, addr)
	defer conn.Close()

	// Wait longer than the reaper interval -- connection should still be alive.
	time.Sleep(200 * time.Millisecond)

	if svc.ActiveConns() == 0 {
		t.Error("connection should not have been reaped when StaleConnectionTimeout=0")
	}
}

// startReaperDeviceWithCategories is a helper for tests that need a
// fully-custom DeviceConfig.
func startReaperDeviceWithCategories(t *testing.T, categories []discovery.DeviceCategory) *DeviceService {
	t.Helper()
	device := model.NewDevice("reaper-custom-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.Categories = categories
	config.StaleConnectionTimeout = 500 * time.Millisecond
	config.ReaperInterval = 100 * time.Millisecond

	svc, err := NewDeviceService(device, config)
	if err != nil {
		t.Fatalf("NewDeviceService: %v", err)
	}

	advertiser := mocks.NewMockAdvertiser(t)
	advertiser.EXPECT().AdvertiseCommissionable(mock.Anything, mock.Anything).Return(nil).Maybe()
	advertiser.EXPECT().StopCommissionable().Return(nil).Maybe()
	advertiser.EXPECT().StopAll().Return().Maybe()
	svc.SetAdvertiser(advertiser)

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = svc.Stop() })
	return svc
}

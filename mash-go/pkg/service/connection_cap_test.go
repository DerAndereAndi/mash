package service

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/mash-protocol/mash-go/pkg/discovery/mocks"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/transport"
)

// startCappedDevice creates and starts a DeviceService in commissioning mode
// with the given MaxZones. Commissioning mode keeps connections alive during
// WaitForPASERequest, so we can observe the concurrent connection cap.
func startCappedDevice(t *testing.T, maxZones int) *DeviceService {
	t.Helper()

	device := model.NewDevice("cap-test-device", 0x1234, 0x5678)
	config := validDeviceConfig()
	config.ListenAddress = "localhost:0"
	config.MaxZones = maxZones

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

	// Enter commissioning mode so connections are held during WaitForPASERequest.
	if err := svc.EnterCommissioningMode(); err != nil {
		t.Fatalf("EnterCommissioningMode: %v", err)
	}

	return svc
}

// capDialTLS opens a commissioning TLS connection to the device.
func capDialTLS(t *testing.T, addr net.Addr) *tls.Conn {
	t.Helper()

	tlsConfig := transport.NewCommissioningTLSConfig()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr.String(), tlsConfig)
	if err != nil {
		t.Fatalf("capDialTLS: %v", err)
	}
	return conn
}

// capTryDialTLS attempts a TLS connection; returns conn and error.
func capTryDialTLS(addr net.Addr) (*tls.Conn, error) {
	tlsConfig := transport.NewCommissioningTLSConfig()
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	return tls.DialWithDialer(dialer, "tcp", addr.String(), tlsConfig)
}

// waitForActiveConns polls until ActiveConns reaches the expected value or timeout.
func waitForActiveConns(svc *DeviceService, expected int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if svc.ActiveConns() == expected {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestConnectionCapBasic verifies the cap at MaxZones+1.
// With MaxZones=2, the cap is 3: open 3 (succeed), 4th rejected, close 1, retry succeeds.
func TestConnectionCapBasic(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.CommissioningAddr()
	cap := int32(3) // MaxZones(2) + 1

	// Open cap connections -- all should succeed.
	conns := make([]*tls.Conn, 0, cap)
	for i := int32(0); i < cap; i++ {
		conn := capDialTLS(t, addr)
		conns = append(conns, conn)
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	// Wait for server to register all connections.
	if !waitForActiveConns(svc, cap, 2*time.Second) {
		t.Errorf("ActiveConns after %d connects = %d, want %d", cap, svc.ActiveConns(), cap)
	}

	// 4th connection should be rejected (TCP close before TLS).
	extra, err := capTryDialTLS(addr)
	if err == nil {
		_ = extra.Close()
		t.Fatal("4th connection should have been rejected but succeeded")
	}

	// Close one, retry should succeed.
	_ = conns[0].Close()
	conns = conns[1:]

	// Wait for decrement.
	if !waitForActiveConns(svc, cap-1, 2*time.Second) {
		t.Fatalf("ActiveConns after close = %d, want %d", svc.ActiveConns(), cap-1)
	}

	retry := capDialTLS(t, addr)
	conns = append(conns, retry)

	if !waitForActiveConns(svc, cap, 2*time.Second) {
		t.Errorf("ActiveConns after retry = %d, want %d", svc.ActiveConns(), cap)
	}
}

// TestConnectionCapFlood launches 100 concurrent connections and verifies
// that at most MaxZones+1 are accepted concurrently.
func TestConnectionCapFlood(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.CommissioningAddr()
	cap := int32(3)

	const numAttempts = 100
	var accepted int32
	var wg sync.WaitGroup
	var mu sync.Mutex
	var openConns []*tls.Conn

	for i := 0; i < numAttempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := capTryDialTLS(addr)
			if err != nil {
				return
			}
			atomic.AddInt32(&accepted, 1)
			mu.Lock()
			openConns = append(openConns, conn)
			mu.Unlock()
		}()
	}
	wg.Wait()

	defer func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range openConns {
			_ = c.Close()
		}
	}()

	got := atomic.LoadInt32(&accepted)
	if got > cap {
		t.Errorf("accepted connections = %d, want <= %d", got, cap)
	}
}

// TestConnectionCapDecrementOnTLSFailure verifies that a failed TLS handshake
// decrements the counter, freeing a cap slot.
func TestConnectionCapDecrementOnTLSFailure(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.CommissioningAddr()

	// Open a connection with an invalid TLS config (max TLS 1.2 won't match server's TLS 1.3).
	badConfig := &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS10,
		MaxVersion:         tls.VersionTLS12,
	}
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr.String(), badConfig)
	if err == nil {
		_ = conn.Close()
	}

	// Counter should return to 0 after the failed handshake.
	if !waitForActiveConns(svc, 0, 2*time.Second) {
		t.Errorf("ActiveConns after TLS failure = %d, want 0", svc.ActiveConns())
	}

	// Verify we can still open connections up to cap.
	cap := int32(3)
	conns := make([]*tls.Conn, 0, cap)
	for i := int32(0); i < cap; i++ {
		c := capDialTLS(t, addr)
		conns = append(conns, c)
	}
	defer func() {
		for _, c := range conns {
			_ = c.Close()
		}
	}()

	if !waitForActiveConns(svc, cap, 2*time.Second) {
		t.Errorf("ActiveConns = %d, want %d", svc.ActiveConns(), cap)
	}
}

// TestConnectionCapDecrementOnClose verifies that client close decrements the counter.
func TestConnectionCapDecrementOnClose(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.CommissioningAddr()

	conn := capDialTLS(t, addr)
	if !waitForActiveConns(svc, 1, 2*time.Second) {
		t.Errorf("ActiveConns after connect = %d, want 1", svc.ActiveConns())
	}

	_ = conn.Close()
	if !waitForActiveConns(svc, 0, 2*time.Second) {
		t.Errorf("ActiveConns after close = %d, want 0", svc.ActiveConns())
	}
}

// TestActiveConns verifies the ActiveConns accessor returns the correct count.
func TestActiveConns(t *testing.T) {
	svc := startCappedDevice(t, 2)
	addr := svc.CommissioningAddr()

	// Initially zero.
	if got := svc.ActiveConns(); got != 0 {
		t.Fatalf("ActiveConns initially = %d, want 0", got)
	}

	// Open two connections.
	c1 := capDialTLS(t, addr)
	defer func() { _ = c1.Close() }()
	c2 := capDialTLS(t, addr)
	defer func() { _ = c2.Close() }()

	if !waitForActiveConns(svc, 2, 2*time.Second) {
		t.Errorf("ActiveConns after 2 connects = %d, want 2", svc.ActiveConns())
	}

	// Close one.
	_ = c1.Close()
	if !waitForActiveConns(svc, 1, 2*time.Second) {
		t.Errorf("ActiveConns after 1 close = %d, want 1", svc.ActiveConns())
	}
}

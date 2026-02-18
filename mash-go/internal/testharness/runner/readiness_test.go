package runner

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/transport"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

func TestWaitForOperationalReadyOnConn_SuccessWithInterleavedFrames(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })

	conn := &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}

	var buffered int
	done := make(chan struct{})
	go func() {
		defer close(done)
		srvFramer := transport.NewFramer(server)

		reqFrame, err := srvFramer.ReadFrame()
		if err != nil {
			return
		}
		req, err := wire.DecodeRequest(reqFrame)
		if err != nil {
			return
		}

		// Notification frame should be buffered and ignored by readiness matcher.
		notifFrame, _ := wire.EncodeResponse(&wire.Response{
			MessageID: 0,
			Status:    wire.StatusSuccess,
		})
		_ = srvFramer.WriteFrame(notifFrame)

		// Orphaned response should be discarded.
		orphanFrame, _ := wire.EncodeResponse(&wire.Response{
			MessageID: req.MessageID + 99,
			Status:    wire.StatusSuccess,
		})
		_ = srvFramer.WriteFrame(orphanFrame)

		okFrame, _ := wire.EncodeResponse(&wire.Response{
			MessageID: req.MessageID,
			Status:    wire.StatusSuccess,
		})
		_ = srvFramer.WriteFrame(okFrame)
	}()

	err := waitForOperationalReadyOnConn(
		conn,
		2*time.Second,
		func() uint32 { return 42 },
		func([]byte) { buffered++ },
		nil,
	)
	if err != nil {
		t.Fatalf("waitForOperationalReadyOnConn failed: %v", err)
	}
	<-done
	if buffered != 1 {
		t.Fatalf("expected 1 buffered notification, got %d", buffered)
	}
}

func TestWaitForOperationalReadyOnConn_ReadFailureDisconnectsConn(t *testing.T) {
	server, client := net.Pipe()
	t.Cleanup(func() { _ = server.Close() })
	t.Cleanup(func() { _ = client.Close() })

	conn := &Connection{
		conn:   client,
		framer: transport.NewFramer(client),
		state:  ConnOperational,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		srvFramer := transport.NewFramer(server)
		_, _ = srvFramer.ReadFrame()
		_ = server.Close()
	}()

	err := waitForOperationalReadyOnConn(
		conn,
		500*time.Millisecond,
		func() uint32 { return 77 },
		nil,
		nil,
	)
	<-done
	if err == nil {
		t.Fatal("expected readiness error")
	}
	if conn.state != ConnDisconnected {
		t.Fatalf("expected disconnected conn after read failure, got state %v", conn.state)
	}
}

func TestWaitForCommissioningAvailable_UsesCurrentSnapshot(t *testing.T) {
	tb := newTestBrowser()
	r := newTestRunner()
	r.observer = newMDNSObserver(tb, func(string, ...any) {})
	t.Cleanup(r.stopObserver)

	// First call waits for an emitted commissionable service.
	go func() {
		time.Sleep(20 * time.Millisecond)
		tb.commAdded <- &discovery.CommissionableService{
			InstanceName:  "MASH-READY",
			Host:          "device.local",
			Port:          8443,
			Discriminator: 1234,
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := r.waitForCommissioningAvailable(ctx, 500*time.Millisecond); err != nil {
		t.Fatalf("first waitForCommissioningAvailable failed: %v", err)
	}

	// Second call should succeed immediately from existing snapshot state,
	// without requiring a new browse event.
	start := time.Now()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if err := r.waitForCommissioningAvailable(ctx2, 150*time.Millisecond); err != nil {
		t.Fatalf("second waitForCommissioningAvailable failed: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Fatalf("expected snapshot-based readiness, took too long: %v", elapsed)
	}
}

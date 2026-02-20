package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
)

func TestRequireCommissioningAvailable_SucceedsFromSnapshot(t *testing.T) {
	tb := newTestBrowser()
	r := newTestRunner()
	r.observer = newMDNSObserver(tb, func(string, ...any) {})
	t.Cleanup(r.stopObserver)

	// Seed observer with one commissionable service.
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
	if err := r.requireCommissioningAvailable(ctx, "phase-setup"); err != nil {
		t.Fatalf("requireCommissioningAvailable() unexpected error: %v", err)
	}
}

func TestRequireCommissioningAvailable_AnnotatesPhaseOnTimeout(t *testing.T) {
	tb := newTestBrowser()
	r := newTestRunner()
	r.observer = newMDNSObserver(tb, func(string, ...any) {})
	t.Cleanup(r.stopObserver)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := r.requireCommissioningAvailable(ctx, "two_zones_connected before LOCAL")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "two_zones_connected before LOCAL") {
		t.Fatalf("expected phase annotation in error, got: %v", err)
	}
}

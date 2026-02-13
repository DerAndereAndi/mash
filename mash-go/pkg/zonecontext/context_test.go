package zonecontext

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

func TestCallerZoneIDRoundTrip(t *testing.T) {
	ctx := ContextWithCallerZoneID(context.Background(), "z1")
	if got := CallerZoneIDFromContext(ctx); got != "z1" {
		t.Errorf("CallerZoneIDFromContext = %q, want %q", got, "z1")
	}
}

func TestCallerZoneTypeRoundTrip(t *testing.T) {
	ctx := ContextWithCallerZoneType(context.Background(), cert.ZoneTypeLocal)
	if got := CallerZoneTypeFromContext(ctx); got != cert.ZoneTypeLocal {
		t.Errorf("CallerZoneTypeFromContext = %v, want %v", got, cert.ZoneTypeLocal)
	}
}

func TestEmptyContextReturnsZeroValues(t *testing.T) {
	ctx := context.Background()
	if got := CallerZoneIDFromContext(ctx); got != "" {
		t.Errorf("CallerZoneIDFromContext on empty ctx = %q, want empty", got)
	}
	if got := CallerZoneTypeFromContext(ctx); got != 0 {
		t.Errorf("CallerZoneTypeFromContext on empty ctx = %v, want 0", got)
	}
}

func TestBothValuesCompose(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithCallerZoneID(ctx, "zone-abc")
	ctx = ContextWithCallerZoneType(ctx, cert.ZoneTypeGrid)

	if got := CallerZoneIDFromContext(ctx); got != "zone-abc" {
		t.Errorf("CallerZoneIDFromContext = %q, want %q", got, "zone-abc")
	}
	if got := CallerZoneTypeFromContext(ctx); got != cert.ZoneTypeGrid {
		t.Errorf("CallerZoneTypeFromContext = %v, want %v", got, cert.ZoneTypeGrid)
	}
}

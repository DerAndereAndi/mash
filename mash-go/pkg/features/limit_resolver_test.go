package features

import (
	"context"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

// testContext creates a context with zone ID and type injected via the
// same mechanism the LimitResolver uses (injected functions).
type testContextKey struct{ name string }

var (
	testZoneIDKey   = testContextKey{"zoneID"}
	testZoneTypeKey = testContextKey{"zoneType"}
)

func testCtx(zoneID string, zoneType cert.ZoneType) context.Context {
	ctx := context.WithValue(context.Background(), testZoneIDKey, zoneID)
	ctx = context.WithValue(ctx, testZoneTypeKey, zoneType)
	return ctx
}

func testZoneIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(testZoneIDKey).(string); ok {
		return v
	}
	return ""
}

func testZoneTypeFromCtx(ctx context.Context) cert.ZoneType {
	if v, ok := ctx.Value(testZoneTypeKey).(cert.ZoneType); ok {
		return v
	}
	return 0
}

func newTestResolver() *LimitResolver {
	ec := NewEnergyControl()
	_ = ec.SetControlState(ControlStateControlled)
	ec.SetCapabilities(true, false, false, false, false, false, false)

	lr := NewLimitResolver(ec)
	lr.ZoneIDFromContext = testZoneIDFromCtx
	lr.ZoneTypeFromContext = testZoneTypeFromCtx
	return lr
}

func intPtr(v int64) *int64 { return &v }

func TestLimitResolver_SingleZoneSetClear(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set consumption limit
	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})
	if !resp.Applied {
		t.Fatal("expected limit to be applied")
	}
	if resp.ControlState != ControlStateLimited {
		t.Fatalf("expected LIMITED, got %s", resp.ControlState)
	}
	if resp.EffectiveConsumptionLimit == nil || *resp.EffectiveConsumptionLimit != 5000000 {
		t.Fatal("expected effective consumption limit of 5000000")
	}

	// Clear limit
	err := lr.HandleClearLimit(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected CONTROLLED after clear, got %s", lr.ec.ControlState())
	}
	if _, ok := lr.ec.EffectiveConsumptionLimit(); ok {
		t.Fatal("expected no effective consumption limit after clear")
	}
}

func TestLimitResolver_TwoZonesMostRestrictiveWins(t *testing.T) {
	lr := newTestResolver()
	ctxGrid := testCtx("zone-GRID", cert.ZoneTypeGrid)
	ctxLocal := testCtx("zone-LOCAL", cert.ZoneTypeLocal)

	// GRID sets 6 kW
	lr.HandleSetLimit(ctxGrid, SetLimitRequest{
		ConsumptionLimit: intPtr(6000000),
	})

	// LOCAL sets 5 kW (more restrictive)
	resp := lr.HandleSetLimit(ctxLocal, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})

	if resp.EffectiveConsumptionLimit == nil || *resp.EffectiveConsumptionLimit != 5000000 {
		t.Fatalf("expected effective=5000000, got %v", resp.EffectiveConsumptionLimit)
	}
}

func TestLimitResolver_ClearPromotesRemaining(t *testing.T) {
	lr := newTestResolver()
	ctxGrid := testCtx("zone-GRID", cert.ZoneTypeGrid)
	ctxLocal := testCtx("zone-LOCAL", cert.ZoneTypeLocal)

	// GRID=6kW, LOCAL=5kW
	lr.HandleSetLimit(ctxGrid, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})
	lr.HandleSetLimit(ctxLocal, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Clear LOCAL -> effective should be GRID's 6kW
	_ = lr.HandleClearLimit(ctxLocal, nil)

	eff, ok := lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 6000000 {
		t.Fatalf("expected promoted limit 6000000, got %d (ok=%v)", eff, ok)
	}
	if lr.ec.ControlState() != ControlStateLimited {
		t.Fatalf("expected still LIMITED, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_DurationExpiry(t *testing.T) {
	lr := newTestResolver()
	ctxA := testCtx("zone-A", cert.ZoneTypeLocal)
	ctxB := testCtx("zone-B", cert.ZoneTypeGrid)

	// Zone B sets indefinite 6kW
	lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

	// Zone A sets 3kW with very short duration (1s is the minimum)
	dur := uint32(1)
	lr.HandleSetLimit(ctxA, SetLimitRequest{
		ConsumptionLimit: intPtr(3000000),
		Duration:         &dur,
	})

	// Initially zone A wins (3kW < 6kW)
	eff, ok := lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 3000000 {
		t.Fatalf("expected 3000000, got %d", eff)
	}

	// Wait for expiry
	time.Sleep(1500 * time.Millisecond)

	// After expiry, zone B's 6kW should be promoted
	eff, ok = lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 6000000 {
		t.Fatalf("after expiry, expected 6000000, got %d (ok=%v)", eff, ok)
	}
}

func TestLimitResolver_BothDirectionsIndependent(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set both directions
	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
		ProductionLimit:  intPtr(3000000),
	})
	if !resp.Applied {
		t.Fatal("expected applied")
	}

	effC, okC := lr.ec.EffectiveConsumptionLimit()
	effP, okP := lr.ec.EffectiveProductionLimit()
	if !okC || effC != 5000000 {
		t.Fatalf("consumption: expected 5000000, got %d", effC)
	}
	if !okP || effP != 3000000 {
		t.Fatalf("production: expected 3000000, got %d", effP)
	}
}

func TestLimitResolver_ClearByDirection(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set both
	lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
		ProductionLimit:  intPtr(3000000),
	})

	// Clear only consumption
	dirConsumption := DirectionConsumption
	_ = lr.HandleClearLimit(ctx, &dirConsumption)

	// Consumption should be nil, production should remain
	if _, ok := lr.ec.EffectiveConsumptionLimit(); ok {
		t.Fatal("expected consumption to be cleared")
	}
	effP, okP := lr.ec.EffectiveProductionLimit()
	if !okP || effP != 3000000 {
		t.Fatalf("production should remain, got %d (ok=%v)", effP, okP)
	}
	if lr.ec.ControlState() != ControlStateLimited {
		t.Fatalf("expected still LIMITED (production active), got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_ValidationNegativeValue(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(-1000),
	})
	if resp.Applied {
		t.Fatal("expected reject for negative value")
	}
	if resp.RejectReason == nil || *resp.RejectReason != LimitRejectInvalidValue {
		t.Fatal("expected INVALID_VALUE reject reason")
	}
}

func TestLimitResolver_ValidationOverride(t *testing.T) {
	lr := newTestResolver()
	_ = lr.ec.SetControlState(ControlStateOverride)
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})
	if resp.Applied {
		t.Fatal("expected reject in override state")
	}
	if resp.RejectReason == nil || *resp.RejectReason != LimitRejectDeviceOverride {
		t.Fatal("expected DEVICE_OVERRIDE reject reason")
	}
}

func TestLimitResolver_ValidationNoZoneID(t *testing.T) {
	lr := newTestResolver()
	ctx := context.Background() // no zone info

	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})
	if resp.Applied {
		t.Fatal("expected reject with no zone ID")
	}
}

func TestLimitResolver_ControlStateTransitions(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Initially CONTROLLED
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected CONTROLLED, got %s", lr.ec.ControlState())
	}

	// Set limit -> LIMITED
	lr.HandleSetLimit(ctx, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})
	if lr.ec.ControlState() != ControlStateLimited {
		t.Fatalf("expected LIMITED, got %s", lr.ec.ControlState())
	}

	// Clear -> CONTROLLED
	_ = lr.HandleClearLimit(ctx, nil)
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected CONTROLLED, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_NilLimitsDeactivate(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set a limit first
	lr.HandleSetLimit(ctx, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Send nil limits = deactivate this zone
	resp := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: nil,
		ProductionLimit:  nil,
	})
	if !resp.Applied {
		t.Fatal("expected applied (deactivation)")
	}
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected CONTROLLED after deactivation, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_ClearZone(t *testing.T) {
	lr := newTestResolver()
	ctxA := testCtx("zone-A", cert.ZoneTypeLocal)
	ctxB := testCtx("zone-B", cert.ZoneTypeGrid)

	lr.HandleSetLimit(ctxA, SetLimitRequest{ConsumptionLimit: intPtr(3000000)})
	lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

	// ClearZone removes all of zone A's limits
	lr.ClearZone("zone-A")

	eff, ok := lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 6000000 {
		t.Fatalf("expected zone B promoted to 6000000, got %d (ok=%v)", eff, ok)
	}
}

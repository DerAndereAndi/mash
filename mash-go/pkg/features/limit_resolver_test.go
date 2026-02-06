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
	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})
	if !resp.Applied {
		t.Fatal("expected limit to be applied")
	}
	if resp.ControlState != ControlStateControlled {
		t.Fatalf("expected CONTROLLED, got %s", resp.ControlState)
	}
	if resp.EffectiveConsumptionLimit == nil || *resp.EffectiveConsumptionLimit != 5000000 {
		t.Fatal("expected effective consumption limit of 5000000")
	}

	// Clear limit
	err := lr.HandleClearLimit(ctx, ClearLimitRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lr.ec.ControlState() != ControlStateAutonomous {
		t.Fatalf("expected AUTONOMOUS after clear, got %s", lr.ec.ControlState())
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
	_, _ = lr.HandleSetLimit(ctxGrid, SetLimitRequest{
		ConsumptionLimit: intPtr(6000000),
	})

	// LOCAL sets 5 kW (more restrictive)
	resp, _ := lr.HandleSetLimit(ctxLocal, SetLimitRequest{
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
	_, _ = lr.HandleSetLimit(ctxGrid, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})
	_, _ = lr.HandleSetLimit(ctxLocal, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Clear LOCAL -> effective should be GRID's 6kW
	_ = lr.HandleClearLimit(ctxLocal, ClearLimitRequest{})

	eff, ok := lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 6000000 {
		t.Fatalf("expected promoted limit 6000000, got %d (ok=%v)", eff, ok)
	}
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected still CONTROLLED, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_DurationExpiry(t *testing.T) {
	lr := newTestResolver()
	ctxA := testCtx("zone-A", cert.ZoneTypeLocal)
	ctxB := testCtx("zone-B", cert.ZoneTypeGrid)

	// Zone B sets indefinite 6kW
	_, _ = lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

	// Zone A sets 3kW with very short duration (1s is the minimum)
	dur := uint32(1)
	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{
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
	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
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
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
		ProductionLimit:  intPtr(3000000),
	})

	// Clear only consumption
	dirConsumption := DirectionConsumption
	_ = lr.HandleClearLimit(ctx, ClearLimitRequest{Direction: &dirConsumption})

	// Consumption should be nil, production should remain
	if _, ok := lr.ec.EffectiveConsumptionLimit(); ok {
		t.Fatal("expected consumption to be cleared")
	}
	effP, okP := lr.ec.EffectiveProductionLimit()
	if !okP || effP != 3000000 {
		t.Fatalf("production should remain, got %d (ok=%v)", effP, okP)
	}
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected still CONTROLLED (production active), got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_ValidationNegativeValue(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(-1000),
	})
	if resp.Applied {
		t.Fatal("expected reject for negative value")
	}
	if resp.RejectReason == nil || *resp.RejectReason != LimitRejectReasonInvalidValue {
		t.Fatal("expected INVALID_VALUE reject reason")
	}
}

func TestLimitResolver_ValidationOverride(t *testing.T) {
	lr := newTestResolver()
	_ = lr.ec.SetControlState(ControlStateOverride)
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})
	if resp.Applied {
		t.Fatal("expected reject in override state")
	}
	if resp.RejectReason == nil || *resp.RejectReason != LimitRejectReasonDeviceOverride {
		t.Fatal("expected DEVICE_OVERRIDE reject reason")
	}
}

func TestLimitResolver_ValidationNoZoneID(t *testing.T) {
	lr := newTestResolver()
	ctx := context.Background() // no zone info

	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
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

	// Set limit -> CONTROLLED (controller has authority)
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})
	if lr.ec.ControlState() != ControlStateControlled {
		t.Fatalf("expected CONTROLLED, got %s", lr.ec.ControlState())
	}

	// Clear -> AUTONOMOUS (no external control)
	_ = lr.HandleClearLimit(ctx, ClearLimitRequest{})
	if lr.ec.ControlState() != ControlStateAutonomous {
		t.Fatalf("expected AUTONOMOUS, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_NilLimitsDeactivate(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set a limit first
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Send nil limits = deactivate this zone
	resp, _ := lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: nil,
		ProductionLimit:  nil,
	})
	if !resp.Applied {
		t.Fatal("expected applied (deactivation)")
	}
	if lr.ec.ControlState() != ControlStateAutonomous {
		t.Fatalf("expected AUTONOMOUS after deactivation, got %s", lr.ec.ControlState())
	}
}

func TestLimitResolver_ClearZone(t *testing.T) {
	lr := newTestResolver()
	ctxA := testCtx("zone-A", cert.ZoneTypeLocal)
	ctxB := testCtx("zone-B", cert.ZoneTypeGrid)

	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{ConsumptionLimit: intPtr(3000000)})
	_, _ = lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

	// ClearZone removes all of zone A's limits
	lr.ClearZone("zone-A")

	eff, ok := lr.ec.EffectiveConsumptionLimit()
	if !ok || eff != 6000000 {
		t.Fatalf("expected zone B promoted to 6000000, got %d (ok=%v)", eff, ok)
	}
}

// newTestResolverRegistered creates a test resolver with Register() called,
// which wires up the ReadHook on the underlying model.Feature.
func newTestResolverRegistered() *LimitResolver {
	lr := newTestResolver()
	lr.Register()
	return lr
}

func TestLimitResolver_ReadHook_MyConsumptionLimit(t *testing.T) {
	lr := newTestResolverRegistered()
	ctxA := testCtx("zone-A", cert.ZoneTypeGrid)
	ctxB := testCtx("zone-B", cert.ZoneTypeLocal)

	// Zone A sets 6 kW consumption limit
	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

	// Zone B sets 5 kW consumption limit
	_, _ = lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Zone A reads myConsumptionLimit -> should see its own value (6 kW)
	val, err := lr.ec.ReadAttributeWithContext(ctxA, EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := val.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", val)
	}
	if v != 6000000 {
		t.Fatalf("zone A: expected 6000000, got %d", v)
	}

	// Zone B reads myConsumptionLimit -> should see its own value (5 kW)
	val, err = lr.ec.ReadAttributeWithContext(ctxB, EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok = val.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", val)
	}
	if v != 5000000 {
		t.Fatalf("zone B: expected 5000000, got %d", v)
	}
}

func TestLimitResolver_ReadHook_MyProductionLimit(t *testing.T) {
	lr := newTestResolverRegistered()
	ctxA := testCtx("zone-A", cert.ZoneTypeGrid)
	ctxB := testCtx("zone-B", cert.ZoneTypeLocal)

	// Zone A sets 4 kW production limit
	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{ProductionLimit: intPtr(4000000)})

	// Zone B sets 2 kW production limit
	_, _ = lr.HandleSetLimit(ctxB, SetLimitRequest{ProductionLimit: intPtr(2000000)})

	// Zone A reads myProductionLimit -> should see 4 kW
	val, err := lr.ec.ReadAttributeWithContext(ctxA, EnergyControlAttrMyProductionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := val.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", val)
	}
	if v != 4000000 {
		t.Fatalf("zone A: expected 4000000, got %d", v)
	}

	// Zone B reads myProductionLimit -> should see 2 kW
	val, err = lr.ec.ReadAttributeWithContext(ctxB, EnergyControlAttrMyProductionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok = val.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", val)
	}
	if v != 2000000 {
		t.Fatalf("zone B: expected 2000000, got %d", v)
	}
}

func TestLimitResolver_ReadHook_NoLimitSet(t *testing.T) {
	lr := newTestResolverRegistered()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Zone A has not set any limit, reads myConsumptionLimit -> expect nil
	val, err := lr.ec.ReadAttributeWithContext(ctx, EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil for unset limit, got %v", val)
	}
}

func TestLimitResolver_ReadHook_EffectiveUnchanged(t *testing.T) {
	lr := newTestResolverRegistered()
	ctxA := testCtx("zone-A", cert.ZoneTypeGrid)
	ctxB := testCtx("zone-B", cert.ZoneTypeLocal)

	// Zone A sets 6 kW, Zone B sets 5 kW
	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})
	_, _ = lr.HandleSetLimit(ctxB, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// effectiveConsumptionLimit (attr 20) should NOT be intercepted by hook.
	// It should return the resolved minimum (5 kW), regardless of which zone reads it.
	val, err := lr.ec.ReadAttributeWithContext(ctxA, EnergyControlAttrEffectiveConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, ok := val.(int64)
	if !ok {
		t.Fatalf("expected int64, got %T", val)
	}
	if v != 5000000 {
		t.Fatalf("effective limit: expected 5000000, got %d", v)
	}
}

func TestLimitResolver_ReadHook_AfterClear(t *testing.T) {
	lr := newTestResolverRegistered()
	ctx := testCtx("zone-A", cert.ZoneTypeLocal)

	// Set a limit
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Verify it's readable
	val, err := lr.ec.ReadAttributeWithContext(ctx, EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Fatal("expected non-nil value after set")
	}

	// Clear the limit
	_ = lr.HandleClearLimit(ctx, ClearLimitRequest{})

	// After clear, myConsumptionLimit should be nil
	val, err = lr.ec.ReadAttributeWithContext(ctx, EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil after clear, got %v", val)
	}
}

func TestLimitResolver_ReadHook_NoContext(t *testing.T) {
	lr := newTestResolverRegistered()

	// Set a limit from zone-A
	ctxA := testCtx("zone-A", cert.ZoneTypeLocal)
	_, _ = lr.HandleSetLimit(ctxA, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

	// Read with background context (no zone ID) -> expect nil
	val, err := lr.ec.ReadAttributeWithContext(context.Background(), EnergyControlAttrMyConsumptionLimit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil with no zone context, got %v", val)
	}
}

// =============================================================================
// OnZoneMyChange callback tests
// =============================================================================

// zoneMyChangeCall records a single invocation of the OnZoneMyChange callback.
type zoneMyChangeCall struct {
	zoneID  string
	changes map[uint16]any
}

func TestLimitResolver_NotifiesMyLimitOnSet(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-a", cert.ZoneTypeLocal)

	var calls []zoneMyChangeCall
	lr.OnZoneMyChange = func(zoneID string, changes map[uint16]any) {
		calls = append(calls, zoneMyChangeCall{zoneID, changes})
	}

	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(6000000),
	})

	if len(calls) != 1 {
		t.Fatalf("expected 1 callback call, got %d", len(calls))
	}
	if calls[0].zoneID != "zone-a" {
		t.Fatalf("expected zoneID 'zone-a', got %q", calls[0].zoneID)
	}
	if v, ok := calls[0].changes[EnergyControlAttrMyConsumptionLimit]; !ok || v != int64(6000000) {
		t.Fatalf("expected myConsumptionLimit=6000000, got %v", v)
	}
	if _, ok := calls[0].changes[EnergyControlAttrMyProductionLimit]; ok {
		t.Fatal("did not expect myProductionLimit in changes")
	}
}

func TestLimitResolver_NotifiesMyLimitOnClear(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-a", cert.ZoneTypeLocal)

	// Set a limit first
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})

	var calls []zoneMyChangeCall
	lr.OnZoneMyChange = func(zoneID string, changes map[uint16]any) {
		calls = append(calls, zoneMyChangeCall{zoneID, changes})
	}

	// Clear consumption only
	dirConsumption := DirectionConsumption
	_ = lr.HandleClearLimit(ctx, ClearLimitRequest{Direction: &dirConsumption})

	if len(calls) != 1 {
		t.Fatalf("expected 1 callback call, got %d", len(calls))
	}
	if calls[0].zoneID != "zone-a" {
		t.Fatalf("expected zoneID 'zone-a', got %q", calls[0].zoneID)
	}
	v, ok := calls[0].changes[EnergyControlAttrMyConsumptionLimit]
	if !ok {
		t.Fatal("expected myConsumptionLimit in changes")
	}
	if v != nil {
		t.Fatalf("expected nil for cleared limit, got %v", v)
	}
}

func TestLimitResolver_NotifiesMyLimitOnDeactivate(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-a", cert.ZoneTypeLocal)

	// Set a limit first
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
	})

	var calls []zoneMyChangeCall
	lr.OnZoneMyChange = func(zoneID string, changes map[uint16]any) {
		calls = append(calls, zoneMyChangeCall{zoneID, changes})
	}

	// Deactivate: both nil
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: nil,
		ProductionLimit:  nil,
	})

	if len(calls) != 1 {
		t.Fatalf("expected 1 callback call, got %d", len(calls))
	}
	if calls[0].zoneID != "zone-a" {
		t.Fatalf("expected zoneID 'zone-a', got %q", calls[0].zoneID)
	}
	// Both directions should be nil
	if v, ok := calls[0].changes[EnergyControlAttrMyConsumptionLimit]; !ok || v != nil {
		t.Fatalf("expected myConsumptionLimit=nil, got %v (ok=%v)", v, ok)
	}
	if v, ok := calls[0].changes[EnergyControlAttrMyProductionLimit]; !ok || v != nil {
		t.Fatalf("expected myProductionLimit=nil, got %v (ok=%v)", v, ok)
	}
}

func TestLimitResolver_NotifiesMyLimitOnClearZone(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-a", cert.ZoneTypeLocal)

	// Set limits
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(5000000),
		ProductionLimit:  intPtr(3000000),
	})

	var calls []zoneMyChangeCall
	lr.OnZoneMyChange = func(zoneID string, changes map[uint16]any) {
		calls = append(calls, zoneMyChangeCall{zoneID, changes})
	}

	lr.ClearZone("zone-a")

	if len(calls) != 1 {
		t.Fatalf("expected 1 callback call, got %d", len(calls))
	}
	if calls[0].zoneID != "zone-a" {
		t.Fatalf("expected zoneID 'zone-a', got %q", calls[0].zoneID)
	}
	if v, ok := calls[0].changes[EnergyControlAttrMyConsumptionLimit]; !ok || v != nil {
		t.Fatalf("expected myConsumptionLimit=nil, got %v (ok=%v)", v, ok)
	}
	if v, ok := calls[0].changes[EnergyControlAttrMyProductionLimit]; !ok || v != nil {
		t.Fatalf("expected myProductionLimit=nil, got %v (ok=%v)", v, ok)
	}
}

func TestLimitResolver_NoNotifyWhenNoCallback(t *testing.T) {
	lr := newTestResolver()
	ctx := testCtx("zone-a", cert.ZoneTypeLocal)

	// OnZoneMyChange is NOT set -- should not panic
	_, _ = lr.HandleSetLimit(ctx, SetLimitRequest{
		ConsumptionLimit: intPtr(6000000),
	})

	_ = lr.HandleClearLimit(ctx, ClearLimitRequest{})

	lr.ClearZone("zone-a")
	// If we reach here without panic, the test passes
}

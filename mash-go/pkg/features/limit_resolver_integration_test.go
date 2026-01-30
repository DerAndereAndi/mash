package features

import (
	"context"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

// readAsZone reads a single attribute using ReadAttributeWithContext with the
// given zone identity injected into the context.
func readAsZone(t *testing.T, ec *EnergyControl, zoneID string, zoneType cert.ZoneType, attrID uint16) any {
	t.Helper()
	ctx := testCtx(zoneID, zoneType)
	val, err := ec.ReadAttributeWithContext(ctx, attrID)
	if err != nil {
		t.Fatalf("ReadAttributeWithContext(%d) as %s failed: %v", attrID, zoneID, err)
	}
	return val
}

// assertInt64 asserts the value is an int64 matching the expected value.
func assertInt64(t *testing.T, label string, val any, expected int64) {
	t.Helper()
	v, ok := val.(int64)
	if !ok {
		t.Fatalf("%s: expected int64, got %T (%v)", label, val, val)
	}
	if v != expected {
		t.Fatalf("%s: expected %d, got %d", label, expected, v)
	}
}

// assertNil asserts the value is nil.
func assertNil(t *testing.T, label string, val any) {
	t.Helper()
	if val != nil {
		t.Fatalf("%s: expected nil, got %v (%T)", label, val, val)
	}
}

func TestLimitResolver_Integration_TwoZonePerZoneReads(t *testing.T) {
	t.Run("consumption limits", func(t *testing.T) {
		lr := newTestResolverRegistered()
		ctxGrid := testCtx("zone-GRID", cert.ZoneTypeGrid)
		ctxLocal := testCtx("zone-LOCAL", cert.ZoneTypeLocal)

		// Step 1: Zone GRID sets consumptionLimit = 6 kW
		_, _ = lr.HandleSetLimit(ctxGrid, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})

		// Step 2: Zone LOCAL sets consumptionLimit = 5 kW (more restrictive)
		_, _ = lr.HandleSetLimit(ctxLocal, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

		// Step 3: Zone GRID reads
		valMy := readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrMyConsumptionLimit)
		assertInt64(t, "GRID myConsumptionLimit", valMy, 6000000)

		valEff := readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrEffectiveConsumptionLimit)
		assertInt64(t, "GRID effectiveConsumptionLimit", valEff, 5000000)

		// Step 4: Zone LOCAL reads
		valMy = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrMyConsumptionLimit)
		assertInt64(t, "LOCAL myConsumptionLimit", valMy, 5000000)

		valEff = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrEffectiveConsumptionLimit)
		assertInt64(t, "LOCAL effectiveConsumptionLimit", valEff, 5000000)

		// Step 5: Zone GRID clears its limit
		_ = lr.HandleClearLimit(ctxGrid, ClearLimitRequest{})

		valMy = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrMyConsumptionLimit)
		assertNil(t, "GRID myConsumptionLimit after clear", valMy)

		valEff = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrEffectiveConsumptionLimit)
		assertInt64(t, "GRID effectiveConsumptionLimit after GRID clear", valEff, 5000000)

		valMy = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrMyConsumptionLimit)
		assertInt64(t, "LOCAL myConsumptionLimit after GRID clear", valMy, 5000000)

		valEff = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrEffectiveConsumptionLimit)
		assertInt64(t, "LOCAL effectiveConsumptionLimit after GRID clear", valEff, 5000000)

		// Step 6: Zone LOCAL clears its limit -- everything should be nil
		_ = lr.HandleClearLimit(ctxLocal, ClearLimitRequest{})

		valMy = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrMyConsumptionLimit)
		assertNil(t, "GRID myConsumptionLimit after all clear", valMy)

		valEff = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrEffectiveConsumptionLimit)
		assertNil(t, "GRID effectiveConsumptionLimit after all clear", valEff)

		valMy = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrMyConsumptionLimit)
		assertNil(t, "LOCAL myConsumptionLimit after all clear", valMy)

		valEff = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrEffectiveConsumptionLimit)
		assertNil(t, "LOCAL effectiveConsumptionLimit after all clear", valEff)
	})

	t.Run("production limits", func(t *testing.T) {
		lr := newTestResolverRegistered()
		ctxGrid := testCtx("zone-GRID", cert.ZoneTypeGrid)
		ctxLocal := testCtx("zone-LOCAL", cert.ZoneTypeLocal)

		// Zone GRID sets productionLimit = 4 kW
		_, _ = lr.HandleSetLimit(ctxGrid, SetLimitRequest{ProductionLimit: intPtr(4000000)})

		// Zone LOCAL sets productionLimit = 3 kW (more restrictive)
		_, _ = lr.HandleSetLimit(ctxLocal, SetLimitRequest{ProductionLimit: intPtr(3000000)})

		// Zone GRID reads its own production limit
		valMy := readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrMyProductionLimit)
		assertInt64(t, "GRID myProductionLimit", valMy, 4000000)

		valEff := readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrEffectiveProductionLimit)
		assertInt64(t, "GRID effectiveProductionLimit", valEff, 3000000)

		// Zone LOCAL reads its own production limit
		valMy = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrMyProductionLimit)
		assertInt64(t, "LOCAL myProductionLimit", valMy, 3000000)

		valEff = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrEffectiveProductionLimit)
		assertInt64(t, "LOCAL effectiveProductionLimit", valEff, 3000000)

		// Zone GRID clears its limit
		_ = lr.HandleClearLimit(ctxGrid, ClearLimitRequest{})

		valMy = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrMyProductionLimit)
		assertNil(t, "GRID myProductionLimit after clear", valMy)

		valEff = readAsZone(t, lr.ec, "zone-GRID", cert.ZoneTypeGrid, EnergyControlAttrEffectiveProductionLimit)
		assertInt64(t, "GRID effectiveProductionLimit after GRID clear", valEff, 3000000)

		// Zone LOCAL clears its limit
		_ = lr.HandleClearLimit(ctxLocal, ClearLimitRequest{})

		valMy = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrMyProductionLimit)
		assertNil(t, "LOCAL myProductionLimit after all clear", valMy)

		valEff = readAsZone(t, lr.ec, "zone-LOCAL", cert.ZoneTypeLocal, EnergyControlAttrEffectiveProductionLimit)
		assertNil(t, "LOCAL effectiveProductionLimit after all clear", valEff)
	})

	t.Run("ReadAllAttributesWithContext", func(t *testing.T) {
		lr := newTestResolverRegistered()
		ctxGrid := testCtx("zone-GRID", cert.ZoneTypeGrid)
		ctxLocal := testCtx("zone-LOCAL", cert.ZoneTypeLocal)

		// Both zones set consumption limits
		_, _ = lr.HandleSetLimit(ctxGrid, SetLimitRequest{ConsumptionLimit: intPtr(6000000)})
		_, _ = lr.HandleSetLimit(ctxLocal, SetLimitRequest{ConsumptionLimit: intPtr(5000000)})

		// ReadAllAttributesWithContext as Zone GRID
		allGrid := lr.ec.ReadAllAttributesWithContext(ctxGrid)

		myC, ok := allGrid[EnergyControlAttrMyConsumptionLimit]
		if !ok {
			t.Fatal("GRID ReadAll: myConsumptionLimit missing from result")
		}
		assertInt64(t, "GRID ReadAll myConsumptionLimit", myC, 6000000)

		effC, ok := allGrid[EnergyControlAttrEffectiveConsumptionLimit]
		if !ok {
			t.Fatal("GRID ReadAll: effectiveConsumptionLimit missing from result")
		}
		assertInt64(t, "GRID ReadAll effectiveConsumptionLimit", effC, 5000000)

		// ReadAllAttributesWithContext as Zone LOCAL
		allLocal := lr.ec.ReadAllAttributesWithContext(ctxLocal)

		myC, ok = allLocal[EnergyControlAttrMyConsumptionLimit]
		if !ok {
			t.Fatal("LOCAL ReadAll: myConsumptionLimit missing from result")
		}
		assertInt64(t, "LOCAL ReadAll myConsumptionLimit", myC, 5000000)

		effC, ok = allLocal[EnergyControlAttrEffectiveConsumptionLimit]
		if !ok {
			t.Fatal("LOCAL ReadAll: effectiveConsumptionLimit missing from result")
		}
		assertInt64(t, "LOCAL ReadAll effectiveConsumptionLimit", effC, 5000000)

		// Zone with no limits set should see nil for myConsumptionLimit
		ctxOther := testCtx("zone-OTHER", cert.ZoneTypeLocal)
		allOther := lr.ec.ReadAllAttributesWithContext(ctxOther)

		myC, ok = allOther[EnergyControlAttrMyConsumptionLimit]
		if !ok {
			t.Fatal("OTHER ReadAll: myConsumptionLimit missing from result")
		}
		assertNil(t, "OTHER ReadAll myConsumptionLimit", myC)

		// Effective limit should still be visible to any zone
		effC, ok = allOther[EnergyControlAttrEffectiveConsumptionLimit]
		if !ok {
			t.Fatal("OTHER ReadAll: effectiveConsumptionLimit missing from result")
		}
		assertInt64(t, "OTHER ReadAll effectiveConsumptionLimit", effC, 5000000)

		// ReadAllAttributesWithContext with background context (no zone)
		allNoZone := lr.ec.ReadAllAttributesWithContext(context.Background())

		myC, ok = allNoZone[EnergyControlAttrMyConsumptionLimit]
		if !ok {
			t.Fatal("no-zone ReadAll: myConsumptionLimit missing from result")
		}
		assertNil(t, "no-zone ReadAll myConsumptionLimit", myC)
	})
}

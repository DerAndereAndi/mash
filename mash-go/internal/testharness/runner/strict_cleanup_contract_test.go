package runner

import "testing"

func TestVisibleNonSuiteZoneCountFromSnapshot_PrefersZonesListOverZoneCount(t *testing.T) {
	snap := DeviceStateSnapshot{
		KeyZoneCount: 2, // stale aggregate
		"zones": []any{
			map[string]any{
				"id":   "suite-zone",
				"type": 3,
			},
		},
	}

	got, ok := visibleNonSuiteZoneCountFromSnapshot(snap, "suite-zone")
	if !ok {
		t.Fatal("expected visible zone count to be available")
	}
	if got != 0 {
		t.Fatalf("visibleNonSuiteZoneCountFromSnapshot() = %d, want 0", got)
	}
}

func TestVisibleNonSuiteZoneCountFromSnapshot_CountsResidualNonSuiteZones(t *testing.T) {
	snap := DeviceStateSnapshot{
		KeyZoneCount: 2,
		"zones": []any{
			map[string]any{"id": "suite-zone", "type": 3},
			map[string]any{"id": "grid-zone", "type": 1},
		},
	}

	got, ok := visibleNonSuiteZoneCountFromSnapshot(snap, "suite-zone")
	if !ok {
		t.Fatal("expected visible zone count to be available")
	}
	if got != 1 {
		t.Fatalf("visibleNonSuiteZoneCountFromSnapshot() = %d, want 1", got)
	}
}

func TestVisibleNonSuiteZoneCountFromSnapshot_FallsBackToZoneCount(t *testing.T) {
	snap := DeviceStateSnapshot{
		KeyZoneCount: 1,
	}

	got, ok := visibleNonSuiteZoneCountFromSnapshot(snap, "suite-zone")
	if !ok {
		t.Fatal("expected visible zone count to be available")
	}
	if got != 0 {
		t.Fatalf("visibleNonSuiteZoneCountFromSnapshot() = %d, want 0", got)
	}
}


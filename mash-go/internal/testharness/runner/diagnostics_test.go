package runner

import (
	"testing"
)

func TestDiffSnapshots_NoDiffs(t *testing.T) {
	before := DeviceStateSnapshot{
		"zone_count":     0,
		"clock_offset_s": 0,
		"active_conns":   1,
	}
	after := DeviceStateSnapshot{
		"zone_count":     0,
		"clock_offset_s": 0,
		"active_conns":   1,
	}
	diffs := diffSnapshots(before, after)
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(diffs), diffs)
	}
}

func TestDiffSnapshots_DetectsChanges(t *testing.T) {
	before := DeviceStateSnapshot{
		"zone_count":     1,
		"clock_offset_s": 0,
		"active_conns":   1,
	}
	after := DeviceStateSnapshot{
		"zone_count":     2,
		"clock_offset_s": 0,
		"active_conns":   3,
	}
	diffs := diffSnapshots(before, after)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %v", len(diffs), diffs)
	}
	// Sorted by key: active_conns, zone_count
	if diffs[0].Key != "active_conns" {
		t.Errorf("diffs[0].Key = %q, want active_conns", diffs[0].Key)
	}
	if diffs[1].Key != "zone_count" {
		t.Errorf("diffs[1].Key = %q, want zone_count", diffs[1].Key)
	}
}

func TestDiffSnapshots_MissingKeys(t *testing.T) {
	before := DeviceStateSnapshot{
		"zone_count": 1,
		"old_field":  "gone",
	}
	after := DeviceStateSnapshot{
		"zone_count": 1,
		"new_field":  "appeared",
	}
	diffs := diffSnapshots(before, after)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d: %v", len(diffs), diffs)
	}
	// new_field (after-only), old_field (before-only)
	if diffs[0].Key != "new_field" || diffs[0].Before != nil {
		t.Errorf("diffs[0] = %+v, want new_field with before=nil", diffs[0])
	}
	if diffs[1].Key != "old_field" || diffs[1].After != nil {
		t.Errorf("diffs[1] = %+v, want old_field with after=nil", diffs[1])
	}
}

func TestDiffSnapshots_NilInputs(t *testing.T) {
	if diffs := diffSnapshots(nil, DeviceStateSnapshot{"a": 1}); diffs != nil {
		t.Errorf("expected nil for nil before, got %v", diffs)
	}
	if diffs := diffSnapshots(DeviceStateSnapshot{"a": 1}, nil); diffs != nil {
		t.Errorf("expected nil for nil after, got %v", diffs)
	}
}

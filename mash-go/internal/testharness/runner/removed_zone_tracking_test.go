package runner

import "testing"

func TestRemovedZoneTracking_RoundTrip(t *testing.T) {
	state := newTestState()
	recordRemovedZoneID(state, "zone-a")
	recordRemovedZoneID(state, "zone-b")
	recordRemovedZoneID(state, "zone-a") // duplicate

	if !wasZoneRemovedInTest(state, "zone-a") {
		t.Fatal("expected zone-a to be marked removed")
	}
	if !wasZoneRemovedInTest(state, "zone-b") {
		t.Fatal("expected zone-b to be marked removed")
	}
	if wasZoneRemovedInTest(state, "zone-c") {
		t.Fatal("did not expect zone-c to be marked removed")
	}
}


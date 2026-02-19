package runner

import "github.com/mash-protocol/mash-go/internal/testharness/engine"

func recordRemovedZoneID(state *engine.ExecutionState, zoneID string) {
	if state == nil || zoneID == "" {
		return
	}
	existing := removedZoneIDSet(state)
	existing[zoneID] = struct{}{}
	out := make([]string, 0, len(existing))
	for id := range existing {
		out = append(out, id)
	}
	state.Set(StateRemovedZoneIDs, out)
}

func wasZoneRemovedInTest(state *engine.ExecutionState, zoneID string) bool {
	if state == nil || zoneID == "" {
		return false
	}
	_, ok := removedZoneIDSet(state)[zoneID]
	return ok
}

func removedZoneIDSet(state *engine.ExecutionState) map[string]struct{} {
	out := make(map[string]struct{})
	if state == nil {
		return out
	}
	raw, ok := state.Get(StateRemovedZoneIDs)
	if !ok || raw == nil {
		return out
	}
	switch v := raw.(type) {
	case []string:
		for _, id := range v {
			if id != "" {
				out[id] = struct{}{}
			}
		}
	case []any:
		for _, it := range v {
			if s, ok := it.(string); ok && s != "" {
				out[s] = struct{}{}
			}
		}
	}
	return out
}


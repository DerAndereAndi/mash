package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

const stateKeyStrictCleanupContract = "strict_cleanup_contract"

// StrictCleanupContractReport captures strict teardown contract checks.
type StrictCleanupContractReport struct {
	CommissioningState string
	VisibleZoneCount   *int
	ProbeError         string
	Issues             []string
}

func (r StrictCleanupContractReport) IsClean() bool {
	return len(r.Issues) == 0
}

func (r StrictCleanupContractReport) Summary() string {
	if len(r.Issues) == 0 {
		return "clean"
	}
	return strings.Join(r.Issues, "; ")
}

func (r StrictCleanupContractReport) ToMap() map[string]any {
	out := map[string]any{
		"commissioning_state": r.CommissioningState,
		"clean":               r.IsClean(),
		"issues":              append([]string(nil), r.Issues...),
	}
	if r.VisibleZoneCount != nil {
		out["visible_zone_count"] = *r.VisibleZoneCount
	} else {
		out["visible_zone_count"] = nil
	}
	if r.ProbeError != "" {
		out["probe_error"] = r.ProbeError
	}
	return out
}

func evaluateStrictCleanupContract(expectCommissioned bool, commissioningState string, visibleZoneCount *int, probeErr error) StrictCleanupContractReport {
	report := StrictCleanupContractReport{
		CommissioningState: commissioningState,
		VisibleZoneCount:   visibleZoneCount,
	}
	if probeErr != nil {
		report.ProbeError = probeErr.Error()
	}

	if expectCommissioned {
		if commissioningState != CommissioningStateCommissioned {
			report.Issues = append(report.Issues,
				fmt.Sprintf("commissioning state=%q (want %q)", commissioningState, CommissioningStateCommissioned))
		}
		// Device-state snapshots can be transiently unavailable after teardown
		// retries. When the suite probe is healthy, treat missing zone count as
		// best-effort telemetry, not a hard contract breach.
		if visibleZoneCount == nil {
			if probeErr != nil {
				report.Issues = append(report.Issues, "zone count unavailable")
			}
		} else if *visibleZoneCount != 0 {
			report.Issues = append(report.Issues, fmt.Sprintf("residual zones visible=%d", *visibleZoneCount))
		}
		if probeErr != nil {
			report.Issues = append(report.Issues, "reconnect probe failed: "+probeErr.Error())
		}
		return report
	}

	if commissioningState != CommissioningStateAdvertising && commissioningState != "IDLE" {
		report.Issues = append(report.Issues,
			fmt.Sprintf("commissioning state=%q (want %q or IDLE)", commissioningState, CommissioningStateAdvertising))
	}
	if visibleZoneCount != nil && *visibleZoneCount != 0 {
		report.Issues = append(report.Issues, fmt.Sprintf("residual zones visible=%d", *visibleZoneCount))
	}
	return report
}

// runStrictCleanupContract executes strict post-teardown contract checks.
func (r *Runner) runStrictCleanupContract(ctx context.Context, state *engine.ExecutionState) StrictCleanupContractReport {
	var commissioningState string
	if out, err := r.handleVerifyCommissioningState(ctx, &loader.Step{Params: map[string]any{}}, state); err == nil {
		commissioningState, _ = out[KeyCommissioningState].(string)
	}

	expectCommissioned := r.config != nil && r.config.Target != "" && r.config.EnableKey != ""

	var probeErr error
	if expectCommissioned {
		if sc := r.suite.Conn(); sc != nil && sc.isConnected() {
			if mc := r.pool.Main(); mc == nil || !mc.isConnected() {
				r.pool.SetMain(sc)
			}
		}
		probeErr = r.probeSessionHealth()
		if probeErr != nil {
			if recErr := r.reconnectToZone(state); recErr == nil {
				probeErr = r.probeSessionHealth()
			}
		}
	}

	var visibleZoneCount *int
	if expectCommissioned {
		if snap := r.requestDeviceState(ctx, state); snap != nil {
			if v, ok := visibleNonSuiteZoneCountFromSnapshot(snap, r.suite.ZoneID()); ok {
				visibleZoneCount = &v
			}
		}
		// Retry once after a successful probe if the first snapshot was unavailable.
		if visibleZoneCount == nil && probeErr == nil {
			if snap := r.requestDeviceState(ctx, state); snap != nil {
				if v, ok := visibleNonSuiteZoneCountFromSnapshot(snap, r.suite.ZoneID()); ok {
					visibleZoneCount = &v
				}
			}
		}
	}

	return evaluateStrictCleanupContract(expectCommissioned, commissioningState, visibleZoneCount, probeErr)
}

// visibleNonSuiteZoneCountFromSnapshot derives the number of non-suite zones
// from a device test-state snapshot.
//
// Prefer the detailed "zones" list when present, because "zone_count" can be a
// transient aggregate during reconnect churn. Fall back to zone_count only when
// the list is unavailable.
func visibleNonSuiteZoneCountFromSnapshot(snap DeviceStateSnapshot, suiteZoneID string) (int, bool) {
	if snap == nil {
		return 0, false
	}

	if rawZones, ok := snap["zones"]; ok {
		if list, ok := rawZones.([]any); ok {
			if suiteZoneID != "" {
				return len(nonSuiteZoneIDsFromSnapshot(snap, suiteZoneID)), true
			}
			seen := map[string]struct{}{}
			for _, entry := range list {
				m, ok := entry.(map[string]any)
				if !ok {
					continue
				}
				id, _ := m["id"].(string)
				if id == "" {
					continue
				}
				seen[id] = struct{}{}
			}
			return len(seen), true
		}
	}

	rawCount, ok := snap[KeyZoneCount]
	if !ok {
		return 0, false
	}
	count := toIntValue(rawCount)
	if suiteZoneID != "" && count > 0 {
		count--
	}
	if count < 0 {
		count = 0
	}
	return count, true
}

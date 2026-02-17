package runner

import (
	"fmt"
	"strings"
	"time"
)

// CleanupReport captures post-test lifecycle invariants for runner state.
// It is used to detect hidden state leakage across tests.
type CleanupReport struct {
	Timestamp time.Time

	HasPhantomMainSocket bool
	HasPhantomZoneSocket bool
	PhantomZoneConnKey   string

	ActiveZoneCount int

	HasIncompletePASE bool

	// HasResidualSuiteConnection is true when a suite connection still exists
	// but no suite zone is commissioned (dangling control channel).
	HasResidualSuiteConnection bool

	Issues []string
}

// IsClean reports whether all cleanup invariants hold.
func (cr CleanupReport) IsClean() bool {
	return !cr.HasPhantomMainSocket &&
		!cr.HasPhantomZoneSocket &&
		cr.ActiveZoneCount == 0 &&
		!cr.HasIncompletePASE &&
		!cr.HasResidualSuiteConnection
}

// Summary returns a compact human-readable description of cleanup issues.
func (cr CleanupReport) Summary() string {
	if len(cr.Issues) == 0 {
		return "clean"
	}
	return strings.Join(cr.Issues, "; ")
}

// ToMap converts the report to a generic map for ExecutionState.Custom.
func (cr CleanupReport) ToMap() map[string]any {
	return map[string]any{
		"timestamp":                     cr.Timestamp,
		"clean":                         cr.IsClean(),
		"issues":                        append([]string(nil), cr.Issues...),
		"has_phantom_main_socket":       cr.HasPhantomMainSocket,
		"has_phantom_zone_socket":       cr.HasPhantomZoneSocket,
		"phantom_zone_conn_key":         cr.PhantomZoneConnKey,
		"active_zone_count":             cr.ActiveZoneCount,
		"has_incomplete_pase":           cr.HasIncompletePASE,
		"has_residual_suite_connection": cr.HasResidualSuiteConnection,
	}
}

// BuildCleanupReport inspects current runner state and returns invariant checks.
func (r *Runner) BuildCleanupReport() CleanupReport {
	cr := CleanupReport{
		Timestamp: time.Now(),
	}

	if r == nil {
		cr.Issues = append(cr.Issues, "runner is nil")
		return cr
	}

	s := r.snapshot()

	cr.HasPhantomMainSocket = s.HasPhantomSocket()
	if cr.HasPhantomMainSocket {
		cr.Issues = append(cr.Issues, "phantom main socket")
	}

	if zoneKey, ok := s.HasPhantomZoneSocket(); ok {
		cr.HasPhantomZoneSocket = true
		cr.PhantomZoneConnKey = zoneKey
		cr.Issues = append(cr.Issues, fmt.Sprintf("phantom zone socket (%s)", zoneKey))
	}

	cr.ActiveZoneCount = len(s.ActiveZones)
	if cr.ActiveZoneCount > 0 {
		cr.Issues = append(cr.Issues, fmt.Sprintf("active zone connections remain: %d", cr.ActiveZoneCount))
	}

	if r.connMgr != nil {
		if ps := r.connMgr.PASEState(); ps != nil && !ps.completed {
			cr.HasIncompletePASE = true
			cr.Issues = append(cr.Issues, "incomplete PASE state")
		}
	}

	// A suite connection is expected only when a suite zone is commissioned.
	// If no suite zone exists but a suite connection remains, that's a leak.
	if r.suite != nil {
		if sc := r.suite.Conn(); sc != nil && r.suite.ZoneID() == "" {
			cr.HasResidualSuiteConnection = true
			cr.Issues = append(cr.Issues, "residual suite connection")
		}
	}

	return cr
}

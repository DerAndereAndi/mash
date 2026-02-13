package runner

import "github.com/mash-protocol/mash-go/internal/testharness/loader"

// Connection tier constants define how aggressively a test isolates its
// connection state from prior tests.
const (
	// TierInfrastructure forces a full disconnect before setup. Used for
	// tests that exercise commissioning, TLS handshakes, or other
	// infrastructure-level behavior.
	TierInfrastructure = "infrastructure"

	// TierProtocol disconnects the TCP socket but preserves crypto state,
	// then reconnects using stored zone credentials. Used for tests that
	// need a fresh connection but can reuse zone material.
	TierProtocol = "protocol"

	// TierApplication reuses an existing commissioned connection if
	// healthy. Used for application-level tests (reads, writes,
	// subscribes) that don't depend on connection freshness.
	TierApplication = "application"
)

// connectionTierFor returns the effective connection tier for a test case.
// If the test case explicitly declares a tier, that value is used. Otherwise
// the tier is inferred from preconditions for backward compatibility.
func connectionTierFor(tc *loader.TestCase) string {
	if tc.ConnectionTier != "" {
		return tc.ConnectionTier
	}
	needed := preconditionLevelFor(tc.Preconditions)
	if needed <= precondLevelCommissioning {
		return TierInfrastructure
	}
	if needsFreshCommission(tc.Preconditions) {
		return TierProtocol
	}
	return TierApplication
}

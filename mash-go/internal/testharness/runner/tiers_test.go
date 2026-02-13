package runner

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

func TestConnectionTierFor_DefaultIsApplication(t *testing.T) {
	tc := &loader.TestCase{
		ID: "TC-APP-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierApplication {
		t.Fatalf("expected %q, got %q", TierApplication, tier)
	}
}

func TestConnectionTierFor_ExplicitInfrastructure(t *testing.T) {
	tc := &loader.TestCase{
		ID:             "TC-INFRA-001",
		ConnectionTier: TierInfrastructure,
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierInfrastructure {
		t.Fatalf("expected %q, got %q", TierInfrastructure, tier)
	}
}

func TestConnectionTierFor_ExplicitProtocol(t *testing.T) {
	tc := &loader.TestCase{
		ID:             "TC-PROTO-001",
		ConnectionTier: TierProtocol,
	}
	if tier := connectionTierFor(tc); tier != TierProtocol {
		t.Fatalf("expected %q, got %q", TierProtocol, tier)
	}
}

func TestConnectionTierFor_InfersInfraForL1(t *testing.T) {
	tc := &loader.TestCase{
		ID: "TC-COMM-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceInCommissioningMode: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierInfrastructure {
		t.Fatalf("expected %q for L1 precondition, got %q", TierInfrastructure, tier)
	}
}

func TestConnectionTierFor_InfersInfraForL0(t *testing.T) {
	tc := &loader.TestCase{
		ID: "TC-DISC-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceBooted: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierInfrastructure {
		t.Fatalf("expected %q for L0 precondition, got %q", TierInfrastructure, tier)
	}
}

func TestConnectionTierFor_InfersProtocolForFreshCommission(t *testing.T) {
	tc := &loader.TestCase{
		ID: "TC-ZONE-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
			{PrecondFreshCommission: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierProtocol {
		t.Fatalf("expected %q for fresh_commission, got %q", TierProtocol, tier)
	}
}

func TestConnectionTierFor_InfersApplicationForL3(t *testing.T) {
	tc := &loader.TestCase{
		ID: "TC-MEAS-001",
		Preconditions: []loader.Condition{
			{PrecondDeviceCommissioned: true},
		},
	}
	if tier := connectionTierFor(tc); tier != TierApplication {
		t.Fatalf("expected %q for L3 (no fresh_commission), got %q", TierApplication, tier)
	}
}

func TestConnectionTierFor_NoPreconditionsIsInfra(t *testing.T) {
	tc := &loader.TestCase{ID: "TC-BARE-001"}
	if tier := connectionTierFor(tc); tier != TierInfrastructure {
		t.Fatalf("expected %q for no preconditions, got %q", TierInfrastructure, tier)
	}
}

package rules

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

func TestCMD001_AcceptsLimitsRequiresCommands(t *testing.T) {
	rule := NewCMD001()

	// No A0A - not applicable
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1`)
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without A0A")
	}

	// A0A without commands - violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for A0A without C01/C02")
	}

	// A0A with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.C02.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD002_AcceptsCurrentLimitsRequiresCommands(t *testing.T) {
	rule := NewCMD002()

	// A0B without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0B=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for A0B without C03/C04")
	}

	// A0B with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0B=1
MASH.S.E01.CTRL.C03.Rsp=1
MASH.S.E01.CTRL.C04.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD003_AcceptsSetpointsRequiresCommands(t *testing.T) {
	rule := NewCMD003()

	// A0C without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0C=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for A0C without C05/C06")
	}

	// A0C with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0C=1
MASH.S.E01.CTRL.C05.Rsp=1
MASH.S.E01.CTRL.C06.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD004_IsPausableRequiresCommands(t *testing.T) {
	rule := NewCMD004()

	// A0E without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0E=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for A0E without C09/C0A")
	}

	// A0E with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0E=1
MASH.S.E01.CTRL.C09.Rsp=1
MASH.S.E01.CTRL.C0A.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD005_IsStoppableRequiresCommand(t *testing.T) {
	rule := NewCMD005()

	// A10 without command - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A10=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for A10 without C0B")
	}

	// A10 with command - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A10=1
MASH.S.E01.CTRL.C0B.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD006_V2XRequiresCommands(t *testing.T) {
	rule := NewCMD006()

	// F0A without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F0A=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for F0A without C07/C08")
	}

	// F0A with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F0A=1
MASH.S.E01.CTRL.C07.Rsp=1
MASH.S.E01.CTRL.C08.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD007_PROCESSRequiresCommands(t *testing.T) {
	rule := NewCMD007()

	// F07 without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F07=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for F07 without C0C/C0D")
	}

	// F07 with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F07=1
MASH.S.E01.CTRL.C0C.Rsp=1
MASH.S.E01.CTRL.C0D.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD008_SIGRequiresCommands(t *testing.T) {
	rule := NewCMD008()

	// SIG without commands - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.SIG=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for SIG without C01/C02")
	}

	// SIG with commands - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.SIG=1
MASH.S.E01.SIG.C01.Rsp=1
MASH.S.E01.SIG.C02.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD009_CHRGRequiresCommand(t *testing.T) {
	rule := NewCMD009()

	// CHRG without command - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CHRG=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for CHRG without C01")
	}

	// CHRG with command - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CHRG=1
MASH.S.E01.CHRG.C01.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD010_PLANRequiresRequestPlan(t *testing.T) {
	rule := NewCMD010()

	// PLAN without command - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=HEAT_PUMP
MASH.S.E01.PLAN=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for PLAN without C01")
	}

	// PLAN with command - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=HEAT_PUMP
MASH.S.E01.PLAN=1
MASH.S.E01.PLAN.C01.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestCMD011_TARRequiresSetTariff(t *testing.T) {
	rule := NewCMD011()

	// TAR without command - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.TAR=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for TAR without C01")
	}

	// TAR with command - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.TAR=1
MASH.S.E01.TAR.C01.Rsp=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestRegisterConsistencyRules(t *testing.T) {
	registry := pics.NewRuleRegistry()
	RegisterConsistencyRules(registry)

	rules := registry.RulesByCategory("consistency")
	if len(rules) != 11 {
		t.Errorf("Expected 11 consistency rules, got %d", len(rules))
	}

	expectedIDs := []string{
		"CMD-001", "CMD-002", "CMD-003", "CMD-004", "CMD-005",
		"CMD-006", "CMD-007", "CMD-008", "CMD-009", "CMD-010", "CMD-011",
	}

	for _, id := range expectedIDs {
		if registry.GetRule(id) == nil {
			t.Errorf("Expected rule %s to be registered", id)
		}
	}
}

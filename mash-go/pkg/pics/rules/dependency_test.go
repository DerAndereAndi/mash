package rules

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

func TestDEP001_V2XRequiresEMOB(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectViolate bool
	}{
		{
			name: "V2X without EMOB - violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F0A=1`,
			expectViolate: true,
		},
		{
			name: "V2X with EMOB - no violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F03=1
MASH.S.E01.CTRL.F0A=1`,
			expectViolate: false,
		},
		{
			name: "No V2X - not applicable",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F03=1`,
			expectViolate: false,
		},
	}

	rule := NewDEP001()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := pics.ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}

			violations := rule.Check(p)
			hasViolation := len(violations) > 0

			if hasViolation != tt.expectViolate {
				t.Errorf("Check() violation=%v, want=%v", hasViolation, tt.expectViolate)
			}

			if hasViolation && violations[0].RuleID != "DEP-001" {
				t.Errorf("RuleID=%s, want DEP-001", violations[0].RuleID)
			}
		})
	}
}

func TestDEP002_ASYMMETRICRequiresMultiPhase(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectViolate bool
	}{
		{
			name: "ASYMMETRIC without ELEC on same endpoint - violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F09=1`,
			expectViolate: true,
		},
		{
			name: "ASYMMETRIC with ELEC but single phase - violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F09=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A01=1`,
			expectViolate: true,
		},
		{
			name: "ASYMMETRIC with ELEC multi-phase - no violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F09=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A01=3`,
			expectViolate: false,
		},
		{
			name: "No ASYMMETRIC - not applicable",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1`,
			expectViolate: false,
		},
	}

	rule := NewDEP002()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := pics.ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}

			violations := rule.Check(p)
			hasViolation := len(violations) > 0

			if hasViolation != tt.expectViolate {
				t.Errorf("Check() violation=%v, want=%v", hasViolation, tt.expectViolate)
			}
		})
	}
}

func TestDEP003_SIGNALSRequiresSIG(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectViolate bool
	}{
		{
			name: "SIGNALS without SIG on same endpoint - violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F04=1`,
			expectViolate: true,
		},
		{
			name: "SIGNALS with SIG on same endpoint - no violation",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F04=1
MASH.S.E01.SIG=1`,
			expectViolate: false,
		},
		{
			name: "No SIGNALS flag - not applicable",
			input: `MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1`,
			expectViolate: false,
		},
	}

	rule := NewDEP003()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := pics.ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}

			violations := rule.Check(p)
			hasViolation := len(violations) > 0

			if hasViolation != tt.expectViolate {
				t.Errorf("Check() violation=%v, want=%v", hasViolation, tt.expectViolate)
			}
		})
	}
}

func TestDEP004_TARIFFRequiresTAR(t *testing.T) {
	rule := NewDEP004()

	// Without TAR on same endpoint - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F05=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for TARIFF without TAR")
	}

	// With TAR on same endpoint - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F05=1
MASH.S.E01.TAR=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for TARIFF with TAR")
	}
}

func TestDEP005_PLANRequiresPLANFeature(t *testing.T) {
	rule := NewDEP005()

	// Without PLAN feature on same endpoint - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F06=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for PLAN flag without PLAN feature")
	}

	// With PLAN feature on same endpoint - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F06=1
MASH.S.E01.PLAN=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for PLAN flag with PLAN feature")
	}
}

func TestDEP006_PROCESSRequiresAttributes(t *testing.T) {
	rule := NewDEP006()

	// Without required attributes - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F07=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for PROCESS without A50/A51")
	}

	// With required attributes - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F07=1
MASH.S.E01.CTRL.A50=1
MASH.S.E01.CTRL.A51=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation, got: %v", violations)
	}
}

func TestDEP007_FORECASTRequiresA3D(t *testing.T) {
	rule := NewDEP007()

	// Without A3D - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F08=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for FORECAST without A3D")
	}

	// With A3D - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F08=1
MASH.S.E01.CTRL.A3D=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for FORECAST with A3D")
	}
}

func TestDEP008_EMOBRequiresCHRG(t *testing.T) {
	rule := NewDEP008()

	// Without CHRG on same endpoint - violation
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F03=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for EMOB without CHRG")
	}

	// With CHRG on same endpoint - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F03=1
MASH.S.E01.CHRG=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for EMOB with CHRG")
	}
}

func TestDEP009_V2XRequiresBIDIRECTIONAL(t *testing.T) {
	rule := NewDEP009()

	// V2X without ELEC.A05 on same endpoint - warning
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F0A=1
MASH.S.E01.ELEC=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected warning for V2X without supportedDirections")
	}
	if len(violations) > 0 && violations[0].Severity != pics.SeverityWarning {
		t.Errorf("Expected warning severity, got %v", violations[0].Severity)
	}

	// V2X with ELEC.A05 on same endpoint - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F0A=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A05=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation when A05 is present")
	}
}

func TestDEP010_BATTERYEMOBMutualExclusion(t *testing.T) {
	rule := NewDEP010()

	// Both BATTERY and EMOB on same endpoint - warning
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F02=1
MASH.S.E01.CTRL.F03=1`)
	violations := rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected warning for both BATTERY and EMOB")
	}
	if len(violations) > 0 && violations[0].Severity != pics.SeverityWarning {
		t.Errorf("Expected warning severity, got %v", violations[0].Severity)
	}

	// Only BATTERY on one endpoint - no violation
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=BATTERY
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F02=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for only BATTERY")
	}

	// BATTERY on ep1, EMOB on ep2 - no violation (different endpoints)
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.E01=BATTERY
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.F02=1
MASH.S.E02=EV_CHARGER
MASH.S.E02.CTRL=1
MASH.S.E02.CTRL.F03=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation for BATTERY/EMOB on different endpoints")
	}
}

func TestRegisterDependencyRules(t *testing.T) {
	registry := pics.NewRuleRegistry()
	RegisterDependencyRules(registry)

	// Should have 10 dependency rules
	rules := registry.RulesByCategory("dependency")
	if len(rules) != 10 {
		t.Errorf("Expected 10 dependency rules, got %d", len(rules))
	}

	// Verify rule IDs
	expectedIDs := []string{
		"DEP-001", "DEP-002", "DEP-003", "DEP-004", "DEP-005",
		"DEP-006", "DEP-007", "DEP-008", "DEP-009", "DEP-010",
	}

	for _, id := range expectedIDs {
		if registry.GetRule(id) == nil {
			t.Errorf("Expected rule %s to be registered", id)
		}
	}
}

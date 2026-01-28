package rules

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

func TestMAN001_ProtocolDeclaration(t *testing.T) {
	rule := NewMAN001()

	tests := []struct {
		name          string
		input         string
		expectViolate bool
	}{
		{
			name:          "has MASH.S",
			input:         "MASH.S=1",
			expectViolate: false,
		},
		{
			name:          "has MASH.C",
			input:         "MASH.C=1",
			expectViolate: false,
		},
		{
			name:          "missing both",
			input:         "MASH.S.CTRL=1",
			expectViolate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := pics.ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}
			violations := rule.Check(p)
			if (len(violations) > 0) != tt.expectViolate {
				t.Errorf("Check() violation=%v, want=%v", len(violations) > 0, tt.expectViolate)
			}
		})
	}
}

func TestMAN002_CTRLMandatoryAttributes(t *testing.T) {
	rule := NewMAN002()

	// Without CTRL - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without CTRL feature")
	}

	// With CTRL but missing mandatory attrs
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.CTRL=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing mandatory attributes")
	}

	// With all mandatory attributes
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.A01=1
MASH.S.CTRL.A02=1
MASH.S.CTRL.A0A=1
MASH.S.CTRL.A0B=1
MASH.S.CTRL.A0C=1
MASH.S.CTRL.A0E=1
MASH.S.CTRL.A46=1
MASH.S.CTRL.A48=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation with all mandatory attrs, got: %v", violations)
	}
}

func TestMAN003_ELECMandatoryAttributes(t *testing.T) {
	rule := NewMAN003()

	// Without ELEC - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without ELEC feature")
	}

	// With ELEC but missing mandatory attrs
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.ELEC=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing mandatory attributes")
	}

	// With all mandatory attributes
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.ELEC=1
MASH.S.ELEC.A01=1
MASH.S.ELEC.A02=1
MASH.S.ELEC.A03=1
MASH.S.ELEC.A04=1
MASH.S.ELEC.A05=1
MASH.S.ELEC.A0D=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation with all mandatory attrs, got: %v", violations)
	}
}

func TestMAN004_CHRGMandatoryAttributes(t *testing.T) {
	rule := NewMAN004()

	// Without CHRG - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without CHRG feature")
	}

	// With CHRG but missing mandatory attrs
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.CHRG=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing mandatory attributes")
	}
}

func TestMAN005_SIGMandatoryAttributes(t *testing.T) {
	rule := NewMAN005()

	// Without SIG - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without SIG feature")
	}

	// With SIG but missing mandatory attrs
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.SIG=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing mandatory attributes")
	}
}

func TestMAN006_STATMandatoryAttributes(t *testing.T) {
	rule := NewMAN006()

	// Without STAT - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without STAT feature")
	}

	// With STAT but missing A01
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.STAT=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing A01")
	}

	// With STAT and A01
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.STAT=1
MASH.S.STAT.A01=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation with A01, got: %v", violations)
	}
}

func TestMAN007_MEASMandatoryAttributes(t *testing.T) {
	rule := NewMAN007()

	// Without MEAS - not applicable
	p, _ := pics.ParseString("MASH.S=1")
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violation without MEAS feature")
	}

	// With MEAS but no AC or DC power
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.MEAS=1`)
	violations = rule.Check(p)
	if len(violations) == 0 {
		t.Error("Expected violation for missing A01 or A28")
	}

	// With AC power
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.MEAS=1
MASH.S.MEAS.A01=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation with A01, got: %v", violations)
	}

	// With DC power
	p, _ = pics.ParseString(`MASH.S=1
MASH.S.MEAS=1
MASH.S.MEAS.A28=1`)
	violations = rule.Check(p)
	if len(violations) > 0 {
		t.Errorf("Expected no violation with A28, got: %v", violations)
	}
}

func TestRegisterMandatoryRules(t *testing.T) {
	registry := pics.NewRuleRegistry()
	RegisterMandatoryRules(registry)

	rules := registry.RulesByCategory("mandatory")
	if len(rules) != 7 {
		t.Errorf("Expected 7 mandatory rules, got %d", len(rules))
	}

	expectedIDs := []string{
		"MAN-001", "MAN-002", "MAN-003", "MAN-004",
		"MAN-005", "MAN-006", "MAN-007",
	}

	for _, id := range expectedIDs {
		if registry.GetRule(id) == nil {
			t.Errorf("Expected rule %s to be registered", id)
		}
	}
}

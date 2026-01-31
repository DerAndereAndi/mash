package rules

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

func TestEPT001_BATTERYMissingDCPower(t *testing.T) {
	rule := NewEPT001()

	// BATTERY endpoint missing dcPower -> error
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=BATTERY
MASH.S.E01.MEAS=1`)
	violations := rule.Check(p)
	hasError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("Expected error for BATTERY endpoint missing dcPower")
	}
}

func TestEPT001_BATTERYComplete(t *testing.T) {
	rule := NewEPT001()

	// BATTERY with dcPower + stateOfCharge + required ELEC attrs -> pass
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=BATTERY
MASH.S.E01.MEAS=1
MASH.S.E01.MEAS.A28=1
MASH.S.E01.MEAS.A32=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A05=1
MASH.S.E01.ELEC.A0A=1
MASH.S.E01.ELEC.A0B=1
MASH.S.E01.ELEC.A14=1`)
	violations := rule.Check(p)
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			t.Errorf("Expected no error for BATTERY with required attrs, got: %v", v)
		}
	}
}

func TestEPT001_GridConnectionMissingACPower(t *testing.T) {
	rule := NewEPT001()

	// GRID_CONNECTION missing acActivePower -> error
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=GRID_CONNECTION
MASH.S.E01.MEAS=1`)
	violations := rule.Check(p)
	hasError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("Expected error for GRID_CONNECTION missing acActivePower")
	}
}

func TestEPT001_PVStringMissingDCPower(t *testing.T) {
	rule := NewEPT001()

	// PV_STRING missing dcPower -> error
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=PV_STRING
MASH.S.E01.MEAS=1`)
	violations := rule.Check(p)
	hasError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("Expected error for PV_STRING missing dcPower")
	}
}

func TestEPT001_MissingRecommended(t *testing.T) {
	rule := NewEPT001()

	// EV_CHARGER with all mandatory but missing recommended -> warning
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.MEAS=1
MASH.S.E01.MEAS.A01=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A01=1
MASH.S.E01.ELEC.A05=1
MASH.S.E01.ELEC.A0A=1
MASH.S.E01.ELEC.A0D=1`)
	violations := rule.Check(p)
	hasWarning := false
	for _, v := range violations {
		if v.Severity == pics.SeverityWarning {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("Expected warning for EV_CHARGER missing recommended attributes")
	}
}

func TestEPT001_UnknownEndpointType(t *testing.T) {
	rule := NewEPT001()

	// Unknown endpoint type -> skip (no conformance to check)
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=CUSTOM_DEVICE
MASH.S.E01.MEAS=1`)
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violations for unknown endpoint type")
	}
}

func TestEPT001_NoEndpoints(t *testing.T) {
	rule := NewEPT001()

	// No endpoints -> no violations
	p, _ := pics.ParseString(`MASH.S=1`)
	violations := rule.Check(p)
	if len(violations) > 0 {
		t.Error("Expected no violations without endpoints")
	}
}

func TestEPT001_MultipleEndpoints(t *testing.T) {
	rule := NewEPT001()

	// One valid endpoint, one invalid
	p, _ := pics.ParseString(`MASH.S=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.MEAS=1
MASH.S.E01.MEAS.A01=1
MASH.S.E02=BATTERY
MASH.S.E02.MEAS=1`)
	violations := rule.Check(p)
	hasError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			hasError = true
			break
		}
	}
	if !hasError {
		t.Error("Expected error for BATTERY endpoint missing dcPower")
	}
}

func TestRegisterConformanceRules(t *testing.T) {
	registry := pics.NewRuleRegistry()
	RegisterConformanceRules(registry)

	rules := registry.RulesByCategory("conformance")
	if len(rules) != 1 {
		t.Errorf("Expected 1 conformance rule, got %d", len(rules))
	}

	if registry.GetRule("EPT-001") == nil {
		t.Error("Expected rule EPT-001 to be registered")
	}
}

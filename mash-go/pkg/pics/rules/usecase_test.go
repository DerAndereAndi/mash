package rules

import (
	"testing"

	"github.com/mash-protocol/mash-go/pkg/pics"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

func TestUC001_DeviceWithRequiredFeatures(t *testing.T) {
	// Device declares UC.LPC and has the required features (CTRL with acceptsLimits, ELEC)
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.C02.Rsp=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A0A=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			t.Errorf("unexpected error violation: %s", v.Message)
		}
	}
}

func TestUC001_DeviceMissingRequiredFeature(t *testing.T) {
	// Device declares UC.LPC but has no CTRL feature on any endpoint
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.ELEC=1
MASH.S.E01.MEAS=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	// Should have an error for missing EnergyControl (required for LPC)
	foundError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			foundError = true
			t.Logf("expected error: %s", v.Message)
		}
	}
	if !foundError {
		t.Error("expected error violation for missing EnergyControl feature")
	}
}

func TestUC001_DeviceMissingElectrical(t *testing.T) {
	// Device declares UC.LPC, has CTRL but missing Electrical (required for LPC)
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	foundError := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			foundError = true
			t.Logf("expected error: %s", v.Message)
		}
	}
	if !foundError {
		t.Error("expected error violation for missing Electrical feature")
	}
}

func TestUC001_ControllerNoFeatureValidation(t *testing.T) {
	// Controller PICS with UC codes should NOT produce feature-presence errors
	input := `
MASH.C=1
MASH.C.UC.LPC=1
MASH.C.UC.LPP=1
MASH.C.UC.MPD=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			t.Errorf("unexpected error violation for controller: %s", v.Message)
		}
	}
}

func TestUC001_UCNotInRegistry(t *testing.T) {
	// Device declares a use case not in the registry -- should warn
	input := `
MASH.S=1
MASH.S.UC.UNKNOWN_UC=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	foundWarning := false
	for _, v := range violations {
		if v.Severity == pics.SeverityWarning {
			foundWarning = true
			t.Logf("expected warning: %s", v.Message)
		}
	}
	if !foundWarning {
		t.Error("expected warning for unknown use case")
	}
}

func TestUC001_MultipleUseCases(t *testing.T) {
	// Device declares LPC and MPD -- both should be validated
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.UC.MPD=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.C02.Rsp=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A0A=1
MASH.S.E01.MEAS=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			t.Errorf("unexpected error: %s", v.Message)
		}
	}
}

func TestUC001_DisabledUC(t *testing.T) {
	// UC code present but set to 0 -- should not be validated
	input := `
MASH.S=1
MASH.S.UC.LPC=0
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	if len(violations) != 0 {
		t.Errorf("expected no violations for disabled UC, got %d", len(violations))
	}
}

func TestUC001_RegisteredInAll(t *testing.T) {
	registry := NewDefaultRegistry()

	rule := registry.GetRule("UC-001")
	if rule == nil {
		t.Fatal("expected UC-001 to be registered in default registry")
	}

	if rule.Category() != "usecase" {
		t.Errorf("UC-001 category = %s, want usecase", rule.Category())
	}
}

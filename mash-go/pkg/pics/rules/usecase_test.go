package rules

import (
	"strings"
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

func TestUC001_MissingPresenceOnlyAttribute(t *testing.T) {
	// LPC device with CTRL+ELEC but ELEC missing nominalMaxConsumption (A0A)
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.C02.Rsp=1
MASH.S.E01.ELEC=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	foundWarning := false
	for _, v := range violations {
		if v.Severity == pics.SeverityWarning && strings.Contains(v.Message, "ELEC.nominalMaxConsumption") {
			foundWarning = true
			t.Logf("expected warning: %s", v.Message)
		}
	}
	if !foundWarning {
		t.Error("expected warning for missing presence-only attribute ELEC.nominalMaxConsumption (A0A)")
	}
}

func TestUC001_PresenceOnlyAttributePresent(t *testing.T) {
	// LPC device with CTRL+ELEC and ELEC.A0A present -- no warning
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
		if strings.Contains(v.Message, "ELEC.nominalMaxConsumption") {
			t.Errorf("unexpected violation for present attribute: %s", v.Message)
		}
	}
}

func TestUC001_OHPCF_MissingProcessAttributes(t *testing.T) {
	// OHPCF with CTRL but missing process attributes (isPausable, processState, etc.)
	input := `
MASH.S=1
MASH.S.UC.OHPCF=1
MASH.S.E01=HEAT_PUMP
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.C01.Rsp=1
MASH.S.E01.CTRL.C02.Rsp=1
MASH.S.E01.CTRL.C09.Rsp=1
MASH.S.E01.CTRL.C0A.Rsp=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A0A=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	// Expect 8 warnings for missing process attributes:
	// isPausable (A0E), processState (A50), optionalProcess (A51),
	// minRunDuration (A52), minPauseDuration (A53), maxRunDuration (A54),
	// maxPauseDuration (A55), optionalProcessPower (A56)
	expectedAttrs := []string{
		"isPausable", "processState", "optionalProcess",
		"minRunDuration", "minPauseDuration", "maxRunDuration",
		"maxPauseDuration", "optionalProcessPower",
	}

	warningCount := 0
	for _, v := range violations {
		if v.Severity == pics.SeverityWarning {
			for _, attr := range expectedAttrs {
				if strings.Contains(v.Message, attr) {
					warningCount++
					t.Logf("expected warning: %s", v.Message)
					break
				}
			}
		}
	}
	if warningCount != 8 {
		t.Errorf("expected 8 attribute warnings, got %d", warningCount)
		for _, v := range violations {
			t.Logf("  violation: [%s] %s", v.Severity, v.Message)
		}
	}
}

func TestUC001_MissingRequiredCommands(t *testing.T) {
	// LPC device with CTRL (A0A) and ELEC (A0A) but missing C01.Rsp and C02.Rsp
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=EV_CHARGER
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.ELEC=1
MASH.S.E01.ELEC.A0A=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	foundSetLimit := false
	foundClearLimit := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			if strings.Contains(v.Message, "setLimit") || strings.Contains(v.Message, "C01") {
				foundSetLimit = true
			}
			if strings.Contains(v.Message, "clearLimit") || strings.Contains(v.Message, "C02") {
				foundClearLimit = true
			}
		}
	}
	if !foundSetLimit || !foundClearLimit {
		t.Errorf("expected errors for missing setLimit (C01) and clearLimit (C02), foundSetLimit=%v foundClearLimit=%v", foundSetLimit, foundClearLimit)
		for _, v := range violations {
			t.Logf("  violation: [%s] %s", v.Severity, v.Message)
		}
	}
}

func TestUC001_RequiredCommandsPresent(t *testing.T) {
	// LPC device with all required commands present -- no command errors
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
		if v.Severity == pics.SeverityError && strings.Contains(v.Message, "command") {
			t.Errorf("unexpected command error: %s", v.Message)
		}
	}
}

func TestUC001_OHPCF_MissingPauseResumeCommands(t *testing.T) {
	// OHPCF with setLimit/clearLimit present but missing pause (C09) and resume (C0A)
	input := `
MASH.S=1
MASH.S.UC.OHPCF=1
MASH.S.E01=HEAT_PUMP
MASH.S.E01.CTRL=1
MASH.S.E01.CTRL.A0A=1
MASH.S.E01.CTRL.A0E=1
MASH.S.E01.CTRL.A50=1
MASH.S.E01.CTRL.A51=1
MASH.S.E01.CTRL.A52=1
MASH.S.E01.CTRL.A53=1
MASH.S.E01.CTRL.A54=1
MASH.S.E01.CTRL.A55=1
MASH.S.E01.CTRL.A56=1
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

	foundPause := false
	foundResume := false
	for _, v := range violations {
		if v.Severity == pics.SeverityError {
			if strings.Contains(v.Message, "pause") || strings.Contains(v.Message, "C09") {
				foundPause = true
			}
			if strings.Contains(v.Message, "resume") || strings.Contains(v.Message, "C0A") {
				foundResume = true
			}
		}
	}
	if !foundPause || !foundResume {
		t.Errorf("expected errors for missing pause (C09) and resume (C0A), foundPause=%v foundResume=%v", foundPause, foundResume)
		for _, v := range violations {
			t.Logf("  violation: [%s] %s", v.Severity, v.Message)
		}
	}
}

func TestUC001_EndpointTypeMismatch(t *testing.T) {
	// LPC on GRID_CONNECTION endpoint (LPC requires INVERTER/EV_CHARGER/BATTERY/etc.)
	input := `
MASH.S=1
MASH.S.UC.LPC=1
MASH.S.E01=GRID_CONNECTION
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

	foundWarning := false
	for _, v := range violations {
		if v.Severity == pics.SeverityWarning && strings.Contains(v.Message, "endpoint type") {
			foundWarning = true
			t.Logf("expected warning: %s", v.Message)
		}
	}
	if !foundWarning {
		t.Error("expected warning for LPC on GRID_CONNECTION endpoint")
		for _, v := range violations {
			t.Logf("  violation: [%s] %s", v.Severity, v.Message)
		}
	}
}

func TestUC001_EndpointTypeMatch(t *testing.T) {
	// LPC on EV_CHARGER -- no endpoint type warning
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
		if strings.Contains(v.Message, "endpoint type") {
			t.Errorf("unexpected endpoint type warning: %s", v.Message)
		}
	}
}

func TestUC001_NoEndpointTypeConstraint(t *testing.T) {
	// PODF has no EndpointTypes constraint -- any endpoint type is fine
	input := `
MASH.S=1
MASH.S.UC.PODF=1
MASH.S.E01=CUSTOM_DEVICE
MASH.S.E01.PLAN=1
MASH.S.E01.PLAN.A03=1
MASH.S.E01.PLAN.A0A=1
MASH.S.E01.PLAN.A0B=1
MASH.S.E01.PLAN.A14=1
MASH.S.E01.PLAN.C01.Rsp=1
`
	p, err := pics.ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	rule := NewUC001(usecase.Registry)
	violations := rule.Check(p)

	for _, v := range violations {
		if strings.Contains(v.Message, "endpoint type") {
			t.Errorf("unexpected endpoint type warning for PODF: %s", v.Message)
		}
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

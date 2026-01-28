package loader_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestPICSValidation_V2XRequiresEMOB tests that V2X implies EMOB support.
func TestPICSValidation_V2XRequiresEMOB(t *testing.T) {
	// V2X (F0A) requires EMOB (F03) base support
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"MASH.S.CTRL.F0A": true,
			// Missing MASH.S.CTRL.F03
		},
	}

	errors := loader.ValidatePICS(pf)
	if len(errors) == 0 {
		t.Error("Expected validation error: V2X requires EMOB")
	}

	// Should pass when EMOB (F03) is present
	pf.Items["MASH.S.CTRL.F03"] = true
	errors = loader.ValidatePICS(pf)
	hasV2XError := false
	for _, e := range errors {
		if e.Field == "MASH.S.CTRL.F0A" {
			hasV2XError = true
		}
	}
	if hasV2XError {
		t.Error("V2X validation should pass when EMOB (F03) is present")
	}
}

// TestPICSValidation_FeatureImpliesAttributes tests that declaring a feature implies mandatory attributes.
func TestPICSValidation_FeatureImpliesAttributes(t *testing.T) {
	// Electrical feature should imply phase count is specified
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"MASH.S.ELEC": true,
			// Missing MASH.S.ELEC.A01 (phaseCount)
		},
	}

	errors := loader.ValidatePICS(pf)
	hasPhaseError := false
	for _, e := range errors {
		if e.Field == "MASH.S.ELEC.A01" {
			hasPhaseError = true
		}
	}
	if !hasPhaseError {
		t.Error("Expected validation error: ELEC feature requires phaseCount (A01)")
	}

	// Should pass when phases is specified
	pf.Items["MASH.S.ELEC.A01"] = 3
	errors = loader.ValidatePICS(pf)
	hasPhaseError = false
	for _, e := range errors {
		if e.Field == "MASH.S.ELEC.A01" {
			hasPhaseError = true
		}
	}
	if hasPhaseError {
		t.Error("phaseCount validation should pass when specified")
	}
}

// TestPICSValidation_NumericRanges tests that numeric values are within valid ranges.
func TestPICSValidation_NumericRanges(t *testing.T) {
	tests := []struct {
		name        string
		items       map[string]interface{}
		shouldError bool
		errorField  string
	}{
		{
			name:        "phases valid 1",
			items:       map[string]interface{}{"MASH.S.ELEC.A01": 1},
			shouldError: false,
		},
		{
			name:        "phases valid 3",
			items:       map[string]interface{}{"MASH.S.ELEC.A01": 3},
			shouldError: false,
		},
		{
			name:        "phases invalid 0",
			items:       map[string]interface{}{"MASH.S.ELEC.A01": 0},
			shouldError: true,
			errorField:  "MASH.S.ELEC.A01",
		},
		{
			name:        "phases invalid 4",
			items:       map[string]interface{}{"MASH.S.ELEC.A01": 4},
			shouldError: true,
			errorField:  "MASH.S.ELEC.A01",
		},
		{
			name:        "max zones valid",
			items:       map[string]interface{}{"MASH.S.ZONE.MAX": 5},
			shouldError: false,
		},
		{
			name:        "max zones invalid",
			items:       map[string]interface{}{"MASH.S.ZONE.MAX": 10},
			shouldError: true,
			errorField:  "MASH.S.ZONE.MAX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := &loader.PICSFile{Name: "test", Items: tt.items}
			errors := loader.ValidatePICS(pf)

			hasError := false
			for _, e := range errors {
				if e.Field == tt.errorField {
					hasError = true
				}
			}

			if tt.shouldError && !hasError {
				t.Errorf("Expected validation error for field %s", tt.errorField)
			}
			if !tt.shouldError && hasError {
				t.Errorf("Unexpected validation error for field %s", tt.errorField)
			}
		})
	}
}

// TestPICSValidation_ValidFile tests that a valid PICS file passes validation.
func TestPICSValidation_ValidFile(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "valid-ev-charger",
		Device: loader.PICSDevice{
			Vendor:  "Test Corp",
			Product: "Charger",
			Model:   "TC-1",
			Version: "1.0.0",
		},
		Items: map[string]interface{}{
			"MASH.S.TRANS.SC":    true,
			"MASH.S.TRANS.TLS13": true,
			"MASH.S.ELEC":        true,
			"MASH.S.ELEC.A01":    3,
			"MASH.S.ELEC.AC":     true,
			"MASH.S.MEAS.POWER":  true,
			"MASH.S.ZONE.MAX":    3,
			"MASH.S.CHRG":        true,
			"MASH.S.CHRG.SESSION": true,
		},
	}

	errors := loader.ValidatePICS(pf)
	if len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("Unexpected validation error: %s - %s", e.Field, e.Message)
		}
	}
}

// TestPICSValidation_ErrorLevels tests that errors have appropriate severity levels.
func TestPICSValidation_ErrorLevels(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"MASH.S.ELEC.A01": 0, // Error: invalid range
		},
	}

	errors := loader.ValidatePICS(pf)
	for _, e := range errors {
		if e.Field == "MASH.S.ELEC.A01" {
			if e.Level != loader.ValidationLevelError {
				t.Errorf("Phase range violation should be Level=Error, got %s", e.Level)
			}
		}
	}
}

// TestPICSValidation_NilAndEmpty tests handling of nil/empty PICS.
func TestPICSValidation_NilAndEmpty(t *testing.T) {
	// Nil PICS
	errors := loader.ValidatePICS(nil)
	if len(errors) != 0 {
		t.Error("ValidatePICS(nil) should return no errors")
	}

	// Empty PICS
	pf := &loader.PICSFile{Name: "empty", Items: map[string]interface{}{}}
	errors = loader.ValidatePICS(pf)
	// Empty PICS might have warnings but should not cause crashes
	for _, e := range errors {
		if e.Level == loader.ValidationLevelError {
			t.Errorf("Empty PICS should not have errors, got: %s", e.Message)
		}
	}
}

// TestPICSValidation_ChargingSessionRequiresCHRG tests ChargingSession implies CHRG feature.
func TestPICSValidation_ChargingSessionRequiresCHRG(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"MASH.S.CHRG.SESSION": true,
			// Missing MASH.S.CHRG
		},
	}

	errors := loader.ValidatePICS(pf)
	hasError := false
	for _, e := range errors {
		if e.Field == "MASH.S.CHRG.SESSION" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("Expected validation error: CHRG.SESSION requires CHRG feature")
	}
}

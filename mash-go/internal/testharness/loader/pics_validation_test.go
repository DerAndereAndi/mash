package loader_test

import (
	"testing"

	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// TestPICSValidation_V2XRequiresEMOB tests that V2X implies EMOB support.
func TestPICSValidation_V2XRequiresEMOB(t *testing.T) {
	// V2X (bidirectional EV) requires EMOB (e-mobility) base support
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"D.EMOB.V2X": true,
			// Missing D.EMOB.BASE
		},
	}

	errors := loader.ValidatePICS(pf)
	if len(errors) == 0 {
		t.Error("Expected validation error: V2X requires EMOB")
	}

	// Should pass when EMOB is present
	pf.Items["D.EMOB.BASE"] = true
	errors = loader.ValidatePICS(pf)
	hasV2XError := false
	for _, e := range errors {
		if e.Field == "D.EMOB.V2X" {
			hasV2XError = true
		}
	}
	if hasV2XError {
		t.Error("V2X validation should pass when EMOB.BASE is present")
	}
}

// TestPICSValidation_FeatureImpliesAttributes tests that declaring a feature implies mandatory attributes.
func TestPICSValidation_FeatureImpliesAttributes(t *testing.T) {
	// Electrical feature should imply phase count is specified
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"D.ELEC.PRESENT": true,
			// Missing D.ELEC.PHASES
		},
	}

	errors := loader.ValidatePICS(pf)
	hasPhaseError := false
	for _, e := range errors {
		if e.Field == "D.ELEC.PHASES" {
			hasPhaseError = true
		}
	}
	if !hasPhaseError {
		t.Error("Expected validation error: ELEC feature requires PHASES")
	}

	// Should pass when phases is specified
	pf.Items["D.ELEC.PHASES"] = 3
	errors = loader.ValidatePICS(pf)
	hasPhaseError = false
	for _, e := range errors {
		if e.Field == "D.ELEC.PHASES" {
			hasPhaseError = true
		}
	}
	if hasPhaseError {
		t.Error("PHASES validation should pass when specified")
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
			items:       map[string]interface{}{"D.ELEC.PHASES": 1},
			shouldError: false,
		},
		{
			name:        "phases valid 3",
			items:       map[string]interface{}{"D.ELEC.PHASES": 3},
			shouldError: false,
		},
		{
			name:        "phases invalid 0",
			items:       map[string]interface{}{"D.ELEC.PHASES": 0},
			shouldError: true,
			errorField:  "D.ELEC.PHASES",
		},
		{
			name:        "phases invalid 4",
			items:       map[string]interface{}{"D.ELEC.PHASES": 4},
			shouldError: true,
			errorField:  "D.ELEC.PHASES",
		},
		{
			name:        "max zones valid",
			items:       map[string]interface{}{"D.ZONE.MAX": 5},
			shouldError: false,
		},
		{
			name:        "max zones invalid",
			items:       map[string]interface{}{"D.ZONE.MAX": 10},
			shouldError: true,
			errorField:  "D.ZONE.MAX",
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
			"D.COMM.SC":       true,
			"D.COMM.TLS13":    true,
			"D.ELEC.PRESENT":  true,
			"D.ELEC.PHASES":   3,
			"D.ELEC.AC":       true,
			"D.MEAS.POWER":    true,
			"D.ZONE.MAX":      3,
			"D.EMOB.BASE":     true,
			"D.CHARGE.SESSION": true,
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
			"D.ELEC.PHASES": 0, // Error: invalid range
		},
	}

	errors := loader.ValidatePICS(pf)
	for _, e := range errors {
		if e.Field == "D.ELEC.PHASES" {
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

// TestPICSValidation_ChargingSessionRequiresEMOB tests ChargingSession implies EMOB.
func TestPICSValidation_ChargingSessionRequiresEMOB(t *testing.T) {
	pf := &loader.PICSFile{
		Name: "test",
		Items: map[string]interface{}{
			"D.CHARGE.SESSION": true,
			// Missing D.EMOB.BASE
		},
	}

	errors := loader.ValidatePICS(pf)
	hasError := false
	for _, e := range errors {
		if e.Field == "D.CHARGE.SESSION" {
			hasError = true
		}
	}
	if !hasError {
		t.Error("Expected validation error: CHARGE.SESSION requires EMOB.BASE")
	}
}

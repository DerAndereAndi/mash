package pics

import (
	"testing"
)

func TestValidateProtocolDeclaration(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		wantError string
	}{
		{
			name:      "valid server PICS",
			input:     "MASH.S=1\nMASH.S.VERSION=1",
			wantValid: true,
		},
		{
			name:      "valid client PICS",
			input:     "MASH.C=1\nMASH.C.VERSION=1",
			wantValid: true,
		},
		{
			name:      "missing protocol declaration",
			input:     "MASH.S.CTRL=1",
			wantValid: false,
			wantError: "PROTOCOL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pics, err := ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}

			result := ValidatePICS(pics)
			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if tt.wantError != "" {
				found := false
				for _, e := range result.Errors {
					if e.Code == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error with code %s, got %v", tt.wantError, result.Errors)
				}
			}
		})
	}
}

func TestValidateFeatureFlagDependencies(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValid bool
		wantError string
	}{
		{
			name: "V2X without EMOB",
			input: `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.F0A=1
`,
			wantValid: false,
			wantError: "DEPENDENCY",
		},
		{
			name: "V2X with EMOB",
			input: `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.F03=1
MASH.S.CTRL.F0A=1
`,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pics, err := ParseString(tt.input)
			if err != nil {
				t.Fatalf("ParseString failed: %v", err)
			}

			result := ValidatePICS(pics)
			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v; errors: %v", result.Valid, tt.wantValid, result.Errors)
			}

			if tt.wantError != "" && tt.wantValid == false {
				found := false
				for _, e := range result.Errors {
					if e.Code == tt.wantError {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error with code %s, got %v", tt.wantError, result.Errors)
				}
			}
		})
	}
}

func TestValidateStrictMode(t *testing.T) {
	// PICS with acceptsLimits but missing SetLimit command
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.A0A=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Non-strict mode should pass
	result := ValidatePICS(pics)
	if !result.Valid {
		t.Errorf("Non-strict validation should pass, got errors: %v", result.Errors)
	}

	// Strict mode should fail
	result = ValidatePICSStrict(pics)
	if result.Valid {
		t.Error("Strict validation should fail for missing SetLimit command")
	}

	// Check for the specific error
	found := false
	for _, e := range result.Errors {
		if e.Code == "CONSISTENCY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected CONSISTENCY error, got %v", result.Errors)
	}
}

func TestMeetsRequirements(t *testing.T) {
	input := `
MASH.S=1
MASH.S.CTRL=1
MASH.S.CTRL.A0A=1
MASH.S.CTRL.C01.Rsp=1
MASH.S.ELEC=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	tests := []struct {
		name         string
		requirements []string
		wantMeets    bool
		wantMissing  int
	}{
		{
			name:         "all present",
			requirements: []string{"MASH.S", "MASH.S.CTRL", "MASH.S.CTRL.A0A"},
			wantMeets:    true,
			wantMissing:  0,
		},
		{
			name:         "one missing",
			requirements: []string{"MASH.S", "MASH.S.CTRL", "MASH.S.MEAS"},
			wantMeets:    false,
			wantMissing:  1,
		},
		{
			name:         "negation - should not have",
			requirements: []string{"MASH.S", "!MASH.S.CTRL.F0A"},
			wantMeets:    true,
			wantMissing:  0,
		},
		{
			name:         "negation - has but should not",
			requirements: []string{"MASH.S", "!MASH.S.CTRL"},
			wantMeets:    false,
			wantMissing:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meets, missing := MeetsRequirements(pics, tt.requirements)
			if meets != tt.wantMeets {
				t.Errorf("MeetsRequirements() = %v, want %v; missing: %v", meets, tt.wantMeets, missing)
			}
			if len(missing) != tt.wantMissing {
				t.Errorf("len(missing) = %d, want %d; missing: %v", len(missing), tt.wantMissing, missing)
			}
		})
	}
}

func TestValidationResult(t *testing.T) {
	result := &ValidationResult{Valid: true}

	// Add error should set Valid to false
	result.AddError("TEST", "test error", 10)
	if result.Valid {
		t.Error("Valid should be false after AddError")
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Line != 10 {
		t.Errorf("expected line 10, got %d", result.Errors[0].Line)
	}

	// Add warning should not change Valid
	result = &ValidationResult{Valid: true}
	result.AddWarning("TEST", "test warning", 0)
	if !result.Valid {
		t.Error("Valid should still be true after AddWarning")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(result.Warnings))
	}
}

func TestValidationErrorString(t *testing.T) {
	// With line number
	e := ValidationError{Code: "TEST", Message: "test message", Line: 5}
	expected := "line 5: TEST: test message"
	if e.Error() != expected {
		t.Errorf("Error() = %s, want %s", e.Error(), expected)
	}

	// Without line number
	e = ValidationError{Code: "TEST", Message: "test message", Line: 0}
	expected = "TEST: test message"
	if e.Error() != expected {
		t.Errorf("Error() = %s, want %s", e.Error(), expected)
	}
}

func TestValidateFullPICS(t *testing.T) {
	// A complete, valid PICS file
	input := `
# Full-featured device PICS
MASH.S=1
MASH.S.VERSION=1

# Features
MASH.S.CTRL=1
MASH.S.ELEC=1
MASH.S.MEAS=1
MASH.S.STAT=1

# Feature flags
MASH.S.CTRL.F00=1
MASH.S.CTRL.F03=1
MASH.S.CTRL.F0A=1

# Mandatory EnergyControl attributes
MASH.S.CTRL.A01=1
MASH.S.CTRL.A02=1
MASH.S.CTRL.A0A=1
MASH.S.CTRL.A0B=1
MASH.S.CTRL.A0C=1
MASH.S.CTRL.A0E=1
MASH.S.CTRL.A46=1
MASH.S.CTRL.A48=1

# Commands
MASH.S.CTRL.C01.Rsp=1
MASH.S.CTRL.C02.Rsp=1
MASH.S.CTRL.C05.Rsp=1

# Mandatory Electrical attributes
MASH.S.ELEC.A01=1
MASH.S.ELEC.A05=1
`

	pics, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Should pass both normal and strict validation
	result := ValidatePICS(pics)
	if !result.Valid {
		t.Errorf("Normal validation should pass, got errors: %v", result.Errors)
	}

	result = ValidatePICSStrict(pics)
	if !result.Valid {
		t.Errorf("Strict validation should pass, got errors: %v", result.Errors)
	}
}

package pics

import (
	"fmt"
	"strings"
)

// ValidationError represents a PICS validation error.
type ValidationError struct {
	Code    string
	Message string
	Line    int
}

func (e ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s: %s", e.Line, e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ValidationResult contains the results of PICS validation.
type ValidationResult struct {
	// Valid is true if the PICS passed all validation checks.
	Valid bool

	// Errors contains all validation errors.
	Errors []ValidationError

	// Warnings contains non-fatal issues.
	Warnings []ValidationError
}

// AddError adds a validation error.
func (r *ValidationResult) AddError(code, message string, line int) {
	r.Errors = append(r.Errors, ValidationError{
		Code:    code,
		Message: message,
		Line:    line,
	})
	r.Valid = false
}

// AddWarning adds a validation warning.
func (r *ValidationResult) AddWarning(code, message string, line int) {
	r.Warnings = append(r.Warnings, ValidationError{
		Code:    code,
		Message: message,
		Line:    line,
	})
}

// Validator validates PICS files against conformance rules.
type Validator struct {
	// Strict enables strict validation (e.g., require all mandatory attributes).
	Strict bool
}

// NewValidator creates a new PICS validator.
func NewValidator() *Validator {
	return &Validator{
		Strict: false,
	}
}

// Validate validates a PICS file.
func (v *Validator) Validate(pics *PICS) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Check protocol declaration
	v.checkProtocolDeclaration(pics, result)

	// Check feature flag dependencies
	v.checkFeatureFlagDependencies(pics, result)

	// Check command consistency (if strict)
	if v.Strict {
		v.checkCommandConsistency(pics, result)
		v.checkMandatoryAttributes(pics, result)
	}

	return result
}

// checkProtocolDeclaration verifies MASH.S or MASH.C is present.
func (v *Validator) checkProtocolDeclaration(pics *PICS, result *ValidationResult) {
	hasServer := pics.Has("MASH.S")
	hasClient := pics.Has("MASH.C")

	if !hasServer && !hasClient {
		result.AddError("PROTOCOL", "missing protocol declaration (MASH.S or MASH.C required)", 0)
	}

	if hasServer && hasClient {
		result.AddWarning("PROTOCOL", "both MASH.S and MASH.C declared (unusual)", 0)
	}
}

// checkFeatureFlagDependencies verifies feature flag dependencies.
func (v *Validator) checkFeatureFlagDependencies(pics *PICS, result *ValidationResult) {
	side := string(pics.Side)

	// V2X (F0A) requires EMOB (F03)
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.F0A", side)) {
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.F03", side)) {
			result.AddError("DEPENDENCY", "V2X (F0A) requires EMOB (F03)", 0)
		}
	}

	// ASYMMETRIC (F09) requires phaseCount > 1 (from Electrical feature)
	// This is a soft check since we don't have the actual phaseCount value
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.F09", side)) {
		if !pics.HasFeature("ELEC") {
			result.AddWarning("DEPENDENCY", "ASYMMETRIC (F09) typically requires Electrical feature", 0)
		}
	}

	// SIGNALS (F04) should have Signals feature
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.F04", side)) {
		if !pics.HasFeature("SIG") {
			result.AddWarning("DEPENDENCY", "SIGNALS flag (F04) should have Signals feature enabled", 0)
		}
	}

	// PLAN (F06) should have Plan feature
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.F06", side)) {
		if !pics.HasFeature("PLAN") {
			result.AddWarning("DEPENDENCY", "PLAN flag (F06) should have Plan feature enabled", 0)
		}
	}
}

// checkCommandConsistency verifies command-attribute consistency.
func (v *Validator) checkCommandConsistency(pics *PICS, result *ValidationResult) {
	side := string(pics.Side)

	// If acceptsLimits (A0A), must support SetLimit (C01)
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.A0A", side)) {
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.C01.Rsp", side)) {
			result.AddError("CONSISTENCY", "acceptsLimits (A0A) requires SetLimit command (C01.Rsp)", 0)
		}
	}

	// If acceptsCurrentLimits (A0B), must support SetCurrentLimits (C05)
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.A0B", side)) {
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.C05.Rsp", side)) {
			result.AddError("CONSISTENCY", "acceptsCurrentLimits (A0B) requires SetCurrentLimits command (C05.Rsp)", 0)
		}
	}

	// If acceptsSetpoints (A0C), must support SetSetpoint (C03)
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.A0C", side)) {
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.C03.Rsp", side)) {
			result.AddWarning("CONSISTENCY", "acceptsSetpoints (A0C) typically requires SetSetpoint command", 0)
		}
	}

	// If isPausable (A0E), must support Pause (C09) and Resume (C0A)
	if pics.Has(fmt.Sprintf("MASH.%s.CTRL.A0E", side)) {
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.C09.Rsp", side)) {
			result.AddWarning("CONSISTENCY", "isPausable (A0E) typically requires Pause command (C09.Rsp)", 0)
		}
		if !pics.Has(fmt.Sprintf("MASH.%s.CTRL.C0A.Rsp", side)) {
			result.AddWarning("CONSISTENCY", "isPausable (A0E) typically requires Resume command (C0A.Rsp)", 0)
		}
	}
}

// checkMandatoryAttributes verifies mandatory attributes are present.
func (v *Validator) checkMandatoryAttributes(pics *PICS, result *ValidationResult) {
	side := string(pics.Side)

	// EnergyControl mandatory attributes
	if pics.HasFeature("CTRL") {
		mandatory := []struct {
			id   string
			name string
		}{
			{"01", "deviceType"},
			{"02", "controlState"},
			{"0A", "acceptsLimits"},
			{"0B", "acceptsCurrentLimits"},
			{"0C", "acceptsSetpoints"},
			{"0E", "isPausable"},
			{"46", "failsafeConsumptionLimit"},
			{"48", "failsafeDuration"},
		}

		for _, attr := range mandatory {
			code := fmt.Sprintf("MASH.%s.CTRL.A%s", side, attr.id)
			if !pics.Has(code) {
				result.AddError("MANDATORY", fmt.Sprintf("EnergyControl requires %s (A%s)", attr.name, attr.id), 0)
			}
		}
	}

	// Electrical mandatory attributes
	if pics.HasFeature("ELEC") {
		mandatory := []struct {
			id   string
			name string
		}{
			{"01", "phaseCount"},
			{"05", "supportedDirections"},
		}

		for _, attr := range mandatory {
			code := fmt.Sprintf("MASH.%s.ELEC.A%s", side, attr.id)
			if !pics.Has(code) {
				result.AddError("MANDATORY", fmt.Sprintf("Electrical requires %s (A%s)", attr.name, attr.id), 0)
			}
		}
	}
}

// ValidatePICS is a convenience function to validate a PICS.
func ValidatePICS(pics *PICS) *ValidationResult {
	return NewValidator().Validate(pics)
}

// ValidatePICSStrict validates with strict mode enabled.
func ValidatePICSStrict(pics *PICS) *ValidationResult {
	v := NewValidator()
	v.Strict = true
	return v.Validate(pics)
}

// ValidateOptions configures validation behavior.
type ValidateOptions struct {
	// Registry is the rule registry to use. If nil, uses legacy validation.
	Registry *RuleRegistry
	// MinSeverity filters violations to only those at or above this severity.
	MinSeverity Severity
	// DisabledRules is a list of rule IDs to disable.
	DisabledRules []string
	// EnabledCategories limits validation to rules in these categories.
	// If empty, all categories are included.
	EnabledCategories []string
}

// ValidateWithOptions validates a PICS using the rule registry system.
func (v *Validator) ValidateWithOptions(pics *PICS, opts ValidateOptions) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if opts.Registry == nil {
		// Fall back to legacy validation
		return v.Validate(pics)
	}

	// Apply disabled rules
	for _, id := range opts.DisabledRules {
		opts.Registry.Disable(id)
	}

	// Apply category filter
	if len(opts.EnabledCategories) > 0 {
		opts.Registry.DisableAll()
		for _, cat := range opts.EnabledCategories {
			opts.Registry.EnableCategory(cat)
		}
	}

	// Run rules
	violations := opts.Registry.RunRules(pics)

	// Convert violations to ValidationResult
	for _, v := range violations {
		// Filter by severity
		if v.Severity > opts.MinSeverity {
			continue
		}

		line := 0
		if len(v.LineNumbers) > 0 {
			line = v.LineNumbers[0]
		}

		switch v.Severity {
		case SeverityError:
			result.AddError(v.RuleID, v.Message, line)
		case SeverityWarning:
			result.AddWarning(v.RuleID, v.Message, line)
		default:
			// Info level - add as warning to avoid breaking backward compatibility
			result.AddWarning(v.RuleID, v.Message, line)
		}
	}

	return result
}

// ValidateWithRegistry validates using all rules in the provided registry.
func ValidateWithRegistry(pics *PICS, registry *RuleRegistry) *ValidationResult {
	v := NewValidator()
	return v.ValidateWithOptions(pics, ValidateOptions{
		Registry:    registry,
		MinSeverity: SeverityWarning,
	})
}

// MeetsRequirements checks if the PICS meets a set of requirements.
// Requirements are PICS codes that must be present and true.
func MeetsRequirements(pics *PICS, requirements []string) (bool, []string) {
	var missing []string
	for _, req := range requirements {
		// Handle negation (e.g., "!MASH.S.CTRL.F0A" means must NOT have)
		if strings.HasPrefix(req, "!") {
			code := req[1:]
			if pics.Has(code) {
				missing = append(missing, req+" (should NOT be present)")
			}
		} else {
			if !pics.Has(req) {
				missing = append(missing, req)
			}
		}
	}
	return len(missing) == 0, missing
}

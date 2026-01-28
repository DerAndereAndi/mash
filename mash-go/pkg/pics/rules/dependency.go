// Package rules contains PICS validation rules.
package rules

import (
	"fmt"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

// RegisterDependencyRules registers all dependency rules with the given registry.
func RegisterDependencyRules(registry *pics.RuleRegistry) {
	registry.Register(NewDEP001())
	registry.Register(NewDEP002())
	registry.Register(NewDEP003())
	registry.Register(NewDEP004())
	registry.Register(NewDEP005())
	registry.Register(NewDEP006())
	registry.Register(NewDEP007())
	registry.Register(NewDEP008())
	registry.Register(NewDEP009())
	registry.Register(NewDEP010())
}

// DEP001 checks that V2X (F0A) requires EMOB (F03).
type DEP001 struct {
	*pics.BaseRule
}

func NewDEP001() *DEP001 {
	return &DEP001{
		BaseRule: pics.NewBaseRule("DEP-001", "V2X requires EMOB", "dependency", pics.SeverityError),
	}
}

func (r *DEP001) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	v2xCode := fmt.Sprintf("MASH.%s.CTRL.F0A", side)
	emobCode := fmt.Sprintf("MASH.%s.CTRL.F03", side)

	if !p.Has(v2xCode) {
		return nil // Rule not applicable
	}

	if !p.Has(emobCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "V2X feature flag (F0A) requires EMOB feature flag (F03)",
			PICSCodes:  []string{v2xCode, emobCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable EMOB", emobCode),
		}}
	}
	return nil
}

// DEP002 checks that ASYMMETRIC (F09) requires phaseCount > 1.
type DEP002 struct {
	*pics.BaseRule
}

func NewDEP002() *DEP002 {
	return &DEP002{
		BaseRule: pics.NewBaseRule("DEP-002", "ASYMMETRIC requires multi-phase", "dependency", pics.SeverityWarning),
	}
}

func (r *DEP002) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	asymCode := fmt.Sprintf("MASH.%s.CTRL.F09", side)
	elecCode := fmt.Sprintf("MASH.%s.ELEC", side)
	phaseCode := fmt.Sprintf("MASH.%s.ELEC.A01", side)

	if !p.Has(asymCode) {
		return nil // Rule not applicable
	}

	// Check if Electrical feature is present
	if !p.Has(elecCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "ASYMMETRIC flag (F09) typically requires Electrical feature",
			PICSCodes:  []string{asymCode, elecCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable Electrical feature", elecCode),
		}}
	}

	// Check phaseCount if available
	phaseCount := p.GetInt(phaseCode)
	if phaseCount > 0 && phaseCount == 1 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "ASYMMETRIC flag (F09) requires phaseCount > 1",
			PICSCodes:  []string{asymCode, phaseCode},
			Suggestion: "Set phaseCount to 3 for three-phase support",
		}}
	}

	return nil
}

// DEP003 checks that SIGNALS (F04) requires SIG feature.
type DEP003 struct {
	*pics.BaseRule
}

func NewDEP003() *DEP003 {
	return &DEP003{
		BaseRule: pics.NewBaseRule("DEP-003", "SIGNALS requires SIG feature", "dependency", pics.SeverityError),
	}
}

func (r *DEP003) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F04", side)
	featureCode := fmt.Sprintf("MASH.%s.SIG", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	if !p.Has(featureCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "SIGNALS flag (F04) requires Signals feature",
			PICSCodes:  []string{flagCode, featureCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable Signals feature", featureCode),
		}}
	}
	return nil
}

// DEP004 checks that TARIFF (F05) requires TAR feature.
type DEP004 struct {
	*pics.BaseRule
}

func NewDEP004() *DEP004 {
	return &DEP004{
		BaseRule: pics.NewBaseRule("DEP-004", "TARIFF requires TAR feature", "dependency", pics.SeverityError),
	}
}

func (r *DEP004) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F05", side)
	featureCode := fmt.Sprintf("MASH.%s.TAR", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	if !p.Has(featureCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "TARIFF flag (F05) requires Tariff feature",
			PICSCodes:  []string{flagCode, featureCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable Tariff feature", featureCode),
		}}
	}
	return nil
}

// DEP005 checks that PLAN (F06) requires PLAN feature.
type DEP005 struct {
	*pics.BaseRule
}

func NewDEP005() *DEP005 {
	return &DEP005{
		BaseRule: pics.NewBaseRule("DEP-005", "PLAN flag requires PLAN feature", "dependency", pics.SeverityError),
	}
}

func (r *DEP005) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F06", side)
	featureCode := fmt.Sprintf("MASH.%s.PLAN", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	if !p.Has(featureCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "PLAN flag (F06) requires Plan feature",
			PICSCodes:  []string{flagCode, featureCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable Plan feature", featureCode),
		}}
	}
	return nil
}

// DEP006 checks that PROCESS (F07) requires processState (A50) and optionalProcess (A51).
type DEP006 struct {
	*pics.BaseRule
}

func NewDEP006() *DEP006 {
	return &DEP006{
		BaseRule: pics.NewBaseRule("DEP-006", "PROCESS requires process attributes", "dependency", pics.SeverityError),
	}
}

func (r *DEP006) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F07", side)
	a50Code := fmt.Sprintf("MASH.%s.CTRL.A50", side)
	a51Code := fmt.Sprintf("MASH.%s.CTRL.A51", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	var violations []pics.Violation
	var missing []string

	if !p.Has(a50Code) {
		missing = append(missing, "processState (A50)")
	}
	if !p.Has(a51Code) {
		missing = append(missing, "optionalProcess (A51)")
	}

	if len(missing) > 0 {
		violations = append(violations, pics.Violation{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("PROCESS flag (F07) requires: %v", missing),
			PICSCodes:  []string{flagCode, a50Code, a51Code},
			Suggestion: "Add required process attributes",
		})
	}

	return violations
}

// DEP007 checks that FORECAST (F08) requires forecast attribute (A3D).
type DEP007 struct {
	*pics.BaseRule
}

func NewDEP007() *DEP007 {
	return &DEP007{
		BaseRule: pics.NewBaseRule("DEP-007", "FORECAST requires forecast attribute", "dependency", pics.SeverityError),
	}
}

func (r *DEP007) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F08", side)
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A3D", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	if !p.Has(attrCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "FORECAST flag (F08) requires forecast attribute (A3D)",
			PICSCodes:  []string{flagCode, attrCode},
			Suggestion: fmt.Sprintf("Add %s=1 to declare forecast support", attrCode),
		}}
	}
	return nil
}

// DEP008 checks that EMOB (F03) requires CHRG feature.
type DEP008 struct {
	*pics.BaseRule
}

func NewDEP008() *DEP008 {
	return &DEP008{
		BaseRule: pics.NewBaseRule("DEP-008", "EMOB requires CHRG feature", "dependency", pics.SeverityError),
	}
}

func (r *DEP008) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F03", side)
	featureCode := fmt.Sprintf("MASH.%s.CHRG", side)

	if !p.Has(flagCode) {
		return nil // Rule not applicable
	}

	if !p.Has(featureCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "EMOB flag (F03) requires ChargingSession feature",
			PICSCodes:  []string{flagCode, featureCode},
			Suggestion: fmt.Sprintf("Add %s=1 to enable ChargingSession feature", featureCode),
		}}
	}
	return nil
}

// DEP009 checks that V2X requires BIDIRECTIONAL direction.
type DEP009 struct {
	*pics.BaseRule
}

func NewDEP009() *DEP009 {
	return &DEP009{
		BaseRule: pics.NewBaseRule("DEP-009", "V2X requires BIDIRECTIONAL", "dependency", pics.SeverityWarning),
	}
}

func (r *DEP009) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	v2xCode := fmt.Sprintf("MASH.%s.CTRL.F0A", side)
	elecCode := fmt.Sprintf("MASH.%s.ELEC", side)
	dirCode := fmt.Sprintf("MASH.%s.ELEC.A05", side) // supportedDirections

	if !p.Has(v2xCode) {
		return nil // Rule not applicable
	}

	// Check if Electrical feature is present and has supportedDirections
	if !p.Has(elecCode) {
		return nil // Covered by other rules
	}

	// If supportedDirections is declared, we can check it
	// Note: Without the actual value, we can only warn
	if p.Has(dirCode) {
		// The value should include BIDIRECTIONAL; this is informational
		// since we can't easily check the string value meaning
		return nil
	}

	return []pics.Violation{{
		RuleID:     r.ID(),
		Severity:   r.DefaultSeverity(),
		Message:    "V2X (F0A) typically requires BIDIRECTIONAL in supportedDirections",
		PICSCodes:  []string{v2xCode, dirCode},
		Suggestion: "Ensure supportedDirections includes BIDIRECTIONAL",
	}}
}

// DEP010 warns about BATTERY (F02) and EMOB (F03) mutual exclusion.
type DEP010 struct {
	*pics.BaseRule
}

func NewDEP010() *DEP010 {
	return &DEP010{
		BaseRule: pics.NewBaseRule("DEP-010", "BATTERY and EMOB mutual exclusion", "dependency", pics.SeverityWarning),
	}
}

func (r *DEP010) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	batteryCode := fmt.Sprintf("MASH.%s.CTRL.F02", side)
	emobCode := fmt.Sprintf("MASH.%s.CTRL.F03", side)

	hasBattery := p.Has(batteryCode)
	hasEMOB := p.Has(emobCode)

	// Only warn if both are present
	if hasBattery && hasEMOB {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "BATTERY (F02) and EMOB (F03) are typically mutually exclusive - a device is usually one or the other",
			PICSCodes:  []string{batteryCode, emobCode},
			Suggestion: "Verify this device genuinely supports both stationary battery and e-mobility charging",
		}}
	}
	return nil
}

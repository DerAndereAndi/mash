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

// epCode builds an endpoint-scoped PICS code string.
func epCode(side string, epID uint8, suffix string) string {
	return fmt.Sprintf("MASH.%s.E%02X.%s", side, epID, suffix)
}

// DEP001 checks that V2X (F0A) requires EMOB (F03) per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		v2xCode := epCode(side, ep.ID, "CTRL.F0A")
		emobCode := epCode(side, ep.ID, "CTRL.F03")

		if !p.Has(v2xCode) {
			continue
		}
		if !p.Has(emobCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): V2X feature flag (F0A) requires EMOB feature flag (F03)", ep.ID, ep.Type),
				PICSCodes:  []string{v2xCode, emobCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable EMOB", emobCode),
			})
		}
	}

	return violations
}

// DEP002 checks that ASYMMETRIC (F09) requires phaseCount > 1 per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		asymCode := epCode(side, ep.ID, "CTRL.F09")
		if !p.Has(asymCode) {
			continue
		}

		elecCode := epCode(side, ep.ID, "ELEC")
		phaseCode := epCode(side, ep.ID, "ELEC.A01")

		if !p.Has(elecCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): ASYMMETRIC flag (F09) typically requires Electrical feature", ep.ID, ep.Type),
				PICSCodes:  []string{asymCode, elecCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable Electrical feature", elecCode),
			})
			continue
		}

		phaseCount := p.GetInt(phaseCode)
		if phaseCount > 0 && phaseCount == 1 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): ASYMMETRIC flag (F09) requires phaseCount > 1", ep.ID, ep.Type),
				PICSCodes:  []string{asymCode, phaseCode},
				Suggestion: "Set phaseCount to 3 for three-phase support",
			})
		}
	}

	return violations
}

// DEP003 checks that SIGNALS (F04) requires SIG feature per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F04")
		featureCode := epCode(side, ep.ID, "SIG")

		if !p.Has(flagCode) {
			continue
		}
		if !p.Has(featureCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): SIGNALS flag (F04) requires Signals feature", ep.ID, ep.Type),
				PICSCodes:  []string{flagCode, featureCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable Signals feature", featureCode),
			})
		}
	}

	return violations
}

// DEP004 checks that TARIFF (F05) requires TAR feature per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F05")
		featureCode := epCode(side, ep.ID, "TAR")

		if !p.Has(flagCode) {
			continue
		}
		if !p.Has(featureCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): TARIFF flag (F05) requires Tariff feature", ep.ID, ep.Type),
				PICSCodes:  []string{flagCode, featureCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable Tariff feature", featureCode),
			})
		}
	}

	return violations
}

// DEP005 checks that PLAN (F06) requires PLAN feature per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F06")
		featureCode := epCode(side, ep.ID, "PLAN")

		if !p.Has(flagCode) {
			continue
		}
		if !p.Has(featureCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): PLAN flag (F06) requires Plan feature", ep.ID, ep.Type),
				PICSCodes:  []string{flagCode, featureCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable Plan feature", featureCode),
			})
		}
	}

	return violations
}

// DEP006 checks that PROCESS (F07) requires processState (A50) and optionalProcess (A51) per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F07")
		a50Code := epCode(side, ep.ID, "CTRL.A50")
		a51Code := epCode(side, ep.ID, "CTRL.A51")

		if !p.Has(flagCode) {
			continue
		}

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
				Message:    fmt.Sprintf("Endpoint %d (%s): PROCESS flag (F07) requires: %v", ep.ID, ep.Type, missing),
				PICSCodes:  []string{flagCode, a50Code, a51Code},
				Suggestion: "Add required process attributes",
			})
		}
	}

	return violations
}

// DEP007 checks that FORECAST (F08) requires forecast attribute (A3D) per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F08")
		attrCode := epCode(side, ep.ID, "CTRL.A3D")

		if !p.Has(flagCode) {
			continue
		}
		if !p.Has(attrCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): FORECAST flag (F08) requires forecast attribute (A3D)", ep.ID, ep.Type),
				PICSCodes:  []string{flagCode, attrCode},
				Suggestion: fmt.Sprintf("Add %s=1 to declare forecast support", attrCode),
			})
		}
	}

	return violations
}

// DEP008 checks that EMOB (F03) requires CHRG feature per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F03")
		featureCode := epCode(side, ep.ID, "CHRG")

		if !p.Has(flagCode) {
			continue
		}
		if !p.Has(featureCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): EMOB flag (F03) requires ChargingSession feature", ep.ID, ep.Type),
				PICSCodes:  []string{flagCode, featureCode},
				Suggestion: fmt.Sprintf("Add %s=1 to enable ChargingSession feature", featureCode),
			})
		}
	}

	return violations
}

// DEP009 checks that V2X requires BIDIRECTIONAL direction per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		v2xCode := epCode(side, ep.ID, "CTRL.F0A")
		elecCode := epCode(side, ep.ID, "ELEC")
		dirCode := epCode(side, ep.ID, "ELEC.A05")

		if !p.Has(v2xCode) {
			continue
		}
		if !p.Has(elecCode) {
			continue // Covered by other rules
		}

		if !p.Has(dirCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): V2X (F0A) typically requires BIDIRECTIONAL in supportedDirections", ep.ID, ep.Type),
				PICSCodes:  []string{v2xCode, dirCode},
				Suggestion: "Ensure supportedDirections includes BIDIRECTIONAL",
			})
		}
	}

	return violations
}

// DEP010 warns about BATTERY (F02) and EMOB (F03) mutual exclusion per endpoint.
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
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		batteryCode := epCode(side, ep.ID, "CTRL.F02")
		emobCode := epCode(side, ep.ID, "CTRL.F03")

		hasBattery := p.Has(batteryCode)
		hasEMOB := p.Has(emobCode)

		if hasBattery && hasEMOB {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): BATTERY (F02) and EMOB (F03) are typically mutually exclusive", ep.ID, ep.Type),
				PICSCodes:  []string{batteryCode, emobCode},
				Suggestion: "Verify this endpoint genuinely supports both stationary battery and e-mobility charging",
			})
		}
	}

	return violations
}

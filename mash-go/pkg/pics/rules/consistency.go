package rules

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

// RegisterConsistencyRules registers all command consistency rules with the given registry.
func RegisterConsistencyRules(registry *pics.RuleRegistry) {
	registry.Register(NewCMD001())
	registry.Register(NewCMD002())
	registry.Register(NewCMD003())
	registry.Register(NewCMD004())
	registry.Register(NewCMD005())
	registry.Register(NewCMD006())
	registry.Register(NewCMD007())
	registry.Register(NewCMD008())
	registry.Register(NewCMD009())
	registry.Register(NewCMD010())
	registry.Register(NewCMD011())
}

// CMD001 checks that acceptsLimits (A0A) requires SetLimit (C01) and ClearLimit (C02) per endpoint.
type CMD001 struct {
	*pics.BaseRule
}

func NewCMD001() *CMD001 {
	return &CMD001{
		BaseRule: pics.NewBaseRule("CMD-001", "acceptsLimits requires limit commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD001) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		attrCode := epCode(side, ep.ID, "CTRL.A0A")
		if !p.Has(attrCode) {
			continue
		}

		c01Code := epCode(side, ep.ID, "CTRL.C01.Rsp")
		c02Code := epCode(side, ep.ID, "CTRL.C02.Rsp")

		var missing []string
		if !p.Has(c01Code) {
			missing = append(missing, "SetLimit (C01.Rsp)")
		}
		if !p.Has(c02Code) {
			missing = append(missing, "ClearLimit (C02.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): acceptsLimits (A0A) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{attrCode, c01Code, c02Code},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD002 checks that acceptsCurrentLimits (A0B) requires SetCurrentLimits (C03) and ClearCurrentLimits (C04) per endpoint.
type CMD002 struct {
	*pics.BaseRule
}

func NewCMD002() *CMD002 {
	return &CMD002{
		BaseRule: pics.NewBaseRule("CMD-002", "acceptsCurrentLimits requires current limit commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD002) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		attrCode := epCode(side, ep.ID, "CTRL.A0B")
		if !p.Has(attrCode) {
			continue
		}

		c03Code := epCode(side, ep.ID, "CTRL.C03.Rsp")
		c04Code := epCode(side, ep.ID, "CTRL.C04.Rsp")

		var missing []string
		if !p.Has(c03Code) {
			missing = append(missing, "SetCurrentLimits (C03.Rsp)")
		}
		if !p.Has(c04Code) {
			missing = append(missing, "ClearCurrentLimits (C04.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): acceptsCurrentLimits (A0B) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{attrCode, c03Code, c04Code},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD003 checks that acceptsSetpoints (A0C) requires SetSetpoint (C05) and ClearSetpoint (C06) per endpoint.
type CMD003 struct {
	*pics.BaseRule
}

func NewCMD003() *CMD003 {
	return &CMD003{
		BaseRule: pics.NewBaseRule("CMD-003", "acceptsSetpoints requires setpoint commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD003) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		attrCode := epCode(side, ep.ID, "CTRL.A0C")
		if !p.Has(attrCode) {
			continue
		}

		c05Code := epCode(side, ep.ID, "CTRL.C05.Rsp")
		c06Code := epCode(side, ep.ID, "CTRL.C06.Rsp")

		var missing []string
		if !p.Has(c05Code) {
			missing = append(missing, "SetSetpoint (C05.Rsp)")
		}
		if !p.Has(c06Code) {
			missing = append(missing, "ClearSetpoint (C06.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): acceptsSetpoints (A0C) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{attrCode, c05Code, c06Code},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD004 checks that isPausable (A0E) requires Pause (C09) and Resume (C0A) per endpoint.
type CMD004 struct {
	*pics.BaseRule
}

func NewCMD004() *CMD004 {
	return &CMD004{
		BaseRule: pics.NewBaseRule("CMD-004", "isPausable requires pause/resume commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD004) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		attrCode := epCode(side, ep.ID, "CTRL.A0E")
		if !p.Has(attrCode) {
			continue
		}

		c09Code := epCode(side, ep.ID, "CTRL.C09.Rsp")
		c0aCode := epCode(side, ep.ID, "CTRL.C0A.Rsp")

		var missing []string
		if !p.Has(c09Code) {
			missing = append(missing, "Pause (C09.Rsp)")
		}
		if !p.Has(c0aCode) {
			missing = append(missing, "Resume (C0A.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): isPausable (A0E) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{attrCode, c09Code, c0aCode},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD005 checks that isStoppable (A10) requires Stop (C0B) per endpoint.
type CMD005 struct {
	*pics.BaseRule
}

func NewCMD005() *CMD005 {
	return &CMD005{
		BaseRule: pics.NewBaseRule("CMD-005", "isStoppable requires stop command", "consistency", pics.SeverityError),
	}
}

func (r *CMD005) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		attrCode := epCode(side, ep.ID, "CTRL.A10")
		if !p.Has(attrCode) {
			continue
		}

		c0bCode := epCode(side, ep.ID, "CTRL.C0B.Rsp")
		if !p.Has(c0bCode) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): isStoppable (A10) requires Stop command (C0B.Rsp)", ep.ID, ep.Type),
				PICSCodes:  []string{attrCode, c0bCode},
				Suggestion: fmt.Sprintf("Add %s=1", c0bCode),
			})
		}
	}

	return violations
}

// CMD006 checks that V2X (F0A) requires SetCurrentSetpoints (C07) and ClearCurrentSetpoints (C08) per endpoint.
type CMD006 struct {
	*pics.BaseRule
}

func NewCMD006() *CMD006 {
	return &CMD006{
		BaseRule: pics.NewBaseRule("CMD-006", "V2X requires current setpoint commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD006) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F0A")
		if !p.Has(flagCode) {
			continue
		}

		c07Code := epCode(side, ep.ID, "CTRL.C07.Rsp")
		c08Code := epCode(side, ep.ID, "CTRL.C08.Rsp")

		var missing []string
		if !p.Has(c07Code) {
			missing = append(missing, "SetCurrentSetpoints (C07.Rsp)")
		}
		if !p.Has(c08Code) {
			missing = append(missing, "ClearCurrentSetpoints (C08.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): V2X (F0A) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{flagCode, c07Code, c08Code},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD007 checks that PROCESS (F07) requires ScheduleProcess (C0C) and CancelProcess (C0D) per endpoint.
type CMD007 struct {
	*pics.BaseRule
}

func NewCMD007() *CMD007 {
	return &CMD007{
		BaseRule: pics.NewBaseRule("CMD-007", "PROCESS requires process commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD007) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CTRL") {
		flagCode := epCode(side, ep.ID, "CTRL.F07")
		if !p.Has(flagCode) {
			continue
		}

		c0cCode := epCode(side, ep.ID, "CTRL.C0C.Rsp")
		c0dCode := epCode(side, ep.ID, "CTRL.C0D.Rsp")

		var missing []string
		if !p.Has(c0cCode) {
			missing = append(missing, "ScheduleProcess (C0C.Rsp)")
		}
		if !p.Has(c0dCode) {
			missing = append(missing, "CancelProcess (C0D.Rsp)")
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): PROCESS (F07) requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{flagCode, c0cCode, c0dCode},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD008 checks that SIG feature requires SendSignal (C01) and ClearSignals (C02) per endpoint.
type CMD008 struct {
	*pics.BaseRule
}

func NewCMD008() *CMD008 {
	return &CMD008{
		BaseRule: pics.NewBaseRule("CMD-008", "SIG requires signal commands", "consistency", pics.SeverityError),
	}
}

func (r *CMD008) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("SIG") {
		c01Code := epCode(side, ep.ID, "SIG.C01.Rsp")
		c02Code := epCode(side, ep.ID, "SIG.C02.Rsp")

		var missing []string
		if !p.Has(c01Code) {
			missing = append(missing, "SendSignal (C01.Rsp)")
		}
		if !p.Has(c02Code) {
			missing = append(missing, "ClearSignals (C02.Rsp)")
		}

		if len(missing) > 0 {
			featureCode := epCode(side, ep.ID, "SIG")
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): SIG feature requires: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
				PICSCodes:  []string{featureCode, c01Code, c02Code},
				Suggestion: "Add the required command declarations",
			})
		}
	}

	return violations
}

// CMD009 checks that CHRG feature requires SetChargingMode (C01) per endpoint.
type CMD009 struct {
	*pics.BaseRule
}

func NewCMD009() *CMD009 {
	return &CMD009{
		BaseRule: pics.NewBaseRule("CMD-009", "CHRG requires SetChargingMode command", "consistency", pics.SeverityError),
	}
}

func (r *CMD009) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("CHRG") {
		c01Code := epCode(side, ep.ID, "CHRG.C01.Rsp")

		if !p.Has(c01Code) {
			featureCode := epCode(side, ep.ID, "CHRG")
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): CHRG feature requires SetChargingMode command (C01.Rsp)", ep.ID, ep.Type),
				PICSCodes:  []string{featureCode, c01Code},
				Suggestion: fmt.Sprintf("Add %s=1", c01Code),
			})
		}
	}

	return violations
}

// CMD010 checks that PLAN feature requires RequestPlan (C01) per endpoint.
type CMD010 struct {
	*pics.BaseRule
}

func NewCMD010() *CMD010 {
	return &CMD010{
		BaseRule: pics.NewBaseRule("CMD-010", "PLAN requires RequestPlan command", "consistency", pics.SeverityError),
	}
}

func (r *CMD010) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("PLAN") {
		c01Code := epCode(side, ep.ID, "PLAN.C01.Rsp")

		if !p.Has(c01Code) {
			featureCode := epCode(side, ep.ID, "PLAN")
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): PLAN feature requires RequestPlan command (C01.Rsp)", ep.ID, ep.Type),
				PICSCodes:  []string{featureCode, c01Code},
				Suggestion: fmt.Sprintf("Add %s=1", c01Code),
			})
		}
	}

	return violations
}

// CMD011 checks that TAR feature requires SetTariff (C01) per endpoint.
type CMD011 struct {
	*pics.BaseRule
}

func NewCMD011() *CMD011 {
	return &CMD011{
		BaseRule: pics.NewBaseRule("CMD-011", "TAR requires SetTariff command", "consistency", pics.SeverityError),
	}
}

func (r *CMD011) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range p.EndpointsWithFeature("TAR") {
		c01Code := epCode(side, ep.ID, "TAR.C01.Rsp")

		if !p.Has(c01Code) {
			featureCode := epCode(side, ep.ID, "TAR")
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): TAR feature requires SetTariff command (C01.Rsp)", ep.ID, ep.Type),
				PICSCodes:  []string{featureCode, c01Code},
				Suggestion: fmt.Sprintf("Add %s=1", c01Code),
			})
		}
	}

	return violations
}

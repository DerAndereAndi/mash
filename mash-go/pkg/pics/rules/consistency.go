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
}

// CMD001 checks that acceptsLimits (A0A) requires SetLimit (C01) and ClearLimit (C02).
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
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A0A", side)

	if !p.Has(attrCode) {
		return nil
	}

	c01Code := fmt.Sprintf("MASH.%s.CTRL.C01.Rsp", side)
	c02Code := fmt.Sprintf("MASH.%s.CTRL.C02.Rsp", side)

	var missing []string
	if !p.Has(c01Code) {
		missing = append(missing, "SetLimit (C01.Rsp)")
	}
	if !p.Has(c02Code) {
		missing = append(missing, "ClearLimit (C02.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("acceptsLimits (A0A) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{attrCode, c01Code, c02Code},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD002 checks that acceptsCurrentLimits (A0B) requires SetCurrentLimits (C05) and ClearCurrentLimits (C06).
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
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A0B", side)

	if !p.Has(attrCode) {
		return nil
	}

	c05Code := fmt.Sprintf("MASH.%s.CTRL.C05.Rsp", side)
	c06Code := fmt.Sprintf("MASH.%s.CTRL.C06.Rsp", side)

	var missing []string
	if !p.Has(c05Code) {
		missing = append(missing, "SetCurrentLimits (C05.Rsp)")
	}
	if !p.Has(c06Code) {
		missing = append(missing, "ClearCurrentLimits (C06.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("acceptsCurrentLimits (A0B) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{attrCode, c05Code, c06Code},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD003 checks that acceptsSetpoints (A0C) requires SetSetpoint (C03) and ClearSetpoint (C04).
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
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A0C", side)

	if !p.Has(attrCode) {
		return nil
	}

	c03Code := fmt.Sprintf("MASH.%s.CTRL.C03.Rsp", side)
	c04Code := fmt.Sprintf("MASH.%s.CTRL.C04.Rsp", side)

	var missing []string
	if !p.Has(c03Code) {
		missing = append(missing, "SetSetpoint (C03.Rsp)")
	}
	if !p.Has(c04Code) {
		missing = append(missing, "ClearSetpoint (C04.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("acceptsSetpoints (A0C) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{attrCode, c03Code, c04Code},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD004 checks that isPausable (A0E) requires Pause (C09) and Resume (C0A).
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
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A0E", side)

	if !p.Has(attrCode) {
		return nil
	}

	c09Code := fmt.Sprintf("MASH.%s.CTRL.C09.Rsp", side)
	c0aCode := fmt.Sprintf("MASH.%s.CTRL.C0A.Rsp", side)

	var missing []string
	if !p.Has(c09Code) {
		missing = append(missing, "Pause (C09.Rsp)")
	}
	if !p.Has(c0aCode) {
		missing = append(missing, "Resume (C0A.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("isPausable (A0E) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{attrCode, c09Code, c0aCode},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD005 checks that isStoppable (A10) requires Stop (C0B).
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
	attrCode := fmt.Sprintf("MASH.%s.CTRL.A10", side)

	if !p.Has(attrCode) {
		return nil
	}

	c0bCode := fmt.Sprintf("MASH.%s.CTRL.C0B.Rsp", side)

	if !p.Has(c0bCode) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "isStoppable (A10) requires Stop command (C0B.Rsp)",
			PICSCodes:  []string{attrCode, c0bCode},
			Suggestion: fmt.Sprintf("Add %s=1", c0bCode),
		}}
	}
	return nil
}

// CMD006 checks that V2X (F0A) requires SetCurrentSetpoints (C07) and ClearCurrentSetpoints (C08).
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
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F0A", side)

	if !p.Has(flagCode) {
		return nil
	}

	c07Code := fmt.Sprintf("MASH.%s.CTRL.C07.Rsp", side)
	c08Code := fmt.Sprintf("MASH.%s.CTRL.C08.Rsp", side)

	var missing []string
	if !p.Has(c07Code) {
		missing = append(missing, "SetCurrentSetpoints (C07.Rsp)")
	}
	if !p.Has(c08Code) {
		missing = append(missing, "ClearCurrentSetpoints (C08.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("V2X (F0A) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{flagCode, c07Code, c08Code},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD007 checks that PROCESS (F07) requires ScheduleProcess (C0C) and CancelProcess (C0D).
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
	flagCode := fmt.Sprintf("MASH.%s.CTRL.F07", side)

	if !p.Has(flagCode) {
		return nil
	}

	c0cCode := fmt.Sprintf("MASH.%s.CTRL.C0C.Rsp", side)
	c0dCode := fmt.Sprintf("MASH.%s.CTRL.C0D.Rsp", side)

	var missing []string
	if !p.Has(c0cCode) {
		missing = append(missing, "ScheduleProcess (C0C.Rsp)")
	}
	if !p.Has(c0dCode) {
		missing = append(missing, "CancelProcess (C0D.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("PROCESS (F07) requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{flagCode, c0cCode, c0dCode},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD008 checks that SIG feature requires SendSignal (C01) and ClearSignals (C02).
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
	featureCode := fmt.Sprintf("MASH.%s.SIG", side)

	if !p.Has(featureCode) {
		return nil
	}

	c01Code := fmt.Sprintf("MASH.%s.SIG.C01.Rsp", side)
	c02Code := fmt.Sprintf("MASH.%s.SIG.C02.Rsp", side)

	var missing []string
	if !p.Has(c01Code) {
		missing = append(missing, "SendSignal (C01.Rsp)")
	}
	if !p.Has(c02Code) {
		missing = append(missing, "ClearSignals (C02.Rsp)")
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("SIG feature requires: %s", strings.Join(missing, ", ")),
			PICSCodes:  []string{featureCode, c01Code, c02Code},
			Suggestion: "Add the required command declarations",
		}}
	}
	return nil
}

// CMD009 checks that CHRG feature requires SetChargingMode (C01).
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
	featureCode := fmt.Sprintf("MASH.%s.CHRG", side)

	if !p.Has(featureCode) {
		return nil
	}

	c01Code := fmt.Sprintf("MASH.%s.CHRG.C01.Rsp", side)

	if !p.Has(c01Code) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "CHRG feature requires SetChargingMode command (C01.Rsp)",
			PICSCodes:  []string{featureCode, c01Code},
			Suggestion: fmt.Sprintf("Add %s=1", c01Code),
		}}
	}
	return nil
}

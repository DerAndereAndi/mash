package rules

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

// RegisterMandatoryRules registers all mandatory attribute rules with the given registry.
func RegisterMandatoryRules(registry *pics.RuleRegistry) {
	registry.Register(NewMAN001())
	registry.Register(NewMAN002())
	registry.Register(NewMAN003())
	registry.Register(NewMAN004())
	registry.Register(NewMAN005())
	registry.Register(NewMAN006())
	registry.Register(NewMAN007())
}

// MAN001 checks that MASH.S or MASH.C is present.
type MAN001 struct {
	*pics.BaseRule
}

func NewMAN001() *MAN001 {
	return &MAN001{
		BaseRule: pics.NewBaseRule("MAN-001", "Protocol declaration required", "mandatory", pics.SeverityError),
	}
}

func (r *MAN001) Check(p *pics.PICS) []pics.Violation {
	hasServer := p.Has("MASH.S")
	hasClient := p.Has("MASH.C")

	if !hasServer && !hasClient {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "Missing protocol declaration (MASH.S or MASH.C required)",
			Suggestion: "Add MASH.S=1 for device or MASH.C=1 for controller",
		}}
	}
	return nil
}

// MAN002 checks mandatory attributes for CTRL feature.
type MAN002 struct {
	*pics.BaseRule
}

func NewMAN002() *MAN002 {
	return &MAN002{
		BaseRule: pics.NewBaseRule("MAN-002", "CTRL mandatory attributes", "mandatory", pics.SeverityError),
	}
}

func (r *MAN002) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.CTRL", side)

	if !p.Has(featureCode) {
		return nil // Not applicable
	}

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

	var missing []string
	var codes []string
	for _, attr := range mandatory {
		code := fmt.Sprintf("MASH.%s.CTRL.A%s", side, attr.id)
		if !p.Has(code) {
			missing = append(missing, fmt.Sprintf("%s (A%s)", attr.name, attr.id))
			codes = append(codes, code)
		}
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("EnergyControl missing mandatory attributes: %s", strings.Join(missing, ", ")),
			PICSCodes:  codes,
			Suggestion: "Add the missing mandatory attributes",
		}}
	}
	return nil
}

// MAN003 checks mandatory attributes for ELEC feature.
type MAN003 struct {
	*pics.BaseRule
}

func NewMAN003() *MAN003 {
	return &MAN003{
		BaseRule: pics.NewBaseRule("MAN-003", "ELEC mandatory attributes", "mandatory", pics.SeverityError),
	}
}

func (r *MAN003) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.ELEC", side)

	if !p.Has(featureCode) {
		return nil
	}

	mandatory := []struct {
		id   string
		name string
	}{
		{"01", "phaseCount"},
		{"02", "phaseMapping"},
		{"03", "nominalVoltage"},
		{"04", "nominalFrequency"},
		{"05", "supportedDirections"},
		{"0D", "maxCurrentPerPhase"},
	}

	var missing []string
	var codes []string
	for _, attr := range mandatory {
		code := fmt.Sprintf("MASH.%s.ELEC.A%s", side, attr.id)
		if !p.Has(code) {
			missing = append(missing, fmt.Sprintf("%s (A%s)", attr.name, attr.id))
			codes = append(codes, code)
		}
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("Electrical missing mandatory attributes: %s", strings.Join(missing, ", ")),
			PICSCodes:  codes,
			Suggestion: "Add the missing mandatory attributes",
		}}
	}
	return nil
}

// MAN004 checks mandatory attributes for CHRG feature.
type MAN004 struct {
	*pics.BaseRule
}

func NewMAN004() *MAN004 {
	return &MAN004{
		BaseRule: pics.NewBaseRule("MAN-004", "CHRG mandatory attributes", "mandatory", pics.SeverityError),
	}
}

func (r *MAN004) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.CHRG", side)

	if !p.Has(featureCode) {
		return nil
	}

	mandatory := []struct {
		id   string
		name string
	}{
		{"01", "state"},
		{"02", "sessionId"},
		{"03", "sessionStartTime"},
		{"0A", "sessionEnergyCharged"},
		{"28", "evDemandMode"},
		{"46", "chargingMode"},
		{"47", "supportedChargingModes"},
	}

	var missing []string
	var codes []string
	for _, attr := range mandatory {
		code := fmt.Sprintf("MASH.%s.CHRG.A%s", side, attr.id)
		if !p.Has(code) {
			missing = append(missing, fmt.Sprintf("%s (A%s)", attr.name, attr.id))
			codes = append(codes, code)
		}
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("ChargingSession missing mandatory attributes: %s", strings.Join(missing, ", ")),
			PICSCodes:  codes,
			Suggestion: "Add the missing mandatory attributes",
		}}
	}
	return nil
}

// MAN005 checks mandatory attributes for SIG feature.
type MAN005 struct {
	*pics.BaseRule
}

func NewMAN005() *MAN005 {
	return &MAN005{
		BaseRule: pics.NewBaseRule("MAN-005", "SIG mandatory attributes", "mandatory", pics.SeverityError),
	}
}

func (r *MAN005) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.SIG", side)

	if !p.Has(featureCode) {
		return nil
	}

	mandatory := []struct {
		id   string
		name string
	}{
		{"01", "activeSignals"},
		{"02", "signalCount"},
		{"0A", "lastReceivedSignalId"},
		{"0B", "signalStatus"},
	}

	var missing []string
	var codes []string
	for _, attr := range mandatory {
		code := fmt.Sprintf("MASH.%s.SIG.A%s", side, attr.id)
		if !p.Has(code) {
			missing = append(missing, fmt.Sprintf("%s (A%s)", attr.name, attr.id))
			codes = append(codes, code)
		}
	}

	if len(missing) > 0 {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    fmt.Sprintf("Signals missing mandatory attributes: %s", strings.Join(missing, ", ")),
			PICSCodes:  codes,
			Suggestion: "Add the missing mandatory attributes",
		}}
	}
	return nil
}

// MAN006 checks mandatory attributes for STAT feature.
type MAN006 struct {
	*pics.BaseRule
}

func NewMAN006() *MAN006 {
	return &MAN006{
		BaseRule: pics.NewBaseRule("MAN-006", "STAT mandatory attributes", "mandatory", pics.SeverityError),
	}
}

func (r *MAN006) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.STAT", side)

	if !p.Has(featureCode) {
		return nil
	}

	a01Code := fmt.Sprintf("MASH.%s.STAT.A01", side)
	if !p.Has(a01Code) {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "Status feature requires operatingState (A01)",
			PICSCodes:  []string{a01Code},
			Suggestion: fmt.Sprintf("Add %s=1", a01Code),
		}}
	}
	return nil
}

// MAN007 checks mandatory attributes for MEAS feature.
type MAN007 struct {
	*pics.BaseRule
}

func NewMAN007() *MAN007 {
	return &MAN007{
		BaseRule: pics.NewBaseRule("MAN-007", "MEAS mandatory attributes", "mandatory", pics.SeverityWarning),
	}
}

func (r *MAN007) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	featureCode := fmt.Sprintf("MASH.%s.MEAS", side)

	if !p.Has(featureCode) {
		return nil
	}

	// Check for AC power (A01) or DC power (A28)
	a01Code := fmt.Sprintf("MASH.%s.MEAS.A01", side)
	a28Code := fmt.Sprintf("MASH.%s.MEAS.A28", side)

	hasAC := p.Has(a01Code)
	hasDC := p.Has(a28Code)

	if !hasAC && !hasDC {
		return []pics.Violation{{
			RuleID:     r.ID(),
			Severity:   r.DefaultSeverity(),
			Message:    "Measurement feature requires either acActivePower (A01) for AC or dcPower (A28) for DC",
			PICSCodes:  []string{a01Code, a28Code},
			Suggestion: "Add A01=1 for AC measurement or A28=1 for DC measurement",
		}}
	}
	return nil
}

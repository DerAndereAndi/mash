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

// checkMandatoryPerEndpoint is a helper that checks mandatory attributes for a feature
// across all endpoints that have that feature.
func checkMandatoryPerEndpoint(p *pics.PICS, ruleID string, severity pics.Severity, feature string, featureLabel string, mandatory []struct{ id, name string }) []pics.Violation {
	eps := p.EndpointsWithFeature(feature)
	if len(eps) == 0 {
		return nil
	}

	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range eps {
		var missing []string
		var codes []string
		for _, attr := range mandatory {
			code := fmt.Sprintf("MASH.%s.E%02X.%s.A%s", side, ep.ID, feature, attr.id)
			if !p.Has(code) {
				missing = append(missing, fmt.Sprintf("%s (A%s)", attr.name, attr.id))
				codes = append(codes, code)
			}
		}

		if len(missing) > 0 {
			violations = append(violations, pics.Violation{
				RuleID:     ruleID,
				Severity:   severity,
				Message:    fmt.Sprintf("Endpoint %d (%s): %s missing mandatory attributes: %s", ep.ID, ep.Type, featureLabel, strings.Join(missing, ", ")),
				PICSCodes:  codes,
				Suggestion: "Add the missing mandatory attributes",
			})
		}
	}

	return violations
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
	mandatory := []struct{ id, name string }{
		{"01", "deviceType"},
		{"02", "controlState"},
		{"0A", "acceptsLimits"},
		{"0B", "acceptsCurrentLimits"},
		{"0C", "acceptsSetpoints"},
		{"0E", "isPausable"},
		{"46", "failsafeConsumptionLimit"},
		{"48", "failsafeDuration"},
	}
	return checkMandatoryPerEndpoint(p, r.ID(), r.DefaultSeverity(), "CTRL", "EnergyControl", mandatory)
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
	mandatory := []struct{ id, name string }{
		{"01", "phaseCount"},
		{"02", "phaseMapping"},
		{"03", "nominalVoltage"},
		{"04", "nominalFrequency"},
		{"05", "supportedDirections"},
		{"0D", "maxCurrentPerPhase"},
	}
	return checkMandatoryPerEndpoint(p, r.ID(), r.DefaultSeverity(), "ELEC", "Electrical", mandatory)
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
	mandatory := []struct{ id, name string }{
		{"01", "state"},
		{"02", "sessionId"},
		{"03", "sessionStartTime"},
		{"0A", "sessionEnergyCharged"},
		{"28", "evDemandMode"},
		{"46", "chargingMode"},
		{"47", "supportedChargingModes"},
	}
	return checkMandatoryPerEndpoint(p, r.ID(), r.DefaultSeverity(), "CHRG", "ChargingSession", mandatory)
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
	mandatory := []struct{ id, name string }{
		{"01", "activeSignals"},
		{"02", "signalCount"},
		{"0A", "lastReceivedSignalId"},
		{"0B", "signalStatus"},
	}
	return checkMandatoryPerEndpoint(p, r.ID(), r.DefaultSeverity(), "SIG", "Signals", mandatory)
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
	eps := p.EndpointsWithFeature("STAT")
	if len(eps) == 0 {
		return nil
	}

	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range eps {
		a01Code := fmt.Sprintf("MASH.%s.E%02X.STAT.A01", side, ep.ID)
		if !p.Has(a01Code) {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   r.DefaultSeverity(),
				Message:    fmt.Sprintf("Endpoint %d (%s): Status feature requires operatingState (A01)", ep.ID, ep.Type),
				PICSCodes:  []string{a01Code},
				Suggestion: fmt.Sprintf("Add %s=1", a01Code),
			})
		}
	}

	return violations
}

// MAN007 checks mandatory attributes for MEAS feature.
// Uses endpoint type from EndpointPICS to determine mandatory attributes:
//   - BATTERY/PV_STRING → dcPower (A28) required; BATTERY also requires stateOfCharge (A32)
//   - AC endpoint types → acActivePower (A01) required
//   - Unknown types → at least AC or DC power must be present
type MAN007 struct {
	*pics.BaseRule
}

func NewMAN007() *MAN007 {
	return &MAN007{
		BaseRule: pics.NewBaseRule("MAN-007", "MEAS mandatory attributes", "mandatory", pics.SeverityWarning),
	}
}

// dcEndpointTypes are endpoint types that require DC measurement.
var dcEndpointTypes = map[string]bool{
	"BATTERY":   true,
	"PV_STRING": true,
}

func (r *MAN007) Check(p *pics.PICS) []pics.Violation {
	eps := p.EndpointsWithFeature("MEAS")
	if len(eps) == 0 {
		return nil
	}

	side := string(p.Side)
	var violations []pics.Violation

	for _, ep := range eps {
		a01Code := fmt.Sprintf("MASH.%s.E%02X.MEAS.A01", side, ep.ID) // acActivePower
		a28Code := fmt.Sprintf("MASH.%s.E%02X.MEAS.A28", side, ep.ID) // dcPower
		a32Code := fmt.Sprintf("MASH.%s.E%02X.MEAS.A32", side, ep.ID) // stateOfCharge

		hasAC := p.Has(a01Code)
		hasDC := p.Has(a28Code)

		if dcEndpointTypes[ep.Type] {
			// DC endpoint: dcPower required
			var missing []string
			var codes []string
			if !hasDC {
				missing = append(missing, "dcPower (A28)")
				codes = append(codes, a28Code)
			}
			if ep.Type == "BATTERY" && !p.Has(a32Code) {
				missing = append(missing, "stateOfCharge (A32)")
				codes = append(codes, a32Code)
			}
			if len(missing) > 0 {
				violations = append(violations, pics.Violation{
					RuleID:     r.ID(),
					Severity:   pics.SeverityError,
					Message:    fmt.Sprintf("Endpoint %d (%s): Measurement missing: %s", ep.ID, ep.Type, strings.Join(missing, ", ")),
					PICSCodes:  codes,
					Suggestion: fmt.Sprintf("%s endpoints require DC measurement attributes (see endpoint-conformance.yaml)", ep.Type),
				})
			}
		} else if ep.Type != "" && !dcEndpointTypes[ep.Type] {
			// Known AC endpoint type: acActivePower required
			if !hasAC {
				violations = append(violations, pics.Violation{
					RuleID:     r.ID(),
					Severity:   r.DefaultSeverity(),
					Message:    fmt.Sprintf("Endpoint %d (%s): Measurement requires acActivePower (A01)", ep.ID, ep.Type),
					PICSCodes:  []string{a01Code},
					Suggestion: fmt.Sprintf("Add %s=1 for AC power measurement", a01Code),
				})
			}
		} else {
			// Unknown endpoint type: at least AC or DC power must be present
			if !hasAC && !hasDC {
				violations = append(violations, pics.Violation{
					RuleID:     r.ID(),
					Severity:   r.DefaultSeverity(),
					Message:    fmt.Sprintf("Endpoint %d (%s): Measurement requires either acActivePower (A01) or dcPower (A28)", ep.ID, ep.Type),
					PICSCodes:  []string{a01Code, a28Code},
					Suggestion: "Add A01=1 for AC measurement or A28=1 for DC measurement",
				})
			}
		}
	}

	return violations
}

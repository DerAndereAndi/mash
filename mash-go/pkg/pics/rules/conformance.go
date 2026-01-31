package rules

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
)

// RegisterConformanceRules registers all endpoint conformance rules with the given registry.
func RegisterConformanceRules(registry *pics.RuleRegistry) {
	registry.Register(NewEPT001())
}

// attrRequirement defines mandatory and recommended attributes for a feature on an endpoint type.
type attrRequirement struct {
	mandatory   []string // hex attribute IDs (e.g., "01", "28")
	recommended []string
}

// endpointConformance maps feature name -> attrRequirement for a given endpoint type.
type endpointConformance map[string]attrRequirement

// conformanceRegistry maps endpoint type to its conformance requirements.
// Attribute hex IDs are derived from the feature YAML definitions:
//
//	Measurement: acActivePower=01, acReactivePower=02, acCurrentPerPhase=14, acVoltagePerPhase=15,
//	  acFrequency=17, powerFactor=18, acEnergyConsumed=1E, acEnergyProduced=1F,
//	  dcPower=28, dcCurrent=29, dcVoltage=2A, dcEnergyIn=2B, dcEnergyOut=2C,
//	  stateOfCharge=32, stateOfHealth=33, stateOfEnergy=34, useableCapacity=35, cycleCount=36,
//	  temperature=3C
//	Electrical: phaseCount=01, phaseMapping=02, nominalVoltage=03, nominalFrequency=04,
//	  supportedDirections=05, nominalMaxConsumption=0A, nominalMaxProduction=0B,
//	  maxCurrentPerPhase=0D, minCurrentPerPhase=0E, energyCapacity=14
var conformanceRegistry = map[string]endpointConformance{
	"GRID_CONNECTION": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"02", "14", "15", "17", "1E", "1F", "18"},
		},
		"ELEC": {
			mandatory:   []string{"01", "05", "0A"},
			recommended: []string{"04", "03"},
		},
	},
	"INVERTER": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"02", "14", "15", "17", "1E", "1F", "18", "3C"},
		},
		"ELEC": {
			mandatory: []string{"01", "05", "0A", "0B"},
		},
	},
	"PV_STRING": {
		"MEAS": {
			mandatory:   []string{"28"},
			recommended: []string{"2A", "29", "2C", "3C"},
		},
		"ELEC": {
			recommended: []string{"0B"},
		},
	},
	"BATTERY": {
		"MEAS": {
			mandatory:   []string{"28", "32"},
			recommended: []string{"2A", "29", "2B", "2C", "33", "34", "35", "36", "3C"},
		},
		"ELEC": {
			mandatory: []string{"05", "0A", "0B", "14"},
		},
	},
	"EV_CHARGER": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"14", "15", "1E"},
		},
		"ELEC": {
			mandatory:   []string{"01", "05", "0A", "0D"},
			recommended: []string{"0E"},
		},
	},
	"HEAT_PUMP": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"1E", "3C"},
		},
		"ELEC": {
			mandatory: []string{"01", "0A"},
		},
	},
	"WATER_HEATER": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"1E", "3C"},
		},
		"ELEC": {
			mandatory: []string{"01", "0A"},
		},
	},
	"HVAC": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"1E", "3C"},
		},
		"ELEC": {
			mandatory: []string{"01", "0A"},
		},
	},
	"APPLIANCE": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"1E"},
		},
		"ELEC": {
			mandatory: []string{"0A"},
		},
	},
	"SUB_METER": {
		"MEAS": {
			mandatory:   []string{"01"},
			recommended: []string{"14", "15", "17", "1E", "1F"},
		},
		"ELEC": {
			mandatory:   []string{"01"},
			recommended: []string{"0A"},
		},
	},
}

// EPT001 checks that each endpoint has the mandatory and recommended attributes
// for its type, as defined in the endpoint conformance registry.
type EPT001 struct {
	*pics.BaseRule
}

func NewEPT001() *EPT001 {
	return &EPT001{
		BaseRule: pics.NewBaseRule("EPT-001", "Endpoint type conformance", "conformance", pics.SeverityError),
	}
}

func (r *EPT001) Check(p *pics.PICS) []pics.Violation {
	side := string(p.Side)
	var violations []pics.Violation

	for _, epID := range p.EndpointIDs() {
		ep := p.Endpoints[epID]
		conf, ok := conformanceRegistry[ep.Type]
		if !ok {
			continue // Unknown endpoint type, skip
		}

		for _, feature := range ep.Features {
			req, ok := conf[feature]
			if !ok {
				continue // No conformance requirements for this feature on this type
			}

			// Check mandatory attributes
			var missing []string
			for _, attrID := range req.mandatory {
				code := fmt.Sprintf("MASH.%s.E%02X.%s.A%s", side, epID, feature, strings.ToUpper(attrID))
				if !p.Has(code) {
					missing = append(missing, fmt.Sprintf("A%s", strings.ToUpper(attrID)))
				}
			}
			if len(missing) > 0 {
				violations = append(violations, pics.Violation{
					RuleID:     r.ID(),
					Severity:   pics.SeverityError,
					Message:    fmt.Sprintf("Endpoint %d (%s): %s feature missing mandatory attributes: %s", epID, ep.Type, feature, strings.Join(missing, ", ")),
					PICSCodes:  missing,
					Suggestion: "Add the required attribute declarations",
				})
			}

			// Check recommended attributes
			var missingRec []string
			for _, attrID := range req.recommended {
				code := fmt.Sprintf("MASH.%s.E%02X.%s.A%s", side, epID, feature, strings.ToUpper(attrID))
				if !p.Has(code) {
					missingRec = append(missingRec, fmt.Sprintf("A%s", strings.ToUpper(attrID)))
				}
			}
			if len(missingRec) > 0 {
				violations = append(violations, pics.Violation{
					RuleID:     r.ID(),
					Severity:   pics.SeverityWarning,
					Message:    fmt.Sprintf("Endpoint %d (%s): %s feature missing recommended attributes: %s", epID, ep.Type, feature, strings.Join(missingRec, ", ")),
					PICSCodes:  missingRec,
					Suggestion: "Consider adding recommended attributes if hardware supports them",
				})
			}
		}
	}

	return violations
}

package rules

import (
	"fmt"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/pics"
	"github.com/mash-protocol/mash-go/pkg/usecase"
)

// featureNameToID maps use case feature names to PICS feature identifiers.
var featureNameToID = map[string]string{
	"EnergyControl":   "CTRL",
	"Electrical":      "ELEC",
	"Measurement":     "MEAS",
	"Status":          "STAT",
	"ChargingSession": "CHRG",
	"Signals":         "SIG",
	"Tariff":          "TAR",
	"Plan":            "PLAN",
	"DeviceInfo":      "INFO",
}

// RegisterUseCaseRules registers all use case validation rules with the given registry.
func RegisterUseCaseRules(registry *pics.RuleRegistry) {
	registry.Register(NewUC001(usecase.Registry))
}

// UC001 validates that declared use cases have required features present on at
// least one endpoint. For controllers (client-side), no feature validation is
// performed since controllers are the client side of the interaction.
type UC001 struct {
	*pics.BaseRule
	registry map[usecase.UseCaseName]*usecase.UseCaseDef
}

// NewUC001 creates a UC-001 rule with the given use case registry.
func NewUC001(registry map[usecase.UseCaseName]*usecase.UseCaseDef) *UC001 {
	return &UC001{
		BaseRule: pics.NewBaseRule("UC-001", "Use case feature requirements", "usecase", pics.SeverityError),
		registry: registry,
	}
}

func (r *UC001) Check(p *pics.PICS) []pics.Violation {
	var violations []pics.Violation

	// Only validate device (server) PICS -- controllers don't host features
	if p.Side == pics.SideClient {
		return nil
	}

	for _, ucName := range p.UseCases() {
		def, ok := r.registry[usecase.UseCaseName(ucName)]
		if !ok {
			violations = append(violations, pics.Violation{
				RuleID:     r.ID(),
				Severity:   pics.SeverityWarning,
				Message:    fmt.Sprintf("Use case %s declared but not found in registry", ucName),
				PICSCodes:  []string{fmt.Sprintf("MASH.S.UC.%s", ucName)},
				Suggestion: "Verify the use case name is correct",
			})
			continue
		}

		// Check each required feature is present on at least one endpoint
		for _, freq := range def.Features {
			if !freq.Required {
				continue
			}

			picsFeature, ok := featureNameToID[freq.FeatureName]
			if !ok {
				continue // Unknown feature mapping, skip
			}

			// Find at least one endpoint that has this feature
			eps := p.EndpointsWithFeature(picsFeature)
			if len(eps) == 0 {
				violations = append(violations, pics.Violation{
					RuleID:   r.ID(),
					Severity: pics.SeverityError,
					Message: fmt.Sprintf("Use case %s requires %s (%s) but no endpoint declares it",
						ucName, freq.FeatureName, picsFeature),
					PICSCodes:  []string{fmt.Sprintf("MASH.S.UC.%s", ucName)},
					Suggestion: fmt.Sprintf("Add %s feature to an appropriate endpoint", picsFeature),
				})
				continue
			}

			// Check required attributes (presence-only or value-checked)
			for _, attr := range freq.Attributes {
				attrID := fmt.Sprintf("%02X", attr.AttrID)
				found := false
				for _, ep := range eps {
					code := fmt.Sprintf("MASH.S.E%02X.%s.A%s", ep.ID, picsFeature, strings.ToUpper(attrID))
					if p.Has(code) {
						found = true
						break
					}
				}
				if !found {
					violations = append(violations, pics.Violation{
						RuleID:   r.ID(),
						Severity: pics.SeverityWarning,
						Message: fmt.Sprintf("Use case %s: %s.%s (A%s) expected to be declared",
							ucName, picsFeature, attr.Name, strings.ToUpper(attrID)),
						PICSCodes:  []string{fmt.Sprintf("MASH.S.UC.%s", ucName)},
						Suggestion: fmt.Sprintf("Add attribute A%s to %s feature on an endpoint", strings.ToUpper(attrID), picsFeature),
					})
				}
			}

			// Check required commands
			for _, cmd := range freq.Commands {
				cmdID := fmt.Sprintf("%02X", cmd.CommandID)
				found := false
				for _, ep := range eps {
					code := fmt.Sprintf("MASH.S.E%02X.%s.C%s.Rsp", ep.ID, picsFeature, strings.ToUpper(cmdID))
					if p.Has(code) {
						found = true
						break
					}
				}
				if !found {
					violations = append(violations, pics.Violation{
						RuleID:   r.ID(),
						Severity: pics.SeverityError,
						Message: fmt.Sprintf("Use case %s: %s command %s (C%s.Rsp) required but not declared",
							ucName, picsFeature, cmd.Name, strings.ToUpper(cmdID)),
						PICSCodes:  []string{fmt.Sprintf("MASH.S.UC.%s", ucName)},
						Suggestion: fmt.Sprintf("Add command C%s.Rsp to %s feature on an endpoint", strings.ToUpper(cmdID), picsFeature),
					})
				}
			}
		}

		// Check endpoint type constraints
		if len(def.EndpointTypes) > 0 {
			allowed := make(map[string]bool, len(def.EndpointTypes))
			for _, t := range def.EndpointTypes {
				allowed[t] = true
			}

			// Collect all endpoint types that have any required feature
			hasMatchingType := false
			var seenTypes []string
			for _, freq := range def.Features {
				if !freq.Required {
					continue
				}
				picsFeature, ok := featureNameToID[freq.FeatureName]
				if !ok {
					continue
				}
				for _, ep := range p.EndpointsWithFeature(picsFeature) {
					if allowed[ep.Type] {
						hasMatchingType = true
					}
					seenTypes = append(seenTypes, ep.Type)
				}
			}

			if !hasMatchingType && len(seenTypes) > 0 {
				violations = append(violations, pics.Violation{
					RuleID:   r.ID(),
					Severity: pics.SeverityWarning,
					Message: fmt.Sprintf("Use case %s: features found but not on allowed endpoint type (allowed: %s)",
						ucName, strings.Join(def.EndpointTypes, ", ")),
					PICSCodes:  []string{fmt.Sprintf("MASH.S.UC.%s", ucName)},
					Suggestion: fmt.Sprintf("Place required features on an endpoint of type: %s", strings.Join(def.EndpointTypes, ", ")),
				})
			}
		}
	}

	return violations
}

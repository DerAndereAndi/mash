package usecase

// MatchAll matches all use case definitions against a device profile.
// Returns a DeviceUseCases with match results for each use case.
func MatchAll(profile *DeviceProfile, defs map[UseCaseName]*UseCaseDef) *DeviceUseCases {
	du := &DeviceUseCases{
		DeviceID: profile.DeviceID,
		Profile:  profile,
		registry: defs,
	}

	for name, def := range defs {
		best := MatchResult{
			UseCase: name,
			Matched: false,
		}

		for _, ep := range profile.Endpoints {
			result := matchEndpoint(def, ep)
			if result.Matched && !best.Matched {
				best = result
			} else if result.Matched && result.Scenarios > best.Scenarios {
				// Prefer endpoint with more scenarios matched
				best = result
			}
		}

		// If no endpoint matched, collect missing info from first compatible endpoint
		if !best.Matched {
			for _, ep := range profile.Endpoints {
				result := matchEndpoint(def, ep)
				if len(result.MissingRequired) > 0 || result.UseCase == name {
					best = result
					break
				}
			}
		}

		du.Matches = append(du.Matches, best)
	}

	return du
}

func matchEndpoint(def *UseCaseDef, ep *EndpointProfile) MatchResult {
	result := MatchResult{
		UseCase:    def.Name,
		EndpointID: ep.EndpointID,
	}

	// Check endpoint type constraint
	if len(def.EndpointTypes) > 0 {
		found := false
		for _, et := range def.EndpointTypes {
			if et == ep.EndpointType {
				found = true
				break
			}
		}
		if !found {
			result.MissingRequired = append(result.MissingRequired,
				"endpoint type "+ep.EndpointType+" not in "+string(def.Name)+" supported types")
			return result
		}
	}

	// Match each scenario independently
	for _, scenario := range def.Scenarios {
		// Per-scenario endpoint type filter
		if len(scenario.EndpointTypes) > 0 && !containsType(scenario.EndpointTypes, ep.EndpointType) {
			continue
		}
		if matchScenarioFeatures(scenario.Features, ep) {
			result.Scenarios |= 1 << ScenarioMap(scenario.Bit)
		}
	}

	// Enforce requires/requiresAny constraints
	result.Scenarios = enforceScenarioConstraints(def, result.Scenarios)

	// BASE must match for the use case to match
	result.Matched = (result.Scenarios & ScenarioBASE) != 0

	// If BASE didn't match, collect missing info
	if !result.Matched {
		base := def.BaseScenario()
		if base != nil {
			for _, freq := range base.Features {
				if !freq.Required {
					continue
				}
				fp, featurePresent := ep.Features[freq.FeatureID]
				if !featurePresent {
					result.MissingRequired = append(result.MissingRequired, freq.FeatureName)
					continue
				}
				if !checkAttributes(freq, fp) {
					result.MissingRequired = append(result.MissingRequired, freq.FeatureName+" (attributes)")
					continue
				}
				if !checkCommands(freq, fp) {
					result.MissingRequired = append(result.MissingRequired, freq.FeatureName+" (commands)")
				}
			}
		}
	}

	return result
}

// matchScenarioFeatures checks if all features in a scenario are satisfied.
// Scenarios are atomic: ALL features must be present for the scenario to match,
// regardless of the Required flag (which only affects PICS validation severity).
func matchScenarioFeatures(features []FeatureRequirement, ep *EndpointProfile) bool {
	for _, freq := range features {
		fp, featurePresent := ep.Features[freq.FeatureID]
		if !featurePresent {
			return false
		}

		if !checkAttributes(freq, fp) {
			return false
		}

		if !checkCommands(freq, fp) {
			return false
		}
	}

	return true
}

func checkAttributes(freq FeatureRequirement, fp *FeatureProfile) bool {
	attrSet := make(map[uint16]bool, len(fp.AttributeIDs))
	for _, id := range fp.AttributeIDs {
		attrSet[id] = true
	}

	for _, ar := range freq.Attributes {
		if !attrSet[ar.AttrID] {
			return false
		}

		// Check required value constraint
		if ar.RequiredValue != nil {
			val, ok := fp.Attributes[ar.AttrID]
			if !ok {
				return false
			}
			boolVal, ok := val.(bool)
			if !ok || boolVal != *ar.RequiredValue {
				return false
			}
		}
	}

	return true
}

func checkCommands(freq FeatureRequirement, fp *FeatureProfile) bool {
	cmdSet := make(map[uint8]bool, len(fp.CommandIDs))
	for _, id := range fp.CommandIDs {
		cmdSet[id] = true
	}

	for _, cr := range freq.Commands {
		if !cmdSet[cr.CommandID] {
			return false
		}
	}

	return true
}

func containsType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

// enforceScenarioConstraints iteratively clears scenario bits whose
// Requires (all must be set) or RequiresAny (at least one must be set)
// constraints are not satisfied, until stable.
func enforceScenarioConstraints(def *UseCaseDef, scenarios ScenarioMap) ScenarioMap {
	// Build name -> bit lookup
	nameToBit := make(map[string]ScenarioBit, len(def.Scenarios))
	for _, s := range def.Scenarios {
		nameToBit[s.Name] = s.Bit
	}

	for {
		changed := false
		for _, s := range def.Scenarios {
			bit := ScenarioMap(1) << ScenarioMap(s.Bit)
			if scenarios&bit == 0 {
				continue // already cleared
			}

			// Check Requires: all listed scenarios must be set
			for _, req := range s.Requires {
				if reqBit, ok := nameToBit[req]; ok {
					if scenarios&(1<<ScenarioMap(reqBit)) == 0 {
						scenarios &^= bit
						changed = true
						break
					}
				}
			}
			if scenarios&bit == 0 {
				continue
			}

			// Check RequiresAny: at least one must be set
			if len(s.RequiresAny) > 0 {
				found := false
				for _, req := range s.RequiresAny {
					if reqBit, ok := nameToBit[req]; ok {
						if scenarios&(1<<ScenarioMap(reqBit)) != 0 {
							found = true
							break
						}
					}
				}
				if !found {
					scenarios &^= bit
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	return scenarios
}

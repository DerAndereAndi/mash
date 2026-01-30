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

	allRequired := true

	for _, freq := range def.Features {
		fp, featurePresent := ep.Features[freq.FeatureID]

		if !featurePresent {
			if freq.Required {
				result.MissingRequired = append(result.MissingRequired, freq.FeatureName)
				allRequired = false
			} else {
				result.OptionalMissing = append(result.OptionalMissing, freq.FeatureName)
			}
			continue
		}

		// Check required attributes
		if !checkAttributes(freq, fp) {
			if freq.Required {
				result.MissingRequired = append(result.MissingRequired, freq.FeatureName+" (attributes)")
				allRequired = false
			} else {
				result.OptionalMissing = append(result.OptionalMissing, freq.FeatureName+" (attributes)")
			}
			continue
		}

		// Check required commands
		if !checkCommands(freq, fp) {
			if freq.Required {
				result.MissingRequired = append(result.MissingRequired, freq.FeatureName+" (commands)")
				allRequired = false
			} else {
				result.OptionalMissing = append(result.OptionalMissing, freq.FeatureName+" (commands)")
			}
		}
	}

	result.Matched = allRequired
	return result
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

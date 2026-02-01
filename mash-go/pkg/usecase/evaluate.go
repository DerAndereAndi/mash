package usecase

import (
	"sort"

	"github.com/mash-protocol/mash-go/pkg/model"
)

// DeviceEvaluator provides access to the local device model for use case evaluation.
type DeviceEvaluator interface {
	DeviceID() string
	Endpoints() []*model.Endpoint
}

// EvaluateDevice determines which use cases a device supports based on its
// local features and the given use case registry. It returns a slice of
// UseCaseDecl suitable for populating the DeviceInfo useCases attribute.
func EvaluateDevice(device DeviceEvaluator, registry map[UseCaseName]*UseCaseDef) []*model.UseCaseDecl {
	// Build a DeviceProfile from the local model
	profile := buildLocalProfile(device)

	// Run the existing matcher
	du := MatchAll(profile, registry)

	// Convert matches to wire declarations
	var decls []*model.UseCaseDecl
	for _, m := range du.Matches {
		if !m.Matched {
			continue
		}
		def := registry[m.UseCase]
		if def == nil {
			continue
		}
		decls = append(decls, &model.UseCaseDecl{
			EndpointID: m.EndpointID,
			ID:         uint16(def.ID),
			Major:      def.Major,
			Minor:      def.Minor,
			Scenarios:  uint32(m.Scenarios),
		})
	}

	return decls
}

// EvaluateController returns use case declarations for a controller.
// Controllers declare all registered use cases with EndpointID 0,
// indicating client-side support. Results are sorted by ID for determinism.
// Controllers declare all defined scenarios for each use case.
func EvaluateController(registry map[UseCaseName]*UseCaseDef) []*model.UseCaseDecl {
	decls := make([]*model.UseCaseDecl, 0, len(registry))
	for _, def := range registry {
		decls = append(decls, &model.UseCaseDecl{
			EndpointID: 0,
			ID:         uint16(def.ID),
			Major:      def.Major,
			Minor:      def.Minor,
			Scenarios:  uint32(def.DefinedScenarioMask()),
		})
	}
	sort.Slice(decls, func(i, j int) bool {
		return decls[i].ID < decls[j].ID
	})
	return decls
}

// buildLocalProfile creates a DeviceProfile from the local device model.
func buildLocalProfile(device DeviceEvaluator) *DeviceProfile {
	profile := &DeviceProfile{
		DeviceID:  device.DeviceID(),
		Endpoints: make(map[uint8]*EndpointProfile),
	}

	for _, ep := range device.Endpoints() {
		if ep.ID() == 0 {
			continue // Skip DEVICE_ROOT
		}

		epProfile := &EndpointProfile{
			EndpointID:   ep.ID(),
			EndpointType: ep.Type().String(),
			Features:     make(map[uint8]*FeatureProfile),
		}

		for _, f := range ep.Features() {
			fp := &FeatureProfile{
				FeatureID:    uint8(f.Type()),
				FeatureMap:   f.FeatureMap(),
				AttributeIDs: f.AttributeList(),
				CommandIDs:   f.CommandList(),
				Attributes:   make(map[uint16]any),
			}

			// Read capability boolean attributes for matching
			for _, attrID := range f.AttributeList() {
				val, err := f.ReadAttribute(attrID)
				if err == nil {
					if _, ok := val.(bool); ok {
						fp.Attributes[attrID] = val
					}
				}
			}

			epProfile.Features[uint8(f.Type())] = fp
		}

		profile.Endpoints[ep.ID()] = epProfile
	}

	return profile
}

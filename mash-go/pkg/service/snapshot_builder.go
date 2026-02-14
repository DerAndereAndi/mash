package service

import (
	"sort"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// buildDeviceSnapshot creates a DeviceSnapshot from a model.Device.
// It captures the complete capability state: endpoints, features,
// featureMaps, attributeLists, commandLists, specVersion, and use cases.
func buildDeviceSnapshot(device DeviceModel) *log.DeviceSnapshot {
	if device == nil {
		return nil
	}

	snap := &log.DeviceSnapshot{
		DeviceID: device.DeviceID(),
	}

	// Read specVersion and useCases from DeviceInfo on endpoint 0.
	if di, err := device.RootEndpoint().GetFeature(model.FeatureDeviceInfo); err == nil {
		if v, err := di.ReadAttribute(features.DeviceInfoAttrSpecVersion); err == nil {
			if s, ok := v.(string); ok {
				snap.SpecVersion = s
			}
		}
		if v, err := di.ReadAttribute(features.DeviceInfoAttrUseCases); err == nil {
			if ucs, ok := v.([]*model.UseCaseDecl); ok {
				for _, uc := range ucs {
					snap.UseCases = append(snap.UseCases, log.UseCaseSnapshot{
						EndpointID: uc.EndpointID,
						ID:         uc.ID,
						Major:      uc.Major,
						Minor:      uc.Minor,
						Scenarios:  uc.Scenarios,
					})
				}
			}
		}
	}

	// Build endpoint snapshots, sorted by ID for deterministic output.
	endpoints := device.Endpoints()
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].ID() < endpoints[j].ID()
	})

	for _, ep := range endpoints {
		epSnap := log.EndpointSnapshot{
			ID:   ep.ID(),
			Type: uint8(ep.Type()),
		}
		if ep.Label() != "" {
			epSnap.Label = ep.Label()
		}

		// Build feature snapshots, sorted by type ID for deterministic output.
		feats := ep.Features()
		sort.Slice(feats, func(i, j int) bool {
			return feats[i].Type() < feats[j].Type()
		})

		for _, f := range feats {
			fSnap := log.FeatureSnapshot{
				ID:            uint16(f.Type()),
				FeatureMap:    f.FeatureMap(),
				AttributeList: f.AttributeList(),
			}
			if cmds := f.CommandList(); len(cmds) > 0 {
				fSnap.CommandList = cmds
			}
			epSnap.Features = append(epSnap.Features, fSnap)
		}

		snap.Endpoints = append(snap.Endpoints, epSnap)
	}

	return snap
}

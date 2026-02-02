package service

import (
	"context"
	"sort"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// readRemoteCapabilities reads the full device model from a remote device
// and returns it as a DeviceSnapshot for caching on the session's snapshot
// tracker. Returns nil on failure (graceful degradation).
func readRemoteCapabilities(ctx context.Context, session *DeviceSession) *log.DeviceSnapshot {
	// Step 1: Read DeviceInfo attributes from endpoint 0.
	attrs, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), []uint16{
		features.DeviceInfoAttrDeviceID,
		features.DeviceInfoAttrSpecVersion,
		features.DeviceInfoAttrEndpoints,
		features.DeviceInfoAttrUseCases,
	})
	if err != nil {
		return nil
	}

	snap := &log.DeviceSnapshot{}

	if v, ok := attrs[features.DeviceInfoAttrDeviceID].(string); ok {
		snap.DeviceID = v
	}
	if v, ok := attrs[features.DeviceInfoAttrSpecVersion].(string); ok {
		snap.SpecVersion = v
	}

	snap.UseCases = parseRemoteUseCases(attrs[features.DeviceInfoAttrUseCases])

	// Step 2: For each endpoint, read global attributes from each feature.
	endpoints := parseRemoteEndpoints(attrs[features.DeviceInfoAttrEndpoints])
	for _, ep := range endpoints {
		epSnap := log.EndpointSnapshot{
			ID:    ep.id,
			Type:  ep.epType,
			Label: ep.label,
		}

		sort.Slice(ep.features, func(i, j int) bool {
			return ep.features[i] < ep.features[j]
		})

		for _, featureID := range ep.features {
			fSnap := readRemoteFeature(ctx, session, ep.id, featureID)
			epSnap.Features = append(epSnap.Features, fSnap)
		}

		snap.Endpoints = append(snap.Endpoints, epSnap)
	}

	return snap
}

// remoteEndpointInfo holds parsed endpoint data from DeviceInfo.
type remoteEndpointInfo struct {
	id       uint8
	epType   uint8
	label    string
	features []uint16
}

// parseRemoteEndpoints parses the CBOR-decoded endpoints attribute value.
func parseRemoteEndpoints(raw any) []remoteEndpointInfo {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	var eps []remoteEndpointInfo
	for _, item := range arr {
		m, ok := item.(map[any]any)
		if !ok {
			continue
		}

		var ep remoteEndpointInfo

		if v, ok := m[uint64(1)]; ok {
			if id, ok := wire.ToUint8Public(v); ok {
				ep.id = id
			}
		}
		if v, ok := m[uint64(2)]; ok {
			if t, ok := wire.ToUint8Public(v); ok {
				ep.epType = t
			}
		}
		if v, ok := m[uint64(3)]; ok {
			if s, ok := v.(string); ok {
				ep.label = s
			}
		}
		if v, ok := m[uint64(4)]; ok {
			if feats, ok := v.([]any); ok {
				for _, f := range feats {
					if id, ok := wire.ToUint32(f); ok {
						ep.features = append(ep.features, uint16(id))
					}
				}
			}
		}

		eps = append(eps, ep)
	}

	sort.Slice(eps, func(i, j int) bool {
		return eps[i].id < eps[j].id
	})

	return eps
}

// parseRemoteUseCases parses the CBOR-decoded useCases attribute value.
func parseRemoteUseCases(raw any) []log.UseCaseSnapshot {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}

	var ucs []log.UseCaseSnapshot
	for _, item := range arr {
		m, ok := item.(map[any]any)
		if !ok {
			continue
		}

		var uc log.UseCaseSnapshot

		if v, ok := m[uint64(1)]; ok {
			if id, ok := wire.ToUint8Public(v); ok {
				uc.EndpointID = id
			}
		}
		if v, ok := m[uint64(2)]; ok {
			if id, ok := wire.ToUint32(v); ok {
				uc.ID = uint16(id)
			}
		}
		if v, ok := m[uint64(3)]; ok {
			if maj, ok := wire.ToUint8Public(v); ok {
				uc.Major = maj
			}
		}
		if v, ok := m[uint64(4)]; ok {
			if min, ok := wire.ToUint8Public(v); ok {
				uc.Minor = min
			}
		}
		if v, ok := m[uint64(5)]; ok {
			if sc, ok := wire.ToUint32(v); ok {
				uc.Scenarios = sc
			}
		}

		ucs = append(ucs, uc)
	}

	return ucs
}

// readRemoteFeature reads global attributes for a single feature.
func readRemoteFeature(ctx context.Context, session *DeviceSession, endpointID uint8, featureTypeID uint16) log.FeatureSnapshot {
	fSnap := log.FeatureSnapshot{
		ID: featureTypeID,
	}

	attrs, err := session.Read(ctx, endpointID, uint8(featureTypeID), []uint16{
		model.AttrIDFeatureMap,
		model.AttrIDAttributeList,
		model.AttrIDCommandList,
	})
	if err != nil {
		return fSnap
	}

	if v, ok := wire.ToUint32(attrs[model.AttrIDFeatureMap]); ok {
		fSnap.FeatureMap = v
	}

	if v, ok := attrs[model.AttrIDAttributeList].([]any); ok {
		for _, a := range v {
			if id, ok := wire.ToUint32(a); ok {
				fSnap.AttributeList = append(fSnap.AttributeList, uint16(id))
			}
		}
	}

	if v, ok := attrs[model.AttrIDCommandList].([]any); ok {
		for _, c := range v {
			if id, ok := wire.ToUint8Public(c); ok {
				fSnap.CommandList = append(fSnap.CommandList, id)
			}
		}
	}

	return fSnap
}

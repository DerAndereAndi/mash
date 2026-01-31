package usecase

import (
	"context"
	"fmt"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// DeviceReader is the subset of DeviceClient needed for discovery.
type DeviceReader interface {
	Read(ctx context.Context, endpointID uint8, featureID uint8, attrIDs []uint16) (map[uint16]any, error)
}

// knownFeatureIDs lists the feature types to probe on each endpoint.
var knownFeatureIDs = []uint8{
	uint8(model.FeatureStatus),
	uint8(model.FeatureElectrical),
	uint8(model.FeatureMeasurement),
	uint8(model.FeatureEnergyControl),
	uint8(model.FeatureChargingSession),
}

// energyControlCapabilityAttrs are the boolean capability attributes to read
// from EnergyControl when it is present.
var energyControlCapabilityAttrs = []uint16{
	features.EnergyControlAttrAcceptsLimits,
	features.EnergyControlAttrAcceptsCurrentLimits,
	features.EnergyControlAttrAcceptsSetpoints,
	features.EnergyControlAttrAcceptsCurrentSetpoints,
	features.EnergyControlAttrIsPausable,
	features.EnergyControlAttrIsShiftable,
	features.EnergyControlAttrIsStoppable,
}

// DiscoverDevice probes a device to build a DeviceProfile.
func DiscoverDevice(ctx context.Context, client DeviceReader, deviceID string) (*DeviceProfile, error) {
	// Read endpoints from DeviceInfo on endpoint 0
	attrs, err := client.Read(ctx, 0, uint8(model.FeatureDeviceInfo), []uint16{features.DeviceInfoAttrEndpoints})
	if err != nil {
		return nil, fmt.Errorf("reading DeviceInfo endpoints: %w", err)
	}

	endpoints, err := parseEndpoints(attrs[features.DeviceInfoAttrEndpoints])
	if err != nil {
		return nil, fmt.Errorf("parsing endpoints: %w", err)
	}

	profile := &DeviceProfile{
		DeviceID:  deviceID,
		Endpoints: make(map[uint8]*EndpointProfile),
	}

	for _, epInfo := range endpoints {
		ep, err := discoverEndpoint(ctx, client, epInfo.id, epInfo.epType, knownFeatureIDs)
		if err != nil {
			continue // Skip endpoints that fail to probe
		}
		profile.Endpoints[epInfo.id] = ep
	}

	return profile, nil
}

// DiscoverUseCases discovers what use cases a remote device supports.
// It first tries the fast path: reading the useCases attribute from DeviceInfo.
// If that attribute is present, it builds DeviceUseCases directly from the
// declarations without probing individual features. If absent, it falls back
// to the existing probe-and-match flow.
func DiscoverUseCases(ctx context.Context, client DeviceReader, deviceID string, registry map[UseCaseName]*UseCaseDef) (*DeviceUseCases, error) {
	// Try fast path: read both endpoints and useCases from DeviceInfo
	attrs, err := client.Read(ctx, 0, uint8(model.FeatureDeviceInfo), []uint16{
		features.DeviceInfoAttrEndpoints,
		features.DeviceInfoAttrUseCases,
	})
	if err != nil {
		return nil, fmt.Errorf("reading DeviceInfo: %w", err)
	}

	// Check if useCases attribute is present
	if ucRaw, ok := attrs[features.DeviceInfoAttrUseCases]; ok {
		decls, err := parseUseCases(ucRaw)
		if err == nil && len(decls) > 0 {
			return buildFromDeclarations(deviceID, decls, registry), nil
		}
	}

	// Fall back to probe-and-match
	endpoints, err := parseEndpoints(attrs[features.DeviceInfoAttrEndpoints])
	if err != nil {
		return nil, fmt.Errorf("parsing endpoints: %w", err)
	}

	profile := &DeviceProfile{
		DeviceID:  deviceID,
		Endpoints: make(map[uint8]*EndpointProfile),
	}
	for _, epInfo := range endpoints {
		ep, err := discoverEndpoint(ctx, client, epInfo.id, epInfo.epType, knownFeatureIDs)
		if err != nil {
			continue
		}
		profile.Endpoints[epInfo.id] = ep
	}

	du := MatchAll(profile, registry)
	return du, nil
}

// buildFromDeclarations constructs DeviceUseCases from parsed UseCaseDecl values.
func buildFromDeclarations(deviceID string, decls []*model.UseCaseDecl, registry map[UseCaseName]*UseCaseDef) *DeviceUseCases {
	du := &DeviceUseCases{
		DeviceID: deviceID,
		registry: registry,
	}
	for _, d := range decls {
		du.Matches = append(du.Matches, MatchResult{
			UseCase:    UseCaseName(d.Name),
			Matched:    true,
			EndpointID: d.EndpointID,
		})
	}
	return du
}

// parseUseCases parses the raw useCases attribute value into UseCaseDecl structs.
func parseUseCases(raw any) ([]*model.UseCaseDecl, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("useCases is not an array: %T", raw)
	}

	var decls []*model.UseCaseDecl
	for _, item := range arr {
		m, ok := item.(map[any]any)
		if !ok {
			continue
		}

		d := &model.UseCaseDecl{}

		// Parse endpointId (key 1)
		if v, ok := m[uint64(1)]; ok {
			if id, ok := toUint8(v); ok {
				d.EndpointID = id
			}
		}

		// Parse name (key 2)
		if v, ok := m[uint64(2)]; ok {
			if s, ok := v.(string); ok {
				d.Name = s
			}
		}

		// Parse major (key 3)
		if v, ok := m[uint64(3)]; ok {
			if maj, ok := toUint8(v); ok {
				d.Major = maj
			}
		}

		// Parse minor (key 4)
		if v, ok := m[uint64(4)]; ok {
			if min, ok := toUint8(v); ok {
				d.Minor = min
			}
		}

		if d.Name != "" {
			decls = append(decls, d)
		}
	}

	return decls, nil
}

type endpointInfo struct {
	id     uint8
	epType string
}

func parseEndpoints(raw any) ([]endpointInfo, error) {
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("endpoints is not an array: %T", raw)
	}

	var eps []endpointInfo
	for _, item := range arr {
		m, ok := item.(map[any]any)
		if !ok {
			continue
		}

		var ei endpointInfo

		// Parse ID (key 1)
		if v, ok := m[uint64(1)]; ok {
			if id, ok := toUint8(v); ok {
				ei.id = id
			}
		}

		// Parse Type (key 2)
		if v, ok := m[uint64(2)]; ok {
			if typ, ok := toUint8(v); ok {
				ei.epType = model.EndpointType(typ).String()
			}
		}

		// Skip endpoint 0 (DEVICE_ROOT)
		if ei.id == 0 {
			continue
		}

		eps = append(eps, ei)
	}

	return eps, nil
}

func discoverEndpoint(ctx context.Context, client DeviceReader, epID uint8, epType string, featureIDs []uint8) (*EndpointProfile, error) {
	ep := &EndpointProfile{
		EndpointID:   epID,
		EndpointType: epType,
		Features:     make(map[uint8]*FeatureProfile),
	}

	for _, fID := range featureIDs {
		fp, err := probeFeature(ctx, client, epID, fID)
		if err != nil {
			continue // Probe error = feature not supported
		}
		if fp != nil {
			ep.Features[fID] = fp
		}
	}

	return ep, nil
}

func probeFeature(ctx context.Context, client DeviceReader, epID uint8, featureID uint8) (*FeatureProfile, error) {
	// Step 1: Read attributeList to determine presence
	attrs, err := client.Read(ctx, epID, featureID, []uint16{model.AttrIDAttributeList})
	if err != nil {
		return nil, nil // Feature absent -- not an error
	}

	fp := &FeatureProfile{
		FeatureID:  featureID,
		Attributes: make(map[uint16]any),
	}

	// Parse attribute list
	if raw, ok := attrs[model.AttrIDAttributeList]; ok {
		fp.AttributeIDs = parseUint16List(raw)
	}

	// Step 2: Read commandList
	cmdAttrs, err := client.Read(ctx, epID, featureID, []uint16{model.AttrIDCommandList})
	if err == nil {
		if raw, ok := cmdAttrs[model.AttrIDCommandList]; ok {
			fp.CommandIDs = parseUint8List(raw)
		}
	}

	// Step 3: Read featureMap
	fmAttrs, err := client.Read(ctx, epID, featureID, []uint16{model.AttrIDFeatureMap})
	if err == nil {
		if raw, ok := fmAttrs[model.AttrIDFeatureMap]; ok {
			if v, ok := toUint32(raw); ok {
				fp.FeatureMap = v
			}
		}
	}

	// Step 4: For EnergyControl, read capability booleans
	if featureID == uint8(model.FeatureEnergyControl) {
		capAttrs, err := client.Read(ctx, epID, featureID, energyControlCapabilityAttrs)
		if err == nil {
			for id, val := range capAttrs {
				fp.Attributes[id] = val
			}
		}
	}

	return fp, nil
}

// --- Type conversion helpers ---

func parseUint16List(raw any) []uint16 {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	ids := make([]uint16, 0, len(arr))
	for _, v := range arr {
		if id, ok := toUint16(v); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseUint8List(raw any) []uint8 {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	ids := make([]uint8, 0, len(arr))
	for _, v := range arr {
		if id, ok := toUint8(v); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func toUint8(v any) (uint8, bool) {
	switch val := v.(type) {
	case uint8:
		return val, true
	case uint64:
		return uint8(val), true
	case int64:
		return uint8(val), true
	case uint16:
		return uint8(val), true
	case uint32:
		return uint8(val), true
	case int:
		return uint8(val), true
	}
	return 0, false
}

func toUint16(v any) (uint16, bool) {
	switch val := v.(type) {
	case uint16:
		return val, true
	case uint64:
		return uint16(val), true
	case int64:
		return uint16(val), true
	case uint8:
		return uint16(val), true
	case uint32:
		return uint16(val), true
	case int:
		return uint16(val), true
	}
	return 0, false
}

func toUint32(v any) (uint32, bool) {
	switch val := v.(type) {
	case uint32:
		return val, true
	case uint64:
		return uint32(val), true
	case int64:
		return uint32(val), true
	case int:
		return uint32(val), true
	}
	return 0, false
}

package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/mash-protocol/mash-go/pkg/interaction"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// mockClient implements examples.DeviceClient for testing.
type mockClient struct {
	readFunc func(ctx context.Context, endpointID uint8, featureID uint8, attrIDs []uint16) (map[uint16]any, error)
}

func (m *mockClient) Read(ctx context.Context, endpointID uint8, featureID uint8, attrIDs []uint16) (map[uint16]any, error) {
	if m.readFunc != nil {
		return m.readFunc(ctx, endpointID, featureID, attrIDs)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockClient) Write(ctx context.Context, endpointID uint8, featureID uint8, attrs map[uint16]any) (map[uint16]any, error) {
	return nil, nil
}

func (m *mockClient) Subscribe(ctx context.Context, endpointID uint8, featureID uint8, opts *interaction.SubscribeOptions) (uint32, map[uint16]any, error) {
	return 0, nil, nil
}

func (m *mockClient) Unsubscribe(ctx context.Context, subscriptionID uint32) error {
	return nil
}

func (m *mockClient) Invoke(ctx context.Context, endpointID uint8, featureID uint8, commandID uint8, params map[string]any) (any, error) {
	return nil, nil
}

func TestProbeFeature_Present(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			if epID != 1 || fID != uint8(model.FeatureMeasurement) {
				return nil, fmt.Errorf("unexpected read")
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
				return map[uint16]any{
					model.AttrIDAttributeList: []any{uint64(1), uint64(2), uint64(3)},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
				return map[uint16]any{
					model.AttrIDCommandList: []any{},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
				return map[uint16]any{
					model.AttrIDFeatureMap: uint64(0x0001),
				}, nil
			}
			return nil, fmt.Errorf("unexpected attrIDs: %v", attrIDs)
		},
	}

	fp, err := probeFeature(context.Background(), client, 1, uint8(model.FeatureMeasurement))
	if err != nil {
		t.Fatalf("probeFeature: %v", err)
	}
	if fp == nil {
		t.Fatal("expected non-nil FeatureProfile")
	}
	if fp.FeatureID != uint8(model.FeatureMeasurement) {
		t.Errorf("FeatureID = 0x%02x, want 0x04", fp.FeatureID)
	}
	if len(fp.AttributeIDs) != 3 {
		t.Errorf("AttributeIDs length = %d, want 3", len(fp.AttributeIDs))
	}
	if fp.FeatureMap != 0x0001 {
		t.Errorf("FeatureMap = 0x%04x, want 0x0001", fp.FeatureMap)
	}
}

func TestProbeFeature_Absent(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			return nil, fmt.Errorf("feature not found")
		},
	}

	fp, err := probeFeature(context.Background(), client, 1, uint8(model.FeatureEnergyControl))
	if err != nil {
		t.Fatalf("probeFeature should not error for absent feature: %v", err)
	}
	if fp != nil {
		t.Error("expected nil FeatureProfile for absent feature")
	}
}

func TestProbeFeature_WithCapabilities(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			if fID != uint8(model.FeatureEnergyControl) {
				return nil, fmt.Errorf("unexpected feature")
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
				return map[uint16]any{
					model.AttrIDAttributeList: []any{uint64(1), uint64(2), uint64(10), uint64(14)},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
				return map[uint16]any{
					model.AttrIDCommandList: []any{uint64(1), uint64(2), uint64(9)},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
				return map[uint16]any{
					model.AttrIDFeatureMap: uint64(0x0003),
				}, nil
			}
			// Capability reads
			result := make(map[uint16]any)
			for _, id := range attrIDs {
				switch id {
				case 10: // acceptsLimits
					result[id] = true
				case 14: // isPausable
					result[id] = true
				}
			}
			return result, nil
		},
	}

	fp, err := probeFeature(context.Background(), client, 1, uint8(model.FeatureEnergyControl))
	if err != nil {
		t.Fatalf("probeFeature: %v", err)
	}
	if fp == nil {
		t.Fatal("expected non-nil FeatureProfile")
	}

	// Check capabilities were read
	if v, ok := fp.Attributes[10]; !ok || v != true {
		t.Errorf("acceptsLimits should be true, got %v", fp.Attributes[10])
	}
	if v, ok := fp.Attributes[14]; !ok || v != true {
		t.Errorf("isPausable should be true, got %v", fp.Attributes[14])
	}

	// Check command list
	if len(fp.CommandIDs) != 3 {
		t.Errorf("CommandIDs length = %d, want 3", len(fp.CommandIDs))
	}
}

func TestDiscoverEndpoint_MultipleFeatures(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			if epID != 1 {
				return nil, fmt.Errorf("unexpected endpoint")
			}
			// All three features exist
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
				return map[uint16]any{
					model.AttrIDAttributeList: []any{uint64(1)},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
				return map[uint16]any{
					model.AttrIDCommandList: []any{},
				}, nil
			}
			if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
				return map[uint16]any{
					model.AttrIDFeatureMap: uint64(0),
				}, nil
			}
			// Capability reads for EnergyControl
			if fID == uint8(model.FeatureEnergyControl) {
				result := make(map[uint16]any)
				for _, id := range attrIDs {
					result[id] = false
				}
				return result, nil
			}
			return map[uint16]any{}, nil
		},
	}

	featureIDs := []uint8{
		uint8(model.FeatureElectrical),
		uint8(model.FeatureMeasurement),
		uint8(model.FeatureEnergyControl),
	}

	ep, err := discoverEndpoint(context.Background(), client, 1, "EV_CHARGER", featureIDs)
	if err != nil {
		t.Fatalf("discoverEndpoint: %v", err)
	}
	if len(ep.Features) != 3 {
		t.Errorf("features count = %d, want 3", len(ep.Features))
	}
}

func TestDiscoverEndpoint_SingleFeature(t *testing.T) {
	callCount := 0
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			callCount++
			// Only Measurement exists
			if fID == uint8(model.FeatureMeasurement) {
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
					return map[uint16]any{
						model.AttrIDAttributeList: []any{uint64(1)},
					}, nil
				}
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
					return map[uint16]any{
						model.AttrIDCommandList: []any{},
					}, nil
				}
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
					return map[uint16]any{
						model.AttrIDFeatureMap: uint64(0),
					}, nil
				}
			}
			return nil, fmt.Errorf("feature not found")
		},
	}

	featureIDs := []uint8{
		uint8(model.FeatureElectrical),
		uint8(model.FeatureMeasurement),
		uint8(model.FeatureEnergyControl),
	}

	ep, err := discoverEndpoint(context.Background(), client, 1, "EV_CHARGER", featureIDs)
	if err != nil {
		t.Fatalf("discoverEndpoint: %v", err)
	}
	if len(ep.Features) != 1 {
		t.Errorf("features count = %d, want 1", len(ep.Features))
	}
	if _, ok := ep.Features[uint8(model.FeatureMeasurement)]; !ok {
		t.Error("Measurement feature should be present")
	}
}

func TestDiscoverDevice_SingleEndpoint(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			// Endpoint 0, DeviceInfo, read endpoints
			if epID == 0 && fID == uint8(model.FeatureDeviceInfo) {
				return map[uint16]any{
					20: []any{ // endpoints
						map[any]any{
							uint64(1): uint64(1),    // ID: 1
							uint64(2): uint64(0x05), // Type: EV_CHARGER
						},
					},
				}, nil
			}
			// Endpoint 1 feature probes
			if epID == 1 {
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
					if fID == uint8(model.FeatureMeasurement) {
						return map[uint16]any{
							model.AttrIDAttributeList: []any{uint64(1)},
						}, nil
					}
					return nil, fmt.Errorf("feature not found")
				}
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
					return map[uint16]any{
						model.AttrIDCommandList: []any{},
					}, nil
				}
				if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
					return map[uint16]any{
						model.AttrIDFeatureMap: uint64(0),
					}, nil
				}
			}
			return nil, fmt.Errorf("not found")
		},
	}

	profile, err := DiscoverDevice(context.Background(), client, "test-device")
	if err != nil {
		t.Fatalf("DiscoverDevice: %v", err)
	}
	if profile.DeviceID != "test-device" {
		t.Errorf("DeviceID = %q", profile.DeviceID)
	}
	if len(profile.Endpoints) != 1 {
		t.Errorf("endpoints count = %d, want 1", len(profile.Endpoints))
	}
	ep, ok := profile.Endpoints[1]
	if !ok {
		t.Fatal("missing endpoint 1")
	}
	if ep.EndpointType != "EV_CHARGER" {
		t.Errorf("EndpointType = %q, want EV_CHARGER", ep.EndpointType)
	}
}

func TestDiscoverUseCases_FastPath(t *testing.T) {
	// Mock client returns useCases attribute with uint16 IDs -> fast path
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			if epID == 0 && fID == uint8(model.FeatureDeviceInfo) {
				return map[uint16]any{
					20: []any{ // endpoints
						map[any]any{
							uint64(1): uint64(1),
							uint64(2): uint64(0x05), // EV_CHARGER
						},
					},
					21: []any{ // useCases -- now with uint16 IDs
						map[any]any{
							uint64(1): uint64(1),      // endpointId
							uint64(2): uint64(0x01),    // ID: GPL
							uint64(3): uint64(1),       // major
							uint64(4): uint64(0),       // minor
							uint64(5): uint64(0x03),    // scenarios: BASE + CONSUMPTION
						},
						map[any]any{
							uint64(1): uint64(1),
							uint64(2): uint64(0x02), // ID: MPD
							uint64(3): uint64(1),
							uint64(4): uint64(0),
							uint64(5): uint64(0x01), // scenarios: BASE only
						},
					},
				}, nil
			}
			// If any feature probing happens, fail -- fast path should skip it
			return nil, fmt.Errorf("should not probe features in fast path")
		},
	}

	du, err := DiscoverUseCases(context.Background(), client, "test-device", Registry)
	if err != nil {
		t.Fatalf("DiscoverUseCases: %v", err)
	}
	if !du.HasUseCase(GPL) {
		t.Error("expected GPL to be present")
	}
	if !du.HasUseCase(MPD) {
		t.Error("expected MPD to be present")
	}
	if du.HasUseCase(EVC) {
		t.Error("expected EVC to be absent")
	}

	// Verify endpoint IDs
	epID, ok := du.EndpointForUseCase(GPL)
	if !ok || epID != 1 {
		t.Errorf("EndpointForUseCase(GPL) = (%d, %v), want (1, true)", epID, ok)
	}

	// Verify scenarios
	scenarios, ok := du.ScenariosForUseCase(GPL)
	if !ok {
		t.Fatal("expected ScenariosForUseCase(GPL) to return ok")
	}
	if scenarios != 0x03 {
		t.Errorf("GPL scenarios = 0x%02X, want 0x03", scenarios)
	}
}

func TestDiscoverUseCases_FallbackToProbing(t *testing.T) {
	// Mock client returns endpoints but no useCases -> should fall back to probing
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			if epID == 0 && fID == uint8(model.FeatureDeviceInfo) {
				return map[uint16]any{
					20: []any{ // endpoints only, no useCases
						map[any]any{
							uint64(1): uint64(1),
							uint64(2): uint64(0x05), // EV_CHARGER
						},
					},
				}, nil
			}
			// Endpoint 1 probes: only Measurement exists
			if epID == 1 {
				if fID == uint8(model.FeatureMeasurement) {
					if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDAttributeList {
						return map[uint16]any{
							model.AttrIDAttributeList: []any{uint64(1)},
						}, nil
					}
					if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDCommandList {
						return map[uint16]any{
							model.AttrIDCommandList: []any{},
						}, nil
					}
					if len(attrIDs) == 1 && attrIDs[0] == model.AttrIDFeatureMap {
						return map[uint16]any{
							model.AttrIDFeatureMap: uint64(0),
						}, nil
					}
				}
				return nil, fmt.Errorf("feature not found")
			}
			return nil, fmt.Errorf("not found")
		},
	}

	du, err := DiscoverUseCases(context.Background(), client, "test-device", Registry)
	if err != nil {
		t.Fatalf("DiscoverUseCases: %v", err)
	}
	// Should match MPD via fallback probing
	if !du.HasUseCase(MPD) {
		t.Error("expected MPD to match via fallback probing")
	}
}

func TestParseUseCases_Valid(t *testing.T) {
	raw := []any{
		map[any]any{
			uint64(1): uint64(1),      // endpointId
			uint64(2): uint64(0x01),   // ID: GPL
			uint64(3): uint64(1),      // major
			uint64(4): uint64(0),      // minor
			uint64(5): uint64(0x07),   // scenarios
		},
		map[any]any{
			uint64(1): uint64(2),
			uint64(2): uint64(0x03), // ID: EVC
			uint64(3): uint64(1),
			uint64(4): uint64(2),
			uint64(5): uint64(0x3F), // scenarios
		},
	}

	decls, err := parseUseCases(raw)
	if err != nil {
		t.Fatalf("parseUseCases: %v", err)
	}
	if len(decls) != 2 {
		t.Fatalf("expected 2 decls, got %d", len(decls))
	}
	if decls[0].ID != 0x01 || decls[0].EndpointID != 1 || decls[0].Major != 1 || decls[0].Minor != 0 || decls[0].Scenarios != 0x07 {
		t.Errorf("decl[0] = %+v", decls[0])
	}
	if decls[1].ID != 0x03 || decls[1].EndpointID != 2 || decls[1].Major != 1 || decls[1].Minor != 2 || decls[1].Scenarios != 0x3F {
		t.Errorf("decl[1] = %+v", decls[1])
	}
}

func TestParseUseCases_Empty(t *testing.T) {
	decls, err := parseUseCases([]any{})
	if err != nil {
		t.Fatalf("parseUseCases: %v", err)
	}
	if len(decls) != 0 {
		t.Errorf("expected 0 decls, got %d", len(decls))
	}
}

func TestDiscoverDevice_ErrorReadingDeviceInfo(t *testing.T) {
	client := &mockClient{
		readFunc: func(_ context.Context, epID uint8, fID uint8, attrIDs []uint16) (map[uint16]any, error) {
			return nil, fmt.Errorf("connection error")
		},
	}

	_, err := DiscoverDevice(context.Background(), client, "test-device")
	if err == nil {
		t.Fatal("expected error when DeviceInfo read fails")
	}
}

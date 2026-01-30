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

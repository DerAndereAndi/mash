package service

import (
	"context"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// registerDeviceCommands registers service-level commands on the device's features.
// This must be called after the device model is set but before the service starts.
func (s *DeviceService) registerDeviceCommands() {
	// Get DeviceInfo feature from root endpoint (endpoint 0)
	rootEndpoint := s.device.RootEndpoint()
	if rootEndpoint == nil {
		return
	}

	deviceInfo, err := rootEndpoint.GetFeatureByID(uint8(model.FeatureDeviceInfo))
	if err != nil {
		return
	}

	// Register RemoveZone command
	removeZoneCmd := model.NewCommand(
		&model.CommandMetadata{
			ID:          features.DeviceInfoCmdRemoveZone,
			Name:        "RemoveZone",
			Description: "Remove a zone from this device (self-removal only)",
			Parameters: []model.ParameterMetadata{
				{Name: features.RemoveZoneParamZoneID, Type: model.DataTypeString, Required: true, Description: "Zone ID to remove"},
			},
			Response: []model.ParameterMetadata{
				{Name: features.RemoveZoneRespRemoved, Type: model.DataTypeBool, Description: "Whether the zone was removed"},
			},
		},
		s.makeRemoveZoneHandler(),
	)
	deviceInfo.AddCommand(removeZoneCmd)
}

// callerZoneIDKey is the context key for storing the caller's zone ID.
type callerZoneIDKey struct{}

// ContextWithCallerZoneID returns a new context with the caller's zone ID.
func ContextWithCallerZoneID(ctx context.Context, zoneID string) context.Context {
	return context.WithValue(ctx, callerZoneIDKey{}, zoneID)
}

// CallerZoneIDFromContext extracts the caller's zone ID from the context.
// Returns empty string if not set.
func CallerZoneIDFromContext(ctx context.Context) string {
	if v := ctx.Value(callerZoneIDKey{}); v != nil {
		if zoneID, ok := v.(string); ok {
			return zoneID
		}
	}
	return ""
}

// callerZoneTypeKey is the context key for storing the caller's zone type.
type callerZoneTypeKey struct{}

// ContextWithCallerZoneType returns a new context with the caller's zone type.
func ContextWithCallerZoneType(ctx context.Context, zoneType cert.ZoneType) context.Context {
	return context.WithValue(ctx, callerZoneTypeKey{}, zoneType)
}

// CallerZoneTypeFromContext extracts the caller's zone type from the context.
// Returns 0 (invalid) if not set.
func CallerZoneTypeFromContext(ctx context.Context) cert.ZoneType {
	if v := ctx.Value(callerZoneTypeKey{}); v != nil {
		if zt, ok := v.(cert.ZoneType); ok {
			return zt
		}
	}
	return 0
}

// makeRemoveZoneHandler creates a command handler for the RemoveZone command.
// This handler validates that only the zone itself can request removal (self-removal).
func (s *DeviceService) makeRemoveZoneHandler() model.CommandHandler {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		// Extract the zone ID parameter
		zoneIDParam, ok := params[features.RemoveZoneParamZoneID]
		if !ok {
			return nil, model.ErrInvalidParameters
		}
		zoneID, ok := zoneIDParam.(string)
		if !ok {
			return nil, model.ErrInvalidParameters
		}

		// Get the caller's zone ID from context
		callerZoneID := CallerZoneIDFromContext(ctx)
		if callerZoneID == "" {
			// No caller context - reject for safety
			return nil, model.ErrCommandNotAllowed
		}

		// Validate self-removal only: the caller must be the zone being removed.
		// Exception: when enable-key is valid, any zone can remove any zone
		// (needed for test orchestration, e.g. TC-ZTYPE-005/007).
		if zoneID != callerZoneID && !s.isEnableKeyValid() {
			return nil, model.ErrCommandNotAllowed
		}

		// Perform the removal
		if err := s.RemoveZone(zoneID); err != nil {
			return nil, err
		}

		return map[string]any{
			features.RemoveZoneRespRemoved: true,
		}, nil
	}
}

// Package zonecontext provides context keys for propagating caller zone
// identity (zone ID and zone type) through context.Context. This package
// exists as a neutral dependency that both pkg/service and pkg/features
// can import without creating an import cycle.
package zonecontext

import (
	"context"

	"github.com/mash-protocol/mash-go/pkg/cert"
)

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

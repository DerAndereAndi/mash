package runner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// registerZoneHandlers registers all zone management action handlers.
func (r *Runner) registerZoneHandlers() {
	r.engine.RegisterHandler("create_zone", r.handleCreateZone)
	r.engine.RegisterHandler("add_zone", r.handleAddZone)
	r.engine.RegisterHandler("delete_zone", r.handleDeleteZone)
	r.engine.RegisterHandler("remove_zone", r.handleRemoveZone)
	r.engine.RegisterHandler("get_zone", r.handleGetZone)
	r.engine.RegisterHandler("has_zone", r.handleHasZone)
	r.engine.RegisterHandler("list_zones", r.handleListZones)
	r.engine.RegisterHandler("zone_count", r.handleZoneCount)
	r.engine.RegisterHandler("get_zone_metadata", r.handleGetZoneMetadata)
	r.engine.RegisterHandler("get_zone_ca_fingerprint", r.handleGetZoneCAFingerprint)
	r.engine.RegisterHandler("verify_zone_ca", r.handleVerifyZoneCA)
	r.engine.RegisterHandler("verify_zone_binding", r.handleVerifyZoneBinding)
	r.engine.RegisterHandler("verify_zone_id_derivation", r.handleVerifyZoneIDDerivation)
	r.engine.RegisterHandler("highest_priority_zone", r.handleHighestPriorityZone)
	r.engine.RegisterHandler("highest_priority_connected_zone", r.handleHighestPriorityConnectedZone)
	r.engine.RegisterHandler("disconnect_zone", r.handleDisconnectZone)
	r.engine.RegisterHandler("verify_other_zone", r.handleVerifyOtherZone)
	r.engine.RegisterHandler("verify_bidirectional_active", r.handleVerifyBidirectionalActive)
	r.engine.RegisterHandler("verify_restore_sequence", r.handleVerifyRestoreSequence)
	r.engine.RegisterHandler("verify_tls_state", r.handleVerifyTLSState)
}

// handleCreateZone creates a new zone with a CA.
func (r *Runner) handleCreateZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneType, _ := params["zone_type"].(string)
	if zoneType == "" {
		zoneType = "HOME_MANAGER"
	}

	zoneID, _ := params["zone_id"].(string)
	if zoneID == "" {
		zoneID = generateZoneID()
	}

	if len(zs.zones) >= zs.maxZones {
		return nil, fmt.Errorf("maximum zones (%d) reached", zs.maxZones)
	}

	// Check for duplicate zone type.
	for _, z := range zs.zones {
		if z.ZoneType == zoneType {
			return nil, fmt.Errorf("zone type %s already exists", zoneType)
		}
	}

	priority := zonePriority[zoneType]
	fingerprint := generateFingerprint(zoneID)

	zone := &zoneInfo{
		ZoneID:        zoneID,
		ZoneType:      zoneType,
		Priority:      priority,
		Metadata:      make(map[string]any),
		CAFingerprint: fingerprint,
		Connected:     false,
		DeviceIDs:     make([]string, 0),
	}

	zs.zones[zoneID] = zone
	zs.zoneOrder = append(zs.zoneOrder, zoneID)

	return map[string]any{
		"zone_id":      zoneID,
		"zone_created": true,
		"zone_type":    zoneType,
		"fingerprint":  fingerprint,
	}, nil
}

// handleAddZone adds a device to an existing zone.
func (r *Runner) handleAddZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)
	deviceID, _ := params["device_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return nil, fmt.Errorf("zone %s not found", zoneID)
	}

	zone.DeviceIDs = append(zone.DeviceIDs, deviceID)

	return map[string]any{
		"device_added": true,
		"zone_id":      zoneID,
	}, nil
}

// handleDeleteZone removes a zone.
func (r *Runner) handleDeleteZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	if _, exists := zs.zones[zoneID]; !exists {
		return map[string]any{"zone_removed": false}, nil
	}

	delete(zs.zones, zoneID)

	// Remove from order.
	for i, id := range zs.zoneOrder {
		if id == zoneID {
			zs.zoneOrder = append(zs.zoneOrder[:i], zs.zoneOrder[i+1:]...)
			break
		}
	}

	return map[string]any{"zone_removed": true}, nil
}

// handleRemoveZone is an alias for delete_zone.
func (r *Runner) handleRemoveZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleDeleteZone(ctx, step, state)
}

// handleGetZone returns zone details.
func (r *Runner) handleGetZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"zone_found": false}, nil
	}

	return map[string]any{
		"zone_found":    true,
		"zone_id":       zone.ZoneID,
		"zone_type":     zone.ZoneType,
		"zone_metadata": zone.Metadata,
		"connected":     zone.Connected,
		"device_count":  len(zone.DeviceIDs),
	}, nil
}

// handleHasZone checks if a zone exists.
func (r *Runner) handleHasZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)
	_, exists := zs.zones[zoneID]

	return map[string]any{"zone_exists": exists}, nil
}

// handleListZones lists all active zones.
func (r *Runner) handleListZones(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)

	zones := make([]map[string]any, 0, len(zs.zones))
	for _, id := range zs.zoneOrder {
		if z, ok := zs.zones[id]; ok {
			zones = append(zones, map[string]any{
				"zone_id":   z.ZoneID,
				"zone_type": z.ZoneType,
				"connected": z.Connected,
			})
		}
	}

	return map[string]any{
		"zones":      zones,
		"zone_count": len(zones),
	}, nil
}

// handleZoneCount returns the number of active zones.
func (r *Runner) handleZoneCount(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)
	return map[string]any{"count": len(zs.zones)}, nil
}

// handleGetZoneMetadata returns zone metadata.
func (r *Runner) handleGetZoneMetadata(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"metadata": nil}, nil
	}

	return map[string]any{"metadata": zone.Metadata}, nil
}

// handleGetZoneCAFingerprint returns the Zone CA fingerprint.
func (r *Runner) handleGetZoneCAFingerprint(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"fingerprint": ""}, nil
	}

	return map[string]any{"fingerprint": zone.CAFingerprint}, nil
}

// handleVerifyZoneCA verifies a Zone CA is valid.
func (r *Runner) handleVerifyZoneCA(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"ca_valid": false}, nil
	}

	return map[string]any{
		"ca_valid":    zone.CAFingerprint != "",
		"fingerprint": zone.CAFingerprint,
	}, nil
}

// handleVerifyZoneBinding verifies a device is bound to a zone.
func (r *Runner) handleVerifyZoneBinding(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)
	deviceID, _ := params["device_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"binding_valid": false}, nil
	}

	found := false
	for _, d := range zone.DeviceIDs {
		if d == deviceID {
			found = true
			break
		}
	}

	return map[string]any{"binding_valid": found}, nil
}

// handleVerifyZoneIDDerivation verifies zone ID derivation from cert.
func (r *Runner) handleVerifyZoneIDDerivation(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	zoneID, _ := params["zone_id"].(string)

	// Zone IDs are 16 hex chars (64 bits of SHA-256).
	valid := len(zoneID) == 16 && isHex(zoneID)

	return map[string]any{"derivation_valid": valid}, nil
}

// handleHighestPriorityZone returns the zone with the highest priority.
func (r *Runner) handleHighestPriorityZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)

	if len(zs.zones) == 0 {
		return map[string]any{"zone_id": "", "zone_type": ""}, nil
	}

	var best *zoneInfo
	for _, z := range zs.zones {
		if best == nil || z.Priority > best.Priority {
			best = z
		}
	}

	return map[string]any{
		"zone_id":   best.ZoneID,
		"zone_type": best.ZoneType,
	}, nil
}

// handleHighestPriorityConnectedZone returns the highest priority connected zone.
func (r *Runner) handleHighestPriorityConnectedZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)

	var best *zoneInfo
	for _, z := range zs.zones {
		if z.Connected && (best == nil || z.Priority > best.Priority) {
			best = z
		}
	}

	if best == nil {
		return map[string]any{"zone_id": "", "zone_type": ""}, nil
	}

	return map[string]any{
		"zone_id":   best.ZoneID,
		"zone_type": best.ZoneType,
	}, nil
}

// handleDisconnectZone disconnects a specific zone.
func (r *Runner) handleDisconnectZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"zone_disconnected": false}, nil
	}

	zone.Connected = false

	// Also close any tracked connection.
	ct := getConnectionTracker(state)
	if conn, ok := ct.zoneConnections[zoneID]; ok {
		if conn.connected {
			_ = conn.Close()
		}
		delete(ct.zoneConnections, zoneID)
	}

	return map[string]any{"zone_disconnected": true}, nil
}

// handleVerifyOtherZone verifies another zone's state during multi-zone tests.
func (r *Runner) handleVerifyOtherZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params["zone_id"].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{"zone_exists": false}, nil
	}

	return map[string]any{
		"zone_exists": true,
		"zone_type":   zone.ZoneType,
		"connected":   zone.Connected,
	}, nil
}

// handleVerifyBidirectionalActive verifies bidirectional communication is active.
func (r *Runner) handleVerifyBidirectionalActive(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check connection is alive.
	active := r.conn != nil && r.conn.connected

	return map[string]any{
		"bidirectional_active": active,
	}, nil
}

// handleVerifyRestoreSequence verifies restore sequence after reconnection.
func (r *Runner) handleVerifyRestoreSequence(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Verify connection is re-established.
	restored := r.conn != nil && r.conn.connected

	return map[string]any{
		"sequence_restored": restored,
	}, nil
}

// handleVerifyTLSState verifies TLS connection state.
func (r *Runner) handleVerifyTLSState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{
			"tls_active":  false,
			"tls_version": 0,
		}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	expectedVersion, _ := params["expected_version"].(float64)

	versionMatch := true
	if expectedVersion > 0 {
		versionMatch = float64(tlsState.Version) == expectedVersion
	}

	return map[string]any{
		"tls_active":          true,
		"tls_version":         int(tlsState.Version),
		"negotiated_protocol": tlsState.NegotiatedProtocol,
		"version_matches":     versionMatch,
	}, nil
}

// generateZoneID generates a random 16-character hex zone ID.
func generateZoneID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateFingerprint generates a SHA-256 fingerprint from input.
func generateFingerprint(input string) string {
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16])
}

// isHex checks if a string is valid hexadecimal.
func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

// sortedZonesByPriority returns zones sorted by priority (highest first).
func sortedZonesByPriority(zs *zoneState) []*zoneInfo {
	zones := make([]*zoneInfo, 0, len(zs.zones))
	for _, z := range zs.zones {
		zones = append(zones, z)
	}
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Priority > zones[j].Priority
	})
	return zones
}

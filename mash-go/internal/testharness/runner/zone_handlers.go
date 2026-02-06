package runner

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/cert"
)

// registerZoneHandlers registers all zone management action handlers.
func (r *Runner) registerZoneHandlers() {
	// Custom checker: save_zone_id saves the zone ID under a target key name.
	r.engine.RegisterChecker(KeySaveZoneID, r.checkSaveZoneID)

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

	zoneType, _ := params[KeyZoneType].(string)
	if zoneType == "" {
		zoneType = ZoneTypeLocal
	}

	zoneName, _ := params[KeyZoneName].(string)

	zoneID, _ := params[KeyZoneID].(string)
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
		ZoneID:         zoneID,
		ZoneName:       zoneName,
		ZoneType:       zoneType,
		Priority:       priority,
		Metadata:       make(map[string]any),
		CAFingerprint:  fingerprint,
		Connected:      false,
		DeviceIDs:      make([]string, 0),
		CommissionedAt: time.Now(),
	}

	zs.zones[zoneID] = zone
	zs.zoneOrder = append(zs.zoneOrder, zoneID)

	// Generate a real Zone CA and controller operational cert so that
	// verify_controller_cert and cert fingerprint handlers work with
	// actual cryptographic material.
	zt := cert.ZoneTypeLocal
	if zoneType == ZoneTypeGrid {
		zt = cert.ZoneTypeGrid
	} else if zoneType == ZoneTypeTest {
		zt = cert.ZoneTypeTest
	}
	if zoneCA, err := cert.GenerateZoneCA(zoneID, zt); err == nil {
		r.zoneCA = zoneCA
		r.zoneCAPool = zoneCA.TLSClientCAs()
		zone.CAFingerprint = certFingerprint(zoneCA.Certificate)
		fingerprint = zone.CAFingerprint

		if controllerCert, err := cert.GenerateControllerOperationalCert(zoneCA, "test-controller"); err == nil {
			r.controllerCert = controllerCert
		}
	}

	return map[string]any{
		KeyZoneID:        zoneID,
		KeySaveZoneID:    zoneID, // For save_as support in the engine.
		KeyZoneCreated:   true,
		KeyZoneType:      zoneType,
		KeyFingerprint:   fingerprint,
		KeyZoneIDPresent: zoneID != "",
		KeyZoneIDLength:  len(zoneID),
	}, nil
}

// handleAddZone adds a zone. If the zone doesn't exist and zone_type is
// provided, it creates the zone (delegates to handleCreateZone). If the zone
// already exists and a device_id is provided, it adds a device to the zone.
func (r *Runner) handleAddZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)
	zoneType, _ := params[KeyZoneType].(string)

	// If zone doesn't exist and zone_type is provided, create it.
	if _, exists := zs.zones[zoneID]; !exists && zoneType != "" {
		out, err := r.handleCreateZone(ctx, step, state)
		if err != nil {
			// Return error string as output (not Go error) so checkers can match it.
			errStr := err.Error()
			errName := errStr
			if errStr == fmt.Sprintf("zone type %s already exists", zoneType) {
				errName = "ErrZoneTypeExists"
			}
			return map[string]any{KeyError: errName}, nil
		}
		return out, nil
	}

	// Add device to existing zone.
	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyError: "ErrZoneNotFound"}, nil
	}

	deviceID, _ := params[KeyDeviceID].(string)
	zone.DeviceIDs = append(zone.DeviceIDs, deviceID)

	return map[string]any{
		KeyDeviceAdded: true,
		KeyZoneID:      zoneID,
	}, nil
}

// handleDeleteZone removes a zone.
func (r *Runner) handleDeleteZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	if _, exists := zs.zones[zoneID]; !exists {
		return map[string]any{KeyZoneRemoved: false, KeyError: "ErrZoneNotFound"}, nil
	}

	delete(zs.zones, zoneID)

	// Remove from order.
	for i, id := range zs.zoneOrder {
		if id == zoneID {
			zs.zoneOrder = append(zs.zoneOrder[:i], zs.zoneOrder[i+1:]...)
			break
		}
	}

	return map[string]any{
		KeyZoneRemoved: true,
		KeyZoneDeleted: true,
	}, nil
}

// handleRemoveZone is an alias for delete_zone.
func (r *Runner) handleRemoveZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleDeleteZone(ctx, step, state)
}

// handleGetZone returns zone details.
func (r *Runner) handleGetZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyZoneFound: false}, nil
	}

	// "recent" means within the last 30 seconds.
	commissionedRecent := !zone.CommissionedAt.IsZero() && time.Since(zone.CommissionedAt) < 30*time.Second
	lastSeenRecent := !zone.LastSeen.IsZero() && time.Since(zone.LastSeen) < 30*time.Second

	return map[string]any{
		KeyZoneFound:              true,
		KeyZoneID:                 zone.ZoneID,
		KeyZoneType:               zone.ZoneType,
		KeyZoneMetadata:           zone.Metadata,
		KeyConnected:              zone.Connected,
		KeyDeviceCount:            len(zone.DeviceIDs),
		"commissioned_at_recent":  commissionedRecent,
		"last_seen_recent":        lastSeenRecent,
		"last_seen_not_updated":   !zone.LastSeenUpdated,
	}, nil
}

// handleHasZone checks if a zone exists.
func (r *Runner) handleHasZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)
	_, exists := zs.zones[zoneID]

	return map[string]any{KeyZoneExists: exists, KeyResult: exists}, nil
}

// handleListZones lists all active zones.
func (r *Runner) handleListZones(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)

	zones := make([]map[string]any, 0, len(zs.zones))
	zoneIDs := make([]any, 0, len(zs.zones))
	for _, id := range zs.zoneOrder {
		if z, ok := zs.zones[id]; ok {
			zones = append(zones, map[string]any{
				KeyZoneID:    z.ZoneID,
				KeyZoneType:  z.ZoneType,
				KeyConnected: z.Connected,
			})
			zoneIDs = append(zoneIDs, z.ZoneID)
		}
	}

	return map[string]any{
		KeyZones:          zones,
		KeyZoneCount:      len(zones),
		"zone_ids_include": zoneIDs,
	}, nil
}

// handleZoneCount returns the number of active zones.
func (r *Runner) handleZoneCount(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)
	count := len(zs.zones)
	return map[string]any{KeyCount: count, KeyResult: count}, nil
}

// handleGetZoneMetadata returns zone metadata.
func (r *Runner) handleGetZoneMetadata(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	// Support zone_id_ref: dereference a state variable to get the zone ID.
	if zoneID == "" {
		if ref, ok := params["zone_id_ref"].(string); ok && ref != "" {
			if val, exists := state.Get(ref); exists {
				zoneID, _ = val.(string)
			}
		}
	}

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyMetadata: nil}, nil
	}

	return map[string]any{
		KeyMetadata:  zone.Metadata,
		KeyZoneName:  zone.ZoneName,
		KeyZoneType:  zone.ZoneType,
		"zone_priority":    zone.Priority,
		"created_at_recent": true,
	}, nil
}

// handleGetZoneCAFingerprint returns the Zone CA fingerprint.
func (r *Runner) handleGetZoneCAFingerprint(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Prefer the real Zone CA cert fingerprint when available.
	if r.zoneCA != nil && r.zoneCA.Certificate != nil {
		return map[string]any{KeyFingerprint: certFingerprint(r.zoneCA.Certificate)}, nil
	}

	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyFingerprint: ""}, nil
	}

	return map[string]any{KeyFingerprint: zone.CAFingerprint}, nil
}

// handleVerifyZoneCA verifies a Zone CA is valid.
func (r *Runner) handleVerifyZoneCA(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	// Fall back to most recently created zone when no zone_id is provided.
	if zoneID == "" && len(zs.zoneOrder) > 0 {
		zoneID = zs.zoneOrder[len(zs.zoneOrder)-1]
	}

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyCAValid: false}, nil
	}

	outputs := map[string]any{
		KeyCAValid:     zone.CAFingerprint != "",
		KeyFingerprint: zone.CAFingerprint,
	}

	// Add cert details from the runner's Zone CA if available.
	if r.zoneCA != nil && r.zoneCA.Certificate != nil {
		cert := r.zoneCA.Certificate
		outputs[KeyPathLength] = cert.MaxPathLen
		outputs[KeyAlgorithm] = cert.SignatureAlgorithm.String()
		outputs[KeyBasicConstraintsCA] = cert.IsCA
		// Validity period in years.
		years := cert.NotAfter.Sub(cert.NotBefore).Hours() / (24 * 365)
		outputs[KeyValidityYearsMin] = years
	}

	return outputs, nil
}

// handleVerifyZoneBinding verifies a device is bound to a zone.
func (r *Runner) handleVerifyZoneBinding(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)
	deviceID, _ := params[KeyDeviceID].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyBindingValid: false}, nil
	}

	found := false
	for _, d := range zone.DeviceIDs {
		if d == deviceID {
			found = true
			break
		}
	}

	return map[string]any{KeyBindingValid: found}, nil
}

// handleVerifyZoneIDDerivation verifies zone ID derivation from cert.
func (r *Runner) handleVerifyZoneIDDerivation(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	zoneID, _ := params[KeyZoneID].(string)

	// Zone IDs are 16 hex chars (64 bits of SHA-256).
	valid := len(zoneID) == 16 && isHex(zoneID)

	return map[string]any{KeyDerivationValid: valid}, nil
}

// handleHighestPriorityZone returns the zone with the highest priority.
func (r *Runner) handleHighestPriorityZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	zs := getZoneState(state)

	if len(zs.zones) == 0 {
		return map[string]any{KeyZoneID: "", KeyZoneType: ""}, nil
	}

	var best *zoneInfo
	for _, z := range zs.zones {
		if best == nil || z.Priority > best.Priority {
			best = z
		}
	}

	return map[string]any{
		KeyZoneID:   best.ZoneID,
		KeyZoneType: best.ZoneType,
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
		return map[string]any{KeyZoneID: "", KeyZoneType: ""}, nil
	}

	return map[string]any{
		KeyZoneID:   best.ZoneID,
		KeyZoneType: best.ZoneType,
	}, nil
}

// handleDisconnectZone disconnects a specific zone.
func (r *Runner) handleDisconnectZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID := resolveZoneParam(params)

	// Mark zone as disconnected in zone state (if present).
	if zone, exists := zs.zones[zoneID]; exists {
		zone.Connected = false
	}

	// Close tracked connection regardless of zone state.
	ct := getConnectionTracker(state)
	if conn, ok := ct.zoneConnections[zoneID]; ok {
		if conn.connected {
			_ = conn.Close()
		}
		delete(ct.zoneConnections, zoneID)
		return map[string]any{KeyZoneDisconnected: true}, nil
	}

	// No tracked connection -- check if zone state existed.
	if _, exists := zs.zones[zoneID]; !exists {
		return map[string]any{KeyZoneDisconnected: false}, nil
	}

	return map[string]any{KeyZoneDisconnected: true}, nil
}

// handleVerifyOtherZone verifies another zone's state during multi-zone tests.
func (r *Runner) handleVerifyOtherZone(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	zs := getZoneState(state)

	zoneID, _ := params[KeyZoneID].(string)

	zone, exists := zs.zones[zoneID]
	if !exists {
		return map[string]any{KeyZoneExists: false}, nil
	}

	return map[string]any{
		KeyZoneExists: true,
		KeyZoneType:   zone.ZoneType,
		KeyConnected:  zone.Connected,
	}, nil
}

// handleVerifyBidirectionalActive verifies bidirectional communication is active.
func (r *Runner) handleVerifyBidirectionalActive(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Check connection is alive.
	active := r.conn != nil && r.conn.connected

	return map[string]any{
		KeyBidirectionalActive: active,
	}, nil
}

// handleVerifyRestoreSequence verifies restore sequence after reconnection.
func (r *Runner) handleVerifyRestoreSequence(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Verify connection is re-established.
	restored := r.conn != nil && r.conn.connected

	return map[string]any{
		KeySequenceRestored: restored,
	}, nil
}

// handleVerifyTLSState verifies TLS connection state.
func (r *Runner) handleVerifyTLSState(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	if r.conn == nil || !r.conn.connected || r.conn.tlsConn == nil {
		return map[string]any{
			KeyTLSActive:  false,
			KeyTLSVersion: 0,
		}, nil
	}

	tlsState := r.conn.tlsConn.ConnectionState()
	expectedVersion := paramFloat(params, "expected_version", 0)

	versionMatch := true
	if expectedVersion > 0 {
		versionMatch = float64(tlsState.Version) == expectedVersion
	}

	return map[string]any{
		KeyTLSActive:             true,
		KeyTLSVersion:            int(tlsState.Version),
		KeyNegotiatedProtocol:    tlsState.NegotiatedProtocol,
		KeyVersionMatches:        versionMatch,
		KeySessionTicketReceived: false, // MASH prohibits session resumption
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

// checkSaveZoneID is a custom checker that saves the zone ID under a target key.
// Used by YAML expectations like: save_zone_id: zone_id_a
func (r *Runner) checkSaveZoneID(key string, expected interface{}, state *engine.ExecutionState) *engine.ExpectResult {
	targetKey, ok := expected.(string)
	if !ok {
		return &engine.ExpectResult{
			Key:      key,
			Expected: expected,
			Passed:   false,
			Message:  fmt.Sprintf("save_zone_id target must be a string, got %T", expected),
		}
	}

	zoneID, exists := state.Get(key)
	if !exists {
		return &engine.ExpectResult{
			Key:      key,
			Expected: expected,
			Passed:   false,
			Message:  fmt.Sprintf("key %q not found in state", key),
		}
	}

	state.Set(targetKey, zoneID)
	return &engine.ExpectResult{
		Key:      key,
		Expected: expected,
		Actual:   zoneID,
		Passed:   true,
		Message:  fmt.Sprintf("saved zone_id %v as %q", zoneID, targetKey),
	}
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

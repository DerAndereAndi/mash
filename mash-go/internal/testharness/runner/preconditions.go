package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Precondition levels form a hierarchy:
//
//	Level 0: No relevant preconditions     -> no-op
//	Level 1: device_in_commissioning_mode  -> ensure disconnected (clean state)
//	Level 2: tls/connection_established    -> connect
//	Level 3: session/device_commissioned   -> connect + commission (PASE)
const (
	precondLevelNone         = 0
	precondLevelCommissioning = 1
	precondLevelConnected    = 2
	precondLevelCommissioned = 3
)

// preconditionKeyLevels maps known precondition keys to their required level.
var preconditionKeyLevels = map[string]int{
	// Level 0: Always-true environment preconditions (no setup needed).
	"device_booted":       precondLevelNone,
	"controller_running":  precondLevelNone,
	"device_in_network":   precondLevelNone,

	"device_in_commissioning_mode": precondLevelCommissioning,
	"device_uncommissioned":        precondLevelCommissioning,
	"commissioning_window_open":    precondLevelCommissioning,
	"tls_connection_established":   precondLevelConnected,
	"connection_established":       precondLevelConnected,
	"device_commissioned":          precondLevelCommissioned,
	"session_established":          precondLevelCommissioned,
}

// preconditionLevelFor determines the highest setup level needed for the given conditions.
func preconditionLevelFor(conditions []loader.Condition) int {
	level := precondLevelNone
	for _, cond := range conditions {
		for key, val := range cond {
			// Only consider conditions set to true.
			if b, ok := val.(bool); !ok || !b {
				continue
			}
			if l, ok := preconditionKeyLevels[key]; ok && l > level {
				level = l
			}
		}
	}
	return level
}

// preconditionLevel determines the highest setup level needed for the given conditions.
func (r *Runner) preconditionLevel(conditions []loader.Condition) int {
	return preconditionLevelFor(conditions)
}

// SortByPreconditionLevel sorts test cases by their required precondition level
// (lowest first). The sort is stable, preserving file order within the same level.
func SortByPreconditionLevel(cases []*loader.TestCase) {
	sort.SliceStable(cases, func(i, j int) bool {
		return preconditionLevelFor(cases[i].Preconditions) <
			preconditionLevelFor(cases[j].Preconditions)
	})
}

// currentLevel returns the runner's current precondition level based on connection
// and commissioning state.
func (r *Runner) currentLevel() int {
	if r.paseState != nil && r.paseState.completed {
		return precondLevelCommissioned
	}
	if r.conn != nil && r.conn.connected {
		return precondLevelConnected
	}
	return precondLevelNone
}

// setupPreconditions is the callback registered with the engine.
// It inspects tc.Preconditions and ensures the runner is in the right state.
// When transitioning backwards (e.g., from commissioned to commissioning),
// it disconnects to give the device a clean state.
func (r *Runner) setupPreconditions(ctx context.Context, tc *loader.TestCase, state *engine.ExecutionState) error {
	// Populate setup_code so that test steps using ${setup_code} resolve correctly.
	if r.config.SetupCode != "" {
		state.Set("setup_code", r.config.SetupCode)
	}

	needed := r.preconditionLevel(tc.Preconditions)
	current := r.currentLevel()

	// DEC-059: On backward transition from commissioned, send RemoveZone
	// so the device re-enters commissioning mode before we disconnect.
	if current >= precondLevelCommissioned && needed <= precondLevelCommissioning {
		r.sendRemoveZone()
	}

	// Backwards transition: disconnect to give the device a clean state.
	// This handles cases like a commissioned runner needing to drop back
	// for a commissioning-level test.
	if needed < current && needed <= precondLevelCommissioning {
		r.ensureDisconnected()
	}

	switch needed {
	case precondLevelCommissioned:
		return r.ensureCommissioned(ctx, state)
	case precondLevelConnected:
		// If currently commissioned but only a connection is needed,
		// disconnect and reconnect for a clean TLS session.
		if current > precondLevelConnected {
			r.ensureDisconnected()
		}
		return r.ensureConnected(ctx, state)
	case precondLevelCommissioning:
		r.ensureDisconnected()
		return nil
	default:
		return nil
	}
}

// ensureConnected checks if already connected; if not, establishes a commissioning TLS connection.
func (r *Runner) ensureConnected(ctx context.Context, state *engine.ExecutionState) error {
	if r.conn != nil && r.conn.connected {
		return nil
	}

	// Create a synthetic step to drive handleConnect.
	step := &loader.Step{
		Action: "connect",
		Params: map[string]any{
			"commissioning": true,
		},
	}

	outputs, err := r.handleConnect(ctx, step, state)
	if err != nil {
		return fmt.Errorf("precondition connect failed: %w", err)
	}

	// handleConnect returns connection_established in outputs even on TLS failure.
	if established, ok := outputs["connection_established"].(bool); ok && !established {
		errMsg, _ := outputs["error"].(string)
		return fmt.Errorf("precondition connect failed: %s", errMsg)
	}

	return nil
}

// ensureCommissioned checks if already commissioned; if not, connects and performs PASE.
func (r *Runner) ensureCommissioned(ctx context.Context, state *engine.ExecutionState) error {
	// First ensure we're connected.
	if err := r.ensureConnected(ctx, state); err != nil {
		return err
	}

	// If already commissioned, populate state and return.
	if r.paseState != nil && r.paseState.completed {
		state.Set("session_established", true)
		state.Set("connection_established", true)
		if r.paseState.sessionKey != nil {
			state.Set("session_key", r.paseState.sessionKey)
			state.Set("session_key_length", len(r.paseState.sessionKey))
		}
		return nil
	}

	// Create a synthetic step to drive handleCommission.
	step := &loader.Step{
		Action: "commission",
		Params: map[string]any{},
	}

	// Pass setup_code from config if available.
	if r.config.SetupCode != "" {
		step.Params["setup_code"] = r.config.SetupCode
	}

	_, err := r.handleCommission(ctx, step, state)
	if err != nil {
		// On transient errors (EOF, connection reset, broken pipe), retry once
		// after a short delay. The device may still be cycling its commissioning
		// window in test mode.
		if isTransientError(err) {
			r.ensureDisconnected()
			time.Sleep(1 * time.Second)
			if connErr := r.ensureConnected(ctx, state); connErr != nil {
				return fmt.Errorf("precondition commission retry connect failed: %w", connErr)
			}
			_, err = r.handleCommission(ctx, step, state)
			if err != nil {
				return fmt.Errorf("precondition commission retry failed: %w", err)
			}
			return nil
		}
		return fmt.Errorf("precondition commission failed: %w", err)
	}

	return nil
}

// isTransientError returns true for IO-level errors that may resolve on retry.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		isNetError(err)
}

// isNetError checks if the error is a network-level error.
func isNetError(err error) bool {
	var netErr *net.OpError
	return errors.As(err, &netErr)
}

// ensureDisconnected closes any existing connection for a clean start.
func (r *Runner) ensureDisconnected() {
	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}
	r.paseState = nil
}

// sendRemoveZone sends a RemoveZone invoke to the device so it re-enters
// commissioning mode (DEC-059). Errors are ignored because the device may
// have already closed the connection.
func (r *Runner) sendRemoveZone() {
	if r.conn == nil || !r.conn.connected || r.conn.framer == nil {
		return
	}
	if r.paseState == nil || !r.paseState.completed || r.paseState.sessionKey == nil {
		return
	}

	// Derive zone ID from shared secret (same derivation as device).
	zoneID := deriveZoneIDFromSecret(r.paseState.sessionKey)

	// Build RemoveZone invoke: endpoint 0, DeviceInfo feature (1), command 0x10.
	req := &wire.Request{
		MessageID:  atomic.AddUint32(&r.messageID, 1),
		Operation:  wire.OpInvoke,
		EndpointID: 0,
		FeatureID:  1, // DeviceInfo
		Payload: &wire.InvokePayload{
			CommandID:  16, // DeviceInfoCmdRemoveZone
			Parameters: map[string]any{"zoneId": zoneID},
		},
	}

	data, err := wire.EncodeRequest(req)
	if err != nil {
		return
	}

	// Best-effort: send and read response, ignoring errors.
	_, _ = r.sendRequest(data, "remove-zone")
}

// deriveZoneIDFromSecret derives a zone ID from a PASE shared secret
// using the same SHA-256 derivation as the device side.
func deriveZoneIDFromSecret(secret []byte) string {
	hash := sha256.Sum256(secret)
	return hex.EncodeToString(hash[:8])
}

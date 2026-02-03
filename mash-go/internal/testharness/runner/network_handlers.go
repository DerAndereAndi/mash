package runner

import (
	"context"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
)

// registerNetworkHandlers registers all network simulation action handlers.
func (r *Runner) registerNetworkHandlers() {
	r.engine.RegisterHandler("network_partition", r.handleNetworkPartition)
	r.engine.RegisterHandler("network_filter", r.handleNetworkFilter)
	r.engine.RegisterHandler("interface_down", r.handleInterfaceDown)
	r.engine.RegisterHandler("interface_up", r.handleInterfaceUp)
	r.engine.RegisterHandler("interface_flap", r.handleInterfaceFlap)
	r.engine.RegisterHandler("change_address", r.handleChangeAddress)
	r.engine.RegisterHandler("check_display", r.handleCheckDisplay)
	r.engine.RegisterHandler("adjust_clock", r.handleAdjustClock)
}

// handleNetworkPartition simulates a network partition.
func (r *Runner) handleNetworkPartition(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	zoneID, _ := params[KeyZoneID].(string)

	// Simulate by closing the connection.
	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}

	// Also close zone-specific connections.
	ct := getConnectionTracker(state)
	if zoneID != "" {
		if conn, ok := ct.zoneConnections[zoneID]; ok && conn.connected {
			_ = conn.Close()
			delete(ct.zoneConnections, zoneID)
		}
	}

	state.Set(StateNetworkPartitioned, true)

	return map[string]any{
		KeyPartitionActive: true,
		KeyZoneID:          zoneID,
	}, nil
}

// handleNetworkFilter filters specific traffic patterns.
func (r *Runner) handleNetworkFilter(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	filterType, _ := params[KeyFilterType].(string)

	state.Set(StateNetworkFilter, filterType)

	return map[string]any{
		KeyFilterActive: true,
		KeyFilterType:   filterType,
	}, nil
}

// handleInterfaceDown simulates network interface going down.
func (r *Runner) handleInterfaceDown(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	// Simulate by closing all connections.
	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}

	ct := getConnectionTracker(state)
	for id, conn := range ct.zoneConnections {
		if conn.connected {
			_ = conn.Close()
		}
		delete(ct.zoneConnections, id)
	}

	state.Set(StateInterfaceUp, false)

	return map[string]any{
		KeyInterfaceDown: true,
	}, nil
}

// handleInterfaceUp simulates network interface coming up.
func (r *Runner) handleInterfaceUp(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	state.Set(StateInterfaceUp, true)

	return map[string]any{
		KeyInterfaceUp: true,
	}, nil
}

// handleInterfaceFlap simulates network interface flapping.
func (r *Runner) handleInterfaceFlap(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := 3
	if c, ok := params[KeyCount].(float64); ok {
		count = int(c)
	}
	intervalMs := 100
	if i, ok := params["interval_ms"].(float64); ok {
		intervalMs = int(i)
	}

	for i := 0; i < count; i++ {
		// Down.
		if r.conn != nil && r.conn.connected {
			_ = r.conn.Close()
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(intervalMs) * time.Millisecond):
		}

		// Up (just mark as up, actual reconnect is separate).
		state.Set(StateInterfaceUp, true)
	}

	return map[string]any{
		KeyFlapCount: count,
	}, nil
}

// handleChangeAddress simulates device IP address change.
func (r *Runner) handleChangeAddress(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	newAddress, _ := params[KeyNewAddress].(string)
	state.Set(StateDeviceAddress, newAddress)

	// Close existing connections (address changed).
	if r.conn != nil && r.conn.connected {
		_ = r.conn.Close()
	}

	return map[string]any{
		KeyAddressChanged: true,
		KeyNewAddress:     newAddress,
	}, nil
}

// handleCheckDisplay verifies QR code display (simulated).
func (r *Runner) handleCheckDisplay(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDiscoveryState(state)

	hasQR := ds.qrPayload != ""

	return map[string]any{
		KeyDisplayChecked: true,
		KeyQRDisplayed:    hasQR,
		KeyQRPayload:      ds.qrPayload,
	}, nil
}

// handleAdjustClock simulates clock skew.
func (r *Runner) handleAdjustClock(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ct := getConnectionTracker(state)

	offsetMs := int64(0)
	if o, ok := params[KeyOffsetMs].(float64); ok {
		offsetMs = int64(o)
	}
	if d, ok := params["offset_days"].(float64); ok {
		offsetMs = int64(d) * 24 * 60 * 60 * 1000
	}

	ct.clockOffset = time.Duration(offsetMs) * time.Millisecond
	state.Set(StateClockOffsetMs, offsetMs)

	return map[string]any{
		KeyClockAdjusted: true,
		KeyOffsetMs:      offsetMs,
	}, nil
}

package runner

import (
	"context"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/features"
)

// registerNetworkHandlers registers all network simulation action handlers.
func (r *Runner) registerNetworkHandlers() {
	r.engine.RegisterHandler(ActionNetworkPartition, r.handleNetworkPartition)
	r.engine.RegisterHandler(ActionNetworkFilter, r.handleNetworkFilter)
	r.engine.RegisterHandler(ActionInterfaceDown, r.handleInterfaceDown)
	r.engine.RegisterHandler(ActionInterfaceUp, r.handleInterfaceUp)
	r.engine.RegisterHandler(ActionInterfaceFlap, r.handleInterfaceFlap)
	r.engine.RegisterHandler(ActionChangeAddress, r.handleChangeAddress)
	r.engine.RegisterHandler(ActionCheckDisplay, r.handleCheckDisplay)
	r.engine.RegisterHandler(ActionAdjustClock, r.handleAdjustClock)
}

// handleNetworkPartition simulates a network partition.
func (r *Runner) handleNetworkPartition(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	zoneID, _ := params[KeyZoneID].(string)
	block := true // default: block=true
	if v, ok := params[ParamBlock]; ok {
		block = toBool(v)
	}

	if block {
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
	} else {
		// Restore: clear partition state so subsequent connects succeed.
		state.Set(StateNetworkPartitioned, false)
	}

	return map[string]any{
		KeyPartitionActive: block,
		KeyZoneID:          zoneID,
	}, nil
}

// handleNetworkFilter filters specific traffic patterns.
func (r *Runner) handleNetworkFilter(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	filterType, _ := params[KeyFilterType].(string)
	if filterType == "" && toBool(params[ParamBlockPongs]) {
		filterType = NetworkFilterBlockPongs
	}

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

	// Record the new address as an injected announcement. buildBrowseOutput
	// merges injected addresses into browse results, so this works in both
	// simulation (services already in ds.services) and live mode (services
	// populated by real mDNS). The injection side-channel survives real mDNS
	// overwriting ds.services.
	ds := getDiscoveryState(state)
	ds.injectedAddresses = append(ds.injectedAddresses, "fd34:5678:abcd::1")

	return map[string]any{
		KeyInterfaceUp: true,
	}, nil
}

// handleInterfaceFlap simulates network interface flapping.
func (r *Runner) handleInterfaceFlap(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	count := paramInt(params, KeyCount, 3)
	intervalMs := paramInt(params, ParamIntervalMs, 100)

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

	offsetMs := int64(paramInt(params, KeyOffsetMs, 0))
	if s := paramInt(params, ParamOffsetSeconds, 0); s != 0 {
		offsetMs = int64(s) * 1000
	}
	if d := paramInt(params, ParamOffsetDays, 0); d > 0 {
		offsetMs = int64(d) * 24 * 60 * 60 * 1000
	}

	ct.clockOffset = time.Duration(offsetMs) * time.Millisecond
	state.Set(StateClockOffsetMs, offsetMs)

	// Send clock offset trigger to real device so it adjusts cert validation time.
	if r.config.Target != "" {
		offsetSec := int32(offsetMs / 1000)
		trigger := features.TriggerAdjustClockBase | uint64(uint32(offsetSec))
		if trigErr := r.sendTriggerViaZone(ctx, trigger, state); trigErr != nil {
			r.debugf("adjust_clock: failed to send trigger to device: %v", trigErr)
			// Don't fail the handler -- the local offset is still useful.
		}
	}

	return map[string]any{
		KeyClockAdjusted: true,
		KeyOffsetMs:      offsetMs,
	}, nil
}

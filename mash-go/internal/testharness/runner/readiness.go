package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

const healthProbeWriteTimeout = 500 * time.Millisecond

func waitForOperationalReadyOnConn(
	conn *Connection,
	timeout time.Duration,
	nextMsgID func() uint32,
	appendNotification func([]byte),
	debugf func(string, ...any),
) error {
	if conn == nil || !conn.isConnected() {
		return fmt.Errorf("not connected")
	}
	if conn.framer == nil {
		return fmt.Errorf("not connected")
	}

	req := &wire.Request{
		MessageID:  nextMsgID(),
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0x01, // FeatureDeviceInfo
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode readiness probe: %w", err)
	}

	if err := conn.framer.WriteFrame(data); err != nil {
		conn.transitionTo(ConnDisconnected)
		return fmt.Errorf("send readiness probe: %w", err)
	}

	if conn.tlsConn != nil {
		_ = conn.tlsConn.SetReadDeadline(time.Now().Add(timeout))
		defer func() { _ = conn.tlsConn.SetReadDeadline(time.Time{}) }()
	} else if conn.conn != nil {
		_ = conn.conn.SetReadDeadline(time.Now().Add(timeout))
		defer func() { _ = conn.conn.SetReadDeadline(time.Time{}) }()
	}

	for range 10 {
		respData, err := conn.framer.ReadFrame()
		if err != nil {
			conn.transitionTo(ConnDisconnected)
			return fmt.Errorf("read readiness response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode readiness response: %w", err)
		}
		if resp.MessageID == 0 {
			if appendNotification != nil {
				appendNotification(respData)
			}
			continue
		}
		if resp.MessageID != req.MessageID {
			if debugf != nil {
				debugf("waitForOperationalReady: discarding orphaned response (got msgID=%d, want %d)", resp.MessageID, req.MessageID)
			}
			continue
		}
		if debugf != nil {
			debugf("waitForOperationalReady: device responded (status=%d)", resp.Status)
		}
		return nil
	}
	return fmt.Errorf("readiness probe: too many interleaved frames")
}

// waitForCommissioningMode uses the persistent mDNS observer to wait until
// the device advertises the commissionable service (_mash-comm._tcp),
// indicating it has re-entered commissioning mode.
func (r *Runner) waitForCommissioningMode(ctx context.Context, timeout time.Duration) error {
	start := time.Now()
	obs := r.getOrCreateObserver()
	if obs == nil {
		return fmt.Errorf("failed to create mDNS observer")
	}

	// Clear stale commissionable entries so we only match fresh advertisements.
	// Without this, the observer may still hold an entry from a previous session
	// that would immediately satisfy the predicate.
	obs.ClearSnapshot("commissionable")

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := obs.WaitFor(waitCtx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) > 0
	})
	if err != nil {
		return fmt.Errorf("timeout waiting for commissioning mode after %v", timeout)
	}
	r.debugf("waitForCommissioningMode: device found after %v", time.Since(start))
	return nil
}

// waitForCommissioningAvailable waits until a commissionable advertisement is
// present in the observer snapshot. Unlike waitForCommissioningMode, it does
// not clear the snapshot first, so it can return immediately when the device
// is already advertising.
func (r *Runner) waitForCommissioningAvailable(ctx context.Context, timeout time.Duration) error {
	start := time.Now()
	obs := r.getOrCreateObserver()
	if obs == nil {
		return fmt.Errorf("failed to create mDNS observer")
	}

	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := obs.WaitFor(waitCtx, "commissionable", func(svcs []discoveredService) bool {
		return len(svcs) > 0
	})
	if err != nil {
		return fmt.Errorf("timeout waiting for commissioning availability after %v", timeout)
	}
	r.debugf("waitForCommissioningAvailable: device found after %v", time.Since(start))
	return nil
}

// probeSessionHealth sends a lightweight Read request to DeviceInfo (endpoint 0,
// feature 0x01) to verify the connection is still alive and the device is
// responding. Returns nil if the session is healthy, an error otherwise.
// Used by setupPreconditions to detect corrupted sessions before reuse.
func (r *Runner) probeSessionHealth() error {
	if r.pool.Main() == nil || !r.pool.Main().isConnected() || r.pool.Main().framer == nil {
		return fmt.Errorf("no active connection")
	}

	// Read DeviceInfo (always present on endpoint 0).
	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpRead,
		EndpointID: 0,
		FeatureID:  0x01, // FeatureDeviceInfo
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode health probe: %w", err)
	}

	if r.pool.Main().tlsConn != nil {
		_ = r.pool.Main().tlsConn.SetWriteDeadline(time.Now().Add(healthProbeWriteTimeout))
		defer func() {
			if r.pool.Main().tlsConn != nil {
				_ = r.pool.Main().tlsConn.SetWriteDeadline(time.Time{})
			}
		}()
	} else if r.pool.Main().conn != nil {
		_ = r.pool.Main().conn.SetWriteDeadline(time.Now().Add(healthProbeWriteTimeout))
		defer func() {
			if r.pool.Main().conn != nil {
				_ = r.pool.Main().conn.SetWriteDeadline(time.Time{})
			}
		}()
	}

	if err := r.pool.Main().framer.WriteFrame(data); err != nil {
		r.pool.Main().transitionTo(ConnDisconnected)
		return fmt.Errorf("send health probe: %w", err)
	}

	// Short timeout -- we just need to know the connection is alive.
	if r.pool.Main().tlsConn != nil {
		_ = r.pool.Main().tlsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		defer func() {
			if r.pool.Main().tlsConn != nil {
				_ = r.pool.Main().tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	}

	// Read response, discarding stale notifications (messageId=0) and
	// orphaned responses from previous operations (mismatched messageId).
	drained := 0
	for range 20 {
		respData, err := r.pool.Main().framer.ReadFrame()
		if err != nil {
			r.pool.Main().transitionTo(ConnDisconnected)
			return fmt.Errorf("read health probe response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode health probe response: %w", err)
		}
		if resp.MessageID == 0 {
			drained++
			continue // Discard stale notification
		}
		if resp.MessageID != req.MessageID {
			r.debugf("probeSessionHealth: discarding orphaned response (got msgID=%d, want %d)", resp.MessageID, req.MessageID)
			drained++
			continue
		}
		if drained > 0 {
			r.debugf("probeSessionHealth: discarded %d stale frames", drained)
		}
		r.debugf("probeSessionHealth: device responded (status=%d)", resp.Status)
		return nil
	}
	return fmt.Errorf("health probe: too many interleaved frames (%d discarded)", drained)
}

// waitForOperationalReady subscribes to DeviceInfo (endpoint 0, feature 0x01)
// and waits for the priming report. A successful response confirms the
// device's operational handler is running and processing protocol messages
// on this connection.
//
// This follows Matter's subscribe-based readiness pattern: instead of sleeping
// a fixed duration, we perform a protocol-level probe that returns as soon as
// the device is ready.
func (r *Runner) waitForOperationalReady(timeout time.Duration) error {
	return waitForOperationalReadyOnConn(
		r.pool.Main(),
		timeout,
		r.nextMessageID,
		r.pool.AppendNotification,
		r.debugf,
	)
}

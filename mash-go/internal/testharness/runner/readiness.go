package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/pkg/wire"
)

// waitForCommissioningMode polls mDNS until the device advertises the
// commissionable service (_mashc._udp), indicating it has re-entered
// commissioning mode. Uses exponential backoff on browse windows
// (300ms initial, doubling up to 1s).
func (r *Runner) waitForCommissioningMode(ctx context.Context, timeout time.Duration) error {
	start := time.Now()
	deadline := start.Add(timeout)
	browseMs := 300 // initial browse window in ms
	for time.Now().Before(deadline) {
		browseCtx, cancel := context.WithTimeout(ctx, time.Duration(browseMs)*time.Millisecond)
		services, err := r.browseMDNSOnce(browseCtx, "commissionable", nil, browseMs)
		cancel()
		if err == nil && len(services) > 0 {
			r.debugf("waitForCommissioningMode: device found after %v", time.Since(start))
			return nil
		}
		browseMs = min(browseMs*2, 1000)
	}
	return fmt.Errorf("timeout waiting for commissioning mode after %v", timeout)
}

// probeSessionHealth sends a lightweight Read request to DeviceInfo (endpoint 0,
// feature 0x01) to verify the connection is still alive and the device is
// responding. Returns nil if the session is healthy, an error otherwise.
// Used by setupPreconditions to detect corrupted sessions before reuse.
func (r *Runner) probeSessionHealth() error {
	if r.conn == nil || !r.conn.connected || r.conn.framer == nil {
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

	if err := r.conn.framer.WriteFrame(data); err != nil {
		r.conn.connected = false
		return fmt.Errorf("send health probe: %w", err)
	}

	// Short timeout -- we just need to know the connection is alive.
	if r.conn.tlsConn != nil {
		_ = r.conn.tlsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		defer func() {
			if r.conn.tlsConn != nil {
				_ = r.conn.tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	}

	// Read response, skipping interleaved notifications (messageId=0).
	for range 5 {
		respData, err := r.conn.framer.ReadFrame()
		if err != nil {
			r.conn.connected = false
			return fmt.Errorf("read health probe response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode health probe response: %w", err)
		}
		if resp.MessageID == 0 {
			r.pendingNotifications = append(r.pendingNotifications, respData)
			continue
		}
		r.debugf("probeSessionHealth: device responded (status=%d)", resp.Status)
		return nil
	}
	return fmt.Errorf("health probe: too many interleaved notifications")
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
	if r.conn == nil || !r.conn.connected {
		return fmt.Errorf("not connected")
	}

	// Subscribe to DeviceInfo (feature 0x01) on endpoint 0.
	// DeviceInfo is always present and the subscribe response includes
	// priming data, confirming the full protocol stack is operational.
	req := &wire.Request{
		MessageID:  r.nextMessageID(),
		Operation:  wire.OpSubscribe,
		EndpointID: 0,
		FeatureID:  0x01, // FeatureDeviceInfo
	}
	data, err := wire.EncodeRequest(req)
	if err != nil {
		return fmt.Errorf("encode readiness probe: %w", err)
	}

	// Send the subscribe frame.
	if err := r.conn.framer.WriteFrame(data); err != nil {
		r.conn.connected = false
		return fmt.Errorf("send readiness probe: %w", err)
	}

	// Set a tight read deadline so we don't block long on an unresponsive device.
	if r.conn.tlsConn != nil {
		_ = r.conn.tlsConn.SetReadDeadline(time.Now().Add(timeout))
		defer func() {
			if r.conn.tlsConn != nil {
				_ = r.conn.tlsConn.SetReadDeadline(time.Time{})
			}
		}()
	}

	// Read response, skipping any interleaved notifications.
	for range 5 {
		respData, err := r.conn.framer.ReadFrame()
		if err != nil {
			r.conn.connected = false
			return fmt.Errorf("read readiness response: %w", err)
		}
		resp, err := wire.DecodeResponse(respData)
		if err != nil {
			return fmt.Errorf("decode readiness response: %w", err)
		}
		// Notifications have messageId=0; buffer them for later consumption.
		if resp.MessageID == 0 {
			r.pendingNotifications = append(r.pendingNotifications, respData)
			continue
		}
		r.debugf("waitForOperationalReady: device responded (status=%d)", resp.Status)
		return nil
	}
	return fmt.Errorf("readiness probe: too many interleaved notifications")
}

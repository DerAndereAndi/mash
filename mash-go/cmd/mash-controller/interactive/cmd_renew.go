package interactive

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/pkg/service"
)

// cmdRenew handles the renew command.
// Usage:
//   - renew <device-id>  - Renew specific device certificate
//   - renew --all        - Renew all devices needing renewal
//   - renew --status     - Show certificate expiry status
func (c *Controller) cmdRenew(ctx context.Context, args []string) {
	// Check for --status flag
	if len(args) > 0 && args[0] == "--status" {
		c.showRenewalStatus()
		return
	}

	// Check for --all flag
	if len(args) > 0 && args[0] == "--all" {
		c.renewAllDevices(ctx)
		return
	}

	// Single device renewal
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(), "Usage: renew <device-id>")
		fmt.Fprintln(c.rl.Stdout(), "       renew --status  (show certificate expiry status)")
		fmt.Fprintln(c.rl.Stdout(), "       renew --all     (renew all devices needing renewal)")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(), "Device not found: %s\n", args[0])
		return
	}

	c.renewDevice(ctx, deviceID)
}

// showRenewalStatus displays certificate expiry status for all devices.
func (c *Controller) showRenewalStatus() {
	devices := c.svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Fprintln(c.rl.Stdout(), "No devices connected")
		return
	}

	// Get renewal tracker if available (need to add getter to service)
	// For now, show device info with estimated expiry based on commissioning time
	fmt.Fprintln(c.rl.Stdout(), "\nCertificate Status:")
	fmt.Fprintln(c.rl.Stdout(), "-------------------------------------------")

	for _, d := range devices {
		// Estimate expiry as 1 year from last seen (commissioning time proxy)
		// In production, this would come from actual certificate expiry
		expiryEst := d.LastSeen.Add(365 * 24 * time.Hour)
		daysUntil := int(time.Until(expiryEst).Hours() / 24)

		status := "OK"
		if daysUntil <= 30 {
			status = "NEEDS RENEWAL"
		}
		if daysUntil <= 7 {
			status = "EXPIRING SOON"
		}
		if daysUntil <= 0 {
			status = "EXPIRED"
		}

		// Show short device ID (first 16 chars)
		shortID := d.ID
		if len(shortID) > 16 {
			shortID = shortID[:16]
		}

		connStatus := "connected"
		if !d.Connected {
			connStatus = "disconnected"
		}

		fmt.Fprintf(c.rl.Stdout(), "  %s...  %d days  [%s] (%s)\n",
			shortID, daysUntil, status, connStatus)
	}
	fmt.Fprintln(c.rl.Stdout())
}

// renewDevice renews the certificate for a single device.
func (c *Controller) renewDevice(ctx context.Context, deviceID string) {
	// Check if device is connected
	device := c.svc.GetDevice(deviceID)
	if device == nil {
		fmt.Fprintf(c.rl.Stdout(), "Device not found: %s\n", deviceID)
		return
	}

	if !device.Connected {
		fmt.Fprintf(c.rl.Stdout(), "Cannot renew: device %s is not connected\n", deviceID)
		return
	}

	session := c.svc.GetSession(deviceID)
	if session == nil {
		fmt.Fprintf(c.rl.Stdout(), "No active session for device %s\n", deviceID)
		return
	}

	// Short device ID for display
	shortID := deviceID
	if len(shortID) > 16 {
		shortID = shortID[:16]
	}

	fmt.Fprintf(c.rl.Stdout(), "Renewing certificate for %s...\n", shortID)

	// TODO: Implement actual renewal using ControllerRenewalHandler
	// This requires:
	// 1. Getting the Zone CA from certStore
	// 2. Creating a ControllerRenewalHandler with the session's connection
	// 3. Calling RenewDevice
	// 4. Updating the RenewalTracker
	//
	// For now, show that the infrastructure is in place
	fmt.Fprintln(c.rl.Stdout(), "Certificate renewal not yet fully integrated.")
	fmt.Fprintln(c.rl.Stdout(), "The renewal protocol is implemented in:")
	fmt.Fprintln(c.rl.Stdout(), "  - pkg/service/controller_renewal.go")
	fmt.Fprintln(c.rl.Stdout(), "  - pkg/service/device_renewal.go")
}

// renewAllDevices renews all devices that need renewal.
func (c *Controller) renewAllDevices(ctx context.Context) {
	devices := c.svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Fprintln(c.rl.Stdout(), "No devices connected")
		return
	}

	// Find devices needing renewal
	var needsRenewal []*service.ConnectedDevice
	for _, d := range devices {
		// Estimate expiry as 1 year from last seen
		expiryEst := d.LastSeen.Add(365 * 24 * time.Hour)
		daysUntil := int(time.Until(expiryEst).Hours() / 24)

		if daysUntil <= 30 {
			needsRenewal = append(needsRenewal, d)
		}
	}

	if len(needsRenewal) == 0 {
		fmt.Fprintln(c.rl.Stdout(), "No devices need renewal at this time")
		return
	}

	fmt.Fprintf(c.rl.Stdout(), "Found %d device(s) needing renewal:\n", len(needsRenewal))
	for _, d := range needsRenewal {
		shortID := d.ID
		if len(shortID) > 16 {
			shortID = shortID[:16]
		}
		fmt.Fprintf(c.rl.Stdout(), "  - %s\n", shortID)
	}

	fmt.Fprintln(c.rl.Stdout(), "\nRenewal not yet fully integrated.")
}

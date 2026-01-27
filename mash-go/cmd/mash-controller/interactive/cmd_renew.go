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
	// Check if device exists
	device := c.svc.GetDevice(deviceID)
	if device == nil {
		fmt.Fprintf(c.rl.Stdout(), "Device not found: %s\n", deviceID)
		return
	}

	if !device.Connected {
		fmt.Fprintf(c.rl.Stdout(), "Cannot renew: device %s is not connected\n", deviceID)
		return
	}

	// Short device ID for display
	shortID := deviceID
	if len(shortID) > 16 {
		shortID = shortID[:16]
	}

	fmt.Fprintf(c.rl.Stdout(), "Renewing certificate for %s...\n", shortID)

	// Call the service method - handles all protocol complexity
	err := c.svc.RenewDevice(ctx, deviceID)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(), "Renewal failed: %v\n", err)
		return
	}

	fmt.Fprintf(c.rl.Stdout(), "Certificate renewed successfully for %s\n", shortID)
}

// renewAllDevices renews all devices that need renewal.
func (c *Controller) renewAllDevices(ctx context.Context) {
	devices := c.svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Fprintln(c.rl.Stdout(), "No devices connected")
		return
	}

	// Find devices needing renewal (within 30 days of expiry) AND connected
	var needsRenewal []*service.ConnectedDevice
	for _, d := range devices {
		// Estimate expiry as 1 year from last seen (commissioning time proxy)
		// In production, this would come from actual certificate expiry
		expiryEst := d.LastSeen.Add(365 * 24 * time.Hour)
		daysUntil := int(time.Until(expiryEst).Hours() / 24)

		if daysUntil <= 30 && d.Connected {
			needsRenewal = append(needsRenewal, d)
		}
	}

	if len(needsRenewal) == 0 {
		fmt.Fprintln(c.rl.Stdout(), "No connected devices need renewal at this time")
		return
	}

	fmt.Fprintf(c.rl.Stdout(), "Renewing %d device(s):\n", len(needsRenewal))

	successCount := 0
	failCount := 0

	for _, d := range needsRenewal {
		shortID := d.ID
		if len(shortID) > 16 {
			shortID = shortID[:16]
		}

		fmt.Fprintf(c.rl.Stdout(), "  %s... ", shortID)

		err := c.svc.RenewDevice(ctx, d.ID)
		if err != nil {
			fmt.Fprintf(c.rl.Stdout(), "FAILED: %v\n", err)
			failCount++
		} else {
			fmt.Fprintln(c.rl.Stdout(), "OK")
			successCount++
		}
	}

	fmt.Fprintf(c.rl.Stdout(), "\nRenewal complete: %d succeeded, %d failed\n", successCount, failCount)
}

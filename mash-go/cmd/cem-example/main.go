// Command cem-example demonstrates a MASH-compliant Central Energy Manager.
//
// This example shows how to:
//   - Create a CEM controller
//   - Use ControllerService for device discovery
//   - Commission devices using setup codes
//   - Set power limits on controllable devices
//   - Monitor device state via subscriptions
//
// Usage:
//
//	go run ./cmd/cem-example
//
// The controller will:
//  1. Start and advertise as a commissioner
//  2. Discover commissionable devices via mDNS
//  3. Commission discovered devices
//  4. Set and adjust power limits
//  5. Monitor device measurements
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/service"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("MASH CEM Example Controller")
	log.Println("===========================")

	// Create CEM device model
	cem := createCEM()
	log.Printf("Created CEM device: %s", cem.Device().DeviceID())

	// Create controller service
	config := service.DefaultControllerConfig()
	config.ZoneName = "Home Energy"
	config.ZoneType = cert.ZoneTypeHomeManager
	config.DiscoveryTimeout = 30 * time.Second
	config.ConnectionTimeout = 10 * time.Second

	svc, err := service.NewControllerService(config)
	if err != nil {
		log.Fatalf("Failed to create controller service: %v", err)
	}

	// Register event handler
	svc.OnEvent(func(event service.Event) {
		handleEvent(event, cem)
	})

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Service started (state: %s)", svc.State())
	log.Printf("Zone: %s (type: %s)", config.ZoneName, config.ZoneType)

	// Start controller loop
	go runControllerLoop(ctx, svc, cem)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	if err := svc.Stop(); err != nil {
		log.Printf("Error stopping service: %v", err)
	}
	log.Println("Goodbye!")
}

func createCEM() *examples.CEM {
	cfg := examples.CEMConfig{
		DeviceID:     "cem-example-001",
		VendorName:   "MASH Examples",
		ProductName:  "Example CEM",
		SerialNumber: "CEM-001",
		VendorID:     0x1234,
		ProductID:    0x0002,
	}

	return examples.NewCEM(cfg)
}

func handleEvent(event service.Event, _ *examples.CEM) {
	switch event.Type {
	case service.EventConnected:
		log.Printf("Device connected: %s", event.DeviceID)

	case service.EventDisconnected:
		log.Printf("Device disconnected: %s", event.DeviceID)

	case service.EventCommissioned:
		log.Printf("Device commissioned: %s", event.DeviceID)

	case service.EventDecommissioned:
		log.Printf("Device decommissioned: %s", event.DeviceID)

	case service.EventValueChanged:
		log.Printf("Value changed on device %s", event.DeviceID)
	}
}

func runControllerLoop(ctx context.Context, svc *service.ControllerService, cem *examples.CEM) {
	log.Println("Starting controller loop...")

	// Simulation state
	gridLimit := int64(15000000) // 15 kW grid limit in mW
	currentLimit := int64(22000000) // Start with no limit (22 kW max)

	// Discovery ticker - check for new devices periodically
	discoveryTicker := time.NewTicker(30 * time.Second)
	defer discoveryTicker.Stop()

	// Control ticker - adjust limits periodically
	controlTicker := time.NewTicker(10 * time.Second)
	defer controlTicker.Stop()

	// Initial discovery
	discoverDevices(ctx, svc)

	for {
		select {
		case <-ctx.Done():
			return

		case <-discoveryTicker.C:
			discoverDevices(ctx, svc)

		case <-controlTicker.C:
			// Simulate dynamic load management
			totalPower := cem.GetTotalPower()

			log.Printf("Total power: %.1f kW (grid limit: %.1f kW)",
				float64(totalPower)/1000000, float64(gridLimit)/1000000)

			// Adjust limits based on simulated grid conditions
			if totalPower > gridLimit {
				// Reduce limit
				newLimit := currentLimit - 1000000 // Reduce by 1 kW
				if newLimit < 1380000 { // Minimum 1.38 kW
					newLimit = 1380000
				}
				if newLimit != currentLimit {
					currentLimit = newLimit
					log.Printf("Reducing power limit to %.1f kW", float64(currentLimit)/1000000)
					setLimitsOnAllDevices(ctx, svc, cem, currentLimit)
				}
			} else if totalPower < gridLimit-5000000 { // 5 kW headroom
				// Increase limit
				newLimit := currentLimit + 1000000 // Increase by 1 kW
				if newLimit > 22000000 { // Max 22 kW
					newLimit = 22000000
				}
				if newLimit != currentLimit {
					currentLimit = newLimit
					log.Printf("Increasing power limit to %.1f kW", float64(currentLimit)/1000000)
					setLimitsOnAllDevices(ctx, svc, cem, currentLimit)
				}
			}

			// Log connected devices
			devices := svc.GetAllDevices()
			if len(devices) > 0 {
				log.Printf("Connected devices: %d", len(devices))
				for _, d := range devices {
					log.Printf("  - %s: connected=%v", d.ID, d.Connected)
				}
			}
		}
	}
}

func discoverDevices(ctx context.Context, svc *service.ControllerService) {
	log.Println("Discovering commissionable devices...")

	// Use a short timeout for discovery
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Discover EV chargers
	devices, err := svc.Discover(discoverCtx, discovery.FilterByCategory(discovery.CategoryEMobility))
	if err != nil {
		// Not an error - browser might not be configured
		log.Printf("Discovery not available: %v", err)
		return
	}

	if len(devices) == 0 {
		log.Println("No commissionable devices found")
		return
	}

	log.Printf("Found %d commissionable device(s):", len(devices))
	for _, d := range devices {
		log.Printf("  - %s (discriminator: %d, host: %s:%d)",
			d.InstanceName, d.Discriminator, d.Host, d.Port)

		// Commission the device
		log.Printf("Commissioning device with discriminator %d...", d.Discriminator)

		// In a real implementation, the setup code would come from:
		// - QR code scanned by user
		// - Manual entry
		// - Pre-configured pairing
		setupCode := "12345678" // Default setup code for example devices

		device, err := svc.Commission(ctx, d, setupCode)
		if err != nil {
			log.Printf("Failed to commission device: %v", err)
			continue
		}

		log.Printf("Device commissioned: %s", device.ID)
	}
}

func setLimitsOnAllDevices(ctx context.Context, svc *service.ControllerService, cem *examples.CEM, limitMW int64) {
	devices := svc.GetAllDevices()
	for _, d := range devices {
		if !d.Connected {
			continue
		}

		// In a real implementation, we'd use the interaction client
		// For now, just log the intent
		log.Printf("Would set limit %.1f kW on device %s", float64(limitMW)/1000000, d.ID)

		// If we had a real connection, we'd call:
		// err := cem.SetPowerLimit(ctx, d.ID, 1, limitMW)
		_ = ctx
		_ = cem
	}
}

func printBanner() {
	fmt.Print(`
 __  __    _    ____  _   _    ____ _____ __  __
|  \/  |  / \  / ___|| | | |  / ___| ____|  \/  |
| |\/| | / _ \ \___ \| |_| | | |   |  _| | |\/| |
| |  | |/ ___ \ ___) |  _  | | |___| |___| |  | |
|_|  |_/_/   \_\____/|_| |_|  \____|_____|_|  |_|

Central Energy Manager - Example Implementation
`)
}

func init() {
	printBanner()
}

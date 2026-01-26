// Command evse-example demonstrates a MASH-compliant EV charger device.
//
// This example shows how to:
//   - Create an EVSE device with proper features
//   - Use DeviceService for connection management
//   - Handle commissioning mode
//   - Process controller commands (limits, pause/resume)
//   - Report measurements and session state
//
// Usage:
//
//	go run ./cmd/evse-example
//
// The device will:
//  1. Start in commissioning mode
//  2. Advertise via mDNS (when network layer is available)
//  3. Accept connections from controllers
//  4. Simulate charging sessions
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/service"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Println("MASH EVSE Example Device")
	log.Println("========================")

	// Create EVSE device model
	evse := createEVSE()
	log.Printf("Created EVSE device: %s", evse.Device().DeviceID())

	// Create device service
	config := service.DefaultDeviceConfig()
	config.Discriminator = 1234
	config.SetupCode = "12345678"
	config.SerialNumber = "EVSE-001"
	config.Brand = "MASH"
	config.Model = "Example EVSE"
	config.DeviceName = "Garage Charger"
	config.Categories = []discovery.DeviceCategory{discovery.CategoryEMobility}
	config.FailsafeTimeout = 2 * time.Hour

	svc, err := service.NewDeviceService(evse.Device(), config)
	if err != nil {
		log.Fatalf("Failed to create device service: %v", err)
	}

	// Register event handler
	svc.OnEvent(func(event service.Event) {
		handleEvent(event, evse)
	})

	// Register limit change callback
	evse.OnLimitChanged(func(limit *int64) {
		if limit != nil {
			log.Printf("Limit changed: %d mW (%.1f kW)", *limit, float64(*limit)/1000000)
		} else {
			log.Println("Limit cleared")
		}
	})

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Service started (state: %s)", svc.State())

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		log.Printf("Warning: Failed to enter commissioning mode: %v", err)
	} else {
		log.Printf("Commissioning mode active (discriminator: %d, setup code: %s)",
			config.Discriminator, config.SetupCode)
	}

	// Print QR code info
	printQRCodeInfo(config)

	// Start simulation goroutine
	go runSimulation(ctx, evse)

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

func createEVSE() *examples.EVSE {
	cfg := examples.EVSEConfig{
		DeviceID:     "evse-example-001",
		VendorName:   "MASH Examples",
		ProductName:  "Example EVSE",
		SerialNumber: "EVSE-001",
		VendorID:     0x1234,
		ProductID:    0x0001,

		// 22 kW, 3-phase charger
		PhaseCount:            3,
		NominalVoltage:        230,
		MaxCurrentPerPhase:    32000,  // 32A in mA
		MinCurrentPerPhase:    6000,   // 6A in mA
		NominalMaxPower:       22000000, // 22 kW in mW
		NominalMinPower:       1380000,  // 1.38 kW in mW
		SupportsBidirectional: false,
	}

	return examples.NewEVSE(cfg)
}

func handleEvent(event service.Event, evse *examples.EVSE) {
	switch event.Type {
	case service.EventConnected:
		log.Printf("Zone connected: %s", event.ZoneID)
		evse.AcceptController()

	case service.EventDisconnected:
		log.Printf("Zone disconnected: %s", event.ZoneID)

	case service.EventCommissioningOpened:
		log.Println("Commissioning window opened")

	case service.EventCommissioningClosed:
		log.Println("Commissioning window closed")

	case service.EventFailsafeTriggered:
		log.Printf("FAILSAFE triggered for zone %s!", event.ZoneID)

	case service.EventFailsafeCleared:
		log.Printf("Failsafe cleared for zone %s", event.ZoneID)

	case service.EventValueChanged:
		log.Printf("Value changed in zone %s", event.ZoneID)
	}
}

func printQRCodeInfo(config service.DeviceConfig) {
	// MASH QR code format: MASH:<version>:<discriminator>:<setupcode>
	qrString := fmt.Sprintf("MASH:1:%d:%s", config.Discriminator, config.SetupCode)
	log.Println("")
	log.Println("To commission this device, scan this QR code:")
	log.Printf("  %s", qrString)
	log.Println("")
	log.Println("Or manually enter:")
	log.Printf("  Discriminator: %d", config.Discriminator)
	log.Printf("  Setup Code: %s", config.SetupCode)
	log.Println("")
}

func runSimulation(ctx context.Context, evse *examples.EVSE) {
	log.Println("Starting charging simulation...")

	// Simulation state
	evConnected := false
	sessionNumber := 0

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !evConnected {
				// Occasionally connect an EV
				sessionNumber++
				log.Printf("EV connected (session %d)", sessionNumber)
				evse.SimulateEVConnect(
					30,    // 30% SoC
					60000, // 60 kWh battery capacity (Wh)
					features.EVDemandModeSingleDemand,
				)
				evConnected = true
			} else {
				// Update charging
				power := evse.GetCurrentPower()
				limit := evse.GetEffectiveLimit()

				// Simulate charging at max available power
				targetPower := int64(7400000) // 7.4 kW in mW (single phase limit)
				if limit != nil && *limit < targetPower {
					targetPower = *limit
				}

				if power != targetPower {
					evse.SimulateCharging(targetPower)
				}

				// Log current state
				var limitStr string
				if limit != nil {
					limitStr = fmt.Sprintf("%.1f kW", float64(*limit)/1000000)
				} else {
					limitStr = "none"
				}
				log.Printf("Charging: %.1f kW (limit: %s)",
					float64(evse.GetCurrentPower())/1000000, limitStr)

				// Occasionally disconnect
				if sessionNumber%3 == 0 {
					log.Println("EV disconnected")
					evse.SimulateEVDisconnect()
					evConnected = false
				}
			}
		}
	}
}

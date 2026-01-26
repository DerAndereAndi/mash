// Command mash-controller is a reference MASH controller (EMS) implementation.
//
// This command demonstrates a complete MASH-compliant controller with:
//   - CLI argument parsing
//   - Configuration file support
//   - Device discovery and commissioning
//   - Interactive command interface
//   - Power limit management
//   - Comprehensive logging
//
// Usage:
//
//	mash-controller [flags]
//
// Flags:
//
//	-config string      Configuration file path
//	-zone-name string   Zone name for this controller (default "Home Energy")
//	-zone-type string   Zone type: grid, building, home, user (default "home")
//	-log-level string   Log level: debug, info, warn, error (default "info")
//	-interactive        Enable interactive command mode
//	-auto-commission    Automatically commission discovered devices
//
// Examples:
//
//	# Start controller with interactive mode
//	mash-controller -zone-name "My Home" -interactive
//
//	# Start controller that auto-commissions devices
//	mash-controller -auto-commission -log-level debug
//
// Interactive Commands:
//
//	discover    - Discover commissionable devices
//	list        - List connected devices
//	commission <discriminator> <setup-code> - Commission a device
//	limit <device-id> <power-kw> - Set power limit
//	clear <device-id> - Clear power limit
//	pause <device-id> - Pause device
//	resume <device-id> - Resume device
//	status      - Show controller status
//	quit        - Exit the controller
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Config holds the controller configuration.
type Config struct {
	ConfigFile     string
	ZoneName       string
	ZoneType       string
	LogLevel       string
	Interactive    bool
	AutoCommission bool
}

var (
	config Config
	cem    *examples.CEM
	svc    *service.ControllerService
)

func init() {
	flag.StringVar(&config.ConfigFile, "config", "", "Configuration file path")
	flag.StringVar(&config.ZoneName, "zone-name", "Home Energy", "Zone name for this controller")
	flag.StringVar(&config.ZoneType, "zone-type", "home", "Zone type: grid, building, home, user")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.BoolVar(&config.Interactive, "interactive", false, "Enable interactive command mode")
	flag.BoolVar(&config.AutoCommission, "auto-commission", false, "Automatically commission discovered devices")
}

func main() {
	flag.Parse()

	// Setup logging
	setupLogging(config.LogLevel)

	log.Println("MASH Reference Controller")
	log.Println("=========================")
	log.Printf("Zone name: %s", config.ZoneName)
	log.Printf("Zone type: %s", config.ZoneType)

	// Parse zone type
	zoneType, err := parseZoneType(config.ZoneType)
	if err != nil {
		log.Fatalf("Invalid zone type: %v", err)
	}

	// Create CEM device model
	cem = examples.NewCEM(examples.CEMConfig{
		DeviceID:     fmt.Sprintf("cem-%d", time.Now().Unix()%10000),
		VendorName:   "MASH Reference",
		ProductName:  "Reference Controller",
		SerialNumber: fmt.Sprintf("CEM-%d", time.Now().Unix()%10000),
		VendorID:     0x1234,
		ProductID:    0x1000,
	})
	log.Printf("Created CEM: %s", cem.Device().DeviceID())

	// Create controller service
	svcConfig := service.DefaultControllerConfig()
	svcConfig.ZoneName = config.ZoneName
	svcConfig.ZoneType = zoneType

	svc, err = service.NewControllerService(svcConfig)
	if err != nil {
		log.Fatalf("Failed to create controller service: %v", err)
	}

	// Register event handler
	svc.OnEvent(handleEvent)

	// Start service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("Failed to start service: %v", err)
	}
	log.Printf("Service started (state: %s)", svc.State())

	// Start background tasks
	go runDiscoveryLoop(ctx)
	go runMonitoringLoop(ctx)

	// Run interactive mode or wait for signal
	if config.Interactive {
		go runInteractive(ctx, cancel)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down...")

	cancel()

	if err := svc.Stop(); err != nil {
		log.Printf("Error stopping service: %v", err)
	}

	log.Println("Goodbye!")
}

func setupLogging(level string) {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	switch level {
	case "debug":
		log.SetFlags(log.Ltime | log.Lmicroseconds | log.Lshortfile)
	case "warn", "error":
		log.SetFlags(log.Ltime)
	}
}

func parseZoneType(s string) (cert.ZoneType, error) {
	switch strings.ToLower(s) {
	case "grid", "gridoperator":
		return cert.ZoneTypeGridOperator, nil
	case "building", "buildingmanager":
		return cert.ZoneTypeBuildingManager, nil
	case "home", "homemanager":
		return cert.ZoneTypeHomeManager, nil
	case "user", "userapp":
		return cert.ZoneTypeUserApp, nil
	default:
		return 0, fmt.Errorf("unknown zone type: %s (use: grid, building, home, user)", s)
	}
}

func handleEvent(event service.Event) {
	switch event.Type {
	case service.EventConnected:
		log.Printf("[EVENT] Device connected: %s", event.DeviceID)
	case service.EventDisconnected:
		log.Printf("[EVENT] Device disconnected: %s", event.DeviceID)
		// Remove from CEM
		_ = cem.DisconnectDevice(event.DeviceID)
	case service.EventCommissioned:
		log.Printf("[EVENT] Device commissioned: %s", event.DeviceID)
		// Wire up device to CEM for monitoring
		go setupDeviceMonitoring(event.DeviceID)
	case service.EventDecommissioned:
		log.Printf("[EVENT] Device decommissioned: %s", event.DeviceID)
		_ = cem.DisconnectDevice(event.DeviceID)
	case service.EventValueChanged:
		log.Printf("[EVENT] Value changed (device: %s)", event.DeviceID)
	}
}

func setupDeviceMonitoring(deviceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get the session for this device
	session := svc.GetSession(deviceID)
	if session == nil {
		log.Printf("[MONITOR] No session for device %s", deviceID)
		return
	}

	// Add device to CEM
	_, err := cem.ConnectDevice(deviceID, session)
	if err != nil {
		log.Printf("[MONITOR] Failed to add device to CEM: %v", err)
		return
	}

	// Set up notification handler to route to CEM and display updates
	session.SetNotificationHandler(func(notif *wire.Notification) {
		cem.HandleNotification(deviceID, notif.EndpointID, notif.FeatureID, notif.Changes)

		// Log power updates in real-time
		if notif.FeatureID == 2 { // FeatureMeasurement
			if rawPower, exists := notif.Changes[1]; exists { // MeasurementAttrACActivePower
				if power, ok := wire.ToInt64(rawPower); ok {
					powerKW := float64(power) / 1_000_000.0
					log.Printf("[NOTIFY] Device %s power: %.1f kW", deviceID[:8], powerKW)
				}
			}
		}
	})

	// Subscribe to Measurement on endpoint 1 (functional endpoint)
	if err := cem.SubscribeToMeasurement(ctx, deviceID, 1); err != nil {
		log.Printf("[MONITOR] Failed to subscribe to measurement: %v", err)
	} else {
		log.Printf("[MONITOR] Subscribed to measurements for device %s", deviceID)
	}
}

func runDiscoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial discovery
	discoverDevices(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			discoverDevices(ctx)
		}
	}
}

func discoverDevices(ctx context.Context) {
	log.Println("Discovering commissionable devices...")

	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	devices, err := svc.Discover(discoverCtx, nil)
	if err != nil {
		// Discovery might not be available without a browser
		if config.LogLevel == "debug" {
			log.Printf("Discovery: %v", err)
		}
		return
	}

	if len(devices) == 0 {
		log.Println("No commissionable devices found")
		return
	}

	log.Printf("Found %d commissionable device(s):", len(devices))
	for i, d := range devices {
		log.Printf("  %d. %s (discriminator: %d, host: %s:%d)",
			i+1, d.InstanceName, d.Discriminator, d.Host, d.Port)
		for _, cat := range d.Categories {
			log.Printf("     Category: %s", cat)
		}

		// Auto-commission if enabled
		if config.AutoCommission {
			log.Printf("Auto-commissioning device %d...", d.Discriminator)
			_, err := svc.Commission(ctx, d, "12345678") // Default setup code
			if err != nil {
				log.Printf("Failed to commission: %v", err)
			}
		}
	}
}

func runMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			devices := svc.GetAllDevices()
			if len(devices) > 0 {
				log.Printf("Connected devices: %d", len(devices))
				totalPower := cem.GetTotalPower()
				log.Printf("Total power: %.1f kW", float64(totalPower)/1000000)
			}
		}
	}
}

func runInteractive(ctx context.Context, cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)

	printHelp()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Print("\nmash> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])
		args := parts[1:]

		switch cmd {
		case "help", "?":
			printHelp()

		case "discover":
			discoverDevices(ctx)

		case "list", "ls":
			listDevices()

		case "commission":
			if len(args) < 2 {
				fmt.Println("Usage: commission <discriminator> <setup-code>")
				continue
			}
			disc, err := strconv.ParseUint(args[0], 10, 16)
			if err != nil {
				fmt.Printf("Invalid discriminator: %v\n", err)
				continue
			}
			commissionDevice(ctx, uint16(disc), args[1])

		case "limit":
			if len(args) < 2 {
				fmt.Println("Usage: limit <device-id> <power-kw>")
				continue
			}
			powerKW, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				fmt.Printf("Invalid power: %v\n", err)
				continue
			}
			setLimit(ctx, args[0], int64(powerKW*1000000))

		case "clear":
			if len(args) < 1 {
				fmt.Println("Usage: clear <device-id>")
				continue
			}
			clearLimit(ctx, args[0])

		case "pause":
			if len(args) < 1 {
				fmt.Println("Usage: pause <device-id>")
				continue
			}
			pauseDevice(ctx, args[0])

		case "resume":
			if len(args) < 1 {
				fmt.Println("Usage: resume <device-id>")
				continue
			}
			resumeDevice(ctx, args[0])

		case "status":
			showStatus()

		case "quit", "exit", "q":
			fmt.Println("Exiting...")
			cancel()
			return

		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func printHelp() {
	fmt.Println(`
MASH Controller Commands:
  discover                          - Discover commissionable devices
  list                              - List connected devices
  commission <discriminator> <code> - Commission a device
  limit <device-id> <power-kw>      - Set power limit (kW)
  clear <device-id>                 - Clear power limit
  pause <device-id>                 - Pause device
  resume <device-id>                - Resume device
  status                            - Show controller status
  help                              - Show this help
  quit                              - Exit controller`)
}

func listDevices() {
	devices := svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Println("No devices connected")
		return
	}

	fmt.Printf("\nConnected devices (%d):\n", len(devices))
	fmt.Println("─────────────────────────────────────────")
	for _, d := range devices {
		status := "connected"
		if !d.Connected {
			status = "disconnected"
		}
		fmt.Printf("  ID: %s\n", d.ID)
		fmt.Printf("      Host: %s:%d\n", d.Host, d.Port)
		fmt.Printf("      Status: %s\n", status)
		fmt.Printf("      Last seen: %s\n", d.LastSeen.Format(time.RFC3339))
		fmt.Println()
	}
}

func commissionDevice(ctx context.Context, discriminator uint16, setupCode string) {
	fmt.Printf("Looking for device with discriminator %d...\n", discriminator)

	device, err := svc.DiscoverByDiscriminator(ctx, discriminator)
	if err != nil {
		fmt.Printf("Device not found: %v\n", err)
		return
	}

	fmt.Printf("Found device: %s at %s:%d\n", device.InstanceName, device.Host, device.Port)
	fmt.Println("Commissioning...")

	commissioned, err := svc.Commission(ctx, device, setupCode)
	if err != nil {
		fmt.Printf("Commissioning failed: %v\n", err)
		return
	}

	fmt.Printf("Device commissioned successfully: %s\n", commissioned.ID)
}

func setLimit(ctx context.Context, deviceID string, limitMW int64) {
	device, ok := cem.GetConnectedDevice(deviceID)
	if !ok {
		// Try partial match
		for _, id := range cem.ConnectedDeviceIDs() {
			if strings.Contains(id, deviceID) {
				deviceID = id
				device, _ = cem.GetConnectedDevice(deviceID)
				break
			}
		}
	}
	if device == nil {
		fmt.Printf("Device not found: %s\n", deviceID)
		return
	}

	fmt.Printf("Setting power limit to %.1f kW on %s...\n", float64(limitMW)/1000000, deviceID)

	err := cem.SetPowerLimit(ctx, deviceID, 1, limitMW)
	if err != nil {
		fmt.Printf("Failed to set limit: %v\n", err)
		return
	}

	fmt.Println("Limit set successfully")
}

func clearLimit(ctx context.Context, deviceID string) {
	fmt.Printf("Clearing power limit on %s...\n", deviceID)

	err := cem.ClearPowerLimit(ctx, deviceID, 1)
	if err != nil {
		fmt.Printf("Failed to clear limit: %v\n", err)
		return
	}

	fmt.Println("Limit cleared")
}

func pauseDevice(ctx context.Context, deviceID string) {
	fmt.Printf("Pausing device %s...\n", deviceID)

	err := cem.PauseDevice(ctx, deviceID, 1)
	if err != nil {
		fmt.Printf("Failed to pause: %v\n", err)
		return
	}

	fmt.Println("Device paused")
}

func resumeDevice(ctx context.Context, deviceID string) {
	fmt.Printf("Resuming device %s...\n", deviceID)

	err := cem.ResumeDevice(ctx, deviceID, 1)
	if err != nil {
		fmt.Printf("Failed to resume: %v\n", err)
		return
	}

	fmt.Println("Device resumed")
}

func showStatus() {
	fmt.Println("\nController Status")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  Zone Name: %s\n", config.ZoneName)
	fmt.Printf("  Zone Type: %s\n", config.ZoneType)
	fmt.Printf("  State: %s\n", svc.State())
	fmt.Printf("  Zone ID: %s\n", svc.ZoneID())
	fmt.Printf("  Connected Devices: %d\n", svc.DeviceCount())
	fmt.Printf("  Total Power: %.1f kW\n", float64(cem.GetTotalPower())/1000000)
	fmt.Println()
}

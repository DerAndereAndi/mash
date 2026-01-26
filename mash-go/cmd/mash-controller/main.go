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
//	-state-dir string   Directory for persistent state
//	-reset              Clear all persisted state before starting
//
// Examples:
//
//	# Start controller with interactive mode
//	mash-controller -zone-name "My Home" -interactive
//
//	# Start controller that auto-commissions devices
//	mash-controller -auto-commission -log-level debug
//
//	# Start with persistence (remembers devices across restarts)
//	mash-controller -zone-name "My Home" -state-dir /var/lib/mash-controller
//
//	# Reset persistent state
//	mash-controller -state-dir /var/lib/mash-controller -reset
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
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/cmd/mash-controller/interactive"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/persistence"
	"github.com/mash-protocol/mash-go/pkg/service"
	"github.com/mash-protocol/mash-go/pkg/wire"
)

// Config holds the controller configuration.
// It implements interactive.ControllerConfig.
type Config struct {
	ConfigFile       string
	ZoneNameValue    string
	ZoneTypeValue    string
	LogLevel         string
	Interactive      bool
	AutoCommission   bool

	// Persistence settings
	StateDir string
	Reset    bool
}

// ZoneName implements interactive.ControllerConfig.
func (c *Config) ZoneName() string {
	return c.ZoneNameValue
}

// ZoneType implements interactive.ControllerConfig.
func (c *Config) ZoneType() string {
	return c.ZoneTypeValue
}

var (
	config Config
	cem    *examples.CEM
	svc    *service.ControllerService
)

func init() {
	flag.StringVar(&config.ConfigFile, "config", "", "Configuration file path")
	flag.StringVar(&config.ZoneNameValue, "zone-name", "Home Energy", "Zone name for this controller")
	flag.StringVar(&config.ZoneTypeValue, "zone-type", "home", "Zone type: grid, building, home, user")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.BoolVar(&config.Interactive, "interactive", false, "Enable interactive command mode")
	flag.BoolVar(&config.AutoCommission, "auto-commission", false, "Automatically commission discovered devices")

	flag.StringVar(&config.StateDir, "state-dir", "", "Directory for persistent state")
	flag.BoolVar(&config.Reset, "reset", false, "Clear all persisted state before starting")
}

func main() {
	flag.Parse()

	// Setup logging
	setupLogging(config.LogLevel)

	log.Println("MASH Reference Controller")
	log.Println("=========================")
	log.Printf("Zone name: %s", config.ZoneNameValue)
	log.Printf("Zone type: %s", config.ZoneTypeValue)

	// Parse zone type
	zoneType, err := parseZoneType(config.ZoneTypeValue)
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
	svcConfig.ZoneName = config.ZoneNameValue
	svcConfig.ZoneType = zoneType

	svc, err = service.NewControllerService(svcConfig)
	if err != nil {
		log.Fatalf("Failed to create controller service: %v", err)
	}

	// Set up persistence if state-dir is provided
	if config.StateDir != "" {
		log.Printf("Using state directory: %s", config.StateDir)

		// Create certificate store
		certStore := cert.NewFileControllerStore(config.StateDir)

		// Create state store
		stateStore := persistence.NewControllerStateStore(filepath.Join(config.StateDir, "state.json"))

		// Handle --reset flag
		if config.Reset {
			log.Println("Resetting persisted state...")
			if err := stateStore.Clear(); err != nil {
				log.Printf("Warning: Failed to clear state: %v", err)
			}
		}

		// Load existing certs (may fail if first run)
		if err := certStore.Load(); err != nil {
			log.Printf("No existing certificates found (first run or reset): %v", err)
		}

		// Set stores on the service
		svc.SetCertStore(certStore)
		svc.SetStateStore(stateStore)

		// Load state (will restore device list, zone ID, etc.)
		if err := svc.LoadState(); err != nil {
			log.Printf("Warning: Failed to load state: %v", err)
		}
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
	if err := svc.StartDiscovery(ctx, nil); err != nil {
		log.Printf("Failed to start discovery: %v", err)
	}

	// Start operational discovery for reconnecting to known devices
	if svc.DeviceCount() > 0 {
		log.Printf("Starting operational discovery for %d known device(s)", svc.DeviceCount())
		if err := svc.StartOperationalDiscovery(ctx); err != nil {
			log.Printf("Warning: Failed to start operational discovery: %v", err)
		}
	}

	go runMonitoringLoop(ctx)

	// Run interactive mode or wait for signal
	if config.Interactive {
		ic, err := interactive.New(svc, cem, &config)
		if err != nil {
			log.Fatalf("Failed to create interactive controller: %v", err)
		}
		// Redirect log output through readline to avoid interfering with input
		log.SetOutput(ic.Stdout())
		go ic.Run(ctx, cancel)
	}

	// Wait for shutdown signal or context cancellation
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)
	case <-ctx.Done():
		// Context was cancelled (e.g., by interactive quit command)
	}

	log.Println("Shutting down...")

	// Save state before stopping
	if config.StateDir != "" {
		log.Println("Saving state...")
		if err := svc.SaveState(); err != nil {
			log.Printf("Warning: Failed to save state: %v", err)
		}
	}

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
	case service.EventDeviceDiscovered:
		if d, ok := event.DiscoveredService.(*discovery.CommissionableService); ok {
			log.Printf("[EVENT] Device discovered: %s (discriminator: %d, host: %s:%d)",
				d.InstanceName, d.Discriminator, d.Host, d.Port)
			for _, cat := range d.Categories {
				log.Printf("     Category: %s", cat)
			}

			// Auto-commission if enabled
			if config.AutoCommission {
				log.Printf("Auto-commissioning device %d...", d.Discriminator)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				_, err := svc.Commission(ctx, d, "12345678") // Default setup code
				cancel()
				if err != nil {
					log.Printf("Failed to commission: %v", err)
				}
			}
		}
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

	case service.EventDeviceRediscovered:
		log.Printf("[EVENT] Known device rediscovered: %s (attempting reconnection...)", event.DeviceID)

	case service.EventDeviceReconnected:
		log.Printf("[EVENT] Device reconnected: %s", event.DeviceID)
		// Re-setup device monitoring after reconnection
		go setupDeviceMonitoring(event.DeviceID)
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

func runMonitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Monitoring tick - status available via 'status' command
		}
	}
}


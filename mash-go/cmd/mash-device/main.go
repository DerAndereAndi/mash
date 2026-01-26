// Command mash-device is a reference MASH device implementation.
//
// This command demonstrates a complete MASH-compliant device with:
//   - CLI argument parsing
//   - Configuration file support
//   - Multiple simulated device types (EVSE, inverter, battery)
//   - mDNS discovery advertising
//   - Commissioning support
//   - Comprehensive logging
//
// Usage:
//
//	mash-device [flags]
//
// Flags:
//
//	-type string        Device type: evse, inverter, battery (default "evse")
//	-config string      Configuration file path
//	-discriminator int  Discriminator for commissioning (0-4095)
//	-setup-code string  8-digit setup code for commissioning
//	-port int           Listen port (default 8443)
//	-log-level string   Log level: debug, info, warn, error (default "info")
//	-simulate           Enable simulation mode with synthetic data
//	-interactive        Enable interactive command mode
//	-state-dir string   Directory for persistent state
//	-reset              Clear all persisted state before starting
//
// Examples:
//
//	# Start EVSE device with default settings
//	mash-device -type evse -discriminator 1234 -setup-code 12345678
//
//	# Start inverter with config file
//	mash-device -type inverter -config /etc/mash/inverter.yaml
//
//	# Start battery in simulation mode
//	mash-device -type battery -simulate -log-level debug
//
//	# Start with persistence (remembers zones across restarts)
//	mash-device -type evse -state-dir /var/lib/mash-device
//
//	# Reset persistent state
//	mash-device -type evse -state-dir /var/lib/mash-device -reset
//
//	# Start in interactive mode (simulation controlled manually)
//	mash-device -type evse -interactive
//
// Interactive Commands:
//
//	start       - Start simulation
//	stop        - Stop simulation
//	power <kw>  - Set power value directly (in kW)
//	status      - Show device status
//	help        - Show available commands
//	quit        - Exit the device
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/cmd/mash-device/interactive"
	"github.com/mash-protocol/mash-go/pkg/cert"
	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/persistence"
	"github.com/mash-protocol/mash-go/pkg/service"
)

// DeviceType represents supported device types.
type DeviceType string

const (
	DeviceTypeEVSE     DeviceType = "evse"
	DeviceTypeInverter DeviceType = "inverter"
	DeviceTypeBattery  DeviceType = "battery"
)

// Config holds the device configuration.
// It implements interactive.DeviceConfig.
type Config struct {
	Type          DeviceType
	ConfigFile    string
	Discriminator uint16
	SetupCode     string
	Port          int
	LogLevel      string
	Simulate      bool
	Interactive   bool

	// Device-specific settings
	SerialNumber string
	Brand        string
	Model        string
	DeviceName   string

	// Persistence settings
	StateDir string
	Reset    bool
}

// DeviceType implements interactive.DeviceConfig.
func (c *Config) DeviceType() interactive.DeviceType {
	return interactive.DeviceType(c.Type)
}

var (
	config        Config
	discriminator uint // Temp var for flag parsing

	// Simulation control
	simCtx       context.Context
	simCancel    context.CancelFunc
	simRunning   bool
	connectedCnt int

	// Device service (for simulation to update attributes)
	deviceSvc *service.DeviceService
)

func init() {
	flag.StringVar((*string)(&config.Type), "type", "evse", "Device type: evse, inverter, battery")
	flag.StringVar(&config.ConfigFile, "config", "", "Configuration file path")
	flag.UintVar(&discriminator, "discriminator", 1234, "Discriminator for commissioning (0-4095)")
	flag.StringVar(&config.SetupCode, "setup-code", "12345678", "8-digit setup code for commissioning")
	flag.IntVar(&config.Port, "port", 8443, "Listen port")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.BoolVar(&config.Simulate, "simulate", true, "Enable simulation mode with synthetic data")
	flag.BoolVar(&config.Interactive, "interactive", false, "Enable interactive command mode")

	flag.StringVar(&config.SerialNumber, "serial", "", "Device serial number (auto-generated if empty)")
	flag.StringVar(&config.Brand, "brand", "MASH Reference", "Device brand/vendor name")
	flag.StringVar(&config.Model, "model", "", "Device model name (auto-generated if empty)")
	flag.StringVar(&config.DeviceName, "name", "", "User-friendly device name")

	flag.StringVar(&config.StateDir, "state-dir", "", "Directory for persistent state")
	flag.BoolVar(&config.Reset, "reset", false, "Clear all persisted state before starting")
}

func main() {
	flag.Parse()
	config.Discriminator = uint16(discriminator)

	// Setup logging
	setupLogging(config.LogLevel)

	log.Println("MASH Reference Device")
	log.Println("=====================")
	log.Printf("Device type: %s", config.Type)
	log.Printf("Discriminator: %d", config.Discriminator)
	log.Printf("Port: %d", config.Port)

	// Validate configuration
	if err := validateConfig(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Apply defaults
	applyDefaults()

	// Create device based on type
	device, deviceCategory := createDevice()
	if device == nil {
		log.Fatalf("Failed to create device of type: %s", config.Type)
	}

	// Create device service
	svcConfig := service.DefaultDeviceConfig()
	svcConfig.Discriminator = config.Discriminator
	svcConfig.SetupCode = config.SetupCode
	svcConfig.SerialNumber = config.SerialNumber
	svcConfig.Brand = config.Brand
	svcConfig.Model = config.Model
	svcConfig.DeviceName = config.DeviceName
	svcConfig.Categories = []discovery.DeviceCategory{deviceCategory}
	svcConfig.ListenAddress = fmt.Sprintf(":%d", config.Port)

	svc, err := service.NewDeviceService(device, svcConfig)
	if err != nil {
		log.Fatalf("Failed to create device service: %v", err)
	}

	// Store for simulation
	deviceSvc = svc

	// Set up persistence if state-dir is provided
	if config.StateDir != "" {
		log.Printf("Using state directory: %s", config.StateDir)

		// Create certificate store
		certStore := cert.NewFileStore(config.StateDir)

		// Create state store
		stateStore := persistence.NewDeviceStateStore(filepath.Join(config.StateDir, "state.json"))

		// Handle --reset flag
		if config.Reset {
			log.Println("Resetting persisted state...")
			if err := stateStore.Clear(); err != nil {
				log.Printf("Warning: Failed to clear state: %v", err)
			}
			// Note: certStore.Clear() would need to be implemented if we want to clear certs too
		}

		// Load existing certs (may fail if first run)
		if err := certStore.Load(); err != nil {
			log.Printf("No existing certificates found (first run or reset): %v", err)
		}

		// Set stores on the service
		svc.SetCertStore(certStore)
		svc.SetStateStore(stateStore)

		// Load state (will restore zone memberships, failsafe timers, etc.)
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

	// Enter commissioning mode
	if err := svc.EnterCommissioningMode(); err != nil {
		log.Printf("Warning: Failed to enter commissioning mode: %v", err)
	} else {
		log.Println("Commissioning mode active")
		printCommissioningInfo()
	}

	// Set up simulation behavior
	if config.Interactive {
		log.Println("Interactive mode enabled - use 'start' to begin simulation")
		id := interactive.New(svc, &config)
		go id.Run(ctx, cancel)
	} else if config.Simulate {
		// Note: Simulation starts automatically when a zone connects (see handleEvent)
		log.Println("Simulation mode enabled (will start when controller connects)")
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down...")

	// Save state before stopping
	if config.StateDir != "" {
		log.Println("Saving state...")
		if err := svc.SaveState(); err != nil {
			log.Printf("Warning: Failed to save state: %v", err)
		}
	}

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

func validateConfig() error {
	if config.Discriminator > 4095 {
		return fmt.Errorf("discriminator must be 0-4095, got %d", config.Discriminator)
	}
	if len(config.SetupCode) != 8 {
		return fmt.Errorf("setup code must be 8 digits, got %d", len(config.SetupCode))
	}
	switch config.Type {
	case DeviceTypeEVSE, DeviceTypeInverter, DeviceTypeBattery:
		// Valid
	default:
		return fmt.Errorf("unknown device type: %s", config.Type)
	}
	return nil
}

func applyDefaults() {
	if config.SerialNumber == "" {
		config.SerialNumber = fmt.Sprintf("%s-%d", config.Type, time.Now().Unix()%10000)
	}
	if config.Model == "" {
		switch config.Type {
		case DeviceTypeEVSE:
			config.Model = "Reference EVSE 22kW"
		case DeviceTypeInverter:
			config.Model = "Reference Inverter 10kW"
		case DeviceTypeBattery:
			config.Model = "Reference Battery 10kWh"
		}
	}
	if config.DeviceName == "" {
		config.DeviceName = fmt.Sprintf("MASH %s", config.Type)
	}
}

func createDevice() (*model.Device, discovery.DeviceCategory) {
	switch config.Type {
	case DeviceTypeEVSE:
		evse := examples.NewEVSE(examples.EVSEConfig{
			DeviceID:              config.SerialNumber,
			VendorName:            config.Brand,
			ProductName:           config.Model,
			SerialNumber:          config.SerialNumber,
			VendorID:              0x1234,
			ProductID:             0x0001,
			PhaseCount:            3,
			NominalVoltage:        230,
			MaxCurrentPerPhase:    32000,
			MinCurrentPerPhase:    6000,
			NominalMaxPower:       22000000,
			NominalMinPower:       1380000,
			SupportsBidirectional: false,
		})
		return evse.Device(), discovery.CategoryEMobility

	case DeviceTypeInverter:
		device := createInverterDevice()
		return device, discovery.CategoryInverter

	case DeviceTypeBattery:
		device := createBatteryDevice()
		return device, discovery.CategoryInverter

	default:
		return nil, 0
	}
}

func createInverterDevice() *model.Device {
	device := model.NewDevice(config.SerialNumber, 0x1234, 0x0002)

	// Root endpoint with DeviceInfo
	deviceInfo := features.NewDeviceInfo()
	_ = deviceInfo.SetDeviceID(config.SerialNumber)
	_ = deviceInfo.SetVendorName(config.Brand)
	_ = deviceInfo.SetProductName(config.Model)
	_ = deviceInfo.SetSerialNumber(config.SerialNumber)
	_ = deviceInfo.SetSoftwareVersion("1.0.0")
	device.RootEndpoint().AddFeature(deviceInfo.Feature)

	// Inverter endpoint
	inverter := model.NewEndpoint(1, model.EndpointInverter, "PV Inverter")

	// Electrical capabilities
	electrical := features.NewElectrical()
	_ = electrical.SetPhaseCount(3)
	_ = electrical.SetNominalVoltage(230)
	_ = electrical.SetNominalFrequency(50)
	_ = electrical.SetNominalMaxProduction(10000000) // 10 kW
	_ = electrical.SetSupportedDirections(features.DirectionProduction)
	inverter.AddFeature(electrical.Feature)

	// Measurement
	measurement := features.NewMeasurement()
	inverter.AddFeature(measurement.Feature)

	// Status
	status := features.NewStatus()
	_ = status.SetOperatingState(features.OperatingStateRunning)
	inverter.AddFeature(status.Feature)

	_ = device.AddEndpoint(inverter)

	return device
}

func createBatteryDevice() *model.Device {
	device := model.NewDevice(config.SerialNumber, 0x1234, 0x0003)

	// Root endpoint with DeviceInfo
	deviceInfo := features.NewDeviceInfo()
	_ = deviceInfo.SetDeviceID(config.SerialNumber)
	_ = deviceInfo.SetVendorName(config.Brand)
	_ = deviceInfo.SetProductName(config.Model)
	_ = deviceInfo.SetSerialNumber(config.SerialNumber)
	_ = deviceInfo.SetSoftwareVersion("1.0.0")
	device.RootEndpoint().AddFeature(deviceInfo.Feature)

	// Battery endpoint
	battery := model.NewEndpoint(1, model.EndpointBattery, "Battery Storage")

	// Electrical capabilities
	electrical := features.NewElectrical()
	_ = electrical.SetPhaseCount(3)
	_ = electrical.SetNominalVoltage(230)
	_ = electrical.SetNominalFrequency(50)
	_ = electrical.SetNominalMaxConsumption(5000000)  // 5 kW charge
	_ = electrical.SetNominalMaxProduction(5000000)   // 5 kW discharge
	_ = electrical.SetSupportedDirections(features.DirectionBidirectional)
	battery.AddFeature(electrical.Feature)

	// Measurement
	measurement := features.NewMeasurement()
	battery.AddFeature(measurement.Feature)

	// EnergyControl
	energyControl := features.NewEnergyControl()
	_ = energyControl.SetDeviceType(features.DeviceTypeBattery)
	_ = energyControl.SetControlState(features.ControlStateAutonomous)
	energyControl.SetCapabilities(true, false, true, false, true, true, true)
	battery.AddFeature(energyControl.Feature)

	// Status
	status := features.NewStatus()
	_ = status.SetOperatingState(features.OperatingStateStandby)
	battery.AddFeature(status.Feature)

	_ = device.AddEndpoint(battery)

	return device
}

func handleEvent(event service.Event) {
	switch event.Type {
	case service.EventConnected:
		log.Printf("[EVENT] Zone connected: %s", event.ZoneID)
		connectedCnt++
		// Start simulation on first connection (unless in interactive mode)
		if config.Simulate && !config.Interactive && !simRunning && connectedCnt == 1 {
			startSimulation()
		}

	case service.EventDisconnected:
		log.Printf("[EVENT] Zone disconnected: %s", event.ZoneID)
		connectedCnt--
		// Stop simulation when all zones disconnect
		if connectedCnt <= 0 {
			connectedCnt = 0
			stopSimulation()
		}

	case service.EventCommissioningOpened:
		log.Println("[EVENT] Commissioning window opened")

	case service.EventCommissioningClosed:
		log.Println("[EVENT] Commissioning window closed")

	case service.EventFailsafeTriggered:
		log.Printf("[EVENT] FAILSAFE triggered for zone %s!", event.ZoneID)
		// Stop simulation on failsafe - device should go to safe state
		stopSimulation()

	case service.EventFailsafeCleared:
		log.Printf("[EVENT] Failsafe cleared for zone %s", event.ZoneID)
		// Resume simulation if still connected
		if config.Simulate && !simRunning && connectedCnt > 0 {
			startSimulation()
		}

	case service.EventValueChanged:
		log.Printf("[EVENT] Value changed (zone: %s)", event.ZoneID)
	}
}

func startSimulation() {
	if simRunning {
		return
	}
	simCtx, simCancel = context.WithCancel(context.Background())
	simRunning = true
	go runSimulation(simCtx, config.Type)
	log.Println("[SIM] Simulation started")
}

func stopSimulation() {
	if !simRunning {
		return
	}
	if simCancel != nil {
		simCancel()
	}
	simRunning = false
	log.Println("[SIM] Simulation stopped")
}

func printCommissioningInfo() {
	qrString := fmt.Sprintf("MASH:1:%d:%s", config.Discriminator, config.SetupCode)

	log.Println("")
	log.Println("============================================")
	log.Println("         COMMISSIONING INFORMATION          ")
	log.Println("============================================")
	log.Printf("QR Code String: %s", qrString)
	log.Println("")
	log.Printf("  Discriminator: %d", config.Discriminator)
	log.Printf("  Setup Code:    %s", config.SetupCode)
	log.Printf("  Port:          %d", config.Port)
	log.Println("============================================")
	log.Println("")
}

func runSimulation(ctx context.Context, deviceType DeviceType) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var power int64

	// Attribute IDs from features package
	const (
		attrACActivePower = uint16(1)  // features.MeasurementAttrACActivePower
		attrDCPower       = uint16(40) // features.MeasurementAttrDCPower
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var attrID uint16
			switch deviceType {
			case DeviceTypeEVSE:
				// Simulate varying charging power
				power = (power + 1000000) % 22000000
				if power == 0 {
					power = 1380000
				}
				attrID = attrACActivePower
				log.Printf("[SIM] EVSE charging at %.1f kW", float64(power)/1000000)

			case DeviceTypeInverter:
				// Simulate varying PV production based on time
				hour := time.Now().Hour()
				if hour >= 6 && hour <= 20 {
					// Daytime - produce power (negative = production)
					power = -int64((10 - abs(hour-13)) * 1000000)
				} else {
					power = 0
				}
				attrID = attrACActivePower
				log.Printf("[SIM] Inverter producing %.1f kW", float64(-power)/1000000)

			case DeviceTypeBattery:
				// Simulate charge/discharge cycles
				power = (power + 500000) % 10000000 - 5000000
				attrID = attrDCPower
				if power > 0 {
					log.Printf("[SIM] Battery charging at %.1f kW", float64(power)/1000000)
				} else if power < 0 {
					log.Printf("[SIM] Battery discharging at %.1f kW", float64(-power)/1000000)
				} else {
					log.Println("[SIM] Battery idle")
				}
			}

			// Update the attribute and notify subscribed zones
			// Endpoint 1 = functional endpoint, FeatureMeasurement = 0x0002
			if deviceSvc != nil && attrID != 0 {
				if err := deviceSvc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrID, power); err != nil {
					log.Printf("[SIM] Failed to notify attribute change: %v", err)
				}
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}


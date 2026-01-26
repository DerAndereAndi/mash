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
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
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
type Config struct {
	Type          DeviceType
	ConfigFile    string
	Discriminator uint16
	SetupCode     string
	Port          int
	LogLevel      string
	Simulate      bool

	// Device-specific settings
	SerialNumber string
	Brand        string
	Model        string
	DeviceName   string
}

var (
	config        Config
	discriminator uint // Temp var for flag parsing
)

func init() {
	flag.StringVar((*string)(&config.Type), "type", "evse", "Device type: evse, inverter, battery")
	flag.StringVar(&config.ConfigFile, "config", "", "Configuration file path")
	flag.UintVar(&discriminator, "discriminator", 1234, "Discriminator for commissioning (0-4095)")
	flag.StringVar(&config.SetupCode, "setup-code", "12345678", "8-digit setup code for commissioning")
	flag.IntVar(&config.Port, "port", 8443, "Listen port")
	flag.StringVar(&config.LogLevel, "log-level", "info", "Log level: debug, info, warn, error")
	flag.BoolVar(&config.Simulate, "simulate", true, "Enable simulation mode with synthetic data")

	flag.StringVar(&config.SerialNumber, "serial", "", "Device serial number (auto-generated if empty)")
	flag.StringVar(&config.Brand, "brand", "MASH Reference", "Device brand/vendor name")
	flag.StringVar(&config.Model, "model", "", "Device model name (auto-generated if empty)")
	flag.StringVar(&config.DeviceName, "name", "", "User-friendly device name")
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

	// Start simulation if enabled
	if config.Simulate {
		go runSimulation(ctx, config.Type)
	}

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal: %v", sig)
	log.Println("Shutting down...")

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
	case service.EventDisconnected:
		log.Printf("[EVENT] Zone disconnected: %s", event.ZoneID)
	case service.EventCommissioningOpened:
		log.Println("[EVENT] Commissioning window opened")
	case service.EventCommissioningClosed:
		log.Println("[EVENT] Commissioning window closed")
	case service.EventFailsafeTriggered:
		log.Printf("[EVENT] FAILSAFE triggered for zone %s!", event.ZoneID)
	case service.EventFailsafeCleared:
		log.Printf("[EVENT] Failsafe cleared for zone %s", event.ZoneID)
	case service.EventValueChanged:
		log.Printf("[EVENT] Value changed (zone: %s)", event.ZoneID)
	}
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
	log.Println("Simulation mode enabled")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var power int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			switch deviceType {
			case DeviceTypeEVSE:
				// Simulate varying charging power
				power = (power + 1000000) % 22000000
				if power == 0 {
					power = 1380000
				}
				log.Printf("[SIM] EVSE charging at %.1f kW", float64(power)/1000000)

			case DeviceTypeInverter:
				// Simulate varying PV production based on time
				hour := time.Now().Hour()
				if hour >= 6 && hour <= 20 {
					// Daytime - produce power
					power = int64((10 - abs(hour-13)) * 1000000)
				} else {
					power = 0
				}
				log.Printf("[SIM] Inverter producing %.1f kW", float64(power)/1000000)

			case DeviceTypeBattery:
				// Simulate charge/discharge cycles
				power = (power + 500000) % 10000000 - 5000000
				if power > 0 {
					log.Printf("[SIM] Battery charging at %.1f kW", float64(power)/1000000)
				} else if power < 0 {
					log.Printf("[SIM] Battery discharging at %.1f kW", float64(-power)/1000000)
				} else {
					log.Println("[SIM] Battery idle")
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

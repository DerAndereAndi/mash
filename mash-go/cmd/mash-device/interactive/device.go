// Package interactive provides the interactive command-line interface
// for the MASH device.
package interactive

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/pkg/inspect"
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

// DeviceConfig provides configuration information to the interactive device.
// This interface allows the interactive layer to access device settings
// without depending on the main package's config structure.
type DeviceConfig interface {
	// DeviceType returns the type of device (evse, inverter, battery).
	DeviceType() DeviceType
}

// Device handles interactive mode for mash-device.
type Device struct {
	svc       *service.DeviceService
	config    DeviceConfig
	inspector *inspect.Inspector
	formatter *inspect.Formatter

	// Simulation control
	simCtx     context.Context
	simCancel  context.CancelFunc
	simRunning bool
}

// New creates a new interactive device handler.
func New(svc *service.DeviceService, cfg DeviceConfig) *Device {
	return &Device{
		svc:       svc,
		config:    cfg,
		inspector: inspect.NewInspector(svc.Device()),
		formatter: inspect.NewFormatter(),
	}
}

// Run starts the interactive command loop.
func (d *Device) Run(ctx context.Context, cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)

	d.printHelp()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Print("\ndevice> ")
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
			d.printHelp()

		case "inspect", "i":
			d.cmdInspect(args)

		case "read", "r":
			d.cmdRead(args)

		case "write", "w":
			d.cmdWrite(args)

		case "zones", "z":
			d.cmdZones(args)

		case "kick":
			d.cmdKick(args)

		case "start", "sim-start":
			d.cmdStart()

		case "stop", "sim-stop":
			d.cmdStop()

		case "power":
			d.cmdPower(args)

		case "status":
			d.cmdStatus()

		case "quit", "exit", "q":
			fmt.Println("Exiting...")
			cancel()
			return

		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func (d *Device) printHelp() {
	fmt.Println(`
MASH Device Commands:
  Inspection:
    inspect [path]     - Inspect device structure (or specific endpoint/feature)
    read <path>        - Read an attribute value
    write <path> <val> - Write an attribute value

  Zone Management:
    zones              - List connected zones
    kick <zone-id>     - Remove a zone from this device

  Simulation:
    start              - Start simulation
    stop               - Stop simulation
    power <kw>         - Set power value (kW, positive=consume, negative=produce)
    status             - Show device status

  General:
    help               - Show this help
    quit               - Exit device

  Path Format:
    endpoint/feature/attribute - e.g., 1/measurement/acActivePower
    Can use IDs or names: 1/2/1 or evCharger/measurement/acActivePower`)
}

// cmdInspect handles the inspect command.
func (d *Device) cmdInspect(args []string) {
	if len(args) == 0 {
		// Show full device tree
		tree := d.inspector.InspectDevice()
		fmt.Print(d.inspector.FormatDeviceTree(tree, d.formatter))
		return
	}

	// Parse path
	path, err := inspect.ParsePath(args[0])
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		if path.FeatureID == 0 {
			// Endpoint only
			epInfo, err := d.inspector.InspectEndpoint(path.EndpointID)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Print(d.inspector.FormatEndpoint(epInfo, d.formatter))
		} else {
			// Endpoint and feature
			featInfo, err := d.inspector.InspectFeature(path.EndpointID, path.FeatureID)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Print(d.inspector.FormatFeature(featInfo, d.formatter))
		}
	} else {
		// Full path - show single attribute
		value, meta, err := d.inspector.ReadAttribute(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		valueStr := d.formatter.FormatValue(value, meta.Unit)
		fmt.Printf("%s = %s\n", meta.Name, valueStr)
	}
}

// cmdRead handles the read command.
func (d *Device) cmdRead(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: read <path>")
		fmt.Println("  Example: read 1/measurement/acActivePower")
		return
	}

	path, err := inspect.ParsePath(args[0])
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		// Read all attributes for the feature
		attrs, err := d.inspector.ReadAllAttributes(path.EndpointID, path.FeatureID)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		for attrID, value := range attrs {
			name := inspect.GetAttributeName(path.FeatureID, attrID)
			if name == "" {
				name = fmt.Sprintf("attr_%d", attrID)
			}
			fmt.Printf("  %s: %v\n", name, value)
		}
	} else {
		// Read single attribute
		value, meta, err := d.inspector.ReadAttribute(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		valueStr := d.formatter.FormatValue(value, meta.Unit)
		fmt.Printf("%s = %s\n", meta.Name, valueStr)
	}
}

// cmdWrite handles the write command.
func (d *Device) cmdWrite(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: write <path> <value>")
		fmt.Println("  Example: write 0/deviceInfo/label \"My EVSE\"")
		return
	}

	path, err := inspect.ParsePath(args[0])
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	// Parse the value (try int, then float, then string)
	valueStr := strings.Join(args[1:], " ")
	var value any

	// Try int64 first
	if v, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		value = v
	} else if v, err := strconv.ParseFloat(valueStr, 64); err == nil {
		value = v
	} else if v, err := strconv.ParseBool(valueStr); err == nil {
		value = v
	} else {
		// Treat as string (strip quotes if present)
		value = strings.Trim(valueStr, "\"'")
	}

	if err := d.inspector.WriteAttribute(path, value); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	fmt.Println("OK")
}

// cmdZones handles the zones command.
func (d *Device) cmdZones(_ []string) {
	zones := d.svc.GetAllZones()
	if len(zones) == 0 {
		fmt.Println("No zones connected")
		return
	}

	fmt.Printf("\nConnected Zones (%d):\n", len(zones))
	fmt.Println("-------------------------------------------")
	for _, z := range zones {
		status := "connected"
		if !z.Connected {
			status = "disconnected"
		}
		if z.FailsafeActive {
			status = "FAILSAFE"
		}
		fmt.Printf("  ID: %s\n", z.ID)
		fmt.Printf("      Type: %s (priority %d)\n", z.Type.String(), z.Priority)
		fmt.Printf("      Status: %s\n", status)
		fmt.Printf("      Last seen: %s\n", z.LastSeen.Format("15:04:05"))
		fmt.Println()
	}
}

// cmdKick handles the kick command (removes a zone).
func (d *Device) cmdKick(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: kick <zone-id>")
		fmt.Println("  Use 'zones' to list zone IDs")
		return
	}

	zoneID := args[0]

	// Try exact match first
	zone := d.svc.GetZone(zoneID)
	if zone == nil {
		// Try partial match
		for _, z := range d.svc.GetAllZones() {
			if strings.Contains(z.ID, zoneID) {
				zoneID = z.ID
				zone = z
				break
			}
		}
	}

	if zone == nil {
		fmt.Printf("Zone not found: %s\n", args[0])
		return
	}

	fmt.Printf("Removing zone %s...\n", zoneID)
	if err := d.svc.RemoveZone(zoneID); err != nil {
		fmt.Printf("Failed to remove zone: %v\n", err)
		return
	}

	fmt.Println("Zone removed")
}

// cmdStart starts the simulation.
func (d *Device) cmdStart() {
	if d.simRunning {
		fmt.Println("Simulation already running")
		return
	}
	d.startSimulation()
	fmt.Println("Simulation started")
}

// cmdStop stops the simulation.
func (d *Device) cmdStop() {
	if !d.simRunning {
		fmt.Println("Simulation not running")
		return
	}
	d.stopSimulation()
	fmt.Println("Simulation stopped")
}

// cmdPower sets the power directly.
func (d *Device) cmdPower(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: power <kw>")
		fmt.Println("  Positive values = consumption/charging")
		fmt.Println("  Negative values = production/discharging")
		return
	}

	powerKW, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		fmt.Printf("Invalid power value: %v\n", err)
		return
	}

	powerMW := int64(powerKW * 1_000_000)
	d.setPowerDirect(powerMW)
}

// cmdStatus shows the device status.
func (d *Device) cmdStatus() {
	fmt.Println("\nDevice Status")
	fmt.Println("-------------------------------------------")
	fmt.Printf("  Device ID:      %s\n", d.svc.Device().DeviceID())
	fmt.Printf("  Device Type:    %s\n", d.config.DeviceType())
	fmt.Printf("  Service State:  %s\n", d.svc.State())
	fmt.Printf("  Connected Zones: %d\n", d.svc.ZoneCount())

	simStatus := "stopped"
	if d.simRunning {
		simStatus = "running"
	}
	fmt.Printf("  Simulation:     %s\n", simStatus)

	// Read current power if available
	path := &inspect.Path{
		EndpointID:  1,
		FeatureID:   uint8(model.FeatureMeasurement),
		AttributeID: 1, // acActivePower
	}
	if value, meta, err := d.inspector.ReadAttribute(path); err == nil {
		fmt.Printf("  Current Power:  %s\n", d.formatter.FormatValue(value, meta.Unit))
	}

	fmt.Println()
}

// startSimulation starts the background simulation.
func (d *Device) startSimulation() {
	if d.simRunning {
		return
	}
	d.simCtx, d.simCancel = context.WithCancel(context.Background())
	d.simRunning = true
	go d.runSimulation(d.simCtx)
}

// stopSimulation stops the background simulation.
func (d *Device) stopSimulation() {
	if !d.simRunning {
		return
	}
	if d.simCancel != nil {
		d.simCancel()
	}
	d.simRunning = false
}

// setPowerDirect sets the power value directly.
func (d *Device) setPowerDirect(powerMW int64) {
	// Attribute IDs from features package
	const (
		attrACActivePower = uint16(1)  // features.MeasurementAttrACActivePower
		attrDCPower       = uint16(40) // features.MeasurementAttrDCPower
	)

	var attrID uint16
	switch d.config.DeviceType() {
	case DeviceTypeEVSE, DeviceTypeInverter:
		attrID = attrACActivePower
	case DeviceTypeBattery:
		attrID = attrDCPower
	}

	if err := d.svc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrID, powerMW); err != nil {
		fmt.Printf("Failed to set power: %v\n", err)
		return
	}

	fmt.Printf("Power set to %.1f kW\n", float64(powerMW)/1_000_000)
}

// runSimulation runs the background simulation loop.
func (d *Device) runSimulation(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var power int64

	// Attribute IDs from features package
	const (
		attrACActivePower = uint16(1)  // features.MeasurementAttrACActivePower
		attrDCPower       = uint16(40) // features.MeasurementAttrDCPower
	)

	deviceType := d.config.DeviceType()

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

			case DeviceTypeBattery:
				// Simulate charge/discharge cycles
				power = (power + 500000) % 10000000 - 5000000
				attrID = attrDCPower
			}

			// Update the attribute and notify subscribed zones
			if attrID != 0 {
				if err := d.svc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrID, power); err != nil {
					// Silently ignore errors in simulation
					_ = err
				}
			}
		}
	}
}

// IsRunning returns whether simulation is running (for external access).
func (d *Device) IsRunning() bool {
	return d.simRunning
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

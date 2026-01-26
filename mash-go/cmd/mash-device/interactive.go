package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mash-protocol/mash-go/pkg/inspect"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/service"
)

// InteractiveDevice handles interactive mode for mash-device.
type InteractiveDevice struct {
	svc       *service.DeviceService
	inspector *inspect.Inspector
	formatter *inspect.Formatter

	// Simulation control
	simCtx     context.Context
	simCancel  context.CancelFunc
	simRunning bool
}

// NewInteractiveDevice creates a new interactive device handler.
func NewInteractiveDevice(svc *service.DeviceService) *InteractiveDevice {
	return &InteractiveDevice{
		svc:       svc,
		inspector: inspect.NewInspector(svc.Device()),
		formatter: inspect.NewFormatter(),
	}
}

// Run starts the interactive command loop.
func (i *InteractiveDevice) Run(ctx context.Context, cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)

	i.printHelp()

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
			i.printHelp()

		case "inspect", "i":
			i.cmdInspect(args)

		case "read", "r":
			i.cmdRead(args)

		case "write", "w":
			i.cmdWrite(args)

		case "zones", "z":
			i.cmdZones(args)

		case "kick":
			i.cmdKick(args)

		case "start", "sim-start":
			i.cmdStart()

		case "stop", "sim-stop":
			i.cmdStop()

		case "power":
			i.cmdPower(args)

		case "status":
			i.cmdStatus()

		case "quit", "exit", "q":
			fmt.Println("Exiting...")
			cancel()
			return

		default:
			fmt.Printf("Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func (i *InteractiveDevice) printHelp() {
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
func (i *InteractiveDevice) cmdInspect(args []string) {
	if len(args) == 0 {
		// Show full device tree
		tree := i.inspector.InspectDevice()
		fmt.Print(i.inspector.FormatDeviceTree(tree, i.formatter))
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
			epInfo, err := i.inspector.InspectEndpoint(path.EndpointID)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Print(i.inspector.FormatEndpoint(epInfo, i.formatter))
		} else {
			// Endpoint and feature
			featInfo, err := i.inspector.InspectFeature(path.EndpointID, path.FeatureID)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}
			fmt.Print(i.inspector.FormatFeature(featInfo, i.formatter))
		}
	} else {
		// Full path - show single attribute
		value, meta, err := i.inspector.ReadAttribute(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		valueStr := i.formatter.FormatValue(value, meta.Unit)
		fmt.Printf("%s = %s\n", meta.Name, valueStr)
	}
}

// cmdRead handles the read command.
func (i *InteractiveDevice) cmdRead(args []string) {
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
		attrs, err := i.inspector.ReadAllAttributes(path.EndpointID, path.FeatureID)
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
		value, meta, err := i.inspector.ReadAttribute(path)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		valueStr := i.formatter.FormatValue(value, meta.Unit)
		fmt.Printf("%s = %s\n", meta.Name, valueStr)
	}
}

// cmdWrite handles the write command.
func (i *InteractiveDevice) cmdWrite(args []string) {
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

	if err := i.inspector.WriteAttribute(path, value); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	fmt.Println("OK")
}

// cmdZones handles the zones command.
func (i *InteractiveDevice) cmdZones(_ []string) {
	zones := i.svc.GetAllZones()
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
func (i *InteractiveDevice) cmdKick(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: kick <zone-id>")
		fmt.Println("  Use 'zones' to list zone IDs")
		return
	}

	zoneID := args[0]

	// Try exact match first
	zone := i.svc.GetZone(zoneID)
	if zone == nil {
		// Try partial match
		for _, z := range i.svc.GetAllZones() {
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
	if err := i.svc.RemoveZone(zoneID); err != nil {
		fmt.Printf("Failed to remove zone: %v\n", err)
		return
	}

	fmt.Println("Zone removed")
}

// cmdStart starts the simulation.
func (i *InteractiveDevice) cmdStart() {
	if i.simRunning {
		fmt.Println("Simulation already running")
		return
	}
	i.startSimulation()
	fmt.Println("Simulation started")
}

// cmdStop stops the simulation.
func (i *InteractiveDevice) cmdStop() {
	if !i.simRunning {
		fmt.Println("Simulation not running")
		return
	}
	i.stopSimulation()
	fmt.Println("Simulation stopped")
}

// cmdPower sets the power directly.
func (i *InteractiveDevice) cmdPower(args []string) {
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
	i.setPowerDirect(powerMW)
}

// cmdStatus shows the device status.
func (i *InteractiveDevice) cmdStatus() {
	fmt.Println("\nDevice Status")
	fmt.Println("-------------------------------------------")
	fmt.Printf("  Device ID:      %s\n", i.svc.Device().DeviceID())
	fmt.Printf("  Service State:  %s\n", i.svc.State())
	fmt.Printf("  Connected Zones: %d\n", i.svc.ZoneCount())

	simStatus := "stopped"
	if i.simRunning {
		simStatus = "running"
	}
	fmt.Printf("  Simulation:     %s\n", simStatus)

	// Read current power if available
	path := &inspect.Path{
		EndpointID:  1,
		FeatureID:   uint8(model.FeatureMeasurement),
		AttributeID: 1, // acActivePower
	}
	if value, meta, err := i.inspector.ReadAttribute(path); err == nil {
		fmt.Printf("  Current Power:  %s\n", i.formatter.FormatValue(value, meta.Unit))
	}

	fmt.Println()
}

// startSimulation starts the background simulation.
func (i *InteractiveDevice) startSimulation() {
	if i.simRunning {
		return
	}
	i.simCtx, i.simCancel = context.WithCancel(context.Background())
	i.simRunning = true
	go runSimulation(i.simCtx, config.Type)
}

// stopSimulation stops the background simulation.
func (i *InteractiveDevice) stopSimulation() {
	if !i.simRunning {
		return
	}
	if i.simCancel != nil {
		i.simCancel()
	}
	i.simRunning = false
}

// setPowerDirect sets the power value directly.
func (i *InteractiveDevice) setPowerDirect(powerMW int64) {
	// Attribute IDs from features package
	const (
		attrACActivePower = uint16(1)  // features.MeasurementAttrACActivePower
		attrDCPower       = uint16(40) // features.MeasurementAttrDCPower
	)

	var attrID uint16
	switch config.Type {
	case DeviceTypeEVSE, DeviceTypeInverter:
		attrID = attrACActivePower
	case DeviceTypeBattery:
		attrID = attrDCPower
	}

	if err := i.svc.NotifyAttributeChange(1, uint8(model.FeatureMeasurement), attrID, powerMW); err != nil {
		fmt.Printf("Failed to set power: %v\n", err)
		return
	}

	fmt.Printf("Power set to %.1f kW\n", float64(powerMW)/1_000_000)
}

// IsRunning returns whether simulation is running (for external access).
func (i *InteractiveDevice) IsRunning() bool {
	return i.simRunning
}

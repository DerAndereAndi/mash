package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/inspect"
	"github.com/mash-protocol/mash-go/pkg/service"
)

// InteractiveController handles interactive mode for mash-controller.
type InteractiveController struct {
	svc       *service.ControllerService
	cem       *examples.CEM
	formatter *inspect.Formatter
}

// NewInteractiveController creates a new interactive controller handler.
func NewInteractiveController(svc *service.ControllerService, cem *examples.CEM) *InteractiveController {
	return &InteractiveController{
		svc:       svc,
		cem:       cem,
		formatter: inspect.NewFormatter(),
	}
}

// Run starts the interactive command loop.
func (i *InteractiveController) Run(ctx context.Context, cancel context.CancelFunc) {
	reader := bufio.NewReader(os.Stdin)

	i.printHelp()

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
			i.printHelp()

		case "discover":
			i.cmdDiscover(ctx)

		case "list", "ls", "devices":
			i.cmdDevices()

		case "commission":
			i.cmdCommission(ctx, args)

		case "decommission":
			i.cmdDecommission(args)

		case "inspect", "i":
			i.cmdInspect(ctx, args)

		case "read", "r":
			i.cmdRead(ctx, args)

		case "write", "w":
			i.cmdWrite(ctx, args)

		case "limit":
			i.cmdLimit(ctx, args)

		case "clear":
			i.cmdClear(ctx, args)

		case "pause":
			i.cmdPause(ctx, args)

		case "resume":
			i.cmdResume(ctx, args)

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

func (i *InteractiveController) printHelp() {
	fmt.Println(`
MASH Controller Commands:
  Discovery & Connection:
    discover                          - Discover commissionable devices
    devices                           - List connected devices
    commission <discriminator> <code> - Commission a device
    decommission <device-id>          - Remove a device

  Inspection:
    inspect <device-id> [path]        - Inspect device structure
    read <device-id>/<path>           - Read an attribute value
    write <device-id>/<path> <value>  - Write an attribute value

  Control:
    limit <device-id> <power-kw>      - Set power limit (kW)
    clear <device-id>                 - Clear power limit
    pause <device-id>                 - Pause device
    resume <device-id>                - Resume device

  General:
    status                            - Show controller status
    help                              - Show this help
    quit                              - Exit controller

  Path Format:
    endpoint/feature/attribute - e.g., 1/measurement/acActivePower`)
}

// cmdDiscover handles the discover command.
func (i *InteractiveController) cmdDiscover(ctx context.Context) {
	fmt.Println("Discovering commissionable devices...")
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	devices, err := i.svc.Discover(discoverCtx, nil)
	cancel()
	if err != nil {
		fmt.Printf("Discovery error: %v\n", err)
		return
	}
	if len(devices) == 0 {
		fmt.Println("No commissionable devices found")
		return
	}

	fmt.Printf("Found %d commissionable device(s):\n", len(devices))
	for idx, d := range devices {
		fmt.Printf("  %d. %s (discriminator: %d, host: %s:%d)\n",
			idx+1, d.InstanceName, d.Discriminator, d.Host, d.Port)
	}
}

// cmdDevices handles the devices/list command.
func (i *InteractiveController) cmdDevices() {
	devices := i.svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Println("No devices connected")
		return
	}

	fmt.Printf("\nConnected Devices (%d):\n", len(devices))
	fmt.Println("-------------------------------------------")
	for _, d := range devices {
		status := "connected"
		if !d.Connected {
			status = "disconnected"
		}
		fmt.Printf("  ID: %s\n", d.ID)
		fmt.Printf("      Host: %s:%d\n", d.Host, d.Port)
		fmt.Printf("      Type: %s\n", d.DeviceType)
		fmt.Printf("      Status: %s\n", status)
		fmt.Printf("      Last seen: %s\n", d.LastSeen.Format("15:04:05"))
		fmt.Println()
	}
}

// cmdCommission handles the commission command.
func (i *InteractiveController) cmdCommission(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: commission <discriminator> <setup-code>")
		return
	}

	disc, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		fmt.Printf("Invalid discriminator: %v\n", err)
		return
	}

	fmt.Printf("Looking for device with discriminator %d...\n", disc)

	device, err := i.svc.DiscoverByDiscriminator(ctx, uint16(disc))
	if err != nil {
		fmt.Printf("Device not found: %v\n", err)
		return
	}

	fmt.Printf("Found device: %s at %s:%d\n", device.InstanceName, device.Host, device.Port)
	fmt.Println("Commissioning...")

	commissioned, err := i.svc.Commission(ctx, device, args[1])
	if err != nil {
		fmt.Printf("Commissioning failed: %v\n", err)
		return
	}

	fmt.Printf("Device commissioned successfully: %s\n", commissioned.ID)
}

// cmdDecommission handles the decommission command.
func (i *InteractiveController) cmdDecommission(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: decommission <device-id>")
		fmt.Println("  Use 'devices' to list device IDs")
		return
	}

	deviceID := i.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	fmt.Printf("Decommissioning device %s...\n", deviceID)

	if err := i.svc.Decommission(deviceID); err != nil {
		fmt.Printf("Failed to decommission: %v\n", err)
		return
	}

	// Also remove from CEM
	_ = i.cem.DisconnectDevice(deviceID)

	fmt.Println("Device decommissioned")
}

// cmdInspect handles the inspect command.
func (i *InteractiveController) cmdInspect(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: inspect <device-id> [path]")
		fmt.Println("  Examples:")
		fmt.Println("    inspect evse-1234        - Show device overview")
		fmt.Println("    inspect evse-1234/1      - Show endpoint 1")
		fmt.Println("    inspect evse-1234/1/2    - Show feature on endpoint 1")
		return
	}

	// Parse device ID and optional path
	input := args[0]
	deviceID, pathStr := i.parseDevicePath(input)

	deviceID = i.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	session := i.svc.GetSession(deviceID)
	if session == nil {
		fmt.Printf("No active session for device %s\n", deviceID)
		return
	}

	ri := inspect.NewRemoteInspector(session)

	if pathStr == "" {
		// Show device overview by reading DeviceInfo
		i.showDeviceOverview(ctx, ri, deviceID)
		return
	}

	// Parse the path for specific inspection
	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		// Read all attributes for the endpoint/feature
		attrs, err := ri.ReadAllAttributes(ctx, path.EndpointID, path.FeatureID)
		if err != nil {
			fmt.Printf("Failed to read attributes: %v\n", err)
			return
		}

		fmt.Printf("\nEndpoint %d / Feature %d:\n", path.EndpointID, path.FeatureID)
		fmt.Println("-------------------------------------------")
		for attrID, value := range attrs {
			name := inspect.GetAttributeName(path.FeatureID, attrID)
			if name == "" {
				name = fmt.Sprintf("attr_%d", attrID)
			}
			fmt.Printf("  %s: %v\n", name, value)
		}
	} else {
		// Read single attribute
		value, err := ri.ReadAttribute(ctx, path)
		if err != nil {
			fmt.Printf("Failed to read attribute: %v\n", err)
			return
		}
		fmt.Printf("%s = %v\n", path.Raw, value)
	}
}

// showDeviceOverview displays high-level device information.
func (i *InteractiveController) showDeviceOverview(ctx context.Context, ri *inspect.RemoteInspector, deviceID string) {
	fmt.Printf("\nDevice: %s\n", deviceID)
	fmt.Println("-------------------------------------------")

	// Read DeviceInfo from endpoint 0
	attrs, err := ri.ReadAllAttributes(ctx, 0, 1) // Endpoint 0, Feature DeviceInfo (0x01)
	if err != nil {
		fmt.Printf("Failed to read device info: %v\n", err)
		return
	}

	// Display key attributes
	if v, ok := attrs[1]; ok { // vendorName
		fmt.Printf("  Vendor: %v\n", v)
	}
	if v, ok := attrs[2]; ok { // productName
		fmt.Printf("  Product: %v\n", v)
	}
	if v, ok := attrs[3]; ok { // serialNumber
		fmt.Printf("  Serial: %v\n", v)
	}
	if v, ok := attrs[6]; ok { // firmwareVersion
		fmt.Printf("  Firmware: %v\n", v)
	}
	if v, ok := attrs[10]; ok { // endpointCount
		fmt.Printf("  Endpoints: %v\n", v)
	}
	fmt.Println()
}

// cmdRead handles the read command.
func (i *InteractiveController) cmdRead(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: read <device-id>/<endpoint>/<feature>/<attribute>")
		fmt.Println("  Example: read evse-1234/1/measurement/acActivePower")
		return
	}

	deviceID, pathStr := i.parseDevicePath(args[0])

	deviceID = i.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Printf("Device not found\n")
		return
	}

	if pathStr == "" {
		fmt.Println("Path required: <endpoint>/<feature>/<attribute>")
		return
	}

	session := i.svc.GetSession(deviceID)
	if session == nil {
		fmt.Printf("No active session for device %s\n", deviceID)
		return
	}

	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	ri := inspect.NewRemoteInspector(session)

	if path.IsPartial {
		// Read all attributes
		attrs, err := ri.ReadAllAttributes(ctx, path.EndpointID, path.FeatureID)
		if err != nil {
			fmt.Printf("Read failed: %v\n", err)
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
		value, err := ri.ReadAttribute(ctx, path)
		if err != nil {
			fmt.Printf("Read failed: %v\n", err)
			return
		}

		// Get unit for formatting if available
		name := inspect.GetAttributeName(path.FeatureID, path.AttributeID)
		if name == "" {
			name = fmt.Sprintf("attr_%d", path.AttributeID)
		}
		fmt.Printf("%s = %v\n", name, value)
	}
}

// cmdWrite handles the write command.
func (i *InteractiveController) cmdWrite(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: write <device-id>/<endpoint>/<feature>/<attribute> <value>")
		fmt.Println("  Example: write evse-1234/1/energyControl/20 5000000")
		return
	}

	deviceID, pathStr := i.parseDevicePath(args[0])

	deviceID = i.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Printf("Device not found\n")
		return
	}

	if pathStr == "" {
		fmt.Println("Path required: <endpoint>/<feature>/<attribute>")
		return
	}

	session := i.svc.GetSession(deviceID)
	if session == nil {
		fmt.Printf("No active session for device %s\n", deviceID)
		return
	}

	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Printf("Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		fmt.Println("Cannot write to partial path, specify attribute")
		return
	}

	// Parse the value
	valueStr := strings.Join(args[1:], " ")
	var value any

	if v, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		value = v
	} else if v, err := strconv.ParseFloat(valueStr, 64); err == nil {
		value = v
	} else if v, err := strconv.ParseBool(valueStr); err == nil {
		value = v
	} else {
		value = strings.Trim(valueStr, "\"'")
	}

	ri := inspect.NewRemoteInspector(session)
	if err := ri.WriteAttribute(ctx, path, value); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	fmt.Println("OK")
}

// cmdLimit handles the limit command.
func (i *InteractiveController) cmdLimit(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: limit <device-id> <power-kw>")
		return
	}

	deviceID := i.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	powerKW, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		fmt.Printf("Invalid power: %v\n", err)
		return
	}

	limitMW := int64(powerKW * 1000000)
	fmt.Printf("Setting power limit to %.1f kW on %s...\n", powerKW, deviceID)

	if err := i.cem.SetPowerLimit(ctx, deviceID, 1, limitMW); err != nil {
		fmt.Printf("Failed to set limit: %v\n", err)
		return
	}

	fmt.Println("Limit set successfully")
}

// cmdClear handles the clear command.
func (i *InteractiveController) cmdClear(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: clear <device-id>")
		return
	}

	deviceID := i.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	fmt.Printf("Clearing power limit on %s...\n", deviceID)

	if err := i.cem.ClearPowerLimit(ctx, deviceID, 1); err != nil {
		fmt.Printf("Failed to clear limit: %v\n", err)
		return
	}

	fmt.Println("Limit cleared")
}

// cmdPause handles the pause command.
func (i *InteractiveController) cmdPause(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: pause <device-id>")
		return
	}

	deviceID := i.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	fmt.Printf("Pausing device %s...\n", deviceID)

	if err := i.cem.PauseDevice(ctx, deviceID, 1); err != nil {
		fmt.Printf("Failed to pause: %v\n", err)
		return
	}

	fmt.Println("Device paused")
}

// cmdResume handles the resume command.
func (i *InteractiveController) cmdResume(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: resume <device-id>")
		return
	}

	deviceID := i.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Printf("Device not found: %s\n", args[0])
		return
	}

	fmt.Printf("Resuming device %s...\n", deviceID)

	if err := i.cem.ResumeDevice(ctx, deviceID, 1); err != nil {
		fmt.Printf("Failed to resume: %v\n", err)
		return
	}

	fmt.Println("Device resumed")
}

// cmdStatus handles the status command.
func (i *InteractiveController) cmdStatus() {
	fmt.Println("\nController Status")
	fmt.Println("-------------------------------------------")
	fmt.Printf("  Zone Name:         %s\n", config.ZoneName)
	fmt.Printf("  Zone Type:         %s\n", config.ZoneType)
	fmt.Printf("  Service State:     %s\n", i.svc.State())
	fmt.Printf("  Zone ID:           %s\n", i.svc.ZoneID())
	fmt.Printf("  Connected Devices: %d\n", i.svc.DeviceCount())
	fmt.Printf("  Total Power:       %.1f kW\n", float64(i.cem.GetTotalPower())/1000000)
	fmt.Println()
}

// resolveDeviceID resolves a partial device ID to a full device ID.
func (i *InteractiveController) resolveDeviceID(partial string) string {
	// Try exact match first
	device := i.svc.GetDevice(partial)
	if device != nil {
		return device.ID
	}

	// Try partial match
	for _, d := range i.svc.GetAllDevices() {
		if strings.Contains(d.ID, partial) {
			return d.ID
		}
	}

	// Also check CEM
	for _, id := range i.cem.ConnectedDeviceIDs() {
		if strings.Contains(id, partial) {
			return id
		}
	}

	return ""
}

// parseDevicePath splits a device/path string into device ID and path.
// Examples:
//   - "evse-1234" -> ("evse-1234", "")
//   - "evse-1234/1" -> ("evse-1234", "1")
//   - "evse-1234/1/2/3" -> ("evse-1234", "1/2/3")
func (i *InteractiveController) parseDevicePath(input string) (deviceID, path string) {
	parts := strings.SplitN(input, "/", 2)
	deviceID = parts[0]
	if len(parts) > 1 {
		path = parts[1]
	}
	return
}

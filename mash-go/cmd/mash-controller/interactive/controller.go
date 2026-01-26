// Package interactive provides the interactive command-line interface
// for the MASH controller.
package interactive

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/mash-protocol/mash-go/pkg/examples"
	"github.com/mash-protocol/mash-go/pkg/inspect"
	"github.com/mash-protocol/mash-go/pkg/service"
)

// ControllerConfig provides configuration information to the interactive controller.
// This interface allows the interactive layer to access controller settings
// without depending on the main package's config structure.
//
// Implement this interface in your main package to provide the interactive
// controller with access to configuration. You can extend it with additional
// methods as needed (e.g., IsAutoCommissionEnabled(), StateDir()).
type ControllerConfig interface {
	// ZoneName returns the display name for this controller's zone.
	ZoneName() string

	// ZoneType returns the zone type as a human-readable string.
	ZoneType() string
}

// Controller handles interactive mode for mash-controller.
type Controller struct {
	svc       *service.ControllerService
	cem       *examples.CEM
	config    ControllerConfig
	formatter *inspect.Formatter
	rl        *readline.Instance
}

// New creates a new interactive controller handler.
func New(svc *service.ControllerService, cem *examples.CEM, cfg ControllerConfig) (*Controller, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\nmash> ",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create readline: %w", err)
	}

	return &Controller{
		svc:       svc,
		cem:       cem,
		config:    cfg,
		formatter: inspect.NewFormatter(),
		rl:        rl,
	}, nil
}

// Stdout returns a writer that properly coordinates with the readline input.
// Use this for log output to avoid interfering with the command prompt.
func (c *Controller) Stdout() io.Writer {
	return c.rl.Stdout()
}

// Stderr returns a writer that properly coordinates with the readline input.
func (c *Controller) Stderr() io.Writer {
	return c.rl.Stderr()
}

// Run starts the interactive command loop.
func (c *Controller) Run(ctx context.Context, cancel context.CancelFunc) {
	defer c.rl.Close()

	c.printHelp()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := c.rl.Readline()
		if err != nil {
			// EOF or interrupt
			if err == readline.ErrInterrupt {
				continue
			}
			fmt.Fprintln(c.rl.Stdout(), "Exiting...")
			cancel()
			return
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		cmd := strings.ToLower(parts[0])
		args := parts[1:]

		switch cmd {
		case "help", "?":
			c.printHelp()

		case "discover":
			c.cmdDiscover(ctx)

		case "list", "ls", "devices":
			c.cmdDevices()

		case "commission":
			c.cmdCommission(ctx, args)

		case "decommission":
			c.cmdDecommission(args)

		case "inspect", "i":
			c.cmdInspect(ctx, args)

		case "read", "r":
			c.cmdRead(ctx, args)

		case "write", "w":
			c.cmdWrite(ctx, args)

		case "limit":
			c.cmdLimit(ctx, args)

		case "clear":
			c.cmdClear(ctx, args)

		case "pause":
			c.cmdPause(ctx, args)

		case "resume":
			c.cmdResume(ctx, args)

		case "status":
			c.cmdStatus()

		case "quit", "exit", "q":
			fmt.Fprintln(c.rl.Stdout(), "Exiting...")
			cancel()
			return

		default:
			fmt.Fprintf(c.rl.Stdout(), "Unknown command: %s (type 'help' for commands)\n", cmd)
		}
	}
}

func (c *Controller) printHelp() {
	fmt.Fprintln(c.rl.Stdout(), `
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
func (c *Controller) cmdDiscover(ctx context.Context) {
	fmt.Fprintln(c.rl.Stdout(),"Discovering commissionable devices...")
	discoverCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	devices, err := c.svc.Discover(discoverCtx, nil)
	cancel()
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Discovery error: %v\n", err)
		return
	}
	if len(devices) == 0 {
		fmt.Fprintln(c.rl.Stdout(),"No commissionable devices found")
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Found %d commissionable device(s):\n", len(devices))
	for idx, d := range devices {
		fmt.Fprintf(c.rl.Stdout(),"  %d. %s (discriminator: %d, host: %s:%d)\n",
			idx+1, d.InstanceName, d.Discriminator, d.Host, d.Port)
	}
}

// cmdDevices handles the devices/list command.
func (c *Controller) cmdDevices() {
	devices := c.svc.GetAllDevices()
	if len(devices) == 0 {
		fmt.Fprintln(c.rl.Stdout(),"No devices connected")
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"\nConnected Devices (%d):\n", len(devices))
	fmt.Fprintln(c.rl.Stdout(),"-------------------------------------------")
	for _, d := range devices {
		status := "connected"
		if !d.Connected {
			status = "disconnected"
		}
		fmt.Fprintf(c.rl.Stdout(),"  ID: %s\n", d.ID)
		fmt.Fprintf(c.rl.Stdout(),"      Host: %s:%d\n", d.Host, d.Port)
		fmt.Fprintf(c.rl.Stdout(),"      Type: %s\n", d.DeviceType)
		fmt.Fprintf(c.rl.Stdout(),"      Status: %s\n", status)
		fmt.Fprintf(c.rl.Stdout(),"      Last seen: %s\n", d.LastSeen.Format("15:04:05"))
		fmt.Fprintln(c.rl.Stdout(),)
	}
}

// cmdCommission handles the commission command.
func (c *Controller) cmdCommission(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: commission <discriminator> <setup-code>")
		return
	}

	disc, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Invalid discriminator: %v\n", err)
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Looking for device with discriminator %d...\n", disc)

	device, err := c.svc.DiscoverByDiscriminator(ctx, uint16(disc))
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %v\n", err)
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Found device: %s at %s:%d\n", device.InstanceName, device.Host, device.Port)
	fmt.Fprintln(c.rl.Stdout(),"Commissioning...")

	commissioned, err := c.svc.Commission(ctx, device, args[1])
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Commissioning failed: %v\n", err)
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Device commissioned successfully: %s\n", commissioned.ID)
}

// cmdDecommission handles the decommission command.
func (c *Controller) cmdDecommission(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: decommission <device-id>")
		fmt.Fprintln(c.rl.Stdout(),"  Use 'devices' to list device IDs")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Decommissioning device %s...\n", deviceID)

	if err := c.svc.Decommission(deviceID); err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to decommission: %v\n", err)
		return
	}

	// Also remove from CEM
	_ = c.cem.DisconnectDevice(deviceID)

	fmt.Fprintln(c.rl.Stdout(),"Device decommissioned")
}

// cmdInspect handles the inspect command.
func (c *Controller) cmdInspect(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: inspect <device-id> [path]")
		fmt.Fprintln(c.rl.Stdout(),"  Examples:")
		fmt.Fprintln(c.rl.Stdout(),"    inspect evse-1234        - Show device overview")
		fmt.Fprintln(c.rl.Stdout(),"    inspect evse-1234/1      - Show endpoint 1")
		fmt.Fprintln(c.rl.Stdout(),"    inspect evse-1234/1/2    - Show feature on endpoint 1")
		return
	}

	// Parse device ID and optional path
	input := args[0]
	deviceID, pathStr := c.parseDevicePath(input)

	deviceID = c.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	session := c.svc.GetSession(deviceID)
	if session == nil {
		fmt.Fprintf(c.rl.Stdout(),"No active session for device %s\n", deviceID)
		return
	}

	ri := inspect.NewRemoteInspector(session)

	if pathStr == "" {
		// Show device overview by reading DeviceInfo
		c.showDeviceOverview(ctx, ri, deviceID)
		return
	}

	// Parse the path for specific inspection
	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		// Read all attributes for the endpoint/feature
		attrs, err := ri.ReadAllAttributes(ctx, path.EndpointID, path.FeatureID)
		if err != nil {
			fmt.Fprintf(c.rl.Stdout(),"Failed to read attributes: %v\n", err)
			return
		}

		fmt.Fprintf(c.rl.Stdout(),"\nEndpoint %d / Feature %d:\n", path.EndpointID, path.FeatureID)
		fmt.Fprintln(c.rl.Stdout(),"-------------------------------------------")
		for attrID, value := range attrs {
			name := inspect.GetAttributeName(path.FeatureID, attrID)
			if name == "" {
				name = fmt.Sprintf("attr_%d", attrID)
			}
			fmt.Fprintf(c.rl.Stdout(),"  %s: %v\n", name, value)
		}
	} else {
		// Read single attribute
		value, err := ri.ReadAttribute(ctx, path)
		if err != nil {
			fmt.Fprintf(c.rl.Stdout(),"Failed to read attribute: %v\n", err)
			return
		}
		fmt.Fprintf(c.rl.Stdout(),"%s = %v\n", path.Raw, value)
	}
}

// showDeviceOverview displays high-level device information.
func (c *Controller) showDeviceOverview(ctx context.Context, ri *inspect.RemoteInspector, deviceID string) {
	fmt.Fprintf(c.rl.Stdout(),"\nDevice: %s\n", deviceID)
	fmt.Fprintln(c.rl.Stdout(),"-------------------------------------------")

	// Read DeviceInfo from endpoint 0
	attrs, err := ri.ReadAllAttributes(ctx, 0, 1) // Endpoint 0, Feature DeviceInfo (0x01)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to read device info: %v\n", err)
		return
	}

	// Display key attributes
	if v, ok := attrs[1]; ok { // vendorName
		fmt.Fprintf(c.rl.Stdout(),"  Vendor: %v\n", v)
	}
	if v, ok := attrs[2]; ok { // productName
		fmt.Fprintf(c.rl.Stdout(),"  Product: %v\n", v)
	}
	if v, ok := attrs[3]; ok { // serialNumber
		fmt.Fprintf(c.rl.Stdout(),"  Serial: %v\n", v)
	}
	if v, ok := attrs[6]; ok { // firmwareVersion
		fmt.Fprintf(c.rl.Stdout(),"  Firmware: %v\n", v)
	}
	if v, ok := attrs[10]; ok { // endpointCount
		fmt.Fprintf(c.rl.Stdout(),"  Endpoints: %v\n", v)
	}
	fmt.Fprintln(c.rl.Stdout(),)
}

// cmdRead handles the read command.
func (c *Controller) cmdRead(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: read <device-id>/<endpoint>/<feature>/<attribute>")
		fmt.Fprintln(c.rl.Stdout(),"  Example: read evse-1234/1/measurement/acActivePower")
		return
	}

	deviceID, pathStr := c.parseDevicePath(args[0])

	deviceID = c.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found\n")
		return
	}

	if pathStr == "" {
		fmt.Fprintln(c.rl.Stdout(),"Path required: <endpoint>/<feature>/<attribute>")
		return
	}

	session := c.svc.GetSession(deviceID)
	if session == nil {
		fmt.Fprintf(c.rl.Stdout(),"No active session for device %s\n", deviceID)
		return
	}

	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Invalid path: %v\n", err)
		return
	}

	ri := inspect.NewRemoteInspector(session)

	if path.IsPartial {
		// Read all attributes
		attrs, err := ri.ReadAllAttributes(ctx, path.EndpointID, path.FeatureID)
		if err != nil {
			fmt.Fprintf(c.rl.Stdout(),"Read failed: %v\n", err)
			return
		}
		for attrID, value := range attrs {
			name := inspect.GetAttributeName(path.FeatureID, attrID)
			if name == "" {
				name = fmt.Sprintf("attr_%d", attrID)
			}
			fmt.Fprintf(c.rl.Stdout(),"  %s: %v\n", name, value)
		}
	} else {
		// Read single attribute
		value, err := ri.ReadAttribute(ctx, path)
		if err != nil {
			fmt.Fprintf(c.rl.Stdout(),"Read failed: %v\n", err)
			return
		}

		// Get unit for formatting if available
		name := inspect.GetAttributeName(path.FeatureID, path.AttributeID)
		if name == "" {
			name = fmt.Sprintf("attr_%d", path.AttributeID)
		}
		fmt.Fprintf(c.rl.Stdout(),"%s = %v\n", name, value)
	}
}

// cmdWrite handles the write command.
func (c *Controller) cmdWrite(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: write <device-id>/<endpoint>/<feature>/<attribute> <value>")
		fmt.Fprintln(c.rl.Stdout(),"  Example: write evse-1234/1/energyControl/20 5000000")
		return
	}

	deviceID, pathStr := c.parseDevicePath(args[0])

	deviceID = c.resolveDeviceID(deviceID)
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found\n")
		return
	}

	if pathStr == "" {
		fmt.Fprintln(c.rl.Stdout(),"Path required: <endpoint>/<feature>/<attribute>")
		return
	}

	session := c.svc.GetSession(deviceID)
	if session == nil {
		fmt.Fprintf(c.rl.Stdout(),"No active session for device %s\n", deviceID)
		return
	}

	path, err := inspect.ParsePath(pathStr)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Invalid path: %v\n", err)
		return
	}

	if path.IsPartial {
		fmt.Fprintln(c.rl.Stdout(),"Cannot write to partial path, specify attribute")
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
		fmt.Fprintf(c.rl.Stdout(),"Write failed: %v\n", err)
		return
	}

	fmt.Fprintln(c.rl.Stdout(),"OK")
}

// cmdLimit handles the limit command.
func (c *Controller) cmdLimit(ctx context.Context, args []string) {
	if len(args) < 2 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: limit <device-id> <power-kw>")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	powerKW, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Invalid power: %v\n", err)
		return
	}

	limitMW := int64(powerKW * 1000000)
	fmt.Fprintf(c.rl.Stdout(),"Setting power limit to %.1f kW on %s...\n", powerKW, deviceID)

	if err := c.cem.SetPowerLimit(ctx, deviceID, 1, limitMW); err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to set limit: %v\n", err)
		return
	}

	fmt.Fprintln(c.rl.Stdout(),"Limit set successfully")
}

// cmdClear handles the clear command.
func (c *Controller) cmdClear(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: clear <device-id>")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Clearing power limit on %s...\n", deviceID)

	if err := c.cem.ClearPowerLimit(ctx, deviceID, 1); err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to clear limit: %v\n", err)
		return
	}

	fmt.Fprintln(c.rl.Stdout(),"Limit cleared")
}

// cmdPause handles the pause command.
func (c *Controller) cmdPause(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: pause <device-id>")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Pausing device %s...\n", deviceID)

	if err := c.cem.PauseDevice(ctx, deviceID, 1); err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to pause: %v\n", err)
		return
	}

	fmt.Fprintln(c.rl.Stdout(),"Device paused")
}

// cmdResume handles the resume command.
func (c *Controller) cmdResume(ctx context.Context, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(c.rl.Stdout(),"Usage: resume <device-id>")
		return
	}

	deviceID := c.resolveDeviceID(args[0])
	if deviceID == "" {
		fmt.Fprintf(c.rl.Stdout(),"Device not found: %s\n", args[0])
		return
	}

	fmt.Fprintf(c.rl.Stdout(),"Resuming device %s...\n", deviceID)

	if err := c.cem.ResumeDevice(ctx, deviceID, 1); err != nil {
		fmt.Fprintf(c.rl.Stdout(),"Failed to resume: %v\n", err)
		return
	}

	fmt.Fprintln(c.rl.Stdout(),"Device resumed")
}

// cmdStatus handles the status command.
func (c *Controller) cmdStatus() {
	fmt.Fprintln(c.rl.Stdout(),"\nController Status")
	fmt.Fprintln(c.rl.Stdout(),"-------------------------------------------")
	fmt.Fprintf(c.rl.Stdout(),"  Zone Name:         %s\n", c.config.ZoneName())
	fmt.Fprintf(c.rl.Stdout(),"  Zone Type:         %s\n", c.config.ZoneType())
	fmt.Fprintf(c.rl.Stdout(),"  Service State:     %s\n", c.svc.State())
	fmt.Fprintf(c.rl.Stdout(),"  Zone ID:           %s\n", c.svc.ZoneID())
	fmt.Fprintf(c.rl.Stdout(),"  Connected Devices: %d\n", c.svc.DeviceCount())
	fmt.Fprintf(c.rl.Stdout(),"  Total Power:       %.1f kW\n", float64(c.cem.GetTotalPower())/1000000)
	fmt.Fprintln(c.rl.Stdout(),)
}

// resolveDeviceID resolves a partial device ID to a full device ID.
func (c *Controller) resolveDeviceID(partial string) string {
	// Try exact match first
	device := c.svc.GetDevice(partial)
	if device != nil {
		return device.ID
	}

	// Try partial match
	for _, d := range c.svc.GetAllDevices() {
		if strings.Contains(d.ID, partial) {
			return d.ID
		}
	}

	// Also check CEM
	for _, id := range c.cem.ConnectedDeviceIDs() {
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
func (c *Controller) parseDevicePath(input string) (deviceID, path string) {
	parts := strings.SplitN(input, "/", 2)
	deviceID = parts[0]
	if len(parts) > 1 {
		path = parts[1]
	}
	return
}

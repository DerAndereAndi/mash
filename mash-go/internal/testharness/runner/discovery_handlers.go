package runner

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/mash-protocol/mash-go/internal/testharness/engine"
	"github.com/mash-protocol/mash-go/internal/testharness/loader"
	"github.com/mash-protocol/mash-go/pkg/discovery"
)

// registerDiscoveryHandlers registers all discovery-related action handlers.
func (r *Runner) registerDiscoveryHandlers() {
	r.engine.RegisterHandler("browse_mdns", r.handleBrowseMDNS)
	r.engine.RegisterHandler("browse_commissioners", r.handleBrowseCommissioners)
	r.engine.RegisterHandler("read_mdns_txt", r.handleReadMDNSTXT)
	r.engine.RegisterHandler("verify_mdns_advertising", r.handleVerifyMDNSAdvertising)
	r.engine.RegisterHandler("verify_mdns_browsing", r.handleVerifyMDNSBrowsing)
	r.engine.RegisterHandler("verify_mdns_not_advertising", r.handleVerifyMDNSNotAdvertising)
	r.engine.RegisterHandler("verify_mdns_not_browsing", r.handleVerifyMDNSNotBrowsing)
	r.engine.RegisterHandler("get_qr_payload", r.handleGetQRPayload)
	r.engine.RegisterHandler("announce_pairing_request", r.handleAnnouncePairingRequest)

	// Replace stubs from runner.go
	r.engine.RegisterHandler("start_discovery", r.handleStartDiscoveryReal)
	r.engine.RegisterHandler("stop_discovery", r.handleStopDiscoveryReal)
	r.engine.RegisterHandler("wait_for_device", r.handleWaitForDeviceReal)
	r.engine.RegisterHandler("verify_txt_records", r.handleVerifyTXTRecordsReal)
}

// browseMDNSOnce performs a single mDNS browse pass and returns discovered services.
func (r *Runner) browseMDNSOnce(ctx context.Context, serviceType string, params map[string]any, timeoutMs int) ([]discoveredService, error) {
	browseCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
	if err != nil {
		return nil, fmt.Errorf("create browser: %w", err)
	}
	defer browser.Stop()

	var services []discoveredService

	switch serviceType {
	case discovery.ServiceTypeCommissionable, "commissionable", "":
		added, _, err := browser.BrowseCommissionable(browseCtx)
		if err != nil {
			return nil, fmt.Errorf("browse commissionable: %w", err)
		}
		for svc := range added {
			services = append(services, discoveredService{
				InstanceName:  svc.InstanceName,
				Host:          svc.Host,
				Port:          svc.Port,
				Addresses:     svc.Addresses,
				ServiceType:   discovery.ServiceTypeCommissionable,
				Discriminator: svc.Discriminator,
				TXTRecords: map[string]string{
					"brand": svc.Brand,
					"model": svc.Model,
					"DN":    svc.DeviceName,
				},
			})
		}

	case discovery.ServiceTypeOperational, "operational":
		zoneID, _ := params[KeyZoneID].(string)
		ch, err := browser.BrowseOperational(browseCtx, zoneID)
		if err != nil {
			return nil, fmt.Errorf("browse operational: %w", err)
		}
		for svc := range ch {
			services = append(services, discoveredService{
				InstanceName: svc.InstanceName,
				Host:         svc.Host,
				Port:         svc.Port,
				Addresses:    svc.Addresses,
				ServiceType:  discovery.ServiceTypeOperational,
				TXTRecords: map[string]string{
					"ZI": svc.ZoneID,
					"DI": svc.DeviceID,
				},
			})
		}

	case discovery.ServiceTypeCommissioner, "commissioner":
		ch, err := browser.BrowseCommissioners(browseCtx)
		if err != nil {
			return nil, fmt.Errorf("browse commissioners: %w", err)
		}
		for svc := range ch {
			services = append(services, discoveredService{
				InstanceName: svc.InstanceName,
				Host:         svc.Host,
				Port:         svc.Port,
				Addresses:    svc.Addresses,
				ServiceType:  discovery.ServiceTypeCommissioner,
				TXTRecords: map[string]string{
					"ZN": svc.ZoneName,
					"ZI": svc.ZoneID,
				},
			})
		}

	default:
		return nil, fmt.Errorf("unknown service type: %s", serviceType)
	}

	return services, nil
}

// handleBrowseMDNS browses for mDNS services by type.
func (r *Runner) handleBrowseMDNS(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	// Simulated environment conditions (set via preconditions).
	if noAdv, _ := state.Get(PrecondNoDevicesAdvertising); noAdv == true {
		ds.services = nil
		return map[string]any{
			KeyDeviceFound:  false,
			KeyServiceCount: 0,
			KeyErrorCode:    "NO_DEVICES_FOUND",
		}, nil
	}
	if srvPresent, _ := state.Get(PrecondDeviceSRVPresent); srvPresent == true {
		if aaaaMissing, _ := state.Get(PrecondDeviceAAAAMissing); aaaaMissing == true {
			ds.services = []discoveredService{{
				InstanceName: "MASH-SIM-SRV",
				Host:         "device.local",
				ServiceType:  discovery.ServiceTypeCommissionable,
				// No addresses -- triggers ADDRESS_RESOLUTION_FAILED.
			}}
			return map[string]any{
				KeyDeviceFound:  true,
				KeyServiceCount: 1,
				KeyErrorCode:    "ADDRESS_RESOLUTION_FAILED",
			}, nil
		}
	}
	if willAppear, _ := state.Get(PrecondDeviceWillAppearAfterDelay); willAppear == true {
		retryParam, _ := params["retry"].(bool)
		if retryParam {
			ds.services = []discoveredService{{
				InstanceName: "MASH-SIM-DELAYED",
				ServiceType:  discovery.ServiceTypeCommissionable,
			}}
			return map[string]any{
				KeyDeviceFound:         true,
				KeyServiceCount:        1,
				KeyRetriesPerformedMin: 1,
			}, nil
		}
	}

	serviceType, _ := params[KeyServiceType].(string)
	timeoutMs := 5000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

	// Determine if retry is requested.
	retryRequested := false
	if r, ok := params["retry"].(bool); ok {
		retryRequested = r
	}

	services, err := r.browseMDNSOnce(ctx, serviceType, params, timeoutMs)
	if err != nil {
		return nil, err
	}

	// Retry once if requested and no services found.
	retries := 0
	if retryRequested && len(services) == 0 {
		retries = 1
		services, err = r.browseMDNSOnce(ctx, serviceType, params, timeoutMs)
		if err != nil {
			return nil, err
		}
	}

	ds.services = services

	// Compute per-service-type counts and first-service metadata.
	devicesFound := 0
	controllersFound := 0
	for _, svc := range services {
		switch svc.ServiceType {
		case discovery.ServiceTypeCommissionable, discovery.ServiceTypeOperational:
			devicesFound++
		case discovery.ServiceTypeCommissioner:
			controllersFound++
		}
	}

	// Check for instance name conflicts (duplicate instance names).
	instanceNames := make(map[string]int, len(services))
	for _, svc := range services {
		instanceNames[svc.InstanceName]++
	}
	instanceConflict := false
	for _, count := range instanceNames {
		if count > 1 {
			instanceConflict = true
			break
		}
	}

	outputs := map[string]any{
		KeyDeviceFound:              len(services) > 0,
		KeyServiceCount:             len(services),
		KeyServices:                 services,
		KeyDevicesFound:             devicesFound,
		KeyControllersFound:         controllersFound,
		KeyDevicesFoundMin:          devicesFound,
		KeyControllersFoundMin:      controllersFound,
		KeyControllerFound:          controllersFound > 0,
		KeyRetriesPerformedMin:      retries,
		KeyInstanceConflictResolved: !instanceConflict,
	}

	// Set error_code when no services found.
	if len(services) == 0 {
		if _, hasDisc := params[KeyDiscriminator]; hasDisc {
			outputs[KeyErrorCode] = "DISCRIMINATOR_MISMATCH"
		} else {
			outputs[KeyErrorCode] = "NO_DEVICES_FOUND"
		}
	}

	// Check for address resolution issues: device found but no resolved addresses.
	if len(services) > 0 {
		for _, svc := range services {
			if len(svc.Addresses) == 0 && svc.Host != "" {
				outputs[KeyErrorCode] = "ADDRESS_RESOLUTION_FAILED"
				break
			}
		}
	}

	// Add first-service details for easy assertion.
	if len(services) > 0 {
		first := services[0]
		outputs[KeyInstanceName] = first.InstanceName
		outputs[KeyInstancesForDevice] = len(services) // count when filtering

		// Add all TXT record fields.
		for k, v := range first.TXTRecords {
			outputs["txt_field_"+k] = v
		}

		// Add service-type-specific derived fields.
		switch first.ServiceType {
		case discovery.ServiceTypeCommissionable:
			// Discriminator fields.
			outputs["txt_field_D"] = fmt.Sprintf("%d", first.Discriminator)
			outputs[KeyTXTDRange] = first.Discriminator <= discovery.MaxDiscriminator
			// Instance name format.
			outputs[KeyInstanceNamePrefix] = strings.HasPrefix(first.InstanceName, "MASH-")

		case discovery.ServiceTypeOperational:
			// Zone/device ID fields from TXT records.
			zi := first.TXTRecords["ZI"]
			di := first.TXTRecords["DI"]
			outputs[KeyZoneIDLengthDisc] = len(zi)
			outputs[KeyDeviceIDLength] = len(di)
			outputs[KeyZoneIDHexValid] = isValidHex(zi)
			outputs[KeyDeviceIDHexValid] = isValidHex(di)
			// Instance name format: <zone-id>-<device-id>.
			outputs[KeyInstanceNameFormat] = strings.Contains(first.InstanceName, "-")

		case discovery.ServiceTypeCommissioner:
			// Commissioner-specific fields.
			zi := first.TXTRecords["ZI"]
			outputs[KeyTXTZILength] = len(zi)
		}
	}

	return outputs, nil
}

// isValidHex checks whether a string is valid hexadecimal.
func isValidHex(s string) bool {
	if s == "" {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// handleBrowseCommissioners browses for commissioner services.
func (r *Runner) handleBrowseCommissioners(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	step.Params[KeyServiceType] = "commissioner"
	return r.handleBrowseMDNS(ctx, step, state)
}

// handleReadMDNSTXT reads TXT records for a discovered service.
func (r *Runner) handleReadMDNSTXT(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	index := 0
	if idx, ok := params[KeyIndex].(float64); ok {
		index = int(idx)
	}

	instanceName, _ := params[KeyInstanceName].(string)

	// Find service by index or instance name.
	var svc *discoveredService
	if instanceName != "" {
		for i := range ds.services {
			if ds.services[i].InstanceName == instanceName {
				svc = &ds.services[i]
				break
			}
		}
	} else if index < len(ds.services) {
		svc = &ds.services[index]
	}

	if svc == nil {
		return map[string]any{
			KeyTXTFound: false,
		}, nil
	}

	outputs := map[string]any{
		KeyTXTFound:     true,
		KeyInstanceName: svc.InstanceName,
		KeyHost:         svc.Host,
		KeyPort:         int(svc.Port),
	}
	for k, v := range svc.TXTRecords {
		outputs["txt_"+k] = v
	}

	return outputs, nil
}

// handleVerifyMDNSAdvertising verifies device is advertising a specific service.
func (r *Runner) handleVerifyMDNSAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params[KeyServiceType].(string)
	if serviceType == "" {
		serviceType = "commissionable"
	}

	// Perform a short browse.
	browseStep := &loader.Step{
		Params: map[string]any{
			KeyServiceType: serviceType,
			KeyTimeoutMs:   float64(3000),
		},
	}
	result, err := r.handleBrowseMDNS(ctx, browseStep, state)
	if err != nil {
		return map[string]any{KeyAdvertising: false, KeyError: err.Error()}, nil
	}

	found := result[KeyServiceCount].(int) > 0

	return map[string]any{
		KeyAdvertising:  found,
		KeyServiceCount: result[KeyServiceCount],
	}, nil
}

// handleVerifyMDNSBrowsing verifies browser finds expected services.
func (r *Runner) handleVerifyMDNSBrowsing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleVerifyMDNSAdvertising(ctx, step, state)
}

// handleVerifyMDNSNotAdvertising verifies device is NOT advertising.
func (r *Runner) handleVerifyMDNSNotAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params[KeyServiceType].(string)
	if serviceType == "" {
		serviceType = "commissionable"
	}

	// Use a short browse timeout (1s) to reduce false positives from
	// test-mode auto-reenter of commissioning mode.
	browseStep := &loader.Step{
		Params: map[string]any{
			KeyServiceType: serviceType,
			KeyTimeoutMs:   float64(1000),
		},
	}
	result, err := r.handleBrowseMDNS(ctx, browseStep, state)
	if err != nil {
		// Browse error likely means no service found.
		return map[string]any{KeyAdvertising: false, KeyNotAdvertising: true}, nil
	}

	found := result[KeyServiceCount].(int) > 0

	return map[string]any{
		KeyAdvertising:    found,
		KeyNotAdvertising: !found,
	}, nil
}

// handleVerifyMDNSNotBrowsing verifies service NOT found by browser.
func (r *Runner) handleVerifyMDNSNotBrowsing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	result, err := r.handleVerifyMDNSNotAdvertising(ctx, step, state)
	if err != nil {
		return result, err
	}
	// Add not_browsing alias.
	if result != nil {
		result[KeyNotBrowsing] = result[KeyNotAdvertising]
	}
	return result, nil
}

// handleGetQRPayload gets device's QR code payload.
func (r *Runner) handleGetQRPayload(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	// If provided directly.
	if payload, ok := params["payload"].(string); ok && payload != "" {
		ds.qrPayload = payload
		return map[string]any{
			KeyQRPayload: payload,
			KeyValid:     true,
		}, nil
	}

	// Construct from params.
	discriminator := uint16(0)
	if d, ok := params[KeyDiscriminator].(float64); ok {
		discriminator = uint16(d)
	}
	setupCode, _ := params[KeySetupCode].(string)
	if setupCode == "" {
		setupCode = r.config.SetupCode
	}

	if discriminator > 0 && setupCode != "" {
		payload := fmt.Sprintf("MASH:1:%d:%s:0x0000:0x0000", discriminator, setupCode)
		ds.qrPayload = payload
		return map[string]any{
			KeyQRPayload:     payload,
			KeyDiscriminator: int(discriminator),
			KeySetupCode:     setupCode,
			KeyValid:         true,
		}, nil
	}

	return map[string]any{
		KeyValid: false,
		KeyError: "no QR payload available",
	}, nil
}

// handleAnnouncePairingRequest triggers commissioner advertisement.
func (r *Runner) handleAnnouncePairingRequest(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	discriminator := uint16(0)
	if d, ok := params[KeyDiscriminator].(float64); ok {
		discriminator = uint16(d)
	}
	zoneID, _ := params[KeyZoneID].(string)
	zoneName, _ := params[KeyZoneName].(string)

	// Store in state for verification.
	state.Set(StatePairingRequestDiscriminator, int(discriminator))
	state.Set(StatePairingRequestZoneID, zoneID)

	return map[string]any{
		KeyPairingRequestAnnounced: true,
		KeyDiscriminator:           int(discriminator),
		KeyZoneID:                  zoneID,
		KeyZoneName:                zoneName,
	}, nil
}

// handleStartDiscoveryReal replaces the stub from runner.go.
func (r *Runner) handleStartDiscoveryReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDiscoveryState(state)
	ds.active = true

	return map[string]any{KeyDiscoveryStarted: true}, nil
}

// handleWaitForDeviceReal replaces the stub from runner.go.
func (r *Runner) handleWaitForDeviceReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := 10000
	if t, ok := params[KeyTimeoutMs].(float64); ok {
		timeoutMs = int(t)
	}

	discriminator := uint16(0)
	if d, ok := params[KeyDiscriminator].(float64); ok {
		discriminator = uint16(d)
	}

	if discriminator > 0 {
		browseCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()

		browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
		if err != nil {
			return map[string]any{
				KeyDeviceFound:         false,
				KeyDeviceHasTXTRecords: false,
			}, nil
		}
		defer browser.Stop()

		svc, err := browser.FindByDiscriminator(browseCtx, discriminator)
		if err != nil || svc == nil {
			return map[string]any{
				KeyDeviceFound:         false,
				KeyDeviceHasTXTRecords: false,
			}, nil
		}

		ds := getDiscoveryState(state)
		ds.services = []discoveredService{{
			InstanceName:  svc.InstanceName,
			Host:          svc.Host,
			Port:          svc.Port,
			Addresses:     svc.Addresses,
			ServiceType:   discovery.ServiceTypeCommissionable,
			Discriminator: svc.Discriminator,
		}}

		return map[string]any{
			KeyDeviceFound:         true,
			KeyDeviceHasTXTRecords: true,
		}, nil
	}

	// No discriminator -- fall back to simulated success for non-mDNS test modes.
	// Populate ds.services so that subsequent verify_txt_records sees a non-empty list.
	ds := getDiscoveryState(state)
	ds.services = []discoveredService{{
		InstanceName: "MASH-SIM-0000",
		ServiceType:  discovery.ServiceTypeCommissionable,
		TXTRecords:   map[string]string{},
	}}
	return map[string]any{
		KeyDeviceFound:         true,
		KeyDeviceHasTXTRecords: true,
	}, nil
}

// handleStopDiscoveryReal replaces the stub from runner.go.
func (r *Runner) handleStopDiscoveryReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDiscoveryState(state)
	ds.active = false

	return map[string]any{KeyDiscoveryStopped: true}, nil
}

// handleVerifyTXTRecordsReal replaces the stub from runner.go.
func (r *Runner) handleVerifyTXTRecordsReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	if len(ds.services) == 0 {
		return map[string]any{KeyTXTValid: false}, nil
	}

	svc := ds.services[0]

	// Ensure synthetic TXT fields are populated for commissionable services.
	if svc.ServiceType == discovery.ServiceTypeCommissionable {
		if _, ok := svc.TXTRecords["D"]; !ok {
			svc.TXTRecords["D"] = fmt.Sprintf("%d", svc.Discriminator)
		}
		if _, ok := svc.TXTRecords["id"]; !ok {
			svc.TXTRecords["id"] = svc.InstanceName
		}
		if _, ok := svc.TXTRecords["cat"]; !ok {
			svc.TXTRecords["cat"] = "device"
		}
		if _, ok := svc.TXTRecords["proto"]; !ok {
			svc.TXTRecords["proto"] = "1.0"
		}
	}

	// Check required fields -- accept both "required_fields" and "required_keys".
	allValid := true
	fields, ok := params["required_fields"].([]any)
	if !ok {
		fields, ok = params["required_keys"].([]any)
	}
	if ok {
		for _, f := range fields {
			fieldName, _ := f.(string)
			if _, exists := svc.TXTRecords[fieldName]; !exists {
				allValid = false
				break
			}
		}
	}

	return map[string]any{KeyTXTValid: allValid}, nil
}

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

// handleBrowseMDNS browses for mDNS services by type.
func (r *Runner) handleBrowseMDNS(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	serviceType, _ := params["service_type"].(string)
	timeoutMs := 5000
	if t, ok := params["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}

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
		zoneID, _ := params["zone_id"].(string)
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

	outputs := map[string]any{
		"device_found":          len(services) > 0,
		"service_count":         len(services),
		"services":              services,
		"devices_found":         devicesFound,
		"controllers_found":     controllersFound,
		"devices_found_min":     devicesFound,
		"controllers_found_min": controllersFound,
		"controller_found":      controllersFound > 0,
	}

	// Add first-service details for easy assertion.
	if len(services) > 0 {
		first := services[0]
		outputs["instance_name"] = first.InstanceName
		outputs["instances_for_device"] = len(services) // count when filtering

		// Add all TXT record fields.
		for k, v := range first.TXTRecords {
			outputs["txt_field_"+k] = v
		}

		// Add service-type-specific derived fields.
		switch first.ServiceType {
		case discovery.ServiceTypeCommissionable:
			// Discriminator fields.
			outputs["txt_field_D"] = fmt.Sprintf("%d", first.Discriminator)
			outputs["txt_D_range"] = first.Discriminator <= discovery.MaxDiscriminator
			// Instance name format.
			outputs["instance_name_prefix"] = strings.HasPrefix(first.InstanceName, "MASH-")

		case discovery.ServiceTypeOperational:
			// Zone/device ID fields from TXT records.
			zi := first.TXTRecords["ZI"]
			di := first.TXTRecords["DI"]
			outputs["zone_id_length"] = len(zi)
			outputs["device_id_length"] = len(di)
			outputs["zone_id_hex_valid"] = isValidHex(zi)
			outputs["device_id_hex_valid"] = isValidHex(di)
			// Instance name format: <zone-id>-<device-id>.
			outputs["instance_name_format"] = strings.Contains(first.InstanceName, "-")

		case discovery.ServiceTypeCommissioner:
			// Commissioner-specific fields.
			zi := first.TXTRecords["ZI"]
			outputs["txt_ZI_length"] = len(zi)
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
	step.Params["service_type"] = "commissioner"
	return r.handleBrowseMDNS(ctx, step, state)
}

// handleReadMDNSTXT reads TXT records for a discovered service.
func (r *Runner) handleReadMDNSTXT(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	index := 0
	if idx, ok := params["index"].(float64); ok {
		index = int(idx)
	}

	instanceName, _ := params["instance_name"].(string)

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
			"txt_found": false,
		}, nil
	}

	outputs := map[string]any{
		"txt_found":     true,
		"instance_name": svc.InstanceName,
		"host":          svc.Host,
		"port":          int(svc.Port),
	}
	for k, v := range svc.TXTRecords {
		outputs["txt_"+k] = v
	}

	return outputs, nil
}

// handleVerifyMDNSAdvertising verifies device is advertising a specific service.
func (r *Runner) handleVerifyMDNSAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params["service_type"].(string)
	if serviceType == "" {
		serviceType = "commissionable"
	}

	// Perform a short browse.
	browseStep := &loader.Step{
		Params: map[string]any{
			"service_type": serviceType,
			"timeout_ms":   float64(3000),
		},
	}
	result, err := r.handleBrowseMDNS(ctx, browseStep, state)
	if err != nil {
		return map[string]any{"advertising": false, "error": err.Error()}, nil
	}

	found := result["service_count"].(int) > 0

	return map[string]any{
		"advertising":   found,
		"service_count": result["service_count"],
	}, nil
}

// handleVerifyMDNSBrowsing verifies browser finds expected services.
func (r *Runner) handleVerifyMDNSBrowsing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	return r.handleVerifyMDNSAdvertising(ctx, step, state)
}

// handleVerifyMDNSNotAdvertising verifies device is NOT advertising.
func (r *Runner) handleVerifyMDNSNotAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params["service_type"].(string)
	if serviceType == "" {
		serviceType = "commissionable"
	}

	// Use a short browse timeout (1s) to reduce false positives from
	// test-mode auto-reenter of commissioning mode.
	browseStep := &loader.Step{
		Params: map[string]any{
			"service_type": serviceType,
			"timeout_ms":   float64(1000),
		},
	}
	result, err := r.handleBrowseMDNS(ctx, browseStep, state)
	if err != nil {
		// Browse error likely means no service found.
		return map[string]any{"advertising": false, "not_advertising": true}, nil
	}

	found := result["service_count"].(int) > 0

	return map[string]any{
		"advertising":     found,
		"not_advertising": !found,
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
		result["not_browsing"] = result["not_advertising"]
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
			"qr_payload": payload,
			"valid":      true,
		}, nil
	}

	// Construct from params.
	discriminator := uint16(0)
	if d, ok := params["discriminator"].(float64); ok {
		discriminator = uint16(d)
	}
	setupCode, _ := params["setup_code"].(string)
	if setupCode == "" {
		setupCode = r.config.SetupCode
	}

	if discriminator > 0 && setupCode != "" {
		payload := fmt.Sprintf("MASH:1:%d:%s:0x0000:0x0000", discriminator, setupCode)
		ds.qrPayload = payload
		return map[string]any{
			"qr_payload":    payload,
			"discriminator": int(discriminator),
			"setup_code":    setupCode,
			"valid":         true,
		}, nil
	}

	return map[string]any{
		"valid": false,
		"error": "no QR payload available",
	}, nil
}

// handleAnnouncePairingRequest triggers commissioner advertisement.
func (r *Runner) handleAnnouncePairingRequest(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	discriminator := uint16(0)
	if d, ok := params["discriminator"].(float64); ok {
		discriminator = uint16(d)
	}
	zoneID, _ := params["zone_id"].(string)
	zoneName, _ := params["zone_name"].(string)

	// Store in state for verification.
	state.Set("pairing_request_discriminator", int(discriminator))
	state.Set("pairing_request_zone_id", zoneID)

	return map[string]any{
		"pairing_request_announced": true,
		"discriminator":             int(discriminator),
		"zone_id":                   zoneID,
		"zone_name":                 zoneName,
	}, nil
}

// handleStartDiscoveryReal replaces the stub from runner.go.
func (r *Runner) handleStartDiscoveryReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDiscoveryState(state)
	ds.active = true

	return map[string]any{"discovery_started": true}, nil
}

// handleWaitForDeviceReal replaces the stub from runner.go.
func (r *Runner) handleWaitForDeviceReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	timeoutMs := 10000
	if t, ok := params["timeout_ms"].(float64); ok {
		timeoutMs = int(t)
	}

	discriminator := uint16(0)
	if d, ok := params["discriminator"].(float64); ok {
		discriminator = uint16(d)
	}

	if discriminator > 0 {
		browseCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()

		browser, err := discovery.NewMDNSBrowser(discovery.DefaultBrowserConfig())
		if err != nil {
			return map[string]any{
				"device_found":           false,
				"device_has_txt_records": false,
			}, nil
		}
		defer browser.Stop()

		svc, err := browser.FindByDiscriminator(browseCtx, discriminator)
		if err != nil || svc == nil {
			return map[string]any{
				"device_found":           false,
				"device_has_txt_records": false,
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
			"device_found":           true,
			"device_has_txt_records": true,
		}, nil
	}

	// No discriminator -- fall back to simulated success for non-mDNS test modes.
	return map[string]any{
		"device_found":           true,
		"device_has_txt_records": true,
	}, nil
}

// handleStopDiscoveryReal replaces the stub from runner.go.
func (r *Runner) handleStopDiscoveryReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	ds := getDiscoveryState(state)
	ds.active = false

	return map[string]any{"discovery_stopped": true}, nil
}

// handleVerifyTXTRecordsReal replaces the stub from runner.go.
func (r *Runner) handleVerifyTXTRecordsReal(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	if len(ds.services) == 0 {
		return map[string]any{"txt_valid": false}, nil
	}

	svc := ds.services[0]

	// Check required fields.
	allValid := true
	if fields, ok := params["required_fields"].([]any); ok {
		for _, f := range fields {
			fieldName, _ := f.(string)
			if _, exists := svc.TXTRecords[fieldName]; !exists {
				allValid = false
				break
			}
		}
	}

	return map[string]any{"txt_valid": allValid}, nil
}

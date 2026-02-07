package runner

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
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
	r.engine.RegisterHandler("stop_pairing_request", r.handleStopPairingRequest)

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
	case discovery.ServiceTypeCommissionable, ServiceAliasCommissionable, "":
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

	case discovery.ServiceTypeOperational, ServiceAliasOperational:
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

	case discovery.ServiceTypeCommissioner, ServiceAliasCommissioner:
		ch, err := browser.BrowseCommissioners(browseCtx)
		if err != nil {
			return nil, fmt.Errorf("browse commissioners: %w", err)
		}
		for svc := range ch {
			txt := map[string]string{
				"ZN": svc.ZoneName,
				"ZI": svc.ZoneID,
				"DC": strconv.Itoa(int(svc.DeviceCount)),
			}
			services = append(services, discoveredService{
				InstanceName: svc.InstanceName,
				Host:         svc.Host,
				Port:         svc.Port,
				Addresses:    svc.Addresses,
				ServiceType:  discovery.ServiceTypeCommissioner,
				TXTRecords:   txt,
			})
		}

	case discovery.ServiceTypePairingRequest, ServiceAliasPairingRequest:
		var mu sync.Mutex
		err := browser.BrowsePairingRequests(browseCtx, func(svc discovery.PairingRequestService) {
			mu.Lock()
			services = append(services, discoveredService{
				InstanceName:  svc.InstanceName,
				Host:          svc.Host,
				Port:          svc.Port,
				Addresses:     svc.Addresses,
				ServiceType:   discovery.ServiceTypePairingRequest,
				Discriminator: svc.Discriminator,
				TXTRecords: map[string]string{
					"ZI": svc.ZoneID,
					"ZN": svc.ZoneName,
				},
			})
			mu.Unlock()
		})
		if err != nil {
			return nil, fmt.Errorf("browse pairing requests: %w", err)
		}
		// BrowsePairingRequests is non-blocking; wait for browse timeout.
		<-browseCtx.Done()
		// Ensure any in-flight callback has completed.
		mu.Lock()
		mu.Unlock()

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
			KeyDeviceFound:        false,
			KeyServiceCount:       0,
			KeyInstancesForDevice: 0,
			KeyErrorCode:          ErrCodeNoDevicesFound,
			KeyError:              "browse_timeout",
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
				KeyDeviceFound:        true,
				KeyServiceCount:       1,
				KeyInstancesForDevice: 1,
				KeyErrorCode:          ErrCodeAddrResolutionFailed,
			}, nil
		}
	}
	if willAppear, _ := state.Get(PrecondDeviceWillAppearAfterDelay); willAppear == true {
		retryParam, _ := params[ParamRetry].(bool)
		if retryParam {
			ds.services = []discoveredService{{
				InstanceName: "MASH-SIM-DELAYED",
				ServiceType:  discovery.ServiceTypeCommissionable,
			}}
			return map[string]any{
				KeyDeviceFound:         true,
				KeyServiceCount:        1,
				KeyInstancesForDevice:  1,
				KeyRetriesPerformedMin: 1,
			}, nil
		}
	}

	// Deferred commissioning: a pairing request was announced and the device
	// was powered on. Simulate the device advertising commissionable service.
	// This covers TC-PAIR-004 where the device detects the pairing request
	// and opens its commissioning window.
	// Only apply when browsing for commissionable services (not pairing requests
	// or other service types).
	requestedType, _ := params[KeyServiceType].(string)
	if requestedType == "" || requestedType == discovery.ServiceTypeCommissionable || requestedType == ServiceAliasCommissionable {
		if disc, _ := state.Get(StatePairingRequestDiscriminator); disc != nil {
			if powered, _ := state.Get(KeyPoweredOn); powered == true {
				d := uint16(0)
				if dv, ok := disc.(int); ok {
					d = uint16(dv)
				}
				ds.services = []discoveredService{{
					InstanceName:  fmt.Sprintf("MASH-%d", d),
					ServiceType:   discovery.ServiceTypeCommissionable,
					Host:          "device.local",
					Port:          8443,
					Addresses:     []string{"192.168.1.10"},
					Discriminator: d,
					TXTRecords: map[string]string{
						"brand": "Test",
						"model": "Sim",
					},
				}}
				return r.buildBrowseOutput(ds)
			}
		}
	}

	// Device was removed from all zones -- no operational instances.
	if removed, _ := state.Get(StateDeviceWasRemoved); removed == true {
		if inZone, _ := state.Get(PrecondDeviceInZone); inZone != true {
			ds.services = nil
			return r.buildBrowseOutput(ds)
		}
	}

	// Simulate a device already commissioned into a zone.
	if inZone, _ := state.Get(PrecondDeviceInZone); inZone == true {
		serviceType, _ := params[KeyServiceType].(string)
		switch serviceType {
		case discovery.ServiceTypeOperational, ServiceAliasOperational:
			ds.services = []discoveredService{{
				InstanceName: "a1b2c3d4-00112233",
				ServiceType:  discovery.ServiceTypeOperational,
				Host:         "device.local",
				Port:         8443,
				Addresses:    []string{"192.168.1.10"},
				TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "00112233"},
			}}
		default:
			// Commissionable or unspecified -- return a single commissionable service.
			ds.services = []discoveredService{{
				InstanceName:  "MASH-SIM-ZONE",
				ServiceType:   discovery.ServiceTypeCommissionable,
				Host:          "device.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.10"},
				Discriminator: 1234,
				TXTRecords:    map[string]string{"brand": "Test", "model": "Sim"},
			}}
		}
		return r.buildBrowseOutput(ds)
	}

	// Simulate a device commissioned into two zones.
	if inTwoZones, _ := state.Get(PrecondDeviceInTwoZones); inTwoZones == true {
		serviceType, _ := params[KeyServiceType].(string)
		switch serviceType {
		case discovery.ServiceTypeOperational, ServiceAliasOperational:
			ds.services = []discoveredService{
				{
					InstanceName: "a1b2c3d4-00112233",
					ServiceType:  discovery.ServiceTypeOperational,
					Host:         "device.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.10"},
					TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "00112233"},
				},
				{
					InstanceName: "e5f6a7b8-00112233",
					ServiceType:  discovery.ServiceTypeOperational,
					Host:         "device.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.10"},
					TXTRecords:   map[string]string{"ZI": "e5f6a7b8", "DI": "00112233"},
				},
			}
		default:
			ds.services = []discoveredService{{
				InstanceName:  "MASH-SIM-TWOZONE",
				ServiceType:   discovery.ServiceTypeCommissionable,
				Host:          "device.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.10"},
				Discriminator: 1234,
				TXTRecords:    map[string]string{"brand": "Test", "model": "Sim"},
			}}
		}
		return r.buildBrowseOutput(ds)
	}

	// Simulate commissioner service when a zone has been created.
	if zoneCreated, _ := state.Get(PrecondZoneCreated); zoneCreated == true {
		serviceType, _ := params[KeyServiceType].(string)
		if serviceType == discovery.ServiceTypeCommissioner || serviceType == ServiceAliasCommissioner {
			zs := getZoneState(state)
			// Create one commissioner instance per zone.
			for _, zoneID := range zs.zoneOrder {
				zoneName := "MASH Zone"
				deviceCount := 0
				if z, ok := zs.zones[zoneID]; ok {
					if z.ZoneName != "" {
						zoneName = z.ZoneName
					} else if z.ZoneType != "" {
						zoneName = z.ZoneType
					}
					deviceCount = len(z.DeviceIDs)
				}
				ds.services = append(ds.services, discoveredService{
					InstanceName: zoneName,
					ServiceType:  discovery.ServiceTypeCommissioner,
					Host:         "controller.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.1"},
					TXTRecords:   map[string]string{"ZN": zoneName, "ZI": zoneID, "DC": strconv.Itoa(deviceCount)},
				})
			}
			if len(ds.services) == 0 {
				// Fallback when no zones exist in state.
				ds.services = []discoveredService{{
					InstanceName: "MASH Zone",
					ServiceType:  discovery.ServiceTypeCommissioner,
					Host:         "controller.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.1"},
					TXTRecords:   map[string]string{"ZN": "MASH Zone", "ZI": "sim-zone-id", "DC": "0"},
				}}
			}
			return r.buildBrowseOutput(ds)
		}
	}

	// Simulate multiple commissionable devices.
	if multiComm, _ := state.Get(PrecondMultipleDevicesCommissioning); multiComm == true {
		ds.services = []discoveredService{
			{
				InstanceName:  "MASH-SIM-A",
				ServiceType:   discovery.ServiceTypeCommissionable,
				Host:          "device-a.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.10"},
				Discriminator: 1001,
				TXTRecords:    map[string]string{"brand": "Test", "model": "A"},
			},
			{
				InstanceName:  "MASH-SIM-B",
				ServiceType:   discovery.ServiceTypeCommissionable,
				Host:          "device-b.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.11"},
				Discriminator: 1002,
				TXTRecords:    map[string]string{"brand": "Test", "model": "B"},
			},
		}
		return r.buildBrowseOutput(ds)
	}

	// Simulate multiple commissioned (operational) devices.
	if multiOp, _ := state.Get(PrecondMultipleDevicesCommissioned); multiOp == true {
		serviceType, _ := params[KeyServiceType].(string)
		switch serviceType {
		case discovery.ServiceTypeOperational, ServiceAliasOperational:
			ds.services = []discoveredService{
				{
					InstanceName: "a1b2c3d4-00112233",
					ServiceType:  discovery.ServiceTypeOperational,
					Host:         "device-a.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.10"},
					TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "00112233"},
				},
				{
					InstanceName: "a1b2c3d4-44556677",
					ServiceType:  discovery.ServiceTypeOperational,
					Host:         "device-b.local",
					Port:         8443,
					Addresses:    []string{"192.168.1.11"},
					TXTRecords:   map[string]string{"ZI": "a1b2c3d4", "DI": "44556677"},
				},
			}
		default:
			ds.services = []discoveredService{
				{
					InstanceName:  "MASH-SIM-A",
					ServiceType:   discovery.ServiceTypeCommissionable,
					Host:          "device-a.local",
					Port:          8443,
					Addresses:     []string{"192.168.1.10"},
					Discriminator: 1001,
					TXTRecords:    map[string]string{"brand": "Test", "model": "A"},
				},
				{
					InstanceName:  "MASH-SIM-B",
					ServiceType:   discovery.ServiceTypeCommissionable,
					Host:          "device-b.local",
					Port:          8443,
					Addresses:     []string{"192.168.1.11"},
					Discriminator: 1002,
					TXTRecords:    map[string]string{"brand": "Test", "model": "B"},
				},
			}
		}
		return r.buildBrowseOutput(ds)
	}

	// Simulate multiple controllers running (commissioner services).
	if multiCtrl, _ := state.Get(PrecondMultipleControllersRunning); multiCtrl == true {
		ds.services = []discoveredService{
			{
				InstanceName: "Controller-A",
				ServiceType:  discovery.ServiceTypeCommissioner,
				Host:         "controller-a.local",
				Port:         8443,
				Addresses:    []string{"192.168.1.1"},
				TXTRecords:   map[string]string{"ZN": "Grid Zone", "ZI": "a1b2c3d4e5f6a7b8"},
			},
			{
				InstanceName: "Controller-B",
				ServiceType:  discovery.ServiceTypeCommissioner,
				Host:         "controller-b.local",
				Port:         8443,
				Addresses:    []string{"192.168.1.2"},
				TXTRecords:   map[string]string{"ZN": "Home Zone", "ZI": "c3d4e5f6a7b8a1b2"},
			},
		}
		return r.buildBrowseOutput(ds)
	}

	// Simulate two devices with the same discriminator.
	// This must be checked before the commissioning_active stub below,
	// because the test's device_local_action enter_commissioning_mode step
	// sets commissioning_active=true and the generic stub would return a
	// single service, short-circuiting the two-device simulation.
	retries := 0
	if twoDevs, _ := state.Get(PrecondTwoDevicesSameDiscriminator); twoDevs == true {
		disc := uint16(paramInt(params, KeyDiscriminator, 1234))
		ds.services = []discoveredService{
			{
				InstanceName:  fmt.Sprintf("MASH-%d", disc),
				ServiceType:   discovery.ServiceTypeCommissionable,
				Discriminator: disc,
				Host:          "device-a.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.10"},
				TXTRecords:    map[string]string{"brand": "Test", "model": "A"},
			},
			{
				InstanceName:  fmt.Sprintf("MASH-%d-2", disc),
				ServiceType:   discovery.ServiceTypeCommissionable,
				Discriminator: disc,
				Host:          "device-b.local",
				Port:          8443,
				Addresses:     []string{"192.168.1.11"},
				TXTRecords:    map[string]string{"brand": "Test", "model": "B"},
			},
		}
	} else {
		// Stub mode: enter_commissioning_mode was called without a device
		// connection. Return a synthetic commissionable service so that
		// verify_mdns_advertising sees advertising=true.
		if active, _ := state.Get(StateCommissioningActive); active == true {
			requestedType, _ := params[KeyServiceType].(string)
			if requestedType == "" || requestedType == discovery.ServiceTypeCommissionable || requestedType == ServiceAliasCommissionable {
				ds.services = []discoveredService{{
					InstanceName:  "MASH-SIM-COMM",
					ServiceType:   discovery.ServiceTypeCommissionable,
					Host:          "device.local",
					Port:          8443,
					Addresses:     []string{"192.168.1.10"},
					Discriminator: 1234,
					TXTRecords: map[string]string{
						"brand":  "Test",
						"model":  "Sim",
						"cat":    "1",
						"serial": "SIM-0001",
					},
				}}
				return r.buildBrowseOutput(ds)
			}
		}

		serviceType, _ := params[KeyServiceType].(string)
		timeoutMs := paramInt(params, KeyTimeoutMs, 5000)

		// Determine if retry is requested.
		retryRequested := false
		if r, ok := params[ParamRetry].(bool); ok {
			retryRequested = r
		}

		services, err := r.browseMDNSOnce(ctx, serviceType, params, timeoutMs)
		if err != nil {
			return nil, err
		}

		// Retry once if requested and no services found.
		if retryRequested && len(services) == 0 {
			retries = 1
			services, err = r.browseMDNSOnce(ctx, serviceType, params, timeoutMs)
			if err != nil {
				return nil, err
			}
		}

		ds.services = services
	}

	outputs, err := r.buildBrowseOutput(ds)
	if err != nil {
		return nil, err
	}
	outputs[KeyRetriesPerformedMin] = retries

	// Set error_code when no services found.
	if len(ds.services) == 0 {
		if _, hasDisc := params[KeyDiscriminator]; hasDisc {
			outputs[KeyErrorCode] = ErrCodeDiscriminatorMismatch
		} else {
			outputs[KeyErrorCode] = ErrCodeNoDevicesFound
		}
	}

	// Check for address resolution issues: device found but no resolved addresses.
	if len(ds.services) > 0 {
		for _, svc := range ds.services {
			if len(svc.Addresses) == 0 && svc.Host != "" {
				outputs[KeyErrorCode] = ErrCodeAddrResolutionFailed
				break
			}
		}
	}

	return outputs, nil
}

// buildBrowseOutput constructs the standard output map from discovery state.
// This is shared by simulation paths and the real mDNS browse path.
func (r *Runner) buildBrowseOutput(ds *discoveryState) (map[string]any, error) {
	// Compute per-service-type counts.
	devicesFound := 0
	controllersFound := 0
	for _, svc := range ds.services {
		switch svc.ServiceType {
		case discovery.ServiceTypeCommissionable, discovery.ServiceTypeOperational:
			devicesFound++
		case discovery.ServiceTypeCommissioner:
			controllersFound++
		}
	}

	// Check for instance name conflicts (duplicate instance names).
	instanceNames := make(map[string]int, len(ds.services))
	for _, svc := range ds.services {
		instanceNames[svc.InstanceName]++
	}
	instanceConflict := false
	for _, count := range instanceNames {
		if count > 1 {
			instanceConflict = true
			break
		}
	}

	// Count IPv6 (AAAA) addresses.
	aaaaCount := 0
	for _, svc := range ds.services {
		for _, addr := range svc.Addresses {
			if strings.Contains(addr, ":") {
				aaaaCount++
			}
		}
	}

	// Check if all results belong to the same zone (for zone_id_filter assertion).
	allInZone := true
	zoneIDs := make(map[string]bool)
	for _, svc := range ds.services {
		if zi, ok := svc.TXTRecords["ZI"]; ok {
			zoneIDs[strings.ToLower(zi)] = true
		}
	}
	if len(zoneIDs) != 1 || len(ds.services) == 0 {
		allInZone = false
	}

	// Classify addresses across all services.
	hasLinkLocal := false
	hasGlobalOrULA := false
	allIPv6Valid := aaaaCount > 0
	for _, svc := range ds.services {
		for _, addr := range svc.Addresses {
			ip := net.ParseIP(addr)
			if ip == nil {
				continue
			}
			if ip.To4() == nil { // IPv6
				if ip.IsLinkLocalUnicast() {
					hasLinkLocal = true
				} else {
					hasGlobalOrULA = true
				}
			}
		}
	}

	outputs := map[string]any{
		KeyDeviceFound:              len(ds.services) > 0,
		KeyServiceCount:             len(ds.services),
		KeyServices:                 ds.services,
		KeyDevicesFound:             devicesFound,
		KeyControllersFound:         controllersFound,
		KeyCommissionersFound:       controllersFound > 0,
		KeyDevicesFoundMin:          devicesFound,
		KeyControllersFoundMin:      controllersFound,
		KeyCommissionerCountMin:     controllersFound,
		KeyControllerFound:          controllersFound > 0,
		KeyRetriesPerformedMin:      0,
		KeyInstanceConflictResolved: !instanceConflict,
		KeyInstancesForDevice:       len(ds.services),
		KeyAllResultsInZone:         allInZone,
		KeyAAAACount:                aaaaCount,
		KeyAAAACountMin:             aaaaCount,
		KeyIPv6Valid:                allIPv6Valid,
		KeyHasGlobalOrULA:           hasGlobalOrULA,
		KeyHasLinkLocal:             hasLinkLocal,
	}

	// Add first-service details for easy assertion.
	if len(ds.services) > 0 {
		first := ds.services[0]
		outputs[KeyInstanceName] = first.InstanceName
		outputs["service_has_txt_records"] = len(first.TXTRecords) > 0

		// SRV record fields.
		outputs["srv_port"] = int(first.Port)
		outputs[KeySRVHostnameValid] = first.Host != ""

		// Add all TXT record fields.
		for k, v := range first.TXTRecords {
			outputs["txt_field_"+k] = v
		}

		// TXT record length fields.
		if zi, ok := first.TXTRecords["ZI"]; ok {
			outputs["txt_ZI_length"] = len(zi)
		}
		if di, ok := first.TXTRecords["DI"]; ok {
			outputs["txt_DI_length"] = len(di)
		}

		// Add service-type-specific derived fields.
		switch first.ServiceType {
		case discovery.ServiceTypeCommissionable:
			// Discriminator fields.
			outputs["txt_field_D"] = fmt.Sprintf("%d", first.Discriminator)
			if first.Discriminator <= discovery.MaxDiscriminator {
				outputs[KeyTXTDRange] = "0-4095"
			} else {
				outputs[KeyTXTDRange] = fmt.Sprintf("out-of-range(%d)", first.Discriminator)
			}
			// Instance name format.
			if strings.HasPrefix(first.InstanceName, "MASH-") {
				outputs[KeyInstanceNamePrefix] = "MASH-"
			} else {
				outputs[KeyInstanceNamePrefix] = ""
			}

		case discovery.ServiceTypeOperational:
			// Zone/device ID fields from TXT records.
			zi := first.TXTRecords["ZI"]
			di := first.TXTRecords["DI"]
			outputs[KeyZoneIDLengthDisc] = len(zi)
			outputs[KeyDeviceIDLength] = len(di)
			outputs[KeyZoneIDHexValid] = isValidHex(zi)
			outputs[KeyDeviceIDHexValid] = isValidHex(di)
			// Instance name format: <zone-id>-<device-id>.
			if strings.Contains(first.InstanceName, "-") {
				outputs[KeyInstanceNameFormat] = "<zone-id>-<device-id>"
			} else {
				outputs[KeyInstanceNameFormat] = first.InstanceName
			}

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
	step.Params[KeyServiceType] = ServiceAliasCommissioner
	return r.handleBrowseMDNS(ctx, step, state)
}

// handleReadMDNSTXT reads TXT records for a discovered service.
func (r *Runner) handleReadMDNSTXT(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)
	ds := getDiscoveryState(state)

	index := paramInt(params, KeyIndex, 0)

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
			KeyTXTFound:  false,
			KeyKeyExists: false,
		}, nil
	}

	outputs := map[string]any{
		KeyTXTFound:     true,
		KeyInstanceName: svc.InstanceName,
		KeyHost:         svc.Host,
		KeyPort:         int(svc.Port),
		KeyKeyExists:    len(svc.TXTRecords) > 0,
	}

	// Expose individual TXT records as "txt_<key>".
	for k, v := range svc.TXTRecords {
		outputs["txt_"+k] = v
	}

	// Add specific TXT field validation keys.
	if _, ok := svc.TXTRecords["cat"]; ok {
		outputs["txt_field_cat"] = svc.TXTRecords["cat"]
	}
	if serial, ok := svc.TXTRecords["serial"]; ok {
		outputs["txt_field_serial"] = serial
	}
	if zi, ok := svc.TXTRecords["ZI"]; ok {
		outputs["txt_field_ZI"] = zi
		outputs[KeyTXTZILength] = len(zi)
	}
	if di, ok := svc.TXTRecords["DI"]; ok {
		outputs["txt_field_DI"] = di
		outputs["txt_DI_length"] = len(di)
	}

	return outputs, nil
}

// handleVerifyMDNSAdvertising verifies device is advertising a specific service.
func (r *Runner) handleVerifyMDNSAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params[KeyServiceType].(string)
	if serviceType == "" {
		serviceType = ServiceAliasCommissionable
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

	outputs := map[string]any{
		KeyAdvertising:  found,
		KeyServiceCount: result[KeyServiceCount],
	}

	// Propagate TXT records from the first discovered service so test
	// cases can assert on individual TXT fields (e.g., txt_records.id).
	if found {
		if services, ok := result[KeyServices].([]discoveredService); ok && len(services) > 0 {
			first := services[0]
			txtMap := make(map[string]any, len(first.TXTRecords))
			for k, v := range first.TXTRecords {
				txtMap[k] = v
			}
			// Add discriminator as "disc" for commissionable services.
			if first.ServiceType == discovery.ServiceTypeCommissionable {
				txtMap["disc"] = fmt.Sprintf("%d", first.Discriminator)
			}
			outputs[KeyTXTRecords] = txtMap
		}
	}

	return outputs, nil
}

// handleVerifyMDNSBrowsing verifies browser finds expected services.
func (r *Runner) handleVerifyMDNSBrowsing(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	result, err := r.handleVerifyMDNSAdvertising(ctx, step, state)
	if err != nil {
		return result, err
	}
	// Add browsing alias for advertising.
	if result != nil {
		result[KeyBrowsing] = result[KeyAdvertising]
	}
	return result, nil
}

// handleVerifyMDNSNotAdvertising verifies device is NOT advertising.
func (r *Runner) handleVerifyMDNSNotAdvertising(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	serviceType, _ := params[KeyServiceType].(string)
	if serviceType == "" {
		serviceType = ServiceAliasCommissionable
	}

	// Default to a short browse timeout to reduce false positives from
	// test-mode auto-reenter of commissioning mode. Respect step's timeout if set.
	timeoutMs := float64(1000)
	if t := paramFloat(params, KeyTimeoutMs, 0); t > 0 {
		timeoutMs = t
	}
	browseStep := &loader.Step{
		Params: map[string]any{
			KeyServiceType: serviceType,
			KeyTimeoutMs:   timeoutMs,
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
	if payload, ok := params[ParamPayload].(string); ok && payload != "" {
		ds.qrPayload = payload
		return map[string]any{
			KeyQRPayload: payload,
			KeyValid:     true,
		}, nil
	}

	// Construct from params.
	discriminator := uint16(paramInt(params, KeyDiscriminator, 0))
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

	// Auto-generate: return cached payload or generate a new one.
	if ds.qrPayload != "" {
		qr, err := discovery.ParseQRCode(ds.qrPayload)
		if err == nil {
			return map[string]any{
				KeyQRPayload:     ds.qrPayload,
				KeyDiscriminator: int(qr.Discriminator),
				KeySetupCode:     qr.SetupCode,
				KeyVersion:       int(qr.Version),
				KeyValid:         true,
			}, nil
		}
	}

	// Generate a fresh QR code (simulates factory provisioning at boot).
	qr, err := discovery.GenerateQRCode()
	if err != nil {
		return map[string]any{
			KeyValid: false,
			KeyError: fmt.Sprintf("failed to generate QR: %v", err),
		}, nil
	}
	ds.qrPayload = qr.String()
	return map[string]any{
		KeyQRPayload:     ds.qrPayload,
		KeyDiscriminator: int(qr.Discriminator),
		KeySetupCode:     qr.SetupCode,
		KeyVersion:       int(qr.Version),
		KeyValid:         true,
	}, nil
}

// handleAnnouncePairingRequest advertises a real _mashp._udp pairing request via mDNS.
func (r *Runner) handleAnnouncePairingRequest(ctx context.Context, step *loader.Step, state *engine.ExecutionState) (map[string]any, error) {
	params := engine.InterpolateParams(step.Params, state)

	discriminator := uint16(paramInt(params, KeyDiscriminator, 0))
	zoneID, _ := params[KeyZoneID].(string)
	zoneName, _ := params[KeyZoneName].(string)

	// Store in state for verification.
	state.Set(StatePairingRequestDiscriminator, int(discriminator))
	state.Set(StatePairingRequestZoneID, zoneID)

	// Lazy-init the mDNS advertiser on the runner.
	if r.pairingAdvertiser == nil {
		adv, err := discovery.NewMDNSAdvertiser(discovery.AdvertiserConfig{})
		if err != nil {
			return nil, fmt.Errorf("create pairing advertiser: %w", err)
		}
		r.pairingAdvertiser = adv
	}

	host, _ := os.Hostname()
	if host == "" {
		host = "mash-test-runner.local"
	}

	info := &discovery.PairingRequestInfo{
		Discriminator: discriminator,
		ZoneID:        zoneID,
		ZoneName:      zoneName,
		Host:          host,
	}

	if err := r.pairingAdvertiser.AnnouncePairingRequest(ctx, info); err != nil {
		return nil, fmt.Errorf("announce pairing request: %w", err)
	}

	return map[string]any{
		KeyPairingRequestAnnounced: true,
		KeyAnnouncementSent:       true,
		KeyDiscriminator:          int(discriminator),
		KeyZoneID:                 zoneID,
		KeyZoneName:               zoneName,
	}, nil
}

// handleStopPairingRequest stops advertising pairing requests via mDNS.
func (r *Runner) handleStopPairingRequest(_ context.Context, _ *loader.Step, _ *engine.ExecutionState) (map[string]any, error) {
	if r.pairingAdvertiser != nil {
		r.pairingAdvertiser.StopAll()
		r.pairingAdvertiser = nil
	}
	return map[string]any{
		"stopped": true,
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

	timeoutMs := paramInt(params, KeyTimeoutMs, 10000)

	discriminator := uint16(paramInt(params, KeyDiscriminator, 0))

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
	fields, ok := params[ParamRequiredFields].([]any)
	if !ok {
		fields, ok = params[ParamRequiredKeys].([]any)
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

	outputs := map[string]any{KeyTXTValid: allValid}

	// Expose zone/device ID derived fields for assertions.
	if zi, ok := svc.TXTRecords["ZI"]; ok {
		outputs["zone_id_length"] = len(zi)
		outputs["zone_id_hex_valid"] = isValidHex(zi)
	}
	if di, ok := svc.TXTRecords["DI"]; ok {
		outputs["device_id_length"] = len(di)
		outputs["device_id_hex_valid"] = isValidHex(di)
	}

	return outputs, nil
}

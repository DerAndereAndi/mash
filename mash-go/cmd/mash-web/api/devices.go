package api

import (
	"context"
	"fmt"
	"time"

	"github.com/mash-protocol/mash-go/pkg/discovery"
)

// DiscoverDevices discovers MASH devices on the local network.
func DiscoverDevices(ctx context.Context, timeoutStr string) (*DeviceListResponse, error) {
	// Parse timeout
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 10 * time.Second
	}

	// Create timeout context
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create mDNS browser
	browserConfig := discovery.DefaultBrowserConfig()
	browser, err := discovery.NewMDNSBrowser(browserConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS browser: %w", err)
	}
	defer browser.Stop()

	// Browse for commissionable devices
	added, _, err := browser.BrowseCommissionable(browseCtx)
	if err != nil {
		return nil, fmt.Errorf("mDNS browse failed: %w", err)
	}

	// Collect results
	var devices []Device
	for svc := range added {
		devices = append(devices, commissionableServiceToDevice(svc))
	}

	return &DeviceListResponse{
		Devices:      devices,
		DiscoveredAt: time.Now(),
		Timeout:      timeoutStr,
	}, nil
}

// commissionableServiceToDevice converts a discovery.CommissionableService to an API Device.
func commissionableServiceToDevice(svc *discovery.CommissionableService) Device {
	return Device{
		InstanceName:  svc.InstanceName,
		Host:          svc.Host,
		Port:          svc.Port,
		Addresses:     svc.Addresses,
		Discriminator: svc.Discriminator,
		Brand:         svc.Brand,
		Model:         svc.Model,
		DeviceName:    svc.DeviceName,
		Serial:        svc.Serial,
	}
}

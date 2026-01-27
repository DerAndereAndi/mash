package discovery

import (
	"context"
	"time"

	"github.com/enbility/zeroconf/v3/api"
)

// Browser provides mDNS service browsing capabilities.
type Browser interface {
	// BrowseCommissionable searches for devices in commissioning mode.
	// Returns two channels: added (new devices) and removed (devices that disappeared).
	// Both channels are closed when the context is cancelled or browsing completes.
	BrowseCommissionable(ctx context.Context) (added, removed <-chan *CommissionableService, err error)

	// BrowseOperational searches for commissioned devices.
	// Optionally filter by zone ID.
	BrowseOperational(ctx context.Context, zoneID string) (<-chan *OperationalService, error)

	// BrowseCommissioners searches for zone controllers.
	BrowseCommissioners(ctx context.Context) (<-chan *CommissionerService, error)

	// BrowsePairingRequests searches for pairing requests from controllers.
	// Devices use this to discover controllers that want to commission them.
	// The callback is invoked for each discovered pairing request.
	BrowsePairingRequests(ctx context.Context, callback func(PairingRequestService)) error

	// FindByDiscriminator searches for a specific commissionable device.
	// Returns when found or when context is cancelled/timeout.
	FindByDiscriminator(ctx context.Context, discriminator uint16) (*CommissionableService, error)

	// Stop stops all active browsing operations.
	Stop()
}

// BrowserConfig configures browser behavior.
type BrowserConfig struct {
	// BrowseTimeout is the default timeout for browse operations.
	// Default: 10 seconds.
	BrowseTimeout time.Duration

	// Interface specifies which network interface to use.
	// Empty string means all interfaces.
	Interface string

	// ConnectionFactory creates multicast connections.
	// If nil, uses the default zeroconf connection factory.
	// Set this in tests to inject mock connections.
	ConnectionFactory api.ConnectionFactory

	// InterfaceProvider lists network interfaces.
	// If nil, uses the default zeroconf interface provider.
	// Set this in tests to inject mock interface lists.
	InterfaceProvider api.InterfaceProvider
}

// DefaultBrowserConfig returns the default browser configuration.
func DefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		BrowseTimeout: BrowseTimeout,
		Interface:     "",
	}
}

// BrowseResult is a union type for browse results.
type BrowseResult struct {
	// Type indicates the result type.
	Type ServiceType

	// Commissionable is set when Type is ServiceTypeCommissionable.
	Commissionable *CommissionableService

	// Operational is set when Type is ServiceTypeOperational.
	Operational *OperationalService

	// Commissioner is set when Type is ServiceTypeCommissioner.
	Commissioner *CommissionerService

	// Error is set if an error occurred.
	Error error
}

// ServiceType identifies the type of mDNS service.
type ServiceType int

const (
	// ServiceCommissionable is a commissionable device service.
	ServiceCommissionable ServiceType = iota

	// ServiceOperational is an operational device service.
	ServiceOperational

	// ServiceCommissioner is a commissioner/controller service.
	ServiceCommissioner
)

// FilterFunc is a function that filters browse results.
type FilterFunc func(*CommissionableService) bool

// FilterByCategory returns a filter that matches devices with any of the given categories.
func FilterByCategory(categories ...DeviceCategory) FilterFunc {
	catSet := make(map[DeviceCategory]struct{})
	for _, c := range categories {
		catSet[c] = struct{}{}
	}

	return func(svc *CommissionableService) bool {
		for _, c := range svc.Categories {
			if _, ok := catSet[c]; ok {
				return true
			}
		}
		return false
	}
}

// FilterByDiscriminator returns a filter that matches devices with the given discriminator.
func FilterByDiscriminator(discriminator uint16) FilterFunc {
	return func(svc *CommissionableService) bool {
		return svc.Discriminator == discriminator
	}
}

// FilterBrowseResults filters a channel of commissionable services.
func FilterBrowseResults(in <-chan *CommissionableService, filter FilterFunc) <-chan *CommissionableService {
	out := make(chan *CommissionableService)
	go func() {
		defer close(out)
		for svc := range in {
			if filter(svc) {
				out <- svc
			}
		}
	}()
	return out
}

// ParseServiceEntry parses raw mDNS service entry data into the appropriate service type.
// This is a helper for Browser implementations.
type ServiceEntry struct {
	Instance string
	Service  string
	Domain   string
	Host     string
	Port     uint16
	Text     []string
	Addrs    []string
}

// ToCommissionableService converts a ServiceEntry to CommissionableService.
func (e *ServiceEntry) ToCommissionableService() (*CommissionableService, error) {
	txt := StringsToTXTRecords(e.Text)
	info, err := DecodeCommissionableTXT(txt)
	if err != nil {
		return nil, err
	}

	return &CommissionableService{
		InstanceName:  e.Instance,
		Host:          e.Host,
		Port:          e.Port,
		Addresses:     e.Addrs,
		Discriminator: info.Discriminator,
		Categories:    info.Categories,
		Serial:        info.Serial,
		Brand:         info.Brand,
		Model:         info.Model,
		DeviceName:    info.DeviceName,
	}, nil
}

// ToOperationalService converts a ServiceEntry to OperationalService.
func (e *ServiceEntry) ToOperationalService() (*OperationalService, error) {
	txt := StringsToTXTRecords(e.Text)
	info, err := DecodeOperationalTXT(txt)
	if err != nil {
		return nil, err
	}

	return &OperationalService{
		InstanceName:  e.Instance,
		Host:          e.Host,
		Port:          e.Port,
		Addresses:     e.Addrs,
		ZoneID:        info.ZoneID,
		DeviceID:      info.DeviceID,
		VendorProduct: info.VendorProduct,
		Firmware:      info.Firmware,
		FeatureMap:    info.FeatureMap,
		EndpointCount: info.EndpointCount,
	}, nil
}

// ToCommissionerService converts a ServiceEntry to CommissionerService.
func (e *ServiceEntry) ToCommissionerService() (*CommissionerService, error) {
	txt := StringsToTXTRecords(e.Text)
	info, err := DecodeCommissionerTXT(txt)
	if err != nil {
		return nil, err
	}

	return &CommissionerService{
		InstanceName:   e.Instance,
		Host:           e.Host,
		Port:           e.Port,
		Addresses:      e.Addrs,
		ZoneName:       info.ZoneName,
		ZoneID:         info.ZoneID,
		VendorProduct:  info.VendorProduct,
		ControllerName: info.ControllerName,
		DeviceCount:    info.DeviceCount,
	}, nil
}

// ToPairingRequestService converts a ServiceEntry to PairingRequestService.
func (e *ServiceEntry) ToPairingRequestService() (*PairingRequestService, error) {
	txt := StringsToTXTRecords(e.Text)
	info, err := DecodePairingRequestTXT(txt)
	if err != nil {
		return nil, err
	}

	return &PairingRequestService{
		InstanceName:  e.Instance,
		Host:          e.Host,
		Port:          e.Port,
		Addresses:     e.Addrs,
		Discriminator: info.Discriminator,
		ZoneID:        info.ZoneID,
		ZoneName:      info.ZoneName,
	}, nil
}

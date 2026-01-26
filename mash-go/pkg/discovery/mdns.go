package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/enbility/zeroconf/v3"
)

// MDNSAdvertiser implements the Advertiser interface using zeroconf.
type MDNSAdvertiser struct {
	config AdvertiserConfig

	mu sync.Mutex

	// Active services
	commissionableServer *zeroconf.Server
	operationalServers   map[string]*zeroconf.Server // keyed by zoneID
	commissionerServers  map[string]*zeroconf.Server // keyed by zoneID
}

// NewMDNSAdvertiser creates a new mDNS advertiser.
func NewMDNSAdvertiser(config AdvertiserConfig) (*MDNSAdvertiser, error) {
	return &MDNSAdvertiser{
		config:              config,
		operationalServers:  make(map[string]*zeroconf.Server),
		commissionerServers: make(map[string]*zeroconf.Server),
	}, nil
}

// getInterfaces returns the network interfaces to use for advertising.
// Returns nil to use all interfaces.
func (a *MDNSAdvertiser) getInterfaces() []net.Interface {
	if a.config.Interface == "" {
		return nil
	}

	iface, err := net.InterfaceByName(a.config.Interface)
	if err != nil {
		return nil
	}
	return []net.Interface{*iface}
}

// AdvertiseCommissionable starts advertising a commissionable service.
func (a *MDNSAdvertiser) AdvertiseCommissionable(ctx context.Context, info *CommissionableInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop existing if any
	if a.commissionableServer != nil {
		a.commissionableServer.Shutdown()
		a.commissionableServer = nil
	}

	// Build instance name: "MASH-<discriminator>"
	instanceName := fmt.Sprintf("MASH-%04d", info.Discriminator)

	// Build TXT records
	txtRecords := EncodeCommissionableTXT(info)
	txtStrings := TXTRecordsToStrings(txtRecords)

	// Determine port
	port := int(info.Port)
	if port == 0 {
		port = DefaultPort
	}

	// Register service
	var opts []zeroconf.ServerOption
	if a.config.TTL > 0 {
		opts = append(opts, zeroconf.TTL(uint32(a.config.TTL.Seconds())))
	}

	// Get interfaces (nil means all interfaces)
	ifaces := a.getInterfaces()

	server, err := zeroconf.Register(
		instanceName,
		ServiceTypeCommissionable,
		Domain,
		port,
		txtStrings,
		ifaces,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to register commissionable service: %w", err)
	}

	a.commissionableServer = server
	return nil
}

// StopCommissionable stops advertising the commissionable service.
func (a *MDNSAdvertiser) StopCommissionable() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.commissionableServer != nil {
		a.commissionableServer.Shutdown()
		a.commissionableServer = nil
	}
	return nil
}

// AdvertiseOperational starts advertising an operational service for a zone.
func (a *MDNSAdvertiser) AdvertiseOperational(ctx context.Context, info *OperationalInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop existing for this zone if any
	if server, exists := a.operationalServers[info.ZoneID]; exists {
		server.Shutdown()
		delete(a.operationalServers, info.ZoneID)
	}

	// Build instance name: "<ZoneID>-<DeviceID>"
	instanceName := fmt.Sprintf("%s-%s", info.ZoneID, info.DeviceID)
	if len(instanceName) > MaxInstanceNameLen {
		instanceName = instanceName[:MaxInstanceNameLen]
	}

	// Build TXT records
	txtRecords := EncodeOperationalTXT(info)
	txtStrings := TXTRecordsToStrings(txtRecords)

	// Determine port
	port := int(info.Port)
	if port == 0 {
		port = DefaultPort
	}

	// Register service
	var opts []zeroconf.ServerOption
	if a.config.TTL > 0 {
		opts = append(opts, zeroconf.TTL(uint32(a.config.TTL.Seconds())))
	}

	// Get interfaces (nil means all interfaces)
	ifaces := a.getInterfaces()

	server, err := zeroconf.Register(
		instanceName,
		ServiceTypeOperational,
		Domain,
		port,
		txtStrings,
		ifaces,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to register operational service: %w", err)
	}

	a.operationalServers[info.ZoneID] = server
	return nil
}

// UpdateOperational updates TXT records for an operational service.
func (a *MDNSAdvertiser) UpdateOperational(zoneID string, info *OperationalInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	server, exists := a.operationalServers[zoneID]
	if !exists {
		return ErrNotFound
	}

	// Update TXT records
	txtRecords := EncodeOperationalTXT(info)
	txtStrings := TXTRecordsToStrings(txtRecords)
	server.SetText(txtStrings)

	return nil
}

// StopOperational stops advertising operational service for a specific zone.
func (a *MDNSAdvertiser) StopOperational(zoneID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	server, exists := a.operationalServers[zoneID]
	if !exists {
		return ErrNotFound
	}

	server.Shutdown()
	delete(a.operationalServers, zoneID)
	return nil
}

// AdvertiseCommissioner starts advertising a commissioner service.
func (a *MDNSAdvertiser) AdvertiseCommissioner(ctx context.Context, info *CommissionerInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop existing for this zone if any
	if server, exists := a.commissionerServers[info.ZoneID]; exists {
		server.Shutdown()
		delete(a.commissionerServers, info.ZoneID)
	}

	// Build instance name using zone name
	instanceName := info.ZoneName
	if len(instanceName) > MaxInstanceNameLen {
		instanceName = instanceName[:MaxInstanceNameLen]
	}

	// Build TXT records
	txtRecords := EncodeCommissionerTXT(info)
	txtStrings := TXTRecordsToStrings(txtRecords)

	// Determine port
	port := int(info.Port)
	if port == 0 {
		port = DefaultPort
	}

	// Register service
	var opts []zeroconf.ServerOption
	if a.config.TTL > 0 {
		opts = append(opts, zeroconf.TTL(uint32(a.config.TTL.Seconds())))
	}

	// Get interfaces (nil means all interfaces)
	ifaces := a.getInterfaces()

	server, err := zeroconf.Register(
		instanceName,
		ServiceTypeCommissioner,
		Domain,
		port,
		txtStrings,
		ifaces,
		opts...,
	)
	if err != nil {
		return fmt.Errorf("failed to register commissioner service: %w", err)
	}

	a.commissionerServers[info.ZoneID] = server
	return nil
}

// UpdateCommissioner updates TXT records for a commissioner service.
func (a *MDNSAdvertiser) UpdateCommissioner(zoneID string, info *CommissionerInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	server, exists := a.commissionerServers[zoneID]
	if !exists {
		return ErrNotFound
	}

	// Update TXT records
	txtRecords := EncodeCommissionerTXT(info)
	txtStrings := TXTRecordsToStrings(txtRecords)
	server.SetText(txtStrings)

	return nil
}

// StopCommissioner stops advertising commissioner service for a specific zone.
func (a *MDNSAdvertiser) StopCommissioner(zoneID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	server, exists := a.commissionerServers[zoneID]
	if !exists {
		return ErrNotFound
	}

	server.Shutdown()
	delete(a.commissionerServers, zoneID)
	return nil
}

// StopAll stops all advertisements.
func (a *MDNSAdvertiser) StopAll() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.commissionableServer != nil {
		a.commissionableServer.Shutdown()
		a.commissionableServer = nil
	}

	for zoneID, server := range a.operationalServers {
		server.Shutdown()
		delete(a.operationalServers, zoneID)
	}

	for zoneID, server := range a.commissionerServers {
		server.Shutdown()
		delete(a.commissionerServers, zoneID)
	}
}

// MDNSBrowser implements the Browser interface using zeroconf.
type MDNSBrowser struct {
	config BrowserConfig

	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
}

// NewMDNSBrowser creates a new mDNS browser.
func NewMDNSBrowser(config BrowserConfig) (*MDNSBrowser, error) {
	return &MDNSBrowser{
		config: config,
	}, nil
}

// BrowseCommissionable searches for devices in commissioning mode.
// Services are aggregated by instance name - addresses from multiple interfaces
// are combined into a single entry. Removals are handled when interfaces disappear.
func (b *MDNSBrowser) BrowseCommissionable(ctx context.Context) (<-chan *CommissionableService, error) {
	out := make(chan *CommissionableService)

	entries := make(chan *zeroconf.ServiceEntry)
	removed := make(chan *zeroconf.ServiceEntry)

	// Set up browser options
	opts := b.browserOptions()

	// Process entries with aggregation
	go func() {
		defer close(out)

		// Track services by instance name, aggregating addresses
		services := make(map[string]*CommissionableService)

		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				svc := b.entryToCommissionable(entry)
				if svc == nil {
					continue
				}

				existing, found := services[svc.InstanceName]
				if found {
					// Merge addresses into existing entry
					existing.Addresses = mergeAddresses(existing.Addresses, svc.Addresses)
				} else {
					// New service - store and emit
					services[svc.InstanceName] = svc
					select {
					case out <- svc:
					case <-ctx.Done():
						return
					}
				}

			case entry, ok := <-removed:
				if !ok {
					continue
				}
				// Remove addresses that came from this interface
				if existing, found := services[entry.Instance]; found {
					existing.Addresses = removeAddresses(existing.Addresses, entry)
					// If no addresses remain, remove the service
					if len(existing.Addresses) == 0 {
						delete(services, entry.Instance)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Start browsing in background
	go func() {
		_ = zeroconf.Browse(ctx, ServiceTypeCommissionable, Domain, entries, removed, opts...)
	}()

	return out, nil
}

// BrowseOperational searches for commissioned devices.
// Services are aggregated by instance name - addresses from multiple interfaces
// are combined into a single entry.
func (b *MDNSBrowser) BrowseOperational(ctx context.Context, zoneID string) (<-chan *OperationalService, error) {
	out := make(chan *OperationalService)

	entries := make(chan *zeroconf.ServiceEntry)
	removed := make(chan *zeroconf.ServiceEntry)

	// Set up browser options
	opts := b.browserOptions()

	// Process entries with aggregation
	go func() {
		defer close(out)

		// Track services by instance name, aggregating addresses
		services := make(map[string]*OperationalService)

		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				svc := b.entryToOperational(entry)
				if svc == nil {
					continue
				}

				// Filter by zone if specified
				if zoneID != "" && svc.ZoneID != zoneID {
					continue
				}

				existing, found := services[svc.InstanceName]
				if found {
					// Merge addresses into existing entry
					existing.Addresses = mergeAddresses(existing.Addresses, svc.Addresses)
				} else {
					// New service - store and emit
					services[svc.InstanceName] = svc
					select {
					case out <- svc:
					case <-ctx.Done():
						return
					}
				}

			case entry, ok := <-removed:
				if !ok {
					continue
				}
				if existing, found := services[entry.Instance]; found {
					existing.Addresses = removeAddresses(existing.Addresses, entry)
					if len(existing.Addresses) == 0 {
						delete(services, entry.Instance)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Start browsing in background
	go func() {
		_ = zeroconf.Browse(ctx, ServiceTypeOperational, Domain, entries, removed, opts...)
	}()

	return out, nil
}

// BrowseCommissioners searches for zone controllers.
// Services are aggregated by instance name - addresses from multiple interfaces
// are combined into a single entry.
func (b *MDNSBrowser) BrowseCommissioners(ctx context.Context) (<-chan *CommissionerService, error) {
	out := make(chan *CommissionerService)

	entries := make(chan *zeroconf.ServiceEntry)
	removed := make(chan *zeroconf.ServiceEntry)

	// Set up browser options
	opts := b.browserOptions()

	// Process entries with aggregation
	go func() {
		defer close(out)

		// Track services by instance name, aggregating addresses
		services := make(map[string]*CommissionerService)

		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				svc := b.entryToCommissioner(entry)
				if svc == nil {
					continue
				}

				existing, found := services[svc.InstanceName]
				if found {
					// Merge addresses into existing entry
					existing.Addresses = mergeAddresses(existing.Addresses, svc.Addresses)
				} else {
					// New service - store and emit
					services[svc.InstanceName] = svc
					select {
					case out <- svc:
					case <-ctx.Done():
						return
					}
				}

			case entry, ok := <-removed:
				if !ok {
					continue
				}
				if existing, found := services[entry.Instance]; found {
					existing.Addresses = removeAddresses(existing.Addresses, entry)
					if len(existing.Addresses) == 0 {
						delete(services, entry.Instance)
					}
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Start browsing in background
	go func() {
		_ = zeroconf.Browse(ctx, ServiceTypeCommissioner, Domain, entries, removed, opts...)
	}()

	return out, nil
}

// FindByDiscriminator searches for a specific commissionable device.
func (b *MDNSBrowser) FindByDiscriminator(ctx context.Context, discriminator uint16) (*CommissionableService, error) {
	results, err := b.BrowseCommissionable(ctx)
	if err != nil {
		return nil, err
	}

	for {
		select {
		case svc, ok := <-results:
			if !ok {
				return nil, ErrNotFound
			}
			if svc.Discriminator == discriminator {
				return svc, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Stop stops all active browsing operations.
func (b *MDNSBrowser) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.stopped = true
	if b.cancel != nil {
		b.cancel()
	}
}

// browserOptions returns zeroconf client options based on config.
func (b *MDNSBrowser) browserOptions() []zeroconf.ClientOption {
	var opts []zeroconf.ClientOption

	// Select specific interface if configured
	if b.config.Interface != "" {
		iface, err := net.InterfaceByName(b.config.Interface)
		if err == nil {
			opts = append(opts, zeroconf.SelectIfaces([]net.Interface{*iface}))
		}
	}

	return opts
}

// entryToCommissionable converts a zeroconf entry to CommissionableService.
func (b *MDNSBrowser) entryToCommissionable(entry *zeroconf.ServiceEntry) *CommissionableService {
	txt := StringsToTXTRecords(entry.Text)
	info, err := DecodeCommissionableTXT(txt)
	if err != nil {
		return nil
	}

	// Collect addresses
	addrs := make([]string, 0, len(entry.AddrIPv4)+len(entry.AddrIPv6))
	for _, ip := range entry.AddrIPv4 {
		addrs = append(addrs, ip.String())
	}
	for _, ip := range entry.AddrIPv6 {
		addrs = append(addrs, ip.String())
	}

	return &CommissionableService{
		InstanceName:  entry.Instance,
		Host:          entry.HostName,
		Port:          uint16(entry.Port),
		Addresses:     addrs,
		Discriminator: info.Discriminator,
		Categories:    info.Categories,
		Serial:        info.Serial,
		Brand:         info.Brand,
		Model:         info.Model,
		DeviceName:    info.DeviceName,
	}
}

// entryToOperational converts a zeroconf entry to OperationalService.
func (b *MDNSBrowser) entryToOperational(entry *zeroconf.ServiceEntry) *OperationalService {
	txt := StringsToTXTRecords(entry.Text)
	info, err := DecodeOperationalTXT(txt)
	if err != nil {
		return nil
	}

	// Collect addresses
	addrs := make([]string, 0, len(entry.AddrIPv4)+len(entry.AddrIPv6))
	for _, ip := range entry.AddrIPv4 {
		addrs = append(addrs, ip.String())
	}
	for _, ip := range entry.AddrIPv6 {
		addrs = append(addrs, ip.String())
	}

	return &OperationalService{
		InstanceName:  entry.Instance,
		Host:          entry.HostName,
		Port:          uint16(entry.Port),
		Addresses:     addrs,
		ZoneID:        info.ZoneID,
		DeviceID:      info.DeviceID,
		VendorProduct: info.VendorProduct,
		Firmware:      info.Firmware,
		FeatureMap:    info.FeatureMap,
		EndpointCount: info.EndpointCount,
	}
}

// entryToCommissioner converts a zeroconf entry to CommissionerService.
func (b *MDNSBrowser) entryToCommissioner(entry *zeroconf.ServiceEntry) *CommissionerService {
	txt := StringsToTXTRecords(entry.Text)
	info, err := DecodeCommissionerTXT(txt)
	if err != nil {
		return nil
	}

	// Collect addresses
	addrs := make([]string, 0, len(entry.AddrIPv4)+len(entry.AddrIPv6))
	for _, ip := range entry.AddrIPv4 {
		addrs = append(addrs, ip.String())
	}
	for _, ip := range entry.AddrIPv6 {
		addrs = append(addrs, ip.String())
	}

	return &CommissionerService{
		InstanceName:   entry.Instance,
		Host:           entry.HostName,
		Port:           uint16(entry.Port),
		Addresses:      addrs,
		ZoneName:       info.ZoneName,
		ZoneID:         info.ZoneID,
		VendorProduct:  info.VendorProduct,
		ControllerName: info.ControllerName,
		DeviceCount:    info.DeviceCount,
	}
}

// mergeAddresses adds new addresses to existing list, avoiding duplicates.
func mergeAddresses(existing, new []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, addr := range existing {
		seen[addr] = true
	}

	for _, addr := range new {
		if !seen[addr] {
			existing = append(existing, addr)
			seen[addr] = true
		}
	}
	return existing
}

// removeAddresses removes addresses from a zeroconf entry from the list.
func removeAddresses(addresses []string, entry *zeroconf.ServiceEntry) []string {
	// Build set of addresses to remove
	toRemove := make(map[string]bool)
	for _, ip := range entry.AddrIPv4 {
		toRemove[ip.String()] = true
	}
	for _, ip := range entry.AddrIPv6 {
		toRemove[ip.String()] = true
	}

	// Filter out removed addresses
	result := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		if !toRemove[addr] {
			result = append(result, addr)
		}
	}
	return result
}

// Ensure MDNSAdvertiser implements Advertiser interface.
var _ Advertiser = (*MDNSAdvertiser)(nil)

// Ensure MDNSBrowser implements Browser interface.
var _ Browser = (*MDNSBrowser)(nil)

package cert

import (
	"crypto/ecdsa"
	"crypto/x509"
	"sync"
)

// MemoryStore is an in-memory implementation of the Store interface.
// This is primarily useful for testing and devices that don't need persistence.
type MemoryStore struct {
	mu sync.RWMutex

	// Device identity certificate
	identityCert *x509.Certificate
	identityKey  *ecdsa.PrivateKey

	// Operational certificates by zone ID
	operationalCerts map[string]*OperationalCert

	// Zone CA certificates by zone ID
	zoneCACerts map[string]*x509.Certificate
}

// NewMemoryStore creates a new in-memory certificate store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		operationalCerts: make(map[string]*OperationalCert),
		zoneCACerts:      make(map[string]*x509.Certificate),
	}
}

// GetDeviceIdentity returns the device identity certificate and key.
func (s *MemoryStore) GetDeviceIdentity() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.identityCert == nil {
		return nil, nil, ErrCertNotFound
	}
	return s.identityCert, s.identityKey, nil
}

// SetDeviceIdentity stores the device identity certificate and key.
func (s *MemoryStore) SetDeviceIdentity(cert *x509.Certificate, key *ecdsa.PrivateKey) error {
	if cert == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.identityCert = cert
	s.identityKey = key
	return nil
}

// GetOperationalCert returns the operational certificate for a zone.
func (s *MemoryStore) GetOperationalCert(zoneID string) (*OperationalCert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.operationalCerts[zoneID]
	if !exists {
		return nil, ErrCertNotFound
	}
	return cert, nil
}

// SetOperationalCert stores an operational certificate for a zone.
func (s *MemoryStore) SetOperationalCert(cert *OperationalCert) error {
	if cert == nil || cert.Certificate == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if this is a new zone (not an update)
	if _, exists := s.operationalCerts[cert.ZoneID]; !exists {
		if len(s.operationalCerts) >= MaxZones {
			return ErrMaxZonesExceed
		}
	}

	s.operationalCerts[cert.ZoneID] = cert
	return nil
}

// RemoveOperationalCert removes the operational certificate for a zone.
func (s *MemoryStore) RemoveOperationalCert(zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.operationalCerts[zoneID]; !exists {
		return ErrCertNotFound
	}

	delete(s.operationalCerts, zoneID)
	delete(s.zoneCACerts, zoneID) // Also remove the Zone CA
	return nil
}

// ListZones returns all zone IDs the device belongs to.
func (s *MemoryStore) ListZones() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zones := make([]string, 0, len(s.operationalCerts))
	for zoneID := range s.operationalCerts {
		zones = append(zones, zoneID)
	}
	return zones
}

// ZoneCount returns the number of zones the device belongs to.
func (s *MemoryStore) ZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.operationalCerts)
}

// GetZoneCACert returns the Zone CA certificate for a zone.
func (s *MemoryStore) GetZoneCACert(zoneID string) (*x509.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.zoneCACerts[zoneID]
	if !exists {
		return nil, ErrCertNotFound
	}
	return cert, nil
}

// SetZoneCACert stores a Zone CA certificate for a zone.
func (s *MemoryStore) SetZoneCACert(zoneID string, cert *x509.Certificate) error {
	if cert == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.zoneCACerts[zoneID] = cert
	return nil
}

// GetAllZoneCAs returns all stored Zone CA certificates.
func (s *MemoryStore) GetAllZoneCAs() []*x509.Certificate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	certs := make([]*x509.Certificate, 0, len(s.zoneCACerts))
	for _, cert := range s.zoneCACerts {
		certs = append(certs, cert)
	}
	return certs
}

// Save is a no-op for in-memory stores.
func (s *MemoryStore) Save() error {
	return nil
}

// Load is a no-op for in-memory stores.
func (s *MemoryStore) Load() error {
	return nil
}

// Verify MemoryStore implements Store.
var _ Store = (*MemoryStore)(nil)

// MemoryControllerStore extends MemoryStore with Zone CA storage for controllers.
type MemoryControllerStore struct {
	*MemoryStore
	zoneCA         *ZoneCA
	controllerCert *OperationalCert
}

// NewMemoryControllerStore creates a new in-memory controller certificate store.
func NewMemoryControllerStore() *MemoryControllerStore {
	return &MemoryControllerStore{
		MemoryStore: NewMemoryStore(),
	}
}

// GetZoneCA returns the full Zone CA (including private key).
func (s *MemoryControllerStore) GetZoneCA() (*ZoneCA, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.zoneCA == nil {
		return nil, ErrCertNotFound
	}
	return s.zoneCA, nil
}

// SetZoneCA stores the Zone CA (including private key).
func (s *MemoryControllerStore) SetZoneCA(ca *ZoneCA) error {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.zoneCA = ca
	return nil
}

// GetControllerCert returns the controller's operational certificate.
func (s *MemoryControllerStore) GetControllerCert() (*OperationalCert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.controllerCert == nil {
		return nil, ErrCertNotFound
	}
	return s.controllerCert, nil
}

// SetControllerCert stores the controller's operational certificate.
func (s *MemoryControllerStore) SetControllerCert(cert *OperationalCert) error {
	if cert == nil || cert.Certificate == nil || cert.PrivateKey == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.controllerCert = cert
	return nil
}

// Verify MemoryControllerStore implements ControllerStore.
var _ ControllerStore = (*MemoryControllerStore)(nil)

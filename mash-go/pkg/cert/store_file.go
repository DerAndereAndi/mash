package cert

import (
	"crypto/x509"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// File name constants for certificate storage.
const (
	zoneCACertFile = "zone-ca.pem"
	zoneCAKeyFile  = "zone-ca.key"
	zoneCAMetaFile = "zone-ca.json"
)

// FileStore is a file-based implementation of the Store interface.
// Certificates are stored as PEM files with metadata in JSON.
type FileStore struct {
	mu      sync.RWMutex
	baseDir string

	// In-memory state (same as MemoryStore)
	operationalCerts map[string]*OperationalCert
	zoneCACerts      map[string]*x509.Certificate

	// Track removed zones for cleanup on Save
	removedZones map[string]bool
}

// NewFileStore creates a new file-based certificate store.
// The baseDir is the root directory for storing certificates.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{
		baseDir:          baseDir,
		operationalCerts: make(map[string]*OperationalCert),
		zoneCACerts:      make(map[string]*x509.Certificate),
		removedZones:     make(map[string]bool),
	}
}

// GetOperationalCert returns the operational certificate for a zone.
func (s *FileStore) GetOperationalCert(zoneID string) (*OperationalCert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.operationalCerts[zoneID]
	if !exists {
		return nil, ErrCertNotFound
	}
	return cert, nil
}

// SetOperationalCert stores an operational certificate for a zone.
func (s *FileStore) SetOperationalCert(cert *OperationalCert) error {
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
	delete(s.removedZones, cert.ZoneID) // Unmark as removed if re-added
	return nil
}

// RemoveOperationalCert removes the operational certificate for a zone.
func (s *FileStore) RemoveOperationalCert(zoneID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.operationalCerts[zoneID]; !exists {
		return ErrCertNotFound
	}

	delete(s.operationalCerts, zoneID)
	delete(s.zoneCACerts, zoneID)
	s.removedZones[zoneID] = true
	return nil
}

// ListZones returns all zone IDs the device belongs to.
func (s *FileStore) ListZones() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zones := make([]string, 0, len(s.operationalCerts))
	for zoneID := range s.operationalCerts {
		zones = append(zones, zoneID)
	}
	return zones
}

// ZoneCount returns the number of zones the device belongs to.
func (s *FileStore) ZoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.operationalCerts)
}

// GetZoneCACert returns the Zone CA certificate for a zone.
func (s *FileStore) GetZoneCACert(zoneID string) (*x509.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, exists := s.zoneCACerts[zoneID]
	if !exists {
		return nil, ErrCertNotFound
	}
	return cert, nil
}

// SetZoneCACert stores a Zone CA certificate for a zone.
func (s *FileStore) SetZoneCACert(zoneID string, cert *x509.Certificate) error {
	if cert == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.zoneCACerts[zoneID] = cert
	return nil
}

// GetAllZoneCAs returns all stored Zone CA certificates.
func (s *FileStore) GetAllZoneCAs() []*x509.Certificate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	certs := make([]*x509.Certificate, 0, len(s.zoneCACerts))
	for _, cert := range s.zoneCACerts {
		certs = append(certs, cert)
	}
	return certs
}

// Save persists all certificates to disk.
func (s *FileStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create base directory
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}

	// Save operational certs
	for zoneID, opCert := range s.operationalCerts {
		if err := s.saveOperationalCert(zoneID, opCert); err != nil {
			return err
		}
	}

	// Remove deleted zones
	for zoneID := range s.removedZones {
		zoneDir := filepath.Join(s.baseDir, "zones", zoneID)
		_ = os.RemoveAll(zoneDir)
	}

	return nil
}

// Load reads all certificates from disk.
func (s *FileStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if directory exists
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return nil // Empty store
	}

	// Migration: Remove old identity directory if it exists
	// (commissioning certs are no longer persisted)
	oldIdentityDir := filepath.Join(s.baseDir, "identity")
	if _, err := os.Stat(oldIdentityDir); err == nil {
		_ = os.RemoveAll(oldIdentityDir)
	}

	// Load operational certs
	zonesDir := filepath.Join(s.baseDir, "zones")
	entries, err := os.ReadDir(zonesDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		zoneID := entry.Name()
		opCert, err := s.loadOperationalCert(zoneID)
		if err != nil {
			return err
		}
		s.operationalCerts[zoneID] = opCert
		if opCert.ZoneCACert != nil {
			s.zoneCACerts[zoneID] = opCert.ZoneCACert
		}
	}

	return nil
}

// zoneMetadata stores zone-specific metadata in JSON.
type zoneMetadata struct {
	ZoneID   string   `json:"zone_id"`
	ZoneType ZoneType `json:"zone_type"`
}

func (s *FileStore) saveOperationalCert(zoneID string, opCert *OperationalCert) error {
	dir := filepath.Join(s.baseDir, "zones", zoneID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Save operational cert
	certPath := filepath.Join(dir, "operational.pem")
	if err := WriteCertFile(certPath, opCert.Certificate); err != nil {
		return err
	}

	// Save private key
	keyPath := filepath.Join(dir, "operational.key")
	if err := WriteKeyFile(keyPath, opCert.PrivateKey); err != nil {
		return err
	}

	// Save Zone CA cert
	if opCert.ZoneCACert != nil {
		caPath := filepath.Join(dir, zoneCACertFile)
		if err := WriteCertFile(caPath, opCert.ZoneCACert); err != nil {
			return err
		}
	}

	// Save metadata
	meta := zoneMetadata{
		ZoneID:   zoneID,
		ZoneType: opCert.ZoneType,
	}
	metaPath := filepath.Join(dir, "zone.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return err
	}

	return nil
}

func (s *FileStore) loadOperationalCert(zoneID string) (*OperationalCert, error) {
	dir := filepath.Join(s.baseDir, "zones", zoneID)

	// Load operational cert
	certPath := filepath.Join(dir, "operational.pem")
	cert, err := ReadCertFile(certPath)
	if err != nil {
		return nil, err
	}

	// Load private key
	keyPath := filepath.Join(dir, "operational.key")
	key, err := ReadKeyFile(keyPath)
	if err != nil {
		return nil, err
	}

	// Load Zone CA cert (optional)
	var zoneCACert *x509.Certificate
	caPath := filepath.Join(dir, zoneCACertFile)
	if zoneCACert, err = ReadCertFile(caPath); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load metadata
	meta := zoneMetadata{}
	metaPath := filepath.Join(dir, "zone.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &OperationalCert{
		Certificate: cert,
		PrivateKey:  key,
		ZoneID:      zoneID,
		ZoneType:    meta.ZoneType,
		ZoneCACert:  zoneCACert,
	}, nil
}

// Verify FileStore implements Store.
var _ Store = (*FileStore)(nil)

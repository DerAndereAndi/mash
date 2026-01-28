package cert

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Controller-specific file name constants.
// These are used for storing the Zone CA and controller operational certificate.
const (
	controllerDir      = "controller"
	controllerCertFile = "controller.pem"
	controllerKeyFile  = "controller.key"
)

// Controller store errors.
var (
	ErrDeviceNotFound = errors.New("device not found")
	ErrDeviceExists   = errors.New("device already exists")
)

// CommissionedDevice contains information about a device commissioned into the zone.
type CommissionedDevice struct {
	DeviceID   string `json:"device_id"`
	SKI        []byte `json:"-"` // Stored as hex in JSON
	SKIHex     string `json:"ski"`
	DeviceType string `json:"device_type,omitempty"`
}

// FileControllerStore extends FileStore with Zone CA storage for controllers.
type FileControllerStore struct {
	*FileStore
	zoneCA         *ZoneCA
	controllerCert *OperationalCert
	devices        map[string]*CommissionedDevice

	// Track removed devices for cleanup on Save
	removedDevices map[string]bool
}

// NewFileControllerStore creates a new file-based controller certificate store.
func NewFileControllerStore(baseDir string) *FileControllerStore {
	return &FileControllerStore{
		FileStore:      NewFileStore(baseDir),
		devices:        make(map[string]*CommissionedDevice),
		removedDevices: make(map[string]bool),
	}
}

// GetZoneCA returns the full Zone CA (including private key).
func (s *FileControllerStore) GetZoneCA() (*ZoneCA, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.zoneCA == nil {
		return nil, ErrCertNotFound
	}
	return s.zoneCA, nil
}

// SetZoneCA stores the Zone CA (including private key).
func (s *FileControllerStore) SetZoneCA(ca *ZoneCA) error {
	if ca == nil || ca.Certificate == nil || ca.PrivateKey == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.zoneCA = ca
	return nil
}

// GetControllerCert returns the controller's operational certificate.
func (s *FileControllerStore) GetControllerCert() (*OperationalCert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.controllerCert == nil {
		return nil, ErrCertNotFound
	}
	return s.controllerCert, nil
}

// SetControllerCert stores the controller's operational certificate.
func (s *FileControllerStore) SetControllerCert(cert *OperationalCert) error {
	if cert == nil || cert.Certificate == nil || cert.PrivateKey == nil {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.controllerCert = cert
	return nil
}

// AddDevice adds a commissioned device to the store.
func (s *FileControllerStore) AddDevice(device *CommissionedDevice) error {
	if device == nil || device.DeviceID == "" {
		return ErrInvalidCert
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.devices[device.DeviceID] = device
	delete(s.removedDevices, device.DeviceID)
	return nil
}

// GetDevice returns a commissioned device by ID.
func (s *FileControllerStore) GetDevice(deviceID string) (*CommissionedDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, exists := s.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// RemoveDevice removes a commissioned device from the store.
func (s *FileControllerStore) RemoveDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[deviceID]; !exists {
		return ErrDeviceNotFound
	}

	delete(s.devices, deviceID)
	s.removedDevices[deviceID] = true
	return nil
}

// ListDevices returns all commissioned devices.
func (s *FileControllerStore) ListDevices() []*CommissionedDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*CommissionedDevice, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}
	return devices
}

// Save persists all certificates and device info to disk.
func (s *FileControllerStore) Save() error {
	// Save base store first
	if err := s.FileStore.Save(); err != nil {
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Save Zone CA
	if s.zoneCA != nil {
		if err := s.saveZoneCA(); err != nil {
			return err
		}
	}

	// Save Controller Operational Cert
	if s.controllerCert != nil {
		if err := s.saveControllerCert(); err != nil {
			return err
		}
	}

	// Save devices
	for deviceID, device := range s.devices {
		if err := s.saveDevice(deviceID, device); err != nil {
			return err
		}
	}

	// Remove deleted devices
	for deviceID := range s.removedDevices {
		devDir := filepath.Join(s.baseDir, "controller", "devices", deviceID)
		_ = os.RemoveAll(devDir)
	}

	return nil
}

// Load reads all certificates and device info from disk.
func (s *FileControllerStore) Load() error {
	// Load base store first
	if err := s.FileStore.Load(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Load Zone CA
	if err := s.loadZoneCA(); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Load Controller Operational Cert
	if err := s.loadControllerCert(); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Load devices
	devicesDir := filepath.Join(s.baseDir, "controller", "devices")
	entries, err := os.ReadDir(devicesDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		deviceID := entry.Name()
		device, err := s.loadDevice(deviceID)
		if err != nil {
			return err
		}
		s.devices[deviceID] = device
	}

	return nil
}

// zoneCAMetadata stores Zone CA metadata in JSON.
type zoneCAMetadata struct {
	ZoneID   string   `json:"zone_id"`
	ZoneType ZoneType `json:"zone_type"`
}

func (s *FileControllerStore) saveZoneCA() error {
	dir := filepath.Join(s.baseDir, controllerDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Save certificate
	certPath := filepath.Join(dir, zoneCACertFile)
	if err := WriteCertFile(certPath, s.zoneCA.Certificate); err != nil {
		return err
	}

	// Save private key
	keyPath := filepath.Join(dir, zoneCAKeyFile)
	if err := WriteKeyFile(keyPath, s.zoneCA.PrivateKey); err != nil {
		return err
	}

	// Save metadata
	meta := zoneCAMetadata{
		ZoneID:   s.zoneCA.ZoneID,
		ZoneType: s.zoneCA.ZoneType,
	}
	metaPath := filepath.Join(dir, zoneCAMetaFile)
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return err
	}

	return nil
}

func (s *FileControllerStore) loadZoneCA() error {
	dir := filepath.Join(s.baseDir, controllerDir)

	// Load certificate
	certPath := filepath.Join(dir, zoneCACertFile)
	cert, err := ReadCertFile(certPath)
	if err != nil {
		return err
	}

	// Load private key
	keyPath := filepath.Join(dir, zoneCAKeyFile)
	key, err := ReadKeyFile(keyPath)
	if err != nil {
		return err
	}

	// Load metadata
	meta := zoneCAMetadata{}
	metaPath := filepath.Join(dir, zoneCAMetaFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return err
	}

	s.zoneCA = &ZoneCA{
		Certificate: cert,
		PrivateKey:  key,
		ZoneID:      meta.ZoneID,
		ZoneType:    meta.ZoneType,
	}

	return nil
}

func (s *FileControllerStore) saveControllerCert() error {
	dir := filepath.Join(s.baseDir, controllerDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Save certificate
	certPath := filepath.Join(dir, controllerCertFile)
	if err := WriteCertFile(certPath, s.controllerCert.Certificate); err != nil {
		return err
	}

	// Save private key
	keyPath := filepath.Join(dir, controllerKeyFile)
	if err := WriteKeyFile(keyPath, s.controllerCert.PrivateKey); err != nil {
		return err
	}

	return nil
}

func (s *FileControllerStore) loadControllerCert() error {
	dir := filepath.Join(s.baseDir, controllerDir)

	// Load certificate
	certPath := filepath.Join(dir, controllerCertFile)
	cert, err := ReadCertFile(certPath)
	if err != nil {
		return err
	}

	// Load private key
	keyPath := filepath.Join(dir, controllerKeyFile)
	key, err := ReadKeyFile(keyPath)
	if err != nil {
		return err
	}

	// Controller cert metadata comes from Zone CA (same zone)
	s.controllerCert = &OperationalCert{
		Certificate: cert,
		PrivateKey:  key,
		ZoneID:      s.zoneCA.ZoneID,
		ZoneType:    s.zoneCA.ZoneType,
		ZoneCACert:  s.zoneCA.Certificate,
	}

	return nil
}

func (s *FileControllerStore) saveDevice(deviceID string, device *CommissionedDevice) error {
	dir := filepath.Join(s.baseDir, "controller", "devices", deviceID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Encode SKI as hex for JSON
	deviceCopy := *device
	deviceCopy.SKIHex = hex.EncodeToString(device.SKI)

	infoPath := filepath.Join(dir, "info.json")
	data, err := json.MarshalIndent(deviceCopy, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(infoPath, data, 0644); err != nil {
		return err
	}

	return nil
}

func (s *FileControllerStore) loadDevice(deviceID string) (*CommissionedDevice, error) {
	dir := filepath.Join(s.baseDir, "controller", "devices", deviceID)

	infoPath := filepath.Join(dir, "info.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	device := &CommissionedDevice{}
	if err := json.Unmarshal(data, device); err != nil {
		return nil, err
	}

	// Decode SKI from hex
	if device.SKIHex != "" {
		ski, err := hex.DecodeString(device.SKIHex)
		if err != nil {
			return nil, err
		}
		device.SKI = ski
	}

	return device, nil
}

// Verify FileControllerStore implements ControllerStore.
var _ ControllerStore = (*FileControllerStore)(nil)

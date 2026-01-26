package cert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStore(t *testing.T) {
	t.Run("NewFileStore", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)
		if store == nil {
			t.Fatal("NewFileStore() returned nil")
		}
	})

	t.Run("SaveAndLoadEmpty", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Save empty store should work
		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load into new store
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if store2.ZoneCount() != 0 {
			t.Errorf("ZoneCount() = %d, want 0", store2.ZoneCount())
		}
	})

	t.Run("LoadNonExistentDir", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(filepath.Join(dir, "nonexistent"))

		// Load from non-existent should not error (empty store)
		if err := store.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if store.ZoneCount() != 0 {
			t.Errorf("ZoneCount() = %d, want 0", store.ZoneCount())
		}
	})

	t.Run("DeviceAttestationRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Generate and store attestation cert
		kp, _ := GenerateKeyPair()
		cert, _ := GenerateDeviceAttestationCert(kp, &DeviceIdentity{
			DeviceID:  "device-001",
			VendorID:  1234,
			ProductID: 5678,
		}, nil)

		if err := store.SetDeviceAttestation(cert, kp.PrivateKey); err != nil {
			t.Fatalf("SetDeviceAttestation() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify files exist
		if _, err := os.Stat(filepath.Join(dir, "identity", "attestation.pem")); err != nil {
			t.Errorf("attestation.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "identity", "attestation.key")); err != nil {
			t.Errorf("attestation.key not found: %v", err)
		}

		// Load into new store
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		gotCert, gotKey, err := store2.GetDeviceAttestation()
		if err != nil {
			t.Fatalf("GetDeviceAttestation() error = %v", err)
		}
		if gotCert == nil || gotKey == nil {
			t.Error("GetDeviceAttestation() returned nil")
		}
		if gotCert.Subject.CommonName != cert.Subject.CommonName {
			t.Errorf("Subject = %q, want %q", gotCert.Subject.CommonName, cert.Subject.CommonName)
		}
	})

	t.Run("OperationalCertRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Create Zone CA and operational cert
		ca, _ := GenerateZoneCA("zone-1", ZoneTypeHomeManager)
		deviceKP, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
			ZoneID:   "zone-1",
		})
		cert, _ := SignCSR(ca, csrDER)

		opCert := &OperationalCert{
			Certificate: cert,
			PrivateKey:  deviceKP.PrivateKey,
			ZoneID:      "zone-1",
			ZoneType:    ZoneTypeHomeManager,
			ZoneCACert:  ca.Certificate,
		}

		if err := store.SetOperationalCert(opCert); err != nil {
			t.Fatalf("SetOperationalCert() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify directory structure
		zoneDir := filepath.Join(dir, "zones", "zone-1")
		if _, err := os.Stat(filepath.Join(zoneDir, "operational.pem")); err != nil {
			t.Errorf("operational.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(zoneDir, "operational.key")); err != nil {
			t.Errorf("operational.key not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(zoneDir, "zone-ca.pem")); err != nil {
			t.Errorf("zone-ca.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(zoneDir, "zone.json")); err != nil {
			t.Errorf("zone.json not found: %v", err)
		}

		// Load into new store
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := store2.GetOperationalCert("zone-1")
		if err != nil {
			t.Fatalf("GetOperationalCert() error = %v", err)
		}
		if got.ZoneID != "zone-1" {
			t.Errorf("ZoneID = %q, want %q", got.ZoneID, "zone-1")
		}
		if got.ZoneType != ZoneTypeHomeManager {
			t.Errorf("ZoneType = %v, want %v", got.ZoneType, ZoneTypeHomeManager)
		}
		if got.ZoneCACert == nil {
			t.Error("ZoneCACert should not be nil")
		}
	})

	t.Run("MultipleZonesRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Add multiple zones
		for _, zoneID := range []string{"zone-a", "zone-b", "zone-c"} {
			ca, _ := GenerateZoneCA(zoneID, ZoneTypeHomeManager)
			deviceKP, _ := GenerateKeyPair()
			csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
				Identity: DeviceIdentity{DeviceID: "device-001"},
				ZoneID:   zoneID,
			})
			cert, _ := SignCSR(ca, csrDER)

			opCert := &OperationalCert{
				Certificate: cert,
				PrivateKey:  deviceKP.PrivateKey,
				ZoneID:      zoneID,
				ZoneType:    ZoneTypeHomeManager,
				ZoneCACert:  ca.Certificate,
			}
			if err := store.SetOperationalCert(opCert); err != nil {
				t.Fatalf("SetOperationalCert(%s) error = %v", zoneID, err)
			}
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load and verify
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if store2.ZoneCount() != 3 {
			t.Errorf("ZoneCount() = %d, want 3", store2.ZoneCount())
		}

		for _, zoneID := range []string{"zone-a", "zone-b", "zone-c"} {
			if _, err := store2.GetOperationalCert(zoneID); err != nil {
				t.Errorf("GetOperationalCert(%s) error = %v", zoneID, err)
			}
		}
	})

	t.Run("RemoveAndSave", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Add a zone
		ca, _ := GenerateZoneCA("zone-remove", ZoneTypeHomeManager)
		deviceKP, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001"},
			ZoneID:   "zone-remove",
		})
		cert, _ := SignCSR(ca, csrDER)

		opCert := &OperationalCert{
			Certificate: cert,
			PrivateKey:  deviceKP.PrivateKey,
			ZoneID:      "zone-remove",
			ZoneType:    ZoneTypeHomeManager,
			ZoneCACert:  ca.Certificate,
		}
		_ = store.SetOperationalCert(opCert)
		_ = store.Save()

		// Remove and save
		if err := store.RemoveOperationalCert("zone-remove"); err != nil {
			t.Fatalf("RemoveOperationalCert() error = %v", err)
		}
		if err := store.Save(); err != nil {
			t.Fatalf("Save() after remove error = %v", err)
		}

		// Verify directory is removed
		zoneDir := filepath.Join(dir, "zones", "zone-remove")
		if _, err := os.Stat(zoneDir); !os.IsNotExist(err) {
			t.Errorf("zone directory should be removed, got err = %v", err)
		}

		// Load and verify
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if store2.ZoneCount() != 0 {
			t.Errorf("ZoneCount() = %d, want 0", store2.ZoneCount())
		}
	})
}

func TestFileControllerStore(t *testing.T) {
	t.Run("ZoneCARoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		ca, _ := GenerateZoneCA("my-zone", ZoneTypeHomeManager)
		if err := store.SetZoneCA(ca); err != nil {
			t.Fatalf("SetZoneCA() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify files
		ctrlDir := filepath.Join(dir, "controller")
		if _, err := os.Stat(filepath.Join(ctrlDir, "zone-ca.pem")); err != nil {
			t.Errorf("zone-ca.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(ctrlDir, "zone-ca.key")); err != nil {
			t.Errorf("zone-ca.key not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(ctrlDir, "zone-ca.json")); err != nil {
			t.Errorf("zone-ca.json not found: %v", err)
		}

		// Load and verify
		store2 := NewFileControllerStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := store2.GetZoneCA()
		if err != nil {
			t.Fatalf("GetZoneCA() error = %v", err)
		}
		if got.ZoneID != "my-zone" {
			t.Errorf("ZoneID = %q, want %q", got.ZoneID, "my-zone")
		}
		if got.ZoneType != ZoneTypeHomeManager {
			t.Errorf("ZoneType = %v, want %v", got.ZoneType, ZoneTypeHomeManager)
		}
		if got.PrivateKey == nil {
			t.Error("PrivateKey should not be nil")
		}
	})

	t.Run("DeviceInfoRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		info := &CommissionedDevice{
			DeviceID:   "dev-123",
			SKI:        []byte{0x01, 0x02, 0x03},
			DeviceType: "EVSE",
		}

		if err := store.AddDevice(info); err != nil {
			t.Fatalf("AddDevice() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Load and verify
		store2 := NewFileControllerStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, err := store2.GetDevice("dev-123")
		if err != nil {
			t.Fatalf("GetDevice() error = %v", err)
		}
		if got.DeviceID != "dev-123" {
			t.Errorf("DeviceID = %q, want %q", got.DeviceID, "dev-123")
		}
		if got.DeviceType != "EVSE" {
			t.Errorf("DeviceType = %q, want %q", got.DeviceType, "EVSE")
		}
	})

	t.Run("ListDevices", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		// Add multiple devices
		for i, id := range []string{"dev-a", "dev-b", "dev-c"} {
			info := &CommissionedDevice{
				DeviceID:   id,
				SKI:        []byte{byte(i)},
				DeviceType: "EVSE",
			}
			_ = store.AddDevice(info)
		}

		_ = store.Save()

		// Load and list
		store2 := NewFileControllerStore(dir)
		_ = store2.Load()

		devices := store2.ListDevices()
		if len(devices) != 3 {
			t.Errorf("ListDevices() = %d, want 3", len(devices))
		}
	})

	t.Run("RemoveDevice", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		info := &CommissionedDevice{
			DeviceID: "dev-remove",
			SKI:      []byte{0x01},
		}
		_ = store.AddDevice(info)
		_ = store.Save()

		if err := store.RemoveDevice("dev-remove"); err != nil {
			t.Fatalf("RemoveDevice() error = %v", err)
		}
		_ = store.Save()

		// Verify file removed
		devDir := filepath.Join(dir, "controller", "devices", "dev-remove")
		if _, err := os.Stat(devDir); !os.IsNotExist(err) {
			t.Errorf("device directory should be removed")
		}

		// Load and verify
		store2 := NewFileControllerStore(dir)
		_ = store2.Load()

		if len(store2.ListDevices()) != 0 {
			t.Error("device should be removed")
		}
	})
}

func TestFileStoreInterfaceImplementation(t *testing.T) {
	// Verify interface implementations at compile time
	var _ Store = (*FileStore)(nil)
	var _ ControllerStore = (*FileControllerStore)(nil)
}

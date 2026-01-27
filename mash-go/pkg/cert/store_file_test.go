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

	t.Run("DeviceIdentityRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Generate and store identity cert
		ca, _ := GenerateZoneCA("test-zone", ZoneTypeLocal)
		kp, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(kp, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1234, ProductID: 5678},
			ZoneID:   "test-zone",
		})
		cert, _ := SignCSR(ca, csrDER)

		if err := store.SetDeviceIdentity(cert, kp.PrivateKey); err != nil {
			t.Fatalf("SetDeviceIdentity() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify files exist
		if _, err := os.Stat(filepath.Join(dir, "identity", "identity.pem")); err != nil {
			t.Errorf("identity.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "identity", "identity.key")); err != nil {
			t.Errorf("identity.key not found: %v", err)
		}

		// Load into new store
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		gotCert, gotKey, err := store2.GetDeviceIdentity()
		if err != nil {
			t.Fatalf("GetDeviceIdentity() error = %v", err)
		}
		if gotCert == nil || gotKey == nil {
			t.Error("GetDeviceIdentity() returned nil")
		}
		if gotCert.Subject.CommonName != cert.Subject.CommonName {
			t.Errorf("Subject = %q, want %q", gotCert.Subject.CommonName, cert.Subject.CommonName)
		}
	})

	t.Run("OperationalCertRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Create Zone CA and operational cert
		ca, _ := GenerateZoneCA("zone-1", ZoneTypeLocal)
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
			ZoneType:    ZoneTypeLocal,
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
		if got.ZoneType != ZoneTypeLocal {
			t.Errorf("ZoneType = %v, want %v", got.ZoneType, ZoneTypeLocal)
		}
		if got.ZoneCACert == nil {
			t.Error("ZoneCACert should not be nil")
		}
	})

	t.Run("MultipleZonesRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Add MaxZones zones (one GRID + one LOCAL per DEC-043)
		zoneConfigs := []struct {
			id       string
			zoneType ZoneType
		}{
			{"zone-grid", ZoneTypeGrid},
			{"zone-local", ZoneTypeLocal},
		}
		for _, cfg := range zoneConfigs {
			ca, _ := GenerateZoneCA(cfg.id, cfg.zoneType)
			deviceKP, _ := GenerateKeyPair()
			csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
				Identity: DeviceIdentity{DeviceID: "device-001"},
				ZoneID:   cfg.id,
			})
			cert, _ := SignCSR(ca, csrDER)

			opCert := &OperationalCert{
				Certificate: cert,
				PrivateKey:  deviceKP.PrivateKey,
				ZoneID:      cfg.id,
				ZoneType:    cfg.zoneType,
				ZoneCACert:  ca.Certificate,
			}
			if err := store.SetOperationalCert(opCert); err != nil {
				t.Fatalf("SetOperationalCert(%s) error = %v", cfg.id, err)
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

		if store2.ZoneCount() != MaxZones {
			t.Errorf("ZoneCount() = %d, want %d", store2.ZoneCount(), MaxZones)
		}

		for _, cfg := range zoneConfigs {
			if _, err := store2.GetOperationalCert(cfg.id); err != nil {
				t.Errorf("GetOperationalCert(%s) error = %v", cfg.id, err)
			}
		}
	})

	t.Run("RemoveAndSave", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Add a zone
		ca, _ := GenerateZoneCA("zone-remove", ZoneTypeLocal)
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
			ZoneType:    ZoneTypeLocal,
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

		ca, _ := GenerateZoneCA("my-zone", ZoneTypeLocal)
		if err := store.SetZoneCA(ca); err != nil {
			t.Fatalf("SetZoneCA() error = %v", err)
		}

		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify files
		ctrlDir := filepath.Join(dir, "identity")
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
		if got.ZoneType != ZoneTypeLocal {
			t.Errorf("ZoneType = %v, want %v", got.ZoneType, ZoneTypeLocal)
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
		devDir := filepath.Join(dir, "identity", "devices", "dev-remove")
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

	// TC-IMPL-CERT-STORE-001: Save Controller Operational Cert
	t.Run("ControllerCertRoundTrip", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		// Create Zone CA first (required for controller cert)
		ca, err := GenerateZoneCA("my-zone", ZoneTypeLocal)
		if err != nil {
			t.Fatalf("GenerateZoneCA() error = %v", err)
		}
		if err := store.SetZoneCA(ca); err != nil {
			t.Fatalf("SetZoneCA() error = %v", err)
		}

		// Generate controller operational cert
		controllerCert, err := GenerateControllerOperationalCert(ca, "controller-001")
		if err != nil {
			t.Fatalf("GenerateControllerOperationalCert() error = %v", err)
		}

		// Store controller cert
		if err := store.SetControllerCert(controllerCert); err != nil {
			t.Fatalf("SetControllerCert() error = %v", err)
		}

		// Save to disk
		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Verify files exist
		ctrlDir := filepath.Join(dir, "identity")
		if _, err := os.Stat(filepath.Join(ctrlDir, "identity.pem")); err != nil {
			t.Errorf("identity.pem not found: %v", err)
		}
		if _, err := os.Stat(filepath.Join(ctrlDir, "identity.key")); err != nil {
			t.Errorf("identity.key not found: %v", err)
		}

		// TC-IMPL-CERT-STORE-002: Load Controller Operational Cert
		// Load into new store
		store2 := NewFileControllerStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		loadedCert, err := store2.GetControllerCert()
		if err != nil {
			t.Fatalf("GetControllerCert() error = %v", err)
		}

		// Verify loaded cert matches original
		if loadedCert.Certificate == nil {
			t.Fatal("Loaded certificate should not be nil")
		}
		if loadedCert.Certificate.Subject.CommonName != "controller-001" {
			t.Errorf("Subject.CommonName = %q, want %q", loadedCert.Certificate.Subject.CommonName, "controller-001")
		}
		if loadedCert.ZoneID != "my-zone" {
			t.Errorf("ZoneID = %q, want %q", loadedCert.ZoneID, "my-zone")
		}
		if loadedCert.ZoneType != ZoneTypeLocal {
			t.Errorf("ZoneType = %v, want %v", loadedCert.ZoneType, ZoneTypeLocal)
		}
	})

	// TC-IMPL-CERT-STORE-003: Controller Cert Not Found
	t.Run("ControllerCertNotFound", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		// Should return error when no cert exists
		_, err := store.GetControllerCert()
		if err != ErrCertNotFound {
			t.Errorf("GetControllerCert() error = %v, want ErrCertNotFound", err)
		}
	})

	// TC-IMPL-CERT-STORE: Invalid controller cert rejected
	t.Run("ControllerCertInvalid", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileControllerStore(dir)

		// Should reject nil cert
		err := store.SetControllerCert(nil)
		if err != ErrInvalidCert {
			t.Errorf("SetControllerCert(nil) error = %v, want ErrInvalidCert", err)
		}

		// Should reject cert with nil Certificate
		err = store.SetControllerCert(&OperationalCert{})
		if err != ErrInvalidCert {
			t.Errorf("SetControllerCert(empty) error = %v, want ErrInvalidCert", err)
		}
	})
}

func TestFileStoreInterfaceImplementation(t *testing.T) {
	// Verify interface implementations at compile time
	var _ Store = (*FileStore)(nil)
	var _ ControllerStore = (*FileControllerStore)(nil)
}

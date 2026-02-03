package cert

import (
	"testing"
)

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	t.Run("InitialState", func(t *testing.T) {
		if store.ZoneCount() != 0 {
			t.Errorf("ZoneCount() = %d, want 0", store.ZoneCount())
		}
		if zones := store.ListZones(); len(zones) != 0 {
			t.Errorf("ListZones() = %v, want empty", zones)
		}
	})

	t.Run("OperationalCerts", func(t *testing.T) {
		// Create Zone CA
		ca, _ := GenerateZoneCA("zone-1", ZoneTypeLocal)

		// Generate device cert
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

		// Initially no cert
		_, err := store.GetOperationalCert("zone-1")
		if err != ErrCertNotFound {
			t.Errorf("GetOperationalCert() error = %v, want ErrCertNotFound", err)
		}

		// Store cert
		err = store.SetOperationalCert(opCert)
		if err != nil {
			t.Fatalf("SetOperationalCert() error = %v", err)
		}

		// Retrieve
		got, err := store.GetOperationalCert("zone-1")
		if err != nil {
			t.Errorf("GetOperationalCert() error = %v", err)
		}
		if got.ZoneID != "zone-1" {
			t.Errorf("ZoneID = %q, want %q", got.ZoneID, "zone-1")
		}

		// Check zone count
		if store.ZoneCount() != 1 {
			t.Errorf("ZoneCount() = %d, want 1", store.ZoneCount())
		}

		// List zones
		zones := store.ListZones()
		if len(zones) != 1 || zones[0] != "zone-1" {
			t.Errorf("ListZones() = %v, want [zone-1]", zones)
		}
	})

	t.Run("RemoveOperationalCert", func(t *testing.T) {
		// Remove non-existent
		err := store.RemoveOperationalCert("non-existent")
		if err != ErrCertNotFound {
			t.Errorf("RemoveOperationalCert() error = %v, want ErrCertNotFound", err)
		}

		// Remove existing (from previous test)
		err = store.RemoveOperationalCert("zone-1")
		if err != nil {
			t.Errorf("RemoveOperationalCert() error = %v", err)
		}

		// Verify removed
		if store.ZoneCount() != 0 {
			t.Errorf("ZoneCount() = %d, want 0", store.ZoneCount())
		}
	})
}

func TestMemoryStoreMaxZones(t *testing.T) {
	store := NewMemoryStore()

	// Create MaxZones zones (GRID + LOCAL + TEST per DEC-060)
	zoneTypes := []ZoneType{ZoneTypeGrid, ZoneTypeLocal, ZoneTypeTest}
	for i := 0; i < MaxZones; i++ {
		zoneID := "zone-" + string(rune('A'+i))
		zoneType := zoneTypes[i]
		ca, _ := GenerateZoneCA(zoneID, zoneType)
		deviceKP, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
			ZoneID:   ca.ZoneID,
		})
		cert, _ := SignCSR(ca, csrDER)

		opCert := &OperationalCert{
			Certificate: cert,
			PrivateKey:  deviceKP.PrivateKey,
			ZoneID:      ca.ZoneID,
			ZoneType:    zoneType,
			ZoneCACert:  ca.Certificate,
		}

		err := store.SetOperationalCert(opCert)
		if err != nil {
			t.Fatalf("SetOperationalCert(zone-%d) error = %v", i, err)
		}
	}

	if store.ZoneCount() != MaxZones {
		t.Errorf("ZoneCount() = %d, want %d", store.ZoneCount(), MaxZones)
	}

	// Try to add another zone - should fail with ErrMaxZonesExceed
	// Note: The cert store doesn't check zone types, only capacity.
	// Zone type enforcement is done at the zone.Manager level.
	ca3, _ := GenerateZoneCA("zone-3", ZoneTypeGrid)
	deviceKP, _ := GenerateKeyPair()
	csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
		Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
		ZoneID:   "zone-3",
	})
	cert3, _ := SignCSR(ca3, csrDER)

	opCert3 := &OperationalCert{
		Certificate: cert3,
		PrivateKey:  deviceKP.PrivateKey,
		ZoneID:      "zone-3",
		ZoneType:    ZoneTypeGrid,
		ZoneCACert:  ca3.Certificate,
	}

	err := store.SetOperationalCert(opCert3)
	if err != ErrMaxZonesExceed {
		t.Errorf("SetOperationalCert(zone-3) error = %v, want ErrMaxZonesExceed", err)
	}
}

func TestMemoryStoreZoneCACerts(t *testing.T) {
	store := NewMemoryStore()

	// No Zone CAs initially
	certs := store.GetAllZoneCAs()
	if len(certs) != 0 {
		t.Errorf("GetAllZoneCAs() = %d certs, want 0", len(certs))
	}

	// Add Zone CA
	ca, _ := GenerateZoneCA("zone-1", ZoneTypeLocal)
	err := store.SetZoneCACert("zone-1", ca.Certificate)
	if err != nil {
		t.Fatalf("SetZoneCACert() error = %v", err)
	}

	// Retrieve
	gotCert, err := store.GetZoneCACert("zone-1")
	if err != nil {
		t.Errorf("GetZoneCACert() error = %v", err)
	}
	if gotCert == nil {
		t.Error("GetZoneCACert() returned nil")
	}

	// Get non-existent
	_, err = store.GetZoneCACert("non-existent")
	if err != ErrCertNotFound {
		t.Errorf("GetZoneCACert(non-existent) error = %v, want ErrCertNotFound", err)
	}

	// Get all
	certs = store.GetAllZoneCAs()
	if len(certs) != 1 {
		t.Errorf("GetAllZoneCAs() = %d certs, want 1", len(certs))
	}
}

func TestMemoryControllerStore(t *testing.T) {
	store := NewMemoryControllerStore()

	t.Run("InitialState", func(t *testing.T) {
		_, err := store.GetZoneCA()
		if err != ErrCertNotFound {
			t.Errorf("GetZoneCA() error = %v, want ErrCertNotFound", err)
		}
	})

	t.Run("SetAndGetZoneCA", func(t *testing.T) {
		ca, _ := GenerateZoneCA("my-zone", ZoneTypeLocal)
		err := store.SetZoneCA(ca)
		if err != nil {
			t.Fatalf("SetZoneCA() error = %v", err)
		}

		got, err := store.GetZoneCA()
		if err != nil {
			t.Errorf("GetZoneCA() error = %v", err)
		}
		if got.ZoneID != "my-zone" {
			t.Errorf("ZoneID = %q, want %q", got.ZoneID, "my-zone")
		}
		if got.PrivateKey == nil {
			t.Error("PrivateKey should not be nil")
		}
	})

	t.Run("InvalidZoneCA", func(t *testing.T) {
		err := store.SetZoneCA(nil)
		if err != ErrInvalidCert {
			t.Errorf("SetZoneCA(nil) error = %v, want ErrInvalidCert", err)
		}

		err = store.SetZoneCA(&ZoneCA{})
		if err != ErrInvalidCert {
			t.Errorf("SetZoneCA(empty) error = %v, want ErrInvalidCert", err)
		}
	})
}

func TestStoreInterfaceImplementation(t *testing.T) {
	// Verify interface implementations at compile time
	var _ Store = (*MemoryStore)(nil)
	var _ ControllerStore = (*MemoryControllerStore)(nil)
}

func TestZoneCAPersistence(t *testing.T) {
	// This test verifies that Zone CA certs set separately from operational certs
	// are correctly persisted. This mirrors the actual commissioning flow:
	// 1. SetOperationalCert is called with the device's new operational cert
	// 2. SetZoneCACert is called with the Zone CA cert
	// 3. Save persists both to disk

	t.Run("MemoryStore_SetZoneCASeparately", func(t *testing.T) {
		store := NewMemoryStore()

		// Create Zone CA and device cert
		ca, _ := GenerateZoneCA("zone-1", ZoneTypeLocal)
		deviceKP, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
			ZoneID:   "zone-1",
		})
		cert, _ := SignCSR(ca, csrDER)

		// Step 1: Set operational cert WITHOUT ZoneCACert
		opCert := &OperationalCert{
			Certificate: cert,
			PrivateKey:  deviceKP.PrivateKey,
			ZoneID:      "zone-1",
			ZoneType:    ZoneTypeLocal,
			ZoneCACert:  nil, // Not set initially
		}
		if err := store.SetOperationalCert(opCert); err != nil {
			t.Fatalf("SetOperationalCert() error = %v", err)
		}

		// Step 2: Set Zone CA separately (as happens during cert exchange)
		if err := store.SetZoneCACert("zone-1", ca.Certificate); err != nil {
			t.Fatalf("SetZoneCACert() error = %v", err)
		}

		// Verify Zone CA is retrievable
		gotCA, err := store.GetZoneCACert("zone-1")
		if err != nil {
			t.Errorf("GetZoneCACert() error = %v", err)
		}
		if gotCA == nil {
			t.Error("GetZoneCACert() returned nil")
		}

		// Verify operational cert's ZoneCACert field was updated
		gotOp, err := store.GetOperationalCert("zone-1")
		if err != nil {
			t.Fatalf("GetOperationalCert() error = %v", err)
		}
		if gotOp.ZoneCACert == nil {
			t.Error("OperationalCert.ZoneCACert should be set after SetZoneCACert()")
		}
	})

	t.Run("FileStore_ZoneCAPersisted", func(t *testing.T) {
		dir := t.TempDir()
		store := NewFileStore(dir)

		// Create Zone CA and device cert
		ca, _ := GenerateZoneCA("zone-1", ZoneTypeLocal)
		deviceKP, _ := GenerateKeyPair()
		csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
			Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
			ZoneID:   "zone-1",
		})
		cert, _ := SignCSR(ca, csrDER)

		// Step 1: Set operational cert WITHOUT ZoneCACert
		opCert := &OperationalCert{
			Certificate: cert,
			PrivateKey:  deviceKP.PrivateKey,
			ZoneID:      "zone-1",
			ZoneType:    ZoneTypeLocal,
			ZoneCACert:  nil, // Not set initially
		}
		if err := store.SetOperationalCert(opCert); err != nil {
			t.Fatalf("SetOperationalCert() error = %v", err)
		}

		// Step 2: Set Zone CA separately
		if err := store.SetZoneCACert("zone-1", ca.Certificate); err != nil {
			t.Fatalf("SetZoneCACert() error = %v", err)
		}

		// Step 3: Save to disk
		if err := store.Save(); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		// Step 4: Load into a fresh store
		store2 := NewFileStore(dir)
		if err := store2.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Verify Zone CA was loaded
		gotCA, err := store2.GetZoneCACert("zone-1")
		if err != nil {
			t.Errorf("GetZoneCACert() after Load() error = %v", err)
		}
		if gotCA == nil {
			t.Error("Zone CA should be persisted and loaded")
		}
	})
}

package cert

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if kp.PrivateKey == nil {
		t.Error("PrivateKey should not be nil")
	}
	if kp.PublicKey == nil {
		t.Error("PublicKey should not be nil")
	}

	// Verify it's P-256
	if kp.PrivateKey.Curve.Params().Name != "P-256" {
		t.Errorf("Expected P-256 curve, got %s", kp.PrivateKey.Curve.Params().Name)
	}
}

func TestComputeSKI(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	ski, err := ComputeSKI(kp.PublicKey)
	if err != nil {
		t.Fatalf("ComputeSKI() error = %v", err)
	}

	// SKI should be 20 bytes (160 bits)
	if len(ski) != 20 {
		t.Errorf("SKI length = %d, want 20", len(ski))
	}

	// Same key should produce same SKI
	ski2, _ := ComputeSKI(kp.PublicKey)
	if !bytesEqual(ski, ski2) {
		t.Error("Same key should produce same SKI")
	}

	// Different key should produce different SKI
	kp2, _ := GenerateKeyPair()
	ski3, _ := ComputeSKI(kp2.PublicKey)
	if bytesEqual(ski, ski3) {
		t.Error("Different keys should produce different SKIs")
	}
}

func TestGenerateZoneCA(t *testing.T) {
	tests := []struct {
		name     string
		zoneID   string
		zoneType ZoneType
	}{
		{"GridOperator", "grid-operator-1", ZoneTypeGridOperator},
		{"HomeManager", "home-ems-1", ZoneTypeHomeManager},
		{"UserApp", "mobile-app-1", ZoneTypeUserApp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca, err := GenerateZoneCA(tt.zoneID, tt.zoneType)
			if err != nil {
				t.Fatalf("GenerateZoneCA() error = %v", err)
			}

			// Check Zone CA properties
			if ca.ZoneID != tt.zoneID {
				t.Errorf("ZoneID = %q, want %q", ca.ZoneID, tt.zoneID)
			}
			if ca.ZoneType != tt.zoneType {
				t.Errorf("ZoneType = %v, want %v", ca.ZoneType, tt.zoneType)
			}
			if ca.Certificate == nil {
				t.Error("Certificate should not be nil")
			}
			if ca.PrivateKey == nil {
				t.Error("PrivateKey should not be nil")
			}

			// Check certificate properties
			cert := ca.Certificate
			if !cert.IsCA {
				t.Error("Certificate should be a CA")
			}
			if cert.MaxPathLen != 0 || !cert.MaxPathLenZero {
				t.Error("MaxPathLen should be 0")
			}

			// Check validity (10 years)
			expectedDuration := ZoneCAValidity
			actualDuration := cert.NotAfter.Sub(cert.NotBefore)
			// Allow 1 second tolerance for test execution time
			if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
				t.Errorf("Validity duration = %v, want ~%v", actualDuration, expectedDuration)
			}

			// Check self-signed
			if !bytesEqual(cert.SubjectKeyId, cert.AuthorityKeyId) {
				t.Error("Zone CA should be self-signed (SKI == AKI)")
			}
		})
	}
}

func TestCreateCSRAndSign(t *testing.T) {
	// Create a Zone CA
	ca, err := GenerateZoneCA("test-zone", ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("GenerateZoneCA() error = %v", err)
	}

	// Generate device key pair
	deviceKP, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Create CSR
	csrInfo := &CSRInfo{
		Identity: DeviceIdentity{
			DeviceID:     "device-001",
			VendorID:     1234,
			ProductID:    5678,
			SerialNumber: "SN-12345",
		},
		ZoneID: "test-zone",
	}

	csrDER, err := CreateCSR(deviceKP, csrInfo)
	if err != nil {
		t.Fatalf("CreateCSR() error = %v", err)
	}

	// Parse CSR to verify
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		t.Fatalf("ParseCertificateRequest() error = %v", err)
	}

	if csr.Subject.CommonName != "device-001" {
		t.Errorf("CSR CommonName = %q, want %q", csr.Subject.CommonName, "device-001")
	}

	// Sign CSR
	cert, err := SignCSR(ca, csrDER)
	if err != nil {
		t.Fatalf("SignCSR() error = %v", err)
	}

	// Verify certificate properties
	if cert.IsCA {
		t.Error("Operational certificate should not be a CA")
	}
	if cert.Subject.CommonName != "device-001" {
		t.Errorf("Certificate CommonName = %q, want %q", cert.Subject.CommonName, "device-001")
	}

	// Verify signed by Zone CA
	if !bytesEqual(cert.AuthorityKeyId, ca.Certificate.SubjectKeyId) {
		t.Error("Certificate should be signed by Zone CA")
	}

	// Check validity (1 year)
	expectedDuration := OperationalCertValidity
	actualDuration := cert.NotAfter.Sub(cert.NotBefore)
	if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
		t.Errorf("Validity duration = %v, want ~%v", actualDuration, expectedDuration)
	}
}

func TestVerifyOperationalCert(t *testing.T) {
	// Create Zone CA
	ca, err := GenerateZoneCA("test-zone", ZoneTypeHomeManager)
	if err != nil {
		t.Fatalf("GenerateZoneCA() error = %v", err)
	}

	// Generate and sign device certificate
	deviceKP, _ := GenerateKeyPair()
	csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
		Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
		ZoneID:   "test-zone",
	})
	cert, _ := SignCSR(ca, csrDER)

	t.Run("ValidCert", func(t *testing.T) {
		err := VerifyOperationalCert(cert, ca.Certificate)
		if err != nil {
			t.Errorf("VerifyOperationalCert() error = %v", err)
		}
	})

	t.Run("WrongCA", func(t *testing.T) {
		otherCA, _ := GenerateZoneCA("other-zone", ZoneTypeGridOperator)
		err := VerifyOperationalCert(cert, otherCA.Certificate)
		if err == nil {
			t.Error("VerifyOperationalCert() should fail with wrong CA")
		}
	})

	t.Run("NilCert", func(t *testing.T) {
		err := VerifyOperationalCert(nil, ca.Certificate)
		if err == nil {
			t.Error("VerifyOperationalCert() should fail with nil cert")
		}
	})

	t.Run("NilCA", func(t *testing.T) {
		err := VerifyOperationalCert(cert, nil)
		if err == nil {
			t.Error("VerifyOperationalCert() should fail with nil CA")
		}
	})
}

func TestOperationalCertExpiry(t *testing.T) {
	// Create a cert with known expiry
	ca, _ := GenerateZoneCA("test-zone", ZoneTypeHomeManager)
	deviceKP, _ := GenerateKeyPair()
	csrDER, _ := CreateCSR(deviceKP, &CSRInfo{
		Identity: DeviceIdentity{DeviceID: "device-001", VendorID: 1, ProductID: 1},
		ZoneID:   "test-zone",
	})
	cert, _ := SignCSR(ca, csrDER)

	opCert := &OperationalCert{
		Certificate: cert,
		PrivateKey:  deviceKP.PrivateKey,
		ZoneID:      "test-zone",
		ZoneType:    ZoneTypeHomeManager,
		ZoneCACert:  ca.Certificate,
	}

	t.Run("NotExpired", func(t *testing.T) {
		if opCert.IsExpired() {
			t.Error("Fresh certificate should not be expired")
		}
	})

	t.Run("NeedsRenewalFresh", func(t *testing.T) {
		// Fresh cert should not need renewal (335+ days until expiry)
		if opCert.NeedsRenewal() {
			t.Error("Fresh certificate should not need renewal")
		}
	})

	t.Run("SKI", func(t *testing.T) {
		ski := opCert.SKI()
		if len(ski) == 0 {
			t.Error("SKI should not be empty")
		}
	})

	t.Run("ExpiresAt", func(t *testing.T) {
		exp := opCert.ExpiresAt()
		if exp.IsZero() {
			t.Error("ExpiresAt should return valid time")
		}
	})
}

func TestZoneTypeString(t *testing.T) {
	tests := []struct {
		zoneType ZoneType
		want     string
		priority uint8
	}{
		{ZoneTypeGridOperator, "GRID_OPERATOR", 1},
		{ZoneTypeBuildingManager, "BUILDING_MANAGER", 2},
		{ZoneTypeHomeManager, "HOME_MANAGER", 3},
		{ZoneTypeUserApp, "USER_APP", 4},
		{ZoneType(99), "UNKNOWN", 99},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.zoneType.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
			if got := tt.zoneType.Priority(); got != tt.priority {
				t.Errorf("Priority() = %d, want %d", got, tt.priority)
			}
		})
	}
}

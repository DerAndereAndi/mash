package discovery

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseQRCode parses a MASH QR code string.
//
// Format: MASH:<version>:<discriminator>:<setupcode>
//
// Example: MASH:1:1234:12345678
func ParseQRCode(content string) (*QRCode, error) {
	// Check prefix
	if !strings.HasPrefix(content, QRPrefix) {
		return nil, ErrInvalidPrefix
	}

	// Split into parts
	parts := strings.Split(content, ":")
	if len(parts) != 4 {
		return nil, ErrInvalidFieldCount
	}

	// Parse version
	version, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil || version < 1 || version > 255 {
		return nil, ErrInvalidVersion
	}

	// Parse discriminator
	discriminator, err := strconv.ParseUint(parts[2], 10, 16)
	if err != nil || discriminator > MaxDiscriminator {
		return nil, ErrInvalidDiscriminator
	}

	// Validate setup code (must be 8 digits)
	setupCode := parts[3]
	if len(setupCode) != SetupCodeLength {
		return nil, ErrInvalidSetupCode
	}
	for _, c := range setupCode {
		if c < '0' || c > '9' {
			return nil, ErrInvalidSetupCode
		}
	}

	return &QRCode{
		Version:       uint8(version),
		Discriminator: uint16(discriminator),
		SetupCode:     setupCode,
	}, nil
}

// String returns the QR code as a string suitable for encoding.
//
// The setup code is always formatted with leading zeros to ensure 8 digits.
func (qr *QRCode) String() string {
	return fmt.Sprintf("MASH:%d:%d:%s", qr.Version, qr.Discriminator, qr.SetupCode)
}

// NewQRCode creates a new QR code with the given parameters.
//
// The setupCode must be an 8-digit string (with leading zeros if needed).
// Use FormatSetupCode to convert a numeric setup code to string format.
func NewQRCode(discriminator uint16, setupCode string) (*QRCode, error) {
	if discriminator > MaxDiscriminator {
		return nil, ErrInvalidDiscriminator
	}

	if len(setupCode) != SetupCodeLength {
		return nil, ErrInvalidSetupCode
	}
	for _, c := range setupCode {
		if c < '0' || c > '9' {
			return nil, ErrInvalidSetupCode
		}
	}

	return &QRCode{
		Version:       QRVersion,
		Discriminator: discriminator,
		SetupCode:     setupCode,
	}, nil
}

// FormatSetupCode converts a numeric setup code to the 8-digit string format.
//
// Example: FormatSetupCode(1234) returns "00001234"
func FormatSetupCode(code uint32) string {
	return fmt.Sprintf("%08d", code)
}

// ParseSetupCode parses an 8-digit setup code string to a number.
//
// Note: The string format should be preferred to preserve leading zeros.
func ParseSetupCode(s string) (uint32, error) {
	if len(s) != SetupCodeLength {
		return 0, ErrInvalidSetupCode
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, ErrInvalidSetupCode
	}
	return uint32(n), nil
}

// InstanceNameFromDiscriminator creates the instance name for commissionable discovery.
//
// Format: MASH-<discriminator>
func InstanceNameFromDiscriminator(discriminator uint16) string {
	return fmt.Sprintf("MASH-%d", discriminator)
}

// DiscriminatorFromInstanceName extracts the discriminator from a commissionable instance name.
//
// Returns an error if the instance name is not in the expected format.
func DiscriminatorFromInstanceName(name string) (uint16, error) {
	if !strings.HasPrefix(name, "MASH-") {
		return 0, ErrInvalidQRCode
	}

	dStr := strings.TrimPrefix(name, "MASH-")
	d, err := strconv.ParseUint(dStr, 10, 16)
	if err != nil || d > MaxDiscriminator {
		return 0, ErrInvalidDiscriminator
	}

	return uint16(d), nil
}

// OperationalInstanceName creates the instance name for operational discovery.
//
// Format: <zone-id>-<device-id>
func OperationalInstanceName(zoneID, deviceID string) string {
	return fmt.Sprintf("%s-%s", zoneID, deviceID)
}

// ParseOperationalInstanceName extracts zone ID and device ID from an operational instance name.
func ParseOperationalInstanceName(name string) (zoneID, deviceID string, err error) {
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return "", "", ErrInvalidTXTRecord
	}

	zoneID = parts[0]
	deviceID = parts[1]

	// Validate lengths (should be 16 hex chars each)
	if len(zoneID) != IDLength || len(deviceID) != IDLength {
		return "", "", ErrInvalidTXTRecord
	}

	// Validate hex format
	if !isHexString(zoneID) || !isHexString(deviceID) {
		return "", "", ErrInvalidTXTRecord
	}

	return zoneID, deviceID, nil
}

// isHexString checks if a string contains only valid hex characters.
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

package commissioning

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Setup code constants.
const (
	// SetupCodeLength is the number of digits in a setup code.
	SetupCodeLength = 8

	// SetupCodeMax is the maximum setup code value (99999999).
	SetupCodeMax = 99999999

	// DiscriminatorMax is the maximum discriminator value (12 bits).
	DiscriminatorMax = 0xFFF
)

// Setup code errors.
var (
	ErrInvalidSetupCode   = errors.New("invalid setup code")
	ErrInvalidQRCode      = errors.New("invalid QR code format")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
)

// SetupCode represents an 8-digit setup code.
type SetupCode uint32

// GenerateSetupCode generates a cryptographically random setup code.
func GenerateSetupCode() (SetupCode, error) {
	max := big.NewInt(SetupCodeMax + 1)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, fmt.Errorf("failed to generate random setup code: %w", err)
	}
	return SetupCode(n.Uint64()), nil
}

// ParseSetupCode parses an 8-digit string into a SetupCode.
func ParseSetupCode(s string) (SetupCode, error) {
	s = strings.TrimSpace(s)
	if len(s) != SetupCodeLength {
		return 0, fmt.Errorf("%w: must be %d digits", ErrInvalidSetupCode, SetupCodeLength)
	}

	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidSetupCode, err)
	}

	if n > SetupCodeMax {
		return 0, fmt.Errorf("%w: exceeds maximum value", ErrInvalidSetupCode)
	}

	return SetupCode(n), nil
}

// String returns the setup code as an 8-digit string with leading zeros.
func (sc SetupCode) String() string {
	return fmt.Sprintf("%08d", sc)
}

// Bytes returns the setup code as UTF-8 encoded bytes.
// This is used as the password input for SPAKE2+.
func (sc SetupCode) Bytes() []byte {
	return []byte(sc.String())
}

// Validate checks if the setup code is valid.
func (sc SetupCode) Validate() error {
	if sc > SetupCodeMax {
		return fmt.Errorf("%w: exceeds maximum value", ErrInvalidSetupCode)
	}
	return nil
}

// QRCodeData contains the data encoded in a MASH QR code.
type QRCodeData struct {
	// Version is the protocol version (currently 1).
	Version int

	// Discriminator is the 12-bit device discriminator for mDNS filtering.
	Discriminator uint16

	// SetupCode is the 8-digit setup code.
	SetupCode SetupCode

	// VendorID is the vendor identifier.
	VendorID uint16

	// ProductID is the product identifier.
	ProductID uint16
}

// qrCodeRegex matches the MASH QR code format.
var qrCodeRegex = regexp.MustCompile(`^MASH:(\d+):(\d+):(\d{8}):(0x[0-9a-fA-F]+|[0-9]+):(0x[0-9a-fA-F]+|[0-9]+)$`)

// ParseQRCode parses a MASH QR code string.
// Format: MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>
func ParseQRCode(data string) (*QRCodeData, error) {
	data = strings.TrimSpace(data)

	matches := qrCodeRegex.FindStringSubmatch(data)
	if matches == nil {
		return nil, fmt.Errorf("%w: expected MASH:<version>:<discriminator>:<setupcode>:<vendorid>:<productid>", ErrInvalidQRCode)
	}

	// Parse version
	version, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid version: %v", ErrInvalidQRCode, err)
	}
	if version != 1 {
		return nil, fmt.Errorf("%w: version %d", ErrUnsupportedVersion, version)
	}

	// Parse discriminator
	discriminator, err := strconv.ParseUint(matches[2], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid discriminator: %v", ErrInvalidQRCode, err)
	}
	if discriminator > DiscriminatorMax {
		return nil, fmt.Errorf("%w: discriminator exceeds 12 bits", ErrInvalidQRCode)
	}

	// Parse setup code
	setupCode, err := ParseSetupCode(matches[3])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidQRCode, err)
	}

	// Parse vendor ID (supports both hex and decimal)
	vendorID, err := parseID(matches[4])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid vendor ID: %v", ErrInvalidQRCode, err)
	}

	// Parse product ID (supports both hex and decimal)
	productID, err := parseID(matches[5])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid product ID: %v", ErrInvalidQRCode, err)
	}

	return &QRCodeData{
		Version:       version,
		Discriminator: uint16(discriminator),
		SetupCode:     setupCode,
		VendorID:      uint16(vendorID),
		ProductID:     uint16(productID),
	}, nil
}

// parseID parses a vendor or product ID, supporting both hex (0x...) and decimal.
func parseID(s string) (uint64, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 16)
	}
	return strconv.ParseUint(s, 10, 16)
}

// String returns the QR code as a string.
func (q *QRCodeData) String() string {
	return fmt.Sprintf("MASH:%d:%d:%s:0x%04X:0x%04X",
		q.Version, q.Discriminator, q.SetupCode.String(), q.VendorID, q.ProductID)
}

// GenerateDiscriminator generates a random 12-bit discriminator.
func GenerateDiscriminator() (uint16, error) {
	max := big.NewInt(DiscriminatorMax + 1)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, fmt.Errorf("failed to generate discriminator: %w", err)
	}
	return uint16(n.Uint64()), nil
}

// MustParseSetupCode parses a setup code string and panics on error.
// Use only in tests or when the setup code is known to be valid.
func MustParseSetupCode(s string) SetupCode {
	sc, err := ParseSetupCode(s)
	if err != nil {
		panic(err)
	}
	return sc
}

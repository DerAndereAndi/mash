// Package inspect provides device inspection and attribute manipulation utilities.
//
// The inspect package offers a unified interface for:
//   - Parsing path expressions (e.g., "1/measurement/acActivePower")
//   - Resolving names to numeric IDs
//   - Reading and writing attributes
//   - Formatting output for display
package inspect

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Path errors.
var (
	ErrEmptyPath     = errors.New("empty path")
	ErrInvalidPath   = errors.New("invalid path format")
	ErrInvalidNumber = errors.New("invalid numeric value in path")
)

// Path represents a parsed inspection path.
// Format: [device/]endpoint/feature/attribute or [device/]endpoint/feature/cmd/commandID
type Path struct {
	// DeviceID is the device identifier (empty for local device).
	DeviceID string

	// EndpointID is the endpoint number (0-255).
	EndpointID uint8

	// FeatureID is the feature type ID (0x0001-0xFFFF).
	FeatureID uint8

	// AttributeID is the attribute ID within the feature.
	AttributeID uint16

	// CommandID is the command ID (when IsCommand is true).
	CommandID uint8

	// IsCommand indicates this path refers to a command, not an attribute.
	IsCommand bool

	// IsPartial indicates the path doesn't include an attribute/command
	// (used for inspect operations that show all attributes).
	IsPartial bool

	// Raw stores the original input string.
	Raw string
}

// ParsePath parses a path string into a Path struct.
//
// Supported formats:
//   - "endpoint/feature/attribute" - local path
//   - "device/endpoint/feature/attribute" - remote path
//   - "endpoint/feature/cmd/commandID" - command path
//   - "endpoint/feature" - partial (for listing attributes)
//   - "endpoint" - partial (for listing features)
//
// Numeric values can be decimal or hex (0x prefix).
// Names are resolved via the name tables.
func ParsePath(input string) (*Path, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrEmptyPath
	}

	// Check for invalid patterns
	if strings.HasPrefix(input, "/") || strings.Contains(input, "//") {
		return nil, ErrInvalidPath
	}

	parts := strings.Split(input, "/")
	if len(parts) == 0 {
		return nil, ErrEmptyPath
	}

	p := &Path{Raw: input}

	// Determine if first part is a device ID or endpoint
	// Device IDs contain letters, endpoint IDs are numeric or known names
	firstPart := parts[0]
	isDeviceID := !isNumericOrEndpointName(firstPart)

	var pathParts []string
	if isDeviceID && len(parts) > 1 {
		p.DeviceID = firstPart
		pathParts = parts[1:]
	} else {
		pathParts = parts
	}

	if len(pathParts) == 0 {
		return nil, ErrInvalidPath
	}

	// Parse endpoint
	epID, err := parseEndpointID(pathParts[0])
	if err != nil {
		return nil, fmt.Errorf("endpoint: %w", err)
	}
	p.EndpointID = epID

	if len(pathParts) == 1 {
		p.IsPartial = true
		return p, nil
	}

	// Parse feature
	featID, err := parseFeatureID(pathParts[1])
	if err != nil {
		return nil, fmt.Errorf("feature: %w", err)
	}
	p.FeatureID = featID

	if len(pathParts) == 2 {
		p.IsPartial = true
		return p, nil
	}

	// Check for command path (endpoint/feature/cmd/id)
	if pathParts[2] == "cmd" {
		p.IsCommand = true
		if len(pathParts) < 4 {
			return nil, fmt.Errorf("command path missing command ID")
		}
		cmdID, err := parseUint8(pathParts[3])
		if err != nil {
			return nil, fmt.Errorf("command ID: %w", err)
		}
		p.CommandID = cmdID
		return p, nil
	}

	// Parse attribute
	attrID, err := parseAttributeID(pathParts[2], p.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("attribute: %w", err)
	}
	p.AttributeID = attrID

	return p, nil
}

// String returns the path as a string.
func (p *Path) String() string {
	var sb strings.Builder

	if p.DeviceID != "" {
		sb.WriteString(p.DeviceID)
		sb.WriteString("/")
	}

	sb.WriteString(strconv.Itoa(int(p.EndpointID)))

	if p.IsPartial && p.FeatureID == 0 {
		return sb.String()
	}

	sb.WriteString("/")
	sb.WriteString(strconv.Itoa(int(p.FeatureID)))

	if p.IsPartial && !p.IsCommand {
		return sb.String()
	}

	if p.IsCommand {
		sb.WriteString("/cmd/")
		sb.WriteString(strconv.Itoa(int(p.CommandID)))
	} else {
		sb.WriteString("/")
		sb.WriteString(strconv.Itoa(int(p.AttributeID)))
	}

	return sb.String()
}

// isNumericOrEndpointName checks if the string is a number or known endpoint name.
func isNumericOrEndpointName(s string) bool {
	// Check if numeric
	if _, err := strconv.ParseUint(s, 10, 8); err == nil {
		return true
	}
	// Check if hex
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if _, err := strconv.ParseUint(s[2:], 16, 8); err == nil {
			return true
		}
	}
	// Check if known endpoint name
	_, exists := endpointNames[strings.ToLower(s)]
	return exists
}

// parseEndpointID parses an endpoint ID from string.
func parseEndpointID(s string) (uint8, error) {
	// Try numeric first
	if id, err := parseUint8(s); err == nil {
		return id, nil
	}
	// Try name resolution
	if id, ok := endpointNames[strings.ToLower(s)]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("%w: %s", ErrInvalidNumber, s)
}

// parseFeatureID parses a feature ID from string.
func parseFeatureID(s string) (uint8, error) {
	// Try numeric first
	if id, err := parseUint8(s); err == nil {
		return id, nil
	}
	// Try name resolution
	if id, ok := featureNames[strings.ToLower(s)]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("%w: %s", ErrInvalidNumber, s)
}

// parseAttributeID parses an attribute ID from string.
func parseAttributeID(s string, featureID uint8) (uint16, error) {
	// Try numeric first
	if id, err := parseUint16(s); err == nil {
		return id, nil
	}
	// Try name resolution based on feature (case-insensitive)
	if id, ok := ResolveAttributeName(featureID, s); ok {
		return id, nil
	}
	return 0, fmt.Errorf("%w: %s", ErrInvalidNumber, s)
}

// parseUint8 parses a uint8 from decimal or hex string.
func parseUint8(s string) (uint8, error) {
	var v uint64
	var err error

	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err = strconv.ParseUint(s[2:], 16, 8)
	} else {
		v, err = strconv.ParseUint(s, 10, 8)
	}
	if err != nil {
		return 0, err
	}
	return uint8(v), nil
}

// parseUint16 parses a uint16 from decimal or hex string.
func parseUint16(s string) (uint16, error) {
	var v uint64
	var err error

	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, err = strconv.ParseUint(s[2:], 16, 16)
	} else {
		v, err = strconv.ParseUint(s, 10, 16)
	}
	if err != nil {
		return 0, err
	}
	return uint16(v), nil
}

// Package version provides protocol version parsing, comparison, and ALPN helpers.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Current is the protocol version implemented by this library.
const Current = "1.0"

// SpecVersion represents a parsed "major.minor" protocol version.
type SpecVersion struct {
	Major uint16
	Minor uint16
}

// Parse parses a "major.minor" version string.
func Parse(s string) (SpecVersion, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 2 {
		return SpecVersion{}, fmt.Errorf("invalid version %q: expected major.minor", s)
	}

	major, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil || parts[0] == "" {
		return SpecVersion{}, fmt.Errorf("invalid version %q: bad major component", s)
	}

	minor, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil || parts[1] == "" {
		return SpecVersion{}, fmt.Errorf("invalid version %q: bad minor component", s)
	}

	return SpecVersion{Major: uint16(major), Minor: uint16(minor)}, nil
}

// String returns the version as "major.minor".
func (v SpecVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// Compatible returns true if the other version has the same major version.
func (v SpecVersion) Compatible(other SpecVersion) bool {
	return v.Major == other.Major
}

// ALPNProtocol returns the ALPN protocol string for a major version: "mash/N".
func ALPNProtocol(major uint16) string {
	return fmt.Sprintf("mash/%d", major)
}

// MajorFromALPN extracts the major version from an ALPN protocol string.
func MajorFromALPN(alpn string) (uint16, error) {
	if !strings.HasPrefix(alpn, "mash/") {
		return 0, fmt.Errorf("not a MASH ALPN protocol: %q", alpn)
	}

	suffix := alpn[len("mash/"):]
	if suffix == "" {
		return 0, fmt.Errorf("empty major version in ALPN: %q", alpn)
	}

	major, err := strconv.ParseUint(suffix, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid major version in ALPN %q: %w", alpn, err)
	}

	return uint16(major), nil
}

// SupportedALPNProtocols returns the ALPN protocol strings for all supported
// major versions. Currently only major version 1.
func SupportedALPNProtocols() []string {
	current, _ := Parse(Current)
	return []string{ALPNProtocol(current.Major)}
}

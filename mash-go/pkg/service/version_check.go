package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/mash-protocol/mash-go/pkg/features"
	"github.com/mash-protocol/mash-go/pkg/model"
	"github.com/mash-protocol/mash-go/pkg/version"
)

// ErrIncompatibleVersion is returned when a device's specVersion is not
// compatible with this controller's protocol version.
var ErrIncompatibleVersion = errors.New("incompatible protocol version")

// checkVersionCompatibility checks if a device's specVersion is compatible
// with this controller. An empty string is treated as compatible (backward
// compatibility with devices that predate the specVersion attribute).
func checkVersionCompatibility(deviceSpecVersion string) error {
	if deviceSpecVersion == "" {
		return nil // assume compatible for backward compat
	}

	deviceVer, err := version.Parse(deviceSpecVersion)
	if err != nil {
		return fmt.Errorf("%w: invalid specVersion %q: %v", ErrIncompatibleVersion, deviceSpecVersion, err)
	}

	ourVer, _ := version.Parse(version.Current)
	if !ourVer.Compatible(deviceVer) {
		return fmt.Errorf("%w: device=%s, controller=%s", ErrIncompatibleVersion, deviceVer, ourVer)
	}

	return nil
}

// checkDeviceVersion reads the specVersion attribute from DeviceInfo and
// validates major version compatibility. This is a best-effort check:
// if the read fails (e.g., device doesn't support DeviceInfo), the
// connection proceeds.
func (s *ControllerService) checkDeviceVersion(ctx context.Context, session *DeviceSession) error {
	attrs, err := session.Read(ctx, 0, uint8(model.FeatureDeviceInfo), []uint16{features.DeviceInfoAttrSpecVersion})
	if err != nil {
		// Device may not support DeviceInfo or specVersion -- allow connection
		return nil
	}

	specVersionRaw, ok := attrs[features.DeviceInfoAttrSpecVersion]
	if !ok {
		// Attribute not present -- assume compatible
		return nil
	}

	specVersion, ok := specVersionRaw.(string)
	if !ok {
		// Not a string -- assume compatible (shouldn't happen)
		return nil
	}

	return checkVersionCompatibility(specVersion)
}

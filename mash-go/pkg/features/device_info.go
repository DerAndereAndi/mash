package features

import (
	"github.com/mash-protocol/mash-go/pkg/model"
)

// RemoveZone parameter and response keys (CBOR integer keys).
const (
	RemoveZoneParamZoneID = "zoneId"
	RemoveZoneRespRemoved = "removed"
)

// SetEndpoints sets the endpoint structure.
func (d *DeviceInfo) SetEndpoints(endpoints []*model.EndpointInfo) error {
	attr, err := d.GetAttribute(DeviceInfoAttrEndpoints)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(endpoints)
}

// WriteLocation sets the installation location via external write (ACL-enforced).
func (d *DeviceInfo) WriteLocation(location string) error {
	return d.WriteAttribute(DeviceInfoAttrLocation, location)
}

// WriteLabel sets the user-assigned name via external write (ACL-enforced).
func (d *DeviceInfo) WriteLabel(label string) error {
	return d.WriteAttribute(DeviceInfoAttrLabel, label)
}


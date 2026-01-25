package features

import (
	"github.com/mash-protocol/mash-go/pkg/model"
)

// DeviceInfo attribute IDs.
const (
	// Identification attributes (1-9)
	DeviceInfoAttrDeviceID     uint16 = 1
	DeviceInfoAttrVendorName   uint16 = 2
	DeviceInfoAttrProductName  uint16 = 3
	DeviceInfoAttrSerialNumber uint16 = 4
	DeviceInfoAttrVendorID     uint16 = 5
	DeviceInfoAttrProductID    uint16 = 6

	// Version attributes (10-19)
	DeviceInfoAttrSoftwareVersion uint16 = 10
	DeviceInfoAttrHardwareVersion uint16 = 11
	DeviceInfoAttrSpecVersion     uint16 = 12

	// Device structure (20-29)
	DeviceInfoAttrEndpoints uint16 = 20

	// Optional metadata (30-39)
	DeviceInfoAttrLocation uint16 = 30
	DeviceInfoAttrLabel    uint16 = 31
)

// DeviceInfoFeatureRevision is the current revision of the DeviceInfo feature.
const DeviceInfoFeatureRevision uint16 = 1

// DeviceInfo wraps a Feature with DeviceInfo-specific functionality.
type DeviceInfo struct {
	*model.Feature
}

// NewDeviceInfo creates a new DeviceInfo feature.
func NewDeviceInfo() *DeviceInfo {
	f := model.NewFeature(model.FeatureDeviceInfo, DeviceInfoFeatureRevision)

	// Add attributes

	// Identification
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrDeviceID,
		Name:        "deviceId",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Description: "Globally unique device identifier",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrVendorName,
		Name:        "vendorName",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Description: "Manufacturer name",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrProductName,
		Name:        "productName",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Description: "Product name",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrSerialNumber,
		Name:        "serialNumber",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Description: "Device serial number",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrVendorID,
		Name:        "vendorId",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "IANA Private Enterprise Number",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrProductID,
		Name:        "productId",
		Type:        model.DataTypeUint16,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Vendor's product ID",
	}))

	// Versions
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrSoftwareVersion,
		Name:        "softwareVersion",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Description: "Firmware/software version",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrHardwareVersion,
		Name:        "hardwareVersion",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Hardware revision",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrSpecVersion,
		Name:        "specVersion",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Default:     "1.0",
		Description: "MASH specification version",
	}))

	// Device structure
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrEndpoints,
		Name:        "endpoints",
		Type:        model.DataTypeArray,
		Access:      model.AccessRead,
		Description: "Complete device endpoint structure",
	}))

	// Optional metadata
	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrLocation,
		Name:        "location",
		Type:        model.DataTypeString,
		Access:      model.AccessReadWrite,
		Nullable:    true,
		Description: "Installation location",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          DeviceInfoAttrLabel,
		Name:        "label",
		Type:        model.DataTypeString,
		Access:      model.AccessReadWrite,
		Nullable:    true,
		Description: "User-assigned name",
	}))

	return &DeviceInfo{Feature: f}
}

// Setters for device implementation to populate values

// SetDeviceID sets the device ID.
func (d *DeviceInfo) SetDeviceID(id string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrDeviceID)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(id)
}

// SetVendorName sets the vendor/manufacturer name.
func (d *DeviceInfo) SetVendorName(name string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrVendorName)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(name)
}

// SetProductName sets the product name.
func (d *DeviceInfo) SetProductName(name string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrProductName)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(name)
}

// SetSerialNumber sets the serial number.
func (d *DeviceInfo) SetSerialNumber(sn string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrSerialNumber)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(sn)
}

// SetVendorID sets the IANA PEN (optional).
func (d *DeviceInfo) SetVendorID(id uint32) error {
	attr, err := d.GetAttribute(DeviceInfoAttrVendorID)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(id)
}

// SetProductID sets the vendor's product ID.
func (d *DeviceInfo) SetProductID(id uint16) error {
	attr, err := d.GetAttribute(DeviceInfoAttrProductID)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(id)
}

// SetSoftwareVersion sets the firmware/software version.
func (d *DeviceInfo) SetSoftwareVersion(version string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrSoftwareVersion)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(version)
}

// SetHardwareVersion sets the hardware revision (optional).
func (d *DeviceInfo) SetHardwareVersion(version string) error {
	attr, err := d.GetAttribute(DeviceInfoAttrHardwareVersion)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(version)
}

// SetLocation sets the installation location (user-writable).
func (d *DeviceInfo) SetLocation(location string) error {
	return d.WriteAttribute(DeviceInfoAttrLocation, location)
}

// SetLabel sets the user-assigned name (user-writable).
func (d *DeviceInfo) SetLabel(label string) error {
	return d.WriteAttribute(DeviceInfoAttrLabel, label)
}

// SetEndpoints sets the endpoint structure.
// This should be called with the device's current endpoint info.
func (d *DeviceInfo) SetEndpoints(endpoints []*model.EndpointInfo) error {
	attr, err := d.GetAttribute(DeviceInfoAttrEndpoints)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(endpoints)
}

// Getters for reading values

// DeviceID returns the device ID.
func (d *DeviceInfo) DeviceID() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrDeviceID)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// VendorName returns the vendor name.
func (d *DeviceInfo) VendorName() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrVendorName)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// ProductName returns the product name.
func (d *DeviceInfo) ProductName() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrProductName)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// SerialNumber returns the serial number.
func (d *DeviceInfo) SerialNumber() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrSerialNumber)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// VendorID returns the IANA PEN.
func (d *DeviceInfo) VendorID() uint32 {
	val, _ := d.ReadAttribute(DeviceInfoAttrVendorID)
	if v, ok := val.(uint32); ok {
		return v
	}
	return 0
}

// ProductID returns the product ID.
func (d *DeviceInfo) ProductID() uint16 {
	val, _ := d.ReadAttribute(DeviceInfoAttrProductID)
	if v, ok := val.(uint16); ok {
		return v
	}
	return 0
}

// SoftwareVersion returns the software version.
func (d *DeviceInfo) SoftwareVersion() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrSoftwareVersion)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// SpecVersion returns the MASH spec version.
func (d *DeviceInfo) SpecVersion() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrSpecVersion)
	if s, ok := val.(string); ok {
		return s
	}
	return "1.0"
}

// Location returns the installation location.
func (d *DeviceInfo) Location() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrLocation)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// Label returns the user-assigned label.
func (d *DeviceInfo) Label() string {
	val, _ := d.ReadAttribute(DeviceInfoAttrLabel)
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

package features

import (
	"github.com/mash-protocol/mash-go/pkg/model"
)

// Status attribute IDs.
const (
	StatusAttrOperatingState uint16 = 1
	StatusAttrStateDetail    uint16 = 2
	StatusAttrFaultCode      uint16 = 3
	StatusAttrFaultMessage   uint16 = 4
)

// StatusFeatureRevision is the current revision of the Status feature.
const StatusFeatureRevision uint16 = 1

// OperatingState represents the operating state of a device/endpoint.
type OperatingState uint8

const (
	// OperatingStateUnknown indicates state not known.
	OperatingStateUnknown OperatingState = 0x00

	// OperatingStateOffline indicates not connected / not available.
	OperatingStateOffline OperatingState = 0x01

	// OperatingStateStandby indicates ready but not active.
	OperatingStateStandby OperatingState = 0x02

	// OperatingStateStarting indicates powering up / initializing.
	OperatingStateStarting OperatingState = 0x03

	// OperatingStateRunning indicates actively operating.
	OperatingStateRunning OperatingState = 0x04

	// OperatingStatePaused indicates temporarily paused (can resume).
	OperatingStatePaused OperatingState = 0x05

	// OperatingStateShuttingDown indicates powering down.
	OperatingStateShuttingDown OperatingState = 0x06

	// OperatingStateFault indicates error condition (check faultCode).
	OperatingStateFault OperatingState = 0x07

	// OperatingStateMaintenance indicates under maintenance / firmware update.
	OperatingStateMaintenance OperatingState = 0x08
)

// String returns the operating state name.
func (s OperatingState) String() string {
	switch s {
	case OperatingStateUnknown:
		return "UNKNOWN"
	case OperatingStateOffline:
		return "OFFLINE"
	case OperatingStateStandby:
		return "STANDBY"
	case OperatingStateStarting:
		return "STARTING"
	case OperatingStateRunning:
		return "RUNNING"
	case OperatingStatePaused:
		return "PAUSED"
	case OperatingStateShuttingDown:
		return "SHUTTING_DOWN"
	case OperatingStateFault:
		return "FAULT"
	case OperatingStateMaintenance:
		return "MAINTENANCE"
	default:
		return "UNKNOWN"
	}
}

// Status wraps a Feature with Status-specific functionality.
// It provides per-endpoint operating state and health information.
type Status struct {
	*model.Feature
}

// NewStatus creates a new Status feature.
func NewStatus() *Status {
	f := model.NewFeature(model.FeatureStatus, StatusFeatureRevision)

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          StatusAttrOperatingState,
		Name:        "operatingState",
		Type:        model.DataTypeUint8,
		Access:      model.AccessReadOnly,
		Default:     uint8(OperatingStateUnknown),
		Description: "Current operating state",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          StatusAttrStateDetail,
		Name:        "stateDetail",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Vendor-specific state detail code",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          StatusAttrFaultCode,
		Name:        "faultCode",
		Type:        model.DataTypeUint32,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Fault/error code when state=FAULT",
	}))

	f.AddAttribute(model.NewAttribute(&model.AttributeMetadata{
		ID:          StatusAttrFaultMessage,
		Name:        "faultMessage",
		Type:        model.DataTypeString,
		Access:      model.AccessReadOnly,
		Nullable:    true,
		Description: "Human-readable fault description",
	}))

	return &Status{Feature: f}
}

// Setters

// SetOperatingState sets the operating state.
func (s *Status) SetOperatingState(state OperatingState) error {
	attr, err := s.GetAttribute(StatusAttrOperatingState)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(uint8(state))
}

// SetStateDetail sets the vendor-specific state detail code.
func (s *Status) SetStateDetail(detail uint32) error {
	attr, err := s.GetAttribute(StatusAttrStateDetail)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(detail)
}

// ClearStateDetail clears the state detail.
func (s *Status) ClearStateDetail() error {
	attr, err := s.GetAttribute(StatusAttrStateDetail)
	if err != nil {
		return err
	}
	return attr.SetValueInternal(nil)
}

// SetFault sets the fault state with code and message.
func (s *Status) SetFault(code uint32, message string) error {
	if err := s.SetOperatingState(OperatingStateFault); err != nil {
		return err
	}

	if faultCodeAttr, err := s.GetAttribute(StatusAttrFaultCode); err == nil {
		_ = faultCodeAttr.SetValueInternal(code)
	}

	if faultMsgAttr, err := s.GetAttribute(StatusAttrFaultMessage); err == nil {
		_ = faultMsgAttr.SetValueInternal(message)
	}

	return nil
}

// ClearFault clears the fault state.
func (s *Status) ClearFault() error {
	if faultCodeAttr, err := s.GetAttribute(StatusAttrFaultCode); err == nil {
		_ = faultCodeAttr.SetValueInternal(nil)
	}

	if faultMsgAttr, err := s.GetAttribute(StatusAttrFaultMessage); err == nil {
		_ = faultMsgAttr.SetValueInternal(nil)
	}

	return nil
}

// Getters

// OperatingState returns the current operating state.
func (s *Status) OperatingState() OperatingState {
	val, _ := s.ReadAttribute(StatusAttrOperatingState)
	if v, ok := val.(uint8); ok {
		return OperatingState(v)
	}
	return OperatingStateUnknown
}

// StateDetail returns the vendor-specific state detail.
func (s *Status) StateDetail() (uint32, bool) {
	val, err := s.ReadAttribute(StatusAttrStateDetail)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint32); ok {
		return v, true
	}
	return 0, false
}

// FaultCode returns the fault code.
func (s *Status) FaultCode() (uint32, bool) {
	val, err := s.ReadAttribute(StatusAttrFaultCode)
	if err != nil || val == nil {
		return 0, false
	}
	if v, ok := val.(uint32); ok {
		return v, true
	}
	return 0, false
}

// FaultMessage returns the fault message.
func (s *Status) FaultMessage() string {
	val, _ := s.ReadAttribute(StatusAttrFaultMessage)
	if v, ok := val.(string); ok {
		return v
	}
	return ""
}

// Helper methods

// IsFaulted returns true if the device is in fault state.
func (s *Status) IsFaulted() bool {
	return s.OperatingState() == OperatingStateFault
}

// IsRunning returns true if the device is actively operating.
func (s *Status) IsRunning() bool {
	return s.OperatingState() == OperatingStateRunning
}

// IsReady returns true if the device is ready (standby or running).
func (s *Status) IsReady() bool {
	state := s.OperatingState()
	return state == OperatingStateStandby || state == OperatingStateRunning
}

// IsOffline returns true if the device is offline.
func (s *Status) IsOffline() bool {
	return s.OperatingState() == OperatingStateOffline
}

package features

// SetFault sets the fault state with code and message.
func (s *Status) SetFault(code uint32, message string) error {
	if err := s.SetOperatingState(OperatingStateFault); err != nil {
		return err
	}
	_ = s.SetFaultCode(code)
	_ = s.SetFaultMessage(message)
	return nil
}

// ClearFault clears the fault state.
func (s *Status) ClearFault() error {
	_ = s.ClearFaultCode()
	_ = s.ClearFaultMessage()
	return nil
}

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

package features

// SetCapabilities sets the control capabilities.
func (e *EnergyControl) SetCapabilities(limits, currentLimits, setpoints, currentSetpoints, pausable, shiftable, stoppable bool) {
	_ = e.SetAcceptsLimits(limits)
	_ = e.SetAcceptsCurrentLimits(currentLimits)
	_ = e.SetAcceptsSetpoints(setpoints)
	_ = e.SetAcceptsCurrentSetpoints(currentSetpoints)
	_ = e.SetIsPausable(pausable)
	_ = e.SetIsShiftable(shiftable)
	_ = e.SetIsStoppable(stoppable)
}

// IsLimited returns true if currently in LIMITED state.
func (e *EnergyControl) IsLimited() bool {
	return e.ControlState() == ControlStateLimited
}

// IsFailsafe returns true if in FAILSAFE state.
func (e *EnergyControl) IsFailsafe() bool {
	return e.ControlState() == ControlStateFailsafe
}

// IsOverride returns true if in OVERRIDE state.
func (e *EnergyControl) IsOverride() bool {
	return e.ControlState() == ControlStateOverride
}

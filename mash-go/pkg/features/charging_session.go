package features

// Domain helper methods - EV energy requests

// SetEVEnergyRequests sets the EV's energy requests.
func (c *ChargingSession) SetEVEnergyRequests(min, max, target *int64) error {
	if err := c.SetEVMinEnergyRequestPtr(min); err != nil {
		return err
	}
	if err := c.SetEVMaxEnergyRequestPtr(max); err != nil {
		return err
	}
	return c.SetEVTargetEnergyRequestPtr(target)
}

// Domain helper methods - V2G constraints

// SetEVDischargeConstraints sets the V2G discharge constraints.
func (c *ChargingSession) SetEVDischargeConstraints(minDischarge, maxDischarge *int64, belowTargetPermitted *bool) error {
	if err := c.SetEVMinDischargingRequestPtr(minDischarge); err != nil {
		return err
	}
	if err := c.SetEVMaxDischargingRequestPtr(maxDischarge); err != nil {
		return err
	}
	return c.SetEVDischargeBelowTargetPermittedPtr(belowTargetPermitted)
}

// Domain helper methods - Estimated times

// SetEstimatedTimes sets the estimated times to various SoC levels.
func (c *ChargingSession) SetEstimatedTimes(toMin, toTarget, toFull *uint32) error {
	if err := c.SetEstimatedTimeToMinSoCPtr(toMin); err != nil {
		return err
	}
	if err := c.SetEstimatedTimeToTargetSoCPtr(toTarget); err != nil {
		return err
	}
	return c.SetEstimatedTimeToFullSoCPtr(toFull)
}

// Domain helper methods - Session management

// IsPluggedIn returns true if an EV is connected.
func (c *ChargingSession) IsPluggedIn() bool {
	return c.State() != ChargingStateNotPluggedIn
}

// IsCharging returns true if actively charging.
func (c *ChargingSession) IsCharging() bool {
	return c.State() == ChargingStatePluggedInCharging
}

// IsDischarging returns true if V2G discharging.
func (c *ChargingSession) IsDischarging() bool {
	return c.State() == ChargingStatePluggedInDischarging
}

// CanDischarge returns true if V2G discharge is permitted.
func (c *ChargingSession) CanDischarge() bool {
	minVal, err := c.ReadAttribute(ChargingSessionAttrEVMinDischargingRequest)
	if err != nil || minVal == nil {
		return false
	}
	minDischarge, ok := minVal.(int64)
	if !ok || minDischarge >= 0 {
		return false
	}

	maxVal, err := c.ReadAttribute(ChargingSessionAttrEVMaxDischargingRequest)
	if err != nil || maxVal == nil {
		return false
	}
	maxDischarge, ok := maxVal.(int64)
	if !ok || maxDischarge < 0 {
		return false
	}

	targetVal, err := c.ReadAttribute(ChargingSessionAttrEVTargetEnergyRequest)
	if err == nil && targetVal != nil {
		if target, ok := targetVal.(int64); ok && target <= 0 {
			return true
		}
	}

	permVal, err := c.ReadAttribute(ChargingSessionAttrEVDischargeBelowTargetPermitted)
	if err != nil || permVal == nil {
		return false
	}
	if perm, ok := permVal.(bool); ok {
		return perm
	}
	return false
}

// SupportsMode returns true if the given charging mode is supported.
func (c *ChargingSession) SupportsMode(mode ChargingMode) bool {
	val, err := c.ReadAttribute(ChargingSessionAttrSupportedChargingModes)
	if err != nil || val == nil {
		return mode == ChargingModeOff
	}
	if modes, ok := val.([]uint8); ok {
		for _, m := range modes {
			if ChargingMode(m) == mode {
				return true
			}
		}
	}
	return false
}

// StartSession begins a new charging session.
func (c *ChargingSession) StartSession(startTime uint64) error {
	if err := c.SetSessionStartTime(startTime); err != nil {
		return err
	}
	if err := c.ClearSessionEndTime(); err != nil {
		return err
	}
	if err := c.SetSessionEnergyCharged(0); err != nil {
		return err
	}
	if err := c.SetSessionEnergyDischarged(0); err != nil {
		return err
	}
	return c.SetState(ChargingStatePluggedInNoDemand)
}

// EndSession ends the current charging session.
func (c *ChargingSession) EndSession(endTime uint64) error {
	if err := c.SetSessionEndTime(endTime); err != nil {
		return err
	}
	return c.SetState(ChargingStateNotPluggedIn)
}

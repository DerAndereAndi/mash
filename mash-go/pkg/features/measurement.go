package features

// Domain helper methods

// IsConsuming returns true if currently consuming power (positive active power).
func (m *Measurement) IsConsuming() bool {
	power, ok := m.ACActivePower()
	return ok && power > 0
}

// IsProducing returns true if currently producing power (negative active power).
func (m *Measurement) IsProducing() bool {
	power, ok := m.ACActivePower()
	return ok && power < 0
}

// ActivePowerKW returns the active power in kW (for convenience).
func (m *Measurement) ActivePowerKW() (float64, bool) {
	power, ok := m.ACActivePower()
	if !ok {
		return 0, false
	}
	return float64(power) / 1_000_000.0, true
}

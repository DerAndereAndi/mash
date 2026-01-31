package features

// HasActivePriceSignal returns true if price slots are currently set.
func (s *Signals) HasActivePriceSignal() bool {
	_, ok := s.PriceSlots()
	return ok
}

// HasActiveConstraints returns true if constraint slots are currently set.
func (s *Signals) HasActiveConstraints() bool {
	_, ok := s.ConstraintSlots()
	return ok
}

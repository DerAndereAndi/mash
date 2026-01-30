package features

// IsBidirectional returns true if the device supports both consumption and production.
func (e *Electrical) IsBidirectional() bool {
	return e.SupportedDirections() == DirectionBidirectional
}

// CanConsume returns true if the device can consume power.
func (e *Electrical) CanConsume() bool {
	dir := e.SupportedDirections()
	return dir == DirectionConsumption || dir == DirectionBidirectional
}

// CanProduce returns true if the device can produce power.
func (e *Electrical) CanProduce() bool {
	dir := e.SupportedDirections()
	return dir == DirectionProduction || dir == DirectionBidirectional
}

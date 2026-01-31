package features

// IsCommitted returns true if the plan commitment is COMMITTED or higher.
func (p *Plan) IsCommitted() bool {
	return p.Commitment() >= CommitmentCommitted
}

// IsExecuting returns true if the plan is currently being executed.
func (p *Plan) IsExecuting() bool {
	return p.Commitment() == CommitmentExecuting
}

package runner

import "github.com/mash-protocol/mash-go/internal/testharness/engine"

// invalidateCommissioningState clears partial commissioning artifacts after a
// lifecycle invariant breach (e.g. cert exchange or operational transition
// failure). This prevents stale crypto/session state from poisoning later tests.
func (r *Runner) invalidateCommissioningState(state *engine.ExecutionState) {
	r.disconnectConnection()
	r.closeAllZoneConns()
	r.connMgr.SetPASEState(nil)
	r.connMgr.ClearAllCrypto()
	r.suite.Clear()

	if state == nil {
		return
	}
	state.Set(KeySessionEstablished, false)
	state.Set(KeyConnectionEstablished, false)
	state.Set(StateCommissioningCompleted, false)
	state.Set(StateCommissioningActive, false)
	state.Set(StateSessionKey, nil)
	state.Set(StateSessionKeyLen, 0)
	state.Set(StateCurrentZoneID, "")
}

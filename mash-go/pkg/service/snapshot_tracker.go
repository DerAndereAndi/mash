package service

import (
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// snapshotTracker monitors message flow and triggers capability snapshot
// emission based on a hybrid policy (message count OR time interval).
type snapshotTracker struct {
	policy       SnapshotPolicy
	messageCount int
	lastSnapshot time.Time
	logger       log.Logger
	connID       string
	localRole    log.Role

	localDevice *model.Device
	remoteCache *log.DeviceSnapshot

	// timeNow returns the current time. Defaults to time.Now.
	// Replaced in tests for deterministic behavior.
	timeNow func() time.Time
}

// newSnapshotTracker creates a tracker with the given policy and data sources.
func newSnapshotTracker(policy SnapshotPolicy, device *model.Device, logger log.Logger, connID string, role log.Role) *snapshotTracker {
	return &snapshotTracker{
		policy:      policy,
		localDevice: device,
		logger:      logger,
		connID:      connID,
		localRole:   role,
		timeNow:     time.Now,
	}
}

// onMessageLogged is called after each protocol message is logged.
// It increments the counter and emits a snapshot if a trigger fires.
func (t *snapshotTracker) onMessageLogged() {
	if t.logger == nil {
		return
	}
	t.messageCount++
	if t.shouldEmit() {
		t.emitSnapshot()
	}
}

// shouldEmit checks whether a snapshot should be emitted based on the
// hybrid trigger policy.
func (t *snapshotTracker) shouldEmit() bool {
	// Message count trigger: fires unconditionally when threshold is reached.
	if t.policy.MaxMessages > 0 && t.messageCount >= t.policy.MaxMessages {
		return true
	}

	// Time trigger: fires only if enough messages have been logged since the
	// last snapshot (MinMessages floor).
	if t.policy.MaxInterval > 0 && t.timeNow().Sub(t.lastSnapshot) >= t.policy.MaxInterval {
		if t.messageCount >= t.policy.MinMessages {
			return true
		}
	}

	return false
}

// emitSnapshot logs a capability snapshot event and resets the counters.
func (t *snapshotTracker) emitSnapshot() {
	now := t.timeNow()
	t.messageCount = 0
	t.lastSnapshot = now

	t.logger.Log(log.Event{
		Timestamp:    now,
		ConnectionID: t.connID,
		Layer:        log.LayerService,
		Category:     log.CategorySnapshot,
		LocalRole:    t.localRole,
		Snapshot: &log.CapabilitySnapshotEvent{
			Local:  buildDeviceSnapshot(t.localDevice),
			Remote: t.remoteCache,
		},
	})
}

// emitInitialSnapshot emits a snapshot unconditionally. Called at session
// establishment to ensure every log segment starts with context.
func (t *snapshotTracker) emitInitialSnapshot() {
	if t.logger == nil {
		return
	}
	t.emitSnapshot()
}

// setRemoteCache updates the cached remote device snapshot.
func (t *snapshotTracker) setRemoteCache(snapshot *log.DeviceSnapshot) {
	t.remoteCache = snapshot
}

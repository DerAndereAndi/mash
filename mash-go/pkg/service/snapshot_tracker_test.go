package service

import (
	"sync"
	"testing"
	"time"

	"github.com/mash-protocol/mash-go/pkg/log"
	"github.com/mash-protocol/mash-go/pkg/model"
)

// snapshotCapturingLogger records log events for snapshot tracker tests.
type snapshotCapturingLogger struct {
	mu     sync.Mutex
	events []log.Event
}

func (l *snapshotCapturingLogger) Log(event log.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *snapshotCapturingLogger) snapshotCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, e := range l.events {
		if e.Category == log.CategorySnapshot {
			count++
		}
	}
	return count
}

func (l *snapshotCapturingLogger) lastEvent() log.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.events[len(l.events)-1]
}

// fakeClock provides a controllable time source for deterministic tests.
type fakeClock struct {
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock {
	return &fakeClock{now: t}
}

func (c *fakeClock) Now() time.Time {
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.now = c.now.Add(d)
}

func newTestTracker(policy SnapshotPolicy, logger log.Logger, clock *fakeClock) *snapshotTracker {
	device := model.NewDevice("test-device", 0x1234, 0x0001)
	tr := newSnapshotTracker(policy, device, logger, "conn-test", log.RoleDevice)
	tr.timeNow = clock.Now
	// Set lastSnapshot to clock start so time triggers are relative to it.
	tr.lastSnapshot = clock.Now()
	return tr
}

func TestSnapshotTracker_MaxMessagesTriggersEmission(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 10}, logger, clock)

	for range 9 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 0 {
		t.Fatalf("expected 0 snapshots after 9 messages, got %d", logger.snapshotCount())
	}

	tr.onMessageLogged() // 10th message
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot after 10 messages, got %d", logger.snapshotCount())
	}
	if tr.messageCount != 0 {
		t.Errorf("messageCount should reset to 0 after emission, got %d", tr.messageCount)
	}
}

func TestSnapshotTracker_MaxIntervalTriggersEmission(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxInterval: 5 * time.Minute, MinMessages: 0}, logger, clock)

	// Advance past interval, then log a message to trigger the check.
	clock.Advance(5 * time.Minute)
	tr.onMessageLogged()

	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot after interval elapsed, got %d", logger.snapshotCount())
	}
}

func TestSnapshotTracker_MinMessagesFloor(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxInterval: 5 * time.Minute, MinMessages: 50}, logger, clock)

	// Advance past interval, log 3 messages -- below floor, no snapshot.
	clock.Advance(5 * time.Minute)
	for range 3 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 0 {
		t.Fatalf("expected 0 snapshots (below MinMessages floor), got %d", logger.snapshotCount())
	}

	// Log 46 more (total 49) -- still below floor.
	for range 46 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 0 {
		t.Fatalf("expected 0 snapshots at 49 messages, got %d", logger.snapshotCount())
	}

	// 50th message: interval has elapsed AND messageCount reaches floor -> emit.
	tr.onMessageLogged()
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot at 50 messages (MinMessages floor met), got %d", logger.snapshotCount())
	}
}

func TestSnapshotTracker_MinMessagesFloor_SuppressesTimeTriger(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxInterval: 5 * time.Minute, MinMessages: 50}, logger, clock)

	// Advance past interval
	clock.Advance(6 * time.Minute)

	// Log only 3 messages (below floor of 50)
	for range 3 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 0 {
		t.Fatalf("time trigger should be suppressed when below MinMessages floor, got %d snapshots", logger.snapshotCount())
	}
}

func TestSnapshotTracker_BothDisabled(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxInterval: 0, MaxMessages: 0}, logger, clock)

	// Log many messages, advance time significantly.
	for range 10000 {
		tr.onMessageLogged()
	}
	clock.Advance(24 * time.Hour)
	tr.onMessageLogged()

	if logger.snapshotCount() != 0 {
		t.Fatalf("expected 0 snapshots with both triggers disabled, got %d", logger.snapshotCount())
	}
}

func TestSnapshotTracker_NilLoggerIsNoop(t *testing.T) {
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 1}, nil, clock)

	// Should not panic.
	for range 100 {
		tr.onMessageLogged()
	}
	tr.emitInitialSnapshot()
}

func TestSnapshotTracker_MessageCountResetsAfterEmission(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 5}, logger, clock)

	// First batch: 5 messages -> snapshot
	for range 5 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot after first batch, got %d", logger.snapshotCount())
	}

	// 4 more messages: no snapshot yet
	for range 4 {
		tr.onMessageLogged()
	}
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected still 1 snapshot after 4 more messages, got %d", logger.snapshotCount())
	}

	// 5th message of second batch: snapshot
	tr.onMessageLogged()
	if logger.snapshotCount() != 2 {
		t.Fatalf("expected 2 snapshots after second batch, got %d", logger.snapshotCount())
	}
}

func TestSnapshotTracker_TimeResetsAfterEmission(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxInterval: 5 * time.Minute, MinMessages: 0}, logger, clock)

	// First trigger
	clock.Advance(5 * time.Minute)
	tr.onMessageLogged()
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot after first interval, got %d", logger.snapshotCount())
	}

	// Not enough time elapsed yet
	clock.Advance(4 * time.Minute)
	tr.onMessageLogged()
	if logger.snapshotCount() != 1 {
		t.Fatalf("expected still 1 snapshot before second interval, got %d", logger.snapshotCount())
	}

	// Second trigger
	clock.Advance(1 * time.Minute) // total 5 min since last emission
	tr.onMessageLogged()
	if logger.snapshotCount() != 2 {
		t.Fatalf("expected 2 snapshots after second interval, got %d", logger.snapshotCount())
	}
}

func TestSnapshotTracker_EmittedEventStructure(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 1}, logger, clock)

	tr.onMessageLogged()

	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot, got %d", logger.snapshotCount())
	}

	event := logger.lastEvent()
	if event.Category != log.CategorySnapshot {
		t.Errorf("Category: got %v, want %v", event.Category, log.CategorySnapshot)
	}
	if event.Layer != log.LayerService {
		t.Errorf("Layer: got %v, want %v", event.Layer, log.LayerService)
	}
	if event.ConnectionID != "conn-test" {
		t.Errorf("ConnectionID: got %q, want %q", event.ConnectionID, "conn-test")
	}
	if event.LocalRole != log.RoleDevice {
		t.Errorf("LocalRole: got %v, want %v", event.LocalRole, log.RoleDevice)
	}
	if event.Snapshot == nil {
		t.Fatal("Snapshot is nil")
	}
	if event.Snapshot.Local == nil {
		t.Fatal("Snapshot.Local is nil")
	}
	if event.Snapshot.Local.DeviceID != "test-device" {
		t.Errorf("Local.DeviceID: got %q, want %q", event.Snapshot.Local.DeviceID, "test-device")
	}
	// Remote should be nil since we didn't set a cache.
	if event.Snapshot.Remote != nil {
		t.Errorf("Snapshot.Remote: expected nil, got %+v", event.Snapshot.Remote)
	}
}

func TestSnapshotTracker_EmitInitialSnapshot(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 1000, MaxInterval: 30 * time.Minute}, logger, clock)

	// Initial snapshot fires regardless of message count or time.
	tr.emitInitialSnapshot()

	if logger.snapshotCount() != 1 {
		t.Fatalf("expected 1 snapshot from emitInitialSnapshot, got %d", logger.snapshotCount())
	}

	// lastSnapshot should be set to prevent immediate re-trigger.
	if tr.lastSnapshot != clock.Now() {
		t.Errorf("lastSnapshot: got %v, want %v", tr.lastSnapshot, clock.Now())
	}
	if tr.messageCount != 0 {
		t.Errorf("messageCount should be 0 after initial snapshot, got %d", tr.messageCount)
	}
}

func TestSnapshotTracker_EmitWithRemoteCache(t *testing.T) {
	logger := &snapshotCapturingLogger{}
	clock := newFakeClock(time.Date(2026, 2, 2, 12, 0, 0, 0, time.UTC))
	tr := newTestTracker(SnapshotPolicy{MaxMessages: 1}, logger, clock)

	remote := &log.DeviceSnapshot{
		DeviceID: "remote-controller",
		Endpoints: []log.EndpointSnapshot{
			{ID: 0, Type: 0x00},
		},
	}
	tr.setRemoteCache(remote)

	tr.onMessageLogged()

	event := logger.lastEvent()
	if event.Snapshot.Remote == nil {
		t.Fatal("Snapshot.Remote is nil after setting remote cache")
	}
	if event.Snapshot.Remote.DeviceID != "remote-controller" {
		t.Errorf("Remote.DeviceID: got %q, want %q", event.Snapshot.Remote.DeviceID, "remote-controller")
	}
}
